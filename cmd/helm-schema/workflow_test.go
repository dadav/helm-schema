package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExec_ArchiveErrorsPreventWrites(t *testing.T) {
	for _, mode := range []string{"generate", "check", "annotate", "add-schema-reference", "dry-run"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			values := "replicas: 1\n"
			writeTestFile(t, root, "Chart.yaml", "apiVersion: v2\nname: parent\nversion: 1.0.0\n")
			writeTestFile(t, root, "values.yaml", values)
			setStandardViper(root)
			require.NoError(t, exec(nil, nil))
			schemaPath := filepath.Join(root, "values.schema.json")
			before, err := os.ReadFile(schemaPath)
			require.NoError(t, err)
			writeTestFile(t, root, "charts/broken.tgz", "not a gzip archive")
			if mode != "generate" {
				viper.Set(mode, true)
			}

			assert.ErrorContains(t, exec(nil, nil), "archive discovery failed")
			after, err := os.ReadFile(schemaPath)
			require.NoError(t, err)
			assert.Equal(t, string(before), string(after))
			actualValues, err := os.ReadFile(filepath.Join(root, "values.yaml"))
			require.NoError(t, err)
			assert.Equal(t, values, string(actualValues))
			entries, err := os.ReadDir(filepath.Join(root, "charts"))
			require.NoError(t, err)
			require.Len(t, entries, 1, "temporary extraction directories must be removed")
			assert.Equal(t, "broken.tgz", entries[0].Name())
		})
	}
}

func TestExec_ManyArchiveErrorsReturn(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "Chart.yaml", "apiVersion: v2\nname: parent\nversion: 1.0.0\n")
	writeTestFile(t, root, "values.yaml", "replicas: 1\n")
	for i := 0; i < 101; i++ {
		writeTestFile(t, root, fmt.Sprintf("charts/broken-%03d.tgz", i), "not a gzip archive")
	}
	setStandardViper(root)
	assert.ErrorContains(t, exec(nil, nil), "archive discovery failed")
	assert.NoFileExists(t, filepath.Join(root, "values.schema.json"))
	entries, err := os.ReadDir(filepath.Join(root, "charts"))
	require.NoError(t, err)
	assert.Len(t, entries, 101)
}

func TestExec_ImportValuesRetainsDependencySchema(t *testing.T) {
	for _, test := range []struct {
		name     string
		imports  string
		alias    string
		archived bool
		library  bool
	}{
		{name: "simple", imports: "      - defaults\n"},
		{name: "complex alias", imports: "      - child: exports.defaults\n        parent: config\n", alias: "backend"},
		{name: "archived alias", imports: "      - defaults\n", alias: "backend", archived: true},
		{name: "library", imports: "      - defaults\n", library: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			parentChart := "apiVersion: v2\nname: parent\nversion: 1.0.0\ndependencies:\n  - name: child\n    version: 1.0.0\n    import-values:\n" + test.imports
			if test.alias != "" {
				parentChart += "    alias: " + test.alias + "\n"
			}
			writeTestFile(t, root, "Chart.yaml", parentChart)
			writeTestFile(t, root, "values.yaml", "replicas: 1\n")
			childChart := "apiVersion: v2\nname: child\nversion: 1.0.0\n"
			if test.library {
				childChart += "type: library\n"
			}
			childValues := "exports:\n  defaults:\n    region: eu\nport: 8080\n"
			if test.archived {
				writeTestChartArchive(t, filepath.Join(root, "charts", "child-1.0.0.tgz"), []testArchiveFile{
					{name: "child/Chart.yaml", content: childChart},
					{name: "child/values.yaml", content: childValues},
				})
			} else {
				writeTestFile(t, root, "charts/child/Chart.yaml", childChart)
				writeTestFile(t, root, "charts/child/values.yaml", childValues)
			}
			setStandardViper(root)
			require.NoError(t, exec(nil, nil))
			data, err := os.ReadFile(filepath.Join(root, "values.schema.json"))
			require.NoError(t, err)
			var doc schemaDoc
			require.NoError(t, json.Unmarshal(data, &doc))
			if test.name == "complex alias" {
				assert.Contains(t, doc.Properties["config"].Properties, "region")
			} else {
				assert.Contains(t, doc.Properties, "region")
			}
			if test.library {
				assert.Contains(t, doc.Properties, "port")
				assert.NotContains(t, doc.Properties, "child")
				return
			}
			dependencyName := "child"
			if test.alias != "" {
				dependencyName = test.alias
			}
			require.Contains(t, doc.Properties, dependencyName)
			assert.Equal(t, stringOrArray{"integer"}, doc.Properties[dependencyName].Properties["port"].Type)
			assert.Contains(t, doc.Properties[dependencyName].Properties, "exports")
			if test.alias != "" {
				assert.NotContains(t, doc.Properties, "child")
			}
		})
	}
}

func TestExec_CheckArchivedDependency(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "Chart.yaml", "apiVersion: v2\nname: parent\nversion: 1.0.0\ndependencies:\n  - name: child\n    version: 1.0.0\n")
	writeTestFile(t, root, "values.yaml", "replicas: 1\n")
	archivePath := filepath.Join(root, "charts", "child-1.0.0.tgz")
	files := []testArchiveFile{
		{name: "child/Chart.yaml", content: "apiVersion: v2\nname: child\nversion: 1.0.0\n"},
		{name: "child/values.yaml", content: "port: 8080\n"},
	}
	writeTestChartArchive(t, archivePath, files)
	setStandardViper(root)
	require.NoError(t, exec(nil, nil))
	before, err := os.ReadFile(filepath.Join(root, "values.schema.json"))
	require.NoError(t, err)
	viper.Set("check", true)
	assert.NoError(t, exec(nil, nil), "temporary dependency output is not a committed schema")

	files[1].content = "port: 8080\nnewSetting: true\n"
	writeTestChartArchive(t, archivePath, files)
	assert.ErrorContains(t, exec(nil, nil), "schema files are not up-to-date")
	after, err := os.ReadFile(filepath.Join(root, "values.schema.json"))
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after), "check mode must not write stale schemas")
}
