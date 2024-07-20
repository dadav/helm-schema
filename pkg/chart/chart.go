package chart

import (
	"io"

	"github.com/dadav/helm-schema/pkg/util"
	yaml "gopkg.in/yaml.v3"
)

type Dependency struct {
	Name       string `yaml:"name"`
	Version    string `yaml:"version"`
	Condition  string `yaml:"condition,omitempty"`
	Repository string `yaml:"repository,omitempty"`
	Alias      string `yaml:"alias,omitempty"`
	// Tags         []string `yaml:"tags,omitempty"`
	// ImportValues []string `yaml:"import-values,omitempty"`
}

// Maintainer describes a Chart maintainer.
// https://github.com/helm/helm/blob/main/pkg/chart/metadata.go#L26C1-L34C2
type Maintainer struct {
	// Name is a user name or organization name
	Name string `json:"name,omitempty"`
	// Email is an optional email address to contact the named maintainer
	Email string `json:"email,omitempty"`
	// URL is an optional URL to an address for the named maintainer
	URL string `json:"url,omitempty"`
}

// https://github.com/helm/helm/blob/main/pkg/chart/metadata.go#L48
type ChartFile struct {
	// The name of the chart. Required.
	Name string `yaml:"name,omitempty"`
	// The URL to a relevant project page, git repo, or contact person
	Home string `yaml:"home,omitempty"`
	// Source is the URL to the source code of this chart
	Sources []string `yaml:"sources,omitempty"`
	// A SemVer 2 conformant version string of the chart. Required.
	Version string `yaml:"version,omitempty"`
	// A one-sentence description of the chart
	Description string `yaml:"description,omitempty"`
	// A list of string keywords
	Keywords []string `yaml:"keywords,omitempty"`
	// A list of name and URL/email address combinations for the maintainer(s)
	Maintainers []*Maintainer `yaml:"maintainers,omitempty"`
	// The URL to an icon file.
	Icon string `yaml:"icon,omitempty"`
	// The API Version of this chart. Required.
	APIVersion string `yaml:"apiVersion,omitempty"`
	// The condition to check to enable chart
	Condition string `yaml:"condition,omitempty"`
	// The tags to check to enable chart
	Tags string `yaml:"tags,omitempty"`
	// The version of the application enclosed inside of this chart.
	AppVersion string `yaml:"appVersion,omitempty"`
	// Whether or not this chart is deprecated
	Deprecated bool `yaml:"deprecated,omitempty"`
	// Annotations are additional mappings uninterpreted by Helm,
	// made available for inspection by other applications.
	Annotations map[string]string `yaml:"annotations,omitempty"`
	// KubeVersion is a SemVer constraint specifying the version of Kubernetes required.
	KubeVersion string `yaml:"kubeVersion,omitempty"`
	// Dependencies are a list of dependencies for a chart.
	Dependencies []*Dependency `yaml:"dependencies,omitempty"`
	// Specifies the chart type: application or library
	Type string `yaml:"type,omitempty"`
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
