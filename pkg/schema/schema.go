package schema

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/xeipuuv/gojsonschema"
	yaml "gopkg.in/yaml.v3"
)

const (
	SchemaPrefix  = "# @schema"
	CommentPrefix = "# "
)

const (
	nullTag      = "!!null"
	boolTag      = "!!bool"
	strTag       = "!!str"
	intTag       = "!!int"
	floatTag     = "!!float"
	timestampTag = "!!timestamp"
	arrayTag     = "!!seq"
	mapTag       = "!!map"
)

// Schema struct contains yaml tags for reading, json for writing (creating the jsonschema)
type Schema struct {
	Type                  string            `yaml:"type,omitempty"                  json:"type,omitempty"`
	Title                 string            `yaml:"title,omitempty"                 json:"title,omitempty"`
	Description           string            `yaml:"description,omitempty"           json:"description,omitempty"`
	Default               interface{}       `yaml:"default,omitempty"               json:"default,omitempty"`
	Properties            map[string]Schema `yaml:"properties,omitempty"            json:"properties,omitempty"`
	Pattern               string            `yaml:"pattern,omitempty"               json:"pattern,omitempty"`
	Format                string            `yaml:"format,omitempty"                json:"format,omitempty"`
	Required              bool              `yaml:"required,omitempty"              json:"-"`
	Deprecated            bool              `yaml:"deprecated,omitempty"            json:"deprecated,omitempty"`
	Items                 *Schema           `yaml:"items,omitempty"                 json:"items,omitempty"`
	Enum                  []string          `yaml:"enum,omitempty"                  json:"enum,omitempty"`
	Const                 string            `yaml:"const,omitempty"                 json:"const,omitempty"`
	Examples              []string          `yaml:"examples,omitempty"              json:"examples,omitempty"`
	Minimum               int               `yaml:"minimum,omitempty"               json:"minimum,omitempty"`
	Maximum               int               `yaml:"maximum,omitempty"               json:"maximum,omitempty"`
	ExclusiveMinimum      int               `yaml:"exclusiveMinimum,omitempty"      json:"exclusiveMinimum,omitempty"`
	ExclusiveMaximum      int               `yaml:"exclusiveMaximum,omitempty"      json:"exclusiveMaximum,omitempty"`
	MultipleOf            int               `yaml:"multipleOf,omitempty"            json:"multipleOf,omitempty"`
	AdditionalProperties  bool              `yaml:"additionalProperties,omitempty"  json:"additionalProperties,omitempty"`
	RequiredProperties    []string          `yaml:"-"                               json:"required,omitempty"`
	UnevaluatedProperties []string          `yaml:"unevaluatedProperties,omitempty" json:"unevaluatedProperties,omitempty"`
	AnyOf                 []Schema          `yaml:"anyOf,omitempty"                 json:"anyOf,omitempty"`
	OneOf                 []Schema          `yaml:"oneOf,omitempty"                 json:"oneOf,omitempty"`
	AllOf                 []Schema          `yaml:"allOf,omitempty"                 json:"allOf,omitempty"`
	If                    *Schema           `yaml:"if,omitempty"                    json:"if,omitempty"`
	Then                  *Schema           `yaml:"then,omitempty"                  json:"then,omitempty"`
	Else                  *Schema           `yaml:"else,omitempty"                  json:"else,omitempty"`
	HasData               bool              `yaml:"-"                               json:"-"`
	Ref                   string            `yaml:"$ref,omitempty"                  json:"$ref,omitempty"`
	Schema                string            `yaml:"$schema,omitempty"               json:"$schema,omitempty"`
	Id                    string            `yaml:"$id,omitempty"                   json:"$id,omitempty"`
}

func (s *Schema) Set() {
	s.HasData = true
}

// ToJson converts the data to raw json
func (s Schema) ToJson() ([]byte, error) {
	res, err := json.MarshalIndent(&s, "", "  ")
	if err != nil {
		return nil, err
	}
	return res, nil
}

// Validate the schema
func (s Schema) Validate() error {
	sl := gojsonschema.NewSchemaLoader()
	sl.Validate = true
	return sl.AddSchemas(gojsonschema.NewGoLoader(s))
}

func typeFromTag(tag string) (string, error) {
	switch tag {
	case nullTag:
		return "null", nil
	case boolTag:
		return "boolean", nil
	case strTag:
		return "string", nil
	case intTag:
		return "integer", nil
	case floatTag:
		return "number", nil
	case timestampTag:
		return "string", nil
	case arrayTag:
		return "array", nil
	case mapTag:
		return "object", nil
	}
	return "", errors.New(fmt.Sprintf("Unsupported yaml tag found: %s", tag))
}

