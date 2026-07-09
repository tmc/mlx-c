package generators

import (
	"testing"

	"github.com/tmc/mlx-c/internal/mlxcgen/parser"
	"github.com/tmc/mlx-c/internal/mlxcgen/plan"
)

func TestDiagnosticsReportsUnsupportedTypes(t *testing.T) {
	result := &parser.ParseResult{
		Functions: map[string][]*parser.Function{
			"mlx::core::bad_return": {{
				Name:       "bad_return",
				Namespace:  "mlx::core",
				ReturnType: "UnsupportedReturn",
				File:       "ops.h",
				Line:       12,
				Col:        3,
			}},
			"mlx::core::bad_param": {{
				Name:       "bad_param",
				Namespace:  "mlx::core",
				ReturnType: "void",
				ParamTypes: []string{"array", "UnsupportedParam"},
				ParamNames: []string{"x", "bad"},
				File:       "ops.h",
				Line:       13,
				Col:        3,
			}},
			"mlx::core::synthetic_ok": {{
				Name:       "synthetic_ok",
				Namespace:  "mlx::core",
				ReturnType: "array",
				ParamTypes: []string{"array", "array"},
				ParamNames: []string{"a", "b"},
				File:       "ops.h",
				Line:       14,
				Col:        3,
			}},
		},
		Enums: map[string]*parser.Enum{},
	}

	diagnostics := New().Diagnostics(result)
	want := map[string]int{
		"skip_unsupported_return_type": 12,
		"skip_unsupported_param_type":  13,
	}
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == "emit_implicit_single_overload" {
			continue
		}
		line, ok := want[diagnostic.Code]
		if !ok {
			t.Fatalf("unexpected diagnostic: %#v", diagnostic)
		}
		if diagnostic.File != "ops.h" || diagnostic.Line != line || diagnostic.Col != 3 {
			t.Fatalf("diagnostic location = %#v, want ops.h:%d:3", diagnostic, line)
		}
		delete(want, diagnostic.Code)
	}
	if len(want) != 0 {
		t.Fatalf("missing diagnostics: %#v", want)
	}
}

func TestDiagnosticsSkipHookedFunctions(t *testing.T) {
	result := &parser.ParseResult{
		Functions: map[string][]*parser.Function{
			"mlx::core::fast::metal_kernel": {{
				Name:       "metal_kernel",
				Namespace:  "mlx::core::fast",
				ReturnType: "void",
				ParamTypes: []string{"Unsupported"},
				ParamNames: []string{"x"},
				File:       "fast.h",
				Line:       4,
				Col:        1,
			}},
		},
		Enums: map[string]*parser.Enum{},
	}

	for _, diagnostic := range New().Diagnostics(result) {
		if diagnostic.Code == "skip_unsupported_param_type" || diagnostic.Code == "skip_unsupported_return_type" {
			t.Fatalf("diagnostics = %#v, want no unsupported type diagnostics for hooked function", diagnostic)
		}
	}
}

func TestSelectedGeneratedIRExcludesSkippedAndHookedFunctions(t *testing.T) {
	base := ""
	result := &parser.ParseResult{
		Functions: map[string][]*parser.Function{
			"mlx::core::good": {{
				Name:       "good",
				Namespace:  "mlx::core",
				ReturnType: "array",
				ParamTypes: []string{"array"},
				ParamNames: []string{"x"},
				File:       "mlx/ops.h",
				Line:       10,
				Col:        2,
			}},
			"mlx::core::bad": {{
				Name:       "bad",
				Namespace:  "mlx::core",
				ReturnType: "Missing",
				ParamTypes: []string{"array"},
				ParamNames: []string{"x"},
				File:       "mlx/ops.h",
				Line:       11,
				Col:        2,
			}},
			"mlx::core::fast::metal_kernel": {{
				Name:       "metal_kernel",
				Namespace:  "mlx::core::fast",
				ReturnType: "CustomKernelFunction",
				ParamTypes: []string{"std::string"},
				ParamNames: []string{"name"},
				File:       "mlx/fast.h",
				Line:       12,
				Col:        2,
			}},
		},
		Enums: map[string]*parser.Enum{},
	}
	gen := NewWithManifest(plan.Manifest{
		VariantMappings: map[string]map[string][]plan.Variant{
			"mlx_core": {
				"good": {
					{Signature: "array(array)", Suffix: &base},
				},
				"bad": {
					{Signature: "Missing(array)", Skip: true, Reason: "unsupported_type"},
				},
			},
			"mlx_core_fast": {
				"metal_kernel": {
					{Signature: "CustomKernelFunction(std::string)", Suffix: &base},
				},
			},
		},
	})

	got, err := gen.SelectedGeneratedIR("ops", result)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Functions) != 1 {
		t.Fatalf("selected functions = %#v, want one", got.Functions)
	}
	fn := got.Functions[0]
	if fn.Name != "good" || fn.ID != "ops|mlx/ops.h|mlx::core|good|array(array)" {
		t.Fatalf("selected function = %#v, want good", fn)
	}
}

