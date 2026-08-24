package apilock

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

type parsedHeader struct {
	Macros    []Macro
	Typedefs  []Typedef
	Structs   []Struct
	Enums     []Enum
	Functions []Function
}

var (
	includeRE  = regexp.MustCompile(`#include\s+"([^"]+)"`)
	macroRE    = regexp.MustCompile(`(?m)^\s*#\s*define\s+((?:mlx_[A-Za-z0-9_]*|MLX_C_HAS_[A-Z0-9_]+)(?:\([^)]*\))?)\s+(.+)$`)
	structRE   = regexp.MustCompile(`(?s)typedef\s+struct\s+([A-Za-z_][A-Za-z0-9_]*)?\s*\{(.*?)\}\s*([A-Za-z_][A-Za-z0-9_]*)\s*;`)
	enumRE     = regexp.MustCompile(`(?s)typedef\s+enum\s+([A-Za-z_][A-Za-z0-9_]*)?\s*\{(.*?)\}\s*([A-Za-z_][A-Za-z0-9_]*)\s*;`)
	typedefRE  = regexp.MustCompile(`^typedef\s+(.+?)\s+([A-Za-z_][A-Za-z0-9_]*)$`)
	ftypedefRE = regexp.MustCompile(`^typedef\s+(.+?)\(\s*\*\s*([A-Za-z_][A-Za-z0-9_]*)\s*\)\s*(\(.*\))$`)
	fieldPtrRE = regexp.MustCompile(`\(\s*\*\s*([A-Za-z_][A-Za-z0-9_]*)\s*\)`)
	identRE    = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)
)

func parseHeader(headersDir, header string) (parsedHeader, error) {
	data, err := os.ReadFile(filepath.Join(headersDir, filepath.Base(header)))
	if err != nil {
		return parsedHeader{}, fmt.Errorf("read %s: %w", header, err)
	}
	return parseHeaderText(header, string(data))
}

func parseHeaderText(header, text string) (parsedHeader, error) {
	var out parsedHeader
	for _, m := range macroRE.FindAllStringSubmatch(text, -1) {
		out.Macros = append(out.Macros, Macro{
			Name:       strings.TrimSpace(m[1]),
			Header:     header,
			Definition: normalizeSpace(m[2]),
		})
	}

	code := stripComments(text)
	for _, m := range structRE.FindAllStringSubmatch(code, -1) {
		fields := parseFields(m[2])
		out.Structs = append(out.Structs, Struct{
			Name:   m[3],
			Header: header,
			Opaque: isOpaque(fields),
			Fields: fields,
		})
	}
	code = structRE.ReplaceAllString(code, "")

	for _, m := range enumRE.FindAllStringSubmatch(code, -1) {
		values, err := parseEnumValues(m[2])
		if err != nil {
			return parsedHeader{}, fmt.Errorf("%s: parse enum %s: %w", header, m[3], err)
		}
		out.Enums = append(out.Enums, Enum{
			Name:   m[3],
			Header: header,
			Values: values,
		})
	}
	code = enumRE.ReplaceAllString(code, "")

	code = stripPreprocessor(code)
	code = stripExternC(code)
	for _, decl := range declarations(code) {
		if strings.HasPrefix(decl, "extern ") || decl == `extern "C"` {
			continue
		}
		if strings.HasPrefix(decl, "typedef ") {
			td, ok := parseTypedef(header, decl)
			if ok {
				out.Typedefs = append(out.Typedefs, td)
			}
			continue
		}
		fn, ok := parseFunction(header, decl)
		if ok {
			out.Functions = append(out.Functions, fn)
		}
	}
	return out, nil
}

// ParseHeaderContent extracts public API declarations from one header body.
func ParseHeaderContent(header string, data []byte) (Target, error) {
	parsed, err := parseHeaderText(header, string(data))
	if err != nil {
		return Target{}, err
	}
	target := Target{
		Headers:   []string{header},
		Macros:    parsed.Macros,
		Typedefs:  parsed.Typedefs,
		Structs:   parsed.Structs,
		Enums:     parsed.Enums,
		Functions: parsed.Functions,
	}
	sortTarget(&target)
	return target, nil
}

func localIncludes(text string) []string {
	var out []string
	for _, m := range includeRE.FindAllStringSubmatch(text, -1) {
		out = append(out, m[1])
	}
	return out
}

func normalizeHeader(include string) string {
	if strings.HasPrefix(include, "mlx/c/") {
		return filepath.ToSlash(include)
	}
	return headerPath(include)
}

func parseFields(body string) []Field {
	var fields []Field
	for _, part := range strings.Split(body, ";") {
		decl := normalizeDecl(part)
		if decl == "" {
			continue
		}
		name := fieldName(decl)
		if name == "" {
			continue
		}
		fields = append(fields, Field{Name: name, Declaration: decl})
	}
	return fields
}

