package generators

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tmc/mlx-c/tools/mlx-c-gen/internal/mlxcgen/parser"
)

func TestGenerateGraphUtilsHooks(t *testing.T) {
	result := &parser.ParseResult{
		Functions: map[string][]*parser.Function{
			"mlx::core::export_to_dot": graphUtilsOverloads("export_to_dot"),
			"mlx::core::print_graph":   graphUtilsOverloads("print_graph"),
		},
		Enums: map[string]*parser.Enum{},
	}

	var header bytes.Buffer
	if err := New().Generate(&header, result, "graph_utils", nil, false, "Graph Utils"); err != nil {
		t.Fatalf("Generate header: %v", err)
	}
	headerText := header.String()
	for _, want := range []string{
		"typedef struct mlx_node_namer_",
		"mlx_node_namer mlx_node_namer_new()",
		"int mlx_export_to_dot(",
		"int mlx_print_graph(",
	} {
		if !strings.Contains(headerText, want) {
			t.Fatalf("header missing %q\n%s", want, headerText)
		}
	}
	for _, want := range []string{
		"int mlx_export_to_dot(",
		"int mlx_print_graph(",
	} {
		if got := strings.Count(headerText, want); got != 1 {
			t.Fatalf("header contains %q %d times, want 1\n%s", want, got, headerText)
		}
	}

	var impl bytes.Buffer
	if err := New().Generate(&impl, result, "graph_utils", []string{"mlx/graph_utils.h"}, true, "Graph Utils"); err != nil {
		t.Fatalf("Generate implementation: %v", err)
	}
	implText := impl.String()
	for _, want := range []string{
		"extern \"C\" mlx_node_namer mlx_node_namer_new()",
		"extern \"C\" int mlx_export_to_dot(",
		"extern \"C\" int mlx_print_graph(",
		"CFileOutputStream::as_lvalue(CFileOutputStream(os))",
	} {
		if !strings.Contains(implText, want) {
			t.Fatalf("implementation missing %q\n%s", want, implText)
		}
	}
	for _, want := range []string{
		"extern \"C\" int mlx_export_to_dot(",
		"extern \"C\" int mlx_print_graph(",
	} {
		if got := strings.Count(implText, want); got != 1 {
			t.Fatalf("implementation contains %q %d times, want 1\n%s", want, got, implText)
		}
	}
}

func graphUtilsOverloads(name string) []*parser.Function {
	return []*parser.Function{
		{
			Name:       name,
			Namespace:  "mlx::core",
			ReturnType: "void",
			ParamTypes: []string{
				"std::ostream",
				"NodeNamer",
				"Arrays&&...",
			},
			ParamNames: []string{"os", "namer", "outputs"},
		},
		{
			Name:       name,
			Namespace:  "mlx::core",
			ReturnType: "void",
			ParamTypes: []string{
				"std::ostream",
				"std::vector<array>",
			},
			ParamNames: []string{"os", "outputs"},
		},
		{
			Name:       name,
			Namespace:  "mlx::core",
			ReturnType: "void",
			ParamTypes: []string{
				"std::ostream",
				"NodeNamer",
				"std::vector<array>",
			},
			ParamNames: []string{"os", "namer", "outputs"},
		},
		{
			Name:       name,
			Namespace:  "mlx::core",
			ReturnType: "void",
			ParamTypes: []string{
				"std::ostream",
				"Arrays&&...",
			},
			ParamNames: []string{"os", "outputs"},
		},
	}
}
