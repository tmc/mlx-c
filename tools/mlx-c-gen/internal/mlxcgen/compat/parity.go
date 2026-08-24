package compat

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/tmc/mlx-c/tools/mlx-c-gen/internal/mlxcgen/apilock"
)

// ParityResult summarizes a parity comparison against upstream.
type ParityResult struct {
	Shared    int      // functions declared by both surfaces
	OursOnly  int      // additive functions, exempt from the gate
	Waived    int      // shared functions diverging under a declared waiver
	Problems  []string // gate failures, in report order
	Divergent []string // every shared function that diverges, waived or not
}

// OK reports whether the parity gate passes.
func (r ParityResult) OK() bool { return len(r.Problems) == 0 }

// Err returns the gate failure, or nil.
func (r ParityResult) Err() error {
	if r.OK() {
		return nil
	}
	return fmt.Errorf("upstream parity: %d problem(s):\n\t%s",
		len(r.Problems), strings.Join(r.Problems, "\n\t"))
}

// CheckParity compares the functions ours and upstream have in common. A
// difference in return type or parameter types fails the gate unless waivers
// declares it. Functions only ours declares are additive and ignored;
// functions only upstream declares are not this gate's concern.
func CheckParity(ours, upstream apilock.Target, waivers ParityWaivers) ParityResult {
	var res ParityResult
	up := make(map[string]apilock.Function, len(upstream.Functions))
	for _, fn := range upstream.Functions {
		up[fn.Name] = fn
	}
	declared := make(map[string]Divergence, len(waivers.Divergences))
	for _, d := range waivers.Divergences {
		declared[d.Name] = d
	}
	used := make(map[string]bool, len(declared))

	for _, fn := range ours.Functions {
		other, ok := up[fn.Name]
		if !ok {
			res.OursOnly++
			continue
		}
		res.Shared++
		if compatibleDecl(fn, other) {
			continue
		}
		res.Divergent = append(res.Divergent, fn.Name)
		d, waived := declared[fn.Name]
		if !waived {
			res.Problems = append(res.Problems, fmt.Sprintf(
				"%s: undeclared divergence from upstream\n\t\tours:     %s\n\t\tupstream: %s",
				fn.Name, fn.Signature, other.Signature))
			continue
		}
		used[fn.Name] = true
		if normalizeDecl(d.Ours) != normalizeDecl(fn.Signature) ||
			normalizeDecl(d.Upstream) != normalizeDecl(other.Signature) {
			res.Problems = append(res.Problems, fmt.Sprintf(
				"%s: waiver no longer matches the declarations it describes\n"+
					"\t\twaiver ours:     %s\n\t\tactual ours:     %s\n"+
					"\t\twaiver upstream: %s\n\t\tactual upstream: %s",
				fn.Name, d.Ours, fn.Signature, d.Upstream, other.Signature))
			continue
		}
		res.Waived++
	}

	var stale []string
	for name := range declared {
		if !used[name] {
			stale = append(stale, name)
		}
	}
	sort.Strings(stale)
	for _, name := range stale {
		res.Problems = append(res.Problems, fmt.Sprintf(
			"%s: waiver declares a divergence that no longer exists; delete it", name))
	}
	return res
}

// compatibleDecl reports whether two declarations of the same function agree on
// everything that a caller can observe: return type, parameter count, and
// parameter types in order. Parameter names are not part of the interface.
func compatibleDecl(a, b apilock.Function) bool {
	if normalizeDecl(a.Return) != normalizeDecl(b.Return) {
		return false
	}
	if len(a.Parameters) != len(b.Parameters) {
		return false
	}
	for i := range a.Parameters {
		if paramType(a.Parameters[i]) != paramType(b.Parameters[i]) {
			return false
		}
	}
	return true
}

// paramNameRE matches a trailing parameter name in a C parameter declaration.
// Declarations that end in ')' — function pointers and array declarators —
// deliberately do not match and are compared whole.
var paramNameRE = regexp.MustCompile(`^(.*[\s*])([A-Za-z_][A-Za-z0-9_]*)$`)

// paramType strips the parameter name from a C parameter declaration.
func paramType(param string) string {
	param = normalizeDecl(param)
	m := paramNameRE.FindStringSubmatch(param)
	if m == nil {
		return param
	}
	rest := normalizeDecl(strings.TrimRight(m[1], " "))
	if onlyQualifiers(rest) {
		// The trailing identifier was the type, not a name: "const mlx_array".
		return param
	}
	return rest
}

var qualifiers = map[string]bool{
	"const": true, "volatile": true, "restrict": true,
	"signed": true, "unsigned": true, "struct": true, "enum": true, "union": true,
	"long": true, "short": true,
}

func onlyQualifiers(decl string) bool {
	for _, word := range strings.Fields(decl) {
		if !qualifiers[word] {
			return false
		}
	}
	return true
}
