package schema

import (
	"bytes"
	"errors"
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

func Worker(
	dryRun, uncomment, addSchemaReference, keepFullComment, dontRemoveHelmDocsPrefix bool,
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
			result.Errors = append(result.Errors, errorsWeMaybeCanIgnore...)
			result.Errors = append(result.Errors, errors.New("no values file found"))
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
				err = util.PrefixFirstYamlDocument(schemaRef, valuesPath)
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

		result.Schema = *YamlToSchema(valuesPath, &values, keepFullComment, dontRemoveHelmDocsPrefix, skipAutoGenerationConfig, nil)

		results <- result
	}
}
