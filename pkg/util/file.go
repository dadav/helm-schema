package util

import (
	"io"
	"strings"
)

// ReadFileAndFixNewline reads the content of a io.Reader and replaces \r\n with \n
func ReadFileAndFixNewline(reader io.Reader) ([]byte, error) {
	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	return []byte(strings.Replace(string(content), "\r\n", "\n", -1)), nil
}
