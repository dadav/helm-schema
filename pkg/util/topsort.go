package util

import (
	"errors"
	mapset "github.com/deckarep/golang-set/v2"
)

// TopSort uses topological sorting on the given array of generic type R
func TopSort[R any, I comparable](results []R, identify func(i R) I, dependencies func(d R) []I) ([]R, error) {
	depNamesToResults := make(map[I]R)
	depNamesToNames := make(map[I]mapset.Set[I])

	for _, result := range results {
		depNamesToResults[identify(result)] = result
		dependencySet := mapset.NewSet[I]()
		for _, dep := range dependencies(result) {
			dependencySet.Add(dep)
		}
		depNamesToNames[identify(result)] = dependencySet
	}

	var sorted []R

	for len(depNamesToNames) != 0 {
		readySet := mapset.NewSet[I]()
		for name, deps := range depNamesToNames {
			if deps.Cardinality() == 0 {
				readySet.Add(name)
			}
		}

		if readySet.Cardinality() == 0 {
			var g []R
			for name := range depNamesToNames {
				g = append(g, depNamesToResults[name])
			}

			return g, errors.New("Circular dependency found")
		}

		for name := range readySet.Iter() {
			delete(depNamesToNames, name)
			sorted = append(sorted, depNamesToResults[name])
		}

		for name, deps := range depNamesToNames {
			diff := deps.Difference(readySet)
			depNamesToNames[name] = diff
		}
	}
	return sorted, nil
}
