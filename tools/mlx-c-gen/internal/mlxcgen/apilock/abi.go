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
	// Params preserves declared order; the counts summarize argument
	// classes for positional call-helper reconstruction.
	Params            []string `json:"params"`
	IntegerClassCount int      `json:"integer_class_count"`
	FloatClassCount   int      `json:"float_class_count"`
	Return            string   `json:"return"`
	Header            string   `json:"header"`
	// Fallible is always emitted: false means "computed cannot fail", not
	// "field absent". Int returns are fallible via the status channel;
	// otherwise transitive propagation from impl bodies decides.
	Fallible bool `json:"fallible"`
	// MultipleBodies marks ifdef splits where more than one definition was
	// seen; classification unions all branches conservatively.
	MultipleBodies bool     `json:"multiple_bodies,omitempty"`
	Class          ABIClass `json:"class"`
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

// isStatusReturn reports whether the return type itself carries the failure
// signal (the C API's int status convention).
func isStatusReturn(ret string) bool {
	return strings.TrimSpace(ret) == "int"
}

type abiSource struct {
	fn             Function
	class          ABIClass
	bodies         []string
	multipleBodies bool
	fallible       bool
}

func (s *abiSource) entry() ABIEntry {
	e := ABIEntry{
		Params:         append([]string(nil), s.fn.Parameters...),
		Return:         s.fn.Return,
		Header:         s.fn.Header,
		Fallible:       s.fallible,
		MultipleBodies: s.multipleBodies,
		Class:          s.class,
	}
	for _, p := range e.Params {
		switch ClassifyParam(p) {
		case "float":
			e.FloatClassCount++
		default:
			e.IntegerClassCount++
		}
	}
	return e
}

// ExtractImplBodies returns the brace-matched body of every extern "C"
// definition of name in implText, in textual order. A definition requires
// the extern "C" linkage marker immediately governing the declarator and a
// parameter list closed by '{' rather than ';', so prototypes and plain
// call sites never match. Multiple results indicate an #ifdef split;
// callers must classify conservatively across all branches.
func ExtractImplBodies(implText, name string) []string {
	var bodies []string
	pos := 0
	for {
		i := strings.Index(implText[pos:], `extern "C"`)
		if i < 0 {
			return bodies
		}
		i += pos
		pos = i + len(`extern "C"`)
		segEnd := i + len(`extern "C"`) + 4096
		if segEnd > len(implText) {
			segEnd = len(implText)
		}
		seg := implText[i:segEnd]
		j := strings.Index(seg, name+"(")
		if j < 0 {
			continue
		}
		// The match must be the extern "C" declarator itself: nothing but
		// the return-type tokens may precede it in this segment.
		if strings.ContainsAny(seg[:j], ";{}") {
			continue
		}
		rest := seg[j+len(name)+1:]
		open := strings.IndexByte(rest, '{')
		semi := strings.IndexByte(rest, ';')
		if open < 0 || (semi >= 0 && semi < open) {
			continue
		}
		start := i + j + len(name) + 1 + open
		depth := 0
		var body string
		for k := start; k < len(implText); k++ {
			switch implText[k] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					body = implText[start : k+1]
				}
			case '"':
				k = skipString(implText, k)
			}
			if body != "" {
				break
			}
		}
		if body != "" {
			bodies = append(bodies, body)
			pos = start + len(body)
		}
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

type abiBuilder struct {
	dir     string
	classes map[string]ABIClass
	order   []string
	sources map[string]*abiSource
	private map[string]bool
}

func (b *abiBuilder) collect(headerPath string, private bool) error {
	data, err := os.ReadFile(headerPath)
	if err != nil {
		return err
	}
	base := filepath.Base(headerPath)
	class := ABIClassAPI
	if c, ok := b.classes[base]; ok {
		class = c
	}
	target, err := ParseHeaderContent(base, data)
	if err != nil {
		return fmt.Errorf("parse %s: %w", base, err)
	}
	var implText string
	cppPath := filepath.Join(filepath.Dir(headerPath), strings.TrimSuffix(base, ".h")+".cpp")
	if body, err := os.ReadFile(cppPath); err == nil {
		implText = string(body)
	}
	for _, fn := range target.Functions {
		var bodies []string
		if implText != "" {
			bodies = ExtractImplBodies(implText, fn.Name)
		}
		prev, seen := b.sources[fn.Name]
		src := &abiSource{fn: fn, class: class}
		if seen {
			// Declared in both a public and a private header, or twice:
			// union bodies, keep the stricter (private) visibility.
			src.bodies = append(src.bodies, prev.bodies...)
			src.multipleBodies = prev.multipleBodies
			b.private[fn.Name] = b.private[fn.Name] || private
		}
		src.bodies = append(src.bodies, bodies...)
		src.multipleBodies = src.multipleBodies || len(bodies) > 1
		if !seen {
			b.order = append(b.order, fn.Name)
		}
		b.sources[fn.Name] = src
		b.private[fn.Name] = b.private[fn.Name] || private
	}
	return nil
}

// computeFallibility marks int-returning functions fallible (the status
// channel), then propagates to a fixpoint: a function whose body calls any
// known-fallible function can itself fail, because that callee reports
// through mlx_error. Bodies with multiple ifdef branches contribute
// conservatively (any branch reporting makes the function fallible).
func (b *abiBuilder) computeFallibility() {
	fallible := map[string]bool{}
	for _, n := range b.order {
		if isStatusReturn(b.sources[n].fn.Return) {
			fallible[n] = true
		}
	}
	for changed := true; changed; {
		changed = false
		for _, n := range b.order {
			if fallible[n] {
				continue
			}
			src := b.sources[n]
			for _, body := range src.bodies {
				if strings.Contains(body, "mlx_error(") {
					fallible[n] = true
					break
				}
			}
			if fallible[n] {
				changed = true
				continue
			}
		callees:
			for callee := range b.sources {
				if callee == n || !fallible[callee] {
					continue
				}
				call := callee + "("
				for _, body := range src.bodies {
					if strings.Contains(body, call) {
						fallible[n] = true
						break callees
					}
				}
			}
			if fallible[n] {
				changed = true
			}
		}
	}
	for _, n := range b.order {
		b.sources[n].fallible = fallible[n]
	}
}

// BuildABI walks the output tree: every public header directly in dir plus
// every header under dir/private contributes its declarations; impl bodies
// are read from sibling .cpp files to compute fallibility transitively.
// classes maps a header base name to its ABIClass; unlisted headers are
// ABIClassAPI.
func BuildABI(dir string, classes map[string]ABIClass) (ABIMap, error) {
	m := ABIMap{
		SchemaVersion: ABISchemaVersion,
		Functions:     map[string]ABIEntry{},
		Private:       map[string]ABIEntry{},
	}
	b := &abiBuilder{
		dir:     dir,
		classes: classes,
		sources: map[string]*abiSource{},
		private: map[string]bool{},
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return m, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".h") {
			continue
		}
		if err := b.collect(filepath.Join(dir, e.Name()), false); err != nil {
			return m, err
		}
	}
	privEntries, err := os.ReadDir(filepath.Join(dir, "private"))
	if err != nil && !os.IsNotExist(err) {
		return m, err
	}
	if err == nil {
		for _, e := range privEntries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".h") {
				continue
			}
			if err := b.collect(filepath.Join(dir, "private", e.Name()), true); err != nil {
				return m, err
			}
		}
	}
	b.computeFallibility()
	for _, n := range b.order {
		e := b.sources[n].entry()
		if b.private[n] {
			m.Private[n] = e
			continue
		}
		m.Functions[n] = e
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
