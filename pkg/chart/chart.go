package chart

import (
	"io"

	"github.com/dadav/helm-schema/pkg/util"
	yaml "gopkg.in/yaml.v3"
)

type Dependency struct {
	Name      string `yaml:"name"`
	Version   string `yaml:"version"`
	Condition string `yaml:"condition"`
}

type ChartFile struct {
	Name         string       `yaml:"name"`
	Description  string       `yaml:"description"`
	Dependencies []Dependency `yaml:"dependencies"`
}

// ReadChart parses the given yaml into a ChartFile struct
func ReadChart(reader io.Reader) (ChartFile, error) {
	var chart ChartFile

	chartContent, err := util.ReadFileAndFixNewline(reader)
	if err != nil {
		return chart, err
	}

	err = yaml.Unmarshal(chartContent, &chart)
	if err != nil {
		return chart, err
	}
	return chart, nil
}
