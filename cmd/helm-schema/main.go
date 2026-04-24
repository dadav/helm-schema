package main

import (
	"encoding/json"
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

// mergeSchemaProperties merges properties from source to target schema.
// It skips "global" and properties in the skip map, and returns merged property names.
// Follows Helm's value coalescing behavior with one exception:
// - If target has explicit @schema annotation (HasData=true), target wins
// - If target only has inferred schema (HasData=false), source wins
func mergeSchemaProperties(
	target *schema.Schema,
	source *schema.Schema,
	skip map[string]bool,
	sourceName string,
	targetName string,
) map[string]bool {
	merged := make(map[string]bool)

	if source.Properties == nil {
		return merged
	}

	if target.Properties == nil {
		target.Properties = make(map[string]*schema.Schema)
	}

	for propName, propSchema := range source.Properties {
		if propName == "global" {
			continue
		}
		if skip != nil && skip[propName] {
			continue
		}
		existingProp, exists := target.Properties[propName]
		if !exists {
			target.Properties[propName] = propSchema
			merged[propName] = true
		} else if !existingProp.HasData && propSchema.HasData {
			// Target only has inferred schema, source has explicit annotation - source wins
			target.Properties[propName] = propSchema
			merged[propName] = true
			log.Debugf("Property %s from %s replaces inferred schema in %s", propName, sourceName, targetName)
		} else if existingProp.HasData {
			// Target has explicit @schema annotation, keep it
			log.Debugf("Property %s from %s skipped: %s has explicit @schema annotation", propName, sourceName, targetName)
		} else {
			// Both are inferred schemas, keep target (first wins)
			log.Debugf("Property %s from %s skipped: both schemas are inferred, keeping first", propName, sourceName)
		}
	}

	return merged
}

// processImportValues processes the import-values directive for a dependency.
// It returns a map of property names that were imported (to track what was handled).
func processImportValues(
	parentSchema *schema.Schema,
	depSchema *schema.Schema,
	dep *chart.Dependency,
	parentChartName string,
) map[string]bool {
	importedProps := make(map[string]bool)

	if len(dep.ImportValues) == 0 {
		return importedProps
	}

	for _, importValue := range dep.ImportValues {
		var childPath, parentPath string

		switch v := importValue.(type) {
		case string:
			// Simple form: "defaults" -> imports from exports.<value> to root
			childPath = "exports." + v
			parentPath = ""
		case map[string]interface{}:
			// Complex form: {child: "path", parent: "path"}
			if child, ok := v["child"].(string); ok {
				childPath = child
			}
			if parent, ok := v["parent"].(string); ok {
				parentPath = parent
			}
		case map[interface{}]interface{}:
			// YAML sometimes produces this type variation
			if child, ok := v["child"].(string); ok {
				childPath = child
			}
			if parent, ok := v["parent"].(string); ok {
				parentPath = parent
			}
		default:
			log.Warnf("Unknown import-values format for dependency %s in chart %s: %T", dep.Name, parentChartName, importValue)
			continue
		}

		if childPath == "" {
			log.Warnf("Empty child path in import-values for dependency %s in chart %s", dep.Name, parentChartName)
			continue
		}

		// Get the source schema from the dependency
		sourceSchema := depSchema.GetPropertyAtPath(childPath)
		if sourceSchema == nil {
			log.Warnf("Could not find path %q in dependency %s schema for chart %s", childPath, dep.Name, parentChartName)
			continue
		}

		if sourceSchema.Properties == nil {
			log.Warnf("No properties found at path %q in dependency %s for chart %s", childPath, dep.Name, parentChartName)
			continue
		}

		// Determine target schema in parent
		var targetSchema *schema.Schema
		if parentPath == "" {
			targetSchema = parentSchema
		} else {
			targetSchema = parentSchema.SetPropertyAtPath(parentPath)
		}

		merged := mergeSchemaProperties(
			targetSchema,
			sourceSchema,
			nil,
			fmt.Sprintf("import-values of %s", dep.Name),
			parentChartName,
		)
		targetPathDisplay := parentPath
		if targetPathDisplay == "" {
			targetPathDisplay = "root"
		}
		for k := range merged {
			importedProps[k] = true
			log.Debugf("Imported property %q from %s.%s to %s in chart %s",
				k, dep.Name, childPath, targetPathDisplay, parentChartName)
		}
	}

	return importedProps
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
	annotate := viper.GetBool("annotate")
	keepExistingDepSchemas := viper.GetBool("keep-existing-dep-schemas")
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
	errs := make(chan error, 100) // Buffered to prevent deadlock when errors occur before goroutines start
	done := make(chan struct{})

	tempDir := searching.SearchArchivesOpenTemp(chartSearchRoot, errs)
	if tempDir != "" {
		defer os.RemoveAll(tempDir)
	}

	go searching.SearchFiles(chartSearchRoot, chartSearchRoot, "Chart.yaml", dependenciesFilterMap, queue, errs)

	wg := sync.WaitGroup{}

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
				annotate,
				valueFileNames,
				skipConfig,
				outFile,
				queue,
				resultsChan,
			)
		}()
	}

	// Close resultsChan after all workers are done
	go func() {
		wg.Wait()
		close(resultsChan)
		close(done)
	}()

	// Collect results and errors until both channels are closed
	resultsChanOpen := true
	for resultsChanOpen {
		select {
		case err, ok := <-errs:
			if ok {
				log.Error(err)
			}
		case res, ok := <-resultsChan:
			if !ok {
				resultsChanOpen = false
			} else {
				results = append(results, &res)
			}
		}
	}

	// Drain any remaining errors
