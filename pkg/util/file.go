package util

import (
	"os"
	"strings"
)

// ReadYamlFile reads the content and replaces \r\n with \n
func ReadYamlFile(filename string) ([]byte, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return []byte(strings.Replace(string(content), "\r\n", "\n", -1)), nil
}
