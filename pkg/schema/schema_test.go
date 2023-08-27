package schema

import "testing"

func TestValidate(t *testing.T) {
	tests := []struct {
		comment       string
		expectedValid bool
	}{
		{
			comment: `
# @schema
# multipleOf: true
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
# type: string
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
