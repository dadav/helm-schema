package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/dadav/helm-schema/pkg/chart"
	"github.com/dadav/helm-schema/pkg/schema"
	"github.com/dadav/helm-schema/pkg/util"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	yaml "gopkg.in/yaml.v3"
)

func searchFiles(startPath, fileName string, queue chan<- string, errs chan<- error) {
	defer close(queue)
	err := filepath.Walk(startPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			errs <- err
			return nil
		}

		if !info.IsDir() && info.Name() == fileName {
			queue <- path
		}

		return nil
	})

	if err != nil {
		errs <- err
	}
}

func worker(
	dryRun, skipDeps, keepFullComment bool,
	valueFileNames []string,
	outFile string,
	queue <-chan string,
	errs chan<- error,
) {
	for chartPath := range queue {
		chartBasePath := filepath.Dir(chartPath)
		chart, err := chart.ReadChartFile(chartPath)
		if err != nil {
			errs <- err
			continue
		}

		var valuesPath string
		var valuesFound bool

		for _, possibleValueFileName := range valueFileNames {
			valuesPath = filepath.Join(chartBasePath, possibleValueFileName)
			_, err := os.Stat(valuesPath)
			if err != nil {
				if !errors.Is(os.ErrNotExist, err) {
					errs <- err
				}
				continue
			}
			valuesFound = true
			break
		}

		if !valuesFound {
			continue
		}

		content, err := util.ReadYamlFile(valuesPath)
		if err != nil {
			errs <- err
			continue
		}

		var values yaml.Node
		err = yaml.Unmarshal(content, &values)
		if err != nil {
			log.Fatal(err)
		}

		schema := schema.YamlToJsonSchema(&values, keepFullComment, nil)

		if !skipDeps {
			for _, dep := range chart.Dependencies {
				if depName, ok := dep["name"]; ok {
					schema["properties"].(map[string]interface{})[depName] = map[string]string{
						"title":       chart.Name,
						"description": chart.Description,
						"$ref":        fmt.Sprintf("charts/%s/%s", depName, outFile),
					}
				}
			}
		}

		jsonStr, err := json.MarshalIndent(schema, "", "  ")

		if dryRun {
			log.Infof("Printing jsonschema for %s chart", chart.Name)
			fmt.Printf("%s\n", jsonStr)
		} else {
			if err := os.WriteFile(filepath.Join(chartBasePath, outFile), jsonStr, 0644); err != nil {
				errs <- err
				continue
			}
		}

	}
}

func exec(_ *cobra.Command, _ []string) {
	configureLogging()

	chartSearchRoot := viper.GetString("chart-search-root")
	dryRun := viper.GetBool("dry-run")
	noDeps := viper.GetBool("no-dependencies")
	keepFullComment := viper.GetBool("keep-full-comment")
	outFile := viper.GetString("output-file")
	valueFileNames := viper.GetStringSlice("value-files")

	workersCount := 1
	if !dryRun {
		workersCount = runtime.NumCPU()
	}

	// 1. Start a producer that searches Chart.yaml and values.yaml files
	queue := make(chan string)
	errs := make(chan error)
	done := make(chan struct{})

	go searchFiles(chartSearchRoot, "Chart.yaml", queue, errs)

	// 2. Start workers and every worker does:
	wg := sync.WaitGroup{}
	go func() {
		wg.Wait()
		done <- struct{}{}
	}()

	for i := 0; i < workersCount; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()
			worker(dryRun, noDeps, keepFullComment, valueFileNames, outFile, queue, errs)
		}()
	}

loop:
	for {
		select {
		case err := <-errs:
			log.Error(err)
		case <-done:
			break loop

		}
	}

}

func main() {
	command, err := newCommand(exec)
	if err != nil {
		log.Errorf("Failed to create the CLI commander: %s", err)
		os.Exit(1)
	}

	if err := command.Execute(); err != nil {
		log.Errorf("Failed to start the CLI: %s", err)
		os.Exit(1)
	}
}
