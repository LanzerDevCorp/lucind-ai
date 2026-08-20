package run

import (
	"bytes"
)

// parseDiffNameStatusZ parses `git diff --name-status -z` output.
// Ordinary status (A/C/D/M/T, and R without a captured second path) yields
// one path; R* and C* yield both the old and the new path. Consumed only by
// enforceAllowedPaths.
func parseDiffNameStatusZ(output []byte) []string {
	if len(output) == 0 {
		return nil
	}

	tokens := bytes.Split(output, []byte{0})
	var paths []string

	i := 0
	for i < len(tokens) {
		token := string(tokens[i])
		if token == "" {
			i++
			continue
		}

		status := token
		i++

		if len(status) > 0 && (status[0] == 'R' || status[0] == 'C') {
			// Rename or copy: status \0 old \0 new \0
			if i < len(tokens) && len(tokens[i]) > 0 {
				paths = append(paths, string(tokens[i]))
				i++
			}
			if i < len(tokens) && len(tokens[i]) > 0 {
				paths = append(paths, string(tokens[i]))
				i++
			}
		} else {
			// Ordinary status: status \0 path \0
			if i < len(tokens) && len(tokens[i]) > 0 {
				paths = append(paths, string(tokens[i]))
				i++
			}
		}
	}

	return paths
}

// parseLSFilesZ parses `git ls-files -z` output. Consumed only by
// enforceAllowedPaths.
func parseLSFilesZ(output []byte) []string {
	if len(output) == 0 {
		return nil
	}

	tokens := bytes.Split(output, []byte{0})
	var paths []string
	for _, tok := range tokens {
		if len(tok) > 0 {
			paths = append(paths, string(tok))
		}
	}
	return paths
}
