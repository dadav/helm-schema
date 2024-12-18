package schema

import (
	"fmt"
)

// TopoSort uses topological sorting to sort the results
func TopoSort(results []*Result, dependenciesFilter map[string]bool) ([]*Result, error) {
	// Map chart names to their Result objects for easy lookup
	chartMap := make(map[string]*Result)
	for _, r := range results {
		if r.Chart != nil {
			chartMap[r.Chart.Name] = r
		}
	}

	// Build dependency graph as adjacency list
	// Key is chart name, value is slice of dependency names
	deps := make(map[string][]string)
	for _, r := range results {
		if r.Chart == nil {
			continue
		}

		// Add each dependency as an edge in the graph
		for _, dep := range r.Chart.Dependencies {
			// Skip if dependency filtering is enabled and this dep isn't included
			if len(dependenciesFilter) > 0 && !dependenciesFilter[dep.Name] {
				continue
			}
			deps[r.Chart.Name] = append(deps[r.Chart.Name], dep.Name)
		}
	}

	// Track visited nodes during traversal
	visited := make(map[string]bool)
	// Track nodes in current recursion stack to detect cycles
	inStack := make(map[string]bool)
	// Final sorted results
	var sorted []*Result

	// Recursive DFS helper function
	var visit func(string) error
	visit = func(chart string) error {
		// Return if already visited
		if visited[chart] {
			return nil
		}

		// Check for cycle
		if inStack[chart] {
			return &CircularError{fmt.Sprintf("circular dependency detected: %s", chart)}
		}

		// Mark as being visited
		inStack[chart] = true
		visited[chart] = true

		// Visit all dependencies first
		for _, dep := range deps[chart] {
			if err := visit(dep); err != nil {
				return err
			}
		}

		// Add to sorted results after dependencies
		if result, exists := chartMap[chart]; exists {
			sorted = append(sorted, result)
		}

		// Remove from recursion stack
		inStack[chart] = false
		return nil
	}

	// Visit all charts
	for _, r := range results {
		if r.Chart != nil {
			if err := visit(r.Chart.Name); err != nil {
				return results, err
			}
		}
	}

	return sorted, nil
}
