package regenreport

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tmc/mlx-c/internal/mlxcgen/customspec"
	"github.com/tmc/mlx-c/internal/mlxcgen/inventory"
	"github.com/tmc/mlx-c/internal/mlxcgen/ir"
	"github.com/tmc/mlx-c/internal/mlxcgen/plan"
	"github.com/tmc/mlx-c/internal/mlxcgen/types"
)

func TestCompare(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(t.TempDir(), "mlx", "c")
	write(t, root, "mlx/c/ops.h", "same\n")
	write(t, out, "ops.h", "same\n")
	write(t, root, "mlx/c/ops.cpp", "old\n")
	write(t, out, "ops.cpp", "new\n")
	write(t, root, "mlx/c/missing_generated.h", "checked\n")
	write(t, out, "extra.h", "extra\n")

	report, err := Compare(root, out, []string{
		"mlx/c/ops.h",
		"mlx/c/ops.cpp",
		"mlx/c/missing_generated.h",
		"mlx/c/missing_checked_in.h",
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.Equal != 1 ||
		report.Summary.Different != 1 ||
		report.Summary.MissingGenerated != 1 ||
		report.Summary.MissingCheckedIn != 1 {
		t.Fatalf("summary = %#v", report.Summary)
	}
	if len(report.GeneratedOnly) != 1 || report.GeneratedOnly[0] != "mlx/c/extra.h" {
		t.Fatalf("generated only = %#v", report.GeneratedOnly)
	}
	if report.Clean() {
		t.Fatalf("dirty report marked clean")
	}
}

func TestCleanReport(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(t.TempDir(), "mlx", "c")
	write(t, root, "mlx/c/ops.h", "same\n")
	write(t, out, "ops.h", "same\n")

	report, err := Compare(root, out, []string{"mlx/c/ops.h"})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Clean() {
		t.Fatalf("clean report marked dirty: %#v", report)
	}
}

func TestReportModules(t *testing.T) {
	got := reportModules(plan.Manifest{
		Headers: []plan.HeaderMapping{
			{Name: "ops", Headers: []string{"mlx/ops.h", "mlx/einsum.h"}},
			{Name: "fft", Headers: []string{"mlx/fft.h"}},
		},
	})
	if len(got) != 2 {
		t.Fatalf("modules = %#v, want two", got)
	}
	if got[0].Name != "ops" ||
		got[0].Headers[0] != "mlx/ops.h" ||
		got[0].Outputs[0] != "mlx/c/ops.cpp" ||
		got[0].Outputs[1] != "mlx/c/ops.h" {
		t.Fatalf("first module = %#v", got[0])
	}
}

func TestReportManifest(t *testing.T) {
	got := reportManifest(plan.Manifest{
		SchemaVersion: plan.SchemaVersion,
		MLX: plan.MLXPolicy{
			ExpectedGitRef: "v0.31.2",
		},
		Report: plan.ReportPolicy{
			RequireCleanGenerated:    true,
			RequireAPILock:           true,
			RequireDocCoverage:       true,
			RequireTypeCoverage:      true,
			RequireDiagnosticReasons: true,
			RequireExplicitVariants:  true,
			RequireDecisionDeclIDs:   true,
			RequireDecisionCoverage:  true,
			RequireEmitAPILock:       true,
			RequireUniqueCNames:      true,
			IncludeInventory:         true,
		},
		GeneratedMarkers: plan.GeneratedMarkerPolicy{
			ForbidVolatileData: true,
		},
		CustomHooks: []plan.CustomHook{{CName: "mlx_load_gguf", Reason: "custom GGUF loading API"}},
		HookAPI:     []plan.HookAPI{{CName: "mlx_load_gguf", Names: []string{"mlx_load_gguf"}}},
	})
	if got.SchemaVersion != plan.SchemaVersion ||
		got.MLX.ExpectedGitRef != "v0.31.2" ||
		!got.Report.RequireCleanGenerated ||
		!got.Report.RequireAPILock ||
		!got.Report.RequireDocCoverage ||
		!got.Report.RequireTypeCoverage ||
		!got.Report.RequireDiagnosticReasons ||
		!got.Report.RequireExplicitVariants ||
		!got.Report.RequireDecisionDeclIDs ||
		!got.Report.RequireDecisionCoverage ||
		!got.Report.RequireEmitAPILock ||
		!got.Report.RequireUniqueCNames ||
		!got.Report.IncludeInventory ||
		!got.GeneratedMarkers.ForbidVolatileData ||
		len(got.CustomHooks) != 1 ||
		got.CustomHooks[0].CName != "mlx_load_gguf" ||
		len(got.HookAPI) != 1 ||
		got.HookAPI[0].CName != "mlx_load_gguf" ||
		got.HookAPI[0].Names[0] != "mlx_load_gguf" {
		t.Fatalf("manifest = %#v", got)
	}
}

func TestReportMLXRef(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "test@example.com")
	runGit(t, repo, "config", "user.name", "Test")
	write(t, repo, "file.txt", "one\n")
	runGit(t, repo, "add", "file.txt")
	runGit(t, repo, "commit", "-m", "one")
	expected := runGit(t, repo, "rev-parse", "HEAD")
	runGit(t, repo, "tag", "v1")
	write(t, repo, "file.txt", "two\n")
	runGit(t, repo, "add", "file.txt")
	runGit(t, repo, "commit", "-m", "two")
	actual := runGit(t, repo, "rev-parse", "HEAD")

	got := reportMLXRef(repo, plan.MLXPolicy{ExpectedGitRef: "v1"})
	if got.ExpectedGitRef != "v1" ||
		got.ExpectedRevision != expected ||
		got.ActualRevision != actual ||
		got.MatchesExpected ||
		got.Error != "" {
		t.Fatalf("mlx ref = %#v, want mismatch", got)
	}

	got = reportMLXRef(repo, plan.MLXPolicy{ExpectedGitRef: "HEAD"})
	if got.ExpectedRevision != actual || got.ActualRevision != actual || !got.MatchesExpected || got.Error != "" {
		t.Fatalf("mlx ref = %#v, want match", got)
	}
}

func TestGeneratedOutputsIncludesCustomHeaders(t *testing.T) {
	got := generatedOutputs(plan.Manifest{
		Headers: []plan.HeaderMapping{{
			Name: "ops",
		}},
		Standalone: []string{"vector"},
	}, []customspec.Spec{{
		Header:   "mlx/c/jaccl.h",
		Generate: customspec.GenerateSpec{Header: true},
	}})
	want := []string{
		"mlx/c/jaccl.h",
		"mlx/c/ops.cpp",
		"mlx/c/ops.h",
		"mlx/c/private/vector.h",
		"mlx/c/vector.cpp",
		"mlx/c/vector.h",
	}
	if len(got) != len(want) {
		t.Fatalf("outputs = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("outputs = %#v, want %#v", got, want)
		}
	}
}

func TestReportTypePolicy(t *testing.T) {
	got := reportTypePolicy(types.Policy{
		SchemaVersion: types.SchemaVersion,
		Types: []types.TypeSpec{
			{CPP: "int", C: "int"},
			{CPP: "float", C: "float"},
		},
	}, []types.MissingType{{Type: "Missing"}})
	if got.SchemaVersion != types.SchemaVersion || got.Types != 2 || got.MissingTypes != 1 {
		t.Fatalf("type policy = %#v", got)
	}
}

func TestReportInventory(t *testing.T) {
	got := reportInventory([]inventory.Entry{
		{Kind: "handwritten_runtime", Target: "jacclc", Path: "mlx/c/jaccl.cpp"},
		{Kind: "generated_header_api", Target: "mlxc", Path: "mlx/c/ops.h"},
	})
	want := []Inventory{
		{Kind: "handwritten_runtime", Target: "jacclc", Path: "mlx/c/jaccl.cpp"},
		{Kind: "generated_header_api", Target: "mlxc", Path: "mlx/c/ops.h"},
	}
	if len(got) != len(want) {
		t.Fatalf("inventory = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("inventory = %#v, want %#v", got, want)
		}
	}
}

func TestReportInputDigests(t *testing.T) {
	root := t.TempDir()
	write(t, root, "codegen/manifest.yaml", "manifest\n")
	write(t, root, "codegen/modules/ops.yaml", "ops\n")
	write(t, root, "codegen/modules/fft.yaml", "fft\n")
	write(t, root, "codegen/types.yaml", "types\n")
	write(t, root, "codegen/generated-files.txt", "inventory\n")
	write(t, root, "codegen/custom/jaccl.yaml", "jaccl\n")
	write(t, root, "codegen/custom/readme.txt", "ignored\n")

	got, err := reportInputDigests(
		root,
		filepath.Join(root, "codegen", "manifest.yaml"),
		filepath.Join(root, "codegen", "types.yaml"),
		filepath.Join(root, "codegen", "generated-files.txt"),
		filepath.Join(root, "codegen", "custom"),
		plan.Manifest{ModuleFiles: []string{
			"modules/ops.yaml",
			"modules/fft.yaml",
		}},
	)
	if err != nil {
		t.Fatalf("reportInputDigests: %v", err)
	}
	if got.Manifest == nil || got.Manifest.Path != "codegen/manifest.yaml" || got.Manifest.SHA256 != hash([]byte("manifest\n")) {
		t.Fatalf("manifest digest = %#v", got.Manifest)
	}
	wantModules := []PathDigest{
		{Path: "codegen/modules/ops.yaml", SHA256: hash([]byte("ops\n"))},
		{Path: "codegen/modules/fft.yaml", SHA256: hash([]byte("fft\n"))},
	}
	if !reflect.DeepEqual(got.ModuleFiles, wantModules) {
		t.Fatalf("module digests = %#v, want %#v", got.ModuleFiles, wantModules)
	}
	if got.TypePolicy == nil || got.TypePolicy.Path != "codegen/types.yaml" || got.TypePolicy.SHA256 != hash([]byte("types\n")) {
		t.Fatalf("type policy digest = %#v", got.TypePolicy)
	}
	if got.Inventory == nil || got.Inventory.Path != "codegen/generated-files.txt" || got.Inventory.SHA256 != hash([]byte("inventory\n")) {
		t.Fatalf("inventory digest = %#v", got.Inventory)
	}
	wantCustom := []PathDigest{{Path: "codegen/custom/jaccl.yaml", SHA256: hash([]byte("jaccl\n"))}}
	if !reflect.DeepEqual(got.CustomSpecs, wantCustom) {
		t.Fatalf("custom digests = %#v, want %#v", got.CustomSpecs, wantCustom)
	}
}

func TestCheckInventoryUsesIncludeInventoryPolicy(t *testing.T) {
	root := t.TempDir()
	inventoryPath := filepath.Join(root, "codegen", "generated-files.txt")
	write(t, root, "CMakeLists.txt", "")
	write(t, root, "mlx/c/mlx.h", "")
	write(t, root, "codegen/generated-files.txt", "generated_header_api mlxc mlx/c/not_planned.h\n")
	if _, err := checkInventory(root, inventoryPath, plan.Manifest{}, nil); err != nil {
		t.Fatalf("checkInventory without policy = %v, want nil", err)
	}
	_, err := checkInventory(root, inventoryPath, plan.Manifest{
		Report: plan.ReportPolicy{IncludeInventory: true},
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "inventory path mlx/c/not_planned.h") {
		t.Fatalf("checkInventory with policy = %v, want inventory validation error", err)
	}
}

func TestReportCustomSpecs(t *testing.T) {
	got := reportCustomSpecs([]customspec.Spec{{
		Name:           "jaccl",
		Target:         "jacclc",
		Header:         "mlx/c/jaccl.h",
		Implementation: "mlx/c/jaccl.cpp",
		Ownership:      "handwritten_runtime",
		Generate:       customspec.GenerateSpec{Header: true},
		Items: []customspec.Item{
			{Kind: "struct", Name: "mlx_jaccl_group", Action: "custom_spec", Reason: "runtime_handle", Opaque: true},
			{
				Kind:   "enum",
				Name:   "mlx_jaccl_dtype",
				Action: "custom_spec",
				Reason: "dtype_table",
				Values: []customspec.EnumValue{
					{Name: "MLX_JACCL_FLOAT32", Value: 11},
				},
			},
			{
				Kind:      "function",
				Name:      "mlx_jaccl_group_free",
				Action:    "handwritten",
				Reason:    "runtime_lifetime",
				Signature: "int mlx_jaccl_group_free(mlx_jaccl_group group)",
			},
		},
	}})
	want := []CustomSpec{{
		Name:            "jaccl",
		Target:          "jacclc",
		Header:          "mlx/c/jaccl.h",
		Implementation:  "mlx/c/jaccl.cpp",
		Ownership:       "handwritten_runtime",
		GeneratedHeader: true,
		Items:           3,
		ActionCounts: []Count{
			{Name: "custom_spec", Count: 2},
			{Name: "handwritten", Count: 1},
		},
		KindCounts: []Count{
			{Name: "enum", Count: 1},
			{Name: "function", Count: 1},
			{Name: "struct", Count: 1},
		},
		ReasonCounts: []Count{
			{Name: "dtype_table", Count: 1},
			{Name: "runtime_handle", Count: 1},
			{Name: "runtime_lifetime", Count: 1},
		},
		ItemDecisions: []CustomSpecItem{
			{Kind: "struct", Name: "mlx_jaccl_group", Action: "custom_spec", Reason: "runtime_handle", Opaque: true},
			{
				Kind:   "enum",
				Name:   "mlx_jaccl_dtype",
				Action: "custom_spec",
				Reason: "dtype_table",
				Values: []CustomSpecEnumValue{
					{Name: "MLX_JACCL_FLOAT32", Value: 11},
				},
			},
			{
				Kind:      "function",
				Name:      "mlx_jaccl_group_free",
				Action:    "handwritten",
				Reason:    "runtime_lifetime",
				Signature: "int mlx_jaccl_group_free(mlx_jaccl_group group)",
			},
		},
	}}
	if len(got) != len(want) {
		t.Fatalf("custom specs = %#v, want %#v", got, want)
	}
	for i := range want {
		if !reflect.DeepEqual(got[i], want[i]) {
			t.Fatalf("custom specs = %#v, want %#v", got, want)
		}
	}
}

func TestGeneratorArgsIncludesCompileCommands(t *testing.T) {
	args := generatorArgs(Options{
		MLXSrc:              "/tmp/mlx",
		ManifestPath:        "/repo/codegen/manifest.yaml",
		CustomDir:           "/repo/codegen/custom",
		CompileCommandsPath: "/tmp/build/compile_commands.json",
		ASTCacheDir:         "/tmp/cache",
		FormatCacheDir:      "/tmp/format-cache",
		Generator:           []string{"go", "run", "./tools/mlx-c-gen"},
		NoFormat:            true,
	}, "/tmp/out", "/tmp/meta.yaml")
	want := []string{
		"run",
		"./tools/mlx-c-gen",
		"--mlx-src",
		"/tmp/mlx",
		"--output-dir",
		"/tmp/out",
		"--metadata",
		"/tmp/meta.yaml",
		"--manifest",
		"/repo/codegen/manifest.yaml",
		"--custom-dir",
		"/repo/codegen/custom",
		"--compile-commands",
		"/tmp/build/compile_commands.json",
		"--ast-cache",
		"/tmp/cache",
		"--format-cache",
		"/tmp/format-cache",
		"--no-format",
	}
	if len(args) != len(want) {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("args = %#v, want %#v", args, want)
		}
	}
}

func TestGeneratorArgsCanDisableASTCache(t *testing.T) {
	args := generatorArgs(Options{
		MLXSrc:     "/tmp/mlx",
		Generator:  []string{"go", "run", "./tools/mlx-c-gen"},
		NoASTCache: true,
	}, "/tmp/out", "/tmp/meta.yaml")
	want := "--no-ast-cache"
	for _, arg := range args {
		if arg == want {
			return
		}
	}
	t.Fatalf("args = %#v, missing %s", args, want)
}

func TestGeneratorArgsCanDisableFormatCache(t *testing.T) {
	args := generatorArgs(Options{
		MLXSrc:        "/tmp/mlx",
		Generator:     []string{"go", "run", "./tools/mlx-c-gen"},
		NoFormatCache: true,
	}, "/tmp/out", "/tmp/meta.yaml")
	want := "--no-format-cache"
	for _, arg := range args {
		if arg == want {
			return
		}
	}
	t.Fatalf("args = %#v, missing %s", args, want)
}

func TestResolveOptionsDefaultsASTCache(t *testing.T) {
	t.Setenv("MLX_C_AST_CACHE", "/tmp/mlx-c-report-cache")
	t.Setenv("MLX_C_FORMAT_CACHE", "/tmp/mlx-c-format-cache")
	opts := resolveOptions(Options{})
	if opts.ASTCacheDir != "/tmp/mlx-c-report-cache" {
		t.Fatalf("cache dir = %q, want env", opts.ASTCacheDir)
	}
	if opts.FormatCacheDir != "/tmp/mlx-c-format-cache" {
		t.Fatalf("format cache dir = %q, want env", opts.FormatCacheDir)
	}
	if opts.TypePolicyPath != filepath.Join(".", "codegen", "types.yaml") {
		t.Fatalf("type policy path = %q", opts.TypePolicyPath)
	}
	if opts.NoASTCache {
		t.Fatalf("NoASTCache = true")
	}
}

func TestResolveOptionsCanDisableASTCache(t *testing.T) {
	t.Setenv("MLX_C_AST_CACHE", "/tmp/mlx-c-report-cache")
	opts := resolveOptions(Options{
		ASTCacheDir: "/tmp/explicit-cache",
		NoASTCache:  true,
	})
	if opts.ASTCacheDir != "" {
		t.Fatalf("cache dir = %q, want disabled", opts.ASTCacheDir)
	}
}

func TestResolveOptionsCanDisableFormatCache(t *testing.T) {
	t.Setenv("MLX_C_FORMAT_CACHE", "/tmp/mlx-c-format-cache")
	opts := resolveOptions(Options{
		FormatCacheDir: "/tmp/explicit-format-cache",
		NoFormatCache:  true,
	})
	if opts.FormatCacheDir != "" {
		t.Fatalf("format cache dir = %q, want disabled", opts.FormatCacheDir)
	}
}

func TestRepoPath(t *testing.T) {
	root := filepath.Join(t.TempDir(), "repo")
	relative := filepath.Join("codegen", "generated-files.txt")
	if got := repoPath(root, relative); got != filepath.Join(root, relative) {
		t.Fatalf("repoPath relative = %q", got)
	}
	absolute := filepath.Join(root, "absolute")
	if got := repoPath(root, absolute); got != absolute {
		t.Fatalf("repoPath absolute = %q", got)
	}
}

func TestReadMetadataDiagnostics(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metadata.yaml")
	write(t, filepath.Dir(path), filepath.Base(path), `ir:
  functions:
    - id: ops|mlx/ops.h|mlx::core|add|array(array, array)
      module: ops
      header: mlx/ops.h
      namespace: mlx::core
      name: add
      return: array
type_policy_ir:
  functions:
    - id: ops|mlx/ops.h|mlx::core|selected|Missing(array)
      module: ops
      header: mlx/ops.h
      namespace: mlx::core
      name: selected
      return: Missing
functions: []
enums: []
diagnostics:
  - code: skip_variant_mapping
    message: skipped by variant mapping
    file: mlx/metal.h
    line: 22
    col: 5
`)

	diagnostics, err := readMetadataDiagnostics(path)
	if err != nil {
		t.Fatalf("readMetadataDiagnostics: %v", err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("diagnostics = %#v, want one", diagnostics)
	}
	got := diagnostics[0]
	want := Diagnostic{
		Code:    "skip_variant_mapping",
		Message: "skipped by variant mapping",
		File:    "mlx/metal.h",
		Line:    22,
		Col:     5,
	}
	if got != want {
		t.Fatalf("diagnostic = %#v, want %#v", got, want)
	}

	metadata, err := readMetadata(path)
	if err != nil {
		t.Fatalf("readMetadata: %v", err)
	}
	wantIR := ir.Result{
		Functions: []ir.FuncDecl{{
			ID:        "ops|mlx/ops.h|mlx::core|add|array(array, array)",
			Module:    "ops",
			Header:    "mlx/ops.h",
			Namespace: "mlx::core",
			Name:      "add",
			Return:    "array",
		}},
	}
	if len(metadata.IR.Functions) != 1 ||
		metadata.IR.Functions[0].ID != wantIR.Functions[0].ID ||
		metadata.IR.Functions[0].Module != wantIR.Functions[0].Module ||
		metadata.IR.Functions[0].Header != wantIR.Functions[0].Header ||
		metadata.IR.Functions[0].Namespace != wantIR.Functions[0].Namespace ||
		metadata.IR.Functions[0].Name != wantIR.Functions[0].Name ||
		metadata.IR.Functions[0].Return != wantIR.Functions[0].Return {
		t.Fatalf("IR = %#v, want %#v", metadata.IR, wantIR)
	}
	if len(metadata.TypePolicyIR.Functions) != 1 ||
		metadata.TypePolicyIR.Functions[0].ID != "ops|mlx/ops.h|mlx::core|selected|Missing(array)" {
		t.Fatalf("type policy IR = %#v, want selected function", metadata.TypePolicyIR)
	}
}

func TestReadMetadataDiagnosticsAllowsMissingMetadata(t *testing.T) {
	diagnostics, err := readMetadataDiagnostics(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatalf("readMetadataDiagnostics missing: %v", err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v, want none", diagnostics)
	}
}

func TestCheckGeneratedMarkersAllowsStableMarker(t *testing.T) {
	out := filepath.Join(t.TempDir(), "mlx", "c")
	write(t, out, "ops.h", `/* Copyright 2023-2024 Apple Inc.                     */
/*                                                    */
/* This file is auto-generated. Do not edit manually. */
/*                                                    */

#ifndef MLX_OPS_H
#define MLX_OPS_H
`)
	write(t, out, "jaccl.h", `/* Copyright 2026 Apple Inc. */
/* This file is auto-generated. Do not edit manually. */

#ifndef MLX_JACCL_H
#define MLX_JACCL_H
`)
	got, err := checkGeneratedMarkers(out, []string{
		"mlx/c/ops.h",
		"mlx/c/jaccl.h",
	}, plan.GeneratedMarkerPolicy{ForbidVolatileData: true})
	if err != nil {
		t.Fatalf("checkGeneratedMarkers: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("violations = %#v, want none", got)
	}
}

func TestGeneratedMarkerViolationsReportsVolatileMarkers(t *testing.T) {
	got := generatedMarkerViolations("mlx/c/ops.h", []byte(`/* Generated at 2026-05-31T17:00:00Z */
/* Generated from /Users/tmc/mlx-c */
/* Generated in /tmp/mlx-c */
/* Generated on host: buildbox */

#ifndef MLX_OPS_H
#define MLX_OPS_H
`))
	reasons := map[string]bool{}
	for _, violation := range got {
		reasons[violation.Reason] = true
		if violation.Path != "mlx/c/ops.h" || violation.Marker == "" {
			t.Fatalf("violation = %#v", violation)
		}
	}
	for _, want := range []string{"timestamp", "host_path", "temp_path", "hostname"} {
		if !reasons[want] {
			t.Fatalf("reasons = %#v, missing %s", reasons, want)
		}
	}
}

func TestNormalizeTransientWorkDir(t *testing.T) {
	workDir := filepath.Join(string(filepath.Separator), "tmp", "mlx-c-regen-123")
	report := &Report{
		WorkDir:      workDir,
		OutputDir:    filepath.Join(workDir, "mlx", "c"),
		MetadataPath: filepath.Join(workDir, "metadata.yaml"),
		Command: []string{
			"mlx-c-gen",
			"--output-dir",
			filepath.Join(workDir, "mlx", "c"),
			"--metadata",
			filepath.Join(workDir, "metadata.yaml"),
		},
		GeneratorOut: "Output directory: " + filepath.Join(workDir, "mlx", "c") + "\n",
		GeneratorErr: "metadata: " + filepath.Join(workDir, "metadata.yaml") + "\n",
	}
	normalizeTransientWorkDir(report, workDir)
	if report.WorkDir != "<workdir>" ||
		report.OutputDir != filepath.Join("<workdir>", "mlx", "c") ||
		report.MetadataPath != filepath.Join("<workdir>", "metadata.yaml") {
		t.Fatalf("paths = work %q output %q metadata %q", report.WorkDir, report.OutputDir, report.MetadataPath)
	}
	wantCommand := []string{
		"mlx-c-gen",
		"--output-dir",
		filepath.Join("<workdir>", "mlx", "c"),
		"--metadata",
		filepath.Join("<workdir>", "metadata.yaml"),
	}
	if !reflect.DeepEqual(report.Command, wantCommand) {
		t.Fatalf("command = %#v, want %#v", report.Command, wantCommand)
	}
	if strings.Contains(report.GeneratorOut, workDir) || strings.Contains(report.GeneratorErr, workDir) {
		t.Fatalf("generator output still contains work dir: out=%q err=%q", report.GeneratorOut, report.GeneratorErr)
	}
}

func write(t *testing.T, root, name, data string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o666); err != nil {
		t.Fatal(err)
	}
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}
