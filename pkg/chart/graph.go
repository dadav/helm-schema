package chart

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Masterminds/semver/v3"
)

// Instance identifies a chart source independently of its metadata name.
// For archives, ID includes the archive source and member path; Path points
// into temporary extraction storage and must only be used for file access.
type Instance struct {
	ID       string
	Path     string
	OwnerID  string
	Chart    ChartFile
	Archived bool
}

// ChartOwners finds the nearest containing chart from normalized source IDs.
// Discovery records this before filtering so omitted parents do not reparent
// their children. Previously recorded owners remain authoritative.
func ChartOwners(instances []Instance) map[string]string {
	directoryIDs := make(map[string]string, len(instances))
	for _, instance := range instances {
		directoryIDs[filepath.Dir(instance.ID)] = instance.ID
	}
	owners := make(map[string]string, len(instances))
	for _, instance := range instances {
		if instance.OwnerID != "" {
			owners[instance.ID] = instance.OwnerID
			continue
		}
		for dir := filepath.Dir(filepath.Dir(instance.ID)); ; dir = filepath.Dir(dir) {
			if owner, ok := directoryIDs[dir]; ok {
				owners[instance.ID] = owner
				break
			}
			if filepath.Dir(dir) == dir {
				break
			}
		}
	}
	return owners
}

type Graph struct {
	Charts       map[string]Instance
	IDs          []string
	IDByPath     map[string]string
	Dependencies map[string][]string
	IsDependency map[string]bool
}

// ResolveGraph binds dependency declarations once, before values are processed.
// Dependencies retains declaration indexes, including empty entries for missing
// or filtered dependencies. Aliases do not change the identity of a source.
func ResolveGraph(instances []Instance, filter map[string]bool) (*Graph, error) {
	graph := &Graph{
		Charts:       make(map[string]Instance),
		IDByPath:     make(map[string]string),
		Dependencies: make(map[string][]string),
		IsDependency: make(map[string]bool),
	}
	normalized := make([]Instance, 0, len(instances))
	for _, instance := range instances {
		if instance.Path == "" {
			return nil, fmt.Errorf("chart %q has no source path", instance.Chart.Name)
		}
		path, err := filepath.Abs(instance.Path)
		if err != nil {
			return nil, fmt.Errorf("normalize chart path %q: %w", instance.Path, err)
		}
		instance.Path = path
		if instance.ID == "" {
			instance.ID = path
		}
		instance.ID, err = filepath.Abs(instance.ID)
		if err != nil {
			return nil, fmt.Errorf("normalize chart identity %q: %w", instance.ID, err)
		}
		if _, exists := graph.Charts[instance.ID]; exists {
			return nil, fmt.Errorf("duplicate chart source: %s", instance.ID)
		}
		graph.Charts[instance.ID] = instance
		graph.IDs = append(graph.IDs, instance.ID)
		graph.IDByPath[path] = instance.ID
		normalized = append(normalized, instance)
	}
	slices.Sort(graph.IDs)

	owners := ChartOwners(normalized)
	byName := make(map[string][]string)
	for _, id := range graph.IDs {
		byName[graph.Charts[id].Chart.Name] = append(byName[graph.Charts[id].Chart.Name], id)
	}

	for _, id := range graph.IDs {
		parent := graph.Charts[id]
		bindings := make([]string, len(parent.Chart.Dependencies))
		for index, dep := range parent.Chart.Dependencies {
			if dep == nil || dep.Name == "" || (len(filter) > 0 && !filter[dep.Name]) {
				continue
			}
			child, err := resolveDependency(parent, dep, graph.Charts, byName[dep.Name], owners)
			if err != nil {
				return nil, err
			}
			bindings[index] = child
			if child != "" {
				graph.IsDependency[child] = true
			}
		}
		graph.Dependencies[id] = bindings
	}
	return graph, nil
}

func resolveDependency(parent Instance, dep *Dependency, charts map[string]Instance, candidates []string, owners map[string]string) (string, error) {
	var constraint *semver.Constraints
	if dep.Version != "" {
		var err error
		constraint, err = semver.NewConstraint(dep.Version)
		if err != nil {
			return "", fmt.Errorf("chart %s dependency %q (alias %q) has invalid version constraint %q: %w", parent.ID, dep.Name, dep.Alias, dep.Version, err)
		}
	}
	localID := ""
	if strings.HasPrefix(dep.Repository, "file://") {
		localPath := strings.TrimPrefix(dep.Repository, "file://")
		if !filepath.IsAbs(localPath) {
			localPath = filepath.Join(filepath.Dir(parent.ID), localPath)
		}
		localID = filepath.Join(localPath, "Chart.yaml")
	}

	// The first nonempty tier wins: installed, explicit file source, legacy
	// local child, then unique repository-wide fallback.
	tiers := [4][]string{}
	for _, id := range candidates {
		// A matching metadata name does not make the owner its own subchart.
		if id == parent.ID && id != localID {
			continue
		}
		candidate := charts[id]
		if constraint != nil {
			version, err := semver.NewVersion(candidate.Chart.Version)
			if err != nil || !constraint.Check(version) {
				continue
			}
		}
		tier := 3
		if owners[id] == parent.ID {
			tier = 2
			rel, err := filepath.Rel(filepath.Join(filepath.Dir(parent.ID), "charts"), id)
			if err == nil && filepath.IsLocal(rel) {
				tier = 0
			}
		}
		if id == localID && tier > 1 {
			tier = 1
		}
		tiers[tier] = append(tiers[tier], id)
	}
	for _, matches := range tiers {
		if len(matches) > 1 {
			return "", fmt.Errorf("ambiguous dependency %q (version %q, alias %q, repository %q) in chart %s; matching charts: %s", dep.Name, dep.Version, dep.Alias, dep.Repository, parent.ID, strings.Join(matches, ", "))
		}
		if len(matches) == 1 {
			return matches[0], nil
		}
	}
	return "", nil
}
