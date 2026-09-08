package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func readTestSchema(t *testing.T, path string) schemaDoc {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var doc schemaDoc
	require.NoError(t, json.Unmarshal(data, &doc))
	return doc
}

func TestExec_DuplicateChartsAndCheck(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"first", "second"} {
		writeTestFile(t, root, name+"/Chart.yaml", "apiVersion: v2\nname: common\nversion: 1.0.0\n")
		writeTestFile(t, root, name+"/values.yaml", name+": 1\n")
	}
	setStandardViper(root)
	require.NoError(t, exec(nil, nil))
	for _, name := range []string{"first", "second"} {
		doc := readTestSchema(t, filepath.Join(root, name, "values.schema.json"))
		assert.Contains(t, doc.Properties, name)
	}
	viper.Set("check", true)
	require.NoError(t, exec(nil, nil))
	for _, name := range []string{"first", "second"} {
		path := filepath.Join(root, name, "values.schema.json")
		original, err := os.ReadFile(path)
		require.NoError(t, err)
		for _, missing := range []bool{false, true} {
			if missing {
				require.NoError(t, os.Remove(path))
			} else {
				require.NoError(t, os.WriteFile(path, []byte("{}"), 0o644))
			}
			assert.ErrorContains(t, exec(nil, nil), "schema files are not up-to-date")
			if missing {
				assert.NoFileExists(t, path)
			} else {
				data, err := os.ReadFile(path)
				require.NoError(t, err)
				assert.Equal(t, "{}", string(data))
			}
			require.NoError(t, os.WriteFile(path, original, 0o644))
		}
	}
}

func TestExec_DuplicateDependenciesStayWithOwner(t *testing.T) {
	for _, layout := range []string{"installed", "legacy", "archived", "library"} {
		for _, secondVersion := range []string{"1.0.0", "2.0.0"} {
			t.Run(layout+"/"+secondVersion, func(t *testing.T) {
				root := t.TempDir()
				for i, owner := range []string{"first", "second"} {
					version := "1.0.0"
					if i == 1 {
						version = secondVersion
					}
					parent := fmt.Sprintf("apiVersion: v2\nname: %s\nversion: 1.0.0\ndependencies:\n  - name: common\n    version: %s\n    alias: backend\n    condition: backend.%sEnabled\n    import-values:\n      - defaults\n", owner, version, owner)
					writeTestFile(t, root, owner+"/Chart.yaml", parent)
					writeTestFile(t, root, owner+"/values.yaml", "replicas: 1\n")
					metadata := fmt.Sprintf("apiVersion: v2\nname: common\nversion: %s\n", version)
					if layout == "library" {
						metadata += "type: library\n"
					}
					values := fmt.Sprintf("%s: 1\nexports:\n  defaults:\n    %sImported: true\n", owner, owner)
					if layout == "archived" {
						writeTestChartArchive(t, filepath.Join(root, owner, "charts", "common-"+version+".tgz"), []testArchiveFile{
							{name: "common/Chart.yaml", content: metadata},
							{name: "common/values.yaml", content: values},
						})
					} else {
						dir := owner + "/charts/common"
						if layout == "legacy" {
							dir = owner + "/common"
						}
						writeTestFile(t, root, dir+"/Chart.yaml", metadata)
						writeTestFile(t, root, dir+"/values.yaml", values)
					}
				}
				setStandardViper(root)
				require.NoError(t, exec(nil, nil))
				for _, owner := range []string{"first", "second"} {
					other := "first"
					if owner == "first" {
						other = "second"
					}
					doc := readTestSchema(t, filepath.Join(root, owner, "values.schema.json"))
					properties := doc.Properties
					if layout == "library" {
						assert.NotContains(t, properties, "backend")
					} else {
						require.Contains(t, properties, "backend")
						properties = properties["backend"].Properties
					}
					assert.Contains(t, properties, owner)
					assert.Contains(t, properties, owner+"Enabled")
					assert.NotContains(t, properties, other+"Enabled")
					assert.Contains(t, doc.Properties, owner+"Imported")
					assert.NotContains(t, doc.Properties, other+"Imported")
				}
				viper.Set("check", true)
				require.NoError(t, exec(nil, nil))
			})
		}
	}
}

