package schema

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/magiconair/properties/assert"
	"gopkg.in/yaml.v3"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		comment       string
		expectedValid bool
	}{
		{
			comment: `
# @schema
# multipleOf: 0
# @schema`,
			expectedValid: false,
		},
		{
			comment: `
# @schema
# type: doesnotexist
# @schema`,
			expectedValid: false,
		},
		{
			comment: `
# @schema
# type: [doesnotexist, string]
# @schema`,
			expectedValid: false,
		},
		{
			comment: `
# @schema
# type: [string, integer]
# @schema`,
			expectedValid: true,
		},
		{
			comment: `
# @schema
# type: string
# @schema`,
			expectedValid: true,
		},
		{
			comment: `
# @schema
# const: "hello"
# @schema`,
			expectedValid: true,
		},
		{
			comment: `
# @schema
# const: true
# @schema`,
			expectedValid: true,
		},
		{
			comment: `
# @schema
# const: null
# @schema`,
			expectedValid: true,
		},
		{
			comment: `
# @schema
# format: ipv4
# @schema`,
			expectedValid: true,
		},
		{
			comment: `
# @schema
# pattern: ^foo
# format: ipv4
# @schema`,
			expectedValid: false,
		},
		{
			comment: `
# @schema
# readOnly: true
# @schema`,
			expectedValid: true,
		},
		{
			comment: `
# @schema
# writeOnly: true
# @schema`,
			expectedValid: true,
		},
		{
			comment: `
# @schema
# anyOf:
#   - type: "null"
#   - format: date-time
#   - format: date
# @schema`,
			expectedValid: true,
		},
		{
			comment: `
# @schema
# not:
#   type: "null"
# @schema`,
			expectedValid: true,
		},
		{
			comment: `
# @schema
# anyOf:
#   - type: "null"
#   - format: date-time
# if:
#   type: "null"
# then:
#   description: If set to null, this will do nothing
# else:
#   description: Here goes the description for date-time
# @schema`,
			expectedValid: true,
		},
		{
			comment: `
# @schema
# $ref: https://raw.githubusercontent.com/yannh/kubernetes-json-schema/master/v1.29.2/affinity-v1.json
# @schema`,
			expectedValid: true,
		},
		{
			comment: `
# @schema
# minLength: 1
# maxLength: 0
# @schema`,
			expectedValid: false,
		},
		{
			comment: `
# @schema
# minLength: 1
# maxLength: 2
# @schema`,
			expectedValid: true,
		},
		{
			comment: `
# @schema
# minItems: 1
# maxItems: 2
# @schema`,
			expectedValid: true,
		},
		{
			comment: `
# @schema
# minItems: 2
# maxItems: 1
# @schema`,
			expectedValid: false,
		},
		{
			comment: `
# @schema
# type: string
# minItems: 1
# @schema`,
			expectedValid: false,
		},
		{
			comment: `
# @schema
# type: boolean
# uniqueItems: true
# @schema`,
			expectedValid: true,
		},
	}

	for _, test := range tests {
		schema, _, err := GetSchemaFromComment(test.comment)
		if err != nil && test.expectedValid {
			t.Errorf(
				"Expected the schema %s to be valid=%t, but can't even parse it: %v",
				test.comment,
				test.expectedValid,
				err,
			)
		}
		err = schema.Validate()
		valid := err == nil
		if valid != test.expectedValid {
			t.Errorf(
				"Expected schema\n%s\n\n to be valid=%t, but it's %t",
				test.comment,
				test.expectedValid,
				valid,
			)
		}
	}
}

func TestUnmarshalYAML(t *testing.T) {
	yamlData := `
type: string
x-custom-foo: bar
`

	var schema Schema
	if err := yaml.Unmarshal([]byte(yamlData), &schema); err != nil {
		fmt.Println("Error unmarshaling YAML:", err)
		return
	}
	assert.Equal(t, schema.Type, StringOrArrayOfString{"string"})
	assert.Equal(t, schema.CustomAnnotations["x-custom-foo"], "bar")
}