func TestDiagnosticsReportsVariantSkips(t *testing.T) {
	result := &parser.ParseResult{
		Functions: map[string][]*parser.Function{
			"mlx::core::metal::device_info": {{
				Name:       "device_info",
				Namespace:  "mlx::core::metal",
				ReturnType: "std::unordered_map<std::string, std::variant<std::string, size_t>>",
				File:       "metal.h",
				Line:       22,
				Col:        5,
			}},
		},
		Enums: map[string]*parser.Enum{},
	}

	diagnostics := New().Diagnostics(result)
	if len(diagnostics) != 1 {
		t.Fatalf("diagnostics = %#v, want one", diagnostics)
	}
	diagnostic := diagnostics[0]
	if diagnostic.Code != "skip_variant_mapping" {
		t.Fatalf("diagnostic code = %q, want skip_variant_mapping", diagnostic.Code)
	}
	if diagnostic.File != "metal.h" || diagnostic.Line != 22 || diagnostic.Col != 5 {
		t.Fatalf("diagnostic location = %#v, want metal.h:22:5", diagnostic)
	}
}

func TestDiagnosticsReportsAllowedDetailOverloadSkips(t *testing.T) {
	result := &parser.ParseResult{
		Functions: map[string][]*parser.Function{
			"mlx::core::detail::compile": {
				{
					Name:       "compile",
					Namespace:  "mlx::core::detail",
					ReturnType: "array",
					ParamTypes: []string{"array", "array"},
					ParamNames: []string{"x", "y"},
					File:       "compile_impl.h",
					Line:       20,
					Col:        5,
				},
				{
					Name:       "compile",
					Namespace:  "mlx::core::detail",
					ReturnType: "array",
					ParamTypes: []string{"array"},
					ParamNames: []string{"x"},
					File:       "compile_impl.h",
					Line:       24,
					Col:        5,
				},
			},
		},
		Enums: map[string]*parser.Enum{},
	}
	gen := NewWithManifest(plan.Manifest{
		AllowedDetailFunctions: []string{"compile"},
	})

	var got *parser.Diagnostic
	for _, diagnostic := range gen.Diagnostics(result) {
		if diagnostic.Code == "skip_allowed_detail_overload" {
			diagnostic := diagnostic
			got = &diagnostic
			break
		}
	}
	if got == nil {
		t.Fatalf("diagnostics = %#v, missing skip_allowed_detail_overload", gen.Diagnostics(result))
	}
	if got.Reason != "covered_by_other_variant" {
		t.Fatalf("diagnostic reason = %q, want covered_by_other_variant", got.Reason)
	}
	if got.File != "compile_impl.h" || got.Line != 24 || got.Col != 5 {
		t.Fatalf("diagnostic location = %#v, want compile_impl.h:24:5", got)
	}
}

func TestDiagnosticsPreservesVariantSkipsOnSelectionError(t *testing.T) {
	result := &parser.ParseResult{
		Functions: map[string][]*parser.Function{
			"mlx::core::metal::device_info": {{
				Name:       "device_info",
				Namespace:  "mlx::core::metal",
				ReturnType: "std::unordered_map<std::string, std::variant<std::string, size_t>>",
				File:       "metal.h",
				Line:       22,
				Col:        5,
			}},
			"zzz::bad": {
				{
					Name:       "bad",
					Namespace:  "zzz",
					ReturnType: "void",
				},
				{
					Name:       "bad",
					Namespace:  "zzz",
					ReturnType: "bool",
				},
			},
		},
		Enums: map[string]*parser.Enum{},
	}

	diagnostics := New().Diagnostics(result)
	got := map[string]bool{}
	for _, diagnostic := range diagnostics {
		got[diagnostic.Code] = true
	}
	for _, code := range []string{"skip_variant_mapping", "variant_selection_error"} {
		if !got[code] {
			t.Fatalf("diagnostics = %#v, missing %s", diagnostics, code)
		}
	}
}
