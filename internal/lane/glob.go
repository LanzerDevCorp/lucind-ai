package lane

import (
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

var (
	patternCacheMu sync.RWMutex
	patternCache   = make(map[string]*regexp.Regexp)
)

func patternToRegex(pattern string) (*regexp.Regexp, error) {
	patternCacheMu.RLock()
	re, ok := patternCache[pattern]
	patternCacheMu.RUnlock()
	if ok {
		return re, nil
	}

	var b strings.Builder
	b.WriteString("^")

	n := len(pattern)
	for i := 0; i < n; {
		if strings.HasPrefix(pattern[i:], "/**") && (i+3 == n || pattern[i+3] == '/') {
			if i+3 == n {
				// Trailing /**: matches /.* or nothing (representing directory itself)
				b.WriteString("(?:/.*)?")
				i += 3
			} else {
				// /**/ in the middle: matches / or /.*/
				b.WriteString("(?:/.*/|/)")
				i += 4 // skip /**/
			}
		} else if strings.HasPrefix(pattern[i:], "**/") && i == 0 {
			// Leading **/: matches (?:.*/)?
			b.WriteString("(?:.*/)?")
			i += 3
		} else if pattern[i:] == "**" {
			b.WriteString(".*")
			i += 2
		} else if pattern[i] == '*' {
			b.WriteString("[^/]*")
			i++
		} else if pattern[i] == '?' {
			b.WriteString("[^/]")
			i++
		} else if pattern[i] == '[' {
			j := i + 1
			if j < n && (pattern[j] == '!' || pattern[j] == '^') {
				j++
			}
			if j < n && pattern[j] == ']' {
				j++
			}
			for j < n && pattern[j] != ']' {
				j++
			}
			if j >= n {
				b.WriteString(`\[`)
				i++
			} else {
				class := pattern[i : j+1]
				switch class[1] {
				case '!', '^':
					class = "[^/" + class[2:]
				default:
					class = "[" + class[1:]
				}
				b.WriteString(class)
				i = j + 1
			}
		} else if strings.ContainsRune(".+()^$|{}", rune(pattern[i])) {
			b.WriteByte('\\')
			b.WriteByte(pattern[i])
			i++
		} else {
			b.WriteByte(pattern[i])
			i++
		}
	}
	b.WriteString("$")

	compiled, err := regexp.Compile(b.String())
	if err != nil {
		return nil, err
	}

	patternCacheMu.Lock()
	patternCache[pattern] = compiled
	patternCacheMu.Unlock()

	return compiled, nil
}

// Match reports whether path matches pattern according to glob syntax,
// supporting doublestar '**'.
func Match(pattern, path string) bool {
	pattern = filepath.ToSlash(pattern)
	pattern = strings.TrimPrefix(pattern, "./")
	path = filepath.ToSlash(path)
	path = strings.TrimPrefix(path, "./")

	if pattern == "" && path == "" {
		return true
	}
	if pattern == "" || path == "" {
		return false
	}

	re, err := patternToRegex(pattern)
	if err != nil {
		return false
	}
	return re.MatchString(path)
}

// MatchAny reports whether path matches any of patterns.
func MatchAny(patterns []string, path string) bool {
	for _, p := range patterns {
		if Match(p, path) {
			return true
		}
	}
	return false
}
