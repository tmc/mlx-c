package ir

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/tmc/mlx-c/internal/mlxcgen/parser"
)

// DeclID identifies an upstream declaration by module, header, namespace, name,
// and canonical signature.
type DeclID string

// Result holds normalized declarations discovered from parsed headers.
type Result struct {
	Functions []FuncDecl `json:"functions,omitempty" yaml:"functions,omitempty"`
	Enums     []EnumDecl `json:"enums,omitempty" yaml:"enums,omitempty"`
}

// FuncDecl records a normalized function declaration.
type FuncDecl struct {
	ID        DeclID    `json:"id" yaml:"id"`
	Module    string    `json:"module" yaml:"module"`
	Header    string    `json:"header,omitempty" yaml:"header,omitempty"`
	Namespace string    `json:"namespace" yaml:"namespace"`
	Name      string    `json:"name" yaml:"name"`
	Return    string    `json:"return" yaml:"return"`
	Params    []Param   `json:"params,omitempty" yaml:"params,omitempty"`
	Comment   string    `json:"comment,omitempty" yaml:"comment,omitempty"`
	Loc       SourceLoc `json:"loc,omitempty" yaml:"loc,omitempty"`
}

// Param records a normalized function parameter.
type Param struct {
	Name    string `json:"name,omitempty" yaml:"name,omitempty"`
	Type    string `json:"type" yaml:"type"`
	Default string `json:"default,omitempty" yaml:"default,omitempty"`
}

// EnumDecl records a normalized enum declaration.
type EnumDecl struct {
	ID        DeclID    `json:"id" yaml:"id"`
	Module    string    `json:"module" yaml:"module"`
	Header    string    `json:"header,omitempty" yaml:"header,omitempty"`
	Namespace string    `json:"namespace" yaml:"namespace"`
	Name      string    `json:"name" yaml:"name"`
	Values    []string  `json:"values,omitempty" yaml:"values,omitempty"`
	Loc       SourceLoc `json:"loc,omitempty" yaml:"loc,omitempty"`
}

// SourceLoc records a source location.
type SourceLoc struct {
	File string `json:"file,omitempty" yaml:"file,omitempty"`
	Line int    `json:"line,omitempty" yaml:"line,omitempty"`
	Col  int    `json:"col,omitempty" yaml:"col,omitempty"`
}

// FromParseResult normalizes parser output for one generator module.
func FromParseResult(module string, result *parser.ParseResult) Result {
	if result == nil {
		return Result{}
	}
	var out Result
	for _, funcs := range result.Functions {
		for _, f := range funcs {
			out.Functions = append(out.Functions, funcDecl(module, f))
		}
	}
	for _, enum := range result.Enums {
		out.Enums = append(out.Enums, enumDecl(module, enum))
	}
	out.Sort()
	return out
}

// Merge combines normalized declaration results and returns them sorted.
func Merge(results ...Result) Result {
	var out Result
	for _, result := range results {
		out.Functions = append(out.Functions, result.Functions...)
		out.Enums = append(out.Enums, result.Enums...)
	}
	out.Sort()
	return out
}

// Empty reports whether r contains no declarations.
func (r Result) Empty() bool {
	return len(r.Functions) == 0 && len(r.Enums) == 0
}

// Sort orders declarations deterministically.
func (r *Result) Sort() {
	sort.Slice(r.Functions, func(i, j int) bool {
		return r.Functions[i].ID < r.Functions[j].ID
	})
	sort.Slice(r.Enums, func(i, j int) bool {
		return r.Enums[i].ID < r.Enums[j].ID
	})
}

func funcDecl(module string, f *parser.Function) FuncDecl {
	return NewFuncDecl(module, f.File, f.Namespace, f.Name, f.ReturnType, f.ParamTypes, f.ParamNames, f.ParamDefault, f.Doc, f.Line, f.Col)
}

// NewFuncDecl records a normalized function declaration.
func NewFuncDecl(module, file, namespace, name, returnType string, paramTypes, paramNames, paramDefaults []string, comment string, line, col int) FuncDecl {
	header := headerPath(file)
	params := make([]Param, 0, len(paramTypes))
	for i, typ := range paramTypes {
		param := Param{Type: typ}
		if i < len(paramNames) {
			param.Name = paramNames[i]
		}
		if i < len(paramDefaults) {
			param.Default = paramDefaults[i]
		}
		params = append(params, param)
	}
	signature := canonicalSignature(returnType, paramTypes)
	return FuncDecl{
		ID:        declID(module, header, namespace, name, signature),
		Module:    module,
		Header:    header,
		Namespace: namespace,
		Name:      name,
		Return:    returnType,
		Params:    params,
		Comment:   comment,
		Loc: SourceLoc{
			File: header,
			Line: line,
			Col:  col,
		},
	}
}

func enumDecl(module string, e *parser.Enum) EnumDecl {
	signature := strings.Join(e.Values, ",")
	return EnumDecl{
		ID:        declID(module, "", e.Namespace, e.Name, signature),
		Module:    module,
		Namespace: e.Namespace,
		Name:      e.Name,
		Values:    append([]string(nil), e.Values...),
	}
}

func declID(module, header, namespace, name, signature string) DeclID {
	return DeclID(strings.Join([]string{module, header, namespace, name, signature}, "|"))
}

func canonicalSignature(ret string, params []string) string {
	return ret + "(" + strings.Join(params, ", ") + ")"
}

func headerPath(path string) string {
	path = filepath.ToSlash(filepath.Clean(path))
	if path == "." {
		return ""
	}
	if strings.HasPrefix(path, "mlx/") {
		return path
	}
	if i := strings.LastIndex(path, "/mlx/"); i >= 0 {
		return path[i+1:]
	}
	return path
}
