package schema

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefinitionPrefixPreservesSourceAndAnnotations(t *testing.T) {
	source := &Schema{
		Definitions: map[string]*Schema{"a/b~c": {Type: []string{"string"}}},
		Properties: map[string]*Schema{"value": {
			Ref:         "#/definitions/a~1b~0c",
			HasData:     true,
			constWasSet: true,
			Default:     map[string]interface{}{"$ref": "#/definitions/a~1b~0c"},
		}},
		AdditionalProperties: map[string]interface{}{"$ref": "#/definitions/a~1b~0c"},
	}
	first := source.WithDefinitionPrefix("first")
	second := source.WithDefinitionPrefix("second")
	require.Equal(t, "#/definitions/first~1a~1b~0c", first.Properties["value"].Ref)
	require.Equal(t, "#/definitions/second~1a~1b~0c", second.Properties["value"].Ref)
	require.Equal(t, "#/definitions/a~1b~0c", source.Properties["value"].Ref)
	require.Equal(t, "#/definitions/first~1a~1b~0c", first.AdditionalProperties.(map[string]interface{})["$ref"])
	require.Equal(t, "#/definitions/a~1b~0c", source.AdditionalProperties.(map[string]interface{})["$ref"])
	require.True(t, first.Properties["value"].HasData)
	require.True(t, first.Properties["value"].constWasSet)
	require.Equal(t, source.Properties["value"].Default, first.Properties["value"].Default)
}
