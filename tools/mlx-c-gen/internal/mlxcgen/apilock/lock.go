package apilock

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const SchemaVersion = 1

// Lock records the public C API surface for all shipped C targets.
type Lock struct {
	SchemaVersion int               `json:"schema_version"`
	Targets       map[string]Target `json:"targets"`
}

// Target records the public API surface for one C library target.
type Target struct {
	Umbrella    string       `json:"umbrella"`
	Headers     []string     `json:"headers"`
	ResultCodes []ResultCode `json:"result_codes,omitempty"`
	Macros      []Macro      `json:"macros,omitempty"`
	Typedefs    []Typedef    `json:"typedefs,omitempty"`
	Structs     []Struct     `json:"structs,omitempty"`
	Enums       []Enum       `json:"enums,omitempty"`
	Functions   []Function   `json:"functions,omitempty"`
}

// ResultCode describes a conventional integer return code.
type ResultCode struct {
	Code    int    `json:"code"`
	Meaning string `json:"meaning"`
}

// Macro records a function-like public macro.
type Macro struct {
	Name       string `json:"name"`
	Header     string `json:"header"`
	Definition string `json:"definition"`
}

// Typedef records a public C typedef that is not a struct or enum definition.
type Typedef struct {
	Name        string `json:"name"`
	Header      string `json:"header"`
	Declaration string `json:"declaration"`
}

// Struct records a public C struct typedef.
type Struct struct {
	Name   string  `json:"name"`
	Header string  `json:"header"`
	Opaque bool    `json:"opaque"`
	Fields []Field `json:"fields,omitempty"`
}

// Field records a public struct field.
type Field struct {
	Name        string `json:"name"`
	Declaration string `json:"declaration"`
}

// Enum records a public C enum typedef.
type Enum struct {
	Name   string      `json:"name"`
	Header string      `json:"header"`
	Values []EnumValue `json:"values"`
}

// EnumValue records a public enum constant and its integer value.
type EnumValue struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

// Function records a public C function declaration.
type Function struct {
	Name        string       `json:"name"`
	Header      string       `json:"header"`
	Return      string       `json:"return"`
	Parameters  []string     `json:"parameters"`
	Signature   string       `json:"signature"`
	ResultCodes []ResultCode `json:"result_codes,omitempty"`
}

// Generate extracts the API lock from headersDir, normally mlx/c.
func Generate(headersDir string) (*Lock, error) {
	headersDir = filepath.Clean(headersDir)

	mlxcHeaders, err := collectHeaders(headersDir, "mlx.h")
	if err != nil {
		return nil, err
	}
	targets := map[string]Target{
		"mlxc": {
			Umbrella: headerPath("mlx.h"),
			Headers:  mlxcHeaders,
		},
	}
	// The jacclc target is optional: cuts that defer JACCL have no jaccl.h.
	if _, err := os.Stat(filepath.Join(headersDir, "jaccl.h")); err == nil {
		jacclHeader := headerPath("jaccl.h")
		targets["jacclc"] = Target{
			Umbrella: jacclHeader,
			Headers:  []string{jacclHeader},
		}
	}
	for name, target := range targets {
		if err := fillTarget(headersDir, &target); err != nil {
			return nil, err
		}
		if err := validateTarget(name, target); err != nil {
			return nil, err
		}
		targets[name] = target
	}

	return &Lock{
		SchemaVersion: SchemaVersion,
		Targets:       targets,
	}, nil
}

// GenerateTarget extracts a single target from the headers reachable from
// umbrella under headersDir. It is used to read a foreign header tree, such as
// an upstream mlx-c checkout, with the same parser that produces our own lock.
func GenerateTarget(headersDir, umbrella string) (Target, error) {
	headersDir = filepath.Clean(headersDir)
	headers, err := collectHeaders(headersDir, umbrella)
	if err != nil {
		return Target{}, err
	}
	target := Target{Umbrella: headerPath(umbrella), Headers: headers}
	if err := fillTarget(headersDir, &target); err != nil {
		return Target{}, err
	}
	return target, nil
}

func fillTarget(headersDir string, target *Target) error {
	for _, header := range target.Headers {
		parsed, err := parseHeader(headersDir, header)
		if err != nil {
			return err
		}
		target.Macros = append(target.Macros, parsed.Macros...)
		target.Typedefs = append(target.Typedefs, parsed.Typedefs...)
		target.Structs = append(target.Structs, parsed.Structs...)
		target.Enums = append(target.Enums, parsed.Enums...)
		target.Functions = append(target.Functions, parsed.Functions...)
	}
	sortTarget(target)
	return nil
}

// Load reads a lock file written by JSON.
func Load(path string) (*Lock, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read api lock: %w", err)
	}
	return Decode(data)
}

// Decode parses the JSON representation of a lock.
func Decode(data []byte) (*Lock, error) {
	var lock Lock
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, fmt.Errorf("parse api lock: %w", err)
	}
	return &lock, nil
}

// JSON returns the deterministic JSON representation of lock.
func (l *Lock) JSON() ([]byte, error) {
	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal api lock: %w", err)
	}
	return append(data, '\n'), nil
}

