package schema

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestGetRootSchemaFromComment(t *testing.T) {
	tests := []struct {
		name                string
		comment             string
		expectedHasData     bool
		expectedTitle       string
		expectedDescription string
		expectedRemaining   string
		expectError         bool
	}{
		{
			name: "simple root schema",
			comment: `# @schema.root
# title: My Chart
# description: A chart for testing
# @schema.root
# This is a key comment
key: value`,
			expectedHasData:     true,
			expectedTitle:       "My Chart",
			expectedDescription: "A chart for testing",
			expectedRemaining: `# This is a key comment
key: value`,
		},
		{
			name: "root schema with custom annotations",
			comment: `# @schema.root
# title: Advanced Chart
# x-custom-field: custom-value
# additionalProperties: true
# @schema.root
# Key description`,
			expectedHasData:     true,
			expectedTitle:       "Advanced Chart",
			expectedDescription: "",
			expectedRemaining:   `# Key description`,
		},
		{
			name: "no root schema",
			comment: `# @schema
# type: string
# @schema
# Just a regular comment`,
			expectedHasData: false,
			expectedRemaining: `# @schema
# type: string
# @schema
# Just a regular comment`,
		},
		{
			name: "unclosed root schema block",
			comment: `# @schema.root
# title: Unclosed
# This will fail`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, remaining, err := GetRootSchemaFromComment(tt.comment)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected an error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if schema.HasData != tt.expectedHasData {
				t.Errorf("Expected HasData=%v, got %v", tt.expectedHasData, schema.HasData)
			}

			if tt.expectedHasData {
				if schema.Title != tt.expectedTitle {
					t.Errorf("Expected Title=%q, got %q", tt.expectedTitle, schema.Title)
				}
				if schema.Description != tt.expectedDescription {
					t.Errorf("Expected Description=%q, got %q", tt.expectedDescription, schema.Description)
				}
			}

			if strings.TrimSpace(remaining) != strings.TrimSpace(tt.expectedRemaining) {
				t.Errorf("Expected remaining=%q, got %q", tt.expectedRemaining, remaining)
			}
		})
	}
}

func TestYamlToSchemaWithRootAnnotations(t *testing.T) {
	tests := []struct {
		name                   string
		yamlContent            string
		expectedTitle          string
		expectedDescription    string
		expectedAdditionalProp interface{}
		expectedCustomField    interface{}
	}{
		{
			name: "basic root schema",
			yamlContent: `# @schema.root
# title: Test Chart Values
# description: Test description
# @schema.root
foo: bar`,
			expectedTitle:       "Test Chart Values",
			expectedDescription: "Test description",
		},
		{
			name: "root schema with additionalProperties",
			yamlContent: `# @schema.root
# title: Flexible Chart
# additionalProperties: true
# @schema.root
service:
  port: 8080`,
			expectedTitle:          "Flexible Chart",
			expectedAdditionalProp: true,
		},
		{
			name: "root schema with custom annotations",
			yamlContent: `# @schema.root
# title: Custom Chart
# x-helm-version: "3.0"
# @schema.root
app: myapp`,
			expectedTitle:       "Custom Chart",
			expectedCustomField: "3.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var node yaml.Node
			err := yaml.Unmarshal([]byte(tt.yamlContent), &node)
			if err != nil {
				t.Fatalf("Failed to unmarshal YAML: %v", err)
			}

			skipConfig := &SkipAutoGenerationConfig{}
			schema := YamlToSchema("", &node, false, false, false, true, skipConfig, nil, nil)

			if schema.Title != tt.expectedTitle {
				t.Errorf("Expected Title=%q, got %q", tt.expectedTitle, schema.Title)
			}

			if tt.expectedDescription != "" && schema.Description != tt.expectedDescription {
				t.Errorf("Expected Description=%q, got %q", tt.expectedDescription, schema.Description)
			}

			if tt.expectedAdditionalProp != nil {
				if schema.AdditionalProperties == nil {
					t.Errorf("Expected AdditionalProperties=%v, got nil", tt.expectedAdditionalProp)
				} else if schema.AdditionalProperties != tt.expectedAdditionalProp {
					t.Errorf("Expected AdditionalProperties=%v, got %v", tt.expectedAdditionalProp, schema.AdditionalProperties)
				}
			}

			if tt.expectedCustomField != nil {
				if schema.CustomAnnotations == nil {
					t.Errorf("Expected CustomAnnotations to contain x-helm-version, but CustomAnnotations is nil")
				} else if val, ok := schema.CustomAnnotations["x-helm-version"]; !ok {
					t.Errorf("Expected CustomAnnotations to contain x-helm-version")
				} else if val != tt.expectedCustomField {
					t.Errorf("Expected x-helm-version=%v, got %v", tt.expectedCustomField, val)
				}
			}
		})
	}
}