func TestExec_DependencyFlagsDoNotSkipUnrelatedDuplicate(t *testing.T) {
	for _, mode := range []string{"no-dependencies", "keep-existing-dep-schemas", "annotate", "add-schema-reference"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, root, "parent/Chart.yaml", "apiVersion: v2\nname: parent\nversion: 1.0.0\ndependencies:\n  - name: common\n    version: 1.0.0\n")
			writeTestFile(t, root, "parent/values.yaml", "parent: true\n")
			invalidValues := "# @schema\n# type: string\nbroken: value\n"
			for _, path := range []string{"parent/charts/common", "unrelated"} {
				writeTestFile(t, root, path+"/Chart.yaml", "apiVersion: v2\nname: common\nversion: 1.0.0\n")
			}
			writeTestFile(t, root, "parent/charts/common/values.yaml", invalidValues)
			writeTestFile(t, root, "unrelated/values.yaml", "unrelated: true\n")
			existing := `{"type":"object","properties":{"preserved":{"type":"integer","minimum":1}}}`
			writeTestFile(t, root, "parent/charts/common/values.schema.json", existing)
			writeTestFile(t, root, "unrelated/values.schema.json", existing)
			setStandardViper(root)
			viper.Set(mode, true)
			if mode != "keep-existing-dep-schemas" {
				viper.Set("no-dependencies", true)
			}
			require.NoError(t, exec(nil, nil))
			data, err := os.ReadFile(filepath.Join(root, "parent/charts/common/values.yaml"))
			require.NoError(t, err)
			assert.Equal(t, invalidValues, string(data))
			data, err = os.ReadFile(filepath.Join(root, "parent/charts/common/values.schema.json"))
			require.NoError(t, err)
			assert.Equal(t, existing, string(data))
			data, err = os.ReadFile(filepath.Join(root, "unrelated/values.yaml"))
			require.NoError(t, err)
			if mode == "annotate" {
				assert.Contains(t, string(data), "# @schema")
				return
			}
			if mode == "add-schema-reference" {
				assert.Contains(t, string(data), "# yaml-language-server: $schema=")
			}
			doc := readTestSchema(t, filepath.Join(root, "unrelated/values.schema.json"))
			assert.Contains(t, doc.Properties, "unrelated")
			assert.NotContains(t, doc.Properties, "preserved")
			parent := readTestSchema(t, filepath.Join(root, "parent/values.schema.json"))
			if mode == "keep-existing-dep-schemas" {
				require.Contains(t, parent.Properties, "common")
				assert.Contains(t, parent.Properties["common"].Properties, "preserved")
			} else {
				assert.NotContains(t, parent.Properties, "common")
			}
			viper.Set("add-schema-reference", false)
			viper.Set("check", true)
			require.NoError(t, exec(nil, nil))
		})
	}
}

func TestExec_DuplicateChartWorkerErrors(t *testing.T) {
	for _, invalid := range []string{"first", "second"} {
		t.Run(invalid, func(t *testing.T) {
			root := t.TempDir()
			for _, name := range []string{"first", "second"} {
				writeTestFile(t, root, name+"/Chart.yaml", "apiVersion: v2\nname: common\nversion: 1.0.0\n")
				values := "valid: true\n"
				if name == invalid {
					values = "# @schema\n# type: string\nbroken: value\n"
				}
				writeTestFile(t, root, name+"/values.yaml", values)
			}
			setStandardViper(root)
			assert.ErrorContains(t, exec(nil, nil), "some errors were found")
		})
	}
}

func TestExec_DependencyFilterPreservesOwnership(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "Chart.yaml", "apiVersion: v2\nname: parent\nversion: 1.0.0\ndependencies:\n  - name: common\n    version: 1.0.0\n")
	writeTestFile(t, root, "values.yaml", "parent: true\n")
	writeTestFile(t, root, "charts/excluded/Chart.yaml", "apiVersion: v2\nname: excluded\nversion: 1.0.0\n")
	for _, dir := range []string{"charts/common", "charts/excluded/charts/common"} {
		writeTestFile(t, root, dir+"/Chart.yaml", "apiVersion: v2\nname: common\nversion: 1.0.0\n")
		writeTestFile(t, root, dir+"/values.yaml", "setting: true\n")
	}
	setStandardViper(root)
	viper.Set("dependencies-filter", []string{"common"})
	require.NoError(t, exec(nil, nil))
	assert.FileExists(t, filepath.Join(root, "values.schema.json"))
	assert.FileExists(t, filepath.Join(root, "charts/common/values.schema.json"))
	assert.FileExists(t, filepath.Join(root, "charts/excluded/charts/common/values.schema.json"))
}

