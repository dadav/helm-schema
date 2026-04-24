package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

type stringOrArray []string

func (s *stringOrArray) UnmarshalJSON(value []byte) error {
	if len(value) == 0 {
		return nil
	}
	if value[0] == '"' {
		var single string
		if err := json.Unmarshal(value, &single); err != nil {
			return err
		}
		*s = []string{single}
		return nil
	}
	var multi []string
	if err := json.Unmarshal(value, &multi); err != nil {
		return err
	}
	*s = multi
	return nil
}

type schemaDoc struct {
	Properties map[string]schemaProperty `json:"properties"`
}

type schemaProperty struct {
	Type       stringOrArray              `json:"type"`
	Properties map[string]schemaProperty  `json:"properties"`
}

func TestExec_ConditionPatchingAndRootConditions(t *testing.T) {
	tmpDir := t.TempDir()

	writeFile := func(relPath, content string) {
		path := filepath.Join(tmpDir, relPath)
		err := os.MkdirAll(filepath.Dir(path), 0o755)
		assert.NoError(t, err)
		err = os.WriteFile(path, []byte(content), 0o644)
		assert.NoError(t, err)
	}

	writeFile("dep/Chart.yaml", `
apiVersion: v2
name: dep
version: 1.0.0
`)
	writeFile("dep/values.yaml", `
key: value
`)

	writeFile("dep2/Chart.yaml", `
apiVersion: v2
name: dep2
version: 1.0.0
`)
	writeFile("dep2/values.yaml", `
key: value
`)

	writeFile("parent1/Chart.yaml", `
apiVersion: v2
name: parent1
version: 1.0.0
dependencies:
  - name: dep
    version: 1.0.0
    condition: dep.enabled
  - name: dep2
    version: 1.0.0
    condition: dep2
`)
	writeFile("parent1/values.yaml", `
root: value
`)

	writeFile("parent2/Chart.yaml", `
apiVersion: v2
name: parent2
version: 1.0.0
dependencies:
  - name: dep
    version: 1.0.0
    condition: dep.extra.flag
`)
	writeFile("parent2/values.yaml", `
root: value
`)

	viper.Reset()
	viper.Set("chart-search-root", tmpDir)
	viper.Set("dry-run", false)
	viper.Set("no-dependencies", false)
	viper.Set("add-schema-reference", false)
	viper.Set("keep-full-comment", false)
	viper.Set("helm-docs-compatibility-mode", false)
	viper.Set("uncomment", false)
	viper.Set("output-file", "values.schema.json")
	viper.Set("dont-strip-helm-docs-prefix", false)
	viper.Set("append-newline", false)
	viper.Set("dependencies-filter", []string{})
	viper.Set("dont-add-global", true)
	viper.Set("skip-dependencies-schema-validation", false)
	viper.Set("allow-circular-dependencies", false)
	viper.Set("annotate", false)
	viper.Set("keep-existing-dep-schemas", false)
	viper.Set("value-files", []string{"values.yaml"})
	viper.Set("skip-auto-generation", []string{})
	viper.Set("log-level", "info")

	err := exec(nil, nil)
	assert.NoError(t, err)

	depSchemaPath := filepath.Join(tmpDir, "dep", "values.schema.json")
	depSchemaBytes, err := os.ReadFile(depSchemaPath)
	assert.NoError(t, err)

	var depSchema schemaDoc
	err = json.Unmarshal(depSchemaBytes, &depSchema)
	assert.NoError(t, err)

	enabledProp, ok := depSchema.Properties["enabled"]
	assert.True(t, ok)
	assert.Contains(t, []string(enabledProp.Type), "boolean")

	extraProp, ok := depSchema.Properties["extra"]
	assert.True(t, ok)
	flagProp, ok := extraProp.Properties["flag"]
	assert.True(t, ok)
	assert.Contains(t, []string(flagProp.Type), "boolean")

	parent1SchemaPath := filepath.Join(tmpDir, "parent1", "values.schema.json")
	parent1SchemaBytes, err := os.ReadFile(parent1SchemaPath)
	assert.NoError(t, err)

	var parent1Schema schemaDoc
	err = json.Unmarshal(parent1SchemaBytes, &parent1Schema)
	assert.NoError(t, err)

	dep2Prop, ok := parent1Schema.Properties["dep2"]
	assert.True(t, ok)
	assert.Contains(t, []string(dep2Prop.Type), "object")
	assert.Contains(t, []string(dep2Prop.Type), "boolean")
}

