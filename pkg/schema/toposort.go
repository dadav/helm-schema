package schema

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/dadav/helm-schema/pkg/chart"
)

// TopoSort orders results using the same resolved bindings as schema merging.
// Allowed cycles return all results in stable source order without ordering
// dependencies. Results with worker errors are retained for the caller.
func TopoSort(results []*Result, graph *chart.Graph, allowCircular bool) ([]*Result, error) {
	byID := make(map[string]*Result, len(results))
	ids := make([]string, 0, len(results))
	for _, result := range results {
		if result == nil {
			continue
		}
		if result.ChartPath == "" {
			return nil, fmt.Errorf("cannot sort chart result without a source path")
		}
		path, err := filepath.Abs(result.ChartPath)
		if err != nil {
			return nil, err
		}
		id := graph.IDByPath[path]
		if id == "" {
			id = path
		}
		if _, exists := byID[id]; exists {
			return nil, fmt.Errorf("duplicate chart result for source %s", id)
		}
		byID[id] = result
		ids = append(ids, id)
	}
	slices.Sort(ids)

	state := make(map[string]int)
	stack := []string{}
	sorted := make([]*Result, 0, len(ids))
	var visit func(string) error
	visit = func(id string) error {
		if state[id] == 1 {
			cycle := append(slices.Clone(stack[slices.Index(stack, id):]), id)
			return &CircularError{fmt.Sprintf("circular dependency detected: %s", strings.Join(cycle, " -> "))}
		}
		if state[id] == 2 {
			return nil
		}
		state[id] = 1
		stack = append(stack, id)
		for _, child := range graph.Dependencies[id] {
			if byID[child] == nil {
				continue
			}
			if err := visit(child); err != nil {
				return err
			}
		}
		stack = stack[:len(stack)-1]
		state[id] = 2
		sorted = append(sorted, byID[id])
		return nil
	}
	for _, id := range ids {
		if err := visit(id); err != nil {
			if !allowCircular {
				return nil, err
			}
			fallback := make([]*Result, 0, len(ids))
			for _, id := range ids {
				fallback = append(fallback, byID[id])
			}
			return fallback, nil
		}
	}
	return sorted, nil
}
