package schema

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestHasSchemaAnnotation(t *testing.T) {
	tests := []struct {
		name    string
		comment string
		want    bool
	}{
		{
			name:    "empty comment",
			comment: "",
			want:    false,
		},
		{
			name:    "no schema annotation",
			comment: "# This is a normal comment",
			want:    false,
		},
		{
			name:    "has schema annotation",
			comment: "# @schema\n# type: string\n# @schema",
			want:    true,
		},
		{
			name:    "has schema.root only",
			comment: "# @schema.root\n# title: foo\n# @schema.root",
			want:    false,
		},
		{
			name:    "has both schema and schema.root",
			comment: "# @schema.root\n# title: foo\n# @schema.root\n# @schema\n# type: string\n# @schema",
			want:    true,
		},
		{
			name:    "schema prefix but not exact",
			comment: "# @schema.something",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasSchemaAnnotation(tt.comment)
			if got != tt.want {
				t.Errorf("HasSchemaAnnotation(%q) = %v, want %v", tt.comment, got, tt.want)
			}
		})
	}
}

func TestTypeAnnotationFromTag(t *testing.T) {
	tests := []struct {
		tag  string
		want string
	}{
		{"!!null", `"null"`},
		{"!!bool", "boolean"},
		{"!!str", "string"},
		{"!!int", "integer"},
		{"!!float", "number"},
		{"!!timestamp", "string"},
		{"!!seq", "array"},
		{"!!map", "object"},
		{"!!unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			got := typeAnnotationFromTag(tt.tag)
			if got != tt.want {
				t.Errorf("typeAnnotationFromTag(%q) = %q, want %q", tt.tag, got, tt.want)
			}
		})
	}
}

func TestCollectInsertionPoints(t *testing.T) {
	tests := []struct {
		name       string
		yaml       string
		wantCount  int
		wantTypes  []string
	}{
		{
			name:      "simple flat yaml",
			yaml:      "name: hello\nport: 80\nenabled: true\n",
			wantCount: 3,
			wantTypes: []string{"string", "integer", "boolean"},
		},
		{
			name:      "nested objects",
			yaml:      "service:\n  type: ClusterIP\n  port: 80\n",
			wantCount: 3,
			wantTypes: []string{"object", "string", "integer"},
		},
		{
			name:      "already annotated key",
			yaml:      "# @schema\n# type: string\n# @schema\nname: hello\nport: 80\n",
			wantCount: 1,
			wantTypes: []string{"integer"},
		},
		{
			name:      "null value",
			yaml:      "key:\n",
			wantCount: 1,
			wantTypes: []string{`"null"`},
		},
		{
			name:      "array value",
			yaml:      "items: []\n",
			wantCount: 1,
			wantTypes: []string{"array"},
		},
		{
			name:      "empty map value",
			yaml:      "config: {}\n",
			wantCount: 1,
			wantTypes: []string{"object"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var doc yaml.Node
			if err := yaml.Unmarshal([]byte(tt.yaml), &doc); err != nil {
				t.Fatalf("failed to parse YAML: %v", err)
			}
			points := collectInsertionPoints(&doc)
			if len(points) != tt.wantCount {
				t.Errorf("got %d insertion points, want %d", len(points), tt.wantCount)
				for _, p := range points {
					t.Logf("  line=%d type=%s", p.Line, p.TypeStr)
				}
			}
			for i, wantType := range tt.wantTypes {
				if i >= len(points) {
					break
				}
				if points[i].TypeStr != wantType {
					t.Errorf("point[%d].TypeStr = %q, want %q", i, points[i].TypeStr, wantType)
				}
			}
		})
	}
}

func TestAnnotateContent(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "simple unannotated file",
			input: "port: 80\n",
			want:  "# @schema\n# type: integer\n# @schema\nport: 80\n",
		},
		{
			name:  "multiple keys",
			input: "name: hello\nport: 80\n",
			want:  "# @schema\n# type: string\n# @schema\nname: hello\n# @schema\n# type: integer\n# @schema\nport: 80\n",
		},
		{
			name:  "already fully annotated",
			input: "# @schema\n# type: integer\n# @schema\nport: 80\n",
			want:  "# @schema\n# type: integer\n# @schema\nport: 80\n",
		},
		{
			name:  "partially annotated",
			input: "# @schema\n# type: string\n# @schema\nname: hello\nport: 80\n",
			want:  "# @schema\n# type: string\n# @schema\nname: hello\n# @schema\n# type: integer\n# @schema\nport: 80\n",
		},
		{
			name:  "nested objects with indentation",
			input: "service:\n  type: ClusterIP\n  port: 80\n",
			want:  "# @schema\n# type: object\n# @schema\nservice:\n  # @schema\n  # type: string\n  # @schema\n  type: ClusterIP\n  # @schema\n  # type: integer\n  # @schema\n  port: 80\n",
		},
		{
			name:  "key with existing comment - annotation goes above comment",
			input: "# This is the port\nport: 80\n",
			want:  "# @schema\n# type: integer\n# @schema\n# This is the port\nport: 80\n",
		},
		{
			name:  "empty file",
			input: "",
			want:  "",
		},
		{
			name:  "document separator preserved",
			input: "---\nport: 80\n",
			want:  "---\n# @schema\n# type: integer\n# @schema\nport: 80\n",
		},
		{
			name:  "boolean value",
			input: "enabled: true\n",
			want:  "# @schema\n# type: boolean\n# @schema\nenabled: true\n",
		},
		{
			name:  "null value",
			input: "key:\n",
			want:  "# @schema\n# type: \"null\"\n# @schema\nkey:\n",
		},
		{
			name:  "array value",
			input: "items: []\n",
			want:  "# @schema\n# type: array\n# @schema\nitems: []\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := AnnotateContent([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Fatalf("AnnotateContent() error = %v, wantErr %v", err, tt.wantErr)
			}
			if string(got) != tt.want {
				t.Errorf("AnnotateContent() mismatch\ngot:\n%s\nwant:\n%s", string(got), tt.want)
				// Show diff line by line
				gotLines := strings.Split(string(got), "\n")
				wantLines := strings.Split(tt.want, "\n")
				maxLen := len(gotLines)
				if len(wantLines) > maxLen {
					maxLen = len(wantLines)
				}
				for i := 0; i < maxLen; i++ {
					var g, w string
					if i < len(gotLines) {
						g = gotLines[i]
					}
					if i < len(wantLines) {
						w = wantLines[i]
					}
					if g != w {
						t.Errorf("  line %d: got=%q want=%q", i, g, w)
					}
				}
			}
		})
	}
}
