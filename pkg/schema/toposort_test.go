package schema

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/dadav/helm-schema/pkg/chart"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTopoSort(t *testing.T) {
	for _, test := range []struct {
		name         string
		names        []string
		dependencies map[string][]string
		want         []string
		cycle        bool
	}{
		{name: "chain", names: []string{"A", "B", "C"}, dependencies: map[string][]string{"A": {"B"}, "B": {"C"}}, want: []string{"C", "B", "A"}},
		{name: "diamond", names: []string{"A", "B", "C", "D"}, dependencies: map[string][]string{"A": {"B", "C"}, "B": {"D"}, "C": {"D"}}, want: []string{"D", "B", "C", "A"}},
		{name: "cycle", names: []string{"A", "B"}, dependencies: map[string][]string{"A": {"B"}, "B": {"A"}}, want: []string{"A", "B"}, cycle: true},
		{name: "missing", names: []string{"A", "B"}, dependencies: map[string][]string{"A": {"missing"}}, want: []string{"A", "B"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			var instances []chart.Instance
			var results []*Result
			for _, name := range test.names {
				metadata := chart.ChartFile{Name: name}
				for _, dep := range test.dependencies[name] {
					metadata.Dependencies = append(metadata.Dependencies, &chart.Dependency{Name: dep})
				}
				path := filepath.Join(root, name, "Chart.yaml")
				instances = append(instances, chart.Instance{Path: path, Chart: metadata})
				results = append(results, &Result{ChartPath: path, Chart: &metadata})
			}
			graph, err := chart.ResolveGraph(instances, nil)
			require.NoError(t, err)
			var checkPermutations func(int)
			checkPermutations = func(index int) {
				if index < len(results) {
					for next := index; next < len(results); next++ {
						results[index], results[next] = results[next], results[index]
						checkPermutations(index + 1)
						results[index], results[next] = results[next], results[index]
					}
					return
				}
				for _, allowCircular := range []bool{false, true} {
					sorted, err := TopoSort(results, graph, allowCircular)
					if test.cycle && !allowCircular {
						var circular *CircularError
						require.ErrorAs(t, err, &circular)
						assert.Contains(t, err.Error(), filepath.Join(root, "A", "Chart.yaml"))
						assert.Contains(t, err.Error(), filepath.Join(root, "B", "Chart.yaml"))
						continue
					}
					require.NoError(t, err)
					var names []string
					for _, result := range sorted {
						names = append(names, result.Chart.Name)
					}
					assert.Equal(t, test.want, names)
				}
			}
			checkPermutations(0)
		})
	}
}

func TestTopoSortPreservesDuplicateNamesAndFailures(t *testing.T) {
	root := t.TempDir()
	parent := chart.Instance{Path: filepath.Join(root, "parent", "Chart.yaml"), Chart: chart.ChartFile{Name: "common", Dependencies: []*chart.Dependency{{Name: "common"}}}}
	child := chart.Instance{Path: filepath.Join(root, "parent", "charts", "child", "Chart.yaml"), Chart: chart.ChartFile{Name: "common"}}
	other := chart.Instance{Path: filepath.Join(root, "other", "Chart.yaml"), Chart: chart.ChartFile{Name: "common"}}
	graph, err := chart.ResolveGraph([]chart.Instance{parent, child, other}, nil)
	require.NoError(t, err)
	failed := &Result{ChartPath: other.Path, Errors: []error{errors.New("cannot read chart")}}
	parentResult := &Result{ChartPath: parent.Path, Chart: &parent.Chart}
	childResult := &Result{ChartPath: child.Path, Chart: &child.Chart}
	for _, input := range [][]*Result{{parentResult, failed, childResult}, {childResult, parentResult, failed}} {
		sorted, err := TopoSort(input, graph, false)
		require.NoError(t, err)
		assert.Equal(t, []*Result{failed, childResult, parentResult}, sorted)
	}
}
