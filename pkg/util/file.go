package util

import (
	"bufio"
	"errors"
	"io"
	"os"
	"path"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// ReadFileAndFixNewline reads the content of a io.Reader and replaces \r\n with \n
func ReadFileAndFixNewline(reader io.Reader) ([]byte, error) {
	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	return []byte(strings.ReplaceAll(string(content), "\r\n", "\n")), nil
}

func appendAndNL(to, from *[]byte) {
	if from != nil {
		*to = append(*to, *from...)
	}
	*to = append(*to, '\n')
}

func appendAndNLStr(to *[]byte, from string) {
	*to = append(*to, from...)
	*to = append(*to, '\n')
}

// PrefixFirstYamlDocument inserts a line to the beginning of the first YAML document in a file having content
func PrefixFirstYamlDocument(line, file string) error {
	fileInfo, err := os.Stat(file)
	if err != nil {
		return err
	}
	perm := fileInfo.Mode().Perm()
	content, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	eol := "\n"
	if len(content) >= 2 && content[len(content)-2] == '\r' && content[len(content)-1] == '\n' {
		eol = "\r\n"
	}

	// put line directly below YAML document_start if it exists and nothing is preceding it
	documentStart := "---" + eol
	if strings.HasPrefix(string(content), documentStart) {
		content = content[len(documentStart):]
		line = documentStart + line
	}

	newContent := line + eol + string(content)
	return os.WriteFile(file, []byte(newContent), perm)
}

// RemoveCommentsFromYaml tries to remove comments if they contain valid yaml
func RemoveCommentsFromYaml(reader io.Reader) ([]byte, error) {
	result := make([]byte, 0)
	buff := make([]byte, 0)
	scanner := bufio.NewScanner(reader)

	commentMatcher := regexp.MustCompile(`^\s*#\s*`)
	commentYamlMapMatcher := regexp.MustCompile(`^(\s*#\s*)[^:]+:.*$`)
	schemaMatcher := regexp.MustCompile(`^\s*#\s@schema\s*`)

	var line string
	var inCode, inSchema bool
	var codeIndention int
	var unknownYaml interface{}

	for scanner.Scan() {
		line = scanner.Text()

		// If the line is empty and we are parsing a block of potential yaml,
		// the parsed block of yaml is "finished" and should be added to the
		// result
		if line == "" && inCode {
			appendAndNL(&result, &buff)
			appendAndNLStr(&result, line)
			// reset
			inCode = false
			buff = make([]byte, 0)
			continue
		}

		// Line contains @schema
		// The following lines will be added to result
		if schemaMatcher.Match([]byte(line)) {
			inSchema = !inSchema
			appendAndNLStr(&result, line)
			continue
		}

		// Inside a @schema block
		if inSchema {
			appendAndNLStr(&result, line)
			continue
		}

		// Havent found a potential yaml block yet
		if !inCode {
			if matches := commentYamlMapMatcher.FindStringSubmatch(line); matches != nil {
				codeIndention = len(matches[1])
				inCode = true
			}
		}

		// Try if this line is valid yaml
		if inCode {
			if commentMatcher.Match([]byte(line)) {
				// Strip the commet away
				strippedLine := line[codeIndention:]
				// add it to the already parsed valid yaml
				appendAndNLStr(&buff, strippedLine)
				// check if the new block is still valid yaml
				err := yaml.Unmarshal(buff, &unknownYaml)
				if err != nil {
					// Invalid yaml found,
					// Remove the stripped line again
					buff = buff[:len(buff)-len(strippedLine)-1]
					// and add the commented line instead
					appendAndNLStr(&buff, line)
				}
				// its still valid yaml
				continue
			}

			// If the line is not a comment it must be yaml
			appendAndNLStr(&buff, line)
			continue
		}
		// line is valid yaml
		appendAndNLStr(&result, line)
	}

	if len(buff) > 0 {
		appendAndNL(&result, &buff)
	}

	return result, nil
}

// IsRelativeFile checks if the given string is a relative path to a file
func IsRelativeFile(root, relPath string) (string, error) {
	if !path.IsAbs(relPath) {
		foo := path.Join(path.Dir(root), relPath)
		_, err := os.Stat(foo)
		return foo, err
	}
	return "", errors.New("Is absolute file")
}
