package schema

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dadav/helm-schema/pkg/chart"
	"github.com/dadav/helm-schema/pkg/util"
	"gopkg.in/yaml.v3"
)

type Result struct {
	ChartPath         string
	ValuesPath        string
	Chart             *chart.ChartFile
	Schema            Schema
	Errors            []error
	PreExistingSchema bool
}

func Worker(
	dryRun, uncomment, addSchemaReference, keepFullComment, helmDocsCompatibilityMode, dontRemoveHelmDocsPrefix, dontAddGlobal, annotate bool,
	valueFileNames []string,
	skipAutoGenerationConfig *SkipAutoGenerationConfig,
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
		file.Close()
		if err != nil {
			result.Errors = append(result.Errors, err)
			results <- result
			continue
		}
		result.Chart = &chart

		var valuesPath string
		valuesPaths := []string{}
		errorsWeMaybeCanIgnore := []error{}

		for _, possibleValueFileName := range valueFileNames {
			candidatePath := filepath.Join(chartBasePath, possibleValueFileName)
			_, err := os.Stat(candidatePath)
			if err != nil {
				if !os.IsNotExist(err) {
					errorsWeMaybeCanIgnore = append(errorsWeMaybeCanIgnore, err)
				}
				continue
			}
			valuesPaths = append(valuesPaths, candidatePath)
		}

		if len(valuesPaths) == 0 {
			result.Errors = append(result.Errors, errorsWeMaybeCanIgnore...)
			result.Errors = append(result.Errors, fmt.Errorf("no values file found (tried: %s)", strings.Join(valueFileNames, ", ")))
			results <- result
			continue
		}
		valuesPath = valuesPaths[0]
		result.ValuesPath = valuesPath

		// Annotate mode: write @schema annotations into values.yaml and skip schema generation
		if annotate {
			if err := AnnotateValuesFile(valuesPath, dryRun); err != nil {
				result.Errors = append(result.Errors, err)
			}
			results <- result
			continue
		}

		valuesFile, err := os.Open(valuesPath)
		if err != nil {
			result.Errors = append(result.Errors, err)
			results <- result
			continue
		}
		content, err := util.ReadFileAndFixNewline(valuesFile)
		valuesFile.Close()
		if err != nil {
			result.Errors = append(result.Errors, err)
			results <- result
			continue
		}

		// Check if we need to add a schema reference
		if addSchemaReference && !dryRun {
			schemaRef := `# yaml-language-server: $schema=values.schema.json`
			if !strings.Contains(string(content), schemaRef) {
				err = util.PrefixFirstYamlDocument(schemaRef, valuesPath)
				if err != nil {
					result.Errors = append(result.Errors, err)
					results <- result
					continue
				}
			}
		}

		var mergedValues *yaml.Node
		for _, currentValuesPath := range valuesPaths {
			valuesFile, err := os.Open(currentValuesPath)
			if err != nil {
				result.Errors = append(result.Errors, err)
				break
			}

			currentContent, err := util.ReadFileAndFixNewline(valuesFile)
			valuesFile.Close()
			if err != nil {
				result.Errors = append(result.Errors, err)
				break
			}

			if uncomment {
				// Remove comments from valid yaml before parsing.
				currentContent, err = util.RemoveCommentsFromYaml(bytes.NewReader(currentContent))
				if err != nil {
					result.Errors = append(result.Errors, err)
					break
				}
			}

			var currentValues yaml.Node
			err = yaml.Unmarshal(currentContent, &currentValues)
			if err != nil {
				result.Errors = append(result.Errors, err)
				break
			}

			mergedValues, err = mergeValuesDocuments(mergedValues, &currentValues)
			if err != nil {
				result.Errors = append(result.Errors, err)
				break
			}
		}
		if len(result.Errors) > 0 {
			results <- result
			continue
		}

		schema, err := YamlToSchema(valuesPath, mergedValues, keepFullComment, helmDocsCompatibilityMode, dontRemoveHelmDocsPrefix, dontAddGlobal, skipAutoGenerationConfig, nil)
		if err != nil {
			result.Errors = append(result.Errors, err)
			results <- result
			continue
		}
		result.Schema = *schema

		results <- result
	}
}
