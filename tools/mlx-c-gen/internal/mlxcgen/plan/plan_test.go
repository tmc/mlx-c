package plan

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tmc/mlx-c/tools/mlx-c-gen/internal/mlxcgen/inventory"
)

func TestDefaultManifestPreservesPlan(t *testing.T) {
	manifest, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	if manifest.SchemaVersion != SchemaVersion {
		t.Fatalf("schema version = %d, want %d", manifest.SchemaVersion, SchemaVersion)
	}
	if manifest.MLX.ExpectedGitRef != "v0.32.0" {
		t.Fatalf("MLX expected git ref = %q", manifest.MLX.ExpectedGitRef)
	}
	if !manifest.Report.RequireCleanGenerated ||
		!manifest.Report.RequireAPILock ||
		!manifest.Report.RequireDocCoverage ||
		!manifest.Report.RequireTypeCoverage ||
		!manifest.Report.RequireDiagnosticReasons ||
		!manifest.Report.RequireExplicitVariants ||
		!manifest.Report.IncludeInventory {
		t.Fatalf("report policy = %#v", manifest.Report)
	}
	if !manifest.Report.RequireDecisionDeclIDs ||
		!manifest.Report.RequireDecisionCoverage ||
		!manifest.Report.RequireEmitAPILock ||
		!manifest.Report.RequireUniqueCNames {
		t.Fatalf("decision report policy = %#v", manifest.Report)
	}
	if !manifest.GeneratedMarkers.ForbidVolatileData {
		t.Fatalf("generated marker policy = %#v", manifest.GeneratedMarkers)
	}
	wantCustomHooks := []CustomHook{
		{CName: "mlx_fast_cuda_kernel", Reason: "custom CUDA kernel API"},
		{CName: "mlx_fast_metal_kernel", Reason: "custom Metal kernel API"},
		{CName: "mlx_load_gguf", Reason: "custom GGUF loading API"},
		{CName: "mlx_save_gguf", Reason: "custom GGUF saving API"},
	}
	if !reflect.DeepEqual(manifest.CustomHooks, wantCustomHooks) {
		t.Fatalf("custom hooks = %#v, want %#v", manifest.CustomHooks, wantCustomHooks)
	}
	if len(manifest.HookAPI) != 6 {
		t.Fatalf("hook api = %#v, want six entries", manifest.HookAPI)
	}
	if got := hookAPINames(manifest, "mlx_export_to_dot"); !reflect.DeepEqual(got, []string{
		"mlx_node_namer",
		"mlx_node_namer_new",
		"mlx_node_namer_free",
		"mlx_node_namer_set_name",
		"mlx_node_namer_get_name",
		"mlx_export_to_dot",
	}) {
		t.Fatalf("mlx_export_to_dot hook api = %#v", got)
	}
	if len(manifest.ModuleFiles) != 14 {
		t.Fatalf("module files = %#v", manifest.ModuleFiles)
	}

	headers, err := HeaderMappings()
	if err != nil {
		t.Fatal(err)
	}
	wantHeaders := []HeaderMapping{
		{Name: "ops", Headers: []string{"mlx/ops.h", "mlx/einsum.h"}, Docstring: "Core array operations"},
		{Name: "linalg", Headers: []string{"mlx/linalg.h"}, Docstring: "Linear algebra operations"},
		{Name: "random", Headers: []string{"mlx/random.h"}, Docstring: "Random number operations"},
		{Name: "fft", Headers: []string{"mlx/fft.h"}, Docstring: "FFT operations"},
		{Name: "fast", Headers: []string{"mlx/fast.h"}, Docstring: "Fast custom operations"},
		{Name: "io", Headers: []string{"mlx/io.h"}, Docstring: "IO operations"},
		{Name: "compile", Headers: []string{"mlx/compile.h", "mlx/compile_impl.h"}, Docstring: "Compilation operations"},
		{Name: "transforms", Headers: []string{"mlx/transforms.h"}, Docstring: "Transform operations"},
		{
			Name:        "transforms_impl",
			Headers:     []string{"mlx/transforms_impl.h"},
			Docstring:   "Implementation detail operations",
			PreIncludes: []string{"mlx/array.h", "mlx/transforms.h"},
		},
		{Name: "memory", Headers: []string{"mlx/memory.h"}, Docstring: "Memory operations"},
		{Name: "metal", Headers: []string{"mlx/backend/metal/metal.h"}, Docstring: "Metal specific operations"},
		{Name: "cuda", Headers: []string{"mlx/backend/cuda/cuda.h"}, Docstring: "Cuda specific operations"},
		{Name: "graph_utils", Headers: []string{"mlx/graph_utils.h"}, Docstring: "Graph Utils"},
		{Name: "distributed", Headers: []string{"mlx/distributed/ops.h"}, Docstring: "Distributed collectives"},
	}
	if !reflect.DeepEqual(headers, wantHeaders) {
		t.Fatalf("HeaderMappings = %#v, want %#v", headers, wantHeaders)
	}

	standalone, err := StandaloneNames()
	if err != nil {
		t.Fatal(err)
	}
	wantStandalone := []string{"vector", "closure", "map"}
	if !reflect.DeepEqual(standalone, wantStandalone) {
		t.Fatalf("StandaloneNames = %#v, want %#v", standalone, wantStandalone)
	}

	variants, err := VariantMappings()
	if err != nil {
		t.Fatal(err)
	}
	arange := variants["mlx_core"]["arange"]
	if len(arange) != 9 {
		t.Fatalf("arange variants length = %d, want 9", len(arange))
	}
	if arange[0].Signature != "array(double, double, double, Dtype, StreamOrDevice)" {
		t.Fatalf("arange first signature = %q", arange[0].Signature)
	}
	if arange[0].Suffix == nil || *arange[0].Suffix != "" {
		t.Fatalf("arange first variant = %#v, want base suffix", arange[0])
	}
	if !arange[1].Skip {
		t.Fatalf("arange second variant = %#v, want skip", arange[1])
	}
	if got := variants["mlx_core_fft"]["fftn"]; len(got) != 3 || !got[1].Skip || !got[2].Skip {
		t.Fatalf("fftn variants = %#v, want base then two skips", got)
	}

	allowedDetail, err := AllowedDetailFunctions()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"compile", "compile_clear_cache", "compile_erase", "vmap_replace", "vmap_trace"} {
		if !allowedDetail[name] {
			t.Fatalf("AllowedDetailFunctions missing %s", name)
		}
	}
}

