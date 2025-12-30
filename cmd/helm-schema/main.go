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

	"github.com/dadav/helm-schema/pkg/chart"
	"github.com/dadav/helm-schema/pkg/chart/searching"
	"github.com/dadav/helm-schema/pkg/schema"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// getDependencyNames extracts dependency names (or aliases if present) from a chart
// filtering based on the provided dependenciesFilterMap
func getDependencyNames(dependencies []*chart.Dependency, dependenciesFilterMap map[string]bool) []string {
	var depNames []string
	for _, dep := range dependencies {
		if len(dependenciesFilterMap) > 0 && !dependenciesFilterMap[dep.Name] {
			continue
		}
		if dep.Alias != "" {
			depNames = append(depNames, dep.Alias)
		} else if dep.Name != "" {
			depNames = append(depNames, dep.Name)
		}
	}
	return depNames
}

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
	allowCircularDeps := viper.GetBool("allow-circular-dependencies")
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
		close(resultsChan)
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
		case err, ok := <-errs:
			if ok {
				log.Error(err)
			}
		case res, ok := <-resultsChan:
			if ok {
				results = append(results, &res)
			}
		case <-done:
			break loop

		}
	}

	// Drain any remaining errors and results
	for {
		select {
		case err, ok := <-errs:
			if ok {
				log.Error(err)
			}
		case res, ok := <-resultsChan:
			if ok {
				results = append(results, &res)
			}
		default:
			goto drained
		}
	}
drained:

	if !noDeps {
		results, err = schema.TopoSort(results, allowCircularDeps)
		if err != nil {
			if _, ok := err.(*schema.CircularError); ok {
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

		if result.Chart == nil {
			log.Warnf("Skipping result with nil Chart at path: %s", result.ChartPath)
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

						// Check if this is a library chart
						if dependencyResult.Chart.Type == "library" {
							// For library charts, merge properties directly into parent schema
							log.Debugf("Merging library chart %s properties into parent chart %s at top level", dep.Name, result.Chart.Name)
							for propName, propSchema := range dependencyResult.Schema.Properties {
								// Skip the global property as it's already in the parent
								if propName == "global" {
									continue
								}
								// Only add if the property doesn't already exist in parent
								if _, exists := result.Schema.Properties[propName]; !exists {
									result.Schema.Properties[propName] = propSchema
								} else {
									log.Warnf("Property %s from library chart %s already exists in parent chart %s, skipping", propName, dep.Name, result.Chart.Name)
								}
							}
						} else {
							// For non-library charts, nest under dependency name
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
			// Collect dependency names using helper function
			depNames := getDependencyNames(result.Chart.Dependencies, dependenciesFilterMap)

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
					log.Debugf("Setting additionalProperties to true for dependency %s in chart %s", depName, result.Chart.Name)
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
			if err := os.WriteFile(filepath.Join(chartBasePath, outFile), jsonStr, 0o644); err != nil {
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
