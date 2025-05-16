package schema

import (
	"fmt"
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
# type: string
# uniqueItems: true
# @schema`,
			expectedValid: true,
		},
		{
			comment: `
# @schema
# type: string
# uniqueItems: false
# @schema`,
			expectedValid: false,
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