func hookAPINames(manifest Manifest, cName string) []string {
	for _, api := range manifest.HookAPI {
		if api.CName == cName {
			return api.Names
		}
	}
	return nil
}

func TestGeneratedOutputsIncludeRecentHeaderMappings(t *testing.T) {
	outputs, err := GeneratedOutputs()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, out := range outputs {
		got[out] = true
	}
	for _, want := range []string{
		"mlx/c/cuda.h",
		"mlx/c/cuda.cpp",
		"mlx/c/graph_utils.h",
		"mlx/c/graph_utils.cpp",
		"mlx/c/private/vector.h",
	} {
		if !got[want] {
			t.Fatalf("GeneratedOutputs missing %s", want)
		}
	}
}

func TestCheckInventory(t *testing.T) {
	outputs, err := GeneratedOutputs()
	if err != nil {
		t.Fatal(err)
	}
	var entries []inventory.Entry
	for _, out := range outputs {
		entries = append(entries, inventory.Entry{
			Kind:   "generated_header_api",
			Target: "mlxc",
			Path:   out,
		})
	}
	if err := CheckInventory(entries); err != nil {
		t.Fatal(err)
	}
}

func TestCheckInventoryRejectsDrift(t *testing.T) {
	err := CheckInventory([]inventory.Entry{{
		Kind:   "generated_header_api",
		Target: "mlxc",
		Path:   "mlx/c/not_planned.h",
	}})
	if err == nil || !strings.Contains(err.Error(), "not generated by the Go plan") {
		t.Fatalf("CheckInventory error = %v", err)
	}
}

