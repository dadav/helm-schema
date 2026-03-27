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
	ChartPath  string
	ValuesPath string
	Chart      *chart.ChartFile
	Schema     Schema
	Errors     []error
}

// OverrideInfo tracks which subcharts have schema overrides in parent charts
type OverrideInfo struct {
	overriddenCharts map[string]bool // map of chart names that have $ref overrides
}

func NewOverrideInfo() *OverrideInfo {
	return &OverrideInfo{
		overriddenCharts: make(map[string]bool),
	}
}

func (o *OverrideInfo) MarkOverridden(chartName string) {
	o.overriddenCharts[chartName] = true
}

func (o *OverrideInfo) IsOverridden(chartName string) bool {
	return o.overriddenCharts[chartName]
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

// DetectSubchartOverrides scans a values file for subchart keys with $ref overrides
// Returns a map of subchart names that have schema overrides
func DetectSubchartOverrides(valuesPath string, dependencies []*chart.Dependency) (map[string]bool, error) {
	overrides := make(map[string]bool)

	if len(dependencies) == 0 {
		return overrides, nil
	}

	valuesFile, err := os.Open(valuesPath)
	if err != nil {
		return overrides, err
	}
	defer valuesFile.Close()

	content, err := util.ReadFileAndFixNewline(valuesFile)
	if err != nil {
		return overrides, err
	}

	var values yaml.Node
	if err := yaml.Unmarshal(content, &values); err != nil {
		return overrides, err
	}

	// Navigate to the mapping node
	if values.Kind != yaml.DocumentNode || len(values.Content) != 1 {
		return overrides, nil
	}

	mappingNode := values.Content[0]
	if mappingNode.Kind != yaml.MappingNode {
		return overrides, nil
	}

	// Create a map of dependency names and aliases for quick lookup
	depNames := make(map[string]bool)
	for _, dep := range dependencies {
		depNames[dep.Name] = true
		if dep.Alias != "" {
			depNames[dep.Alias] = true
		}
	}

	// Check each key in the values file
	for i := 0; i < len(mappingNode.Content); i += 2 {
		keyNode := mappingNode.Content[i]

		// Check if this key matches a dependency name or alias
		if !depNames[keyNode.Value] {
			continue
		}

		// Check if this key has a @schema annotation with $ref
		comment := keyNode.HeadComment
		if comment == "" {
			continue
		}

		keyNodeSchema, _, err := GetSchemaFromComment(comment)
		if err != nil {
			continue // Ignore parse errors, just skip this key
		}

		// If this dependency key has a $ref, mark it as overridden
		if keyNodeSchema.Ref != "" {
			overrides[keyNode.Value] = true
		}
	}

	return overrides, nil
}

