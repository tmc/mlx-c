package compat

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tmc/mlx-c/tools/mlx-c-gen/internal/mlxcgen/apilock"
)

// MonotonicResult summarizes a symbol-set comparison against a baseline lock.
type MonotonicResult struct {
	Added    []string // target/name, newly locked
	Removed  []string // target/name, dropped without a waiver
	Waived   []string // target/name, dropped under a declared waiver
	Problems []string
}

// OK reports whether the monotonicity gate passes.
func (r MonotonicResult) OK() bool { return len(r.Problems) == 0 }

// Err returns the gate failure, or nil.
func (r MonotonicResult) Err() error {
	if r.OK() {
		return nil
	}
	return fmt.Errorf("lock monotonicity: %d problem(s):\n\t%s",
		len(r.Problems), strings.Join(r.Problems, "\n\t"))
}

// CheckMonotonic reports the functions that baseline locked and cur no longer
// does. Every removal must be declared in waivers; a waiver for a function
// that is still present, or for a target that no longer exists, is stale and
// fails the gate too. Additions are recorded but never fail: the surface is
// allowed to grow.
func CheckMonotonic(baseline, cur *apilock.Lock, waivers RemovalWaivers) MonotonicResult {
	var res MonotonicResult
	before := lockFunctions(baseline)
	after := lockFunctions(cur)

	declared := make(map[string]bool, len(waivers.Removals))
	for _, r := range waivers.Removals {
		declared[r.Target+"/"+r.Name] = true
	}
	used := make(map[string]bool, len(declared))

	for _, key := range sortedKeys(before) {
		if after[key] {
			continue
		}
		if declared[key] {
			used[key] = true
			res.Waived = append(res.Waived, key)
			continue
		}
		res.Removed = append(res.Removed, key)
		res.Problems = append(res.Problems, fmt.Sprintf(
			"%s: removed from the API lock without a removals waiver", key))
	}
	for _, key := range sortedKeys(after) {
		if !before[key] {
			res.Added = append(res.Added, key)
		}
	}

	var stale []string
	for key := range declared {
		if !used[key] {
			stale = append(stale, key)
		}
	}
	sort.Strings(stale)
	for _, key := range stale {
		res.Problems = append(res.Problems, fmt.Sprintf(
			"%s: removals waiver for a function that was not removed; delete it", key))
	}
	return res
}

func lockFunctions(lock *apilock.Lock) map[string]bool {
	out := map[string]bool{}
	if lock == nil {
		return out
	}
	for target, t := range lock.Targets {
		for _, fn := range t.Functions {
			out[target+"/"+fn.Name] = true
		}
	}
	return out
}

func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for key := range set {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}
