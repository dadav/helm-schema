package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	yaml "gopkg.in/yaml.v3"

	"github.com/dadav/helm-schema/pkg/chart"
	"github.com/dadav/helm-schema/pkg/schema"
	"github.com/dadav/helm-schema/pkg/util"
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

type Result struct {
	ChartPath  string
	ValuesPath string
	Chart      *chart.ChartFile
	Schema     schema.Schema
	Errors     []error
}

func worker(
	dryRun, uncomment, addSchemaReference, keepFullComment, dontRemoveHelmDocsPrefix bool,
	valueFileNames []string,
	skipAutoGenerationConfig *schema.SkipAutoGenerationConfig,
	outFile string,
	queue <-chan string,
	results chan<- Result,
) {
	for chartPath := range queue {
		result := Result{ChartPath: chartPath}

		chartBasePath := filepath.Dir(chartPath)
		file, err := os.Open(chartPath)
		if err != nil {
			result.Errors = append(result.Errors, err)
			results <- result
			continue
		}

		chart, err := chart.ReadChart(file)
		if err != nil {
			result.Errors = append(result.Errors, err)
			results <- result
			continue
		}
		result.Chart = &chart

		var valuesPath string
		var valuesFound bool
		errorsWeMaybeCanIgnore := []error{}

		for _, possibleValueFileName := range valueFileNames {
			valuesPath = filepath.Join(chartBasePath, possibleValueFileName)
			_, err := os.Stat(valuesPath)
			if err != nil {
				if !os.IsNotExist(err) {
					errorsWeMaybeCanIgnore = append(errorsWeMaybeCanIgnore, err)
				}
				continue
			}
			valuesFound = true
			break
		}

		if !valuesFound {
			for _, err := range errorsWeMaybeCanIgnore {
				result.Errors = append(result.Errors, err)
			}
			result.Errors = append(result.Errors, errors.New("No values file found."))
			results <- result
			continue
		}
		result.ValuesPath = valuesPath

		valuesFile, err := os.Open(valuesPath)
		if err != nil {
			result.Errors = append(result.Errors, err)
			results <- result
			continue
		}
		content, err := util.ReadFileAndFixNewline(valuesFile)
		if err != nil {
			result.Errors = append(result.Errors, err)
			results <- result
			continue
		}

		// Check if we need to add a schema reference
		if addSchemaReference {
			schemaRef := `# yaml-language-server: $schema=values.schema.json`
			if !strings.Contains(string(content), schemaRef) {
				err = util.InsertLineToFile(schemaRef, valuesPath)
				if err != nil {
					result.Errors = append(result.Errors, err)
					results <- result
					continue
				}
			}
		}

		// Optional preprocessing
		if uncomment {
			// Remove comments from valid yaml
			content, err = util.RemoveCommentsFromYaml(bytes.NewReader(content))
			if err != nil {
				result.Errors = append(result.Errors, err)
				results <- result
				continue
			}
		}

		var values yaml.Node
		err = yaml.Unmarshal(content, &values)
		if err != nil {
			result.Errors = append(result.Errors, err)
			results <- result
			continue
		}

		result.Schema = schema.YamlToSchema(&values, keepFullComment, dontRemoveHelmDocsPrefix, skipAutoGenerationConfig, nil)

		results <- result
	}
}

func exec(cmd *cobra.Command, _ []string) error {
	configureLogging()

	var skipAutoGeneration, valueFileNames []string

	chartSearchRoot := viper.GetString("chart-search-root")
	dryRun := viper.GetBool("dry-run")
	noDeps := viper.GetBool("no-dependencies")
	addSchemaReference := viper.GetBool("add-schema-reference")
	keepFullComment := viper.GetBool("keep-full-comment")
	uncomment := viper.GetBool("uncomment")
	outFile := viper.GetString("output-file")
	dontRemoveHelmDocsPrefix := viper.GetBool("dont-strip-helm-docs-prefix")
	if err := viper.UnmarshalKey("value-files", &valueFileNames); err != nil {
		return err
	}
	if err := viper.UnmarshalKey("skip-auto-generation", &skipAutoGeneration); err != nil {
		return err
	}
	workersCount := runtime.NumCPU() * 2

	skipConfig, err := schema.NewSkipAutoGenerationConfig(skipAutoGeneration)
	if err != nil {
		return err
	}

	// 1. Start a producer that searches Chart.yaml and values.yaml files
	queue := make(chan string)
	resultsChan := make(chan Result)
	results := []*Result{}
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
			worker(
				dryRun,
				uncomment,
				addSchemaReference,
				keepFullComment,
				dontRemoveHelmDocsPrefix,
				valueFileNames,
				skipConfig,
				outFile,
				queue,
				resultsChan,
			)
		}()
	}

