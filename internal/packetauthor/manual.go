package packetauthor

import (
	"bytes"
	"regexp"
	"strings"
)

const (
	manualOpen       = "```lucind-result-contract"
	manualClose      = "```"
	resultPathPhrase = "Write the result envelope to .lucind/result.json in this worktree."
	schemaPhraseA    = "Validate it against .lucind/result.schema.json before writing."
	schemaPhraseB    = "The schema is at .lucind/result.schema.json in this worktree. Validate against it before writing"
)

var (
	headingPattern        = regexp.MustCompile(`^#{1,2}[ \t]+`)
	requiredCommitPattern = regexp.MustCompile(`(^|[.!?] )(commit |create a commit|after you commit)`)
)

func AdmitManual(packet ManualPacket) (Artifact, error) {
	diagnostics := validateManual(packet)
	bound, targetDiagnostics := validateBinding(packet.Binding)
	diagnostics = append(diagnostics, targetDiagnostics...)
	_, pathDiagnostics := normalizePaths("write_paths", packet.WritePaths)
	diagnostics = append(diagnostics, pathDiagnostics...)
	_, pathDiagnostics = normalizePaths("read_only_paths", packet.ReadOnlyPaths)
	diagnostics = append(diagnostics, pathDiagnostics...)
	if err := diagnosticsError(diagnostics); err != nil {
		return Artifact{}, err
	}
	version := LegacyVersion
	if hasStructuredTrigger(normalizedLines(packet.Body)) {
		version = ContractVersion
	}
	body := append([]byte(nil), packet.Body...)
	manifestJSON := encodeJSON(struct {
		Version string      `json:"version"`
		Binding BoundTarget `json:"binding"`
	}{Version: version, Binding: bound})
	return Artifact{Version: version, Body: body, ManifestJSON: manifestJSON, Digest: digest("lucind:manual-packet/v1", body, manifestJSON), Binding: bound}, nil
}

func AdmitBatch(items []BatchItem) ([]Artifact, error) {
	artifacts := make([]Artifact, len(items))
	var all Diagnostics
	for index, item := range items {
		var err error
		switch {
		case item.Manual != nil && item.Contract == nil:
			artifacts[index], err = AdmitManual(*item.Manual)
		case item.Contract != nil && item.Manual == nil:
			artifacts[index], err = Compile(*item.Contract, item.Binding)
		default:
			err = Diagnostics{diagnostic(10, "item", CodeContractInvalid, "batch item must contain exactly one input")}
		}
		if diagnostics, ok := err.(Diagnostics); ok {
			for _, d := range diagnostics {
				d.PacketIndex = index
				all = append(all, d)
			}
		}
	}
	if err := diagnosticsError(all); err != nil {
		return nil, err
	}
	return artifacts, nil
}

func validateManual(packet ManualPacket) Diagnostics {
	lines := normalizedLines(packet.Body)
	var diagnostics Diagnostics
	if hasStructuredTrigger(lines) {
		if !validStructuredBlock(lines, packet.ReadOnly) {
			diagnostics = append(diagnostics, diagnostic(10, "body", CodeManualMarkerInvalid, "structured result contract marker is malformed or contradicts metadata"))
		}
	} else {
		sections, ok := compatibilitySections(lines)
		if !ok {
			diagnostics = append(diagnostics, diagnostic(10, "body", CodeManualMarkerInvalid, "manual body must contain one ordered Done criteria and Return section"))
		} else {
			returnText := matchText(sections.returnLines)
			if strings.Count(returnText, resultPathPhrase) != 1 {
				diagnostics = append(diagnostics, diagnostic(20, "body", CodeResultPathMissing, "manual body must contain the exact result path obligation once"))
			}
			schemaCount := strings.Count(returnText, schemaPhraseA) + strings.Count(returnText, schemaPhraseB)
			if schemaCount != 1 {
				diagnostics = append(diagnostics, diagnostic(30, "body", CodeResultSchemaMissing, "manual body must contain exactly one accepted schema obligation"))
			}
		}
	}
	if strings.TrimSpace(packet.RouteIntent) == "" {
		diagnostics = append(diagnostics, diagnostic(40, "route_intent", CodeRouteInvalid, "route intent is required"))
	}
	commitText := asciiLower(matchText(nonFencedLines(lines)))
	required := requiredCommitPattern.MatchString(commitText)
	forbidden := strings.Contains(commitText, "do not commit") || strings.Contains(commitText, "no unique commit")
	if (required && forbidden) || (packet.ReadOnly && required) || (!packet.ReadOnly && forbidden) {
		diagnostics = append(diagnostics, diagnostic(50, "mode", CodeModeCommitConflict, "manual commit instructions contradict execution mode"))
	}
	return diagnostics
}

