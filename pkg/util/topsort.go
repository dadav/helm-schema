package util

import (
	"fmt"

	mapset "github.com/deckarep/golang-set/v2"
)

// TopSort uses topological sorting on the given array of generic type R
func TopSort[R any, I comparable](results []R, identify func(i R) I, getDependenciesFromResult func(d R) []I) ([]R, error) {
	lookup := make(map[I]R)
	todo := make(map[I]mapset.Set[I])

	for _, result := range results {
		lookup[identify(result)] = result
		dependencies := mapset.NewSet[I]()
		for _, dep := range getDependenciesFromResult(result) {
			dependencies.Add(dep)
		}
		todo[identify(result)] = dependencies
	}

	var sorted []R

	// if we have work left
	for len(todo) != 0 {
		ready := mapset.NewSet[I]()
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
		}

		// remove ready items from deps list too
		for name, deps := range todo {
			// Problem!
			diff := deps.Difference(ready)
			todo[name] = diff
		}
	}
	return sorted, nil
}
