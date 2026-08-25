package apilock

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// CheckIncludeGuards verifies that every declaration in every shipped header
// under dir (public and private/) sits inside its include guard. Text after
// the final #endif that contains an opening parenthesis or a trailing
// semicolon is a declaration outside the guard -- historically produced by
// appending helpers below the guard, which breaks double inclusion. Returns
// one violation per offending file.
func CheckIncludeGuards(dir string) ([]string, error) {
	var violations []string
	check := func(path string) error {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if tail := textAfterLastEndif(string(data)); hasDecl(tail) {
			violations = append(violations,
				fmt.Sprintf("%s: declaration(s) outside include guard", path))
		}
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".h") {
			continue
		}
		if err := check(filepath.Join(dir, e.Name())); err != nil {
			return nil, err
		}
	}
	privDir := filepath.Join(dir, "private")
	privEntries, err := os.ReadDir(privDir)
	if err == nil {
		for _, e := range privEntries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".h") {
				continue
			}
			if err := check(filepath.Join(privDir, e.Name())); err != nil {
				return nil, err
			}
		}
	}
	sort.Strings(violations)
	return violations, nil
}

// textAfterLastEndif returns everything after the final #endif, with
// comments stripped.
func textAfterLastEndif(src string) string {
	idx := strings.LastIndex(src, "#endif")
	if idx < 0 {
		return ""
	}
	tail := src[idx+len("#endif"):]
	var b strings.Builder
	for i := 0; i < len(tail); i++ {
		if i+1 < len(tail) && tail[i] == '/' && tail[i+1] == '*' {
			end := strings.Index(tail[i+2:], "*/")
			if end < 0 {
				break
			}
			i += end + 3
			continue
		}
		if i+1 < len(tail) && tail[i] == '/' && tail[i+1] == '/' {
			nl := strings.IndexByte(tail[i:], '\n')
			if nl < 0 {
				break
			}
			i += nl
			continue
		}
		b.WriteByte(tail[i])
	}
	return strings.TrimSpace(b.String())
}

// hasDecl reports whether stripped tail text still looks like code.
func hasDecl(tail string) bool {
	tail = strings.TrimSpace(tail)
	if tail == "" {
		return false
	}
	noWS := strings.Map(func(r rune) rune {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			return -1
		}
		return r
	}, tail)
	return strings.ContainsAny(noWS, "(;")
}
