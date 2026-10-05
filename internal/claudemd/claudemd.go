package claudemd

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Outcome represents the result of installing the lucind dispatch block into CLAUDE.md.
type Outcome string

const (
	OutcomeCreated   Outcome = "created"
	OutcomeAppended  Outcome = "appended"
	OutcomeReplaced  Outcome = "replaced"
	OutcomeUnchanged Outcome = "unchanged"
)

const (
	// BeginMarker marks the start of the lucind dispatch block.
	BeginMarker = "<!-- lucind:dispatch -->"
	// EndMarker marks the end of the lucind dispatch block.
	EndMarker = "<!-- /lucind:dispatch -->"
)

var (
	beginMarkerBytes = []byte(BeginMarker)
	endMarkerBytes   = []byte(EndMarker)
)

// ErrMalformedMarkers indicates invalid, unpaired, or duplicated lucind markers.
var ErrMalformedMarkers = errors.New("malformed markers in dispatch block")

// Install installs block into CLAUDE.md at path according to the dispatch rules.
func Install(path string, block []byte) (Outcome, error) {
	normBlock := ensureTrailingNewline(block)
	cleanedPath := filepath.Clean(path)

	lstatInfo, lstatErr := os.Lstat(cleanedPath)
	if lstatErr != nil {
		if errors.Is(lstatErr, fs.ErrNotExist) {
			parentDir := filepath.Dir(cleanedPath)
			if err := os.MkdirAll(parentDir, 0o755); err != nil {
				return "", fmt.Errorf("create parent directory %s: %w", parentDir, err)
			}
			if err := writeAtomic(cleanedPath, normBlock, 0o644); err != nil {
				return "", err
			}
			return OutcomeCreated, nil
		}
		return "", fmt.Errorf("lstat %s: %w", cleanedPath, lstatErr)
	}

	target := cleanedPath
	if lstatInfo.Mode()&os.ModeSymlink != 0 {
		resolved, err := filepath.EvalSymlinks(cleanedPath)
		if err != nil {
			return "", fmt.Errorf("resolve symlink %s: %w", cleanedPath, err)
		}
		target = filepath.Clean(resolved)
	}

	targetInfo, err := os.Stat(target)
	if err != nil {
		return "", fmt.Errorf("stat target %s: %w", target, err)
	}
	origPerm := targetInfo.Mode().Perm()

	currentContent, err := os.ReadFile(target)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", target, err)
	}

	hasMarkers, spanStart, spanEnd, err := inspectMarkers(currentContent)
	if err != nil {
		return "", err
	}

	var (
		newContent []byte
		outcome    Outcome
	)

	if hasMarkers {
		newContent = make([]byte, 0, spanStart+len(normBlock)+(len(currentContent)-spanEnd))
		newContent = append(newContent, currentContent[:spanStart]...)
		newContent = append(newContent, normBlock...)
		newContent = append(newContent, currentContent[spanEnd:]...)
		outcome = OutcomeReplaced
	} else {
		trimmed := bytes.TrimRight(currentContent, "\r\n")
		if len(trimmed) == 0 {
			newContent = normBlock
		} else {
			newContent = make([]byte, 0, len(trimmed)+2+len(normBlock))
			newContent = append(newContent, trimmed...)
			newContent = append(newContent, '\n', '\n')
			newContent = append(newContent, normBlock...)
		}
		outcome = OutcomeAppended
	}

	if bytes.Equal(newContent, currentContent) {
		return OutcomeUnchanged, nil
	}

	bakPath := target + ".lucind-ai.bak"
	_ = os.Remove(bakPath)
	if err := os.WriteFile(bakPath, currentContent, 0o644); err != nil {
		return "", fmt.Errorf("write backup %s: %w", bakPath, err)
	}

	if err := writeAtomic(target, newContent, origPerm); err != nil {
		return "", err
	}

	return outcome, nil
}

func inspectMarkers(content []byte) (hasMarkers bool, spanStart, spanEnd int, err error) {
	beginCount := bytes.Count(content, beginMarkerBytes)
	endCount := bytes.Count(content, endMarkerBytes)

	if beginCount == 0 && endCount == 0 {
		return false, 0, 0, nil
	}
	if beginCount > 1 {
		return false, 0, 0, fmt.Errorf("%w: duplicate begin marker %q found (%d times)", ErrMalformedMarkers, BeginMarker, beginCount)
	}
	if endCount > 1 {
		return false, 0, 0, fmt.Errorf("%w: duplicate end marker %q found (%d times)", ErrMalformedMarkers, EndMarker, endCount)
	}
	if beginCount == 1 && endCount == 0 {
		return false, 0, 0, fmt.Errorf("%w: begin marker %q found without end marker", ErrMalformedMarkers, BeginMarker)
	}
	if beginCount == 0 && endCount == 1 {
		return false, 0, 0, fmt.Errorf("%w: end marker %q found without begin marker", ErrMalformedMarkers, EndMarker)
	}

	bIdx := bytes.Index(content, beginMarkerBytes)
	eIdx := bytes.Index(content, endMarkerBytes)
	if eIdx < bIdx {
		return false, 0, 0, fmt.Errorf("%w: end marker %q appears before begin marker %q", ErrMalformedMarkers, EndMarker, BeginMarker)
	}

	spanStart = bytes.LastIndexByte(content[:bIdx], '\n')
	if spanStart == -1 {
		spanStart = 0
	} else {
		spanStart++
	}

	endMarkerEnd := eIdx + len(endMarkerBytes)
	if nlIdx := bytes.IndexByte(content[endMarkerEnd:], '\n'); nlIdx == -1 {
		spanEnd = len(content)
	} else {
		spanEnd = endMarkerEnd + nlIdx + 1
	}

	return true, spanStart, spanEnd, nil
}

func ensureTrailingNewline(b []byte) []byte {
	if len(b) == 0 {
		return []byte("\n")
	}
	if b[len(b)-1] == '\n' {
		return b
	}
	out := make([]byte, len(b)+1)
	copy(out, b)
	out[len(b)] = '\n'
	return out
}

func writeAtomic(target string, content []byte, perm os.FileMode) error {
	dir := filepath.Dir(target)
	base := filepath.Base(target)

	tmpFile, err := os.CreateTemp(dir, base+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file in %s: %w", dir, err)
	}
	tmpName := tmpFile.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmpFile.Write(content); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("write temp file %s: %w", tmpName, err)
	}

	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("sync temp file %s: %w", tmpName, err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp file %s: %w", tmpName, err)
	}

	if err := os.Chmod(tmpName, perm); err != nil {
		return fmt.Errorf("chmod temp file %s: %w", tmpName, err)
	}

	if err := os.Rename(tmpName, target); err != nil {
		return fmt.Errorf("rename %s to %s: %w", tmpName, target, err)
	}

	cleanup = false
	return nil
}
