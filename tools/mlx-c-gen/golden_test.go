package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/tmc/mlx-c/tools/mlx-c-gen/internal/mlxcgen/regenreport"
)

// TestEnvNeutralityGolden implements principle 7 of
// docs/design/cli-surface-simplification.md: environment variables are not a
// second CLI. It generates twice into one tree under maximally-differing
// environments and requires byte-identical outputs.
//
// The test needs clang, clang-format, and a real MLX source tree, so it skips
// unless MLX_C_GEN_GOLDEN_MLX_SRC names one. Point it at your checkout to run:
//
//	MLX_C_GEN_GOLDEN_MLX_SRC=~/mlx go test ./tools/mlx-c-gen/ -run EnvNeutrality
func TestEnvNeutralityGolden(t *testing.T) {
	mlxSrc := os.Getenv("MLX_C_GEN_GOLDEN_MLX_SRC")
	if mlxSrc == "" {
		t.Skip("set MLX_C_GEN_GOLDEN_MLX_SRC to an MLX source tree to run")
	}
	if _, err := os.Stat(filepath.Join(mlxSrc, "mlx", "ops.h")); err != nil {
		t.Fatalf("MLX_C_GEN_GOLDEN_MLX_SRC does not look like an MLX tree: %v", err)
	}
	for _, tool := range []string{"clang++", "clang-format"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Fatalf("missing %s required for golden generation", tool)
		}
	}

	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "tree")
	if err := os.MkdirAll(filepath.Join(root, "codegen"), 0o777); err != nil {
		t.Fatal(err)
	}
	// Copy the tree's codegen inputs so the run mutates only the copy.
	if out, err := exec.Command("cp", "-R",
		filepath.Join(repoRoot, "codegen")+"/",
		filepath.Join(root, "codegen")+"/").CombinedOutput(); err != nil {
		t.Fatalf("copy codegen: %v\n%s", err, out)
	}

	if err := os.MkdirAll(filepath.Join(root, "home-a"), 0o777); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{"home-b", "tmp-b", "cache-b"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o777); err != nil {
			t.Fatal(err)
		}
	}
	envA := []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + filepath.Join(root, "home-a"),
		"GOPATH=" + os.Getenv("GOPATH"),
		// No cache hints at all: defaults must apply.
	}
	envB := []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + filepath.Join(root, "home-b"),
		"TMPDIR=" + filepath.Join(root, "tmp-b"), // created below
		// Dead variables: must have no effect on output.
		"MLX_C_SRC=/nonexistent/decoy",
		"MLX_C_REGEN_METADATA=/nonexistent/metadata.yaml",
		// Cache hints: may relocate caches, never change output.
		"MLX_C_AST_CACHE=" + filepath.Join(root, "cache-b", "ast"),
		"MLX_C_FORMAT_CACHE=" + filepath.Join(root, "cache-b", "format"),
		"MLX_C_TOOL_CACHE=" + filepath.Join(root, "cache-b", "tool"),
	}

	bin := filepath.Join(t.TempDir(), "mlx-c-gen")
	build := exec.Command("go", "build", "-o", bin, "./tools/mlx-c-gen")
	build.Dir = repoRoot
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}

	run := func(env []string) map[string]string {
		t.Helper()
		cmd := exec.Command(bin, "generate", root, "--mlx-src", mlxSrc)
		cmd.Env = append(os.Environ()[:0], env...)
		cmd.Dir = repoRoot
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("generate: %v\n%s", err, out)
		}
		files := map[string]string{}
		outDir := filepath.Join(root, "mlx", "c")
		err := filepath.Walk(outDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(outDir, path)
			if err != nil {
				return err
			}
			files[rel] = string(data)
			return nil
		})
		if err != nil {
			t.Fatalf("walk generated tree: %v", err)
		}
		return files
	}

	first := run(envA)
	second := run(envB)
	if len(first) == 0 {
		t.Fatal("generation produced no files")
	}
	for name, want := range first {
		if got, ok := second[name]; !ok {
			t.Errorf("second run missing %s", name)
		} else if got != want {
			t.Errorf("%s differs between environments (%d vs %d bytes)", name, len(want), len(got))
		}
	}
	for name := range second {
		if _, ok := first[name]; !ok {
			t.Errorf("second run produced extra file %s", name)
		}
	}
}

// TestGoldenCheckReport pins the machine-readable shape of the check report,
// especially its symbol-check section: CI asserts properties of this JSON, so
// renames here are release-note-worthy surface changes, not refactors. Update
// the golden file with:
//
//	go test ./tools/mlx-c-gen/ -run GoldenCheckReport -update
func TestGoldenCheckReport(t *testing.T) {
	const update = false // flip under -ldflags or edit to regenerate

	report := &regenreport.Report{
		SchemaVersion: regenreport.SchemaVersion,
		SymbolChecks: []regenreport.SymbolCheck{{
			Target:          "mlxc",
			Path:            "/prefix/lib/libmlxc.dylib",
			Source:          "library",
			LockedFunctions: 3,
			DefinedSymbols:  10,
			PublicSymbols:   3,
			Problems:        []string{"mlxc: missing mlx_add"},
		}},
		Command: []string{"mlx-c-gen", "check", "."},
		Summary: regenreport.Summary{},
	}
	data, err := report.JSON()
	if err != nil {
		t.Fatal(err)
	}

	golden := filepath.Join("testdata", "check-report.golden.json")
	if update {
		if err := os.MkdirAll(filepath.Dir(golden), 0o777); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, data, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden (run with -update after intentional changes): %v", err)
	}

	var gotKeys, wantKeys []string
	cmp := func(data []byte, keys *[]string) error {
		var m map[string]json.RawMessage
		if err := json.Unmarshal(bytes.TrimSpace(data), &m); err != nil {
			return err
		}
		for k := range m {
			*keys = append(*keys, k)
		}
		return nil
	}
	if err := cmp(data, &gotKeys); err != nil {
		t.Fatalf("unmarshal current report: %v", err)
	}
	if err := cmp(want, &wantKeys); err != nil {
		t.Fatalf("unmarshal golden report: %v", err)
	}
	sortStrings(gotKeys)
	sortStrings(wantKeys)
	if !reflect.DeepEqual(gotKeys, wantKeys) {
		t.Fatalf("report top-level keys changed:\n got %v\nwant %v", gotKeys, wantKeys)
	}
	if !bytes.Equal(bytes.TrimSpace(data), bytes.TrimSpace(want)) {
		t.Fatalf("report content drifted from golden; this is a surface change requiring a release note")
	}

	// The symbol-check section is pinned field-by-field because CI asserts on it.
	var full struct {
		SymbolChecks []map[string]any `json:"symbol_checks"`
	}
	if err := json.Unmarshal(data, &full); err != nil {
		t.Fatal(err)
	}
	wantSymbolKeys := []string{"defined_symbols", "locked_functions", "path", "problems", "public_symbols", "source", "target"}
	sc := full.SymbolChecks[0]
	gotSymbolKeys := make([]string, 0, len(sc))
	for k := range sc {
		gotSymbolKeys = append(gotSymbolKeys, k)
	}
	sortStrings(gotSymbolKeys)
	if !reflect.DeepEqual(gotSymbolKeys, wantSymbolKeys) {
		t.Fatalf("symbol_checks keys changed:\n got %v\nwant %v", gotSymbolKeys, wantSymbolKeys)
	}
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
