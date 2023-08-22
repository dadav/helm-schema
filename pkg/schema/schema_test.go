package schema

import "testing"

func TestValidate(t *testing.T) {
	tests := []struct {
		comment       string
		expectedValid bool
	}{
		{
			comment:       "foo",
			expectedValid: true,
		},
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
				"Expected schema %s to be valid=%t, but it's %t",
				test.comment,
				test.expectedValid,
				valid,
			)
		}
	}
}
