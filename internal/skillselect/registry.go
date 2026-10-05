package skillselect

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Skill represents an indexed skill from the registry.
type Skill struct {
	Name        string
	Description string
	Scope       string
	Path        string
}

// ParseRegistry parses a skill registry markdown document and returns the skills listed
// under the "## Skills" heading.
func ParseRegistry(r io.Reader) ([]Skill, error) {
	scanner := bufio.NewScanner(r)
	const maxLineLength = 1024 * 1024
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, maxLineLength)

	// Step 1: Find "## Skills"
	foundSkillsSection := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "## Skills" {
			foundSkillsSection = true
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading registry: %w", err)
	}
	if !foundSkillsSection {
		return nil, errors.New("skills section not found in registry")
	}

	// Step 2: Look for the table header: | Skill | Trigger / description | Scope | Path |
	foundHeader := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "## ") {
			return nil, errors.New("skills table header not found before next section")
		}
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "|") && strings.HasSuffix(line, "|") {
			parts := splitTableRow(line)
			if len(parts) == 4 &&
				parts[0] == "Skill" &&
				parts[1] == "Trigger / description" &&
				parts[2] == "Scope" &&
				parts[3] == "Path" {
				foundHeader = true
				break
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading registry: %w", err)
	}
	if !foundHeader {
		return nil, errors.New("skills table header not found in registry")
	}

	// Step 3: Expect the separator line: | --- | --- | --- | --- |
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("reading separator: %w", err)
		}
		return nil, errors.New("unexpected end of file after skills table header")
	}
	sepLine := strings.TrimSpace(scanner.Text())
	if !strings.HasPrefix(sepLine, "|") || !strings.HasSuffix(sepLine, "|") {
		return nil, fmt.Errorf("malformed table separator: %q", sepLine)
	}
	sepParts := splitTableRow(sepLine)
	if len(sepParts) != 4 {
		return nil, fmt.Errorf("malformed table separator columns: %q", sepLine)
	}
	for _, col := range sepParts {
		trimmedCol := strings.Trim(col, "-: ")
		if trimmedCol != "" {
			return nil, fmt.Errorf("invalid table separator: %q", sepLine)
		}
	}

	// Step 4: Parse table rows until empty line, new section, or EOF
	skills := make([]Skill, 0)
	for scanner.Scan() {
		rawLine := scanner.Text()
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			// End of table
			break
		}
		if !strings.HasPrefix(line, "|") || !strings.HasSuffix(line, "|") {
			return nil, fmt.Errorf("malformed table row: %q", line)
		}
		parts := splitTableRow(line)
		if len(parts) != 4 {
			return nil, fmt.Errorf("malformed table row (expected 4 columns, got %d): %q", len(parts), line)
		}

		name := stripBackticks(parts[0])
		desc := parts[1]
		scope := parts[2]
		path := stripBackticks(parts[3])

		if name == "" {
			return nil, fmt.Errorf("malformed table row (empty skill name): %q", line)
		}
		if path == "" {
			return nil, fmt.Errorf("malformed table row (empty path): %q", line)
		}

		skills = append(skills, Skill{
			Name:        name,
			Description: desc,
			Scope:       scope,
			Path:        path,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading table rows: %w", err)
	}

	return skills, nil
}

func splitTableRow(line string) []string {
	trimmed := strings.Trim(line, "|")
	rawParts := strings.Split(trimmed, "|")
	parts := make([]string, len(rawParts))
	for i, p := range rawParts {
		parts[i] = strings.TrimSpace(p)
	}
	return parts
}

func stripBackticks(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "`")
	return strings.TrimSpace(s)
}