// LockC returns a C translation unit that checks the locked declarations.
func (l *Lock) LockC() ([]byte, error) {
	var b bytes.Buffer
	fmt.Fprintln(&b, "/* Code generated by mlx-c-lock; DO NOT EDIT. */")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "#include <stddef.h>")
	fmt.Fprintln(&b, "#include \"mlx/c/mlx.h\"")
	fmt.Fprintln(&b, "#include \"mlx/c/jaccl.h\"")
	fmt.Fprintln(&b)

	targetNames := make([]string, 0, len(l.Targets))
	for name := range l.Targets {
		targetNames = append(targetNames, name)
	}
	sort.Strings(targetNames)

	for _, targetName := range targetNames {
		target := l.Targets[targetName]
		fmt.Fprintf(&b, "/* %s macros */\n", targetName)
		for _, macro := range target.Macros {
			writeMacroCheck(&b, macro)
		}
		fmt.Fprintln(&b)

		fmt.Fprintf(&b, "/* %s enums */\n", targetName)
		for _, enum := range target.Enums {
			for _, value := range enum.Values {
				fmt.Fprintf(&b, "_Static_assert(%s == %d, \"%s.%s ABI break\");\n",
					value.Name, value.Value, enum.Name, value.Name)
			}
		}
		fmt.Fprintln(&b)

		fmt.Fprintf(&b, "/* %s structs */\n", targetName)
		for _, st := range target.Structs {
			if st.Opaque {
				fmt.Fprintf(&b, "_Static_assert(sizeof(%s) == sizeof(void *), \"%s ABI break\");\n", st.Name, st.Name)
			}
			for _, field := range st.Fields {
				fmt.Fprintf(&b, "static int mlx_c_lock_%s_%s(void) { return (int)offsetof(%s, %s); }\n",
					st.Name, field.Name, st.Name, field.Name)
			}
		}
		fmt.Fprintln(&b)

		fmt.Fprintf(&b, "/* %s functions */\n", targetName)
		for _, fn := range target.Functions {
			fmt.Fprintf(&b, "extern %s;\n", fn.Signature)
		}
		fmt.Fprintln(&b)
	}
	return append(bytes.TrimRight(b.Bytes(), "\n"), '\n'), nil
}

func writeMacroCheck(b *bytes.Buffer, macro Macro) {
	if strings.HasPrefix(macro.Name, "MLX_C_HAS_") {
		fmt.Fprintf(b, "#if !defined(%s)\n", macro.Name)
		fmt.Fprintf(b, "#error \"%s API break\"\n", macro.Name)
		fmt.Fprintf(b, "#elif %s != 0 && %s != 1\n", macro.Name, macro.Name)
		fmt.Fprintf(b, "#error \"%s API break\"\n", macro.Name)
		fmt.Fprintln(b, "#endif")
		return
	}
	if value, ok := integerMacroValue(macro); ok {
		fmt.Fprintf(b, "#if %s != %d\n", macro.Name, value)
		fmt.Fprintf(b, "#error \"%s API break\"\n", macro.Name)
		fmt.Fprintln(b, "#endif")
	}
}

func integerMacroValue(macro Macro) (int, bool) {
	if strings.Contains(macro.Name, "(") {
		return 0, false
	}
	value, err := strconv.Atoi(macro.Definition)
	return value, err == nil
}

func collectHeaders(headersDir, root string) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	var visit func(string) error
	visit = func(name string) error {
		rel := normalizeHeader(name)
		if seen[rel] {
			return nil
		}
		seen[rel] = true
		out = append(out, rel)

		data, err := os.ReadFile(filepath.Join(headersDir, filepath.Base(rel)))
		if err != nil {
			return fmt.Errorf("read %s: %w", rel, err)
		}
		for _, include := range localIncludes(string(data)) {
			if normalizeHeader(include) == headerPath("jaccl.h") {
				continue
			}
			if err := visit(include); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit(root); err != nil {
		return nil, err
	}
	return out, nil
}

func sortTarget(t *Target) {
	sort.Strings(t.Headers)
	sort.Slice(t.Macros, func(i, j int) bool {
		return t.Macros[i].Header+"/"+t.Macros[i].Name < t.Macros[j].Header+"/"+t.Macros[j].Name
	})
	sort.Slice(t.Typedefs, func(i, j int) bool {
		return t.Typedefs[i].Header+"/"+t.Typedefs[i].Name < t.Typedefs[j].Header+"/"+t.Typedefs[j].Name
	})
	sort.Slice(t.Structs, func(i, j int) bool {
		return t.Structs[i].Header+"/"+t.Structs[i].Name < t.Structs[j].Header+"/"+t.Structs[j].Name
	})
	sort.Slice(t.Enums, func(i, j int) bool {
		return t.Enums[i].Header+"/"+t.Enums[i].Name < t.Enums[j].Header+"/"+t.Enums[j].Name
	})
	sort.Slice(t.Functions, func(i, j int) bool {
		return t.Functions[i].Header+"/"+t.Functions[i].Name < t.Functions[j].Header+"/"+t.Functions[j].Name
	})
}

func validateTarget(name string, target Target) error {
	for _, fn := range target.Functions {
		isJaccl := strings.HasPrefix(fn.Name, "mlx_jaccl_")
		switch {
		case name == "mlxc" && isJaccl:
			return fmt.Errorf("mlxc target includes jaccl function %s from %s", fn.Name, fn.Header)
		case name == "jacclc" && !isJaccl:
			return fmt.Errorf("jacclc target includes non-jaccl function %s from %s", fn.Name, fn.Header)
		}
	}
	return nil
}

func headerPath(name string) string {
	return "mlx/c/" + filepath.Base(name)
}
