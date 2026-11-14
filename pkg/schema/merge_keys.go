package schema

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

func expandedMappingContent(node *yaml.Node) ([]*yaml.Node, error) {
	active := make(map[*yaml.Node]bool)
	var expand func(*yaml.Node) ([]*yaml.Node, error)
	expand = func(current *yaml.Node) ([]*yaml.Node, error) {
		if current == nil {
			return nil, fmt.Errorf("missing YAML merge target")
		}
		if active[current] {
			return nil, fmt.Errorf("recursive YAML merge at line %d", current.Line)
		}
		active[current] = true
		defer delete(active, current)

		if current.Kind == yaml.AliasNode {
			return expand(current.Alias)
		}

		var content []*yaml.Node
		seen := make(map[string]bool)
		var sources []*yaml.Node
		switch current.Kind {
		case yaml.MappingNode:
			// Explicit keys take precedence regardless of the merge key's position.
			for i := 0; i+1 < len(current.Content); i += 2 {
				key, value := current.Content[i], current.Content[i+1]
				if key.Tag == "!!merge" {
					sources = append(sources, value)
					continue
				}
				content = append(content, key, value)
				seen[key.Value] = true
			}
		case yaml.SequenceNode:
			sources = current.Content
		default:
			return nil, fmt.Errorf("YAML merge target at line %d must be a mapping or sequence of mappings", current.Line)
		}

		for _, source := range sources {
			merged, err := expand(source)
			if err != nil {
				return nil, err
			}
			for i := 0; i+1 < len(merged); i += 2 {
				key := merged[i]
				if !seen[key.Value] {
					content = append(content, key, merged[i+1])
					seen[key.Value] = true
				}
			}
		}
		return content, nil
	}
	return expand(node)
}
