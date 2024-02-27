package schema

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v5"
	log "github.com/sirupsen/logrus"
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

type SchemaOrBool interface{}

// Schema struct contains yaml tags for reading, json for writing (creating the jsonschema)
type Schema struct {
	AdditionalProperties SchemaOrBool       `yaml:"additionalProperties,omitempty" json:"additionalProperties,omitempty"`
	Default              interface{}        `yaml:"default,omitempty"              json:"default,omitempty"`
	Then                 *Schema            `yaml:"then,omitempty"                 json:"then,omitempty"`
	PatternProperties    map[string]*Schema `yaml:"patternProperties,omitempty"    json:"patternProperties,omitempty"`
	Properties           map[string]*Schema `yaml:"properties,omitempty"           json:"properties,omitempty"`
	If                   *Schema            `yaml:"if,omitempty"                   json:"if,omitempty"`
	Minimum              *int               `yaml:"minimum,omitempty"              json:"minimum,omitempty"`
	MultipleOf           *int               `yaml:"multipleOf,omitempty"           json:"multipleOf,omitempty"`
	ExclusiveMaximum     *int               `yaml:"exclusiveMaximum,omitempty"     json:"exclusiveMaximum,omitempty"`
	Items                *Schema            `yaml:"items,omitempty"                json:"items,omitempty"`
	ExclusiveMinimum     *int               `yaml:"exclusiveMinimum,omitempty"     json:"exclusiveMinimum,omitempty"`
	Maximum              *int               `yaml:"maximum,omitempty"              json:"maximum,omitempty"`
	Else                 *Schema            `yaml:"else,omitempty"                 json:"else,omitempty"`
	Pattern              string             `yaml:"pattern,omitempty"              json:"pattern,omitempty"`
	Const                string             `yaml:"const,omitempty"                json:"const,omitempty"`
	Ref                  string             `yaml:"$ref,omitempty"                 json:"$ref,omitempty"`
	Schema               string             `yaml:"$schema,omitempty"              json:"$schema,omitempty"`
	Id                   string             `yaml:"$id,omitempty"                  json:"$id,omitempty"`
	Format               string             `yaml:"format,omitempty"               json:"format,omitempty"`
	Description          string             `yaml:"description,omitempty"          json:"description,omitempty"`
	Title                string             `yaml:"title,omitempty"                json:"title,omitempty"`
	Type                 string             `yaml:"type,omitempty"                 json:"type,omitempty"`
	AnyOf                []*Schema          `yaml:"anyOf,omitempty"                json:"anyOf,omitempty"`
	AllOf                []*Schema          `yaml:"allOf,omitempty"                json:"allOf,omitempty"`
	OneOf                []*Schema          `yaml:"oneOf,omitempty"                json:"oneOf,omitempty"`
	RequiredProperties   []string           `yaml:"-"                              json:"required,omitempty"`
	Examples             []string           `yaml:"examples,omitempty"             json:"examples,omitempty"`
	Enum                 []string           `yaml:"enum,omitempty"                 json:"enum,omitempty"`
	HasData              bool               `yaml:"-"                              json:"-"`
	Deprecated           bool               `yaml:"deprecated,omitempty"           json:"deprecated,omitempty"`
	Required             bool               `yaml:"required,omitempty"             json:"-"`
}

// Set sets the HasData field to true
func (s *Schema) Set() {
	s.HasData = true
}

