package schema

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/dadav/go-jsonpointer"
	"github.com/dadav/helm-schema/pkg/util"
	"github.com/norwoodj/helm-docs/pkg/helm"
	"github.com/santhosh-tekuri/jsonschema/v6"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// SchemaPrefix and CommentPrefix define the markers used for schema annotations in comments
const (
	SchemaPrefix  = "# @schema"
	CommentPrefix = "#"

	// CustomAnnotationPrefix marks custom annotations.
	// Custom annotations are extensions to the JSON Schema specification
	// See: https://json-schema.org/blog/posts/custom-annotations-will-continue
	CustomAnnotationPrefix = "x-"
)

// YAML tag constants used for type inference
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

// SchemaOrBool represents a JSON Schema field that can be either a boolean or a Schema object
type SchemaOrBool interface{}

// BoolOrArrayOfString represents a JSON Schema field that can be either a boolean or an array of strings
// Used primarily for the "required" field which can be either true/false or an array of required property names
type BoolOrArrayOfString struct {
	Strings []string
	Bool    bool
}

func NewBoolOrArrayOfString(arr []string, b bool) BoolOrArrayOfString {
	return BoolOrArrayOfString{
		Strings: arr,
		Bool:    b,
	}
}

func (s *BoolOrArrayOfString) UnmarshalJSON(value []byte) error {
	var multi []string
	var single bool

	if err := json.Unmarshal(value, &multi); err == nil {
		s.Strings = multi
	} else if err := json.Unmarshal(value, &single); err == nil {
		s.Bool = single
	}
	return nil
}

func (s *BoolOrArrayOfString) MarshalJSON() ([]byte, error) {
	if s.Strings == nil {
		return json.Marshal([]string{})
	}
	return json.Marshal(s.Strings)
}

func (s *BoolOrArrayOfString) UnmarshalYAML(value *yaml.Node) error {
	var multi []string
	if value.ShortTag() == arrayTag {
		for _, v := range value.Content {
			var typeStr string
			err := v.Decode(&typeStr)
			if err != nil {
				return err
			}
			multi = append(multi, typeStr)
		}
		s.Strings = multi
	} else if value.ShortTag() == boolTag {
		var single bool
		err := value.Decode(&single)
		if err != nil {
			return err
		}
		s.Bool = single
	} else {
		return fmt.Errorf("could not unmarshal %v to slice of string or bool", value.Content)
	}
	return nil
}

type StringOrArrayOfString []string

func (s *StringOrArrayOfString) UnmarshalYAML(value *yaml.Node) error {
	var multi []string
	if value.ShortTag() == arrayTag {
		for _, v := range value.Content {
			if v.ShortTag() == nullTag {
				multi = append(multi, "null")
			} else {
				var typeStr string
				err := v.Decode(&typeStr)
				if err != nil {
					return err
				}
				multi = append(multi, typeStr)
			}
		}
		*s = multi
	} else {
		var single string
		err := value.Decode(&single)
		if err != nil {
			return err
		}
		*s = []string{single}
	}
	return nil
}

func (s *StringOrArrayOfString) UnmarshalJSON(value []byte) error {
	var multi []string
	var single string

	if err := json.Unmarshal(value, &multi); err == nil {
		*s = multi
	} else if err := json.Unmarshal(value, &single); err == nil {
		*s = []string{single}
	}
	return nil
}

func (s *StringOrArrayOfString) MarshalJSON() ([]byte, error) {
	if len(*s) == 1 {
		return json.Marshal([]string(*s)[0])
	}
	return json.Marshal([]string(*s))
}

func (s *StringOrArrayOfString) Validate() error {
	// Check if type is valid
	for _, t := range []string(*s) {
		if t != "" &&
			t != "object" &&
			t != "string" &&
			t != "integer" &&
			t != "number" &&
			t != "array" &&
			t != "null" &&
			t != "boolean" {
			return fmt.Errorf("unsupported type %s", s)
		}
	}
	return nil
}

func (s *StringOrArrayOfString) IsEmpty() bool {
	for _, t := range []string(*s) {
		if t == "" {
			return true
		}
	}
	return len(*s) == 0
}

func (s *StringOrArrayOfString) Matches(typeString string) bool {
	for _, t := range []string(*s) {
		if t == typeString {
			return true
		}
	}
	return false
}

