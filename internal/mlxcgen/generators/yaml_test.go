package generators

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tmc/mlx-c/internal/mlxcgen/ir"
	"github.com/tmc/mlx-c/internal/mlxcgen/parser"
)

func TestGenerateYamlIncludesParserDiagnostics(t *testing.T) {
	result := &parser.ParseResult{
		Functions: make(map[string][]*parser.Function),
		Enums:     make(map[string]*parser.Enum),
		Diagnostics: []parser.Diagnostic{{
			Code:    "skip_template_function",
			Message: "mlx::core::identity uses template parameters",
			Reason:  "template_function",
			File:    "/tmp/ops.h",
			Line:    12,
			Col:     3,
		}},
	}

	var buf bytes.Buffer
	if err := NewYaml().GenerateYaml(&buf, result); err != nil {
		t.Fatalf("GenerateYaml: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"diagnostics:",
		"code: skip_template_function",
		"message: mlx::core::identity uses template parameters",
		"reason: template_function",
		"file: /tmp/ops.h",
		"line: 12",
		"col: 3",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("metadata missing %q:\n%s", want, out)
		}
	}
}

func TestGenerateYamlIncludesIR(t *testing.T) {
	result := &parser.ParseResult{
		Functions: map[string][]*parser.Function{},
		Enums:     map[string]*parser.Enum{},
	}
	decls := ir.Result{
		Functions: []ir.FuncDecl{{
			ID:        "ops|mlx/ops.h|mlx::core|add|array(array, array)",
			Module:    "ops",
			Header:    "mlx/ops.h",
			Namespace: "mlx::core",
			Name:      "add",
			Return:    "array",
			Params: []ir.Param{
				{Name: "a", Type: "array"},
				{Name: "b", Type: "array"},
			},
		}},
	}

	var buf bytes.Buffer
	if err := NewYaml().GenerateYamlWithIR(&buf, result, decls); err != nil {
		t.Fatalf("GenerateYamlWithIR: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"ir:",
		"id: ops|mlx/ops.h|mlx::core|add|array(array, array)",
		"module: ops",
		"header: mlx/ops.h",
		"namespace: mlx::core",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("metadata missing %q:\n%s", want, out)
		}
	}
}

func TestGenerateYamlIncludesTypePolicyIR(t *testing.T) {
	result := &parser.ParseResult{
		Functions: map[string][]*parser.Function{},
		Enums:     map[string]*parser.Enum{},
	}
	decls := ir.Result{
		Functions: []ir.FuncDecl{{
			ID:        "ops|mlx/ops.h|mlx::core|add|Missing(array)",
			Module:    "ops",
			Header:    "mlx/ops.h",
			Namespace: "mlx::core",
			Name:      "add",
			Return:    "Missing",
			Params:    []ir.Param{{Name: "x", Type: "array"}},
		}},
	}

	var buf bytes.Buffer
	if err := NewYaml().GenerateYamlWithTypePolicyIR(&buf, result, ir.Result{}, decls); err != nil {
		t.Fatalf("GenerateYamlWithTypePolicyIR: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"type_policy_ir:",
		"id: ops|mlx/ops.h|mlx::core|add|Missing(array)",
		"return: Missing",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("metadata missing %q:\n%s", want, out)
		}
	}
}