func TestRootSchemaDoesNotAffectKeyAnnotations(t *testing.T) {
	yamlContent := `# @schema.root
# title: Root Title
# description: Root description
# @schema.root
# @schema
# enum: [dev, prod]
# @schema
# -- Environment setting
environment: dev

service:
  port: 8080`

	var node yaml.Node
	err := yaml.Unmarshal([]byte(yamlContent), &node)
	if err != nil {
		t.Fatalf("Failed to unmarshal YAML: %v", err)
	}

	skipConfig := &SkipAutoGenerationConfig{}
	schema := YamlToSchema("", &node, false, false, false, true, skipConfig, nil, nil)

	// Check root schema
	if schema.Title != "Root Title" {
		t.Errorf("Expected root Title=%q, got %q", "Root Title", schema.Title)
	}
	if schema.Description != "Root description" {
		t.Errorf("Expected root Description=%q, got %q", "Root description", schema.Description)
	}

	// Check that environment key still has its own annotations
	if schema.Properties == nil {
		t.Fatal("Expected Properties to be set")
	}

	envSchema, ok := schema.Properties["environment"]
	if !ok {
		t.Fatal("Expected environment property to exist")
	}

	if len(envSchema.Enum) != 2 {
		t.Errorf("Expected 2 enum values, got %d", len(envSchema.Enum))
	}
	if envSchema.Description != "Environment setting" {
		t.Errorf("Expected environment Description=%q, got %q", "Environment setting", envSchema.Description)
	}
}

func TestDefinitionsPropagationFromExternalSchema(t *testing.T) {
	tests := []struct {
		name                string
		yamlContent         string
		externalSchemaFile  string
		externalSchemaJSON  string
		expectedDefsCount   int
		expectedDefName     string
		useDefinitionsKeywd bool // true = "definitions", false = "$defs"
	}{
		{
			name: "propagate $defs from external schema",
			yamlContent: `# @schema
# $ref: ./external.json#/$defs/baseService
# @schema
service:
  port: 8080`,
			externalSchemaFile: "external.json",
			externalSchemaJSON: `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "$defs": {
    "baseService": {
      "type": "object",
      "properties": {
        "enabled": {"type": "boolean"}
      }
    },
    "anotherDef": {
      "type": "string"
    }
  }
}`,
			expectedDefsCount:   2,
			expectedDefName:     "baseService",
			useDefinitionsKeywd: false,
		},
		{
			name: "propagate definitions from external schema",
			yamlContent: `# @schema
# $ref: ./legacy.json#/definitions/legacyService
# @schema
service:
  port: 8080`,
			externalSchemaFile: "legacy.json",
			externalSchemaJSON: `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "definitions": {
    "legacyService": {
      "type": "object",
      "properties": {
        "enabled": {"type": "boolean"}
      }
    },
    "legacyType": {
      "type": "string"
    }
  }
}`,
			expectedDefsCount:   2,
			expectedDefName:     "legacyService",
			useDefinitionsKeywd: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory and external schema file
			tmpDir := t.TempDir()
			externalSchemaPath := tmpDir + "/" + tt.externalSchemaFile
			err := os.WriteFile(externalSchemaPath, []byte(tt.externalSchemaJSON), 0644)
			if err != nil {
				t.Fatalf("Failed to create external schema file: %v", err)
			}

			// Create temporary values file in the same directory
			valuesPath := tmpDir + "/values.yaml"
			err = os.WriteFile(valuesPath, []byte(tt.yamlContent), 0644)
			if err != nil {
				t.Fatalf("Failed to create values file: %v", err)
			}

			var node yaml.Node
			err = yaml.Unmarshal([]byte(tt.yamlContent), &node)
			if err != nil {
				t.Fatalf("Failed to unmarshal YAML: %v", err)
			}

			skipConfig := &SkipAutoGenerationConfig{}
			schema := YamlToSchema(valuesPath, &node, false, false, false, true, skipConfig, nil, nil)

			// Check if definitions were propagated
			if tt.useDefinitionsKeywd {
				if schema.Definitions == nil {
					t.Errorf("Expected Definitions to be set, got nil")
				} else if len(schema.Definitions) != tt.expectedDefsCount {
					t.Errorf("Expected %d definitions, got %d", tt.expectedDefsCount, len(schema.Definitions))
				} else if _, exists := schema.Definitions[tt.expectedDefName]; !exists {
					t.Errorf("Expected definition %q to exist in Definitions", tt.expectedDefName)
				}
			} else {
				if schema.Defs == nil {
					t.Errorf("Expected $defs to be set, got nil")
				} else if len(schema.Defs) != tt.expectedDefsCount {
					t.Errorf("Expected %d $defs, got %d", tt.expectedDefsCount, len(schema.Defs))
				} else if _, exists := schema.Defs[tt.expectedDefName]; !exists {
					t.Errorf("Expected definition %q to exist in $defs", tt.expectedDefName)
				}
			}
		})
	}
}