func TestExec_AmbiguousDependencyPreventsWrites(t *testing.T) {
	for _, mode := range []string{"generate", "annotate", "add-schema-reference"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, root, "parent/Chart.yaml", "apiVersion: v2\nname: parent\nversion: 1.0.0\ndependencies:\n  - name: common\n    version: 1.0.0\n")
			for _, name := range []string{"first", "second"} {
				writeTestFile(t, root, name+"/Chart.yaml", "apiVersion: v2\nname: common\nversion: 1.0.0\n")
			}
			for _, name := range []string{"parent", "first", "second"} {
				writeTestFile(t, root, name+"/values.yaml", "key: true\n")
			}
			setStandardViper(root)
			if mode != "generate" {
				viper.Set(mode, true)
			}
			err := exec(nil, nil)
			require.ErrorContains(t, err, "ambiguous dependency")
			for _, name := range []string{"parent", "first", "second"} {
				assert.Contains(t, err.Error(), filepath.Join(root, name, "Chart.yaml"))
				assert.NoFileExists(t, filepath.Join(root, name, "values.schema.json"))
				data, readErr := os.ReadFile(filepath.Join(root, name, "values.yaml"))
				require.NoError(t, readErr)
				assert.Equal(t, "key: true\n", string(data))
			}
		})
	}
}

func TestExec_DuplicateNestedArchives(t *testing.T) {
	root := t.TempDir()
	for _, owner := range []string{"first", "second"} {
		writeTestFile(t, root, owner+"/Chart.yaml", "apiVersion: v2\nname: "+owner+"\nversion: 1.0.0\ndependencies:\n  - name: common\n    version: 1.0.0\n")
		writeTestFile(t, root, owner+"/values.yaml", "replicas: 1\n")
		leafArchive := filepath.Join(t.TempDir(), "leaf.tgz")
		writeTestChartArchive(t, leafArchive, []testArchiveFile{
			{name: "leaf/Chart.yaml", content: "apiVersion: v2\nname: leaf\nversion: 1.0.0\n"},
			{name: "leaf/values.yaml", content: owner + ": true\n"},
		})
		leafBytes, err := os.ReadFile(leafArchive)
		require.NoError(t, err)
		writeTestChartArchive(t, filepath.Join(root, owner, "charts", "common.tar.gz"), []testArchiveFile{
			{name: "common/Chart.yaml", content: "apiVersion: v2\nname: common\nversion: 1.0.0\ndependencies:\n  - name: leaf\n    version: 1.0.0\n"},
			{name: "common/values.yaml", content: "port: 80\n"},
			{name: "common/charts/leaf.tgz", content: string(leafBytes)},
		})
	}
	setStandardViper(root)
	require.NoError(t, exec(nil, nil))
	for _, owner := range []string{"first", "second"} {
		doc := readTestSchema(t, filepath.Join(root, owner, "values.schema.json"))
		require.Contains(t, doc.Properties, "common")
		require.Contains(t, doc.Properties["common"].Properties, "leaf")
		assert.Contains(t, doc.Properties["common"].Properties["leaf"].Properties, owner)
		entries, err := os.ReadDir(filepath.Join(root, owner, "charts"))
		require.NoError(t, err)
		require.Len(t, entries, 1)
		assert.Equal(t, "common.tar.gz", entries[0].Name())
	}
	viper.Set("check", true)
	require.NoError(t, exec(nil, nil))
}