func TestCheckInventoryWithGeneratedChecksCustomHeaders(t *testing.T) {
	m := Manifest{}
	entries := []inventory.Entry{{
		Kind:   "custom_spec_generated",
		Target: "jacclc",
		Path:   "mlx/c/jaccl.h",
	}}
	if err := m.CheckInventoryWithGenerated(entries, []string{"mlx/c/jaccl.h"}); err != nil {
		t.Fatalf("CheckInventoryWithGenerated custom header = %v, want nil", err)
	}
	err := m.CheckInventoryWithGenerated(nil, []string{"mlx/c/jaccl.h"})
	if err == nil || !strings.Contains(err.Error(), "missing from inventory") {
		t.Fatalf("CheckInventoryWithGenerated missing = %v, want missing inventory error", err)
	}
	entries[0].Kind = "generated_header_api"
	err = m.CheckInventoryWithGenerated(entries, []string{"mlx/c/jaccl.h"})
	if err == nil || !strings.Contains(err.Error(), "want custom_spec_generated") {
		t.Fatalf("CheckInventoryWithGenerated kind = %v, want custom kind error", err)
	}
}

func TestCheckCMakeMLXRef(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "CMakeLists.txt", `
FetchContent_Declare(
  mlx
  GIT_REPOSITORY https://github.com/ml-explore/mlx.git
  GIT_TAG v0.31.2)
`)
	m := Manifest{
		SchemaVersion: SchemaVersion,
		MLX:           MLXPolicy{ExpectedGitRef: "v0.31.2"},
	}
	if err := m.CheckCMakeMLXRef(root); err != nil {
		t.Fatal(err)
	}
}

func TestCheckCMakeMLXRefRejectsDrift(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "CMakeLists.txt", `GIT_TAG v0.32.0)`)
	m := Manifest{
		SchemaVersion: SchemaVersion,
		MLX:           MLXPolicy{ExpectedGitRef: "v0.31.2"},
	}
	err := m.CheckCMakeMLXRef(root)
	if err == nil || !strings.Contains(err.Error(), "CMake MLX GIT_TAG") {
		t.Fatalf("CheckCMakeMLXRef error = %v", err)
	}
}

func TestLoadManifest(t *testing.T) {
	const manifest = `
schema_version: 1
mlx:
  expected_git_ref: v0.31.2
  release_revision: 2
headers:
  - name: ops
    headers:
      - mlx/ops.h
    doc: Core array operations
    pre_includes:
      - mlx/array.h
standalone:
  - vector
variant_mappings:
  mlx_core:
    squeeze:
      - signature: "array(array, std::vector<int>, StreamOrDevice)"
        suffix: axes
      - signature: "array(array, int, StreamOrDevice)"
        suffix: axis
      - signature: "array(array, StreamOrDevice)"
        suffix: ""
    grad:
      - signature: "std::function<array(const array&)>(std::function<array(const array&)>)"
        skip: true
        reason: unbindable_signature
allowed_detail_functions:
  - compile
`
	m, err := Load(strings.NewReader(manifest))
	if err != nil {
		t.Fatal(err)
	}
	if m.SchemaVersion != SchemaVersion || m.MLX.ExpectedGitRef != "v0.31.2" || m.MLX.ReleaseRevision != 2 {
		t.Fatalf("manifest metadata = %#v", m)
	}
	if len(m.Headers) != 1 || m.Headers[0].Name != "ops" {
		t.Fatalf("headers = %#v", m.Headers)
	}
	if got, want := m.Headers[0].PreIncludes[0], "mlx/array.h"; got != want {
		t.Fatalf("pre include = %q, want %q", got, want)
	}
	outputs := m.GeneratedOutputs()
	for _, want := range []string{"mlx/c/ops.h", "mlx/c/ops.cpp", "mlx/c/private/vector.h"} {
		if !contains(outputs, want) {
			t.Fatalf("GeneratedOutputs missing %s in %v", want, outputs)
		}
	}
	squeeze := m.VariantMappings["mlx_core"]["squeeze"]
	if got, want := squeeze[0].Signature, "array(array, std::vector<int>, StreamOrDevice)"; got != want {
		t.Fatalf("squeeze first signature = %q, want %q", got, want)
	}
	if got, want := *squeeze[0].Suffix, "axes"; got != want {
		t.Fatalf("squeeze first variant = %q, want %q", got, want)
	}
	if got, want := *squeeze[2].Suffix, ""; got != want {
		t.Fatalf("squeeze third variant = %q, want %q", got, want)
	}
	if grad := m.VariantMappings["mlx_core"]["grad"]; len(grad) != 1 || !grad[0].Skip || grad[0].Reason != "unbindable_signature" {
		t.Fatalf("grad variants = %#v, want one skip", grad)
	}
	if !m.AllowedDetailFunctionsSet()["compile"] {
		t.Fatalf("AllowedDetailFunctionsSet missing compile")
	}
}

func TestLoadFileLoadsModuleFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "manifest.yaml", `
schema_version: 1
mlx:
  expected_git_ref: v0.31.2
module_files:
  - modules/ops.yaml
standalone:
  - vector
`)
	writeFile(t, root, "modules/ops.yaml", `
name: ops
headers:
  - mlx/ops.h
doc: Core array operations
pre_includes:
  - mlx/array.h
`)
	m, err := LoadFile(filepath.Join(root, "manifest.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Headers) != 1 ||
		m.Headers[0].Name != "ops" ||
		m.Headers[0].Headers[0] != "mlx/ops.h" ||
		m.Headers[0].PreIncludes[0] != "mlx/array.h" {
		t.Fatalf("headers = %#v", m.Headers)
	}
}

func TestLoadPathLoadsExplicitFile(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "manifest.yaml", `
schema_version: 1
mlx:
  expected_git_ref: v0.31.2
headers:
  - name: ops
    headers:
      - mlx/ops.h
standalone:
  - vector
`)
	m, err := LoadPath(filepath.Join(root, "manifest.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Headers) != 1 || m.Headers[0].Name != "ops" {
		t.Fatalf("headers = %#v", m.Headers)
	}
}

func TestLoadRejectsModuleFilesWithoutPath(t *testing.T) {
	const manifest = `
schema_version: 1
mlx:
  expected_git_ref: v0.31.2
module_files:
  - modules/ops.yaml
standalone:
  - vector
`
	_, err := Load(strings.NewReader(manifest))
	if err == nil || !strings.Contains(err.Error(), "module_files require LoadFile") {
		t.Fatalf("Load error = %v", err)
	}
}

func TestLoadFileRejectsHeadersAndModuleFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "manifest.yaml", `
schema_version: 1
mlx:
  expected_git_ref: v0.31.2
module_files:
  - modules/ops.yaml
headers:
  - name: inline
    headers:
      - mlx/ops.h
standalone:
  - vector
`)
	writeFile(t, root, "modules/ops.yaml", `
name: ops
headers:
  - mlx/ops.h
`)
	_, err := LoadFile(filepath.Join(root, "manifest.yaml"))
	if err == nil || !strings.Contains(err.Error(), "must not set both headers and module_files") {
		t.Fatalf("LoadFile error = %v", err)
	}
}