// GetSchemaFromComment parses the annotations from the given comment
func GetSchemaFromComment(comment string) (Schema, string, error) {
	var result Schema
	scanner := bufio.NewScanner(strings.NewReader(comment))
	description := []string{}
	rawSchema := []string{}
	insideSchemaBlock := false

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, SchemaPrefix) {
			insideSchemaBlock = !insideSchemaBlock
			continue
		}
		if insideSchemaBlock {
			content := strings.TrimPrefix(line, CommentPrefix)
			rawSchema = append(rawSchema, strings.TrimPrefix(content, CommentPrefix))
			result.Set()
		} else {
			description = append(description, strings.TrimPrefix(line, CommentPrefix))
		}
	}

	if insideSchemaBlock {
		return result, "", errors.New(
			fmt.Sprintf("Unclosed schema block found in comment: %s", comment),
		)
	}

	err := yaml.Unmarshal([]byte(strings.Join(rawSchema, "\n")), &result)
	if err != nil {
		return result, "", err
	}

	return result, strings.Join(description, "\n"), nil
}

// YamlToSchema recursevly parses the given yaml.Node and creates a jsonschema from it
func YamlToSchema(
	node *yaml.Node,
	keepFullComment bool,
	parentRequiredProperties *[]string,
) Schema {
	var schema Schema

	requiredProperties := []string{}

	switch node.Kind {
	case yaml.DocumentNode:
		log.Debug("Inside DocumentNode")
		if len(node.Content) != 1 {
			log.Fatalf("Strange yaml document found:\n%v\n", node.Content[:])
		}

		schema.Type = "object"
		schema.Schema = "http://json-schema.org/draft-07/schema#"
		schema.Properties = YamlToSchema(
			node.Content[0],
			keepFullComment,
			&requiredProperties,
		).Properties

		if _, ok := schema.Properties["global"]; !ok {
			// global key must be present, otherwise helm lint will fail
			if schema.Properties == nil {
				schema.Properties = make(map[string]Schema)
			}
			schema.Properties["global"] = Schema{
				Type:        "object",
				Title:       "global",
				Description: "Global values are values that can be accessed from any chart or subchart by exactly the same name.",
			}
		}

		if len(requiredProperties) > 0 {
			schema.RequiredProperties = requiredProperties
		}
		// always disable on top level
		schema.AdditionalProperties = false
	case yaml.MappingNode:
		log.Debug("Inside MappingNode")
		for i := 0; i < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			valueNode := node.Content[i+1]
			log.Debugf("Checking key: %s\n", keyNode.Value)

			comment := keyNode.HeadComment
			if !keepFullComment {
				leadingCommentsRemover := regexp.MustCompile(`(?s)(?m)(?:.*\n{2,})+`)
				comment = leadingCommentsRemover.ReplaceAllString(comment, "")
			}

			valueSchema, description, err := GetSchemaFromComment(comment)
			if err != nil {
				log.Fatalf("Error while parsing comment of key %s: %v", keyNode.Value, err)
			}

			if err := valueSchema.Validate(); err != nil {
				log.Fatalf("Error while validating jsonschema of key %s: %v", keyNode.Value, err)
			}

			// No schemas specified, use the current value as type
			nodeType, err := typeFromTag(valueNode.Tag)
			if err != nil {
				log.Fatal(err)
			}

			if valueSchema.HasData {
				// Add key to required array of parent
				if valueSchema.Required {
					*parentRequiredProperties = append(*parentRequiredProperties, keyNode.Value)
				}

				// If no title was set, use the key value
				if valueSchema.Title == "" {
					valueSchema.Title = keyNode.Value
				}

				// If no description was set, use the rest of the comment as description
				if valueSchema.Description == "" {
					valueSchema.Description = description
				}

				// If no default value was set, use the values node value as default
				if valueSchema.Default == nil {
					valueSchema.Default = valueNode.Value
				}
			} else {
				valueSchema = Schema{
					Type:        nodeType,
					Title:       keyNode.Value,
					Description: description,
					Default:     valueNode.Value,
				}
			}

			// If the value is another map and no properties are set, get them from default values
			if valueNode.Kind == yaml.MappingNode && valueSchema.Properties == nil {
				valueSchema.Properties = make(map[string]Schema)
				valueSchema.Properties[keyNode.Value] = YamlToSchema(
					valueNode,
					keepFullComment,
					&requiredProperties,
				)
			} else if valueNode.Kind == yaml.SequenceNode && valueSchema.Items == nil {
				log.Debug("Value is Sequence...Checking Items")
				// If the value is a sequence, but no items are predefined
				var seqSchema Schema

				for _, itemNode := range valueNode.Content {
					seqSchema.OneOf = append(seqSchema.OneOf, YamlToSchema(itemNode, keepFullComment, &requiredProperties))
				}
				valueSchema.Items = &seqSchema
			}
			if schema.Properties == nil {
				schema.Properties = make(map[string]Schema)
			}
			schema.Properties[keyNode.Value] = valueSchema
		}
	}

	return schema
}