func TestExec_DependencyFilterSkipsConditionPatching(t *testing.T) {
	tmpDir := t.TempDir()

	writeFile := func(relPath, content string) {
		path := filepath.Join(tmpDir, relPath)
		err := os.MkdirAll(filepath.Dir(path), 0o755)
		assert.NoError(t, err)
		err = os.WriteFile(path, []byte(content), 0o644)
		assert.NoError(t, err)
	}

	writeFile("dep/Chart.yaml", `
apiVersion: v2
name: dep
version: 1.0.0
`)
	writeFile("dep/values.yaml", `
key: value
`)

	writeFile("dep2/Chart.yaml", `
apiVersion: v2
name: dep2
version: 1.0.0
`)
	writeFile("dep2/values.yaml", `
key: value
`)

	writeFile("Chart.yaml", `
apiVersion: v2
name: parent
version: 1.0.0
dependencies:
  - name: dep
    version: 1.0.0
    condition: dep.enabled
  - name: dep2
    version: 1.0.0
    condition: dep2.flag
`)
	writeFile("values.yaml", `
root: value
`)

	viper.Reset()
	viper.Set("chart-search-root", tmpDir)
	viper.Set("dry-run", false)
	viper.Set("no-dependencies", false)
	viper.Set("add-schema-reference", false)
	viper.Set("keep-full-comment", false)
	viper.Set("helm-docs-compatibility-mode", false)
	viper.Set("uncomment", false)
	viper.Set("output-file", "values.schema.json")
	viper.Set("dont-strip-helm-docs-prefix", false)
	viper.Set("append-newline", false)
	viper.Set("dependencies-filter", []string{"dep"})
	viper.Set("dont-add-global", true)
	viper.Set("skip-dependencies-schema-validation", false)
	viper.Set("allow-circular-dependencies", false)
	viper.Set("annotate", false)
	viper.Set("keep-existing-dep-schemas", false)
	viper.Set("value-files", []string{"values.yaml"})
	viper.Set("skip-auto-generation", []string{})
	viper.Set("log-level", "info")

	err := exec(nil, nil)
	assert.NoError(t, err)

	parentSchemaPath := filepath.Join(tmpDir, "values.schema.json")
	parentSchemaBytes, err := os.ReadFile(parentSchemaPath)
	assert.NoError(t, err)

	var parentSchema schemaDoc
	err = json.Unmarshal(parentSchemaBytes, &parentSchema)
	assert.NoError(t, err)

	_, hasDep := parentSchema.Properties["dep"]
	_, hasDep2 := parentSchema.Properties["dep2"]
	assert.True(t, hasDep)
	assert.False(t, hasDep2)

	depSchemaPath := filepath.Join(tmpDir, "dep", "values.schema.json")
	depSchemaBytes, err := os.ReadFile(depSchemaPath)
	assert.NoError(t, err)

	var depSchema schemaDoc
	err = json.Unmarshal(depSchemaBytes, &depSchema)
	assert.NoError(t, err)

	enabledProp, ok := depSchema.Properties["enabled"]
	assert.True(t, ok)
	assert.Contains(t, []string(enabledProp.Type), "boolean")

	dep2SchemaPath := filepath.Join(tmpDir, "dep2", "values.schema.json")
	_, err = os.Stat(dep2SchemaPath)
	assert.True(t, os.IsNotExist(err))
}

func TestExec_DependencyAliasConditionPatching(t *testing.T) {
	tmpDir := t.TempDir()

	writeFile := func(relPath, content string) {
		path := filepath.Join(tmpDir, relPath)
		err := os.MkdirAll(filepath.Dir(path), 0o755)
		assert.NoError(t, err)
		err = os.WriteFile(path, []byte(content), 0o644)
		assert.NoError(t, err)
	}

	writeFile("dep/Chart.yaml", `
apiVersion: v2
name: dep
version: 1.0.0
`)
	writeFile("dep/values.yaml", `
key: value
`)

	writeFile("Chart.yaml", `
apiVersion: v2
name: parent
version: 1.0.0
dependencies:
  - name: dep
    alias: depalias
    version: 1.0.0
    condition: depalias.enabled
`)
	writeFile("values.yaml", `
root: value
`)

	viper.Reset()
	viper.Set("chart-search-root", tmpDir)
	viper.Set("dry-run", false)
	viper.Set("no-dependencies", false)
	viper.Set("add-schema-reference", false)
	viper.Set("keep-full-comment", false)
	viper.Set("helm-docs-compatibility-mode", false)
	viper.Set("uncomment", false)
	viper.Set("output-file", "values.schema.json")
	viper.Set("dont-strip-helm-docs-prefix", false)
	viper.Set("append-newline", false)
	viper.Set("dependencies-filter", []string{})
	viper.Set("dont-add-global", true)
	viper.Set("skip-dependencies-schema-validation", false)
	viper.Set("allow-circular-dependencies", false)
	viper.Set("annotate", false)
	viper.Set("keep-existing-dep-schemas", false)
	viper.Set("value-files", []string{"values.yaml"})
	viper.Set("skip-auto-generation", []string{})
	viper.Set("log-level", "info")

	err := exec(nil, nil)
	assert.NoError(t, err)

	depSchemaPath := filepath.Join(tmpDir, "dep", "values.schema.json")
	depSchemaBytes, err := os.ReadFile(depSchemaPath)
	assert.NoError(t, err)

	var depSchema schemaDoc
	err = json.Unmarshal(depSchemaBytes, &depSchema)
	assert.NoError(t, err)

	enabledProp, ok := depSchema.Properties["enabled"]
	assert.True(t, ok)
	assert.Contains(t, []string(enabledProp.Type), "boolean")

	parentSchemaPath := filepath.Join(tmpDir, "values.schema.json")
	parentSchemaBytes, err := os.ReadFile(parentSchemaPath)
	assert.NoError(t, err)

	var parentSchema schemaDoc
	err = json.Unmarshal(parentSchemaBytes, &parentSchema)
	assert.NoError(t, err)

	_, ok = parentSchema.Properties["depalias"]
	assert.True(t, ok)
}

