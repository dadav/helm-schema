package searching

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/dadav/helm-schema/pkg/chart"
	"gopkg.in/yaml.v3"
)

type DiscoveredChart = chart.Instance

type ExtractedArchive struct {
	Directory  string
	SourcePath string
}

func extractTGZ(src, dest string) error {
	file, err := os.Open(src)
	if err != nil {
		return err
	}
	defer file.Close()

	// Open gzip reader
	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()

	// Open tar reader
	tr := tar.NewReader(gzr)

	// Extract files
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Resolve and sanitize file path
		cleanName := filepath.Clean(header.Name)
		// Prevent absolute paths
		if filepath.IsAbs(cleanName) {
			return fmt.Errorf("tar entry has absolute path: %s", cleanName)
		}
		// Prevent path traversal outside dest
		target := filepath.Join(dest, cleanName)
		rel, err := filepath.Rel(dest, target)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %v", err)
		}
		if strings.HasPrefix(rel, "..") || rel == ".." {
			return fmt.Errorf("tar entry attempts to write outside destination: %s", cleanName)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			// Create directory if not exists
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			// Ensure directory exists
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}

			// Create file
			outFile, err := os.Create(target)
			if err != nil {
				return err
			}
			// Copy file content
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return err
			}
			if err := outFile.Close(); err != nil {
				return err
			}
		}
	}
	return nil
}

func SearchFiles(chartSearchRoot, startPath, fileName string, dependenciesFilter map[string]bool, queue chan<- string, errs chan<- error) {
	defer close(queue)
	discoveredCharts, discoveryErrors := DiscoverCharts(chartSearchRoot, startPath, fileName, dependenciesFilter)
	for _, err := range discoveryErrors {
		errs <- err
	}
	for _, discovered := range discoveredCharts {
		queue <- discovered.Path
	}
}

// DiscoverCharts reads chart metadata before values files are processed. This
// lets callers decide which charts should enter the schema worker pipeline.
func DiscoverCharts(chartSearchRoot, startPath, fileName string, dependenciesFilter map[string]bool) ([]DiscoveredChart, []error) {
	return DiscoverChartsWithSources(chartSearchRoot, startPath, fileName, dependenciesFilter, nil)
}

// DiscoverChartsWithSources preserves archive provenance even though workers
// read chart files from temporary extraction directories.
func DiscoverChartsWithSources(chartSearchRoot, startPath, fileName string, dependenciesFilter map[string]bool, archives []ExtractedArchive) ([]DiscoveredChart, []error) {
	discovered := []DiscoveredChart{}
	discoveryErrors := []error{}
	var err error
	chartSearchRoot, err = filepath.Abs(chartSearchRoot)
	if err != nil {
		return nil, []error{err}
	}
	startPath, err = filepath.Abs(startPath)
	if err != nil {
		return nil, []error{err}
	}

	err = filepath.Walk(startPath, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			discoveryErrors = append(discoveryErrors, walkErr)
			return nil
		}
		if info.IsDir() || info.Name() != fileName {
			return nil
		}

		chartData, readErr := os.ReadFile(path)
		if readErr != nil {
			discoveryErrors = append(discoveryErrors, fmt.Errorf("failed to read Chart.yaml at %s: %w", path, readErr))
			return nil
		}

		var chartFile chart.ChartFile
		if unmarshalErr := yaml.Unmarshal(chartData, &chartFile); unmarshalErr != nil {
			discoveryErrors = append(discoveryErrors, fmt.Errorf("failed to parse Chart.yaml at %s: %w", path, unmarshalErr))
			return nil
		}

		instance := DiscoveredChart{ID: path, Path: path, Chart: chartFile}
		for _, archive := range archives {
			rel, err := filepath.Rel(archive.Directory, path)
			if err == nil && filepath.IsLocal(rel) {
				instance.ID = filepath.Join(archive.SourcePath, rel)
				instance.Archived = true
				break
			}
		}
		discovered = append(discovered, instance)
		return nil
	})
	if err != nil {
		discoveryErrors = append(discoveryErrors, err)
	}

	owners := chart.ChartOwners(discovered)
	selected := make([]DiscoveredChart, 0, len(discovered))
	for _, instance := range discovered {
		instance.OwnerID = owners[instance.ID]
		isSearchRootChart := filepath.Dir(instance.Path) == chartSearchRoot
		if !isSearchRootChart && len(dependenciesFilter) > 0 && !dependenciesFilter[instance.Chart.Name] {
			continue
		}
		selected = append(selected, instance)
	}
	return selected, discoveryErrors
}

