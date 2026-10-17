package schema

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestValidateFormatAnnotations(t *testing.T) {
	for _, s := range []Schema{
		{Type: []string{"integer", "null"}, Format: "int64"},
		{Type: []string{"string"}, Format: "custom-format"},
		{Type: []string{"string"}, Format: "hostname", Pattern: "^host"},
	} {
		require.NoError(t, s.Validate())
	}
}