drainErrors:
	for {
		select {
		case err, ok := <-errs:
			if ok {
				log.Error(err)
			}
		default:
			break drainErrors
		}
	}

	// In annotate mode, just report errors and return (no schema generation)
	if annotate {
		foundErrors := false
		for _, result := range results {
			if len(result.Errors) > 0 {
				foundErrors = true
				if result.Chart != nil {
					log.Errorf("Found %d errors while annotating chart %s (%s)", len(result.Errors), result.Chart.Name, result.ChartPath)
				} else {
					log.Errorf("Found %d errors while annotating chart %s", len(result.Errors), result.ChartPath)
				}
				for _, err := range result.Errors {
					log.Error(err)
				}
			}
		}
		if foundErrors {
			return errors.New("some errors were found")
		}
		return nil
	}

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

	// Identify charts that are declared as dependencies of some other discovered
	// chart. Used both to skip dependency charts entirely with --no-dependencies
	// and to opt-in reuse of a dependency's pre-existing schema.
	isDependencyChart := make(map[string]bool)
	for _, result := range results {
		if result.Chart == nil || len(result.Errors) > 0 {
			continue
		}
		for _, dep := range result.Chart.Dependencies {
			isDependencyChart[dep.Name] = true
		}
	}

	// For dependency charts with pre-existing schema files, load them instead of
	// using the worker-generated schema from values.yaml. Opt-in via
	// --keep-existing-dep-schemas; default is to regenerate every discovered
	// chart's schema.
	if !noDeps && keepExistingDepSchemas {
		for _, result := range results {
			if result.Chart == nil || len(result.Errors) > 0 {
				continue
			}
			if !isDependencyChart[result.Chart.Name] {
				continue
			}
			schemaPath := filepath.Join(filepath.Dir(result.ChartPath), outFile)
			schemaData, err := os.ReadFile(schemaPath)
			if err != nil {
				continue
			}
			var existingSchema schema.Schema
			if err := json.Unmarshal(schemaData, &existingSchema); err != nil {
				log.Warnf("Found existing %s for dependency %s but failed to parse it: %s", outFile, result.Chart.Name, err)
				continue
			}
			log.Debugf("Using pre-existing schema for dependency chart %s", result.Chart.Name)
			result.Schema = existingSchema
			result.PreExistingSchema = true
		}
	}

	conditionsToPatch := make(map[string][][]string)
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
					if len(conditionKeys) == 1 {
						continue
					}
					targetName := conditionKeys[0]
					if dep.Alias != "" && dep.Alias == conditionKeys[0] {
						targetName = dep.Name
					}
					if targetName != "" {
						conditionsToPatch[targetName] = append(conditionsToPatch[targetName], conditionKeys[1:])
					}
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

		// With --no-dependencies, skip charts that are declared as dependencies of
		// some other discovered chart. Top-level charts are still processed.
		if noDeps && isDependencyChart[result.Chart.Name] {
			log.Debugf("Skipping dependency chart %s (--no-dependencies)", result.Chart.Name)
			continue
		}

		log.Debugf("Processing result for chart: %s (%s)", result.Chart.Name, result.ChartPath)
		if !noDeps {
			chartNameToResult[result.Chart.Name] = result
			log.Debugf("Stored chart %s in chartNameToResult", result.Chart.Name)

			if patches, ok := conditionsToPatch[result.Chart.Name]; ok {
				for _, patch := range patches {
					schemaToPatch := &result.Schema
					lastIndex := len(patch) - 1
					for i, key := range patch {
						// Ensure Properties map is initialized
						if schemaToPatch.Properties == nil {
							schemaToPatch.Properties = make(map[string]*schema.Schema)
						}
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
								schemaToPatch.Properties[key] = &schema.Schema{
									Type:       []string{"object"},
									Title:      key,
									Properties: make(map[string]*schema.Schema),
								}
								schemaToPatch = schemaToPatch.Properties[key]
							}
						} else {
							schemaToPatch = alreadyPresentSchema
						}
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

						// Process import-values first (before regular dependency nesting)
						importedProps := processImportValues(
							&result.Schema,
							&dependencyResult.Schema,
							dep,
							result.Chart.Name,
						)
						hasImportValues := len(dep.ImportValues) > 0

						// Check if this is a library chart
						if dependencyResult.Chart.Type == "library" {
							// For library charts, merge properties directly into parent schema
							log.Debugf("Merging library chart %s properties into parent chart %s at top level", dep.Name, result.Chart.Name)
							mergeSchemaProperties(
								&result.Schema,
								&dependencyResult.Schema,
								importedProps,
								fmt.Sprintf("library chart %s", dep.Name),
								fmt.Sprintf("parent chart %s", result.Chart.Name),
							)
						} else if !hasImportValues {
							// For non-library charts WITHOUT import-values, nest under dependency name
							// (If import-values is used, user explicitly controls what's imported)
							depSchema := schema.Schema{
								Type:        []string{"object"},
								Title:       dep.Name,
								Description: dependencyResult.Chart.Description,
								Properties:  dependencyResult.Schema.Properties,
							}
							if dep.Condition != "" && !strings.Contains(dep.Condition, ".") {
								depSchema.Type = []string{"object", "boolean"}
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
				if prop, ok := result.Schema.Properties[depName]; ok && prop != nil {
					log.Debugf("Setting additionalProperties to true for dependency %s in chart %s", depName, result.Chart.Name)
					additionalPropsTrue := true
					prop.AdditionalProperties = &additionalPropsTrue
				}
			}
		}

		// Hoist all nested definitions to the root level so $ref pointers resolve correctly
		result.Schema.HoistDefinitions()

		// Skip writing output for dependency charts with pre-existing schema files
		if result.PreExistingSchema {
			log.Debugf("Skipping output for dependency chart %s: using pre-existing schema", result.Chart.Name)
			continue
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
