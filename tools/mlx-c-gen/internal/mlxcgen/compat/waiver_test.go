package compat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o666); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadParityWaivers(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int
		wantErr string
	}{
		{
			name: "complete entry",
			content: `schema_version: 1
divergences:
  - name: mlx_astype
    ours: "int mlx_astype(mlx_array* res)"
    upstream: "int mlx_astype(mlx_array* res, const mlx_stream s)"
    reason: our fork drops the stream parameter
    resolution: emit an upstream-compatible overload in Stage 1
`,
			want: 1,
		},
		{
			name:    "wrong schema version",
			content: "schema_version: 2\n",
			wantErr: "schema_version 2, want 1",
		},
		{
			name: "unknown field",
			content: `schema_version: 1
divergences:
  - name: mlx_astype
    excuse: because
`,
			wantErr: "field excuse not found",
		},
		{
			name: "missing resolution",
			content: `schema_version: 1
divergences:
  - name: mlx_astype
    ours: a
    upstream: b
    reason: c
`,
			wantErr: "divergence mlx_astype: missing resolution",
		},
		{
			name: "duplicate entry",
			content: `schema_version: 1
divergences:
  - {name: mlx_astype, ours: a, upstream: b, reason: c, resolution: d}
  - {name: mlx_astype, ours: a, upstream: b, reason: c, resolution: d}
`,
			wantErr: "divergence mlx_astype: declared twice",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LoadParityWaivers(writeFile(t, "compat-waivers.yaml", tt.content))
			checkWaiverErr(t, err, tt.wantErr)
			if tt.wantErr == "" && len(got.Divergences) != tt.want {
				t.Errorf("got %d divergences, want %d", len(got.Divergences), tt.want)
			}
		})
	}
}

func TestLoadRemovalWaivers(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int
		wantErr string
	}{
		{
			name: "complete entry",
			content: `schema_version: 1
removals:
  - target: mlxc
    name: mlx_flip
    reason: overload dropped by an explicit manifest policy call
    resolution: re-bind in the 0.32 cut; tracked as excluded_by_policy
`,
			want: 1,
		},
		{
			name: "missing target",
			content: `schema_version: 1
removals:
  - name: mlx_flip
    reason: a
    resolution: b
`,
			wantErr: "removal 0: missing target",
		},
		{
			name: "missing reason",
			content: `schema_version: 1
removals:
  - {target: mlxc, name: mlx_flip, resolution: b}
`,
			wantErr: "removal mlxc/mlx_flip: missing reason",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LoadRemovalWaivers(writeFile(t, "removals.yaml", tt.content))
			checkWaiverErr(t, err, tt.wantErr)
			if tt.wantErr == "" && len(got.Removals) != tt.want {
				t.Errorf("got %d removals, want %d", len(got.Removals), tt.want)
			}
		})
	}
}

// A missing waiver file is an empty waiver set, so a tree with no declared
// divergences and no declared removals still gates.
func TestLoadWaiversMissingFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "absent.yaml")
	parity, err := LoadParityWaivers(missing)
	if err != nil || len(parity.Divergences) != 0 {
		t.Errorf("LoadParityWaivers(missing) = %v, %v", parity, err)
	}
	removals, err := LoadRemovalWaivers(missing)
	if err != nil || len(removals.Removals) != 0 {
		t.Errorf("LoadRemovalWaivers(missing) = %v, %v", removals, err)
	}
}

func checkWaiverErr(t *testing.T, err error, want string) {
	t.Helper()
	switch {
	case want == "" && err != nil:
		t.Fatalf("unexpected error: %v", err)
	case want != "" && err == nil:
		t.Fatalf("got nil error, want %q", want)
	case want != "" && !strings.Contains(err.Error(), want):
		t.Fatalf("error = %v, want it to contain %q", err, want)
	}
}
