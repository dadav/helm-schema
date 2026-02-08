package schema

import (
	"fmt"
	"os"
	"sort"
	"strings"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// HasSchemaAnnotation checks if a HeadComment already contains a # @schema block.
// It matches exact "# @schema" lines but not "# @schema.root" lines.
func HasSchemaAnnotation(comment string) bool {
	for _, line := range strings.Split(comment, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "# @schema" {
			return true
		}
	}
	return false
}

// typeAnnotationFromTag maps a YAML tag to the annotation type string.
// Uses the same mapping as typeFromTag.
func typeAnnotationFromTag(tag string) string {
	switch tag {
	case nullTag:
		return `"null"`
	case boolTag:
		return "boolean"
	case strTag:
		return "string"
	case intTag:
		return "integer"
	case floatTag:
		return "number"
	case timestampTag:
		return "string"
	case arrayTag:
		return "array"
	case mapTag:
		return "object"
	default:
		return ""
	}
}

// InsertionPoint represents where to insert an annotation block in the file.
type InsertionPoint struct {
	Line    int    // 1-based line number of the key node
	Indent  string // indentation string (spaces) derived from keyNode.Column
	TypeStr string // type annotation value
}

// collectInsertionPoints walks the yaml.Node tree and collects InsertionPoints
// for keys that don't already have @schema annotations.
func collectInsertionPoints(node *yaml.Node) []InsertionPoint {
	var points []InsertionPoint
	collectInsertionPointsRecursive(node, &points)
	return points
}

func collectInsertionPointsRecursive(node *yaml.Node, points *[]InsertionPoint) {
	if node == nil {
		return
	}

	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			collectInsertionPointsRecursive(child, points)
		}
	case yaml.MappingNode:
		for i := 0; i < len(node.Content)-1; i += 2 {
			keyNode := node.Content[i]
			valueNode := node.Content[i+1]

			if !HasSchemaAnnotation(keyNode.HeadComment) {
				typeStr := typeAnnotationFromTag(valueNode.Tag)
				if typeStr != "" {
					indent := strings.Repeat(" ", keyNode.Column-1)
					*points = append(*points, InsertionPoint{
						Line:    keyNode.Line,
						Indent:  indent,
						TypeStr: typeStr,
					})
				}
			}

			// Recurse into mapping values for nested keys
			if valueNode.Kind == yaml.MappingNode {
				collectInsertionPointsRecursive(valueNode, points)
			}

			// Handle alias nodes that point to mappings
			if valueNode.Kind == yaml.AliasNode && valueNode.Alias != nil && valueNode.Alias.Kind == yaml.MappingNode {
				collectInsertionPointsRecursive(valueNode.Alias, points)
			}
		}
	}
}

// AnnotateContent parses YAML content, collects insertion points for keys
// that lack @schema annotations, and inserts type annotation blocks.
// Returns the modified content.
func AnnotateContent(content []byte) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(content, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	points := collectInsertionPoints(&doc)
	if len(points) == 0 {
		return content, nil
	}

	lines := strings.Split(string(content), "\n")

	// Sort by line number descending so insertions don't shift earlier line numbers
	sort.Slice(points, func(i, j int) bool {
		return points[i].Line > points[j].Line
	})

	for _, pt := range points {
		// pt.Line is 1-based; convert to 0-based index
		targetIdx := pt.Line - 1
		if targetIdx < 0 || targetIdx >= len(lines) {
			continue
		}

		// Walk backwards past any existing HeadComment lines (lines starting with #)
		// to insert the annotation above existing comments
		insertIdx := targetIdx
		for insertIdx > 0 {
			candidate := strings.TrimSpace(lines[insertIdx-1])
			if strings.HasPrefix(candidate, "#") {
				insertIdx--
			} else {
				break
			}
		}

		// Build the 3 annotation lines
		annotationLines := []string{
			pt.Indent + "# @schema",
			pt.Indent + "# type: " + pt.TypeStr,
			pt.Indent + "# @schema",
		}

		// Insert at insertIdx
		newLines := make([]string, 0, len(lines)+3)
		newLines = append(newLines, lines[:insertIdx]...)
		newLines = append(newLines, annotationLines...)
		newLines = append(newLines, lines[insertIdx:]...)
		lines = newLines
	}

	return []byte(strings.Join(lines, "\n")), nil
}

// AnnotateValuesFile reads a values.yaml file, annotates unannotated keys
// with @schema type blocks, and writes the result back (or prints to stdout if dryRun).
func AnnotateValuesFile(valuesPath string, dryRun bool) error {
	fileInfo, err := os.Stat(valuesPath)
	if err != nil {
		return fmt.Errorf("failed to stat %s: %w", valuesPath, err)
	}
	perm := fileInfo.Mode().Perm()

	content, err := os.ReadFile(valuesPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", valuesPath, err)
	}

	annotated, err := AnnotateContent(content)
	if err != nil {
		return fmt.Errorf("failed to annotate %s: %w", valuesPath, err)
	}

	if dryRun {
		log.Infof("Annotated values for %s", valuesPath)
		fmt.Print(string(annotated))
		return nil
	}

	if err := os.WriteFile(valuesPath, annotated, perm); err != nil {
		return fmt.Errorf("failed to write %s: %w", valuesPath, err)
	}

	log.Infof("Annotated %s", valuesPath)
	return nil
}
