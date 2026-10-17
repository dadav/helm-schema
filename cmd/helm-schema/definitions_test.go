package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestExec_DependencyDefinitionsReachAncestors(t *testing.T) {
	for _, keyword := range []string{"definitions", "$defs"} {
		for _, mode := range []string{"nested", "library", "import-values"} {
			t.Run(keyword+"/"+mode, func(t *testing.T) {
				root := t.TempDir()
				t.Cleanup(viper.Reset)
				chartType, imports := "application", ""
				if mode == "library" {
					chartType = "library"
				}
				if mode == "import-values" {
					imports = "    import-values:\n      - child: exports.defaults\n        parent: imported\n"
				}
				writeTestFile(t, root, "child/Chart.yaml", "apiVersion: v2\nname: child\nversion: 1.0.0\ntype: "+chartType+"\n")
				writeTestFile(t, root, "child/values.yaml", fmt.Sprintf("exports:\n  defaults:\n    # @schema\n    # $ref: config.json#/%s/Config\n    # @schema\n    config: {}\n", keyword))
				writeTestFile(t, root, "child/config.json", fmt.Sprintf(`{"%s":{"Config":{"type":"object","properties":{"url":{"$ref":"#/%s/URL"}}},"URL":{"type":"string","minLength":3}}}`, keyword, keyword))
				writeTestFile(t, root, "parent/Chart.yaml", "apiVersion: v2\nname: parent\nversion: 1.0.0\ndependencies:\n  - name: child\n    alias: aliased\n    version: 1.0.0\n"+imports)
				writeTestFile(t, root, "parent/values.yaml", "parentValue: true\n")
				writeTestFile(t, root, "grandparent/Chart.yaml", "apiVersion: v2\nname: grandparent\nversion: 1.0.0\ndependencies:\n  - name: parent\n    version: 1.0.0\n")
				writeTestFile(t, root, "grandparent/values.yaml", "rootValue: true\n")
				setStandardViper(root)
				require.NoError(t, exec(nil, nil))
				for _, chart := range []string{"child", "parent", "grandparent"} {
					data, err := os.ReadFile(filepath.Join(root, chart, "values.schema.json"))
					require.NoError(t, err)
					var doc map[string]interface{}
					require.NoError(t, json.Unmarshal(data, &doc))
					defs, ok := doc["definitions"].(map[string]interface{})
					require.True(t, ok, chart)
					prefix := "config.json/"
					if chart == "parent" {
						prefix = "aliased/" + prefix
					}
					if chart == "grandparent" {
						prefix = "parent/aliased/" + prefix
					}
					require.Contains(t, defs, prefix+"Config", chart)
					require.Contains(t, defs, prefix+"URL", chart)
					require.NotContains(t, string(data), "#/$defs/")
				}
			})
		}
	}
}

func TestExec_ConflictingDefinitionsRemainIndependent(t *testing.T) {
	for _, mode := range []string{"nested", "library", "import-values"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			t.Cleanup(viper.Reset)
			chartType, imports := "application", ""
			if mode == "library" {
				chartType = "library"
			}
			if mode == "import-values" {
				imports = "    import-values:\n      - child: exports.defaults\n        parent: imported\n"
			}
			writeTestFile(t, root, "Chart.yaml", "apiVersion: v2\nname: parent\nversion: 1.0.0\ndependencies:\n  - name: child\n    alias: first\n    version: 1.0.0\n"+imports+"  - name: child\n    alias: second\n    version: 1.0.0\n"+strings.ReplaceAll(imports, "parent: imported", "parent: otherImported"))
			writeTestFile(t, root, "values.yaml", "# @schema\n# $ref: parent.json#/definitions/Config\n# @schema\nparent: parent\n")
			writeTestFile(t, root, "parent.json", `{"definitions":{"Config":{"type":"string","enum":["parent"]}}}`)
			writeTestFile(t, root, "charts/child/Chart.yaml", "apiVersion: v2\nname: child\nversion: 1.0.0\ntype: "+chartType+"\n")
			writeTestFile(t, root, "charts/child/config.json", `{"$defs":{"Config":{"type":"integer","minimum":10},"Map":{"type":"object","additionalProperties":{"$ref":"#/$defs/Config"}}}}`)
			writeTestFile(t, root, "charts/child/values.yaml", "exports:\n  defaults:\n    # @schema\n    # $ref: config.json#/$defs/Map\n    # @schema\n    config: {}\n")
			setStandardViper(root)
			require.NoError(t, exec(nil, nil))
			data, err := os.ReadFile(filepath.Join(root, "values.schema.json"))
			require.NoError(t, err)
			doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
			require.NoError(t, err)
			compiler := jsonschema.NewCompiler()
			require.NoError(t, compiler.AddResource("values.schema.json", doc))
			compiled, err := compiler.Compile("values.schema.json")
			require.NoError(t, err)
			instance := map[string]interface{}{"parent": "parent"}
			var config map[string]interface{}
			switch mode {
			case "library":
				config = map[string]interface{}{"value": float64(12)}
				instance["exports"] = map[string]interface{}{"defaults": map[string]interface{}{"config": config}}
			case "import-values":
				config = map[string]interface{}{"value": float64(12)}
				instance["imported"] = map[string]interface{}{"config": config}
				instance["otherImported"] = map[string]interface{}{"config": map[string]interface{}{"value": float64(13)}}
			default:
				config = map[string]interface{}{"value": float64(12)}
				instance["first"] = map[string]interface{}{"exports": map[string]interface{}{"defaults": map[string]interface{}{"config": config}}}
				instance["second"] = map[string]interface{}{"exports": map[string]interface{}{"defaults": map[string]interface{}{"config": map[string]interface{}{"value": float64(13)}}}}
			}
			require.NoError(t, compiled.Validate(instance))
			config["value"] = "parent"
			require.Error(t, compiled.Validate(instance))
			config["value"] = float64(12)
			instance["parent"] = float64(12)
			require.Error(t, compiled.Validate(instance))
			viper.Set("check", true)
			require.NoError(t, exec(nil, nil))
		})
	}
}
