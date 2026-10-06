package userconfig

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var userHomeDir = os.UserHomeDir

// Dir returns the user configuration directory for lucind:
// $XDG_CONFIG_HOME/lucind when XDG_CONFIG_HOME is set and non-empty,
// otherwise ~/.config/lucind (via os.UserHomeDir).
func Dir() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "lucind"), nil
	}
	home, err := userHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home directory: %w", err)
	}
	return filepath.Join(home, ".config", "lucind"), nil
}

// EnvFile returns the path to the environment config file ($lucindDir/env).
func EnvFile() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "env"), nil
}

// ReadKey reads the value of a key from the user config env file.
// The file is parsed as dotenv-style lines NAME=value.
// Blank lines, # comments, leading "export ", and one pair of single or double
// quotes around the value are tolerated; whitespace is trimmed.
// A missing file returns an empty string and no error.
// Any other read error is returned wrapped without the key value in it.
func ReadKey(name string) (string, error) {
	path, err := EnvFile()
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("read env file %s: %w", path, err)
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	var found string
	for scanner.Scan() {
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" || strings.HasPrefix(raw, "#") {
			continue
		}
		raw = strings.TrimPrefix(raw, "export ")
		raw = strings.TrimSpace(raw)
		k, v, ok := strings.Cut(raw, "=")
		if !ok {
			continue
		}
		if strings.TrimSpace(k) == name {
			v = strings.TrimSpace(v)
			if len(v) >= 2 && ((v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'')) {
				v = v[1 : len(v)-1]
			}
			found = strings.TrimSpace(v)
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("parse env file %s: %w", path, err)
	}

	return found, nil
}

// WriteKey writes a key=value pair to the user config env file.
// It creates the directory with mode 0700 and the file with mode 0600.
// When the file exists, it replaces or appends only that one line and keeps
// every other line byte for byte. The write is atomic (temp file in the same
// directory, then rename).
// The value must never appear in any returned error.
func WriteKey(name, value string) error {
	dir, err := Dir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create config dir %s: %w", dir, err)
	}
	_ = os.Chmod(dir, 0700)

	envPath, err := EnvFile()
	if err != nil {
		return err
	}

	existingContent, err := os.ReadFile(envPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read existing env file %s: %w", envPath, err)
	}

	var newLines [][]byte
	var found bool

	if len(existingContent) > 0 {
		var lines [][]byte
		start := 0
		for i := 0; i < len(existingContent); i++ {
			if existingContent[i] == '\n' {
				lines = append(lines, existingContent[start:i+1])
				start = i + 1
			}
		}
		if start < len(existingContent) {
			lines = append(lines, existingContent[start:])
		}

		for _, line := range lines {
			lineStr := string(line)
			trimmed := strings.TrimSpace(lineStr)
			isMatch := false
			if !strings.HasPrefix(trimmed, "#") {
				noExport := strings.TrimPrefix(trimmed, "export ")
				if k, _, ok := strings.Cut(noExport, "="); ok {
					if strings.TrimSpace(k) == name {
						isMatch = true
					}
				}
			}

			if isMatch && !found {
				newline := "\n"
				if strings.HasSuffix(lineStr, "\r\n") {
					newline = "\r\n"
				}
				newLines = append(newLines, []byte(name+"="+value+newline))
				found = true
			} else {
				newLines = append(newLines, line)
			}
		}
	}

	if !found {
		if len(newLines) > 0 {
			last := newLines[len(newLines)-1]
			if len(last) > 0 && last[len(last)-1] != '\n' {
				newLines[len(newLines)-1] = append(last, '\n')
			}
		}
		newLines = append(newLines, []byte(name+"="+value+"\n"))
	}

	newContent := bytes.Join(newLines, nil)

	tmpFile, err := os.CreateTemp(dir, "env.*")
	if err != nil {
		return fmt.Errorf("create temp env file in %s: %w", dir, err)
	}
	tmpName := tmpFile.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if err := tmpFile.Chmod(0600); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("chmod temp env file: %w", err)
	}

	if _, err := tmpFile.Write(newContent); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("write temp env file: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("sync temp env file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp env file: %w", err)
	}

	if err := os.Rename(tmpName, envPath); err != nil {
		return fmt.Errorf("rename temp env file to %s: %w", envPath, err)
	}
	_ = os.Chmod(envPath, 0600)

	return nil
}
