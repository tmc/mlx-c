package ir

import (
	"testing"

	"github.com/tmc/mlx-c/tools/mlx-c-gen/internal/mlxcgen/parser"
)

func TestFromParseResultNormalizesFunctions(t *testing.T) {
	result := &parser.ParseResult{
		Functions: map[string][]*parser.Function{
			"mlx::core::add": {{
				Name:         "add",
				Namespace:    "mlx::core",
				ReturnType:   "array",
				ParamTypes:   []string{"array", "array", "StreamOrDevice"},
				ParamNames:   []string{"a", "b", "s"},
				ParamDefault: []string{"", "", "mlx::core::default_stream"},
				Doc:          "Adds arrays.",
				File:         "/tmp/mlx/mlx/ops.h",
				Line:         42,
				Col:          7,
			}},
		},
		Enums: map[string]*parser.Enum{
			"mlx::core::Dtype": {
				Name:      "Dtype",
				Namespace: "mlx::core",
				Values:    []string{"float32", "int32"},
			},
		},
	}

	got := FromParseResult("ops", result)
	if len(got.Functions) != 1 {
		t.Fatalf("functions = %#v", got.Functions)
	}
	fn := got.Functions[0]
	if fn.ID != "ops|mlx/ops.h|mlx::core|add|array(array, array, StreamOrDevice)" {
		t.Fatalf("id = %q", fn.ID)
	}
	if fn.Header != "mlx/ops.h" || fn.Loc.File != "mlx/ops.h" || fn.Loc.Line != 42 || fn.Loc.Col != 7 {
		t.Fatalf("location = %#v header %q", fn.Loc, fn.Header)
	}
	if len(fn.Params) != 3 || fn.Params[2].Default != "mlx::core::default_stream" {
		t.Fatalf("params = %#v", fn.Params)
	}
	if len(got.Enums) != 1 || got.Enums[0].ID != "ops||mlx::core|Dtype|float32,int32" {
		t.Fatalf("enums = %#v", got.Enums)
	}
}

func TestMergeSortsDeclarations(t *testing.T) {
	got := Merge(
		Result{Functions: []FuncDecl{{ID: "z"}}},
		Result{Functions: []FuncDecl{{ID: "a"}}},
	)
	if got.Functions[0].ID != "a" || got.Functions[1].ID != "z" {
		t.Fatalf("functions = %#v", got.Functions)
	}
}
