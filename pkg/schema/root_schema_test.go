package schema

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestGetRootSchemaFromComment(t *testing.T) {
	tests := []struct {
		name                 string
		comment              string
		expectedHasData      bool
		expectedTitle        string
		expectedDescription  string
		expectedRemaining    string
		expectError          bool
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
			expectedHasData:   false,
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
		name                  string
		yamlContent           string
		expectedTitle         string
		expectedDescription   string
		expectedAdditionalProp interface{}
		expectedCustomField   interface{}
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
			schema := YamlToSchema("", &node, false, false, false, true, skipConfig, nil)

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
	schema := YamlToSchema("", &node, false, false, false, true, skipConfig, nil)

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
