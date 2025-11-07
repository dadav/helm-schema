package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/dadav/helm-schema/pkg/chart/searching"
	"github.com/dadav/helm-schema/pkg/schema"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func exec(cmd *cobra.Command, _ []string) error {
	configureLogging()

	var skipAutoGeneration, valueFileNames []string

	chartSearchRoot := viper.GetString("chart-search-root")
	dryRun := viper.GetBool("dry-run")
	noDeps := viper.GetBool("no-dependencies")
	addSchemaReference := viper.GetBool("add-schema-reference")
	keepFullComment := viper.GetBool("keep-full-comment")
	helmDocsCompatibilityMode := viper.GetBool("helm-docs-compatibility-mode")
	uncomment := viper.GetBool("uncomment")
	outFile := viper.GetString("output-file")
	dontRemoveHelmDocsPrefix := viper.GetBool("dont-strip-helm-docs-prefix")
	appendNewline := viper.GetBool("append-newline")
	dependenciesFilter := viper.GetStringSlice("dependencies-filter")
	dependenciesFilterMap := make(map[string]bool)
	dontAddGlobal := viper.GetBool("dont-add-global")
	skipDepsSchemaValidation := viper.GetBool("skip-dependencies-schema-validation")
	for _, dep := range dependenciesFilter {
		dependenciesFilterMap[dep] = true
	}
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

	queue := make(chan string)
	resultsChan := make(chan schema.Result)
	results := []*schema.Result{}
	errs := make(chan error)
	done := make(chan struct{})

	tempDir := searching.SearchArchivesOpenTemp(chartSearchRoot, errs)
	if tempDir != "" {
		defer os.RemoveAll(tempDir)
	}

	go searching.SearchFiles(chartSearchRoot, chartSearchRoot, "Chart.yaml", dependenciesFilterMap, queue, errs)

	wg := sync.WaitGroup{}
	go func() {
		wg.Wait()
		done <- struct{}{}
	}()

	for i := 0; i < workersCount; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()
			schema.Worker(
				dryRun,
				uncomment,
				addSchemaReference,
				keepFullComment,
				helmDocsCompatibilityMode,
				dontRemoveHelmDocsPrefix,
				dontAddGlobal,
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

	if !noDeps {
		results, err = schema.TopoSort(results)
		if err != nil {
			if _, ok := err.(*schema.CircularError); !ok {
				log.Errorf("Error while sorting results: %s", err)
				return err
			} else {
				log.Warnf("Could not sort results: %s", err)
			}
		}
	}

	conditionsToPatch := make(map[string][]string)
	if !noDeps {
		for _, result := range results {
			if len(result.Errors) > 0 {
				continue
			}
			for _, dep := range result.Chart.Dependencies {
				if len(dependenciesFilterMap) > 0 && !dependenciesFilterMap[dep.Name] {
					continue
				}

				if dep.Condition != "" {
					conditionKeys := strings.Split(dep.Condition, ".")
					conditionsToPatch[conditionKeys[0]] = conditionKeys[1:]
				}
			}
		}
	}

	chartNameToResult := make(map[string]*schema.Result)
	foundErrors := false

	for _, result := range results {
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
			chartNameToResult[result.Chart.Name] = result
			log.Debugf("Stored chart %s in chartNameToResult", result.Chart.Name)

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
				if len(dependenciesFilterMap) > 0 && !dependenciesFilterMap[dep.Name] {
					continue
				}

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
						depSchema.DisableRequiredProperties()

						if dep.Alias != "" {
							result.Schema.Properties[dep.Alias] = &depSchema
						} else {
							result.Schema.Properties[dep.Name] = &depSchema
						}

					} else {
						log.Warnf("Dependency (%s->%s) specified but no schema found. If you want to create jsonschemas for external dependencies, you need to run helm dep up", result.Chart.Name, dep.Name)
					}
				} else {
					log.Warnf("Dependency without name found (checkout %s).", result.ChartPath)
				}
			}
		}

		// Handle skip-dependencies-schema-validation flag
		if skipDepsSchemaValidation && !noDeps {
			// Collect dependency names
			var depNames []string
			for _, dep := range result.Chart.Dependencies {
				if len(dependenciesFilterMap) > 0 && !dependenciesFilterMap[dep.Name] {
					continue
				}
				if dep.Alias != "" {
					depNames = append(depNames, dep.Alias)
				} else if dep.Name != "" {
					depNames = append(depNames, dep.Name)
				}
			}

			// Remove dependency names from required properties
			oldRequired := result.Schema.Required.Strings
			var newRequired []string
			for _, n := range oldRequired {
				if !slices.Contains(depNames, n) {
					newRequired = append(newRequired, n)
				}
			}
			result.Schema.Required.Strings = newRequired

			// Set additionalProperties to true for dependency schemas
			for _, depName := range depNames {
				if prop, ok := result.Schema.Properties[depName]; ok {
					prop.AdditionalProperties = true
				}
			}
		}

		jsonStr, err := result.Schema.ToJson()
		if err != nil {
			log.Error(err)
			continue
		}

		if appendNewline {
			jsonStr = append(jsonStr, '\n')
		}

		if dryRun {
			log.Infof("Printing jsonschema for %s chart (%s)", result.Chart.Name, result.ChartPath)
			if appendNewline {
				fmt.Printf("%s", jsonStr)
			} else {
				fmt.Printf("%s\n", jsonStr)
			}
		} else {
			chartBasePath := filepath.Dir(result.ChartPath)
			if err := os.WriteFile(filepath.Join(chartBasePath, outFile), jsonStr, 0644); err != nil {
				errs <- err
				continue
			}
		}
	}
	if foundErrors {
		return errors.New("some errors were found")
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
