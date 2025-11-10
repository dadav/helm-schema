package schema

import (
	"fmt"
)

// TopoSort uses topological sorting to sort the results
// If allowCircular is true, circular dependencies will be logged as warnings and results will be returned unsorted
func TopoSort(results []*Result, allowCircular bool) ([]*Result, error) {
	// Map chart names to their Result objects for easy lookup
	chartMap := make(map[string]*Result)
	for _, r := range results {
		if r.Chart != nil {
			chartMap[r.Chart.Name] = r
		}
	}

	// Build dependency graph as adjacency list
	deps := make(map[string][]string)

	// Build dependency graph
	for _, r := range results {
		if r.Chart == nil {
			continue
		}

		// Initialize empty dependency list
		deps[r.Chart.Name] = []string{}

		// Add all dependencies
		for _, dep := range r.Chart.Dependencies {
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
		// Check for cycle first, before the visited check
		if inStack[chart] {
			return &CircularError{fmt.Sprintf("circular dependency detected: %s", chart)}
		}

		// Return if already visited
		if visited[chart] {
			return nil
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
				if allowCircular {
					// Return unsorted results when circular dependencies are allowed
					return results, nil
				}
				return nil, err
			}
		}
	}

	return sorted, nil
}