// MarshalJSON custom marshal method for Schema. It inlines the CustomAnnotations fields
func (s *Schema) MarshalJSON() ([]byte, error) {
	// Create a map to hold all the fields
	type Alias Schema
	data := make(map[string]interface{})

	// Marshal the Schema struct (excluding CustomAnnotations)
	alias := (*Alias)(s)
	aliasJSON, err := json.Marshal(alias)
	if err != nil {
		return nil, err
	}

	// Unmarshal the JSON back into the map
	if err := json.Unmarshal(aliasJSON, &data); err != nil {
		return nil, err
	}

	// inline the CustomAnnotations fields
	for key, value := range s.CustomAnnotations {
		data[key] = value
	}

	delete(data, "CustomAnnotations")

	// Marshal the final map into JSON
	return json.Marshal(data)
}

// Schema struct contains yaml tags for reading, json for writing (creating the jsonschema)
type Schema struct {
	AdditionalProperties SchemaOrBool           `yaml:"additionalProperties,omitempty" json:"additionalProperties,omitempty"`
	Default              interface{}            `yaml:"default,omitempty"              json:"default,omitempty"`
	Then                 *Schema                `yaml:"then,omitempty"                 json:"then,omitempty"`
	PatternProperties    map[string]*Schema     `yaml:"patternProperties,omitempty"    json:"patternProperties,omitempty"`
	Properties           map[string]*Schema     `yaml:"properties,omitempty"           json:"properties,omitempty"`
	If                   *Schema                `yaml:"if,omitempty"                   json:"if,omitempty"`
	Minimum              *int                   `yaml:"minimum,omitempty"              json:"minimum,omitempty"`
	MultipleOf           *int                   `yaml:"multipleOf,omitempty"           json:"multipleOf,omitempty"`
	ExclusiveMaximum     *int                   `yaml:"exclusiveMaximum,omitempty"     json:"exclusiveMaximum,omitempty"`
	Items                *Schema                `yaml:"items,omitempty"                json:"items,omitempty"`
	ExclusiveMinimum     *int                   `yaml:"exclusiveMinimum,omitempty"     json:"exclusiveMinimum,omitempty"`
	Maximum              *int                   `yaml:"maximum,omitempty"              json:"maximum,omitempty"`
	Else                 *Schema                `yaml:"else,omitempty"                 json:"else,omitempty"`
	Pattern              string                 `yaml:"pattern,omitempty"              json:"pattern,omitempty"`
	Const                interface{}            `yaml:"const,omitempty"                json:"const,omitempty"`
	Ref                  string                 `yaml:"$ref,omitempty"                 json:"$ref,omitempty"`
	Schema               string                 `yaml:"$schema,omitempty"              json:"$schema,omitempty"`
	Id                   string                 `yaml:"$id,omitempty"                  json:"$id,omitempty"`
	Format               string                 `yaml:"format,omitempty"               json:"format,omitempty"`
	Description          string                 `yaml:"description,omitempty"          json:"description,omitempty"`
	Title                string                 `yaml:"title,omitempty"                json:"title,omitempty"`
	Type                 StringOrArrayOfString  `yaml:"type,omitempty"                 json:"type,omitempty"`
	AnyOf                []*Schema              `yaml:"anyOf,omitempty"                json:"anyOf,omitempty"`
	AllOf                []*Schema              `yaml:"allOf,omitempty"                json:"allOf,omitempty"`
	OneOf                []*Schema              `yaml:"oneOf,omitempty"                json:"oneOf,omitempty"`
	Not                  *Schema                `yaml:"not,omitempty"                json:"not,omitempty"`
	Examples             []string               `yaml:"examples,omitempty"             json:"examples,omitempty"`
	Enum                 []string               `yaml:"enum,omitempty"                 json:"enum,omitempty"`
	HasData              bool                   `yaml:"-"                              json:"-"`
	Deprecated           bool                   `yaml:"deprecated,omitempty"           json:"deprecated,omitempty"`
	ReadOnly             bool                   `yaml:"readOnly,omitempty"           json:"readOnly,omitempty"`
	WriteOnly            bool                   `yaml:"writeOnly,omitempty"           json:"writeOnly,omitempty"`
	Required             BoolOrArrayOfString    `yaml:"required,omitempty"             json:"required,omitempty"`
	CustomAnnotations    map[string]interface{} `yaml:"-"                              json:",omitempty"`
	MinLength            *int                   `yaml:"minLength,omitempty"              json:"minLength,omitempty"`
	MaxLength            *int                   `yaml:"maxLength,omitempty"              json:"maxLength,omitempty"`
	MinItems             *int                   `yaml:"minItems,omitempty"              json:"minItems,omitempty"`
	MaxItems             *int                   `yaml:"maxItems,omitempty"              json:"maxItems,omitempty"`
	UniqueItems          bool                   `yaml:"uniqueItems,omitempty"          json:"uniqueItems,omitempty"`
	Defs                 map[string]*Schema     `yaml:"$defs,omitempty"                json:"$defs,omitempty"`
}