loop:
	for {
		select {
		case err := <-errs:
			log.Error(err)
		case res := <-resultsChan:
			results = append(results, &res)
		case <-done:
			break loop

		}
	}

	// sort results with topology sort (only if we're checking the dependencies)
	if !noDeps {
		results, err = util.TopSort[*Result, string](results, func(i *Result) string {
			return i.Chart.Name
		},
			func(d *Result) []string {
				deps := []string{}
				if d.Chart.Dependencies != nil {
					for _, dep := range d.Chart.Dependencies {
						deps = append(deps, dep.Name)
					}
				}
				return deps
			},
		)
		if err != nil {
			if _, ok := err.(*util.CircularError); !ok {
				log.Errorf("Error while sorting results: %s", err)
				return err
			} else {
				log.Warnf("Could not sort results: %s", err)
			}
		}
	}

	conditionsToPatch := make(map[string][]string)
	// Sort results if dependencies should be processed
	// Need to resolve the dependencies from deepest level to highest

	if !noDeps {
		// Iterate over deps to find conditions we need to patch (dependencies that have a condition)
		for _, result := range results {
			if len(result.Errors) > 0 {
				continue
			}
			for _, dep := range result.Chart.Dependencies {
				if dep.Condition != "" {
					conditionKeys := strings.Split(dep.Condition, ".")
					conditionsToPatch[conditionKeys[0]] = conditionKeys[1:]
				}
			}
		}
	}

	chartNameToResult := make(map[string]*Result)
	foundErrors := false

	// process results
	for _, result := range results {
		// Error handling
		if len(result.Errors) > 0 {
			foundErrors = true
			if result.Chart != nil {
				log.Errorf(
					"Found %d errors while processing the chart %s (%s)",
					len(result.Errors),
					result.Chart.Name,
					result.ChartPath,
				)
			} else {
				log.Errorf("Found %d errors while processing the chart %s", len(result.Errors), result.ChartPath)
			}
			for _, err := range result.Errors {
				log.Error(err)
			}
			continue
		}

		log.Debugf("Processing result for chart: %s (%s)", result.Chart.Name, result.ChartPath)
		if !noDeps {
			// Patch condition into schema if needed
			if patch, ok := conditionsToPatch[result.Chart.Name]; ok {
				schemaToPatch := &result.Schema
				lastIndex := len(patch) - 1
				for i, key := range patch {
					if alreadyPresentSchema, ok := schemaToPatch.Properties[key]; !ok {
						log.Debugf(
							"Patching conditional field \"%s\" into schema of chart %s",
							key,
							result.Chart.Name,
						)
						if i == lastIndex {
							schemaToPatch.Properties[key] = &schema.Schema{
								Type:        []string{"boolean"},
								Title:       key,
								Description: "Conditional property used in parent chart",
							}
						} else {
							schemaToPatch.Properties[key] = &schema.Schema{Type: []string{"object"}, Title: key}
							schemaToPatch = schemaToPatch.Properties[key]
						}
					} else {
						schemaToPatch = alreadyPresentSchema
					}
				}
			}

			for _, dep := range result.Chart.Dependencies {
				if dep.Name != "" {
					if dependencyResult, ok := chartNameToResult[dep.Name]; ok {
						log.Debugf(
							"Found chart of dependency %s (%s)",
							dependencyResult.Chart.Name,
							dependencyResult.ChartPath,
						)
						depSchema := schema.Schema{
							Type:        []string{"object"},
							Title:       dep.Name,
							Description: dependencyResult.Chart.Description,
							Properties:  dependencyResult.Schema.Properties,
						}
						// you don't NEED to overwrite the values
						// so every required check will be disabled
						depSchema.DisableRequiredProperties()

						result.Schema.Properties[dep.Name] = &depSchema

					} else {
						log.Warnf("Dependency (%s->%s) specified but no schema found. If you want to create jsonschemas for external dependencies, you need to run helm dependency build & untar the charts.", result.Chart.Name, dep.Name)
					}
				} else {
					log.Warnf("Dependency without name found (checkout %s).", result.ChartPath)
				}
			}
			chartNameToResult[result.Chart.Name] = result
		}

		// Print to stdout or write to file
		jsonStr, err := result.Schema.ToJson()
		if err != nil {
			log.Error(err)
			continue
		}

		if dryRun {
			log.Infof("Printing jsonschema for %s chart (%s)", result.Chart.Name, result.ChartPath)
			fmt.Printf("%s\n", jsonStr)
		} else {
			chartBasePath := filepath.Dir(result.ChartPath)
			if err := os.WriteFile(filepath.Join(chartBasePath, outFile), jsonStr, 0644); err != nil {
				errs <- err
				continue
			}
		}
	}
	if foundErrors {
		return errors.New("Some errors were found")
	}
	return nil
}

func main() {
	command, err := newCommand(exec)
	if err != nil {
		log.Errorf("Failed to create the CLI commander: %s", err)
		os.Exit(1)
	}

	if err := command.Execute(); err != nil {
		log.Errorf("Execution error: %s", err)
		os.Exit(1)
	}
}