func TestExec_KeepExistingDepSchemasPreservesDependencySchema(t *testing.T) {
	tmpDir := t.TempDir()

	writeFile := func(relPath, content string) {
		path := filepath.Join(tmpDir, relPath)
		err := os.MkdirAll(filepath.Dir(path), 0o755)
		assert.NoError(t, err)
		err = os.WriteFile(path, []byte(content), 0o644)
		assert.NoError(t, err)
	}

	writeFile("dep/Chart.yaml", `
apiVersion: v2
name: dep
version: 1.0.0
`)
	writeFile("dep/values.yaml", `
port: 8080
`)
	preExistingDepSchema := `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "properties": {
    "port": {
      "type": "integer",
      "description": "The port to listen on",
      "minimum": 1,
      "maximum": 65535,
      "x-custom-annotation": "preserve-me"
    }
  },
  "required": ["port"]
}
`
	writeFile("dep/values.schema.json", preExistingDepSchema)

	writeFile("parent/Chart.yaml", `
apiVersion: v2
name: parent
version: 1.0.0
dependencies:
  - name: dep
    version: 1.0.0
    repository: file://../dep
`)
	writeFile("parent/values.yaml", `
root: value
`)

	viper.Reset()
	viper.Set("chart-search-root", tmpDir)
	viper.Set("dry-run", false)
	viper.Set("no-dependencies", false)
	viper.Set("add-schema-reference", false)
	viper.Set("keep-full-comment", false)
	viper.Set("helm-docs-compatibility-mode", false)
	viper.Set("uncomment", false)
	viper.Set("output-file", "values.schema.json")
	viper.Set("dont-strip-helm-docs-prefix", false)
	viper.Set("append-newline", false)
	viper.Set("dependencies-filter", []string{})
	viper.Set("dont-add-global", true)
	viper.Set("skip-dependencies-schema-validation", false)
	viper.Set("allow-circular-dependencies", false)
	viper.Set("annotate", false)
	viper.Set("keep-existing-dep-schemas", true)
	viper.Set("value-files", []string{"values.yaml"})
	viper.Set("skip-auto-generation", []string{})
	viper.Set("log-level", "info")

	err := exec(nil, nil)
	assert.NoError(t, err)

	depSchemaPath := filepath.Join(tmpDir, "dep", "values.schema.json")
	depSchemaBytes, err := os.ReadFile(depSchemaPath)
	assert.NoError(t, err)
	assert.Equal(t, preExistingDepSchema, string(depSchemaBytes),
		"dependency's pre-existing values.schema.json must not be overwritten when --keep-existing-dep-schemas is set")

	parentSchemaPath := filepath.Join(tmpDir, "parent", "values.schema.json")
	parentSchemaBytes, err := os.ReadFile(parentSchemaPath)
	assert.NoError(t, err)

	var parentRaw map[string]any
	err = json.Unmarshal(parentSchemaBytes, &parentRaw)
	assert.NoError(t, err)

	props, _ := parentRaw["properties"].(map[string]any)
	depNode, ok := props["dep"].(map[string]any)
	assert.True(t, ok, "parent must nest dependency schema under dep key")
	depProps, _ := depNode["properties"].(map[string]any)
	portNode, ok := depProps["port"].(map[string]any)
	assert.True(t, ok, "parent must include port from merged dependency schema")
	assert.Equal(t, "preserve-me", portNode["x-custom-annotation"],
		"pre-existing x-* annotations must be carried into the merged parent schema")
}

