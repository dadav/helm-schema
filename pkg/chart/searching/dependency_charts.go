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

type DiscoveredChart struct {
	Path  string
	Chart chart.ChartFile
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
	discovered := []DiscoveredChart{}
	discoveryErrors := []error{}

	err := filepath.Walk(startPath, func(path string, info os.FileInfo, walkErr error) error {
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

		isSearchRootChart := filepath.Dir(path) == chartSearchRoot
		if !isSearchRootChart && len(dependenciesFilter) > 0 && !dependenciesFilter[chartFile.Name] {
			return nil
		}

		discovered = append(discovered, DiscoveredChart{Path: path, Chart: chartFile})
		return nil
	})
	if err != nil {
		discoveryErrors = append(discoveryErrors, err)
	}

	return discovered, discoveryErrors
}

func SearchArchivesOpenTemp(startPath string, errs chan<- error) string {
	tempDir := ""
	tempDirCreationFailed := false
	err := filepath.Walk(startPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			errs <- err
			return nil
		}
		if strings.HasSuffix(info.Name(), ".tgz") || strings.HasSuffix(info.Name(), ".tar.gz") {
			// Skip extraction if temp dir creation previously failed
			if tempDirCreationFailed {
				return nil
			}
			// Extract archived charts from deps
			if tempDir == "" {
				relativeDir := filepath.Dir(path)
				var mkdirErr error
				tempDir, mkdirErr = os.MkdirTemp(relativeDir, "tmp-*")
				if mkdirErr != nil {
					errs <- fmt.Errorf("failed to create temp directory for chart extraction: %w", mkdirErr)
					tempDirCreationFailed = true
					return nil
				}
			}
			if extractErr := extractTGZ(path, tempDir); extractErr != nil {
				errs <- fmt.Errorf("failed to extract %s: %w", path, extractErr)
				return nil
			}
		}
		return nil
	})
	if err != nil {
		errs <- err
	}
	return tempDir
}