func TestNewDraft7Keywords(t *testing.T) {
	tests := []struct {
		name          string
		comment       string
		expectedValid bool
	}{
		// Float numeric constraints tests
		{
			name: "minimum with float value",
			comment: `
# @schema
# type: number
# minimum: 1.5
# @schema`,
			expectedValid: true,
		},
		{
			name: "maximum with float value",
			comment: `
# @schema
# type: number
# maximum: 99.9
# @schema`,
			expectedValid: true,
		},
		{
			name: "exclusiveMinimum with float value",
			comment: `
# @schema
# type: number
# exclusiveMinimum: 0.1
# @schema`,
			expectedValid: true,
		},
		{
			name: "exclusiveMaximum with float value",
			comment: `
# @schema
# type: number
# exclusiveMaximum: 100.5
# @schema`,
			expectedValid: true,
		},
		{
			name: "multipleOf with float value",
			comment: `
# @schema
# type: number
# multipleOf: 0.1
# @schema`,
			expectedValid: true,
		},
		{
			name: "minimum greater than maximum should fail",
			comment: `
# @schema
# type: number
# minimum: 10.5
# maximum: 5.5
# @schema`,
			expectedValid: false,
		},
		// $comment keyword
		{
			name: "$comment keyword",
			comment: `
# @schema
# type: string
# $comment: This is a schema comment for developers
# @schema`,
			expectedValid: true,
		},
		// contentEncoding and contentMediaType
		{
			name: "contentEncoding keyword",
			comment: `
# @schema
# type: string
# contentEncoding: base64
# @schema`,
			expectedValid: true,
		},
		{
			name: "contentMediaType keyword",
			comment: `
# @schema
# type: string
# contentMediaType: application/json
# @schema`,
			expectedValid: true,
		},
		{
			name: "contentEncoding with non-string type should fail",
			comment: `
# @schema
# type: integer
# contentEncoding: base64
# @schema`,
			expectedValid: false,
		},
		// contains keyword
		{
			name: "contains keyword with array type",
			comment: `
# @schema
# type: array
# contains:
#   type: string
# @schema`,
			expectedValid: true,
		},
		{
			name: "contains with non-array type should fail",
			comment: `
# @schema
# type: object
# contains:
#   type: string
# @schema`,
			expectedValid: false,
		},
		// additionalItems keyword
		{
			name: "additionalItems as boolean",
			comment: `
# @schema
# type: array
# additionalItems: false
# @schema`,
			expectedValid: true,
		},
		{
			name: "additionalItems as schema",
			comment: `
# @schema
# type: array
# additionalItems:
#   type: string
# @schema`,
			expectedValid: true,
		},
		{
			name: "additionalItems with non-array type should fail",
			comment: `
# @schema
# type: object
# additionalItems: false
# @schema`,
			expectedValid: false,
		},
		// minProperties and maxProperties
		{
			name: "minProperties keyword",
			comment: `
# @schema
# type: object
# minProperties: 1
# @schema`,
			expectedValid: true,
		},
		{
			name: "maxProperties keyword",
			comment: `
# @schema
# type: object
# maxProperties: 10
# @schema`,
			expectedValid: true,
		},
		{
			name: "minProperties greater than maxProperties should fail",
			comment: `
# @schema
# type: object
# minProperties: 10
# maxProperties: 5
# @schema`,
			expectedValid: false,
		},
		{
			name: "minProperties with non-object type should fail",
			comment: `
# @schema
# type: string
# minProperties: 1
# @schema`,
			expectedValid: false,
		},
		// propertyNames keyword
		{
			name: "propertyNames keyword",
			comment: `
# @schema
# type: object
# propertyNames:
#   pattern: ^[a-z]+$
# @schema`,
			expectedValid: true,
		},
		{
			name: "propertyNames with non-object type should fail",
			comment: `
# @schema
# type: array
# propertyNames:
#   pattern: ^[a-z]+$
# @schema`,
			expectedValid: false,
		},
		// dependencies keyword
		{
			name: "dependencies with array of property names",
			comment: `
# @schema
# type: object
# dependencies:
#   bar: ["foo"]
# @schema`,
			expectedValid: true,
		},
		{
			name: "dependencies with schema",
			comment: `
# @schema
# type: object
# dependencies:
#   bar:
#     properties:
#       foo:
#         type: string
# @schema`,
			expectedValid: true,
		},
		// definitions keyword
		{
			name: "definitions keyword",
			comment: `
# @schema
# definitions:
#   address:
#     type: object
#     properties:
#       street:
#         type: string
# @schema`,
			expectedValid: true,
		},
		// Invalid pattern regex
		{
			name: "invalid pattern regex should fail",
			comment: `
# @schema
# type: string
# pattern: "[invalid"
# @schema`,
			expectedValid: false,
		},
		// additionalProperties type check
		{
			name: "additionalProperties with non-object type should fail",
			comment: `
# @schema
# type: string
# additionalProperties: false
# @schema`,
			expectedValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, _, err := GetSchemaFromComment(tt.comment)
			if err != nil {
				if tt.expectedValid {
					t.Errorf("Expected the schema to be valid, but can't even parse it: %v", err)
				}
				return
			}
			err = schema.Validate()
			valid := err == nil
			if valid != tt.expectedValid {
				t.Errorf("Expected schema to be valid=%t, but got valid=%t (error: %v)", tt.expectedValid, valid, err)
			}
		})
	}
}

