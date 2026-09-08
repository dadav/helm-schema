package chart

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveGraph(t *testing.T) {
	type candidate struct {
		path    string
		version string
	}
	for _, test := range []struct {
		name       string
		repository string
		version    string
		candidates []candidate
		want       string
		wantError  string
	}{
		{name: "installed before file source", repository: "file://../source", version: "1.0.0", candidates: []candidate{{"parent/charts/dep", "1.0.0"}, {"source", "1.0.0"}}, want: "parent/charts/dep"},
		{name: "explicit before legacy child", repository: "file://../source", candidates: []candidate{{"parent/legacy", "1.0.0"}, {"source", "1.0.0"}}, want: "source"},
		{name: "legacy before global", candidates: []candidate{{"parent/legacy", "1.0.0"}, {"other", "1.0.0"}}, want: "parent/legacy"},
		{name: "unique global fallback", candidates: []candidate{{"other", "1.0.0"}}, want: "other"},
		{name: "file path normalization", repository: "file://../source/../source", candidates: []candidate{{"source", "1.0.0"}, {"other", "1.0.0"}}, want: "source"},
		{name: "version range", version: ">= 2.0.0, < 3.0.0", candidates: []candidate{{"parent/charts/old", "1.0.0"}, {"parent/charts/current", "2.1.0"}}, want: "parent/charts/current"},
		{name: "prerelease excluded", version: ">= 1.0.0", candidates: []candidate{{"parent/charts/dep", "2.0.0-beta.1"}}},
		{name: "prerelease included", version: ">= 1.0.0-0", candidates: []candidate{{"parent/charts/dep", "2.0.0-beta.1"}}, want: "parent/charts/dep"},
		{name: "omitted version", candidates: []candidate{{"other", ""}}, want: "other"},
		{name: "unresolved", version: "2.0.0", candidates: []candidate{{"other", "1.0.0"}}},
		{name: "invalid candidate version", version: "1.0.0", candidates: []candidate{{"other", "invalid"}}},
		{name: "invalid constraint", version: "invalid", wantError: "invalid version constraint"},
		{name: "ambiguous global", candidates: []candidate{{"first", "1.0.0"}, {"second", "1.0.0"}}, wantError: "ambiguous dependency"},
		{name: "ambiguous installed", repository: "file://../source", candidates: []candidate{{"parent/charts/first", "1.0.0"}, {"parent/charts/second", "1.0.0"}, {"source", "1.0.0"}}, wantError: "ambiguous dependency"},
		{name: "ambiguous range", version: "^1.0.0", candidates: []candidate{{"first", "1.0.0"}, {"second", "1.1.0"}}, wantError: "ambiguous dependency"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			parentPath := filepath.Join(root, "parent", "Chart.yaml")
			dep := &Dependency{Name: "common", Version: test.version, Repository: test.repository}
			instances := []Instance{{Path: parentPath, Chart: ChartFile{Name: "parent", Dependencies: []*Dependency{dep}}}}
			for _, candidate := range test.candidates {
				instances = append(instances, Instance{Path: filepath.Join(root, candidate.path, "Chart.yaml"), Chart: ChartFile{Name: "common", Version: candidate.version}})
			}
			graph, err := ResolveGraph(instances, nil)
			slices.Reverse(instances)
			reversed, reversedErr := ResolveGraph(instances, nil)
			if test.wantError != "" {
				require.ErrorContains(t, err, test.wantError)
				require.Error(t, reversedErr)
				assert.Equal(t, err.Error(), reversedErr.Error())
				assert.Contains(t, err.Error(), parentPath)
				return
			}
			require.NoError(t, err)
			require.NoError(t, reversedErr)
			assert.Equal(t, graph, reversed)
			want := ""
			if test.want != "" {
				want = filepath.Join(root, test.want, "Chart.yaml")
				assert.True(t, graph.IsDependency[want])
			}
			assert.Equal(t, []string{want}, graph.Dependencies[parentPath])
		})
	}
}

func TestResolveGraphAliasesVersionsAndFilter(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "Chart.yaml")
	old := filepath.Join(root, "charts", "old", "Chart.yaml")
	current := filepath.Join(root, "charts", "current", "Chart.yaml")
	instances := []Instance{
		{Path: parent, Chart: ChartFile{Name: "parent", Dependencies: []*Dependency{
			{Name: "common", Alias: "old", Version: "1.0.0"},
			{Name: "common", Alias: "current", Version: "2.0.0"},
			{Name: "common", Alias: "replica", Version: "2.0.0"},
		}}},
		{Path: old, Chart: ChartFile{Name: "common", Version: "1.0.0"}},
		{Path: current, Chart: ChartFile{Name: "common", Version: "2.0.0"}},
	}
	graph, err := ResolveGraph(instances, nil)
	require.NoError(t, err)
	assert.Equal(t, []string{old, current, current}, graph.Dependencies[parent])
	graph, err = ResolveGraph(instances, map[string]bool{"different": true})
	require.NoError(t, err)
	assert.Equal(t, []string{"", "", ""}, graph.Dependencies[parent])
	assert.Empty(t, graph.IsDependency)
}

func TestResolveGraphUsesNearestOwner(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "Chart.yaml")
	child := filepath.Join(root, "charts", "child", "Chart.yaml")
	grandchild := filepath.Join(root, "charts", "child", "charts", "common", "Chart.yaml")
	direct := filepath.Join(root, "charts", "common", "Chart.yaml")
	instances := []Instance{
		{Path: parent, Chart: ChartFile{Name: "parent", Dependencies: []*Dependency{{Name: "common"}}}},
		{Path: child, Chart: ChartFile{Name: "child", Dependencies: []*Dependency{{Name: "common"}}}},
		{Path: grandchild, Chart: ChartFile{Name: "common"}},
		{Path: direct, Chart: ChartFile{Name: "common"}},
	}
	graph, err := ResolveGraph(instances, nil)
	require.NoError(t, err)
	assert.Equal(t, []string{direct}, graph.Dependencies[parent])
	assert.Equal(t, []string{grandchild}, graph.Dependencies[child])
}

func TestResolveGraphSameNameDoesNotImplySelfDependency(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Chart.yaml")
	for _, repository := range []string{"", "file://."} {
		graph, err := ResolveGraph([]Instance{{Path: path, Chart: ChartFile{
			Name: "common", Version: "1.0.0", Dependencies: []*Dependency{{Name: "common", Version: "1.0.0", Repository: repository}},
		}}}, nil)
		require.NoError(t, err)
		want := ""
		if repository != "" {
			want = path
		}
		assert.Equal(t, []string{want}, graph.Dependencies[path])
	}
}