func TestExec_AliasedDependencyVersions(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "Chart.yaml", "apiVersion: v2\nname: parent\nversion: 1.0.0\ndependencies:\n  - name: common\n    version: 1.0.0\n    alias: old\n    condition: old.oldEnabled\n  - name: common\n    version: 2.0.0\n    alias: current\n    condition: current.currentEnabled\n  - name: common\n    version: 2.0.0\n    alias: replica\n")
	writeTestFile(t, root, "values.yaml", "replicas: 1\n")
	for _, version := range []string{"1.0.0", "2.0.0"} {
		key := "oldSetting"
		if version == "2.0.0" {
			key = "currentSetting"
		}
		writeTestChartArchive(t, filepath.Join(root, "charts", "common-"+version+".tgz"), []testArchiveFile{
			{name: "common/Chart.yaml", content: "apiVersion: v2\nname: common\nversion: " + version + "\n"},
			{name: "common/values.yaml", content: key + ": true\n"},
		})
	}
	setStandardViper(root)
	require.NoError(t, exec(nil, nil))
	doc := readTestSchema(t, filepath.Join(root, "values.schema.json"))
	require.Contains(t, doc.Properties, "old")
	require.Contains(t, doc.Properties, "current")
	require.Contains(t, doc.Properties, "replica")
	assert.Contains(t, doc.Properties["old"].Properties, "oldSetting")
	assert.Contains(t, doc.Properties["old"].Properties, "oldEnabled")
	assert.NotContains(t, doc.Properties["old"].Properties, "currentEnabled")
	assert.Contains(t, doc.Properties["current"].Properties, "currentSetting")
	assert.Contains(t, doc.Properties["current"].Properties, "currentEnabled")
	assert.NotContains(t, doc.Properties["current"].Properties, "oldEnabled")
	assert.Equal(t, doc.Properties["current"], doc.Properties["replica"])
	viper.Set("check", true)
	require.NoError(t, exec(nil, nil))
}

func TestExec_ConditionAliasMatchesParentNamespace(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "Chart.yaml", "apiVersion: v2\nname: parent\nversion: 1.0.0\ndependencies:\n  - name: common\n    alias: backend\n    condition: backend.ownEnabled,common.siblingEnabled\n  - name: other\n    alias: common\n")
	writeTestFile(t, root, "values.yaml", "replicas: 1\n")
	for _, name := range []string{"common", "other"} {
		writeTestFile(t, root, "charts/"+name+"/Chart.yaml", "apiVersion: v2\nname: "+name+"\nversion: 1.0.0\n")
		writeTestFile(t, root, "charts/"+name+"/values.yaml", "port: 80\n")
	}
	setStandardViper(root)
	require.NoError(t, exec(nil, nil))
	doc := readTestSchema(t, filepath.Join(root, "values.schema.json"))
	assert.Contains(t, doc.Properties["backend"].Properties, "ownEnabled")
	assert.NotContains(t, doc.Properties["backend"].Properties, "siblingEnabled")
	assert.Contains(t, doc.Properties["common"].Properties, "siblingEnabled")
	assert.NotContains(t, doc.Properties["common"].Properties, "ownEnabled")
}

func TestExec_DuplicateChartsDryRun(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"first", "second"} {
		writeTestFile(t, root, name+"/Chart.yaml", "apiVersion: v2\nname: common\nversion: 1.0.0\n")
		writeTestFile(t, root, name+"/values.yaml", name+": true\n")
	}
	setStandardViper(root)
	viper.Set("dry-run", true)
	output, err := os.CreateTemp(t.TempDir(), "stdout")
	require.NoError(t, err)
	t.Cleanup(func() { output.Close() })
	originalStdout := os.Stdout
	err = func() error {
		os.Stdout = output
		defer func() { os.Stdout = originalStdout }()
		return exec(nil, nil)
	}()
	require.NoError(t, err)
	_, err = output.Seek(0, 0)
	require.NoError(t, err)
	decoder := json.NewDecoder(output)
	for _, name := range []string{"first", "second"} {
		var doc schemaDoc
		require.NoError(t, decoder.Decode(&doc))
		assert.Contains(t, doc.Properties, name)
		assert.NoFileExists(t, filepath.Join(root, name, "values.schema.json"))
	}
	var extra schemaDoc
	assert.ErrorIs(t, decoder.Decode(&extra), io.EOF)
}
