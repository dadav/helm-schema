package schema

import "strings"

// WithDefinitionPrefix isolates a schema's definitions before embedding it in another document.
// It copies schema nodes so references in other parents and the source stay unchanged.
func (s *Schema) WithDefinitionPrefix(prefix string) *Schema {
	if s == nil || len(s.Definitions) == 0 {
		return s
	}
	renamed := make(map[string]string, len(s.Definitions))
	for name := range s.Definitions {
		renamed[name] = prefix + "/" + name
	}
	copy := s.copyWithRefs(func(ref string) string {
		pointer, ok := strings.CutPrefix(ref, "#/definitions/")
		if !ok {
			return ref
		}
		name, rest, hasRest := strings.Cut(pointer, "/")
		name = strings.NewReplacer("~1", "/", "~0", "~").Replace(name)
		if replacement, ok := renamed[name]; ok {
			ref = "#/definitions/" + strings.NewReplacer("~", "~0", "/", "~1").Replace(replacement)
			if hasRest {
				ref += "/" + rest
			}
		}
		return ref
	})
	definitions := make(map[string]*Schema, len(copy.Definitions))
	for name, definition := range copy.Definitions {
		definitions[renamed[name]] = definition
	}
	copy.Definitions = definitions
	return copy
}

func (s *Schema) copyWithRefs(rewrite func(string) string) *Schema {
	if s == nil {
		return nil
	}
	copy := *s
	copy.Ref = rewrite(s.Ref)
	copyMap := func(source map[string]*Schema) map[string]*Schema {
		if source == nil {
			return nil
		}
		result := make(map[string]*Schema, len(source))
		for name, child := range source {
			result[name] = child.copyWithRefs(rewrite)
		}
		return result
	}
	copyList := func(source []*Schema) []*Schema {
		if source == nil {
			return nil
		}
		result := make([]*Schema, len(source))
		for i, child := range source {
			result[i] = child.copyWithRefs(rewrite)
		}
		return result
	}
	copy.Properties = copyMap(s.Properties)
	copy.PatternProperties = copyMap(s.PatternProperties)
	copy.Definitions = copyMap(s.Definitions)
	copy.AllOf = copyList(s.AllOf)
	copy.AnyOf = copyList(s.AnyOf)
	copy.OneOf = copyList(s.OneOf)
	copy.Items = s.Items.copyWithRefs(rewrite)
	copy.Contains = s.Contains.copyWithRefs(rewrite)
	copy.PropertyNames = s.PropertyNames.copyWithRefs(rewrite)
	copy.If = s.If.copyWithRefs(rewrite)
	copy.Then = s.Then.copyWithRefs(rewrite)
	copy.Else = s.Else.copyWithRefs(rewrite)
	copy.Not = s.Not.copyWithRefs(rewrite)
	copy.AdditionalProperties = copySchemaValue(s.AdditionalProperties, rewrite)
	copy.AdditionalItems = copySchemaValue(s.AdditionalItems, rewrite)
	if s.Dependencies != nil {
		copy.Dependencies = make(map[string]interface{}, len(s.Dependencies))
		for name, dependency := range s.Dependencies {
			copy.Dependencies[name] = copySchemaValue(dependency, rewrite)
		}
	}
	return &copy
}

func copySchemaValue(value interface{}, rewrite func(string) string) interface{} {
	switch value := value.(type) {
	case *Schema:
		return value.copyWithRefs(rewrite)
	case Schema:
		return *value.copyWithRefs(rewrite)
	case map[string]interface{}:
		copy := make(map[string]interface{}, len(value))
		for keyword, child := range value {
			switch keyword {
			case "$ref":
				if ref, ok := child.(string); ok {
					child = rewrite(ref)
				}
			case "properties", "patternProperties", "definitions", "$defs", "dependencies":
				if children, ok := child.(map[string]interface{}); ok {
					result := make(map[string]interface{}, len(children))
					for name, schema := range children {
						result[name] = copySchemaValue(schema, rewrite)
					}
					child = result
				}
			case "items", "additionalItems", "additionalProperties", "contains", "propertyNames", "not", "if", "then", "else", "allOf", "anyOf", "oneOf":
				child = copySchemaValue(child, rewrite)
			}
			copy[keyword] = child
		}
		return copy
	case []interface{}:
		copy := make([]interface{}, len(value))
		for i, child := range value {
			copy[i] = copySchemaValue(child, rewrite)
		}
		return copy
	default:
		return value
	}
}
