package searching

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"github.com/dadav/helm-schema/pkg/chart"
	"gopkg.in/yaml.v3"
	"io"
	"os"
	"path/filepath"
	"strings"
)

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

		// Resolve file path
		target := filepath.Join(dest, header.Name)

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
			defer outFile.Close()

			// Copy file content
			if _, err := io.Copy(outFile, tr); err != nil {
				return err
			}
		}
	}
	return nil
}

func SearchFiles(chartSearchRoot, startPath, fileName string, dependenciesFilter map[string]bool, queue chan<- string, errs chan<- error) {
	defer close(queue)
	err := filepath.Walk(startPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			errs <- err
			return nil
		}

		if !info.IsDir() && info.Name() == fileName {
			if filepath.Dir(path) == chartSearchRoot {
				queue <- path
				return nil
			}

			if len(dependenciesFilter) > 0 {
				chartData, err := os.ReadFile(path)
				if err != nil {
					errs <- fmt.Errorf("failed to read Chart.yaml at %s: %w", path, err)
					return nil
				}

				var chartFile chart.ChartFile
				if err := yaml.Unmarshal(chartData, &chartFile); err != nil {
					errs <- fmt.Errorf("failed to parse Chart.yaml at %s: %w", path, err)
					return nil
				}

				if dependenciesFilter[chartFile.Name] {
					queue <- path
				}
			} else {
				queue <- path
			}
		}

		return nil
	})
	if err != nil {
		errs <- err
	}
}

func SearchArchivesOpenTemp(startPath string, errs chan<- error) string {
	tempDir := ""
	err := filepath.Walk(startPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			errs <- err
			return nil
		}
		if strings.Contains(info.Name(), "tgz") {
			//unzip archived charts from deps
			if tempDir == "" {
				relativeDir := filepath.Dir(path)
				tempDir, err = os.MkdirTemp(filepath.Join(relativeDir), "tmp")
				if err != nil {
					errs <- err
					return nil
				}
			}
			err = extractTGZ(path, tempDir)
			if err != nil {
				errs <- err
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