func TestExec_DefaultRegeneratesDependencySchema(t *testing.T) {
	tmpDir := t.TempDir()

	writeFile := func(relPath, content string) {
		path := filepath.Join(tmpDir, relPath)
		err := os.MkdirAll(filepath.Dir(path), 0o755)
		assert.NoError(t, err)
		err = os.WriteFile(path, []byte(content), 0o644)
		assert.NoError(t, err)
	}

	writeFile("dep/Chart.yaml", `
apiVersion: v2
name: dep
version: 1.0.0
`)
	writeFile("dep/values.yaml", `
port: 8080
`)
	preExistingDepSchema := `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "properties": {
    "port": {
      "type": "integer",
      "x-custom-annotation": "should-be-lost"
    }
  }
}
`
	writeFile("dep/values.schema.json", preExistingDepSchema)

	writeFile("parent/Chart.yaml", `
apiVersion: v2
name: parent
version: 1.0.0
dependencies:
  - name: dep
    version: 1.0.0
    repository: file://../dep
`)
	writeFile("parent/values.yaml", `
root: value
`)

	viper.Reset()
	viper.Set("chart-search-root", tmpDir)
	viper.Set("dry-run", false)
	viper.Set("no-dependencies", false)
	viper.Set("add-schema-reference", false)
	viper.Set("keep-full-comment", false)
	viper.Set("helm-docs-compatibility-mode", false)
	viper.Set("uncomment", false)
	viper.Set("output-file", "values.schema.json")
	viper.Set("dont-strip-helm-docs-prefix", false)
	viper.Set("append-newline", false)
	viper.Set("dependencies-filter", []string{})
	viper.Set("dont-add-global", true)
	viper.Set("skip-dependencies-schema-validation", false)
	viper.Set("allow-circular-dependencies", false)
	viper.Set("annotate", false)
	viper.Set("keep-existing-dep-schemas", false)
	viper.Set("value-files", []string{"values.yaml"})
	viper.Set("skip-auto-generation", []string{})
	viper.Set("log-level", "info")

	err := exec(nil, nil)
	assert.NoError(t, err)

	depSchemaPath := filepath.Join(tmpDir, "dep", "values.schema.json")
	depSchemaBytes, err := os.ReadFile(depSchemaPath)
	assert.NoError(t, err)
	assert.NotEqual(t, preExistingDepSchema, string(depSchemaBytes),
		"default mode must regenerate a dependency's values.schema.json")
}

func TestExec_NoDependenciesSkipsDependencyCharts(t *testing.T) {
	tmpDir := t.TempDir()

	writeFile := func(relPath, content string) {
		path := filepath.Join(tmpDir, relPath)
		err := os.MkdirAll(filepath.Dir(path), 0o755)
		assert.NoError(t, err)
		err = os.WriteFile(path, []byte(content), 0o644)
		assert.NoError(t, err)
	}

	writeFile("foo/Chart.yaml", `
apiVersion: v2
name: foo
version: 1.0.0
dependencies:
  - name: bar
    version: 1.0.0
    repository: file://./bar
`)
	writeFile("foo/values.yaml", `
top: 1
`)
	writeFile("foo/bar/Chart.yaml", `
apiVersion: v2
name: bar
version: 1.0.0
`)
	writeFile("foo/bar/values.yaml", `
inside: 2
`)

	viper.Reset()
	viper.Set("chart-search-root", filepath.Join(tmpDir, "foo"))
	viper.Set("dry-run", false)
	viper.Set("no-dependencies", true)
	viper.Set("add-schema-reference", false)
	viper.Set("keep-full-comment", false)
	viper.Set("helm-docs-compatibility-mode", false)
	viper.Set("uncomment", false)
	viper.Set("output-file", "values.schema.json")
	viper.Set("dont-strip-helm-docs-prefix", false)
	viper.Set("append-newline", false)
	viper.Set("dependencies-filter", []string{})
	viper.Set("dont-add-global", true)
	viper.Set("skip-dependencies-schema-validation", false)
	viper.Set("allow-circular-dependencies", false)
	viper.Set("annotate", false)
	viper.Set("keep-existing-dep-schemas", false)
	viper.Set("value-files", []string{"values.yaml"})
	viper.Set("skip-auto-generation", []string{})
	viper.Set("log-level", "info")

	err := exec(nil, nil)
	assert.NoError(t, err)

	parentSchemaPath := filepath.Join(tmpDir, "foo", "values.schema.json")
	_, err = os.Stat(parentSchemaPath)
	assert.NoError(t, err, "parent chart schema must still be generated with --no-dependencies")

	depSchemaPath := filepath.Join(tmpDir, "foo", "bar", "values.schema.json")
	_, err = os.Stat(depSchemaPath)
	assert.True(t, os.IsNotExist(err),
		"dependency chart schema must not be generated with --no-dependencies (issue #215)")
}
