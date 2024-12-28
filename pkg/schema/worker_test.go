package schema

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWorker(t *testing.T) {
	tests := []struct {
		name                      string
		setupFiles                map[string]string // map of filepath to content
		chartPath                 string
		valueFileNames            []string
		dryRun                    bool
		uncomment                 bool
		addSchemaReference        bool
		keepFullComment           bool
		helmDocsCompatibilityMode bool
		dontRemoveHelmDocsPrefix  bool
		dontAddGlobal             bool
		skipAutoGenerationConfig  *SkipAutoGenerationConfig
		outFile                   string
		expectedErrors            bool
	}{
		{
			name: "valid chart and values",
			setupFiles: map[string]string{
				"Chart.yaml": `
apiVersion: v2
name: test-chart
version: 1.0.0
`,
				"values.yaml": `
# -- first value
key1: value1
# -- second value
key2: value2
`,
			},
			chartPath:                 "Chart.yaml",
			valueFileNames:            []string{"values.yaml"},
			uncomment:                 true,
			addSchemaReference:        true,
			keepFullComment:           true,
			helmDocsCompatibilityMode: true,
			skipAutoGenerationConfig: &SkipAutoGenerationConfig{
				Title:                false,
				Description:          false,
				Required:             false,
				Default:              false,
				AdditionalProperties: false,
			},
		},
		{
			name: "missing values file",
			setupFiles: map[string]string{
				"Chart.yaml": `
apiVersion: v2
name: test-chart
version: 1.0.0
`,
			},
			chartPath:      "Chart.yaml",
			valueFileNames: []string{"values.yaml"},
			expectedErrors: true,
			skipAutoGenerationConfig: &SkipAutoGenerationConfig{
				Title:                false,
				Description:          false,
				Required:             false,
				Default:              false,
				AdditionalProperties: false,
			},
		},
		{
			name: "invalid chart file",
			setupFiles: map[string]string{
				"Chart.yaml": `
name: [this is invalid yaml
version: 1.0.0
`,
				"values.yaml": `
key1: value1
`,
			},
			chartPath:      "Chart.yaml",
			valueFileNames: []string{"values.yaml"},
			expectedErrors: true,
			skipAutoGenerationConfig: &SkipAutoGenerationConfig{
				Title:                false,
				Description:          false,
				Required:             false,
				Default:              false,
				AdditionalProperties: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory
			tmpDir, err := os.MkdirTemp("", "worker-test-*")
			assert.NoError(t, err)
			defer os.RemoveAll(tmpDir)

			// Create test files
			for filename, content := range tt.setupFiles {
				path := filepath.Join(tmpDir, filename)
				err := os.WriteFile(path, []byte(content), 0644)
				assert.NoError(t, err)
			}

			// Setup channels
			queue := make(chan string, 1)
			results := make(chan Result, 1)

			// Update chart path to use temp directory
			fullChartPath := filepath.Join(tmpDir, tt.chartPath)
			queue <- fullChartPath
			close(queue)

			// Run worker
			Worker(
				tt.dryRun,
				tt.uncomment,
				tt.addSchemaReference,
				tt.keepFullComment,
				tt.helmDocsCompatibilityMode,
				tt.dontRemoveHelmDocsPrefix,
				tt.dontAddGlobal,
				tt.valueFileNames,
				tt.skipAutoGenerationConfig,
				tt.outFile,
				queue,
				results,
			)

			// Get result
			result := <-results

			if tt.expectedErrors {
				assert.NotEmpty(t, result.Errors)
			} else {
				assert.Empty(t, result.Errors)
				assert.NotNil(t, result.Chart)
				assert.NotEmpty(t, result.ValuesPath)
				assert.NotNil(t, result.Schema)
			}
		})
	}
}
