package schema

import (
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/dadav/helm-schema/pkg/chart"
	mapset "github.com/deckarep/golang-set/v2"
)

// TopoSort uses topological sorting to sort the results
func TopoSort(results []*Result) ([]*Result, error) {
	// Map result identifier to result
	lookup := make(map[string]*Result)

	// Map result identifier to dependencies identifiers
	todo := make(map[string]mapset.Set[chart.Dependency])

	// Create the work queue
	for _, result := range results {
		dependencies := mapset.NewSet[chart.Dependency]()
		for _, dep := range result.Chart.Dependencies {
			dependencies.Add(*dep)
		}
		resultId := fmt.Sprintf("%s|%s", result.Chart.Name, result.Chart.Version)
		if _, ok := lookup[resultId]; ok {
			return nil, fmt.Errorf("duplicate chart found: %s - consider changing the name or version, so that helm-schema can distinguish them", result.Chart.Name)
		}
		lookup[resultId] = result
		todo[resultId] = dependencies
	}

	var sorted []*Result

	// if we have work left
	for len(todo) != 0 {
		ready := mapset.NewSet[string]()
		for name, deps := range todo {
			// if no further deps found (end of the dep tree), this chart is ready
			if deps.Cardinality() == 0 {
				ready.Add(name)
			}
		}

		// if no items are ready, we are stuck
		if ready.Cardinality() == 0 {
			// append unsorted to sorted items and return them
			for name := range todo {
				sorted = append(sorted, lookup[name])
			}

			return sorted, &CircularError{fmt.Sprintf("circular or missing dependency found: %v - Please build and untar all your helm dependencies: helm dep build && ls charts/*.tgz |xargs -n1 tar -C charts/ -xzf", todo)}
		}

		// remove ready items from todo list and add to sorted list
		for name := range ready.Iter() {
			delete(todo, name)
			sorted = append(sorted, lookup[name])

			// remove ready items from deps list too
			for todoName, deps := range todo {
				newDeps := deps.Clone()
				for dep := range deps.Iter() {
					c, err := semver.NewConstraint(dep.Version)
					if err != nil {
						return sorted, err
					}

					nameVersion := strings.Split(name, "|")
					sem, err := semver.NewVersion(nameVersion[1])
					if err != nil {
						return sorted, err
					}

					if c.Check(sem) {
						newDeps.Remove(dep)
					}
				}
				todo[todoName] = newDeps
			}
		}

	}
	return sorted, nil
}