func TestFloatNumericConstraintsMarshaling(t *testing.T) {
	tests := []struct {
		name         string
		yamlData     string
		expectedJSON string
	}{
		{
			name:         "minimum with float",
			yamlData:     "type: number\nminimum: 1.5",
			expectedJSON: `"minimum": 1.5`,
		},
		{
			name:         "maximum with float",
			yamlData:     "type: number\nmaximum: 99.9",
			expectedJSON: `"maximum": 99.9`,
		},
		{
			name:         "multipleOf with float",
			yamlData:     "type: number\nmultipleOf: 0.01",
			expectedJSON: `"multipleOf": 0.01`,
		},
		{
			name:         "exclusiveMinimum with float",
			yamlData:     "type: number\nexclusiveMinimum: 0.5",
			expectedJSON: `"exclusiveMinimum": 0.5`,
		},
		{
			name:         "exclusiveMaximum with float",
			yamlData:     "type: number\nexclusiveMaximum: 100.5",
			expectedJSON: `"exclusiveMaximum": 100.5`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var schema Schema
			if err := yaml.Unmarshal([]byte(tt.yamlData), &schema); err != nil {
				t.Fatalf("Error unmarshaling YAML: %v", err)
			}

			jsonData, err := schema.ToJson()
			if err != nil {
				t.Fatalf("Error marshaling to JSON: %v", err)
			}

			jsonStr := string(jsonData)
			if !strings.Contains(jsonStr, tt.expectedJSON) {
				t.Errorf("Expected JSON to contain %q, but got:\n%s", tt.expectedJSON, jsonStr)
			}
		})
	}
}

