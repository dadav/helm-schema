package schema

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefinitionReferenceRejectsMissingPointer(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "config.json"), []byte(`{"definitions":{"Present":{"type":"string"}}}`), 0o600))
	s := &Schema{Ref: "config.json#/definitions/Missing"}
	require.ErrorContains(t, handleSchemaRefs(s, filepath.Join(root, "values.yaml")), "failed to resolve JSON pointer")
}
