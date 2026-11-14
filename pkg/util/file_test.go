package util

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"
)

func TestReadFileAndFixNewline(t *testing.T) {
	tests := []struct {
		input  string
		output []byte
	}{
		{
			input:  "foo",
			output: []byte("foo"),
		},
		{
			input:  "foo\r\nbar",
			output: []byte("foo\nbar"),
		},
		{
			input:  "foo\r\n\r\nbar",
			output: []byte("foo\n\nbar"),
		},
	}
	for _, test := range tests {
		data := []byte(test.input)
		reader := bytes.NewReader(data)
		content, err := ReadFileAndFixNewline(reader)
		if err != nil {
			t.Errorf("Wasn't expecting an error, but got this: %v", err)
		}
		if !bytes.Equal(content, test.output) {
			t.Errorf("Was expecting %s, but got %s", test.output, content)
		}
	}
}

func TestIsRelativeFile(t *testing.T) {
	tempDir := t.TempDir()
	rootFile := filepath.Join(tempDir, "values.yaml")
	relativeFile := filepath.Join(tempDir, "ref.json")
	relativeDir := filepath.Join(tempDir, "schemas")
	absFile := filepath.Join(tempDir, "absolute.json")

	if err := os.WriteFile(rootFile, []byte("root"), 0o644); err != nil {
		t.Fatalf("failed to create root file: %v", err)
	}
	if err := os.WriteFile(relativeFile, []byte("{}"), 0o644); err != nil {
		t.Fatalf("failed to create relative file: %v", err)
	}
	if err := os.Mkdir(relativeDir, 0o755); err != nil {
		t.Fatalf("failed to create relative dir: %v", err)
	}
	if err := os.WriteFile(absFile, []byte("{}"), 0o644); err != nil {
		t.Fatalf("failed to create absolute file: %v", err)
	}

	tests := []struct {
		name         string
		root         string
		relPath      string
		expectedPath string
		wantErr      bool
	}{
		{
			name:         "existing relative file",
			root:         rootFile,
			relPath:      "ref.json",
			expectedPath: relativeFile,
		},
		{
			name:    "empty path",
			root:    rootFile,
			relPath: "",
			wantErr: true,
		},
		{
			name:         "relative directory",
			root:         rootFile,
			relPath:      "schemas",
			expectedPath: relativeDir,
			wantErr:      true,
		},
		{
			name:    "absolute path",
			root:    rootFile,
			relPath: absFile,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolvedPath, err := IsRelativeFile(tt.root, tt.relPath)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error")
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if resolvedPath != tt.expectedPath {
				t.Fatalf("expected resolved path %q, got %q", tt.expectedPath, resolvedPath)
			}
		})
	}
}

func TestRemoveCommentsFromYamlLongLinesAndReadErrors(t *testing.T) {
	longValue := strings.Repeat("x", 70*1024)
	for _, newline := range []string{"\n", "\r\n"} {
		input := "long: " + longValue + newline + "after: true" + newline
		got, err := RemoveCommentsFromYaml(strings.NewReader(input))
		if err != nil {
			t.Fatal(err)
		}
		want := "long: " + longValue + "\nafter: true\n"
		if string(got) != want {
			t.Fatalf("long line or following content changed: got %d bytes, want %d", len(got), len(want))
		}
	}
	readErr := errors.New("read failed")
	reader := io.MultiReader(strings.NewReader("before: true\n"), iotest.ErrReader(readErr))
	got, err := RemoveCommentsFromYaml(reader)
	if !errors.Is(err, readErr) || got != nil {
		t.Fatalf("expected read error without truncated output, got %q, %v", got, err)
	}
}
