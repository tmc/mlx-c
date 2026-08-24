package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tmc/mlx-c/tools/mlx-c-gen/internal/mlxcgen/apilock"
	"github.com/tmc/mlx-c/tools/mlx-c-gen/internal/mlxcgen/plan"
	"github.com/tmc/mlx-c/tools/mlx-c-gen/internal/mlxcgen/regenreport"
)

const testHeader = `#ifndef MLX_C_MLX_H
#define MLX_C_MLX_H
typedef struct mlx_array_ { void* ctx; } mlx_array;
int mlx_add(mlx_array* res, const mlx_array a);
#endif
`

func writeHeaders(t *testing.T, decls string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "mlx.h"), []byte(decls), 0o666); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestCheckUpstreamParityCLI(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "codegen"), 0o777); err != nil {
		t.Fatal(err)
	}
	ours, err := apilock.GenerateTarget(writeHeaders(t, testHeader), "mlx.h")
	if err != nil {
		t.Fatal(err)
	}
	lock := &apilock.Lock{Targets: map[string]apilock.Target{"mlxc": ours}}

	tests := []struct {
		name     string
		upstream string
		waivers  string
		required bool
		wantErr  string
	}{
		{
			name:     "identical surfaces pass",
			upstream: testHeader,
			required: true,
		},
		{
			name:     "divergence fails",
			upstream: strings.Replace(testHeader, "const mlx_array a);", "const mlx_array a, int axis);", 1),
			required: true,
			wantErr:  "mlx_add: undeclared divergence",
		},
		{
			name:     "divergence passes under a waiver",
			upstream: strings.Replace(testHeader, "const mlx_array a);", "const mlx_array a, int axis);", 1),
			required: true,
			waivers: `schema_version: 1
divergences:
  - name: mlx_add
    ours: "int mlx_add(mlx_array* res, const mlx_array a)"
    upstream: "int mlx_add(mlx_array* res, const mlx_array a, int axis)"
    reason: our cut predates the axis parameter
    resolution: rebind with the axis parameter in the next cut
`,
		},
		{
			name:     "no upstream configured but required",
			required: true,
			wantErr:  "declares no upstream.ref",
		},
		{
			name: "no upstream configured and not required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			waiverPath := filepath.Join(root, "codegen", "compat-waivers.yaml")
			os.Remove(waiverPath)
			if tt.waivers != "" {
				if err := os.WriteFile(waiverPath, []byte(tt.waivers), 0o666); err != nil {
					t.Fatal(err)
				}
			}
			opts := checkOptions{
				Options:          regenreport.Options{RepoRoot: root},
				LockPath:         "codegen/mlxc-capi.lock.json",
				ParityWaiverPath: "codegen/compat-waivers.yaml",
			}
			if tt.upstream != "" {
				opts.UpstreamHeaders = writeHeaders(t, tt.upstream)
			}
			manifest := plan.Manifest{}
			manifest.Report.RequireUpstreamParity = tt.required
			checkGateErr(t, checkUpstreamParity(opts, lock, manifest), tt.wantErr)
		})
	}
}

// TestCheckLockMonotonicityCLI drives the gate through its real baseline
// source: the lock committed at a git ref, not the working-tree file.
func TestCheckLockMonotonicityCLI(t *testing.T) {
	root := t.TempDir()
	lockPath := "codegen/mlxc-capi.lock.json"
	baseline := &apilock.Lock{SchemaVersion: 1, Targets: map[string]apilock.Target{"mlxc": {
		Functions: []apilock.Function{{Name: "mlx_add"}, {Name: "mlx_flip"}},
	}}}
	data, err := baseline.JSON()
	if err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, root, lockPath, data)

	tests := []struct {
		name    string
		cur     *apilock.Lock
		waivers string
		wantErr string
	}{
		{
			name: "unchanged passes",
			cur:  baseline,
		},
		{
			name: "growth passes",
			cur: &apilock.Lock{Targets: map[string]apilock.Target{"mlxc": {
				Functions: []apilock.Function{{Name: "mlx_add"}, {Name: "mlx_flip"}, {Name: "mlx_event_new"}},
			}}},
		},
		{
			name: "silent removal fails",
			cur: &apilock.Lock{Targets: map[string]apilock.Target{"mlxc": {
				Functions: []apilock.Function{{Name: "mlx_add"}},
			}}},
			wantErr: "mlxc/mlx_flip: removed from the API lock without a removals waiver",
		},
		{
			name: "waived removal passes",
			cur: &apilock.Lock{Targets: map[string]apilock.Target{"mlxc": {
				Functions: []apilock.Function{{Name: "mlx_add"}},
			}}},
			waivers: `schema_version: 1
removals:
  - target: mlxc
    name: mlx_flip
    reason: upstream renamed it
    resolution: re-expose under the upstream name in the next cut
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			waiverPath := filepath.Join(root, "codegen", "removals.yaml")
			os.Remove(waiverPath)
			if tt.waivers != "" {
				if err := os.WriteFile(waiverPath, []byte(tt.waivers), 0o666); err != nil {
					t.Fatal(err)
				}
			}
			opts := checkOptions{
				Options:           regenreport.Options{RepoRoot: root},
				LockPath:          lockPath,
				RemovalWaiverPath: "codegen/removals.yaml",
				Baseline:          "HEAD",
			}
			manifest := plan.Manifest{}
			manifest.Report.RequireLockMonotonicity = true
			checkGateErr(t, checkLockMonotonicity(opts, tt.cur, manifest), tt.wantErr)
		})
	}
}

func initGitRepo(t *testing.T, root, path string, content []byte) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, content, 0o666); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "gate@example.com"},
		{"config", "user.name", "gate"},
		{"add", path},
		{"commit", "-q", "-m", "baseline"},
	} {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

func checkGateErr(t *testing.T, err error, want string) {
	t.Helper()
	switch {
	case want == "" && err != nil:
		t.Fatalf("unexpected gate failure: %v", err)
	case want != "" && err == nil:
		t.Fatalf("gate passed, want failure containing %q", want)
	case want != "" && !strings.Contains(err.Error(), want):
		t.Fatalf("gate failure = %v, want it to contain %q", err, want)
	}
}