// DiscoverArchives extracts archives and returns every discovery error. The caller
// must remove the returned temporary directory, including when errors occur.
func DiscoverArchives(startPath string) (string, []error) {
	tempDir, _, errs := DiscoverArchivesWithSources(startPath)
	return tempDir, errs
}

// DiscoverArchivesWithSources isolates every archive and records stable source
// paths, including nested archives. The caller owns cleanup of tempDir.
func DiscoverArchivesWithSources(startPath string) (string, []ExtractedArchive, []error) {
	startPath, err := filepath.Abs(startPath)
	if err != nil {
		return "", nil, []error{err}
	}
	paths, discoveryErrors := findArchives(startPath)
	if len(paths) == 0 {
		return "", nil, discoveryErrors
	}
	tempDir, err := os.MkdirTemp(filepath.Dir(paths[0]), "tmp-*")
	if err != nil {
		return "", nil, append(discoveryErrors, fmt.Errorf("failed to create chart extraction directory: %w", err))
	}
	type archiveSource struct {
		path   string
		source string
	}
	queue := make([]archiveSource, 0, len(paths))
	for _, path := range paths {
		queue = append(queue, archiveSource{path: path, source: path})
	}
	archives := []ExtractedArchive{}
	for index := 0; index < len(queue); index++ {
		archive := queue[index]
		directory := filepath.Join(tempDir, fmt.Sprintf("archive-%d", index))
		if err := os.MkdirAll(directory, 0o755); err != nil {
			discoveryErrors = append(discoveryErrors, fmt.Errorf("failed to create extraction directory for %s: %w", archive.source, err))
			continue
		}
		if err := extractTGZ(archive.path, directory); err != nil {
			discoveryErrors = append(discoveryErrors, fmt.Errorf("failed to extract %s: %w", archive.source, err))
			continue
		}
		archives = append(archives, ExtractedArchive{Directory: directory, SourcePath: archive.source})
		nestedPaths, nestedErrors := findArchives(directory)
		discoveryErrors = append(discoveryErrors, nestedErrors...)
		for _, path := range nestedPaths {
			rel, err := filepath.Rel(directory, path)
			if err != nil {
				discoveryErrors = append(discoveryErrors, err)
				continue
			}
			queue = append(queue, archiveSource{path: path, source: filepath.Join(archive.source, rel)})
		}
	}
	return tempDir, archives, discoveryErrors
}

func findArchives(startPath string) ([]string, []error) {
	paths := []string{}
	discoveryErrors := []error{}
	err := filepath.Walk(startPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			discoveryErrors = append(discoveryErrors, err)
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(info.Name(), ".tgz") || strings.HasSuffix(info.Name(), ".tar.gz") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		discoveryErrors = append(discoveryErrors, err)
	}
	return paths, discoveryErrors
}

// SearchArchivesOpenTemp retains the channel-based API. Callers must consume errs
// concurrently; new callers can use DiscoverArchives to collect errors directly.
func SearchArchivesOpenTemp(startPath string, errs chan<- error) string {
	tempDir, discoveryErrors := DiscoverArchives(startPath)
	for _, err := range discoveryErrors {
		errs <- err
	}
	return tempDir
}