func NewSchema(schemaType string) *Schema {
	if schemaType == "" {
		return &Schema{}
	}

	return &Schema{
		Type:     []string{schemaType},
		Required: NewBoolOrArrayOfString([]string{}, false),
	}
}

// getJsonKeys returns a slice of all JSON tag values from the Schema struct fields.
// This is used to identify known fields during YAML unmarshaling to separate them
// from custom annotations.
func (s Schema) getJsonKeys() []string {
	result := []string{}
	t := reflect.TypeOf(s)

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		result = append(result, field.Tag.Get("json"))
	}
	return result
}

// UnmarshalYAML implements custom YAML unmarshaling for Schema objects.
// It handles both standard schema fields and custom annotations (prefixed with "x-").
// Custom annotations are stored in the CustomAnnotations map while standard fields
// are unmarshaled directly into the Schema struct.
func (s *Schema) UnmarshalYAML(node *yaml.Node) error {
	// Create an alias type to avoid recursion
	type schemaAlias Schema
	alias := new(schemaAlias)
	// copy all existing fields
	*alias = schemaAlias(*s)

	// Unmarshal known fields into alias
	if err := node.Decode(alias); err != nil {
		return err
	}

	// Initialize CustomAnnotations map
	alias.CustomAnnotations = make(map[string]interface{})

	knownKeys := s.getJsonKeys()

	// Iterate through all node fields
	for i := 0; i < len(node.Content)-1; i += 2 {
		keyNode := node.Content[i]
		valueNode := node.Content[i+1]
		key := keyNode.Value

		if slices.Contains(knownKeys, key) {
			continue
		}

		// Unmarshal unknown fields into the CustomAnnotations map
		if !strings.HasPrefix(key, CustomAnnotationPrefix) {
			continue
		}
		var value interface{}
		if err := valueNode.Decode(&value); err != nil {
			return err
		}
		alias.CustomAnnotations[key] = value
	}

	// Copy alias to the main struct
	*s = Schema(*alias)
	return nil
}

// Set sets the HasData field to true
func (s *Schema) Set() {
	s.HasData = true
}

