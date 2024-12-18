package chart

import (
	"bytes"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestReadChartFile(t *testing.T) {
	data := []byte(`
  name: test
  description: test
  dependencies:
    - name: test
  `)
	r := bytes.NewReader(data)
	c, err := ReadChart(r)
	if err != nil {
		t.Errorf("Error while reading test data: %v", err)
	}
	if c.Name != "test" {
		t.Errorf("Expected Name was test, but got %v", c.Name)
	}
	if c.Description != "test" {
		t.Errorf("Expected Description was test, but got %v", c.Description)
	}
	if len(c.Dependencies) != 1 {
		t.Errorf("Expected to find one dependency, but got %d", len(c.Dependencies))
	}
	if c.Dependencies[0].Name != "test" {
		t.Errorf("Expected Dependency name was test, but got %v", c.Dependencies[0].Name)
	}
}

func TestChartFileParsing(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected ChartFile
		wantErr  bool
	}{
		{
			name: "basic chart file",
			input: `
name: mychart
description: A test chart
dependencies:
  - name: dep1
    alias: aliased-dep
    condition: subchart.enabled`,
			expected: ChartFile{
				Name:        "mychart",
				Description: "A test chart",
				Dependencies: []*Dependency{
					{
						Name:      "dep1",
						Alias:     "aliased-dep",
						Condition: "subchart.enabled",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "chart file without dependencies",
			input: `
name: standalone
description: A standalone chart`,
			expected: ChartFile{
				Name:        "standalone",
				Description: "A standalone chart",
			},
			wantErr: false,
		},
		{
			name: "invalid yaml",
			input: `
name: broken
description: [broken`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got ChartFile
			err := yaml.Unmarshal([]byte(tt.input), &got)

			if (err != nil) != tt.wantErr {
				t.Errorf("yaml.Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if got.Name != tt.expected.Name {
					t.Errorf("Name = %v, want %v", got.Name, tt.expected.Name)
				}
				if got.Description != tt.expected.Description {
					t.Errorf("Description = %v, want %v", got.Description, tt.expected.Description)
				}
				if len(got.Dependencies) != len(tt.expected.Dependencies) {
					t.Errorf("Dependencies length = %v, want %v", len(got.Dependencies), len(tt.expected.Dependencies))
					return
				}
				for i, dep := range got.Dependencies {
					if dep.Name != tt.expected.Dependencies[i].Name {
						t.Errorf("Dependency[%d].Name = %v, want %v", i, dep.Name, tt.expected.Dependencies[i].Name)
					}
					if dep.Alias != tt.expected.Dependencies[i].Alias {
						t.Errorf("Dependency[%d].Alias = %v, want %v", i, dep.Alias, tt.expected.Dependencies[i].Alias)
					}
					if dep.Condition != tt.expected.Dependencies[i].Condition {
						t.Errorf("Dependency[%d].Condition = %v, want %v", i, dep.Condition, tt.expected.Dependencies[i].Condition)
					}
				}
			}
		})
	}
}
