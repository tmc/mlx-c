package compat

import (
	"strings"
	"testing"

	"github.com/tmc/mlx-c/tools/mlx-c-gen/internal/mlxcgen/apilock"
)

func fn(name, ret string, params ...string) apilock.Function {
	return apilock.Function{
		Name:       name,
		Return:     ret,
		Parameters: params,
		Signature:  ret + " " + name + "(" + strings.Join(params, ", ") + ")",
	}
}

func target(fns ...apilock.Function) apilock.Target {
	return apilock.Target{Functions: fns}
}

func TestCheckParity(t *testing.T) {
	astypeOurs := fn("mlx_astype", "int", "mlx_array* res", "const mlx_array a", "mlx_dtype dtype")
	astypeUpstream := fn("mlx_astype", "int", "mlx_array* res", "const mlx_array a", "mlx_dtype dtype", "const mlx_stream s")

	tests := []struct {
		name    string
		ours    apilock.Target
		up      apilock.Target
		waivers ParityWaivers
		want    []string // substrings every problem list must contain, in order
		shared  int
		extras  int
		waived  int
	}{
		{
			name:   "identical",
			ours:   target(fn("mlx_add", "int", "mlx_array* res", "const mlx_array a")),
			up:     target(fn("mlx_add", "int", "mlx_array* res", "const mlx_array a")),
			shared: 1,
		},
		{
			name:   "parameter names are not part of the interface",
			ours:   target(fn("mlx_add", "int", "mlx_array* res", "const mlx_array lhs")),
			up:     target(fn("mlx_add", "int", "mlx_array*  out", "const mlx_array a")),
			shared: 1,
		},
		{
			name:   "our extras are additive",
			ours:   target(fn("mlx_event_new", "int", "mlx_event* res")),
			up:     target(),
			extras: 1,
		},
		{
			name:   "upstream extras are not this gate's concern",
			ours:   target(),
			up:     target(fn("mlx_new_thing", "int")),
			shared: 0,
		},
		{
			name:   "return type divergence",
			ours:   target(fn("mlx_add", "void", "const mlx_array a")),
			up:     target(fn("mlx_add", "int", "const mlx_array a")),
			shared: 1,
			want:   []string{"mlx_add: undeclared divergence"},
		},
		{
			name:   "parameter count divergence",
			ours:   target(astypeOurs),
			up:     target(astypeUpstream),
			shared: 1,
			want:   []string{"mlx_astype: undeclared divergence"},
		},
		{
			name:   "parameter type divergence",
			ours:   target(fn("mlx_add", "int", "const mlx_array a")),
			up:     target(fn("mlx_add", "int", "const mlx_stream a")),
			shared: 1,
			want:   []string{"mlx_add: undeclared divergence"},
		},
		{
			name: "waived divergence passes",
			ours: target(astypeOurs),
			up:   target(astypeUpstream),
			waivers: ParityWaivers{Divergences: []Divergence{{
				Name:     "mlx_astype",
				Ours:     astypeOurs.Signature,
				Upstream: astypeUpstream.Signature,
			}}},
			shared: 1,
			waived: 1,
		},
		{
			name: "waiver tolerates reformatting",
			ours: target(astypeOurs),
			up:   target(astypeUpstream),
			waivers: ParityWaivers{Divergences: []Divergence{{
				Name:     "mlx_astype",
				Ours:     "int mlx_astype(mlx_array* res,\n    const mlx_array a,\n    mlx_dtype dtype)",
				Upstream: astypeUpstream.Signature,
			}}},
			shared: 1,
			waived: 1,
		},
		{
			name: "waiver describing a different divergence is stale",
			ours: target(astypeOurs),
			up:   target(astypeUpstream),
			waivers: ParityWaivers{Divergences: []Divergence{{
				Name:     "mlx_astype",
				Ours:     "int mlx_astype(mlx_array* res)",
				Upstream: astypeUpstream.Signature,
			}}},
			shared: 1,
			want:   []string{"mlx_astype: waiver no longer matches"},
		},
		{
			name: "waiver for a symbol that agrees is stale",
			ours: target(fn("mlx_add", "int", "const mlx_array a")),
			up:   target(fn("mlx_add", "int", "const mlx_array a")),
			waivers: ParityWaivers{Divergences: []Divergence{{
				Name: "mlx_add", Ours: "x", Upstream: "y",
			}}},
			shared: 1,
			want:   []string{"mlx_add: waiver declares a divergence that no longer exists"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckParity(tt.ours, tt.up, tt.waivers)
			if got.OK() != (len(tt.want) == 0) {
				t.Fatalf("OK() = %v, problems = %v", got.OK(), got.Problems)
			}
			if len(got.Problems) != len(tt.want) {
				t.Fatalf("got %d problems %v, want %d", len(got.Problems), got.Problems, len(tt.want))
			}
			for i, want := range tt.want {
				if !strings.Contains(got.Problems[i], want) {
					t.Errorf("problem %d = %q, want it to contain %q", i, got.Problems[i], want)
				}
			}
			if got.Shared != tt.shared {
				t.Errorf("Shared = %d, want %d", got.Shared, tt.shared)
			}
			if got.OursOnly != tt.extras {
				t.Errorf("OursOnly = %d, want %d", got.OursOnly, tt.extras)
			}
			if got.Waived != tt.waived {
				t.Errorf("Waived = %d, want %d", got.Waived, tt.waived)
			}
		})
	}
}

func TestParamType(t *testing.T) {
	tests := []struct{ param, want string }{
		{"const mlx_array a", "const mlx_array"},
		{"mlx_array*  res", "mlx_array*"},
		{"const int* shape", "const int*"},
		{"size_t shape_num", "size_t"},
		{"void", "void"},
		{"const mlx_array", "const mlx_array"}, // unnamed: the tail is the type
		{"int (*fn)(void*)", "int (*fn)(void*)"},
	}
	for _, tt := range tests {
		if got := paramType(tt.param); got != tt.want {
			t.Errorf("paramType(%q) = %q, want %q", tt.param, got, tt.want)
		}
	}
}

func TestNormalizeDeclPointerStarPlacement(t *testing.T) {
	cases := []struct{ a, b string }{
		{"mlx_array* res", "mlx_array *res"},
		{"const int* axes", "const int * axes"},
		{"char** res", "char **res"},
		{"int mlx_sum(mlx_array* res, bool keepdims)", "int mlx_sum(mlx_array *res, bool keepdims)"},
	}
	for _, tc := range cases {
		if normalizeDecl(tc.a) != normalizeDecl(tc.b) {
			t.Errorf("normalizeDecl(%q)=%q != normalizeDecl(%q)=%q", tc.a, normalizeDecl(tc.a), tc.b, normalizeDecl(tc.b))
		}
	}
}