func TestNewKeywordsMarshaling(t *testing.T) {
	tests := []struct {
		name         string
		yamlData     string
		expectedJSON string
	}{
		{
			name:         "$comment keyword",
			yamlData:     "$comment: Test comment",
			expectedJSON: `"$comment": "Test comment"`,
		},
		{
			name:         "contentEncoding keyword",
			yamlData:     "contentEncoding: base64",
			expectedJSON: `"contentEncoding": "base64"`,
		},
		{
			name:         "contentMediaType keyword",
			yamlData:     "contentMediaType: application/json",
			expectedJSON: `"contentMediaType": "application/json"`,
		},
		{
			name:         "minProperties keyword",
			yamlData:     "minProperties: 2",
			expectedJSON: `"minProperties": 2`,
		},
		{
			name:         "maxProperties keyword",
			yamlData:     "maxProperties: 10",
			expectedJSON: `"maxProperties": 10`,
		},
		{
			name:         "contains keyword",
			yamlData:     "type: array\ncontains:\n  type: string",
			expectedJSON: `"contains"`,
		},
		{
			name:         "propertyNames keyword",
			yamlData:     "type: object\npropertyNames:\n  pattern: ^[a-z]+$",
			expectedJSON: `"propertyNames"`,
		},
		{
			name:         "additionalItems as boolean",
			yamlData:     "type: array\nadditionalItems: false",
			expectedJSON: `"additionalItems": false`,
		},
		{
			name:         "definitions keyword",
			yamlData:     "definitions:\n  myDef:\n    type: string",
			expectedJSON: `"definitions"`,
		},
		{
			name:         "dependencies keyword",
			yamlData:     "dependencies:\n  bar: [\"foo\"]",
			expectedJSON: `"dependencies"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var schema Schema
			if err := yaml.Unmarshal([]byte(tt.yamlData), &schema); err != nil {
				t.Fatalf("Error unmarshaling YAML: %v", err)
			}

			jsonData, err := schema.ToJson()
			if err != nil {
				t.Fatalf("Error marshaling to JSON: %v", err)
			}

			jsonStr := string(jsonData)
			if !strings.Contains(jsonStr, tt.expectedJSON) {
				t.Errorf("Expected JSON to contain %q, but got:\n%s", tt.expectedJSON, jsonStr)
			}
		})
	}
}

func TestDisableRequiredPropertiesWithNewFields(t *testing.T) {
	// Test that DisableRequiredProperties works with all new nested schema fields
	schema := Schema{
		Type: StringOrArrayOfString{"object"},
		Required: BoolOrArrayOfString{
			Strings: []string{"foo"},
		},
		Contains: &Schema{
			Required: BoolOrArrayOfString{Strings: []string{"inner"}},
		},
		PropertyNames: &Schema{
			Required: BoolOrArrayOfString{Strings: []string{"name"}},
		},
		Definitions: map[string]*Schema{
			"myDef": {
				Required: BoolOrArrayOfString{Strings: []string{"defProp"}},
			},
		},
	}

	schema.DisableRequiredProperties()

	if len(schema.Required.Strings) != 0 {
		t.Error("Expected root required to be empty")
	}
	if len(schema.Contains.Required.Strings) != 0 {
		t.Error("Expected Contains required to be empty")
	}
	if len(schema.PropertyNames.Required.Strings) != 0 {
		t.Error("Expected PropertyNames required to be empty")
	}
	if len(schema.Definitions["myDef"].Required.Strings) != 0 {
		t.Error("Expected Definitions[myDef] required to be empty")
	}
}

func TestDefsToDefinitionsConversion(t *testing.T) {
	// Test that $defs is converted to definitions when unmarshaling JSON
	tests := []struct {
		name         string
		jsonInput    string
		expectDefs   bool
		expectedKeys []string
	}{
		{
			name: "$defs is converted to definitions",
			jsonInput: `{
				"$defs": {
					"MyType": {"type": "string"}
				}
			}`,
			expectDefs:   true,
			expectedKeys: []string{"MyType"},
		},
		{
			name: "definitions is preserved",
			jsonInput: `{
				"definitions": {
					"OtherType": {"type": "integer"}
				}
			}`,
			expectDefs:   true,
			expectedKeys: []string{"OtherType"},
		},
		{
			name: "$defs and definitions are merged",
			jsonInput: `{
				"$defs": {
					"FromDefs": {"type": "string"}
				},
				"definitions": {
					"FromDefinitions": {"type": "integer"}
				}
			}`,
			expectDefs:   true,
			expectedKeys: []string{"FromDefs", "FromDefinitions"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var schema Schema
			if err := json.Unmarshal([]byte(tt.jsonInput), &schema); err != nil {
				t.Fatalf("Failed to unmarshal JSON: %v", err)
			}

			if tt.expectDefs && schema.Definitions == nil {
				t.Error("Expected Definitions to be non-nil")
				return
			}

			for _, key := range tt.expectedKeys {
				if _, ok := schema.Definitions[key]; !ok {
					t.Errorf("Expected key %q in Definitions", key)
				}
			}
		})
	}
}

func TestRefPathRewriting(t *testing.T) {
	// Test that $ref paths are rewritten from #/$defs/ to #/definitions/
	tests := []struct {
		name        string
		jsonInput   string
		expectedRef string
	}{
		{
			name: "$ref with $defs path is rewritten",
			jsonInput: `{
				"$ref": "#/$defs/MyType"
			}`,
			expectedRef: "#/definitions/MyType",
		},
		{
			name: "$ref with definitions path is preserved",
			jsonInput: `{
				"$ref": "#/definitions/MyType"
			}`,
			expectedRef: "#/definitions/MyType",
		},
		{
			name: "nested $ref paths are rewritten",
			jsonInput: `{
				"properties": {
					"foo": {
						"$ref": "#/$defs/FooType"
					}
				}
			}`,
			expectedRef: "#/definitions/FooType",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var schema Schema
			if err := json.Unmarshal([]byte(tt.jsonInput), &schema); err != nil {
				t.Fatalf("Failed to unmarshal JSON: %v", err)
			}

			// Check main ref
			if schema.Ref != "" && schema.Ref != tt.expectedRef {
				t.Errorf("Expected Ref to be %q, got %q", tt.expectedRef, schema.Ref)
			}

			// Check nested ref in properties
			if schema.Properties != nil {
				for _, prop := range schema.Properties {
					if prop.Ref != "" && prop.Ref != tt.expectedRef {
						t.Errorf("Expected nested Ref to be %q, got %q", tt.expectedRef, prop.Ref)
					}
				}
			}
		})
	}
}

func TestConstNullMarshaling(t *testing.T) {
	tests := []struct {
		name          string
		yamlData      string
		expectedJSON  string
		shouldContain bool
	}{
		{
			name:          "const with null value should be preserved",
			yamlData:      "const: null",
			expectedJSON:  `"const": null`,
			shouldContain: true,
		},
		{
			name:          "const with false value should be preserved",
			yamlData:      "const: false",
			expectedJSON:  `"const": false`,
			shouldContain: true,
		},
		{
			name:          "const with true value should be preserved",
			yamlData:      "const: true",
			expectedJSON:  `"const": true`,
			shouldContain: true,
		},
		{
			name:          "const with string value should be preserved",
			yamlData:      `const: "test"`,
			expectedJSON:  `"const": "test"`,
			shouldContain: true,
		},
		{
			name:          "schema without const should not have const field",
			yamlData:      "type: string",
			expectedJSON:  `"const"`,
			shouldContain: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var schema Schema
			if err := yaml.Unmarshal([]byte(tt.yamlData), &schema); err != nil {
				t.Fatalf("Error unmarshaling YAML: %v", err)
			}

			jsonData, err := schema.ToJson()
			if err != nil {
				t.Fatalf("Error marshaling to JSON: %v", err)
			}

			jsonStr := string(jsonData)
			contains := strings.Contains(jsonStr, tt.expectedJSON)

			if tt.shouldContain && !contains {
				t.Errorf("Expected JSON to contain %q, but got:\n%s", tt.expectedJSON, jsonStr)
			}
			if !tt.shouldContain && contains {
				t.Errorf("Expected JSON to NOT contain %q, but got:\n%s", tt.expectedJSON, jsonStr)
			}
		})
	}
}

func TestHoistDefinitions(t *testing.T) {
	// Create a schema with nested definitions
	restConfig := &Schema{
		Type:        StringOrArrayOfString{"object"},
		Title:       "RestConfig",
		Description: "REST API configuration",
		Properties: map[string]*Schema{
			"url": {
				Type:   StringOrArrayOfString{"string"},
				Format: "uri",
			},
		},
	}

	workerSchema := &Schema{
		Type:        StringOrArrayOfString{"object"},
		Title:       "Worker",
		Description: "Worker configuration",
		Definitions: map[string]*Schema{
			"RestConfig": restConfig,
		},
		Properties: map[string]*Schema{
			"api": {
				Ref: "#/definitions/RestConfig",
			},
		},
	}

	rootSchema := &Schema{
		Schema: "http://json-schema.org/draft-07/schema#",
		Type:   StringOrArrayOfString{"object"},
		Properties: map[string]*Schema{
			"worker": workerSchema,
		},
	}

	// Verify definitions are nested before hoisting
	if rootSchema.Definitions != nil && len(rootSchema.Definitions) > 0 {
		t.Error("Root should not have definitions before hoisting")
	}
	if workerSchema.Definitions == nil || len(workerSchema.Definitions) == 0 {
		t.Error("Worker should have definitions before hoisting")
	}

	// Hoist definitions
	rootSchema.HoistDefinitions()

	// Verify definitions are at root after hoisting
	if rootSchema.Definitions == nil || len(rootSchema.Definitions) == 0 {
		t.Error("Root should have definitions after hoisting")
	}
	if _, ok := rootSchema.Definitions["RestConfig"]; !ok {
		t.Error("Root should have RestConfig definition after hoisting")
	}

	// Verify definitions are removed from nested schema
	if workerSchema.Definitions != nil && len(workerSchema.Definitions) > 0 {
		t.Error("Worker should not have definitions after hoisting")
	}

	// Verify the $ref still points to the correct location
	if rootSchema.Properties["worker"].Properties["api"].Ref != "#/definitions/RestConfig" {
		t.Error("$ref should still point to #/definitions/RestConfig")
	}

	// Verify the hoisted definition is correct
	if rootSchema.Definitions["RestConfig"].Title != "RestConfig" {
		t.Errorf("Hoisted definition should have correct title, got %s", rootSchema.Definitions["RestConfig"].Title)
	}
}