func normalizedLines(body []byte) []string {
	normalized := bytes.ReplaceAll(body, []byte("\r\n"), []byte("\n"))
	normalized = bytes.ReplaceAll(normalized, []byte("\r"), []byte("\n"))
	return strings.Split(string(normalized), "\n")
}

func hasStructuredTrigger(lines []string) bool {
	for _, line := range lines {
		if strings.Contains(strings.Trim(line, " \t"), "lucind-result-contract") {
			return true
		}
	}
	return false
}

func validStructuredBlock(lines []string, readOnly bool) bool {
	open := -1
	triggers := 0
	for i, line := range lines {
		if strings.Contains(strings.Trim(line, " \t"), "lucind-result-contract") {
			triggers++
		}
		if line == manualOpen {
			if open != -1 {
				return false
			}
			open = i
		}
	}
	if triggers != 1 || open < 0 || open+6 >= len(lines) || lines[open+6] != manualClose {
		return false
	}
	mode, commit := "write", "required"
	if readOnly {
		mode, commit = "read-only", "forbidden"
	}
	want := []string{"version: 1", "path: .lucind/result.json", "schema: .lucind/result.schema.json", "mode: " + mode, "commit: " + commit}
	for i := range want {
		if lines[open+i+1] != want[i] {
			return false
		}
	}
	return true
}

type compatibilityBody struct {
	returnLines []string
}

func compatibilitySections(lines []string) (compatibilityBody, bool) {
	visible := nonFencedLines(lines)
	doneIndexes, returnIndexes := indexes(visible, "## Done criteria"), indexes(visible, "## Return")
	if len(doneIndexes) != 1 || len(returnIndexes) != 1 || doneIndexes[0] >= returnIndexes[0] {
		return compatibilityBody{}, false
	}
	end := len(visible)
	for i := returnIndexes[0] + 1; i < len(visible); i++ {
		if headingPattern.MatchString(visible[i]) {
			end = i
			break
		}
	}
	return compatibilityBody{returnLines: visible[returnIndexes[0]+1 : end]}, true
}

func indexes(lines []string, exact string) []int {
	var found []int
	for i, line := range lines {
		if line == exact {
			found = append(found, i)
		}
	}
	return found
}

func nonFencedLines(lines []string) []string {
	result := make([]string, 0, len(lines))
	inFence, fenceByte, fenceCount := false, byte(0), 0
	for _, line := range lines {
		trimmed := line
		indent := 0
		for indent < len(trimmed) && indent < 4 && trimmed[indent] == ' ' {
			indent++
		}
		candidate := trimmed[indent:]
		if !inFence {
			if indent <= 3 {
				if marker, count := fenceMarker(candidate); count >= 3 {
					inFence, fenceByte, fenceCount = true, marker, count
					continue
				}
			}
			result = append(result, line)
			continue
		}
		if len(candidate) >= fenceCount && candidate[0] == fenceByte {
			count := 0
			for count < len(candidate) && candidate[count] == fenceByte {
				count++
			}
			if count >= fenceCount {
				inFence = false
			}
		}
	}
	return result
}

func fenceMarker(line string) (byte, int) {
	if len(line) == 0 || (line[0] != '`' && line[0] != '~') {
		return 0, 0
	}
	count := 0
	for count < len(line) && line[count] == line[0] {
		count++
	}
	return line[0], count
}

func matchText(lines []string) string {
	text := strings.Join(lines, "\n")
	text = strings.NewReplacer("*", "", "_", "", "`", "").Replace(text)
	return strings.Join(strings.FieldsFunc(text, func(r rune) bool { return r == ' ' || r == '\t' || r == '\n' }), " ")
}

func asciiLower(value string) string {
	bytes := []byte(value)
	for i, b := range bytes {
		if b >= 'A' && b <= 'Z' {
			bytes[i] = b + ('a' - 'A')
		}
	}
	return string(bytes)
}
