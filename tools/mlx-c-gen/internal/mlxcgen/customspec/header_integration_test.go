package customspec

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRenderJacclHeaderMatchesCheckedIn(t *testing.T) {
	root := repoRoot(t)
	specPath := filepath.Join(root, "codegen", "custom", "jaccl.yaml")
	if _, err := os.Stat(specPath); os.IsNotExist(err) {
		t.Skip("jaccl.yaml not present in this tree (jaccl is opt-in/deferred)")
	}
	spec, err := LoadFile(specPath)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	rendered, err := RenderHeader(spec)
	if err != nil {
		t.Fatalf("RenderHeader: %v", err)
	}
	cmd := exec.Command("clang-format", "--assume-filename=mlx/c/jaccl.h")
	cmd.Stdin = bytes.NewReader(rendered)
	formatted, err := cmd.Output()
	if err != nil {
		t.Fatalf("clang-format: %v", err)
	}
	wantPath := filepath.Join(root, "mlx", "c", "jaccl.h")
	want, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("read %s: %v", wantPath, err)
	}
	if bytes.Equal(formatted, want) {
		return
	}
	gotPath := filepath.Join(t.TempDir(), "jaccl.h")
	if err := os.WriteFile(gotPath, formatted, 0o666); err != nil {
		t.Fatalf("write rendered header: %v", err)
	}
	diff, err := exec.Command("diff", "-u", wantPath, gotPath).CombinedOutput()
	if err != nil && len(diff) == 0 {
		t.Fatalf("diff: %v", err)
	}
	t.Fatalf("rendered jaccl header differs:\n%s", diff)
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		next := filepath.Dir(dir)
		if next == dir {
			t.Fatal("go.mod not found")
		}
		dir = next
	}
}
