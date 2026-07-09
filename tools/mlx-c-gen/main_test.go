package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tmc/mlx-c/internal/mlxcgen/apilock"
	"github.com/tmc/mlx-c/internal/mlxcgen/customspec"
	"github.com/tmc/mlx-c/internal/mlxcgen/doccoverage"
	"github.com/tmc/mlx-c/internal/mlxcgen/generators"
	"github.com/tmc/mlx-c/internal/mlxcgen/ir"
	"github.com/tmc/mlx-c/internal/mlxcgen/parser"
	"github.com/tmc/mlx-c/internal/mlxcgen/plan"
	"github.com/tmc/mlx-c/internal/mlxcgen/regenreport"
	"github.com/tmc/mlx-c/internal/mlxcgen/symbols"
	"github.com/tmc/mlx-c/internal/mlxcgen/types"
)

func TestPrepareOutputDirCreatesPrivateDir(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "mlx", "c")
	if err := prepareOutputDir(outDir, false); err != nil {
		t.Fatalf("prepareOutputDir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "private")); err != nil {
		t.Fatalf("stat private dir: %v", err)
	}
}

func TestPrepareOutputDirDryRunDoesNotCreate(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "mlx", "c")
	if err := prepareOutputDir(outDir, true); err != nil {
		t.Fatalf("prepareOutputDir dry run: %v", err)
	}
	if _, err := os.Stat(outDir); !os.IsNotExist(err) {
		t.Fatalf("stat output dir after dry run: %v, want not exist", err)
	}
}

func TestPrepareOutputDirRejectsEmptyOutputDir(t *testing.T) {
	if err := prepareOutputDir("", false); err == nil || !strings.Contains(err.Error(), "missing output directory") {
		t.Fatalf("prepareOutputDir empty = %v, want missing output directory", err)
	}
}

func TestPrepareOutputDirReportsCreateError(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "out")
	if err := os.WriteFile(outDir, []byte("not a directory"), 0o666); err != nil {
		t.Fatalf("write output path file: %v", err)
	}
	err := prepareOutputDir(outDir, false)
	if err == nil || !strings.Contains(err.Error(), "create output directory") {
		t.Fatalf("prepareOutputDir file path = %v, want create output directory error", err)
	}
}

func TestParseCheckOptions(t *testing.T) {
	opts, err := parseCheckOptions([]string{
		"--root", "/repo",
		"--mlx-src", "/mlx",
		"--manifest", "/repo/codegen/manifest.yaml",
		"--custom-dir", "/repo/codegen/custom",
		"--types", "/repo/codegen/types.yaml",
		"--compile-commands", "/build/compile_commands.json",
		"--inventory", "/repo/codegen/generated-files.txt",
		"--lock", "/repo/codegen/mlxc-capi.lock.json",
		"--lock-tu", "/repo/codegen/lock.c",
		"--report", "/tmp/report.json",
		"--work-dir", "/tmp/work",
		"--ast-cache", "/tmp/cache",
		"--generator", "go run ./tools/mlx-c-gen",
		"--nm", "llvm-nm",
		"--symbol", "mlxc=/tmp/libmlxc.dylib",
		"--no-format",
		"--keep-work",
		"--strict-generated",
		"--strict-docs",
	})
	if err != nil {
		t.Fatalf("parseCheckOptions: %v", err)
	}
	got := opts.Options
	if got.RepoRoot != "/repo" ||
		got.MLXSrc != "/mlx" ||
		got.ManifestPath != "/repo/codegen/manifest.yaml" ||
		got.CustomDir != "/repo/codegen/custom" ||
		got.TypePolicyPath != "/repo/codegen/types.yaml" ||
		got.CompileCommandsPath != "/build/compile_commands.json" ||
		got.InventoryPath != "/repo/codegen/generated-files.txt" ||
		opts.LockPath != "/repo/codegen/mlxc-capi.lock.json" ||
		opts.LockTUPath != "/repo/codegen/lock.c" ||
		opts.ReportPath != "/tmp/report.json" ||
		opts.NM != "llvm-nm" ||
		got.WorkDir != "/tmp/work" ||
		got.ASTCacheDir != "/tmp/cache" ||
		got.NoASTCache ||
		got.NoFormatCache ||
		!got.NoFormat ||
		!got.KeepWork ||
		!opts.StrictGenerated ||
		!opts.StrictDocs {
		t.Fatalf("options = %#v check = %#v", got, opts)
	}
	wantGenerator := []string{"go", "run", "./tools/mlx-c-gen"}
	if !reflect.DeepEqual(got.Generator, wantGenerator) {
		t.Fatalf("generator = %#v, want %#v", got.Generator, wantGenerator)
	}
	if len(opts.Symbols) != 1 || opts.Symbols[0].Target != "mlxc" || opts.Symbols[0].Path != "/tmp/libmlxc.dylib" {
		t.Fatalf("symbols = %#v", opts.Symbols)
	}
}

func TestParseCheckOptionsFormatCache(t *testing.T) {
	opts, err := parseCheckOptions([]string{"--format-cache", "/tmp/format-cache"})
	if err != nil {
		t.Fatalf("parseCheckOptions format cache: %v", err)
	}
	if opts.Options.FormatCacheDir != "/tmp/format-cache" || opts.Options.NoFormatCache {
		t.Fatalf("cache options = %#v", opts.Options)
	}
}

func TestReportSymbolChecks(t *testing.T) {
	got := reportSymbolChecks([]symbols.Result{{
		Target:          "mlxc",
		Path:            "/tmp/libmlxc.dylib",
		Source:          "library",
		LockedFunctions: 3,
		DefinedSymbols:  10,
		PublicSymbols:   3,
		Problems:        []string{"mlxc: missing mlx_add"},
	}})
	if len(got) != 1 ||
		got[0].Target != "mlxc" ||
		got[0].Path != "/tmp/libmlxc.dylib" ||
		got[0].Source != "library" ||
		got[0].LockedFunctions != 3 ||
		got[0].DefinedSymbols != 10 ||
		got[0].PublicSymbols != 3 ||
		len(got[0].Problems) != 1 {
		t.Fatalf("symbol checks = %#v", got)
	}
}

func TestParseOptionsFromArgs(t *testing.T) {
	opts, err := parseOptionsFromArgs([]string{
		"--root", "/repo",
		"--mlx-src", "/mlx",
		"--manifest", "/repo/codegen/manifest.yaml",
		"--custom-dir", "/repo/codegen/custom",
		"--types", "/repo/codegen/types.yaml",
		"--compile-commands", "/build/compile_commands.json",
		"--inventory", "/repo/codegen/generated-files.txt",
		"--ast-cache", "/tmp/cache",
		"--out", "/tmp/ir.json",
		"--report", "/tmp/report.json",
	})
	if err != nil {
		t.Fatalf("parseOptionsFromArgs: %v", err)
	}
	if opts.RepoRoot != "/repo" ||
		opts.MLXSrc != "/mlx" ||
		opts.ManifestPath != "/repo/codegen/manifest.yaml" ||
		opts.CustomDir != "/repo/codegen/custom" ||
		opts.TypePolicyPath != "/repo/codegen/types.yaml" ||
		opts.CompileCommandsPath != "/build/compile_commands.json" ||
		opts.InventoryPath != "/repo/codegen/generated-files.txt" ||
		opts.ASTCacheDir != "/tmp/cache" ||
		opts.NoASTCache ||
		opts.OutPath != "/tmp/ir.json" ||
		opts.ReportPath != "/tmp/report.json" {
		t.Fatalf("options = %#v", opts)
	}
}

func TestParseOptionsFromArgsDefaults(t *testing.T) {
	t.Setenv("MLX_C_AST_CACHE", "/tmp/mlx-c-default-cache")
	opts, err := parseOptionsFromArgs(nil)
	if err != nil {
		t.Fatalf("parseOptionsFromArgs defaults: %v", err)
	}
	if opts.RepoRoot != "." ||
		opts.MLXSrc != "" ||
		opts.TypePolicyPath != filepath.Join(".", "codegen", "types.yaml") ||
		opts.InventoryPath != "codegen/generated-files.txt" ||
		opts.ASTCacheDir != "/tmp/mlx-c-default-cache" ||
		opts.NoASTCache ||
		opts.OutPath != "-" ||
		opts.ReportPath != "" {
		t.Fatalf("defaults = %#v", opts)
	}
}

