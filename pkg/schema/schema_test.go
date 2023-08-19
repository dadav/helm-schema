package schema

import "testing"

func TestValidate(t *testing.T) {
	tests := []struct {
		comment       string
		expectedValid bool
	}{
		{
			comment:       "foo",
			expectedValid: false,
		},
		{
			comment:       "# @schema type=string",
			expectedValid: true,
		},
		{
			comment:       "# @schema type=string type=integer",
			expectedValid: false,
		},
		{
			comment:       "# @schema type=string min=1",
			expectedValid: false,
		},
		{
			comment:       "# @schema type=object min=1",
			expectedValid: false,
		},
		{
			comment:       "# @schema type=doesntexist",
			expectedValid: false,
		},
	}

	for _, test := range tests {
		schemas, _, err := GetSchemasFromComment(test.comment)
		if err != nil && test.expectedValid {
			t.Errorf(
				"Expected the schema %s to be valid=%t, but can't even parse it: %v",
				test.comment,
				test.expectedValid,
				err,
			)
		}
		for i, schema := range schemas {
			err := schema.Validate()
			valid := err == nil
			if valid != test.expectedValid {
				t.Errorf(
					"Expected schema %s (number %d) to be valid=%t, but it's %t",
					test.comment,
					i,
					test.expectedValid,
					valid,
				)
			}
		}
	}
}
