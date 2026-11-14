package schema

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dadav/helm-schema/pkg/chart"
	"github.com/dadav/helm-schema/pkg/util"
	log "github.com/sirupsen/logrus"
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

type WorkerOptions struct {
	DryRun                    bool
	Uncomment                 bool
	AddSchemaReference        bool
	KeepFullComment           bool
	HelmDocsCompatibilityMode bool
	DontRemoveHelmDocsPrefix  bool
	DontAddGlobal             bool
	Annotate                  bool
	ValueFileNames            []string
	SkipAutoGenerationConfig  *SkipAutoGenerationConfig
	OutFile                   string
}

func Worker(options WorkerOptions, queue <-chan string, results chan<- Result) {
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
		statErrors := []error{}

		for _, possibleValueFileName := range options.ValueFileNames {
			candidatePath := filepath.Join(chartBasePath, possibleValueFileName)
			_, err := os.Stat(candidatePath)
			if err != nil {
				if !os.IsNotExist(err) {
					statErrors = append(statErrors, err)
				}
				continue
			}
			valuesPaths = append(valuesPaths, candidatePath)
		}

		if len(statErrors) > 0 {
			result.Errors = append(result.Errors, statErrors...)
			results <- result
			continue
		}
		if len(valuesPaths) == 0 {
			entry := log.WithFields(log.Fields{"chart_path": chartPath, "tried_names": options.ValueFileNames})
			if options.Annotate || options.AddSchemaReference {
				entry.Warn("No values file found; skipping chart")
				continue
			}
			entry.Warn("No values file found; generating schema from empty values")
		} else {
			valuesPath = valuesPaths[0]
			result.ValuesPath = valuesPath
		}

		// Annotate mode: write @schema annotations into values.yaml and skip schema generation
		if options.Annotate {
			if err := AnnotateValuesFile(valuesPath, options.DryRun); err != nil {
				result.Errors = append(result.Errors, err)
			}
			results <- result
			continue
		}

		if options.AddSchemaReference && !options.DryRun {
			content, err := os.ReadFile(valuesPath)
			if err != nil {
				result.Errors = append(result.Errors, err)
				results <- result
				continue
			}
			if !hasSchemaReference(string(content)) {
				schemaPath := filepath.Join(chartBasePath, options.OutFile)
				relativePath, err := filepath.Rel(filepath.Dir(valuesPath), schemaPath)
				if err != nil {
					result.Errors = append(result.Errors, fmt.Errorf("failed to make schema path %s relative to %s: %w", schemaPath, valuesPath, err))
					results <- result
					continue
				}
				schemaRef := "# yaml-language-server: $schema=" + filepath.ToSlash(relativePath)
				if err := util.PrefixFirstYamlDocument(schemaRef, valuesPath); err != nil {
					result.Errors = append(result.Errors, err)
					results <- result
					continue
				}
			}
		}

		mergedValues := normalizeEmptyValuesDocument(&yaml.Node{})
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

			if options.Uncomment {
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

		schema, err := YamlToSchema(valuesPath, mergedValues, options.KeepFullComment, options.HelmDocsCompatibilityMode, options.DontRemoveHelmDocsPrefix, options.DontAddGlobal, options.SkipAutoGenerationConfig, nil)
		if err != nil {
			result.Errors = append(result.Errors, err)
			results <- result
			continue
		}
		result.Schema = *schema

		results <- result
	}
}

func hasSchemaReference(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		comment, ok := strings.CutPrefix(strings.TrimSpace(line), "#")
		if !ok {
			continue
		}
		directive, ok := strings.CutPrefix(strings.TrimSpace(comment), "yaml-language-server:")
		if ok && strings.Contains(directive, "$schema=") {
			return true
		}
	}
	return false
}
