package util

import (
	"bytes"
	"testing"
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
