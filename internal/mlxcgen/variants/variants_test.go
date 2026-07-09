package variants

import (
	"testing"

	"github.com/tmc/mlx-c/internal/mlxcgen/plan"
)

func testPolicy() variantPolicy {
	return variantPolicy{
		mappings: map[string]map[string]map[string]variantRule{
			"mlx_core": {
				"squeeze": {
					"array(array, std::vector<int>, StreamOrDevice)": {suffix: testSuffix("axes"), index: 0},
					"array(array, int, StreamOrDevice)":              {suffix: testSuffix("axis"), index: 1},
					"array(array, StreamOrDevice)":                   {suffix: testSuffix(""), index: 2},
				},
			},
			"mlx_core_metal": {
				"device_info": {
					"std::unordered_map<std::string, std::variant<std::string, size_t>>()": {reason: "unsupported_type"},
				},
			},
		},
		allowedDetailFuncs: map[string]bool{
			"compile": true,
		},
	}
}

func testSuffix(s string) VariantSuffix {
	return &s
}

func TestSelectVariantsUsesManifestSuffixes(t *testing.T) {
	defs := []*Func{
		{Name: "squeeze", Namespace: "mlx::core", ReturnType: "array", ParamTypes: []string{"array", "std::vector<int>", "StreamOrDevice"}},
		{Name: "squeeze", Namespace: "mlx::core", ReturnType: "array", ParamTypes: []string{"array", "int", "StreamOrDevice"}},
		{Name: "squeeze", Namespace: "mlx::core", ReturnType: "array", ParamTypes: []string{"array", "StreamOrDevice"}},
	}
	got, diagnostics, err := selectVariantsWithPolicy(testPolicy(), "mlx::core", "squeeze", defs)
	if err != nil {
		t.Fatalf("SelectVariantsWithDiagnostics: %v", err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v, want none", diagnostics)
	}
	if len(got) != 3 {
		t.Fatalf("SelectVariants returned %d definitions, want 3", len(got))
	}
	for i, want := range []string{"axes", "axis", ""} {
		if got[i] != defs[i] {
			t.Fatalf("selected[%d] = %#v, want original definition", i, got[i])
		}
		if got[i].Variant != want {
			t.Fatalf("selected[%d].Variant = %q, want %q", i, got[i].Variant, want)
		}
		if got[i].VariantIndex != i {
			t.Fatalf("selected[%d].VariantIndex = %d, want %d", i, got[i].VariantIndex, i)
		}
	}
}

func TestSelectorUsesExplicitManifest(t *testing.T) {
	suffix := ""
	doc := "Return squeeze without singleton dimensions."
	selector := NewSelector(plan.Manifest{
		SchemaVersion: plan.SchemaVersion,
		MLX:           plan.MLXPolicy{ExpectedGitRef: "v0.31.2"},
		Headers:       []plan.HeaderMapping{{Name: "ops", Headers: []string{"mlx/ops.h"}}},
		Standalone:    []string{"vector"},
		VariantMappings: map[string]map[string][]plan.Variant{
			"mlx_core": {
				"squeeze": {
					{Signature: "array(array, StreamOrDevice)", Suffix: &suffix, Doc: doc},
				},
			},
		},
	})
	defs := []*Func{{
		Name:       "squeeze",
		Namespace:  "mlx::core",
		ReturnType: "array",
		ParamTypes: []string{"array", "StreamOrDevice"},
	}}
	got, diagnostics, err := selector.SelectWithDiagnostics("mlx::core", "squeeze", defs)
	if err != nil {
		t.Fatalf("SelectWithDiagnostics: %v", err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v, want none", diagnostics)
	}
	if len(got) != 1 || got[0] != defs[0] {
		t.Fatalf("selected = %#v, want original definition", got)
	}
	if got[0].Doc != doc {
		t.Fatalf("selected doc = %q, want manifest override", got[0].Doc)
	}
}

func TestSelectVariantsSkipsMetalDeviceInfo(t *testing.T) {
	defs := []*Func{{
		Name:       "device_info",
		Namespace:  "mlx::core::metal",
		ReturnType: "std::unordered_map<std::string, std::variant<std::string, size_t>>",
	}}
	got, diagnostics, err := selectVariantsWithPolicy(testPolicy(), "mlx::core::metal", "device_info", defs)
	if err != nil {
		t.Fatalf("SelectVariantsWithDiagnostics: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("SelectVariants returned %d definitions, want 0", len(got))
	}
	if len(diagnostics) != 1 ||
		diagnostics[0].Code != "skip_variant_mapping" ||
		diagnostics[0].Reason != "unsupported_type" ||
		diagnostics[0].Func != defs[0] {
		t.Fatalf("diagnostics = %#v, want one skip_variant_mapping for original definition", diagnostics)
	}
}

func TestSelectVariantsKeepsCompatibilityWrapper(t *testing.T) {
	defs := []*Func{{
		Name:       "device_info",
		Namespace:  "mlx::core::metal",
		ReturnType: "std::unordered_map<std::string, std::variant<std::string, size_t>>",
	}}
	got, _, err := selectVariantsWithPolicy(testPolicy(), "mlx::core::metal", "device_info", defs)
	if err != nil {
		t.Fatalf("SelectVariants: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("SelectVariants returned %d definitions, want 0", len(got))
	}
}

func TestSelectVariantsReportsDetailSkipReason(t *testing.T) {
	defs := []*Func{{
		Name:       "helper",
		Namespace:  "mlx::core::detail",
		ReturnType: "void",
	}}
	_, diagnostics, err := selectVariantsWithPolicy(testPolicy(), "mlx::core::detail", "helper", defs)
	if err != nil {
		t.Fatalf("SelectVariantsWithDiagnostics: %v", err)
	}
	if len(diagnostics) != 1 ||
		diagnostics[0].Code != "skip_unallowed_detail_function" ||
		diagnostics[0].Reason != "internal_namespace" ||
		diagnostics[0].Func != defs[0] {
		t.Fatalf("diagnostics = %#v, want detail skip with internal_namespace reason", diagnostics)
	}
}

func TestSelectVariantsKeepsMetalSingles(t *testing.T) {
	defs := []*Func{{
		Name:       "is_available",
		Namespace:  "mlx::core::metal",
		ReturnType: "bool",
	}}
	got, diagnostics, err := selectVariantsWithPolicy(testPolicy(), "mlx::core::metal", "is_available", defs)
	if err != nil {
		t.Fatalf("SelectVariants: %v", err)
	}
	if len(got) != 1 || got[0] != defs[0] {
		t.Fatalf("SelectVariants returned %#v, want original definition", got)
	}
	if len(diagnostics) != 1 || diagnostics[0].Code != "emit_implicit_single_overload" || diagnostics[0].Func != defs[0] {
		t.Fatalf("diagnostics = %#v, want implicit single-overload diagnostic", diagnostics)
	}
}

func TestSelectVariantsAllowsManifestDetailFunction(t *testing.T) {
	defs := []*Func{{
		Name:       "compile",
		Namespace:  "mlx::core::detail",
		ReturnType: "void",
	}}
	got, diagnostics, err := selectVariantsWithPolicy(testPolicy(), "mlx::core::detail", "compile", defs)
	if err != nil {
		t.Fatalf("SelectVariantsWithDiagnostics: %v", err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v, want none", diagnostics)
	}
	if len(got) != 1 || got[0] != defs[0] {
		t.Fatalf("SelectVariants returned %#v, want original definition", got)
	}
}

func TestSelectVariantsRejectsUnmappedOverloads(t *testing.T) {
	defs := []*Func{
		{Name: "new_overload", Namespace: "mlx::core", ReturnType: "array", ParamTypes: []string{"array"}},
		{Name: "new_overload", Namespace: "mlx::core", ReturnType: "array", ParamTypes: []string{"array", "array"}},
	}
	_, _, err := selectVariantsWithPolicy(testPolicy(), "mlx::core", "new_overload", defs)
	if err == nil {
		t.Fatal("SelectVariantsWithDiagnostics succeeded for unmapped overloads")
	}
}

func TestSelectVariantsRejectsMissingMappedSignature(t *testing.T) {
	defs := []*Func{
		{Name: "squeeze", Namespace: "mlx::core", ReturnType: "array", ParamTypes: []string{"array", "std::vector<int>", "StreamOrDevice"}},
		{Name: "squeeze", Namespace: "mlx::core", ReturnType: "array", ParamTypes: []string{"array", "int", "StreamOrDevice"}},
	}
	_, _, err := selectVariantsWithPolicy(testPolicy(), "mlx::core", "squeeze", defs)
	if err == nil {
		t.Fatal("SelectVariantsWithDiagnostics succeeded with a missing mapped signature")
	}
}
