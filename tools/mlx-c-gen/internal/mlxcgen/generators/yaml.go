package generators

import (
	"io"
	"sort"

	"github.com/tmc/mlx-c/tools/mlx-c-gen/internal/mlxcgen/ir"
	"github.com/tmc/mlx-c/tools/mlx-c-gen/internal/mlxcgen/parser"
	"gopkg.in/yaml.v3"
)

// YamlGenerator generates YAML metadata from parsed C++ headers.
type YamlGenerator struct{}

// NewYaml creates a new YamlGenerator.
func NewYaml() *YamlGenerator {
	return &YamlGenerator{}
}

// Metadata represents the top-level structure of the YAML output.
type Metadata struct {
	IR           *ir.Result       `yaml:"ir,omitempty"`
	TypePolicyIR *ir.Result       `yaml:"type_policy_ir,omitempty"`
	Functions    []FunctionMeta   `yaml:"functions"`
	Enums        []EnumMeta       `yaml:"enums"`
	Diagnostics  []DiagnosticMeta `yaml:"diagnostics,omitempty"`
}

// FunctionMeta represents metadata for a single function.
type FunctionMeta struct {
	Name      string      `yaml:"name"`
	CName     string      `yaml:"cname"`
	Namespace string      `yaml:"namespace"`
	Doc       string      `yaml:"doc,omitempty"`
	Params    []ParamMeta `yaml:"params"`
	Returns   string      `yaml:"returns"`
	Variant   string      `yaml:"variant,omitempty"`
}

// ParamMeta represents metadata for a function parameter.
type ParamMeta struct {
	Name    string `yaml:"name"`
	Type    string `yaml:"type"`
	Default string `yaml:"default,omitempty"`
}

// EnumMeta represents metadata for an enum.
type EnumMeta struct {
	Name      string   `yaml:"name"`
	Namespace string   `yaml:"namespace"`
	Values    []string `yaml:"values"`
}

// DiagnosticMeta represents a parser diagnostic in the metadata output.
type DiagnosticMeta struct {
	Code    string `yaml:"code"`
	Message string `yaml:"message"`
	Reason  string `yaml:"reason,omitempty"`
	File    string `yaml:"file,omitempty"`
	Line    int    `yaml:"line,omitempty"`
	Col     int    `yaml:"col,omitempty"`
}

// GenerateYaml generates YAML metadata for the given parsed result.
func (g *YamlGenerator) GenerateYaml(w io.Writer, result *parser.ParseResult) error {
	return g.GenerateYamlWithIR(w, result, ir.Result{})
}

// GenerateYamlWithIR generates YAML metadata with normalized declarations.
func (g *YamlGenerator) GenerateYamlWithIR(w io.Writer, result *parser.ParseResult, decls ir.Result) error {
	return g.GenerateYamlWithTypePolicyIR(w, result, decls, ir.Result{})
}

// GenerateYamlWithTypePolicyIR generates YAML metadata with full normalized
// declarations and the selected declarations checked by the type policy.
func (g *YamlGenerator) GenerateYamlWithTypePolicyIR(w io.Writer, result *parser.ParseResult, decls, typePolicyIR ir.Result) error {
	meta := Metadata{
		Functions: []FunctionMeta{},
		Enums:     []EnumMeta{},
	}
	if !decls.Empty() {
		decls.Sort()
		meta.IR = &decls
	}
	if !typePolicyIR.Empty() {
		typePolicyIR.Sort()
		meta.TypePolicyIR = &typePolicyIR
	}

	// Collect functions
	for _, funcs := range result.Functions {
		for _, f := range funcs {
			fm := FunctionMeta{
				Name:      f.Name,
				CName:     cFuncName(f.Namespace, f.Name, f.Variant),
				Namespace: f.Namespace,
				Doc:       f.Doc,
				Returns:   f.ReturnType,
				Variant:   f.Variant,
			}

			for i, pName := range f.ParamNames {
				pm := ParamMeta{
					Name: pName,
					Type: f.ParamTypes[i],
				}
				if i < len(f.ParamDefault) {
					pm.Default = f.ParamDefault[i]
				}
				fm.Params = append(fm.Params, pm)
			}

			meta.Functions = append(meta.Functions, fm)
		}
	}

	// Sort functions for deterministic output
	sort.Slice(meta.Functions, func(i, j int) bool {
		if meta.Functions[i].Namespace != meta.Functions[j].Namespace {
			return meta.Functions[i].Namespace < meta.Functions[j].Namespace
		}
		if meta.Functions[i].Name != meta.Functions[j].Name {
			return meta.Functions[i].Name < meta.Functions[j].Name
		}
		return meta.Functions[i].Variant < meta.Functions[j].Variant
	})

	// Collect enums
	for _, e := range result.Enums {
		em := EnumMeta{
			Name:      e.Name,
			Namespace: e.Namespace,
			Values:    e.Values,
		}
		meta.Enums = append(meta.Enums, em)
	}

	// Sort enums
	sort.Slice(meta.Enums, func(i, j int) bool {
		return meta.Enums[i].Name < meta.Enums[j].Name
	})

	for _, d := range result.Diagnostics {
		meta.Diagnostics = append(meta.Diagnostics, DiagnosticMeta{
			Code:    d.Code,
			Message: d.Message,
			Reason:  d.Reason,
			File:    d.File,
			Line:    d.Line,
			Col:     d.Col,
		})
	}
	sort.Slice(meta.Diagnostics, func(i, j int) bool {
		a, b := meta.Diagnostics[i], meta.Diagnostics[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Col != b.Col {
			return a.Col < b.Col
		}
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		return a.Message < b.Message
	})

	// Encode to YAML
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	return enc.Encode(meta)
}

func cFuncName(namespace, name, variant string) string {
	funcName := cNamespace(namespace) + "_" + name
	if variant != "" {
		funcName += "_" + variant
	}
	return funcName
}