// DisableRequiredProperties recursively disables all required property validations throughout the schema.
// This includes:
// - Setting the root schema's required field to an empty array
// - Recursively disabling required properties in all nested schemas (properties, items, etc.)
// - Handling all conditional schemas (if/then/else)
// - Processing all composition schemas (anyOf/oneOf/allOf)
func (s *Schema) DisableRequiredProperties() {
	s.Required = NewBoolOrArrayOfString([]string{}, false)
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
	if s.Not != nil {
		s.Not.DisableRequiredProperties()
	}

	// Add handling for AdditionalProperties when it's a Schema
	if s.AdditionalProperties != nil {
		if subSchema, ok := s.AdditionalProperties.(Schema); ok {
			subSchema.DisableRequiredProperties()
			s.AdditionalProperties = subSchema
		}
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

// Supported format values according to JSON Schema specification
const (
	FormatDateTime       = "date-time"
	FormatTime           = "time"
	FormatDate           = "date"
	FormatDuration       = "duration"
	FormatEmail          = "email"
	FormatIDNEmail       = "idn-email"
	FormatHostname       = "hostname"
	FormatIDNHostname    = "idn-hostname"
	FormatIPv4           = "ipv4"
	FormatIPv6           = "ipv6"
	FormatUUID           = "uuid"
	FormatURI            = "uri"
	FormatURIReference   = "uri-reference"
	FormatIRI            = "iri"
	FormatIRIReference   = "iri-reference"
	FormatURITemplate    = "uri-template"
	FormatJSONPointer    = "json-pointer"
	FormatRelJSONPointer = "relative-json-pointer"
	FormatRegex          = "regex"
)

var supportedFormats = map[string]bool{
	FormatDateTime: true, FormatTime: true, FormatDate: true,
	FormatDuration: true, FormatEmail: true, FormatIDNEmail: true,
	FormatHostname: true, FormatIDNHostname: true, FormatIPv4: true,
	FormatIPv6: true, FormatUUID: true, FormatURI: true,
	FormatURIReference: true, FormatIRI: true, FormatIRIReference: true,
	FormatURITemplate: true, FormatJSONPointer: true,
	FormatRelJSONPointer: true, FormatRegex: true,
}

// Validate performs comprehensive validation of the schema
func (s Schema) Validate() error {
	// Validate schema syntax
	if err := s.validateSchemaSyntax(); err != nil {
		return err
	}

	// Validate type constraints
	if err := s.validateTypeConstraints(); err != nil {
		return err
	}

	// Validate numeric constraints
	if err := s.validateNumericConstraints(); err != nil {
		return err
	}

	// Validate string constraints
	if err := s.validateStringConstraints(); err != nil {
		return err
	}

	// Validate array constraints
	if err := s.validateArrayConstraints(); err != nil {
		return err
	}

	// Validate nested schemas
	if err := s.validateNestedSchemas(); err != nil {
		return err
	}

	return nil
}

func (s Schema) validateSchemaSyntax() error {
	jsonStr, err := s.ToJson()
	if err != nil {
		return fmt.Errorf("failed to convert schema to JSON: %w", err)
	}

	c := jsonschema.NewCompiler()
	if err := c.AddResource("schema.json", jsonStr); err != nil {
		return fmt.Errorf("invalid schema syntax: %w", err)
	}

	return s.Type.Validate()
}

func (s Schema) validateTypeConstraints() error {
	if s.Const != nil && !s.Type.IsEmpty() {
		return errors.New("cannot use both 'const' and 'type' in the same schema")
	}

	if s.Enum != nil && !s.Type.IsEmpty() {
		return errors.New("cannot use both 'enum' and 'type' in the same schema")
	}

	return nil
}

func (s Schema) validateNumericConstraints() error {
	if !s.hasNumericConstraints() {
		return nil
	}

	if !s.Type.IsEmpty() && !s.Type.Matches("number") && !s.Type.Matches("integer") {
		return fmt.Errorf("numeric constraints can only be used with number or integer types, got %v", s.Type)
	}

	if s.MultipleOf != nil && *s.MultipleOf <= 0 {
		return errors.New("multipleOf must be greater than 0")
	}

	if s.Minimum != nil && s.ExclusiveMinimum != nil {
		return errors.New("cannot use both minimum and exclusiveMinimum")
	}

	if s.Maximum != nil && s.ExclusiveMaximum != nil {
		return errors.New("cannot use both maximum and exclusiveMaximum")
	}

	return nil
}

func (s Schema) validateStringConstraints() error {
	if s.Format != "" {
		if !s.Type.IsEmpty() && !s.Type.Matches("string") {
			return fmt.Errorf("format can only be used with string type, got %v", s.Type)
		}

		if !supportedFormats[s.Format] {
			return fmt.Errorf("unsupported format: %s", s.Format)
		}
	}

	if s.Pattern != "" {
		if !s.Type.IsEmpty() && !s.Type.Matches("string") {
			return fmt.Errorf("pattern can only be used with string type, got %v", s.Type)
		}
	}

	if s.Format != "" && s.Pattern != "" {
		return errors.New("cannot use both format and pattern in the same schema")
	}

	if s.MaxLength != nil && s.MinLength != nil && *s.MinLength > *s.MaxLength {
		return fmt.Errorf("minLength (%d) cannot be greater than maxLength (%d)", *s.MinLength, *s.MaxLength)
	}

	return nil
}

func (s Schema) validateArrayConstraints() error {
	if s.Items != nil {
		if !s.Type.IsEmpty() && !s.Type.Matches("array") {
			return fmt.Errorf("items can only be used with array type, got %v", s.Type)
		}

		if err := s.Items.Validate(); err != nil {
			return fmt.Errorf("invalid items schema: %w", err)
		}
	}

	if s.MinItems != nil || s.MaxItems != nil {
		if !s.Type.IsEmpty() && !s.Type.Matches("array") {
			return fmt.Errorf("minItems/maxItems can only be used with array type, got %v", s.Type)
		}

		if s.MinItems != nil && s.MaxItems != nil && *s.MaxItems < *s.MinItems {
			return fmt.Errorf("maxItems (%d) cannot be less than minItems (%d)", *s.MaxItems, *s.MinItems)
		}
	}

	return nil
}

func (s Schema) validateNestedSchemas() error {
	// Validate combinatorial schemas
	for _, schemas := range [][]*Schema{s.AllOf, s.AnyOf, s.OneOf} {
		for _, schema := range schemas {
			if err := schema.Validate(); err != nil {
				return err
			}
		}
	}

	// Validate conditional schemas
	for _, schema := range []*Schema{s.If, s.Then, s.Else, s.Not} {
		if schema != nil {
			if err := schema.Validate(); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s Schema) hasNumericConstraints() bool {
	return s.Minimum != nil || s.Maximum != nil ||
		s.ExclusiveMinimum != nil || s.ExclusiveMaximum != nil ||
		s.MultipleOf != nil
}

var possibleSkipFields = []string{"type", "title", "description", "required", "default", "additionalProperties"}

type SkipAutoGenerationConfig struct {
	Type, Title, Description, Required, Default, AdditionalProperties bool
}

func NewSkipAutoGenerationConfig(flag []string) (*SkipAutoGenerationConfig, error) {
	var config SkipAutoGenerationConfig

	var invalidFlags []string

	for _, fieldName := range flag {
		if !slices.Contains(possibleSkipFields, fieldName) {
			invalidFlags = append(invalidFlags, fieldName)
		}
		if fieldName == "type" {
			config.Type = true
		}
		if fieldName == "title" {
			config.Title = true
		}
		if fieldName == "description" {
			config.Description = true
		}
		if fieldName == "required" {
			config.Required = true
		}
		if fieldName == "default" {
			config.Default = true
		}
		if fieldName == "additionalProperties" {
			config.AdditionalProperties = true
		}
	}

	if len(invalidFlags) != 0 {
		return nil, fmt.Errorf("unsupported field names '%s' for skipping auto-generation", strings.Join(invalidFlags, "', '"))
	}

	return &config, nil
}

func typeFromTag(tag string) ([]string, error) {
	switch tag {
	case nullTag:
		return []string{"null"}, nil
	case boolTag:
		return []string{"boolean"}, nil
	case strTag:
		return []string{"string"}, nil
	case intTag:
		return []string{"integer"}, nil
	case floatTag:
		return []string{"number"}, nil
	case timestampTag:
		return []string{"string"}, nil
	case arrayTag:
		return []string{"array"}, nil
	case mapTag:
		return []string{"object"}, nil
	}
	return []string{}, fmt.Errorf("unsupported yaml tag found: %s", tag)
}

// FixRequiredProperties iterates over the properties and checks if required has a boolean value.
// Then the property is added to the parents required property list
func FixRequiredProperties(schema *Schema) error {
	if schema.Properties != nil {
		for propName, propValue := range schema.Properties {
			FixRequiredProperties(propValue)
			if propValue.Required.Bool && !slices.Contains(schema.Required.Strings, propName) {
				schema.Required.Strings = append(schema.Required.Strings, propName)
			}
		}
		if !slices.Contains(schema.Type, "object") {
			// If .Properties is set, type must be object
			schema.Type = []string{"object"}
		}
	}

	if schema.Then != nil {
		FixRequiredProperties(schema.Then)
	}

	if schema.If != nil {
		FixRequiredProperties(schema.If)
	}

	if schema.Else != nil {
		FixRequiredProperties(schema.Else)
	}

	if schema.Items != nil {
		FixRequiredProperties(schema.Items)
	}

	if schema.AdditionalProperties != nil {
		if subSchema, ok := schema.AdditionalProperties.(Schema); ok {
			FixRequiredProperties(&subSchema)
		}
	}

	if len(schema.AnyOf) > 0 {
		for _, subSchema := range schema.AnyOf {
			FixRequiredProperties(subSchema)
		}
	}

	if len(schema.AllOf) > 0 {
		for _, subSchema := range schema.AllOf {
			FixRequiredProperties(subSchema)
		}
	}

	if len(schema.OneOf) > 0 {
		for _, subSchema := range schema.OneOf {
			FixRequiredProperties(subSchema)
		}
	}

	if schema.Not != nil {
		FixRequiredProperties(schema.Not)
	}

	return nil
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
			rawSchema = append(rawSchema, strings.TrimPrefix(strings.TrimPrefix(content, CommentPrefix), " "))
			result.Set()
		} else {
			description = append(description, strings.TrimPrefix(strings.TrimPrefix(line, CommentPrefix), " "))
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

// YamlToSchema recursively parses a YAML node and creates a JSON Schema from it
// Parameters:
//   - valuesPath: path to the values file being processed
//   - node: current YAML node being processed
//   - keepFullComment: whether to preserve all comment text
//   - helmDocsCompatibilityMode: whether to parse helm-docs annotations
//   - dontRemoveHelmDocsPrefix: whether to keep helm-docs prefixes in comments
//   - skipAutoGeneration: configuration for which fields should not be auto-generated
//   - parentRequiredProperties: list of required properties to populate in parent
func YamlToSchema(
	valuesPath string,
	node *yaml.Node,
	keepFullComment bool,
	helmDocsCompatibilityMode bool,
	dontRemoveHelmDocsPrefix bool,
	dontAddGlobal bool,
	skipAutoGeneration *SkipAutoGenerationConfig,
	parentRequiredProperties *[]string,
) *Schema {
	schema := NewSchema("object")

	switch node.Kind {
	case yaml.DocumentNode:
		if len(node.Content) != 1 {
			log.Fatalf("Strange yaml document found:\n%v\n", node.Content[:])
		}

		schema.Schema = "http://json-schema.org/draft-07/schema#"
		schema.Properties = YamlToSchema(
			valuesPath,
			node.Content[0],
			keepFullComment,
			helmDocsCompatibilityMode,
			dontRemoveHelmDocsPrefix,
			dontAddGlobal,
			skipAutoGeneration,
			&schema.Required.Strings,
		).Properties

		if _, ok := schema.Properties["global"]; !ok && !dontAddGlobal {
			// global key must be present, otherwise helm lint will fail
			if schema.Properties == nil {
				schema.Properties = make(map[string]*Schema)
			}
			schema.Properties["global"] = NewSchema(
				"object",
			)
			if !skipAutoGeneration.Title {
				schema.Properties["global"].Title = "global"
			}
			if !skipAutoGeneration.Description {
				schema.Properties["global"].Description = "Global values are values that can be accessed from any chart or subchart by exactly the same name."
			}
		}

		// always disable on top level
		if !skipAutoGeneration.AdditionalProperties {
			schema.AdditionalProperties = new(bool)
		}
	case yaml.MappingNode:
		for i := 0; i < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			valueNode := node.Content[i+1]

			if valueNode.Kind == yaml.AliasNode {
				valueNode = valueNode.Alias
			}

			comment := keyNode.HeadComment
			if !keepFullComment {
				leadingCommentsRemover := regexp.MustCompile(`(?s)(?m)(?:.*\n{2,})+`)
				comment = leadingCommentsRemover.ReplaceAllString(comment, "")
			}

			keyNodeSchema, description, err := GetSchemaFromComment(comment)
			if err != nil {
				log.Fatalf("Error while parsing comment of key %s: %v", keyNode.Value, err)
			}

			if helmDocsCompatibilityMode {
				_, helmDocsValue := helm.ParseComment(strings.Split(keyNode.HeadComment, "\n"))
				if helmDocsValue.Default != "" {
					keyNodeSchema.Set()
					keyNodeSchema.Default = helmDocsValue.Default
				}
				if helmDocsValue.Description != "" {
					keyNodeSchema.Set()
					keyNodeSchema.Description = helmDocsValue.Description
				}
				if helmDocsValue.ValueType != "" {
					helmDocsType, err := helmDocsTypeToSchemaType(helmDocsValue.ValueType)
					if err != nil {
						log.Warnln(err)
					} else {
						keyNodeSchema.Set()
						keyNodeSchema.Type = StringOrArrayOfString{helmDocsType}
					}
				}
			}

			if !dontRemoveHelmDocsPrefix {
				// remove all lines containing helm-docs @tags, like @ignored, or one of those:
				// https://github.com/norwoodj/helm-docs/blob/v1.14.2/pkg/helm/chart_info.go#L18-L24
				helmDocsTagsRemover := regexp.MustCompile(`(?ms)(\r\n|\r|\n)?\s*@\w+(\s+--\s)?[^\n\r]*`)
				description = helmDocsTagsRemover.ReplaceAllString(description, "")

				prefixRemover := regexp.MustCompile(`(?m)^--\s?`)
				description = prefixRemover.ReplaceAllString(description, "")
			}

			if keyNodeSchema.Ref != "" || len(keyNodeSchema.PatternProperties) > 0 {
				// Handle $ref in main schema and pattern properties
				handleSchemaRefs(&keyNodeSchema, valuesPath)
			}

			if keyNodeSchema.HasData {
				if err := keyNodeSchema.Validate(); err != nil {
					log.Fatalf(
						"Error while validating jsonschema of key %s: %v",
						keyNode.Value,
						err,
					)
				}
			} else if !skipAutoGeneration.Type {
				nodeType, err := typeFromTag(valueNode.Tag)
				if err != nil {
					log.Fatal(err)
				}
				keyNodeSchema.Type = nodeType
			}

			// only validate or default if $ref is not set
			if keyNodeSchema.Ref == "" {

				// Add key to required array of parent
				if keyNodeSchema.Required.Bool || (len(keyNodeSchema.Required.Strings) == 0 && !skipAutoGeneration.Required && !keyNodeSchema.HasData) {
					if !slices.Contains(*parentRequiredProperties, keyNode.Value) {
						*parentRequiredProperties = append(*parentRequiredProperties, keyNode.Value)
					}
				}

				if !skipAutoGeneration.AdditionalProperties && valueNode.Kind == yaml.MappingNode &&
					(!keyNodeSchema.HasData || keyNodeSchema.AdditionalProperties == nil) {
					keyNodeSchema.AdditionalProperties = new(bool)
				}

				// If no title was set, use the key value
				if keyNodeSchema.Title == "" && !skipAutoGeneration.Title {
					keyNodeSchema.Title = keyNode.Value
				}

				// If no description was set, use the rest of the comment as description
				if keyNodeSchema.Description == "" && !skipAutoGeneration.Description {
					keyNodeSchema.Description = description
				}

				// If no default value was set, use the values node value as default
				if !skipAutoGeneration.Default && keyNodeSchema.Default == nil && valueNode.Kind == yaml.ScalarNode {
					keyNodeSchema.Default = castNodeValueByType(valueNode.Value, keyNodeSchema.Type)
				}

				// If the value is another map and no properties are set, get them from default values
				if valueNode.Kind == yaml.MappingNode && keyNodeSchema.Properties == nil {
					// Initialize properties map if needed
					if keyNodeSchema.Properties == nil {
						keyNodeSchema.Properties = make(map[string]*Schema)
					}

					generatedProperties := YamlToSchema(
						valuesPath,
						valueNode,
						keepFullComment,
						helmDocsCompatibilityMode,
						dontRemoveHelmDocsPrefix,
						dontAddGlobal,
						skipAutoGeneration,
						&keyNodeSchema.Required.Strings,
					).Properties

					// Process each property
					for i := 0; i < len(valueNode.Content); i += 2 {
						propKeyNode := valueNode.Content[i]
						// propValueNode := valueNode.Content[i+1]

						// Check if this specific property matches any pattern
						skipProperty := false
						for pattern := range keyNodeSchema.PatternProperties {
							matched, err := regexp.MatchString(pattern, propKeyNode.Value)
							if err != nil {
								log.Fatalf("Invalid pattern '%s' in patternProperties: %v", pattern, err)
							}
							if matched {
								skipProperty = true
								break
							}
						}

						// Only add schema for non-skipped properties
						if !skipProperty {
							keyNodeSchema.Properties[propKeyNode.Value] = generatedProperties[propKeyNode.Value]
						}
					}
				} else if valueNode.Kind == yaml.SequenceNode && keyNodeSchema.Items == nil {
					// If the value is a sequence, but no items are predefined
					seqSchema := NewSchema("")

					for _, itemNode := range valueNode.Content {
						if itemNode.Kind == yaml.ScalarNode {
							itemNodeType, err := typeFromTag(itemNode.Tag)
							if err != nil {
								log.Fatal(err)
							}
							seqSchema.AnyOf = append(seqSchema.AnyOf, NewSchema(itemNodeType[0]))
						} else {
							itemRequiredProperties := []string{}
							itemSchema := YamlToSchema(valuesPath, itemNode, keepFullComment, helmDocsCompatibilityMode, dontRemoveHelmDocsPrefix, dontAddGlobal, skipAutoGeneration, &itemRequiredProperties)

							itemSchema.Required.Strings = append(itemSchema.Required.Strings, itemRequiredProperties...)

							if !skipAutoGeneration.AdditionalProperties && itemNode.Kind == yaml.MappingNode && (!itemSchema.HasData || itemSchema.AdditionalProperties == nil) {
								itemSchema.AdditionalProperties = new(bool)
							}

							seqSchema.AnyOf = append(seqSchema.AnyOf, itemSchema)
						}
					}
					keyNodeSchema.Items = seqSchema

					// Because the `required` field isn't valid jsonschema (but just a helper boolean)
					// we must convert them to valid requiredProperties fields
					FixRequiredProperties(&keyNodeSchema)
				}
			}

			if schema.Properties == nil {
				schema.Properties = make(map[string]*Schema)
			}
			schema.Properties[keyNode.Value] = &keyNodeSchema
		}
	}

	return schema
}

func helmDocsTypeToSchemaType(helmDocsType string) (string, error) {
	switch helmDocsType {
	case "int":
		return "integer", nil
	case "bool":
		return "boolean", nil
	case "float":
		return "number", nil
	case "list":
		return "array", nil
	case "map":
		return "object", nil
	case "string", "object":
		return helmDocsType, nil
	}

	return "", fmt.Errorf("cant translate helm-docs type (%s) to helm-schema type", helmDocsType)
}

// castNodeValueByType attempts to convert a raw string value into the appropriate type based on
// the provided fieldType. It handles boolean, integer, and number conversions. If the conversion
// fails or the type is not supported (e.g., string), it returns the original raw value.
//
// Parameters:
//   - rawValue: The string value to be converted
//   - fieldType: Array of allowed JSON Schema types for this field
//
// Returns:
//   - The converted value as interface{}, or the original string if conversion fails/isn't needed
func castNodeValueByType(rawValue string, fieldType StringOrArrayOfString) any {
	if len(fieldType) == 0 {
		return rawValue
	}

	// rawValue must be one of fielTypes
	for _, t := range fieldType {
		switch t {
		case "boolean":
			switch rawValue {
			case "true":
				return true
			case "false":
				return false
			}
		case "integer":
			v, err := strconv.Atoi(rawValue)
			if err == nil {
				return v
			}
		case "number":
			v, err := strconv.ParseFloat(rawValue, 64)
			if err == nil {
				return v
			}
		}
	}

	return rawValue
}

// handleSchemaRefs processes and resolves JSON Schema references ($ref) within a schema.
// It handles both direct schema references and references within patternProperties.
// For each reference:
// - If it's a relative file path, it attempts to load and parse the referenced schema
// - If it includes a JSON pointer (#/path/to/schema), it extracts the specific schema section
// - The resolved schema replaces the original reference
//
// Parameters:
//   - schema: Pointer to the Schema object containing the references to resolve
//   - valuesPath: Path to the current values file, used for resolving relative paths
//
// The function will log.Fatal on any critical errors (file not found, invalid JSON, etc.)
// and log.Debug for non-critical issues (e.g., non-relative paths that may be handled elsewhere)
func handleSchemaRefs(schema *Schema, valuesPath string) {
	// Handle main schema $ref
	if schema.Ref != "" {
		refParts := strings.Split(schema.Ref, "#")
		if relFilePath, err := util.IsRelativeFile(valuesPath, refParts[0]); err == nil {
			var relSchema Schema
			file, err := os.Open(relFilePath)
			if err == nil {
				defer file.Close()
				byteValue, _ := io.ReadAll(file)

				if len(refParts) > 1 {
					// Found json-pointer
					var obj interface{}
					json.Unmarshal(byteValue, &obj)
					jsonPointerResultRaw, err := jsonpointer.Get(obj, refParts[1])
					if err != nil {
						log.Fatal(err)
					}
					jsonPointerResultMarshaled, err := json.Marshal(jsonPointerResultRaw)
					if err != nil {
						log.Fatal(err)
					}
					err = json.Unmarshal(jsonPointerResultMarshaled, &relSchema)
					if err != nil {
						log.Fatal(err)
					}
				} else {
					// No json-pointer
					err = json.Unmarshal(byteValue, &relSchema)
					if err != nil {
						log.Fatal(err)
					}
				}
				*schema = relSchema
				schema.HasData = true
			} else {
				log.Fatal(err)
			}
		} else {
			log.Debug(err)
		}
	}

	// Handle $ref in pattern properties
	if schema.PatternProperties != nil {
		for pattern, subSchema := range schema.PatternProperties {
			if subSchema.Ref != "" {
				handleSchemaRefs(subSchema, valuesPath)
				schema.PatternProperties[pattern] = subSchema // Update the original schema in the map
			}
		}
	}
}
