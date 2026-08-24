package compat

import (
	"reflect"
	"strings"
	"testing"

	"github.com/tmc/mlx-c/tools/mlx-c-gen/internal/mlxcgen/apilock"
)

func lockOf(names ...string) *apilock.Lock {
	t := apilock.Target{}
	for _, name := range names {
		t.Functions = append(t.Functions, apilock.Function{Name: name})
	}
	return &apilock.Lock{Targets: map[string]apilock.Target{"mlxc": t}}
}

func TestCheckMonotonic(t *testing.T) {
	tests := []struct {
		name     string
		baseline *apilock.Lock
		cur      *apilock.Lock
		waivers  RemovalWaivers
		want     []string
		added    []string
		waived   []string
	}{
		{
			name:     "unchanged",
			baseline: lockOf("mlx_add", "mlx_flip"),
			cur:      lockOf("mlx_add", "mlx_flip"),
		},
		{
			name:     "growth is always allowed",
			baseline: lockOf("mlx_add"),
			cur:      lockOf("mlx_add", "mlx_event_new"),
			added:    []string{"mlxc/mlx_event_new"},
		},
		{
			name:     "silent removal fails",
			baseline: lockOf("mlx_add", "mlx_flip"),
			cur:      lockOf("mlx_add"),
			want:     []string{"mlxc/mlx_flip: removed from the API lock without a removals waiver"},
		},
		{
			name:     "several removals are all reported",
			baseline: lockOf("mlx_count_nonzero", "mlx_flip", "mlx_unstack"),
			cur:      lockOf(),
			want: []string{
				"mlxc/mlx_count_nonzero: removed",
				"mlxc/mlx_flip: removed",
				"mlxc/mlx_unstack: removed",
			},
		},
		{
			name:     "waived removal passes",
			baseline: lockOf("mlx_add", "mlx_flip"),
			cur:      lockOf("mlx_add"),
			waivers: RemovalWaivers{Removals: []Removal{
				{Target: "mlxc", Name: "mlx_flip"},
			}},
			waived: []string{"mlxc/mlx_flip"},
		},
		{
			name:     "waiver is scoped to its target",
			baseline: lockOf("mlx_flip"),
			cur:      lockOf(),
			waivers: RemovalWaivers{Removals: []Removal{
				{Target: "jacclc", Name: "mlx_flip"},
			}},
			want: []string{
				"mlxc/mlx_flip: removed",
				"jacclc/mlx_flip: removals waiver for a function that was not removed",
			},
		},
		{
			name:     "waiver for a still-present function is stale",
			baseline: lockOf("mlx_add"),
			cur:      lockOf("mlx_add"),
			waivers: RemovalWaivers{Removals: []Removal{
				{Target: "mlxc", Name: "mlx_add"},
			}},
			want: []string{"mlxc/mlx_add: removals waiver for a function that was not removed"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckMonotonic(tt.baseline, tt.cur, tt.waivers)
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
			if len(got.Added)+len(tt.added) > 0 && !reflect.DeepEqual(got.Added, tt.added) {
				t.Errorf("Added = %v, want %v", got.Added, tt.added)
			}
			if len(got.Waived)+len(tt.waived) > 0 && !reflect.DeepEqual(got.Waived, tt.waived) {
				t.Errorf("Waived = %v, want %v", got.Waived, tt.waived)
			}
		})
	}
}
