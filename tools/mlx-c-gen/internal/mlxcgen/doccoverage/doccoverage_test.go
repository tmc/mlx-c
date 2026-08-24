package doccoverage

import (
	"testing"

	"github.com/tmc/mlx-c/tools/mlx-c-gen/internal/mlxcgen/ir"
	"github.com/tmc/mlx-c/tools/mlx-c-gen/internal/mlxcgen/plan"
)

func TestAnalyzeReportsSelectedMissingDocs(t *testing.T) {
	base := ""
	axis := "axis"
	manifest := plan.Manifest{
		VariantMappings: map[string]map[string][]plan.Variant{
			"mlx_core": {
				"add": {
					{Signature: "array(array, array, StreamOrDevice)", Suffix: &base},
				},
				"sum": {
					{Signature: "array(array, int, bool, StreamOrDevice)", Suffix: &axis},
					{Signature: "array(array, StreamOrDevice)", Skip: true},
				},
				"stack": {
					{Signature: "array(std::vector<array>, StreamOrDevice)", Suffix: &base, Doc: "Stack arrays along a new axis."},
				},
				"unsupported": {
					{Signature: "Missing(array)", Suffix: &base},
				},
			},
		},
	}
	result := ir.Result{Functions: []ir.FuncDecl{
		{
			Module:    "ops",
			Header:    "mlx/ops.h",
			Namespace: "mlx::core",
			Name:      "stack",
			Return:    "array",
			Params: []ir.Param{
				{Type: "std::vector<array>"},
				{Type: "StreamOrDevice"},
			},
			Loc: ir.SourceLoc{File: "mlx/ops.h", Line: 35, Col: 1},
		},
		{
			Module:    "ops",
			Header:    "mlx/ops.h",
			Namespace: "mlx::core",
			Name:      "add",
			Return:    "array",
			Params: []ir.Param{
				{Type: "array"},
				{Type: "array"},
				{Type: "StreamOrDevice"},
			},
			Comment: "Add two arrays.",
			Loc:     ir.SourceLoc{File: "mlx/ops.h", Line: 10, Col: 1},
		},
		{
			Module:    "ops",
			Header:    "mlx/ops.h",
			Namespace: "mlx::core",
			Name:      "sum",
			Return:    "array",
			Params: []ir.Param{
				{Type: "array"},
				{Type: "int"},
				{Type: "bool"},
				{Type: "StreamOrDevice"},
			},
			Loc: ir.SourceLoc{File: "mlx/ops.h", Line: 20, Col: 1},
		},
		{
			Module:    "ops",
			Header:    "mlx/ops.h",
			Namespace: "mlx::core",
			Name:      "sum",
			Return:    "array",
			Params: []ir.Param{
				{Type: "array"},
				{Type: "StreamOrDevice"},
			},
			Loc: ir.SourceLoc{File: "mlx/ops.h", Line: 30, Col: 1},
		},
		{
			Module:    "ops",
			Header:    "mlx/ops.h",
			Namespace: "mlx::core",
			Name:      "unsupported",
			Return:    "Missing",
			Params:    []ir.Param{{Type: "array"}},
			Loc:       ir.SourceLoc{File: "mlx/ops.h", Line: 40, Col: 1},
		},
	}}

	coverage, missing := Analyze(manifest, result)
	if coverage.Exported != 3 || coverage.WithDoc != 2 || coverage.Missing != 1 {
		t.Fatalf("coverage = %#v, want exported=3 with_doc=2 missing=1", coverage)
	}
	if len(missing) != 1 {
		t.Fatalf("missing = %#v, want one", missing)
	}
	got := missing[0]
	if got.CName != "mlx_sum_axis" ||
		got.Action != "emit" ||
		got.Signature != "array(array, int, bool, StreamOrDevice)" ||
		got.File != "mlx/ops.h" ||
		got.Line != 20 {
		t.Fatalf("missing doc = %#v", got)
	}
}

func TestAnalyzeReportsHookDocs(t *testing.T) {
	base := ""
	manifest := plan.Manifest{
		VariantMappings: map[string]map[string][]plan.Variant{
			"mlx_core_fast": {
				"metal_kernel": {
					{Signature: "CustomKernelFunction(std::string, std::vector<std::string>, std::vector<std::string>, std::string, std::string, bool, bool)", Suffix: &base},
				},
			},
		},
	}
	result := ir.Result{Functions: []ir.FuncDecl{{
		Module:    "fast",
		Header:    "mlx/fast.h",
		Namespace: "mlx::core::fast",
		Name:      "metal_kernel",
		Return:    "CustomKernelFunction",
		Params: []ir.Param{
			{Type: "std::string"},
			{Type: "std::vector<std::string>"},
			{Type: "std::vector<std::string>"},
			{Type: "std::string"},
			{Type: "std::string"},
			{Type: "bool"},
			{Type: "bool"},
		},
		Loc: ir.SourceLoc{File: "mlx/fast.h", Line: 7, Col: 1},
	}}}

	coverage, missing := Analyze(manifest, result)
	if coverage.Exported != 1 || coverage.Missing != 1 {
		t.Fatalf("coverage = %#v, want hook counted even with unsupported types", coverage)
	}
	if len(missing) != 1 || missing[0].Action != "hook" || missing[0].CName != "mlx_fast_metal_kernel" {
		t.Fatalf("missing = %#v, want hook missing doc", missing)
	}
}