func TestLoadFileRejectsEscapingModuleFiles(t *testing.T) {
	tests := []struct {
		name string
		file string
		want string
	}{
		{name: "absolute", file: "/tmp/ops.yaml", want: "must be relative"},
		{name: "traversal", file: "../ops.yaml", want: "must be clean and stay under the manifest directory"},
		{name: "unclean", file: "modules/../ops.yaml", want: "must be clean and stay under the manifest directory"},
		{name: "backslash", file: `modules\ops.yaml`, want: "contains backslash"},
		{name: "extension", file: "modules/ops.yml", want: "must have .yaml extension"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, root, "manifest.yaml", `
schema_version: 1
mlx:
  expected_git_ref: v0.31.2
module_files:
  - `+tt.file+`
standalone:
  - vector
`)
			_, err := LoadFile(filepath.Join(root, "manifest.yaml"))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("LoadFile error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestLoadFileRejectsOrphanModuleFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "manifest.yaml", `
schema_version: 1
mlx:
  expected_git_ref: v0.31.2
module_files:
  - modules/ops.yaml
standalone:
  - vector
`)
	writeFile(t, root, "modules/ops.yaml", `
name: ops
headers:
  - mlx/ops.h
`)
	writeFile(t, root, "modules/stale.yaml", `
name: stale
headers:
  - mlx/stale.h
`)
	_, err := LoadFile(filepath.Join(root, "manifest.yaml"))
	if err == nil || !strings.Contains(err.Error(), "module file modules/stale.yaml exists but is not listed in manifest") {
		t.Fatalf("LoadFile error = %v, want orphan module file error", err)
	}
}

func TestLoadManifestRejectsVariantWithoutDisposition(t *testing.T) {
	const manifest = `
schema_version: 1
mlx:
  expected_git_ref: v0.31.2
headers:
  - name: ops
    headers:
      - mlx/ops.h
standalone:
  - vector
variant_mappings:
  mlx_core:
    squeeze:
      - signature: "array(array, StreamOrDevice)"
`
	_, err := Load(strings.NewReader(manifest))
	if err == nil || !strings.Contains(err.Error(), "must set exactly one of suffix or skip") {
		t.Fatalf("Load error = %v", err)
	}
}

func TestLoadManifestRejectsDuplicateVariantSignatures(t *testing.T) {
	const manifest = `
schema_version: 1
mlx:
  expected_git_ref: v0.31.2
headers:
  - name: ops
    headers:
      - mlx/ops.h
standalone:
  - vector
variant_mappings:
  mlx_core:
    squeeze:
      - signature: "array(array, StreamOrDevice)"
        suffix: ""
      - signature: "array(array, StreamOrDevice)"
        skip: true
`
	_, err := Load(strings.NewReader(manifest))
	if err == nil || !strings.Contains(err.Error(), `duplicate signature "array(array, StreamOrDevice)"`) {
		t.Fatalf("Load error = %v", err)
	}
}

func TestLoadManifestRejectsVariantReasonWithoutSkip(t *testing.T) {
	const manifest = `
schema_version: 1
mlx:
  expected_git_ref: v0.31.2
headers:
  - name: ops
    headers:
      - mlx/ops.h
standalone:
  - vector
variant_mappings:
  mlx_core:
    squeeze:
      - signature: "array(array, StreamOrDevice)"
        suffix: ""
        reason: unbindable_signature
`
	_, err := Load(strings.NewReader(manifest))
	if err == nil || !strings.Contains(err.Error(), "has reason without skip") {
		t.Fatalf("Load error = %v", err)
	}
}

func TestLoadManifestRejectsVariantSkipWithoutReason(t *testing.T) {
	const manifest = `
schema_version: 1
mlx:
  expected_git_ref: v0.31.2
headers:
  - name: ops
    headers:
      - mlx/ops.h
standalone:
  - vector
variant_mappings:
  mlx_core:
    squeeze:
      - signature: "array(array, StreamOrDevice)"
        skip: true
`
	_, err := Load(strings.NewReader(manifest))
	if err == nil || !strings.Contains(err.Error(), "has skip without reason") {
		t.Fatalf("Load error = %v", err)
	}
}

func TestLoadManifestRejectsUnknownVariantSkipReason(t *testing.T) {
	const manifest = `
schema_version: 1
mlx:
  expected_git_ref: v0.31.2
headers:
  - name: ops
    headers:
      - mlx/ops.h
standalone:
  - vector
variant_mappings:
  mlx_core:
    squeeze:
      - signature: "array(array, StreamOrDevice)"
        skip: true
        reason: maybe_later
`
	_, err := Load(strings.NewReader(manifest))
	if err == nil || !strings.Contains(err.Error(), `unknown skip reason "maybe_later"`) {
		t.Fatalf("Load error = %v", err)
	}
}

func TestLoadManifestRejectsUnknownFields(t *testing.T) {
	const manifest = `
schema_version: 1
mlx:
  expected_git_ref: v0.31.2
headers:
  - name: ops
    headers:
      - mlx/ops.h
    unknown: value
standalone:
  - vector
`
	_, err := Load(strings.NewReader(manifest))
	if err == nil || !strings.Contains(err.Error(), "field unknown not found") {
		t.Fatalf("Load error = %v", err)
	}
}

func TestLoadManifestRejectsDuplicateHeaderMappings(t *testing.T) {
	const manifest = `
schema_version: 1
mlx:
  expected_git_ref: v0.31.2
headers:
  - name: ops
    headers:
      - mlx/ops.h
  - name: ops
    headers:
      - mlx/einsum.h
standalone:
  - vector
`
	_, err := Load(strings.NewReader(manifest))
	if err == nil || !strings.Contains(err.Error(), `duplicate header mapping "ops"`) {
		t.Fatalf("Load error = %v", err)
	}
}

func TestLoadManifestRejectsInvalidHeaderMappingName(t *testing.T) {
	const manifest = `
schema_version: 1
mlx:
  expected_git_ref: v0.31.2
headers:
  - name: ../ops
    headers:
      - mlx/ops.h
standalone:
  - vector
`
	_, err := Load(strings.NewReader(manifest))
	if err == nil || !strings.Contains(err.Error(), `header mapping "../ops" has invalid name`) {
		t.Fatalf("Load error = %v", err)
	}
}

func TestLoadManifestRejectsInvalidStandaloneName(t *testing.T) {
	const manifest = `
schema_version: 1
mlx:
  expected_git_ref: v0.31.2
headers:
  - name: ops
    headers:
      - mlx/ops.h
standalone:
  - ../vector
`
	_, err := Load(strings.NewReader(manifest))
	if err == nil || !strings.Contains(err.Error(), `standalone generator "../vector" has invalid name`) {
		t.Fatalf("Load error = %v", err)
	}
}

func TestLoadManifestRejectsInvalidHeaderPaths(t *testing.T) {
	tests := []struct {
		name   string
		field  string
		header string
		want   string
	}{
		{name: "absolute", field: "headers", header: "/tmp/ops.h", want: "must be relative"},
		{name: "traversal", field: "headers", header: "../ops.h", want: "must be clean and stay under the include root"},
		{name: "unclean", field: "headers", header: "mlx/../ops.h", want: "must be clean and stay under the include root"},
		{name: "backslash", field: "headers", header: `mlx\ops.h`, want: "contains backslash"},
		{name: "extension", field: "headers", header: "mlx/ops.hpp", want: "must have .h extension"},
		{name: "preinclude", field: "pre_includes", header: "../array.h", want: "pre-include"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headerBlock := "headers:\n      - mlx/ops.h"
			if tt.field == "headers" {
				headerBlock = "headers:\n      - " + tt.header
			}
			preIncludeBlock := ""
			if tt.field == "pre_includes" {
				preIncludeBlock = "\n    pre_includes:\n      - " + tt.header
			}
			_, err := Load(strings.NewReader(`
schema_version: 1
mlx:
  expected_git_ref: v0.31.2
headers:
  - name: ops
    ` + headerBlock + preIncludeBlock + `
standalone:
  - vector
`))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Load error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestLoadManifestRejectsDuplicateAllowedDetailFunctions(t *testing.T) {
	const manifest = `
schema_version: 1
mlx:
  expected_git_ref: v0.31.2
headers:
  - name: ops
    headers:
      - mlx/ops.h
standalone:
  - vector
allowed_detail_functions:
  - compile
  - compile
`
	_, err := Load(strings.NewReader(manifest))
	if err == nil || !strings.Contains(err.Error(), `duplicate allowed detail function "compile"`) {
		t.Fatalf("Load error = %v", err)
	}
}

func TestLoadManifestRejectsInvalidAllowedDetailFunction(t *testing.T) {
	const manifest = `
schema_version: 1
mlx:
  expected_git_ref: v0.31.2
headers:
  - name: ops
    headers:
      - mlx/ops.h
standalone:
  - vector
allowed_detail_functions:
  - compile-clear-cache
`
	_, err := Load(strings.NewReader(manifest))
	if err == nil || !strings.Contains(err.Error(), `allowed detail function "compile-clear-cache" has invalid name`) {
		t.Fatalf("Load error = %v", err)
	}
}

func TestLoadManifestRejectsDuplicateCustomHooks(t *testing.T) {
	const manifest = `
schema_version: 1
mlx:
  expected_git_ref: v0.31.2
headers:
  - name: ops
    headers:
      - mlx/ops.h
standalone:
  - vector
custom_hooks:
  - c_name: mlx_load_gguf
    reason: custom GGUF loading API
  - c_name: mlx_load_gguf
    reason: duplicate GGUF loading API
`
	_, err := Load(strings.NewReader(manifest))
	if err == nil || !strings.Contains(err.Error(), `duplicate custom hook "mlx_load_gguf"`) {
		t.Fatalf("Load error = %v", err)
	}
}

func TestLoadManifestRejectsInvalidCustomHookName(t *testing.T) {
	const manifest = `
schema_version: 1
mlx:
  expected_git_ref: v0.31.2
headers:
  - name: ops
    headers:
      - mlx/ops.h
standalone:
  - vector
custom_hooks:
  - c_name: mlx-load-gguf
    reason: custom GGUF loading API
`
	_, err := Load(strings.NewReader(manifest))
	if err == nil || !strings.Contains(err.Error(), `custom hook "mlx-load-gguf" has invalid c_name`) {
		t.Fatalf("Load error = %v", err)
	}
}

func TestLoadManifestRejectsCustomHookWithoutReason(t *testing.T) {
	const manifest = `
schema_version: 1
mlx:
  expected_git_ref: v0.31.2
headers:
  - name: ops
    headers:
      - mlx/ops.h
standalone:
  - vector
custom_hooks:
  - c_name: mlx_load_gguf
`
	_, err := Load(strings.NewReader(manifest))
	if err == nil || !strings.Contains(err.Error(), `custom hook "mlx_load_gguf" has empty reason`) {
		t.Fatalf("Load error = %v", err)
	}
}

func TestLoadManifestRejectsInvalidHookAPI(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "duplicate hook",
			body: `
hook_api:
  - c_name: mlx_load_gguf
    names:
      - mlx_load_gguf
  - c_name: mlx_load_gguf
    names:
      - mlx_load_gguf_again
`,
			want: `duplicate hook api "mlx_load_gguf"`,
		},
		{
			name: "empty names",
			body: `
hook_api:
  - c_name: mlx_load_gguf
`,
			want: `hook api "mlx_load_gguf" has no names`,
		},
		{
			name: "invalid name",
			body: `
hook_api:
  - c_name: mlx_load_gguf
    names:
      - mlx-load-gguf
`,
			want: `hook api "mlx_load_gguf" has invalid name "mlx-load-gguf"`,
		},
		{
			name: "duplicate name",
			body: `
hook_api:
  - c_name: mlx_load_gguf
    names:
      - mlx_load_gguf
      - mlx_load_gguf
`,
			want: `hook api "mlx_load_gguf" has duplicate name "mlx_load_gguf"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest := `
schema_version: 1
mlx:
  expected_git_ref: v0.31.2
headers:
  - name: ops
    headers:
      - mlx/ops.h
standalone:
  - vector
` + tt.body
			_, err := Load(strings.NewReader(manifest))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Load error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestLoadManifestRejectsMissingSchemaVersion(t *testing.T) {
	const manifest = `
mlx:
  expected_git_ref: v0.31.2
headers:
  - name: ops
    headers:
      - mlx/ops.h
standalone:
  - vector
`
	_, err := Load(strings.NewReader(manifest))
	if err == nil || !strings.Contains(err.Error(), "schema_version") {
		t.Fatalf("Load error = %v", err)
	}
}

func TestLoadManifestRejectsMissingMLXGitRef(t *testing.T) {
	const manifest = `
schema_version: 1
headers:
  - name: ops
    headers:
      - mlx/ops.h
standalone:
  - vector
`
	_, err := Load(strings.NewReader(manifest))
	if err == nil || !strings.Contains(err.Error(), "mlx expected_git_ref") {
		t.Fatalf("Load error = %v", err)
	}
}

func contains(list []string, s string) bool {
	for _, elem := range list {
		if elem == s {
			return true
		}
	}
	return false
}

func writeFile(t *testing.T, root, name, data string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o666); err != nil {
		t.Fatal(err)
	}
}
