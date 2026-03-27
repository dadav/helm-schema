package schema

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// mergeValuesDocuments merges YAML documents using Helm-style precedence:
// later files override earlier files, and nested mappings merge recursively.
func mergeValuesDocuments(base *yaml.Node, overlay *yaml.Node) (*yaml.Node, error) {
	if base == nil {
		return cloneYAMLNode(overlay), nil
	}
	if overlay == nil {
		return cloneYAMLNode(base), nil
	}
	if base.Kind != yaml.DocumentNode || overlay.Kind != yaml.DocumentNode {
		return nil, fmt.Errorf("expected yaml document nodes, got %d and %d", base.Kind, overlay.Kind)
	}
	if len(base.Content) != 1 || len(overlay.Content) != 1 {
		return nil, fmt.Errorf("unexpected yaml document structure while merging values")
	}

	merged := cloneYAMLNode(base)
	merged.HeadComment = mergeCommentText(merged.HeadComment, overlay.HeadComment)
	merged.LineComment = mergeCommentText(merged.LineComment, overlay.LineComment)
	merged.FootComment = mergeCommentText(merged.FootComment, overlay.FootComment)

	mergedContent, err := mergeValuesNodes(merged.Content[0], overlay.Content[0])
	if err != nil {
		return nil, err
	}
	merged.Content[0] = mergedContent

	return merged, nil
}

func mergeValuesNodes(base *yaml.Node, overlay *yaml.Node) (*yaml.Node, error) {
	if base == nil {
		return cloneYAMLNode(overlay), nil
	}
	if overlay == nil {
		return cloneYAMLNode(base), nil
	}

	if base.Kind == yaml.AliasNode && base.Alias != nil {
		base = base.Alias
	}
	if overlay.Kind == yaml.AliasNode && overlay.Alias != nil {
		overlay = overlay.Alias
	}

	if base.Kind == yaml.MappingNode && overlay.Kind == yaml.MappingNode {
		return mergeMappingNodes(base, overlay)
	}

	replacement := cloneYAMLNode(overlay)
	replacement.HeadComment = mergeCommentText(base.HeadComment, overlay.HeadComment)
	replacement.LineComment = mergeCommentText(base.LineComment, overlay.LineComment)
	replacement.FootComment = mergeCommentText(base.FootComment, overlay.FootComment)
	return replacement, nil
}

func mergeMappingNodes(base *yaml.Node, overlay *yaml.Node) (*yaml.Node, error) {
	merged := cloneYAMLNode(base)
	merged.Content = nil
	merged.HeadComment = mergeCommentText(base.HeadComment, overlay.HeadComment)
	merged.LineComment = mergeCommentText(base.LineComment, overlay.LineComment)
	merged.FootComment = mergeCommentText(base.FootComment, overlay.FootComment)

	overlayIndex := make(map[string]int, len(overlay.Content)/2)
	for i := 0; i+1 < len(overlay.Content); i += 2 {
		overlayIndex[overlay.Content[i].Value] = i
	}

	usedOverlayKeys := make(map[string]bool, len(overlayIndex))

	for i := 0; i+1 < len(base.Content); i += 2 {
		baseKey := base.Content[i]
		baseValue := base.Content[i+1]
		overlayPos, exists := overlayIndex[baseKey.Value]
		if !exists {
			merged.Content = append(merged.Content, cloneYAMLNode(baseKey), cloneYAMLNode(baseValue))
			continue
		}

		overlayKey := overlay.Content[overlayPos]
		overlayValue := overlay.Content[overlayPos+1]
		usedOverlayKeys[baseKey.Value] = true

		mergedKey := cloneYAMLNode(baseKey)
		mergedKey.HeadComment = mergeCommentText(baseKey.HeadComment, overlayKey.HeadComment)
		mergedKey.LineComment = mergeCommentText(baseKey.LineComment, overlayKey.LineComment)
		mergedKey.FootComment = mergeCommentText(baseKey.FootComment, overlayKey.FootComment)

		mergedValue, err := mergeValuesNodes(baseValue, overlayValue)
		if err != nil {
			return nil, err
		}

		merged.Content = append(merged.Content, mergedKey, mergedValue)
	}

	for i := 0; i+1 < len(overlay.Content); i += 2 {
		overlayKey := overlay.Content[i]
		if usedOverlayKeys[overlayKey.Value] {
			continue
		}
		merged.Content = append(merged.Content, cloneYAMLNode(overlayKey), cloneYAMLNode(overlay.Content[i+1]))
	}

	return merged, nil
}

func cloneYAMLNode(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}

	cloned := *node
	if node.Content != nil {
		cloned.Content = make([]*yaml.Node, len(node.Content))
		for i, child := range node.Content {
			cloned.Content[i] = cloneYAMLNode(child)
		}
	}
	if node.Alias != nil {
		cloned.Alias = cloneYAMLNode(node.Alias)
	}

	return &cloned
}

func mergeCommentText(base string, overlay string) string {
	if overlay != "" {
		return overlay
	}
	return base
}
