package revision

import (
	"strings"
	"testing"
)

func TestNext(t *testing.T) {
	tests := []struct {
		name    string
		base    State
		core    string
		changed bool
		want    State
	}{
		{"initial", State{}, "0.32.0", true, State{"0.32.0", 1}},
		{"same cut", State{"0.32.0", 1}, "0.32.0", false, State{"0.32.0", 1}},
		{"changed cut", State{"0.32.0", 1}, "0.32.0", true, State{"0.32.0", 2}},
		{"new core", State{"0.32.0", 4}, "0.33.0", true, State{"0.33.0", 1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Next(tt.base, tt.core, tt.changed)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("Next() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestValidateTransitionRejectsReuseAndDecrement(t *testing.T) {
	base := State{Core: "0.32.0", Revision: 2}
	for _, next := range []State{
		{Core: "0.32.0", Revision: 2},
		{Core: "0.32.0", Revision: 1},
	} {
		if err := ValidateTransition(base, next, true); err == nil {
			t.Fatalf("ValidateTransition(%#v) succeeded", next)
		}
	}
}

func TestIdentityNames(t *testing.T) {
	tests := []struct {
		name       string
		sha        string
		generator  string
		library    string
		branchName string
	}{
		{
			name:       "stable",
			generator:  "mlx-v0.32.0-rev2",
			library:    "libs-v0.32.0-rev2",
			branchName: "build/mlx-v0.32.0-rev2",
		},
		{
			name:       "preview",
			sha:        "97ae2cfa020f258a222a452998fdce8ed81e3903",
			generator:  "mlx-v0.32.0-rev2-dev.97ae2cfa020f",
			library:    "libs-v0.32.0-rev2-dev.97ae2cfa020f",
			branchName: "build/mlx-v0.32.0-rev2-dev.97ae2cfa020f",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := NewIdentity(State{Core: "0.32.0", Revision: 2}, tt.sha)
			if err != nil {
				t.Fatal(err)
			}
			if got := id.GeneratorTag(); got != tt.generator {
				t.Errorf("GeneratorTag() = %q, want %q", got, tt.generator)
			}
			if got := id.LibraryTag(); got != tt.library {
				t.Errorf("LibraryTag() = %q, want %q", got, tt.library)
			}
			if got := id.CandidateBranch(); got != tt.branchName {
				t.Errorf("CandidateBranch() = %q, want %q", got, tt.branchName)
			}
		})
	}
}

func TestParseCoreVersion(t *testing.T) {
	const header = `
#define MLX_VERSION_MAJOR 0
#define MLX_VERSION_MINOR 32
#define MLX_VERSION_PATCH 1
`
	got, err := ParseCoreVersion(strings.NewReader(header))
	if err != nil {
		t.Fatal(err)
	}
	if got != "0.32.1" {
		t.Fatalf("ParseCoreVersion() = %q, want 0.32.1", got)
	}
}
