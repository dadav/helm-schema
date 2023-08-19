package schema

import (
	"bufio"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	yaml "gopkg.in/yaml.v3"
)

const (
	SchemaPrefix  = "# @schema "
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

type SchemaAnnotation struct {
	Type             string
	Title            string
	Description      string
	Pattern          string
	Format           string
	Required         bool
	Deprecated       bool
	Items            string
	Enum             string
	Const            string
	Examples         string
	Minimum          *int
	Maximum          *int
	ExclusiveMinimum *int
	ExclusiveMaximum *int
	MultipleOf       *int
}

// ToSchema converts the SchemaAnnotation struct to the final jsonschema
func (s SchemaAnnotation) ToSchema() map[string]interface{} {
	ret := map[string]interface{}{}

	if s.Type != "" {
		ret["type"] = s.Type
	}

	if s.Title != "" {
		ret["title"] = s.Title
	}

	if s.Description != "" {
		ret["description"] = s.Description
	}

	if s.Pattern != "" {
		ret["pattern"] = s.Pattern
	}

	if s.Format != "" {
		ret["format"] = s.Format
	}

	if s.Enum != "" {
		ret["enum"] = strings.Split(s.Enum, "|")
	}

	if s.Examples != "" {
		ret["examples"] = strings.Split(s.Examples, "|")
	}

	if s.Deprecated {
		ret["deprecated"] = true
	}

	if s.Const != "" {
		ret["const"] = s.Const
	}

	if s.Minimum != nil {
		ret["minimum"] = *s.Minimum
	}

	if s.Maximum != nil {
		ret["maximum"] = *s.Maximum
	}

	if s.ExclusiveMaximum != nil {
		ret["exclusiveMaximum"] = *s.ExclusiveMaximum
	}

	if s.ExclusiveMinimum != nil {
		ret["exclusiveMinimum"] = s.ExclusiveMinimum
	}

	if s.MultipleOf != nil {
		ret["multipleOf"] = s.MultipleOf
	}

	if len(s.Items) > 0 {
		items := []interface{}{}
		for _, item := range strings.Split(s.Items, "|") {
			items = append(items, map[string]string{"type": item})
		}
		ret["items"] = map[string]interface{}{"oneOf": items}
	}

	return ret
}

// Validate checks if there are some semantic errors in the given schema data
func (s SchemaAnnotation) Validate() error {
	// Check if type is valid
	if s.Type != "" &&
		s.Type != "object" &&
		s.Type != "string" &&
		s.Type != "integer" &&
		s.Type != "number" &&
		s.Type != "array" &&
		s.Type != "null" &&
		s.Type != "boolean" {
		return errors.New(fmt.Sprintf("Unsupported type %s", s.Type))
	}

	// Check if type=string if pattern!=""
	if s.Pattern != "" && s.Type != "" && s.Type != "string" {
		return errors.New(fmt.Sprintf("Cant use pattern if type is %s. Use type=string", s.Type))
	}

	// Check if type=string if format!=""
	if s.Format != "" && s.Type != "" && s.Type != "string" {
		return errors.New(fmt.Sprintf("Cant use format if type is %s. Use type=string", s.Type))
	}

	// Cant use Format and Pattern together
	if s.Format != "" && s.Pattern != "" {
		return errors.New(fmt.Sprintf("Cant use format and pattern option at the same time"))
	}

	// Only supported types of Items are allowed
	if s.Items != "" {
		for _, item := range strings.Split(s.Items, "|") {
			if item != "string" &&
				item != "integer" &&
				item != "object" &&
				item != "number" &&
				item != "boolean" &&
				item != "array" &&
				item != "null" {
				return errors.New(fmt.Sprintf("Item type %s not supported", item))
			}
		}
	}

	// If type and items are used, type must be array
	if s.Items != "" && s.Type != "" && s.Type != "array" {
		return errors.New(fmt.Sprintf("Cant use items if type is %s. Use type=array", s.Type))
	}

	if s.Const != "" && s.Type != "" {
		return errors.New("If your are using const, you can't use type")
	}

	if s.Enum != "" && s.Type != "" {
		return errors.New("If your are using enum, you can't use type")
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
		return errors.New(fmt.Sprintf("The format %s is not supported.", s.Format))
	}

	if s.Minimum != nil && s.Type != "" && s.Type != "number" && s.Type != "integer" {
		return errors.New(fmt.Sprintf("If your use min, you cant use type=%s", s.Type))
	}
	if s.Maximum != nil && s.Type != "" && s.Type != "number" && s.Type != "integer" {
		return errors.New(fmt.Sprintf("If your use max, you cant use type=%s", s.Type))
	}
	if s.ExclusiveMinimum != nil && s.Type != "" && s.Type != "number" && s.Type != "integer" {
		return errors.New(fmt.Sprintf("If your use xmin, you cant use type=%s", s.Type))
	}
	if s.ExclusiveMaximum != nil && s.Type != "" && s.Type != "number" && s.Type != "integer" {
		return errors.New(fmt.Sprintf("If your use xmax, you cant use type=%s", s.Type))
	}
	if s.MultipleOf != nil && s.Type != "" && s.Type != "number" && s.Type != "integer" {
		return errors.New(fmt.Sprintf("If your use multiple, you cant use type=%s", s.Type))
	}
	if s.MultipleOf != nil && *s.MultipleOf <= 0 {
		return errors.New("multiple option must be greater than 0")
	}
	if s.Minimum != nil && s.ExclusiveMinimum != nil {
		return errors.New("You cant set min and xmin")
	}
	if s.Maximum != nil && s.ExclusiveMaximum != nil {
		return errors.New("You cant set min and xmin")
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
	return "", errors.New(fmt.Sprintf("Unsupported yaml tag found: %s", tag))
}

func getSchemaFromLine(line string) (SchemaAnnotation, error) {
	schema := SchemaAnnotation{}
	pat := regexp.MustCompile(`(\w+)=([^=]*.)(?:\s|$)`)
	matches := pat.FindAllStringSubmatch(line, -1)

	seen := make(map[string]bool)

	for _, match := range matches {
		k, v := match[1], match[2]
		if _, ok := seen[k]; ok {
			return schema, errors.New(fmt.Sprintf("Duplicate option %s", k))
		} else {
			seen[k] = true
		}

		switch k {
		case "type":
			schema.Type = v
		case "title":
			schema.Title = v
		case "description":
			schema.Description = v
		case "pattern":
			schema.Pattern = v
		case "format":
			schema.Format = v
		case "required":
			schema.Required = strings.ToLower(v) == "true"
		case "deprecated":
			schema.Deprecated = strings.ToLower(v) == "true"
		case "examples":
			schema.Examples = v
		case "enum":
			schema.Enum = v
		case "const":
			schema.Const = v
		case "items":
			schema.Items = v
		case "min":
			if num, err := strconv.Atoi(v); err == nil {
				schema.Minimum = &num
			} else {
				return schema, errors.New(fmt.Sprintf("Cant parse %v as int", v))
			}
		case "max":
			if num, err := strconv.Atoi(v); err == nil {
				schema.Maximum = &num
			} else {
				return schema, errors.New(fmt.Sprintf("Cant parse %v as int", v))
			}
		case "xmax":
			if num, err := strconv.Atoi(v); err == nil {
				schema.ExclusiveMaximum = &num
			} else {
				return schema, errors.New(fmt.Sprintf("Cant parse %v as int", v))
			}
		case "xmin":
			if num, err := strconv.Atoi(v); err == nil {
				schema.ExclusiveMinimum = &num
			} else {
				return schema, errors.New(fmt.Sprintf("Cant parse %v as int", v))
			}
		case "multiple":
			if num, err := strconv.Atoi(v); err == nil {
				schema.MultipleOf = &num
			} else {
				return schema, errors.New(fmt.Sprintf("Cant parse %v as int", v))
			}
		default:
			log.Warnf("Unknown schema option %s. Ignoring", k)
		}
	}

	if err := schema.Validate(); err != nil {
		return schema, err
	}

	if schema.Pattern != "" || schema.Format != "" {
		schema.Type = "string"
	}

	if schema.Items != "" {
		schema.Type = "array"
	}

	return schema, nil
}

// GetSchemasFromComment parses the annotations from the given comment
func GetSchemasFromComment(comment string) ([]SchemaAnnotation, string, error) {
	result := []SchemaAnnotation{}
	scanner := bufio.NewScanner(strings.NewReader(comment))
	description := []string{}
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, SchemaPrefix) {
			schema, err := getSchemaFromLine(strings.TrimPrefix(line, SchemaPrefix))
			if err != nil {
				return nil, "", err
			}
			result = append(result, schema)
		} else {
			description = append(description, strings.TrimPrefix(line, CommentPrefix))
		}
	}
	return result, strings.Join(description, "\n"), nil
}