func TestCheckUsesDefinitions(t *testing.T) {
	tests := []struct {
		name     string
		schema   *Schema
		expected bool
	}{
		{
			name: "schema with #/definitions/ ref",
			schema: &Schema{
				Ref: "#/definitions/myType",
			},
			expected: true,
		},
		{
			name: "schema with #/$defs/ ref",
			schema: &Schema{
				Ref: "#/$defs/myType",
			},
			expected: false,
		},
		{
			name: "schema with nested definitions ref in properties",
			schema: &Schema{
				Properties: map[string]*Schema{
					"foo": {
						Ref: "#/definitions/fooType",
					},
				},
			},
			expected: true,
		},
		{
			name: "schema with definitions ref in allOf",
			schema: &Schema{
				AllOf: []*Schema{
					{
						Ref: "#/definitions/baseType",
					},
				},
			},
			expected: true,
		},
		{
			name: "schema with definitions ref in items",
			schema: &Schema{
				Items: &Schema{
					Ref: "#/definitions/itemType",
				},
			},
			expected: true,
		},
		{
			name: "schema without any refs",
			schema: &Schema{
				Type: []string{"object"},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checkUsesDefinitions(tt.schema)
			if result != tt.expected {
				t.Errorf("Expected checkUsesDefinitions=%v, got %v", tt.expected, result)
			}
		})
	}
}

func TestRootSchemaAnnotationsPropagation(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		checkField  string
		checkValue  interface{}
	}{
		{
			name: "root schema with Ref propagation",
			yamlContent: `# @schema.root
# $ref: "#/$defs/commonConfig"
# @schema.root
foo: bar`,
			checkField: "Ref",
			checkValue: "#/$defs/commonConfig",
		},
		{
			name: "root schema with Examples propagation",
			yamlContent: `# @schema.root
# examples:
#   - name: example1
# @schema.root
foo: bar`,
			checkField: "Examples",
			checkValue: 1, // count of examples
		},
		{
			name: "root schema with Deprecated propagation",
			yamlContent: `# @schema.root
# deprecated: true
# @schema.root
foo: bar`,
			checkField: "Deprecated",
			checkValue: true,
		},
		{
			name: "root schema with ReadOnly propagation",
			yamlContent: `# @schema.root
# readOnly: true
# @schema.root
foo: bar`,
			checkField: "ReadOnly",
			checkValue: true,
		},
		{
			name: "root schema with WriteOnly propagation",
			yamlContent: `# @schema.root
# writeOnly: true
# @schema.root
foo: bar`,
			checkField: "WriteOnly",
			checkValue: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var node yaml.Node
			err := yaml.Unmarshal([]byte(tt.yamlContent), &node)
			if err != nil {
				t.Fatalf("Failed to unmarshal YAML: %v", err)
			}

			skipConfig := &SkipAutoGenerationConfig{}
			schema := YamlToSchema("", &node, false, false, false, true, skipConfig, nil, nil)

			switch tt.checkField {
			case "Ref":
				if schema.Ref != tt.checkValue.(string) {
					t.Errorf("Expected Ref=%q, got %q", tt.checkValue.(string), schema.Ref)
				}
			case "Examples":
				if len(schema.Examples) != tt.checkValue.(int) {
					t.Errorf("Expected %d examples, got %d", tt.checkValue.(int), len(schema.Examples))
				}
			case "Deprecated":
				if schema.Deprecated != tt.checkValue.(bool) {
					t.Errorf("Expected Deprecated=%v, got %v", tt.checkValue.(bool), schema.Deprecated)
				}
			case "ReadOnly":
				if schema.ReadOnly != tt.checkValue.(bool) {
					t.Errorf("Expected ReadOnly=%v, got %v", tt.checkValue.(bool), schema.ReadOnly)
				}
			case "WriteOnly":
				if schema.WriteOnly != tt.checkValue.(bool) {
					t.Errorf("Expected WriteOnly=%v, got %v", tt.checkValue.(bool), schema.WriteOnly)
				}
			}
		})
	}
}