// DisableRequiredProperties sets all RequiredProperties in this schema to an empty slice
func (s *Schema) DisableRequiredProperties() {
	s.RequiredProperties = nil
	for _, v := range s.Properties {
		v.DisableRequiredProperties()
	}
	if s.Items != nil {
		s.Items.DisableRequiredProperties()
	}

	if s.AnyOf != nil {
		for _, v := range s.AnyOf {
			v.DisableRequiredProperties()
		}
	}
	if s.OneOf != nil {
		for _, v := range s.OneOf {
			v.DisableRequiredProperties()
		}
	}
	if s.AllOf != nil {
		for _, v := range s.AllOf {
			v.DisableRequiredProperties()
		}
	}
	if s.If != nil {
		s.If.DisableRequiredProperties()
	}
	if s.Else != nil {
		s.Else.DisableRequiredProperties()
	}
	if s.Then != nil {
		s.Then.DisableRequiredProperties()
	}
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
	jsonStr, err := s.ToJson()
	if err != nil {
		return err
	}

	if _, err := jsonschema.CompileString("schema.json", string(jsonStr)); err != nil {
		return err
	}

	// Check if type is valid
	if s.Type != "" &&
		s.Type != "object" &&
		s.Type != "string" &&
		s.Type != "integer" &&
		s.Type != "number" &&
		s.Type != "array" &&
		s.Type != "null" &&
		s.Type != "boolean" {
		return fmt.Errorf("unsupported type %s", s.Type)
	}

	// Check if type=string if pattern!=""
	if s.Pattern != "" && s.Type != "" && s.Type != "string" {
		return fmt.Errorf("cant use pattern if type is %s. Use type=string", s.Type)
	}

	// Check if type=string if format!=""
	if s.Format != "" && s.Type != "" && s.Type != "string" {
		return fmt.Errorf("cant use format if type is %s. Use type=string", s.Type)
	}

	// Cant use Format and Pattern together
	if s.Format != "" && s.Pattern != "" {
		return errors.New("cant use format and pattern option at the same time")
	}

	// Validate nested Items schema
	if s.Items != nil {
		if err := s.Items.Validate(); err != nil {
			return err
		}
	}

	// If type and items are used, type must be array
	if s.Items != nil && s.Type != "" && s.Type != "array" {
		return fmt.Errorf("cant use items if type is %s. Use type=array", s.Type)
	}

	if s.Const != "" && s.Type != "" {
		return errors.New("if your are using const, you can't use type")
	}

	if s.Enum != nil && s.Type != "" {
		return errors.New("if your are using enum, you can't use type")
	}

	// Check if format is valid
	// https://json-schema.org/understanding-json-schema/reference/string.html#built-in-formats
	// We currently dont support https://datatracker.ietf.org/doc/html/rfc3339#appendix-A
	if s.Format != "" &&
		s.Format != "date-time" &&
		s.Format != "time" &&
		s.Format != "date" &&
		s.Format != "duration" &&
		s.Format != "email" &&
		s.Format != "idn-email" &&
		s.Format != "hostname" &&
		s.Format != "idn-hostname" &&
		s.Format != "ipv4" &&
		s.Format != "ipv6" &&
		s.Format != "uuid" &&
		s.Format != "uri" &&
		s.Format != "uri-reference" &&
		s.Format != "iri" &&
		s.Format != "iri-reference" &&
		s.Format != "uri-template" &&
		s.Format != "json-pointer" &&
		s.Format != "relative-json-pointer" &&
		s.Format != "regex" {
		return fmt.Errorf("the format %s is not supported", s.Format)
	}

	if s.Minimum != nil && s.Type != "" && s.Type != "number" && s.Type != "integer" {
		return fmt.Errorf("if you use minimum, you cant use type=%s", s.Type)
	}
	if s.Maximum != nil && s.Type != "" && s.Type != "number" && s.Type != "integer" {
		return fmt.Errorf("if you use maximum, you cant use type=%s", s.Type)
	}
	if s.ExclusiveMinimum != nil && s.Type != "" && s.Type != "number" && s.Type != "integer" {
		return fmt.Errorf("if you use exclusiveMinimum, you cant use type=%s", s.Type)
	}
	if s.ExclusiveMaximum != nil && s.Type != "" && s.Type != "number" && s.Type != "integer" {
		return fmt.Errorf("if you use exclusiveMaximum, you cant use type=%s", s.Type)
	}
	if s.MultipleOf != nil && s.Type != "" && s.Type != "number" && s.Type != "integer" {
		return fmt.Errorf("if you use multiple, you cant use type=%s", s.Type)
	}
	if s.MultipleOf != nil && *s.MultipleOf <= 0 {
		return errors.New("multiple option must be greater than 0")
	}
	if s.Minimum != nil && s.ExclusiveMinimum != nil {
		return errors.New("you cant set minimum and exclusiveMinimum")
	}
	if s.Maximum != nil && s.ExclusiveMaximum != nil {
		return errors.New("you cant set minimum and exclusiveMaximum")
	}
	return nil
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
	return "", fmt.Errorf("unsupported yaml tag found: %s", tag)
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
		return result, "",
			fmt.Errorf("unclosed schema block found in comment: %s", comment)
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
	dontRemoveHelmDocsPrefix bool,
	omitAdditionalProperties bool,
	parentRequiredProperties *[]string,
) Schema {
	var schema Schema

	switch node.Kind {
	case yaml.DocumentNode:
		if len(node.Content) != 1 {
			log.Fatalf("Strange yaml document found:\n%v\n", node.Content[:])
		}

		requiredProperties := []string{}

		schema.Type = "object"
		schema.Schema = "http://json-schema.org/draft-07/schema#"
		schema.Properties = YamlToSchema(
			node.Content[0],
			keepFullComment,
			dontRemoveHelmDocsPrefix,
			omitAdditionalProperties,
			&requiredProperties,
		).Properties

		if _, ok := schema.Properties["global"]; !ok {
			// global key must be present, otherwise helm lint will fail
			if schema.Properties == nil {
				schema.Properties = make(map[string]*Schema)
			}
			schema.Properties["global"] = &Schema{
				Type:        "object",
				Title:       "global",
				Description: "Global values are values that can be accessed from any chart or subchart by exactly the same name.",
			}
		}

		if len(requiredProperties) > 0 {
			schema.RequiredProperties = requiredProperties
		}
		if !omitAdditionalProperties {
			schema.AdditionalProperties = new(bool)
		}
	case yaml.MappingNode:
		for i := 0; i < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			valueNode := node.Content[i+1]

			comment := keyNode.HeadComment
			if !keepFullComment {
				leadingCommentsRemover := regexp.MustCompile(`(?s)(?m)(?:.*\n{2,})+`)
				comment = leadingCommentsRemover.ReplaceAllString(comment, "")
			}

			keyNodeSchema, description, err := GetSchemaFromComment(comment)
			if err != nil {
				log.Fatalf("Error while parsing comment of key %s: %v", keyNode.Value, err)
			}
			if !dontRemoveHelmDocsPrefix {
				prefixRemover := regexp.MustCompile(`(?m)^--\s?`)
				description = prefixRemover.ReplaceAllString(description, "")
			}

			if keyNodeSchema.HasData {
				if err := keyNodeSchema.Validate(); err != nil {
					log.Fatalf(
						"Error while validating jsonschema of key %s: %v",
						keyNode.Value,
						err,
					)
				}
			} else {
				nodeType, err := typeFromTag(valueNode.Tag)
				if err != nil {
					log.Fatal(err)
				}
				keyNodeSchema.Type = nodeType
			}

			// Add key to required array of parent
			if keyNodeSchema.Required || !keyNodeSchema.HasData {
				*parentRequiredProperties = append(*parentRequiredProperties, keyNode.Value)
			}

			if !omitAdditionalProperties && valueNode.Kind == yaml.MappingNode &&
				(!keyNodeSchema.HasData || keyNodeSchema.AdditionalProperties == nil) {
				keyNodeSchema.AdditionalProperties = new(bool)
			}

			// If no title was set, use the key value
			if keyNodeSchema.Title == "" {
				keyNodeSchema.Title = keyNode.Value
			}

			// If no description was set, use the rest of the comment as description
			if keyNodeSchema.Description == "" {
				keyNodeSchema.Description = description
			}

			// If no default value was set, use the values node value as default
			if keyNodeSchema.Default == nil && valueNode.Kind == yaml.ScalarNode {
				keyNodeSchema.Default = valueNode.Value
			}

			// If the value is another map and no properties are set, get them from default values
			if valueNode.Kind == yaml.MappingNode && keyNodeSchema.Properties == nil {
				requiredProperties := []string{}
				keyNodeSchema.Properties = YamlToSchema(
					valueNode,
					keepFullComment,
					dontRemoveHelmDocsPrefix,
					omitAdditionalProperties,
					&requiredProperties,
				).Properties
				if len(requiredProperties) > 0 {
					keyNodeSchema.RequiredProperties = requiredProperties
				}
			} else if valueNode.Kind == yaml.SequenceNode && keyNodeSchema.Items == nil {
				// If the value is a sequence, but no items are predefined
				var seqSchema Schema

				for _, itemNode := range valueNode.Content {
					if itemNode.Kind == yaml.ScalarNode {
						itemNodeType, err := typeFromTag(itemNode.Tag)
						if err != nil {
							log.Fatal(err)
						}
						seqSchema.AnyOf = append(seqSchema.AnyOf, &Schema{Type: itemNodeType})
					} else {
						itemRequiredProperties := []string{}
						itemSchema := YamlToSchema(itemNode, keepFullComment, dontRemoveHelmDocsPrefix, omitAdditionalProperties, &itemRequiredProperties)

						if len(itemRequiredProperties) > 0 {
							itemSchema.RequiredProperties = itemRequiredProperties
						}

						if !omitAdditionalProperties && itemNode.Kind == yaml.MappingNode && (!itemSchema.HasData || itemSchema.AdditionalProperties == nil) {
							itemSchema.AdditionalProperties = new(bool)
						}

						seqSchema.AnyOf = append(seqSchema.AnyOf, &itemSchema)
					}
				}
				keyNodeSchema.Items = &seqSchema
			}
			if schema.Properties == nil {
				schema.Properties = make(map[string]*Schema)
			}
			schema.Properties[keyNode.Value] = &keyNodeSchema
		}
	}

	return schema
}
