package chart

import (
	"github.com/dadav/helm-schema/pkg/util"
	yaml "gopkg.in/yaml.v3"
)

type ChartFile struct {
	Name         string              `yaml:"name"`
	Description  string              `yaml:"description"`
	Dependencies []map[string]string `yaml:"dependencies"`
}

func ReadChartFile(path string) (ChartFile, error) {
	var chart ChartFile

	chartContent, err := util.ReadYamlFile(path)
	if err != nil {
		return chart, err
	}

	err = yaml.Unmarshal(chartContent, &chart)
	if err != nil {
		return chart, err
	}
	return chart, nil
}
