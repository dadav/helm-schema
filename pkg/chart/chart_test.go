package chart

import (
	"bytes"
	"testing"
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