// YamlToJsonSchema recursevly parses the given yaml.Node and creates a jsonschema from it
func YamlToJsonSchema(
	node *yaml.Node,
	keepFullComment bool,
	parentRequiredProperties *[]string,
) map[string]interface{} {
	schema := make(map[string]interface{})
	requiredProperties := []string{}

	switch node.Kind {
	case yaml.DocumentNode:
		if len(node.Content) != 1 {
			log.Fatalf("Strange yaml document found:\n%v\n", node.Content[:])
		}
		schema["$schema"] = "http://json-schema.org/schema#"
		schema["type"] = "object"
		schema["properties"] = YamlToJsonSchema(
			node.Content[0],
			keepFullComment,
			&requiredProperties,
		)
		if len(requiredProperties) > 0 {
			schema["required"] = requiredProperties
		}
	case yaml.MappingNode:

		for i := 0; i < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			valueNode := node.Content[i+1]

			backslashRemover := regexp.MustCompile(`\\\n\s*#\s*`)
			comment := backslashRemover.ReplaceAllString(keyNode.HeadComment, "")
			if !keepFullComment {
				leadingCommentsRemover := regexp.MustCompile(`(?s)(?m)(?:.*\n{2,})+`)
				comment = leadingCommentsRemover.ReplaceAllString(comment, "")
			}

			parsedSchemas, description, err := GetSchemasFromComment(comment)
			if err != nil {
				log.Fatalf("Error while parsing comment of key %s: %v", keyNode.Value, err)
			}

			if len(parsedSchemas) == 1 {
				newSchema := parsedSchemas[0].ToSchema()

				if parsedSchemas[0].Required {
					*parentRequiredProperties = append(*parentRequiredProperties, keyNode.Value)
				}

				if _, ok := newSchema["title"]; !ok {
					newSchema["title"] = keyNode.Value
				}

				if _, ok := newSchema["description"]; !ok {
					newSchema["description"] = description
				}

				if _, ok := newSchema["default"]; !ok {
					newSchema["default"] = valueNode.Value
				}

				schema[keyNode.Value] = newSchema
			} else if len(parsedSchemas) > 1 {
				newSchema := map[string]interface{}{}
				oneOf := []map[string]interface{}{}

				for _, s := range parsedSchemas {
					subSchema := s.ToSchema()

					if s.Required {
						*parentRequiredProperties = append(*parentRequiredProperties, keyNode.Value)
					}

					if _, ok := subSchema["title"]; !ok {
						subSchema["title"] = keyNode.Value
					}

					if _, ok := subSchema["description"]; !ok {
						subSchema["description"] = description
					}

					if _, ok := subSchema["default"]; !ok {
						subSchema["default"] = valueNode.Value
					}

					oneOf = append(oneOf, subSchema)
				}

				newSchema["oneOf"] = oneOf
				schema[keyNode.Value] = newSchema
			} else {
				// No schemas specified, use the current value as type
				nodeType, err := typeFromTag(valueNode.Tag)
				if err != nil {
					log.Fatal(err)
				}

				schema[keyNode.Value] = map[string]interface{}{
					"type":        nodeType,
					"title":       keyNode.Value,
					"description": description,
					"default":     valueNode.Value,
				}
			}

			if valueNode.Kind == yaml.MappingNode {
				schema[keyNode.Value].(map[string]interface{})["properties"] = YamlToJsonSchema(
					valueNode,
					keepFullComment,
					&requiredProperties,
				)
				if len(requiredProperties) > 0 {
					schema[keyNode.Value].(map[string]interface{})["required"] = requiredProperties
				}
			}
		}
	}

	return schema
}