func fieldName(decl string) string {
	if m := fieldPtrRE.FindStringSubmatch(decl); len(m) == 2 {
		return m[1]
	}
	ids := identRE.FindAllString(decl, -1)
	if len(ids) == 0 {
		return ""
	}
	return ids[len(ids)-1]
}

func isOpaque(fields []Field) bool {
	if len(fields) != 1 || fields[0].Name != "ctx" {
		return false
	}
	return strings.ReplaceAll(fields[0].Declaration, " ", "") == "void*ctx"
}

func parseEnumValues(body string) ([]EnumValue, error) {
	var values []EnumValue
	next := 0
	for _, part := range strings.Split(body, ",") {
		part = normalizeSpace(part)
		if part == "" {
			continue
		}
		name, valueText, ok := strings.Cut(part, "=")
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		value := next
		if ok {
			n, err := strconv.Atoi(strings.TrimSpace(valueText))
			if err != nil {
				return nil, fmt.Errorf("unsupported enum value %q", part)
			}
			value = n
		}
		values = append(values, EnumValue{Name: name, Value: value})
		next = value + 1
	}
	return values, nil
}

func parseTypedef(header, decl string) (Typedef, bool) {
	if strings.HasPrefix(decl, "typedef struct ") || strings.HasPrefix(decl, "typedef enum ") {
		return Typedef{}, false
	}
	if m := ftypedefRE.FindStringSubmatch(decl); len(m) == 4 {
		return Typedef{
			Name:        m[2],
			Header:      header,
			Declaration: decl,
		}, true
	}
	if m := typedefRE.FindStringSubmatch(decl); len(m) == 3 {
		return Typedef{
			Name:        m[2],
			Header:      header,
			Declaration: decl,
		}, true
	}
	return Typedef{}, false
}

func parseFunction(header, decl string) (Function, bool) {
	open := strings.IndexByte(decl, '(')
	if open < 0 {
		return Function{}, false
	}
	close := matchingParen(decl, open)
	if close < 0 {
		return Function{}, false
	}
	prefix := strings.TrimSpace(decl[:open])
	ids := identRE.FindAllStringIndex(prefix, -1)
	if len(ids) == 0 {
		return Function{}, false
	}
	last := ids[len(ids)-1]
	name := prefix[last[0]:last[1]]
	if !strings.HasPrefix(name, "mlx_") && !strings.HasPrefix(name, "_mlx_") {
		return Function{}, false
	}
	ret := normalizeSpace(strings.TrimSpace(prefix[:last[0]]))
	if ret == "" {
		return Function{}, false
	}
	params := splitParams(decl[open+1 : close])
	signature := normalizeDecl(decl)
	if strings.HasSuffix(signature, ";") {
		signature = strings.TrimSuffix(signature, ";")
	}
	fn := Function{
		Name:       name,
		Header:     header,
		Return:     ret,
		Parameters: params,
		Signature:  signature,
	}
	return fn, true
}

func declarations(code string) []string {
	var out []string
	var b strings.Builder
	depth := 0
	for _, r := range code {
		switch r {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ';':
			if depth == 0 {
				decl := normalizeDecl(b.String())
				if decl != "" {
					out = append(out, decl)
				}
				b.Reset()
				continue
			}
		}
		b.WriteRune(r)
	}
	return out
}

func splitParams(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" || s == "void" {
		return nil
	}
	var out []string
	start := 0
	depth := 0
	for i, r := range s {
		switch r {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				out = append(out, normalizeSpace(s[start:i]))
				start = i + len(string(r))
			}
		}
	}
	out = append(out, normalizeSpace(s[start:]))
	return out
}

func matchingParen(s string, open int) int {
	depth := 0
	for i := open; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func stripComments(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if i+1 < len(s) && s[i] == '/' && s[i+1] == '/' {
			for i < len(s) && s[i] != '\n' {
				i++
			}
			continue
		}
		if i+1 < len(s) && s[i] == '/' && s[i+1] == '*' {
			i += 2
			for i+1 < len(s) && !(s[i] == '*' && s[i+1] == '/') {
				if s[i] == '\n' {
					b.WriteByte('\n')
				}
				i++
			}
			if i+1 < len(s) {
				i += 2
			}
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func stripPreprocessor(s string) string {
	var b strings.Builder
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

func stripExternC(s string) string {
	s = strings.ReplaceAll(s, `extern "C" {`, "")
	var b strings.Builder
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) == "}" {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

func normalizeDecl(s string) string {
	return strings.TrimSpace(normalizeSpace(s))
}

func normalizeSpace(s string) string {
	var b strings.Builder
	space := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			space = true
			continue
		}
		if space && b.Len() > 0 && !strings.ContainsRune("),;]", r) {
			last := lastByte(&b)
			if last != '(' && last != '[' && !strings.HasSuffix(b.String(), "(*") {
				b.WriteByte(' ')
			}
		}
		b.WriteRune(r)
		space = false
	}
	return strings.TrimSpace(b.String())
}

func lastByte(b *strings.Builder) byte {
	s := b.String()
	return s[len(s)-1]
}