func TestParseOptionsFromArgsNoASTCache(t *testing.T) {
	t.Setenv("MLX_C_AST_CACHE", "/tmp/mlx-c-default-cache")
	opts, err := parseOptionsFromArgs([]string{"--no-ast-cache"})
	if err != nil {
		t.Fatalf("parseOptionsFromArgs no cache: %v", err)
	}
	if opts.ASTCacheDir != "" || !opts.NoASTCache {
		t.Fatalf("cache options = %#v", opts)
	}
}

func TestNormalizedParseCommandArgs(t *testing.T) {
	got := normalizedParseCommandArgs([]string{
		"--manifest=codegen/manifest.yaml",
		"--out",
		"/tmp/ir1.json",
		"--report=/tmp/report1.json",
		"--mlx-src",
		"/repo/mlx",
	})
	want := []string{
		"--manifest=codegen/manifest.yaml",
		"--out",
		"<path>",
		"--report=<path>",
		"--mlx-src",
		"/repo/mlx",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalized args = %#v, want %#v", got, want)
	}
}

func TestGenerateOutputDir(t *testing.T) {
	tests := []struct {
		name       string
		outputDir  string
		outputRoot string
		want       string
	}{
		{name: "default", outputRoot: ".", want: filepath.Join("mlx", "c")},
		{name: "root", outputRoot: "/repo", want: filepath.Join("/repo", "mlx", "c")},
		{name: "empty root", want: filepath.Join("mlx", "c")},
		{name: "explicit dir", outputDir: "/tmp/out", outputRoot: "/repo", want: "/tmp/out"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := generateOutputDir(tt.outputDir, tt.outputRoot); got != tt.want {
				t.Fatalf("generateOutputDir = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizedGenerateCommandArgs(t *testing.T) {
	got := normalizedGenerateCommandArgs([]string{
		"--manifest=codegen/manifest.yaml",
		"--output-root",
		".",
		"--format-cache=/tmp/format-cache",
		"--report=/tmp/report.json",
	})
	want := []string{
		"--manifest=codegen/manifest.yaml",
		"--output-root",
		".",
		"--format-cache=<path>",
		"--report=<path>",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalized args = %#v, want %#v", got, want)
	}
}

func TestNewGenerateReport(t *testing.T) {
	report := newGenerateReport(generateReportOptions{
		Args:           []string{"--output-root", ".", "--report", "/tmp/report.json"},
		OutputRoot:     ".",
		OutputDir:      filepath.Join("mlx", "c"),
		MLXSrc:         "/missing/mlx",
		ManifestPath:   "codegen/manifest.yaml",
		FormatCacheDir: "/tmp/format-cache",
		Manifest: plan.Manifest{
			SchemaVersion: plan.SchemaVersion,
			Headers: []plan.HeaderMapping{{
				Name:    "ops",
				Headers: []string{"mlx/ops.h"},
			}},
			Standalone: []string{"vector"},
		},
		CustomSpecs: []customspec.Spec{{
			Name:     "jaccl",
			Header:   "mlx/c/jaccl.h",
			Generate: customspec.GenerateSpec{Header: true},
		}},
		NoFormat: true,
	})
	if report.SchemaVersion != 1 ||
		report.OutputRoot != "." ||
		report.OutputDir != filepath.Join("mlx", "c") ||
		report.FormatCacheDir != "<path>" ||
		len(report.Modules) != 1 ||
		len(report.GeneratedFiles) != 6 ||
		report.Summary.HeaderModules != 1 ||
		report.Summary.Standalone != 1 ||
		!report.Summary.NoFormat {
		t.Fatalf("report = %#v", report)
	}
	wantCommand := []string{"mlx-c-gen", "generate", "--output-root", ".", "--report", "<path>"}
	if !reflect.DeepEqual(report.Command, wantCommand) {
		t.Fatalf("command = %#v, want %#v", report.Command, wantCommand)
	}
}

func TestCustomHeaderOutputPath(t *testing.T) {
	got, err := customHeaderOutputPath(filepath.Join("/tmp", "out"), "mlx/c/jaccl.h")
	if err != nil {
		t.Fatalf("customHeaderOutputPath: %v", err)
	}
	if got != filepath.Join("/tmp", "out", "jaccl.h") {
		t.Fatalf("path = %q, want jaccl header under output dir", got)
	}
	if _, err := customHeaderOutputPath("/tmp/out", "include/jaccl.h"); err == nil {
		t.Fatal("customHeaderOutputPath accepted header outside mlx/c")
	}
	if _, err := customHeaderOutputPath("/tmp/out", "mlx/c/../jaccl.h"); err == nil {
		t.Fatal("customHeaderOutputPath accepted escaping header")
	}
}

func TestNewGenerateReportNormalizesVolatilePaths(t *testing.T) {
	report := newGenerateReport(generateReportOptions{
		Args: []string{
			"--output-root",
			"/tmp/root",
			"--metadata=/tmp/metadata.yaml",
			"--report",
			"/tmp/report.json",
		},
		OutputRoot:   "/tmp/root",
		OutputDir:    "/tmp/root/mlx/c",
		MetadataPath: "/tmp/metadata.yaml",
		MLXSrc:       "/missing/mlx",
		Manifest: plan.Manifest{
			SchemaVersion: plan.SchemaVersion,
			Headers: []plan.HeaderMapping{{
				Name:    "ops",
				Headers: []string{"mlx/ops.h"},
			}},
			Standalone: []string{"vector"},
		},
	})
	if report.OutputRoot != "<path>" ||
		report.OutputDir != "<path>" ||
		report.MetadataPath != "<path>" {
		t.Fatalf("paths = root %q dir %q metadata %q", report.OutputRoot, report.OutputDir, report.MetadataPath)
	}
	wantCommand := []string{
		"mlx-c-gen",
		"generate",
		"--output-root",
		"<path>",
		"--metadata=<path>",
		"--report",
		"<path>",
	}
	if !reflect.DeepEqual(report.Command, wantCommand) {
		t.Fatalf("command = %#v, want %#v", report.Command, wantCommand)
	}
}

func TestParseVariantDecisions(t *testing.T) {
	base := ""
	axis := "axis"
	manifest := plan.Manifest{
		VariantMappings: map[string]map[string][]plan.Variant{
			"mlx_core": {
				"export_to_dot": {
					{Signature: "void(std::ostream&, NodeNamer, std::vector<array>)", Suffix: &base},
				},
				"sum": {
					{Signature: "array(array, int, bool, StreamOrDevice)", Suffix: &axis},
					{Signature: "array(array, bool, StreamOrDevice)", Suffix: &base},
					{Signature: "array(array, StreamOrDevice)", Skip: true},
				},
			},
			"mlx_core_fft": {
				"fftn": {
					{Signature: "array(array, std::vector<int>, StreamOrDevice)", Suffix: &base},
				},
			},
		},
	}
	parsed := ir.Result{Functions: []ir.FuncDecl{
		{
			ID:        "graph_utils|mlx/graph_utils.h|mlx::core|export_to_dot|void(std::ostream&, NodeNamer, std::vector<array>)",
			Namespace: "mlx::core",
			Name:      "export_to_dot",
			Return:    "void",
			Params: []ir.Param{
				{Type: "std::ostream&"},
				{Type: "NodeNamer"},
				{Type: "std::vector<array>"},
			},
		},
		{
			ID:        "ops|mlx/ops.h|mlx::core|sum|array(array, int, bool, StreamOrDevice)",
			Namespace: "mlx::core",
			Name:      "sum",
			Return:    "array",
			Params: []ir.Param{
				{Type: "array"},
				{Type: "int"},
				{Type: "bool"},
				{Type: "StreamOrDevice"},
			},
		},
		{
			ID:        "ops|mlx/ops.h|mlx::core|sum|array(array, bool, StreamOrDevice)",
			Namespace: "mlx::core",
			Name:      "sum",
			Return:    "array",
			Params: []ir.Param{
				{Type: "array"},
				{Type: "bool"},
				{Type: "StreamOrDevice"},
			},
		},
		{
			ID:        "ops|mlx/ops.h|mlx::core|sum|array(array, StreamOrDevice)",
			Namespace: "mlx::core",
			Name:      "sum",
			Return:    "array",
			Params: []ir.Param{
				{Type: "array"},
				{Type: "StreamOrDevice"},
			},
		},
		{
			ID:        "fft|mlx/fft.h|mlx::core::fft|fftn|array(array, std::vector<int>, StreamOrDevice)",
			Namespace: "mlx::core::fft",
			Name:      "fftn",
			Return:    "array",
			Params: []ir.Param{
				{Type: "array"},
				{Type: "std::vector<int>"},
				{Type: "StreamOrDevice"},
			},
		},
	}}
	decisions, summary := parseVariantDecisions(manifest, parsed)
	want := []parseDecision{
		{
			Source:    "variant_mapping",
			DeclID:    "graph_utils|mlx/graph_utils.h|mlx::core|export_to_dot|void(std::ostream&, NodeNamer, std::vector<array>)",
			Namespace: "mlx_core",
			Function:  "export_to_dot",
			Signature: "void(std::ostream&, NodeNamer, std::vector<array>)",
			Action:    "hook",
			CName:     "mlx_export_to_dot",
			Reason:    "custom_hook",
		},
		{
			Source:    "variant_mapping",
			DeclID:    "ops|mlx/ops.h|mlx::core|sum|array(array, int, bool, StreamOrDevice)",
			Namespace: "mlx_core",
			Function:  "sum",
			Signature: "array(array, int, bool, StreamOrDevice)",
			Action:    "emit",
			CName:     "mlx_sum_axis",
			Suffix:    "axis",
		},
		{
			Source:    "variant_mapping",
			DeclID:    "ops|mlx/ops.h|mlx::core|sum|array(array, bool, StreamOrDevice)",
			Namespace: "mlx_core",
			Function:  "sum",
			Signature: "array(array, bool, StreamOrDevice)",
			Action:    "emit",
			CName:     "mlx_sum",
		},
		{
			Source:    "variant_mapping",
			DeclID:    "ops|mlx/ops.h|mlx::core|sum|array(array, StreamOrDevice)",
			Namespace: "mlx_core",
			Function:  "sum",
			Signature: "array(array, StreamOrDevice)",
			Action:    "skip",
			Reason:    "variant_mapping",
		},
		{
			Source:    "variant_mapping",
			DeclID:    "fft|mlx/fft.h|mlx::core::fft|fftn|array(array, std::vector<int>, StreamOrDevice)",
			Namespace: "mlx_core_fft",
			Function:  "fftn",
			Signature: "array(array, std::vector<int>, StreamOrDevice)",
			Action:    "emit",
			CName:     "mlx_fft_fftn",
		},
		{
			Source:   "hook_registry",
			Function: "fast_cuda_kernel",
			Action:   "hook",
			CName:    "mlx_fast_cuda_kernel",
			Reason:   "custom_hook_unmatched",
		},
		{
			Source:   "hook_registry",
			Function: "fast_metal_kernel",
			Action:   "hook",
			CName:    "mlx_fast_metal_kernel",
			Reason:   "custom_hook_unmatched",
		},
		{
			Source:   "hook_registry",
			Function: "load_gguf",
			Action:   "hook",
			CName:    "mlx_load_gguf",
			Reason:   "custom_hook_unmatched",
		},
		{
			Source:   "hook_registry",
			Function: "print_graph",
			Action:   "hook",
			CName:    "mlx_print_graph",
			Reason:   "custom_hook_unmatched",
		},
		{
			Source:   "hook_registry",
			Function: "save_gguf",
			Action:   "hook",
			CName:    "mlx_save_gguf",
			Reason:   "custom_hook_unmatched",
		},
	}
	if !reflect.DeepEqual(decisions, want) {
		t.Fatalf("decisions = %#v, want %#v", decisions, want)
	}
	if summary.Emits != 3 || summary.Hooks != 6 || summary.Skips != 1 {
		t.Fatalf("summary = %#v, want 3 emits, 6 hooks, and 1 skip", summary)
	}
}

func TestParseVariantDecisionsUsesCustomHooks(t *testing.T) {
	decisions, summary := parseVariantDecisions(hookManifest(), ir.Result{})
	if summary.Hooks != 6 {
		t.Fatalf("summary = %#v, want 6 hooks", summary)
	}
	for _, decision := range decisions {
		if decision.Reason == "custom_hook_unmatched" {
			t.Fatalf("unexpected unmatched hook decision: %#v", decision)
		}
	}
	var found bool
	for _, decision := range decisions {
		if decision.Source == "custom_hook" && decision.CName == "mlx_load_gguf" {
			found = true
			if decision.Reason != "custom GGUF loading API" {
				t.Fatalf("custom hook decision = %#v", decision)
			}
		}
	}
	if !found {
		t.Fatalf("decisions = %#v, missing mlx_load_gguf custom hook", decisions)
	}
}

func TestParseInventoryDecisions(t *testing.T) {
	root := t.TempDir()
	inventoryPath := filepath.Join(root, "codegen", "generated-files.txt")
	if err := os.MkdirAll(filepath.Dir(inventoryPath), 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inventoryPath, []byte(`
generated_header_api mlxc mlx/c/ops.h
handwritten_runtime jacclc mlx/c/jaccl.cpp
custom_spec_generated mlxc mlx/c/array.h
not_owned_by_codegen mlxc mlx/c/mlx.h
generated_support mlxc mlx/c/vector.h
`), 0o666); err != nil {
		t.Fatal(err)
	}
	decisions, summary, err := parseInventoryDecisions(parseOptions{
		RepoRoot:      root,
		InventoryPath: filepath.Join("codegen", "generated-files.txt"),
	}, plan.Manifest{})
	if err != nil {
		t.Fatalf("parseInventoryDecisions: %v", err)
	}
	want := []parseFileDecision{
		{Source: "inventory", Target: "mlxc", Path: "mlx/c/array.h", Action: "custom_spec", Reason: "custom_spec_generated"},
		{Source: "inventory", Target: "jacclc", Path: "mlx/c/jaccl.cpp", Action: "handwritten", Reason: "handwritten_runtime"},
		{Source: "inventory", Target: "mlxc", Path: "mlx/c/mlx.h", Action: "not_owned", Reason: "not_owned_by_codegen"},
		{Source: "inventory", Target: "mlxc", Path: "mlx/c/ops.h", Action: "emit", Reason: "generated_header_api"},
		{Source: "inventory", Target: "mlxc", Path: "mlx/c/vector.h", Action: "emit", Reason: "generated_support"},
	}
	if !reflect.DeepEqual(decisions, want) {
		t.Fatalf("decisions = %#v, want %#v", decisions, want)
	}
	if summary.Handwritten != 1 || summary.CustomSpecs != 1 || summary.NotOwned != 1 {
		t.Fatalf("summary = %#v, want one handwritten, custom spec, and not owned", summary)
	}
}

func TestParseInventoryDecisionsChecksIncludedInventory(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "CMakeLists.txt", "")
	writeTestFile(t, root, "mlx/c/mlx.h", `#include "mlx/c/not_planned.h"`)
	writeTestFile(t, root, "mlx/c/not_planned.h", "")
	writeTestFile(t, root, "codegen/generated-files.txt", `
not_owned_by_codegen mlxc mlx/c/mlx.h
generated_header_api mlxc mlx/c/not_planned.h
`)
	_, _, err := parseInventoryDecisions(parseOptions{
		RepoRoot:      root,
		InventoryPath: filepath.Join("codegen", "generated-files.txt"),
	}, plan.Manifest{Report: plan.ReportPolicy{IncludeInventory: true}})
	if err == nil || !strings.Contains(err.Error(), "not generated by the Go plan") {
		t.Fatalf("parseInventoryDecisions error = %v, want plan inventory error", err)
	}
}

func TestParseInventoryDecisionsChecksCustomSpecGeneratedHeaders(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "CMakeLists.txt", "")
	writeTestFile(t, root, "mlx/c/mlx.h", "")
	writeTestFile(t, root, "codegen/generated-files.txt", "not_owned_by_codegen mlxc mlx/c/mlx.h\n")
	writeTestFile(t, root, "codegen/custom/sample.yaml", `
schema_version: 1
name: sample
target: mlxc
header: mlx/c/sample.h
ownership: custom_spec_generated
generate:
  header: true
copyright: Copyright © 2026 Apple Inc.
include_guard: MLX_SAMPLE_H
group:
  name: mlx_sample
  title: Sample
  doc: Sample declarations.
items:
  - kind: struct
    name: mlx_sample
    action: custom_spec
    reason: runtime_handle
    doc: Sample handle.
    opaque: true
`)
	_, _, err := parseInventoryDecisions(parseOptions{
		RepoRoot:      root,
		CustomDir:     filepath.Join("codegen", "custom"),
		InventoryPath: filepath.Join("codegen", "generated-files.txt"),
	}, plan.Manifest{Report: plan.ReportPolicy{IncludeInventory: true}})
	if err == nil || !strings.Contains(err.Error(), "mlx/c/sample.h is generated by plan but missing from inventory") {
		t.Fatalf("parseInventoryDecisions error = %v, want missing custom spec inventory error", err)
	}
}

func writeTestFile(t *testing.T, root, name, data string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o666); err != nil {
		t.Fatal(err)
	}
}

func TestParseCheckOptionsDefaults(t *testing.T) {
	t.Setenv("MLX_C_AST_CACHE", "/tmp/mlx-c-default-cache")
	t.Setenv("MLX_C_FORMAT_CACHE", "/tmp/mlx-c-format-cache")
	opts, err := parseCheckOptions(nil)
	if err != nil {
		t.Fatalf("parseCheckOptions defaults: %v", err)
	}
	got := opts.Options
	if got.RepoRoot != "." ||
		got.InventoryPath != "codegen/generated-files.txt" ||
		got.MLXSrc != "" ||
		got.ASTCacheDir != "/tmp/mlx-c-default-cache" ||
		got.FormatCacheDir != "/tmp/mlx-c-format-cache" ||
		got.NoASTCache ||
		got.NoFormatCache ||
		got.NoFormat ||
		got.KeepWork ||
		opts.LockPath != "codegen/mlxc-capi.lock.json" ||
		opts.LockTUPath != "codegen/lock.c" ||
		opts.ReportPath != "" ||
		opts.NM != "nm" ||
		opts.StrictGenerated {
		t.Fatalf("defaults = %#v check = %#v", got, opts)
	}
	wantGenerator := strings.Fields(defaultGeneratorCommand())
	if !reflect.DeepEqual(got.Generator, wantGenerator) {
		t.Fatalf("generator = %#v, want %#v", got.Generator, wantGenerator)
	}
}

func TestParseCheckOptionsSynthesisAliases(t *testing.T) {
	t.Setenv("MLX_C_AST_CACHE", "/tmp/mlx-c-default-cache")
	opts, err := parseCheckOptions([]string{
		"--output-root", "/repo",
		"--generated-files", "/repo/codegen/generated-files.txt",
		"--manifest", "/repo/codegen/manifest.yaml",
		"--custom-dir", "/repo/codegen/custom",
		"--report", "/repo/build/report.json",
	})
	if err != nil {
		t.Fatalf("parseCheckOptions aliases: %v", err)
	}
	if opts.Options.RepoRoot != "/repo" ||
		opts.Options.InventoryPath != "/repo/codegen/generated-files.txt" ||
		opts.Options.ManifestPath != "/repo/codegen/manifest.yaml" ||
		opts.Options.CustomDir != "/repo/codegen/custom" ||
		opts.ReportPath != "/repo/build/report.json" {
		t.Fatalf("options = %#v check = %#v", opts.Options, opts)
	}
}

func TestParseCheckOptionsNoASTCache(t *testing.T) {
	t.Setenv("MLX_C_AST_CACHE", "/tmp/mlx-c-default-cache")
	opts, err := parseCheckOptions([]string{"--no-ast-cache"})
	if err != nil {
		t.Fatalf("parseCheckOptions no ast cache: %v", err)
	}
	if opts.Options.ASTCacheDir != "" || !opts.Options.NoASTCache {
		t.Fatalf("cache options = %#v, want disabled", opts.Options)
	}
}

func TestParseCheckOptionsNoFormatCache(t *testing.T) {
	t.Setenv("MLX_C_FORMAT_CACHE", "/tmp/mlx-c-format-cache")
	opts, err := parseCheckOptions([]string{"--no-format-cache"})
	if err != nil {
		t.Fatalf("parseCheckOptions no format cache: %v", err)
	}
	if opts.Options.FormatCacheDir != "" || !opts.Options.NoFormatCache {
		t.Fatalf("cache options = %#v, want disabled", opts.Options)
	}
}

func TestCheckDocCoverageUsesManifestPolicy(t *testing.T) {
	report := &regenreport.Report{
		DocCoverage: doccoverage.Coverage{Missing: 1},
	}
	if err := checkDocCoverage(report, false); err != nil {
		t.Fatalf("checkDocCoverage without policy = %v, want nil", err)
	}
	report.Manifest.Report.RequireDocCoverage = true
	err := checkDocCoverage(report, false)
	if err == nil || !strings.Contains(err.Error(), "missing doc source") {
		t.Fatalf("checkDocCoverage with manifest policy = %v, want missing docs error", err)
	}
	report.DocCoverage.Missing = 0
	if err := checkDocCoverage(report, false); err != nil {
		t.Fatalf("checkDocCoverage clean = %v, want nil", err)
	}
}

func TestCheckTypeCoverageUsesManifestPolicy(t *testing.T) {
	report := &regenreport.Report{
		Diagnostics: []regenreport.Diagnostic{{
			Code: "skip_unsupported_return_type",
		}},
	}
	if err := checkTypeCoverage(report); err != nil {
		t.Fatalf("checkTypeCoverage without policy = %v, want nil", err)
	}
	report.Manifest.Report.RequireTypeCoverage = true
	err := checkTypeCoverage(report)
	if err == nil || !strings.Contains(err.Error(), "unsupported types") {
		t.Fatalf("checkTypeCoverage with manifest policy = %v, want unsupported types error", err)
	}
	report.Diagnostics = nil
	report.MissingTypes = []types.MissingType{{Type: "Missing"}}
	err = checkTypeCoverage(report)
	if err == nil || !strings.Contains(err.Error(), "missing type policy entries") {
		t.Fatalf("checkTypeCoverage missing type = %v, want missing type error", err)
	}
	report.MissingTypes = nil
	if err := checkTypeCoverage(report); err != nil {
		t.Fatalf("checkTypeCoverage clean = %v, want nil", err)
	}
}

func TestCheckDiagnosticReasonsUsesManifestPolicy(t *testing.T) {
	report := &regenreport.Report{
		Diagnostics: []regenreport.Diagnostic{{
			Code: "skip_operator",
		}},
	}
	if err := checkDiagnosticReasons(report); err != nil {
		t.Fatalf("checkDiagnosticReasons without policy = %v, want nil", err)
	}
	report.Manifest.Report.RequireDiagnosticReasons = true
	err := checkDiagnosticReasons(report)
	if err == nil || !strings.Contains(err.Error(), "missing reason") {
		t.Fatalf("checkDiagnosticReasons with manifest policy = %v, want missing reason error", err)
	}
	report.Diagnostics[0].Reason = "not_c_api"
	if err := checkDiagnosticReasons(report); err != nil {
		t.Fatalf("checkDiagnosticReasons clean = %v, want nil", err)
	}
}

func TestCheckExplicitVariantsUsesManifestPolicy(t *testing.T) {
	report := &regenreport.Report{
		Diagnostics: []regenreport.Diagnostic{{
			Code: "emit_implicit_single_overload",
		}},
	}
	if err := checkExplicitVariants(report); err != nil {
		t.Fatalf("checkExplicitVariants without policy = %v, want nil", err)
	}
	report.Manifest.Report.RequireExplicitVariants = true
	err := checkExplicitVariants(report)
	if err == nil || !strings.Contains(err.Error(), "implicit variant selection") {
		t.Fatalf("checkExplicitVariants with manifest policy = %v, want implicit variant error", err)
	}
	report.Diagnostics = nil
	if err := checkExplicitVariants(report); err != nil {
		t.Fatalf("checkExplicitVariants clean = %v, want nil", err)
	}
}

func TestCheckParseReportPolicyUsesManifestPolicy(t *testing.T) {
	manifest := plan.Manifest{Report: plan.ReportPolicy{
		RequireDiagnosticReasons: true,
		RequireDocCoverage:       true,
		RequireExplicitVariants:  true,
		RequireTypeCoverage:      true,
	}}
	if err := checkParseReportPolicy(manifest, doccoverage.Coverage{}, nil, nil); err != nil {
		t.Fatalf("checkParseReportPolicy clean = %v, want nil", err)
	}
	err := checkParseReportPolicy(manifest, doccoverage.Coverage{Missing: 1}, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "missing doc source") {
		t.Fatalf("checkParseReportPolicy missing doc = %v, want missing doc error", err)
	}
	err = checkParseReportPolicy(manifest, doccoverage.Coverage{}, []types.MissingType{{Type: "Missing"}}, nil)
	if err == nil || !strings.Contains(err.Error(), "missing type policy entries") {
		t.Fatalf("checkParseReportPolicy missing type = %v, want missing type error", err)
	}
	err = checkParseReportPolicy(manifest, doccoverage.Coverage{}, nil, []parseDiagnostic{{Code: "skip_operator"}})
	if err == nil || !strings.Contains(err.Error(), "missing reason") {
		t.Fatalf("checkParseReportPolicy missing reason = %v, want missing reason error", err)
	}
	err = checkParseReportPolicy(manifest, doccoverage.Coverage{}, nil, []parseDiagnostic{{Code: "emit_implicit_single_overload"}})
	if err == nil || !strings.Contains(err.Error(), "implicit variant selection") {
		t.Fatalf("checkParseReportPolicy implicit variant = %v, want implicit variant error", err)
	}
	err = checkParseReportPolicy(manifest, doccoverage.Coverage{}, nil, []parseDiagnostic{{Code: "skip_unsupported_return_type", Reason: "unsupported_type"}})
	if err == nil || !strings.Contains(err.Error(), "unsupported types") {
		t.Fatalf("checkParseReportPolicy unsupported type = %v, want unsupported type error", err)
	}
}

func TestCheckGeneratedCleanUsesManifestPolicy(t *testing.T) {
	report := &regenreport.Report{
		Summary: regenreport.Summary{Different: 1},
	}
	if err := checkGeneratedClean(report, false); err != nil {
		t.Fatalf("checkGeneratedClean without policy = %v, want nil", err)
	}
	report.Manifest.Report.RequireCleanGenerated = true
	err := checkGeneratedClean(report, false)
	if err == nil || !strings.Contains(err.Error(), "regenerated files differ") {
		t.Fatalf("checkGeneratedClean with manifest policy = %v, want generated diff error", err)
	}
	report.Summary.Different = 0
	if err := checkGeneratedClean(report, false); err != nil {
		t.Fatalf("checkGeneratedClean clean = %v, want nil", err)
	}
}

func TestCheckGeneratedCleanUsesStrictFlag(t *testing.T) {
	report := &regenreport.Report{
		GeneratedOnly: []string{"mlx/c/extra.h"},
	}
	err := checkGeneratedClean(report, true)
	if err == nil || !strings.Contains(err.Error(), "regenerated files differ") {
		t.Fatalf("checkGeneratedClean strict = %v, want generated diff error", err)
	}
}

func TestCheckAPILockRequiresLockPathWhenManifestRequiresIt(t *testing.T) {
	report := &regenreport.Report{}
	if err := checkAPILock(checkOptions{}, report); err != nil {
		t.Fatalf("checkAPILock without policy = %v, want nil", err)
	}
	report.Manifest.Report.RequireAPILock = true
	err := checkAPILock(checkOptions{}, report)
	if err == nil || !strings.Contains(err.Error(), "requires API lock") {
		t.Fatalf("checkAPILock without lock path = %v, want required lock error", err)
	}
}

func TestCheckCustomHooksLocked(t *testing.T) {
	report := &regenreport.Report{}
	report.Manifest.CustomHooks = []plan.CustomHook{
		{CName: "mlx_fast_cuda_kernel", Reason: "custom CUDA kernel API"},
		{CName: "mlx_load_gguf", Reason: "custom GGUF loading API"},
		{CName: "mlx_jaccl_barrier", Reason: "JACCL compatibility API"},
	}
	lock := &apilock.Lock{Targets: map[string]apilock.Target{
		"mlxc": {
			Structs:   []apilock.Struct{{Name: "mlx_fast_cuda_kernel"}},
			Functions: []apilock.Function{{Name: "mlx_load_gguf"}},
		},
		"jacclc": {
			Functions: []apilock.Function{{Name: "mlx_jaccl_barrier"}},
		},
	}}
	if err := checkCustomHooksLocked(report, lock); err != nil {
		t.Fatalf("checkCustomHooksLocked complete = %v, want nil", err)
	}
	lock.Targets["mlxc"] = apilock.Target{
		Structs: []apilock.Struct{{Name: "mlx_fast_cuda_kernel"}},
	}
	err := checkCustomHooksLocked(report, lock)
	if err == nil || !strings.Contains(err.Error(), "custom hook mlx_load_gguf missing from mlxc API lock") {
		t.Fatalf("checkCustomHooksLocked missing hook = %v, want missing lock error", err)
	}
	delete(lock.Targets, "jacclc")
	err = checkCustomHooksLocked(report, lock)
	if err == nil || !strings.Contains(err.Error(), "custom hook mlx_jaccl_barrier requires missing jacclc API lock target") {
		t.Fatalf("checkCustomHooksLocked missing target = %v, want missing target error", err)
	}
}

func TestCheckHookAPILocked(t *testing.T) {
	report := &regenreport.Report{}
	report.Manifest.HookAPI = []plan.HookAPI{{
		CName: "mlx_export_to_dot",
		Names: []string{"mlx_node_namer", "mlx_export_to_dot"},
	}, {
		CName: "mlx_jaccl_barrier",
		Names: []string{"mlx_jaccl_barrier"},
	}}
	lock := &apilock.Lock{Targets: map[string]apilock.Target{
		"mlxc": {
			Structs:   []apilock.Struct{{Name: "mlx_node_namer"}},
			Functions: []apilock.Function{{Name: "mlx_export_to_dot"}},
		},
		"jacclc": {
			Functions: []apilock.Function{{Name: "mlx_jaccl_barrier"}},
		},
	}}
	if err := checkHookAPILocked(report, lock); err != nil {
		t.Fatalf("checkHookAPILocked complete = %v, want nil", err)
	}
	lock.Targets["mlxc"] = apilock.Target{
		Structs: []apilock.Struct{{Name: "mlx_node_namer"}},
	}
	err := checkHookAPILocked(report, lock)
	if err == nil || !strings.Contains(err.Error(), "hook mlx_export_to_dot API name mlx_export_to_dot missing from mlxc API lock") {
		t.Fatalf("checkHookAPILocked missing name = %v, want missing lock error", err)
	}
	delete(lock.Targets, "jacclc")
	err = checkHookAPILocked(report, lock)
	if err == nil || !strings.Contains(err.Error(), "hook mlx_jaccl_barrier API name mlx_jaccl_barrier requires missing jacclc API lock target") {
		t.Fatalf("checkHookAPILocked missing target = %v, want missing target error", err)
	}
}

func TestCheckEmittedAPILocked(t *testing.T) {
	base := ""
	manifest := plan.Manifest{
		Report: plan.ReportPolicy{RequireEmitAPILock: true},
		VariantMappings: map[string]map[string][]plan.Variant{
			"mlx_core": {
				"add": {
					{Signature: "array(array, array)", Suffix: &base},
				},
				"skip_me": {
					{Signature: "array(array)", Skip: true, Reason: "not_c_api"},
				},
			},
		},
	}
	report := &regenreport.Report{
		IR: ir.Result{Functions: []ir.FuncDecl{{
			ID:        "ops|mlx/ops.h|mlx::core|add|array(array, array)",
			Namespace: "mlx::core",
			Name:      "add",
			Return:    "array",
			Params: []ir.Param{
				{Type: "array"},
				{Type: "array"},
			},
		}}},
		TypePolicyIR: ir.Result{Functions: []ir.FuncDecl{{
			ID:        "compile|mlx/compile_impl.h|mlx::core::detail|compile_clear_cache|void()",
			Namespace: "mlx::core::detail",
			Name:      "compile_clear_cache",
			Return:    "void",
		}}},
	}
	lock := &apilock.Lock{Targets: map[string]apilock.Target{
		"mlxc": {
			Functions: []apilock.Function{
				{Name: "mlx_add"},
				{Name: "mlx_detail_compile_clear_cache"},
			},
		},
	}}
	if err := checkEmittedAPILocked(report, manifest, lock); err != nil {
		t.Fatalf("checkEmittedAPILocked complete = %v, want nil", err)
	}
	lock.Targets["mlxc"] = apilock.Target{
		Functions: []apilock.Function{{Name: "mlx_detail_compile_clear_cache"}},
	}
	err := checkEmittedAPILocked(report, manifest, lock)
	if err == nil || !strings.Contains(err.Error(), "emit mlx_add missing from mlxc API lock functions") {
		t.Fatalf("checkEmittedAPILocked missing emit = %v, want missing lock error", err)
	}
	delete(lock.Targets, "mlxc")
	err = checkEmittedAPILocked(report, manifest, lock)
	if err == nil || !strings.Contains(err.Error(), "emit mlx_add requires missing mlxc API lock target") {
		t.Fatalf("checkEmittedAPILocked missing target = %v, want missing target error", err)
	}
}

func TestCheckDocCoverageUsesStrictFlag(t *testing.T) {
	report := &regenreport.Report{
		DocCoverage: doccoverage.Coverage{Missing: 1},
	}
	err := checkDocCoverage(report, true)
	if err == nil || !strings.Contains(err.Error(), "missing doc source") {
		t.Fatalf("checkDocCoverage strict = %v, want missing docs error", err)
	}
}

func TestCheckGeneratedMarkersUsesManifestPolicy(t *testing.T) {
	report := &regenreport.Report{
		GeneratedMarkerViolations: []regenreport.GeneratedMarkerViolation{{
			Path:   "mlx/c/ops.h",
			Reason: "timestamp",
			Marker: "/* Generated at 2026-05-31T17:00:00Z */",
		}},
	}
	if err := checkGeneratedMarkers(report); err != nil {
		t.Fatalf("checkGeneratedMarkers without policy = %v, want nil", err)
	}
	report.Manifest.GeneratedMarkers.ForbidVolatileData = true
	err := checkGeneratedMarkers(report)
	if err == nil || !strings.Contains(err.Error(), "volatile data") {
		t.Fatalf("checkGeneratedMarkers with policy = %v, want volatile data error", err)
	}
	report.GeneratedMarkerViolations = nil
	if err := checkGeneratedMarkers(report); err != nil {
		t.Fatalf("checkGeneratedMarkers clean = %v, want nil", err)
	}
}

func TestCheckHookManifestPolicy(t *testing.T) {
	if err := checkHookManifestPolicy(hookManifest()); err != nil {
		t.Fatalf("checkHookManifestPolicy complete = %v, want nil", err)
	}
	manifest := hookManifest()
	manifest.CustomHooks = manifest.CustomHooks[:len(manifest.CustomHooks)-1]
	err := checkHookManifestPolicy(manifest)
	if err == nil || !strings.Contains(err.Error(), "not declared in manifest") {
		t.Fatalf("checkHookManifestPolicy missing hook = %v, want undeclared error", err)
	}
	manifest = hookManifest()
	manifest.CustomHooks = append(manifest.CustomHooks, plan.CustomHook{CName: "mlx_missing_hook"})
	err = checkHookManifestPolicy(manifest)
	if err == nil || !strings.Contains(err.Error(), "unknown custom hook") {
		t.Fatalf("checkHookManifestPolicy unknown hook = %v, want unknown hook error", err)
	}
	manifest = hookManifest()
	manifest.HookAPI = manifest.HookAPI[:len(manifest.HookAPI)-1]
	err = checkHookManifestPolicy(manifest)
	if err == nil || !strings.Contains(err.Error(), "has no hook_api entry") {
		t.Fatalf("checkHookManifestPolicy missing hook api = %v, want hook_api error", err)
	}
	manifest = hookManifest()
	manifest.HookAPI = append(manifest.HookAPI, plan.HookAPI{CName: "mlx_missing_hook", Names: []string{"mlx_missing_hook"}})
	err = checkHookManifestPolicy(manifest)
	if err == nil || !strings.Contains(err.Error(), "API for unknown hook") {
		t.Fatalf("checkHookManifestPolicy unknown hook api = %v, want unknown hook api error", err)
	}
	manifest = hookManifest()
	manifest.HookAPI[0].Names = []string{"mlx_export_to_dot"}
	err = checkHookManifestPolicy(manifest)
	if err == nil || !strings.Contains(err.Error(), "emits public API name mlx_node_namer") {
		t.Fatalf("checkHookManifestPolicy incomplete hook api = %v, want emitted name error", err)
	}
	manifest = hookManifest()
	manifest.HookAPI[0].Names = append(manifest.HookAPI[0].Names, "mlx_not_emitted")
	err = checkHookManifestPolicy(manifest)
	if err == nil || !strings.Contains(err.Error(), "hook_api name mlx_not_emitted is not emitted") {
		t.Fatalf("checkHookManifestPolicy extra hook api = %v, want not emitted error", err)
	}
}

func TestParseDetailDecisions(t *testing.T) {
	selected := ir.Result{Functions: []ir.FuncDecl{
		{
			ID:        "compile|mlx/compile_impl.h|mlx::core::detail|compile_clear_cache|void()",
			Namespace: "mlx::core::detail",
			Name:      "compile_clear_cache",
			Return:    "void",
		},
		{
			ID:        "ops|mlx/ops.h|mlx::core|sum|array(array)",
			Namespace: "mlx::core",
			Name:      "sum",
			Return:    "array",
			Params:    []ir.Param{{Type: "array"}},
		},
	}}
	decisions, summary := parseDetailDecisions(selected)
	want := []parseDecision{{
		Source:    "allowed_detail_function",
		DeclID:    "compile|mlx/compile_impl.h|mlx::core::detail|compile_clear_cache|void()",
		Namespace: "mlx_core_detail",
		Function:  "compile_clear_cache",
		Signature: "void()",
		Action:    "emit",
		CName:     "mlx_detail_compile_clear_cache",
	}}
	if !reflect.DeepEqual(decisions, want) {
		t.Fatalf("decisions = %#v, want %#v", decisions, want)
	}
	if summary.Emits != 1 || summary.Hooks != 0 || summary.Skips != 0 {
		t.Fatalf("summary = %#v, want one emit", summary)
	}
}

func TestParseDiagnosticDecisions(t *testing.T) {
	parsed := ir.Result{Functions: []ir.FuncDecl{{
		ID:        "compile|mlx/compile_impl.h|mlx::core::detail|compile_available_for_device|bool(Device)",
		Namespace: "mlx::core::detail",
		Name:      "compile_available_for_device",
		Return:    "bool",
		Params:    []ir.Param{{Type: "Device"}},
	}, {
		ID:        "compile|mlx/compile_impl.h|mlx::core::detail|compile|ArrayFnWithExtra(ArrayFnWithExtra, std::uintptr_t, bool, std::vector<uint64_t>)",
		Namespace: "mlx::core::detail",
		Name:      "compile",
		Return:    "ArrayFnWithExtra",
		Params: []ir.Param{
			{Type: "ArrayFnWithExtra"},
			{Type: "std::uintptr_t"},
			{Type: "bool"},
			{Type: "std::vector<uint64_t>"},
		},
	}}}
	diagnostics := []parseDiagnostic{
		{
			Code:   "skip_unallowed_detail_function",
			DeclID: "compile|mlx/compile_impl.h|mlx::core::detail|compile_available_for_device|bool(Device)",
			Reason: "internal_namespace",
		},
		{
			Code:   "skip_allowed_detail_overload",
			DeclID: "compile|mlx/compile_impl.h|mlx::core::detail|compile|ArrayFnWithExtra(ArrayFnWithExtra, std::uintptr_t, bool, std::vector<uint64_t>)",
			Reason: "covered_by_other_variant",
		},
		{
			Code:    "skip_operator",
			Message: "mlx::core::operator== is an operator overload",
			Reason:  "not_c_api",
			File:    "/repo/mlx/ops.h",
			Line:    405,
			Col:     14,
		},
	}

	decisions, summary, err := parseDiagnosticDecisions(parsed, diagnostics)
	if err != nil {
		t.Fatalf("parseDiagnosticDecisions: %v", err)
	}
	want := []parseDecision{
		{
			Source:    "unallowed_detail_function",
			DeclID:    "compile|mlx/compile_impl.h|mlx::core::detail|compile_available_for_device|bool(Device)",
			Namespace: "mlx_core_detail",
			Function:  "compile_available_for_device",
			Signature: "bool(Device)",
			Action:    "skip",
			Reason:    "internal_namespace",
		},
		{
			Source:    "allowed_detail_overload",
			DeclID:    "compile|mlx/compile_impl.h|mlx::core::detail|compile|ArrayFnWithExtra(ArrayFnWithExtra, std::uintptr_t, bool, std::vector<uint64_t>)",
			Namespace: "mlx_core_detail",
			Function:  "compile",
			Signature: "ArrayFnWithExtra(ArrayFnWithExtra, std::uintptr_t, bool, std::vector<uint64_t>)",
			Action:    "skip",
			Reason:    "covered_by_other_variant",
		},
		{
			Source:    "parser_diagnostic",
			Namespace: "mlx_core",
			Function:  "operator==",
			Action:    "skip",
			Reason:    "not_c_api",
			File:      "mlx/ops.h",
			Line:      405,
			Col:       14,
		},
	}
	if !reflect.DeepEqual(decisions, want) {
		t.Fatalf("decisions = %#v, want %#v", decisions, want)
	}
	if summary.Skips != 3 || summary.Emits != 0 || summary.Hooks != 0 {
		t.Fatalf("summary = %#v, want three skips", summary)
	}
}

func TestParseDiagnosticDecisionsRejectsUnqualifiedParserSkip(t *testing.T) {
	_, _, err := parseDiagnosticDecisions(ir.Result{}, []parseDiagnostic{{
		Code:    "skip_operator",
		Message: "operator overload",
		Reason:  "not_c_api",
	}})
	if err == nil || !strings.Contains(err.Error(), "missing qualified name") {
		t.Fatalf("parseDiagnosticDecisions error = %v, want missing qualified name", err)
	}
}

func TestCheckDecisionDeclIDsUsesManifestPolicy(t *testing.T) {
	decisions := []parseDecision{{
		Source:    "variant_mapping",
		Function:  "sum",
		Signature: "array(array, StreamOrDevice)",
		Action:    "emit",
	}}
	if err := checkDecisionDeclIDs(plan.Manifest{}, decisions); err != nil {
		t.Fatalf("checkDecisionDeclIDs without policy = %v, want nil", err)
	}
	manifest := plan.Manifest{
		Report: plan.ReportPolicy{RequireDecisionDeclIDs: true},
	}
	err := checkDecisionDeclIDs(manifest, decisions)
	if err == nil || !strings.Contains(err.Error(), "missing declaration id") {
		t.Fatalf("checkDecisionDeclIDs missing id = %v, want missing declaration id error", err)
	}
	decisions[0].DeclID = "ops|mlx/ops.h|mlx::core|sum|array(array, StreamOrDevice)"
	decisions = append(decisions, parseDecision{
		Source:   "parser_diagnostic",
		Function: "operator==",
		Action:   "skip",
		Reason:   "not_c_api",
	})
	if err := checkDecisionDeclIDs(manifest, decisions); err != nil {
		t.Fatalf("checkDecisionDeclIDs with id = %v, want nil", err)
	}
}

func TestCheckDecisionCoverageUsesManifestPolicy(t *testing.T) {
	parsed := ir.Result{Functions: []ir.FuncDecl{{
		ID:   "ops|mlx/ops.h|mlx::core|add|array(array, array)",
		Name: "add",
	}, {
		ID:   "ops|mlx/ops.h|mlx::core|subtract|array(array, array)",
		Name: "subtract",
	}}}
	decisions := []parseDecision{{
		DeclID: "ops|mlx/ops.h|mlx::core|add|array(array, array)",
		Action: "emit",
	}}
	if err := checkDecisionCoverage(plan.Manifest{}, parsed, decisions); err != nil {
		t.Fatalf("checkDecisionCoverage without policy = %v, want nil", err)
	}
	manifest := plan.Manifest{
		Report: plan.ReportPolicy{RequireDecisionCoverage: true},
	}
	err := checkDecisionCoverage(manifest, parsed, decisions)
	if err == nil || !strings.Contains(err.Error(), "missing decision") {
		t.Fatalf("checkDecisionCoverage missing = %v, want missing decision error", err)
	}
	decisions = append(decisions, parseDecision{
		DeclID: "ops|mlx/ops.h|mlx::core|subtract|array(array, array)",
		Action: "emit",
	})
	if err := checkDecisionCoverage(manifest, parsed, decisions); err != nil {
		t.Fatalf("checkDecisionCoverage clean = %v, want nil", err)
	}
	decisions = append(decisions, parseDecision{
		DeclID: "ops|mlx/ops.h|mlx::core|subtract|array(array, array)",
		Action: "skip",
	})
	err = checkDecisionCoverage(manifest, parsed, decisions)
	if err == nil || !strings.Contains(err.Error(), "has 2 decisions") {
		t.Fatalf("checkDecisionCoverage duplicate = %v, want duplicate decision error", err)
	}
	decisions = []parseDecision{{
		DeclID: "ops|mlx/ops.h|mlx::core|unknown|array(array)",
		Action: "emit",
	}}
	err = checkDecisionCoverage(manifest, parsed, decisions)
	if err == nil || !strings.Contains(err.Error(), "unknown declaration id") {
		t.Fatalf("checkDecisionCoverage unknown = %v, want unknown declaration error", err)
	}
}

func TestCheckCNameUniquenessUsesManifestPolicy(t *testing.T) {
	decisions := []parseDecision{{
		Source:   "variant_mapping",
		Function: "add",
		Action:   "emit",
		CName:    "mlx_add",
	}, {
		Source:   "allowed_detail_function",
		Function: "add",
		Action:   "emit",
		CName:    "mlx_add",
	}}
	if err := checkCNameUniqueness(plan.Manifest{}, decisions, nil); err != nil {
		t.Fatalf("checkCNameUniqueness without policy = %v, want nil", err)
	}
	manifest := plan.Manifest{
		Report: plan.ReportPolicy{RequireUniqueCNames: true},
	}
	err := checkCNameUniqueness(manifest, decisions, nil)
	if err == nil || !strings.Contains(err.Error(), "public C name mlx_add has multiple owners") {
		t.Fatalf("checkCNameUniqueness duplicate emit = %v, want duplicate name error", err)
	}
	decisions[1].Action = "skip"
	if err := checkCNameUniqueness(manifest, decisions, nil); err != nil {
		t.Fatalf("checkCNameUniqueness skipped duplicate = %v, want nil", err)
	}
	manifest.HookAPI = []plan.HookAPI{{
		CName: "mlx_custom_add",
		Names: []string{"mlx_add"},
	}}
	err = checkCNameUniqueness(manifest, decisions, nil)
	if err == nil || !strings.Contains(err.Error(), "hook_api mlx_custom_add") {
		t.Fatalf("checkCNameUniqueness hook duplicate = %v, want hook api duplicate error", err)
	}
	customSpecs := []customspec.Spec{{
		Name: "jaccl",
		Items: []customspec.Item{{
			Kind: "function",
			Name: "mlx_add",
		}, {
			Kind: "enum",
			Name: "mlx_jaccl_dtype",
			Values: []customspec.EnumValue{{
				Name: "MLX_JACCL_FLOAT32",
			}},
		}},
	}}
	manifest.HookAPI = nil
	err = checkCNameUniqueness(manifest, decisions, customSpecs)
	if err == nil || !strings.Contains(err.Error(), "custom_spec jaccl function:mlx_add") {
		t.Fatalf("checkCNameUniqueness custom function duplicate = %v, want custom spec duplicate error", err)
	}
	decisions[0].CName = "MLX_JACCL_FLOAT32"
	err = checkCNameUniqueness(manifest, decisions, customSpecs)
	if err == nil || !strings.Contains(err.Error(), "custom_spec jaccl enum:mlx_jaccl_dtype value:MLX_JACCL_FLOAT32") {
		t.Fatalf("checkCNameUniqueness custom enum value duplicate = %v, want enum value duplicate error", err)
	}
}

func TestEnrichDiagnosticsWithDeclIDs(t *testing.T) {
	diagnostics := []parseDiagnostic{
		{
			Code: "skip_variant_mapping",
			File: "/repo/mlx/ops.h",
			Line: 12,
			Col:  4,
		},
		{
			Code: "skip_template_function",
			File: "/repo/mlx/ops.h",
			Line: 20,
			Col:  1,
		},
	}
	parsed := ir.Result{Functions: []ir.FuncDecl{{
		ID:     "ops|mlx/ops.h|mlx::core|sum|array(array)",
		Loc:    ir.SourceLoc{File: "mlx/ops.h", Line: 12, Col: 4},
		Return: "array",
	}}}

	enrichDiagnosticsWithDeclIDs(diagnostics, parsed)
	if diagnostics[0].DeclID != "ops|mlx/ops.h|mlx::core|sum|array(array)" {
		t.Fatalf("diagnostic decl id = %q, want parsed declaration id", diagnostics[0].DeclID)
	}
	if diagnostics[1].DeclID != "" {
		t.Fatalf("unmatched diagnostic decl id = %q, want empty", diagnostics[1].DeclID)
	}
}

func hookManifest() plan.Manifest {
	base := ""
	return plan.Manifest{
		VariantMappings: map[string]map[string][]plan.Variant{
			"mlx_core": {
				"export_to_dot": {
					{Signature: "void(std::ostream&, NodeNamer, std::vector<array>)", Suffix: &base},
				},
				"print_graph": {
					{Signature: "void(std::ostream&, NodeNamer, std::vector<array>)", Suffix: &base},
				},
			},
		},
		CustomHooks: []plan.CustomHook{
			{CName: "mlx_fast_cuda_kernel", Reason: "custom CUDA kernel API"},
			{CName: "mlx_fast_metal_kernel", Reason: "custom Metal kernel API"},
			{CName: "mlx_load_gguf", Reason: "custom GGUF loading API"},
			{CName: "mlx_save_gguf", Reason: "custom GGUF saving API"},
		},
		HookAPI: []plan.HookAPI{
			{CName: "mlx_export_to_dot", Names: []string{
				"mlx_export_to_dot",
				"mlx_node_namer",
				"mlx_node_namer_free",
				"mlx_node_namer_get_name",
				"mlx_node_namer_new",
				"mlx_node_namer_set_name",
			}},
			{CName: "mlx_fast_cuda_kernel", Names: []string{
				"mlx_fast_cuda_kernel",
				"mlx_fast_cuda_kernel_apply",
				"mlx_fast_cuda_kernel_config",
				"mlx_fast_cuda_kernel_config_add_output_arg",
				"mlx_fast_cuda_kernel_config_add_template_arg_bool",
				"mlx_fast_cuda_kernel_config_add_template_arg_dtype",
				"mlx_fast_cuda_kernel_config_add_template_arg_int",
				"mlx_fast_cuda_kernel_config_free",
				"mlx_fast_cuda_kernel_config_new",
				"mlx_fast_cuda_kernel_config_set_grid",
				"mlx_fast_cuda_kernel_config_set_init_value",
				"mlx_fast_cuda_kernel_config_set_thread_group",
				"mlx_fast_cuda_kernel_config_set_verbose",
				"mlx_fast_cuda_kernel_free",
				"mlx_fast_cuda_kernel_new",
			}},
			{CName: "mlx_fast_metal_kernel", Names: []string{
				"mlx_fast_metal_kernel",
				"mlx_fast_metal_kernel_apply",
				"mlx_fast_metal_kernel_config",
				"mlx_fast_metal_kernel_config_add_output_arg",
				"mlx_fast_metal_kernel_config_add_template_arg_bool",
				"mlx_fast_metal_kernel_config_add_template_arg_dtype",
				"mlx_fast_metal_kernel_config_add_template_arg_int",
				"mlx_fast_metal_kernel_config_free",
				"mlx_fast_metal_kernel_config_new",
				"mlx_fast_metal_kernel_config_set_grid",
				"mlx_fast_metal_kernel_config_set_init_value",
				"mlx_fast_metal_kernel_config_set_thread_group",
				"mlx_fast_metal_kernel_config_set_verbose",
				"mlx_fast_metal_kernel_free",
				"mlx_fast_metal_kernel_new",
			}},
			{CName: "mlx_load_gguf", Names: []string{"mlx_load_gguf"}},
			{CName: "mlx_print_graph", Names: []string{"mlx_print_graph"}},
			{CName: "mlx_save_gguf", Names: []string{"mlx_save_gguf"}},
		},
	}
}

func TestResolveASTCacheDir(t *testing.T) {
	t.Setenv("MLX_C_AST_CACHE", "/tmp/mlx-c-env-cache")
	if got := resolveASTCacheDir("", false); got != "/tmp/mlx-c-env-cache" {
		t.Fatalf("default cache dir = %q, want env", got)
	}
	if got := resolveASTCacheDir("/tmp/mlx-c-explicit-cache", false); got != "/tmp/mlx-c-explicit-cache" {
		t.Fatalf("explicit cache dir = %q, want explicit", got)
	}
	if got := resolveASTCacheDir("/tmp/mlx-c-explicit-cache", true); got != "" {
		t.Fatalf("disabled cache dir = %q, want empty", got)
	}
}

func TestResolveFormatCacheDir(t *testing.T) {
	t.Setenv("MLX_C_FORMAT_CACHE", "/tmp/mlx-c-env-format-cache")
	if got := resolveFormatCacheDir("", false); got != "/tmp/mlx-c-env-format-cache" {
		t.Fatalf("default cache dir = %q, want env", got)
	}
	if got := resolveFormatCacheDir("/tmp/mlx-c-explicit-format-cache", false); got != "/tmp/mlx-c-explicit-format-cache" {
		t.Fatalf("explicit cache dir = %q, want explicit", got)
	}
	if got := resolveFormatCacheDir("/tmp/mlx-c-explicit-format-cache", true); got != "" {
		t.Fatalf("disabled cache dir = %q, want empty", got)
	}
}

func TestParseTypePolicyReportsMissingTypes(t *testing.T) {
	summary, missing, diagnostics, err := parseTypePolicy(parseOptions{
		RepoRoot:       ".",
		TypePolicyPath: "",
	}, ir.Result{
		Functions: []ir.FuncDecl{{
			ID:        "ops|mlx/ops.h|mlx::core|bad|Missing(array)",
			Module:    "ops",
			Header:    "mlx/ops.h",
			Namespace: "mlx::core",
			Name:      "bad",
			Return:    "Missing",
			Params:    []ir.Param{{Name: "x", Type: "array"}},
			Loc:       ir.SourceLoc{File: "mlx/ops.h", Line: 7, Col: 2},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if summary.MissingTypes != 1 || len(missing) != 1 || len(diagnostics) != 1 {
		t.Fatalf("summary=%#v missing=%#v diagnostics=%#v", summary, missing, diagnostics)
	}
	if diagnostics[0].Code != "missing_type_policy" ||
		diagnostics[0].File != "mlx/ops.h" ||
		diagnostics[0].Line != 7 ||
		diagnostics[0].Col != 2 {
		t.Fatalf("diagnostic = %#v", diagnostics[0])
	}
}

func TestGeneratorDiagnosticsConvertsUnsupportedTypes(t *testing.T) {
	diagnostics := generatorDiagnostics(generators.New(), &parser.ParseResult{
		Functions: map[string][]*parser.Function{
			"mlx::core::bad_return": {{
				Name:       "bad_return",
				Namespace:  "mlx::core",
				ReturnType: "MissingReturn",
				File:       "mlx/ops.h",
				Line:       9,
				Col:        4,
			}},
		},
		Enums: map[string]*parser.Enum{},
	})
	got, ok := firstDiagnosticWithCode(diagnostics, "skip_unsupported_return_type")
	if !ok {
		t.Fatalf("diagnostics = %#v, missing skip_unsupported_return_type", diagnostics)
	}
	if got.Code != "skip_unsupported_return_type" ||
		got.File != "mlx/ops.h" ||
		got.Line != 9 ||
		got.Col != 4 {
		t.Fatalf("diagnostic = %#v", got)
	}
}

func firstDiagnosticWithCode(diagnostics []parseDiagnostic, code string) (parseDiagnostic, bool) {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return diagnostic, true
		}
	}
	return parseDiagnostic{}, false
}

func TestRepoPath(t *testing.T) {
	root := filepath.Join(t.TempDir(), "repo")
	relative := filepath.Join("codegen", "mlxc-capi.lock.json")
	if got := repoPath(root, relative); got != filepath.Join(root, relative) {
		t.Fatalf("repoPath relative = %q", got)
	}
	absolute := filepath.Join(root, "absolute")
	if got := repoPath(root, absolute); got != absolute {
		t.Fatalf("repoPath absolute = %q", got)
	}
}

func TestWriteCheckReport(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report", "check.json")
	if err := writeCheckReport(path, []byte("report\n")); err != nil {
		t.Fatalf("writeCheckReport: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	if string(data) != "report\n" {
		t.Fatalf("report = %q, want report", string(data))
	}
}

func TestDiscoverMLXSourceUsesExplicitPath(t *testing.T) {
	got, err := discoverMLXSource("/tmp/mlx")
	if err != nil {
		t.Fatalf("discoverMLXSource explicit: %v", err)
	}
	if got != "/tmp/mlx" {
		t.Fatalf("discoverMLXSource explicit = %q", got)
	}
}

func TestDiscoverMLXSourceFindsBuildTree(t *testing.T) {
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldwd)

	want := filepath.Join("build-Release", "_deps", "mlx-src")
	if err := os.MkdirAll(want, 0o777); err != nil {
		t.Fatal(err)
	}
	got, err := discoverMLXSource("")
	if err != nil {
		t.Fatalf("discoverMLXSource fallback: %v", err)
	}
	if got != want {
		t.Fatalf("discoverMLXSource fallback = %q, want %q", got, want)
	}
}
