package apilock

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ABISchemaVersion is the schema version of emitted abi.json files.
const ABISchemaVersion = 2

// ABIClass classifies a symbol's contract level.
type ABIClass string

const (
	// ABIClassAPI marks symbols that carry an API stability contract.
	ABIClassAPI ABIClass = "api"
	// ABIClassDiagnostic marks shipped symbols that are explicitly not an
	// API contract (e.g. tracing hooks).
	ABIClassDiagnostic ABIClass = "diagnostic"
)

// ABIEntry records the calling-convention classification of one C function,
// matching the split encoded in hand-written call helpers (e.g.
// Call8Float32x1 takes eight integer-class arguments followed by one
// float32), along with its failure contract.
type ABIEntry struct {
	Params            []string `json:"params"`
	IntegerClassCount int      `json:"integer_class_count"`
	FloatClassCount   int      `json:"float_class_count"`
	Return            string   `json:"return"`
	Header            string   `json:"header"`
	Fallible          bool     `json:"fallible,omitempty"`
	Class             ABIClass `json:"class"`
}

// ABIMap is the machine-readable ABI manifest written next to the generated
// headers as abi.json. It describes every function declared by the shipped
// headers of the exact tree it ships beside.
type ABIMap struct {
	SchemaVersion int                 `json:"schema_version"`
	Functions     map[string]ABIEntry `json:"functions"`
	// Private holds functions declared under private/ headers. They are
	// shipped and linkable but not part of the public contract.
	Private map[string]ABIEntry `json:"private,omitempty"`
}

// ClassifyParam reports whether a C parameter declaration belongs to the
// integer or floating-point argument class. Pointers (including typedef'd
// handles like mlx_array) pass as integers regardless of their pointee type;
// only scalar float/double values are float-class.
func ClassifyParam(param string) string {
	t := strings.TrimSpace(param)
	if strings.Contains(t, "*") {
		return "integer"
	}
	if strings.Contains(t, "float") || strings.Contains(t, "double") {
		return "float"
	}
	return "integer"
}

// entryFromFunction builds an ABIEntry from a parsed declaration, computing
// fallibility from the impl body when one is available.
func entryFromFunction(fn Function, class ABIClass, body string) ABIEntry {
	e := ABIEntry{
		Params:   append([]string(nil), fn.Parameters...),
		Return:   fn.Return,
		Header:   fn.Header,
		Class:    class,
		Fallible: isStatusReturn(fn.Return),
	}
	sort.Strings(e.Params)
	for _, p := range e.Params {
		switch ClassifyParam(p) {
		case "float":
			e.FloatClassCount++
		default:
			e.IntegerClassCount++
		}
	}
	if !e.Fallible && body != "" && strings.Contains(body, "mlx_error(") {
		e.Fallible = true
	}
	return e
}

// isStatusReturn reports whether the return type itself carries the failure
// signal (the C API's int status convention).
func isStatusReturn(ret string) bool {
	return strings.TrimSpace(ret) == "int"
}

// ExtractImplBody returns the body of `extern "C" ... name(` in impl text,
// brace-matched from the first '{' after the declarator. It returns "" when
// the function has no definition in this file.
func ExtractImplBody(implText, name string) string {
	decl := name + "("
	idx := 0
	for {
		i := strings.Index(implText[idx:], decl)
		if i < 0 {
			return ""
		}
		i += idx
		// Require an extern "C" definition context: the match must not be a
		// call site. A definition is followed (after whitespace/comments)
		// by '{' before any ';'.
		rest := implText[i+len(decl):]
		open := strings.IndexByte(rest, '{')
		semi := strings.IndexByte(rest, ';')
		if open < 0 || (semi >= 0 && semi < open) {
			idx = i + len(decl)
			continue
		}
		start := i + len(decl) + open
		depth := 0
		for j := start; j < len(implText); j++ {
			switch implText[j] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					return implText[start : j+1]
				}
			case '"':
				j = skipString(implText, j)
			}
		}
		return ""
	}
}

func skipString(s string, i int) int {
	for j := i + 1; j < len(s); j++ {
		if s[j] == '\\' {
			j++
			continue
		}
		if s[j] == '"' {
			return j
		}
	}
	return len(s) - 1
}

// BuildABI walks the output tree: every public header directly in dir plus
// every header under dir/private contributes its declarations; impl bodies
// are read from the sibling .cpp files to compute fallibility. classes maps
// a header base name to its ABIClass; unlisted headers are ABIClassAPI.
func BuildABI(dir string, classes map[string]ABIClass) (ABIMap, error) {
	m := ABIMap{
		SchemaVersion: ABISchemaVersion,
		Functions:     map[string]ABIEntry{},
		Private:       map[string]ABIEntry{},
	}
	add := func(headerPath string, private bool) error {
		data, err := os.ReadFile(headerPath)
		if err != nil {
			return err
		}
		base := filepath.Base(headerPath)
		class := ABIClassAPI
		if c, ok := classes[base]; ok {
			class = c
		}
		target, err := ParseHeaderContent(base, data)
		if err != nil {
			return fmt.Errorf("parse %s: %w", base, err)
		}
		var implText string
		cppPath := filepath.Join(filepath.Dir(headerPath), strings.TrimSuffix(base, ".h")+".cpp")
		if b, err := os.ReadFile(cppPath); err == nil {
			implText = string(b)
		}
		dest := m.Functions
		if private {
			dest = m.Private
		}
		for _, fn := range target.Functions {
			body := ""
			if implText != "" {
				body = ExtractImplBody(implText, fn.Name)
			}
			dest[fn.Name] = entryFromFunction(fn, class, body)
		}
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return m, err
	}
	privateDir := filepath.Join(dir, "private")
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".h") {
			continue
		}
		if err := add(filepath.Join(dir, e.Name()), false); err != nil {
			return m, err
		}
	}
	privEntries, err := os.ReadDir(privateDir)
	if err != nil {
		if os.IsNotExist(err) {
			return m, nil
		}
		return m, err
	}
	for _, e := range privEntries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".h") {
			continue
		}
		if err := add(filepath.Join(privateDir, e.Name()), true); err != nil {
			return m, err
		}
	}
	return m, nil
}

// JSON renders the manifest.
func (m ABIMap) JSON() ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}

// SortNames returns the sorted symbol names at the given visibility.
func (m ABIMap) SortNames(private bool) []string {
	src := m.Functions
	if private {
		src = m.Private
	}
	names := make([]string, 0, len(src))
	for name := range src {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
