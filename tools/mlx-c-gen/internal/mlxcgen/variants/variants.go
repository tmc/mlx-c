// Package variants handles function overload disambiguation for MLX C bindings.
package variants

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/tmc/mlx-c/tools/mlx-c-gen/internal/mlxcgen/plan"
)

// VariantSuffix indicates what suffix (if any) to use for a function variant.
// nil means skip this variant, empty string means use base name, non-empty means use as suffix.
type VariantSuffix *string

// Func represents a parsed C++ function for variant selection.
type Func struct {
	Name         string
	Namespace    string
	ReturnType   string
	ParamTypes   []string
	ParamNames   []string
	ParamDefault []string
	Doc          string
	Variant      string // Set by variant selection
	VariantIndex int    // Index in variant list for sorting
	File         string
	Line         int
	Col          int
}

// Diagnostic records a variant-selection decision.
type Diagnostic struct {
	Code    string
	Message string
	Reason  string
	Func    *Func
}

// PrettyString returns a human-readable function signature.
func (f *Func) PrettyString() string {
	var parts []string
	parts = append(parts, f.ReturnType)
	parts = append(parts, f.Namespace+"::"+f.Name)
	parts = append(parts, "(")

	var args []string
	for i := range f.ParamTypes {
		name := ""
		if i < len(f.ParamNames) {
			name = f.ParamNames[i]
		}
		args = append(args, f.ParamTypes[i]+" "+name)
	}
	parts = append(parts, strings.Join(args, ", "))
	parts = append(parts, ")")
	return strings.Join(parts, " ")
}

// Signature returns the canonical overload key used by generator policy.
func (f *Func) Signature() string {
	return f.ReturnType + "(" + strings.Join(f.ParamTypes, ", ") + ")"
}

type variantPolicy struct {
	mappings           map[string]map[string]map[string]variantRule
	allowedDetailFuncs map[string]bool
}

type variantRule struct {
	suffix VariantSuffix
	index  int
	doc    string
	reason string
}

// Selector applies a manifest's overload selection policy.
type Selector struct {
	policy variantPolicy
}

// NewSelector returns a selector backed by manifest.
func NewSelector(manifest plan.Manifest) *Selector {
	return &Selector{
		policy: policyFromManifest(manifest),
	}
}

var (
	defaultPolicyOnce sync.Once
	defaultPolicy     variantPolicy
	defaultPolicyErr  error
)

func loadDefaultPolicy() (variantPolicy, error) {
	defaultPolicyOnce.Do(func() {
		manifest, err := plan.Default()
		if err != nil {
			defaultPolicyErr = err
			return
		}
		defaultPolicy = policyFromManifest(manifest)
	})
	return defaultPolicy, defaultPolicyErr
}

func policyFromManifest(manifest plan.Manifest) variantPolicy {
	return variantPolicy{
		mappings:           variantMappingsFromManifest(manifest.VariantMappings),
		allowedDetailFuncs: manifest.AllowedDetailFunctionsSet(),
	}
}

func variantMappingsFromManifest(in map[string]map[string][]plan.Variant) map[string]map[string]map[string]variantRule {
	out := make(map[string]map[string]map[string]variantRule, len(in))
	for namespace, funcs := range in {
		out[namespace] = make(map[string]map[string]variantRule, len(funcs))
		for name, variants := range funcs {
			out[namespace][name] = variantRulesFromManifest(variants)
		}
	}
	return out
}

func variantRulesFromManifest(in []plan.Variant) map[string]variantRule {
	out := make(map[string]variantRule, len(in))
	index := 0
	for _, variant := range in {
		var suffix VariantSuffix
		if !variant.Skip {
			s := *variant.Suffix
			suffix = &s
		}
		out[variant.Signature] = variantRule{
			suffix: suffix,
			index:  index,
			doc:    variant.Doc,
			reason: variant.Reason,
		}
		if !variant.Skip {
			index++
		}
	}
	return out
}

// SelectVariants filters and assigns variant suffixes to function definitions.
// It returns the functions that should be included in the bindings.
func SelectVariants(namespace, name string, defs []*Func) ([]*Func, error) {
	selected, _, err := SelectVariantsWithDiagnostics(namespace, name, defs)
	return selected, err
}

// SelectVariantsWithDiagnostics filters and assigns variant suffixes to
// function definitions, and reports selected-out definitions.
func SelectVariantsWithDiagnostics(namespace, name string, defs []*Func) ([]*Func, []Diagnostic, error) {
	policy, err := loadDefaultPolicy()
	if err != nil {
		return nil, nil, err
	}
	return selectVariantsWithPolicy(policy, namespace, name, defs)
}

// SelectWithDiagnostics filters and assigns variant suffixes using s.
func (s *Selector) SelectWithDiagnostics(namespace, name string, defs []*Func) ([]*Func, []Diagnostic, error) {
	if s == nil {
		return SelectVariantsWithDiagnostics(namespace, name, defs)
	}
	return selectVariantsWithPolicy(s.policy, namespace, name, defs)
}

func selectVariantsWithPolicy(policy variantPolicy, namespace, name string, defs []*Func) ([]*Func, []Diagnostic, error) {
	nsKey := strings.ReplaceAll(namespace, "::", "_")

	// Special handling for detail namespace
	if nsKey == "mlx_core_detail" {
		if !policy.allowedDetailFuncs[name] {
			return nil, diagnosticsForSkipped("skip_unallowed_detail_function", defs), nil
		}
		if len(defs) > 0 {
			return []*Func{defs[0]}, diagnosticsForSkipped("skip_allowed_detail_overload", defs[1:]), nil
		}
		return nil, nil, nil
	}

	variants, hasVariants := policy.mappings[nsKey]
	if !hasVariants {
		if len(defs) > 1 {
			return nil, nil, unmappedOverloadError(namespace, name, defs)
		}
		if len(defs) > 0 {
			return []*Func{defs[0]}, diagnosticsForImplicitSingle(defs), nil
		}
		return nil, nil, nil
	}

	funcVariants, hasFunc := variants[name]
	if !hasFunc {
		if len(defs) > 1 {
			return nil, nil, unmappedOverloadError(namespace, name, defs)
		}
		if len(defs) > 0 {
			return []*Func{defs[0]}, diagnosticsForImplicitSingle(defs), nil
		}
		return nil, nil, nil
	}

	var result []*Func
	var diagnostics []Diagnostic
	seen := map[string]bool{}
	for _, d := range defs {
		signature := d.Signature()
		rule, ok := funcVariants[signature]
		if !ok {
			return nil, nil, unmappedSignatureError(namespace, name, d, funcVariants)
		}
		seen[signature] = true
		if rule.suffix == nil {
			message := fmt.Sprintf("%s skipped by variant mapping", d.PrettyString())
			if rule.reason != "" {
				message += ": " + rule.reason
			}
			diagnostics = append(diagnostics, Diagnostic{
				Code:    "skip_variant_mapping",
				Message: message,
				Reason:  rule.reason,
				Func:    d,
			})
			continue
		}
		if *rule.suffix != "" {
			d.Variant = *rule.suffix
		}
		if rule.doc != "" {
			d.Doc = rule.doc
		}
		d.VariantIndex = rule.index
		result = append(result, d)
	}
	if len(seen) != len(funcVariants) {
		return nil, nil, missingSignatureError(namespace, name, seen, funcVariants)
	}

	return result, diagnostics, nil
}

func diagnosticsForSkipped(code string, defs []*Func) []Diagnostic {
	var diagnostics []Diagnostic
	for _, d := range defs {
		diagnostics = append(diagnostics, Diagnostic{
			Code:    code,
			Message: fmt.Sprintf("%s skipped by variant selection", d.PrettyString()),
			Reason:  variantDiagnosticReason(code),
			Func:    d,
		})
	}
	return diagnostics
}

func variantDiagnosticReason(code string) string {
	switch code {
	case "skip_allowed_detail_overload":
		return "covered_by_other_variant"
	case "skip_unallowed_detail_function":
		return "internal_namespace"
	default:
		return ""
	}
}

func diagnosticsForImplicitSingle(defs []*Func) []Diagnostic {
	var diagnostics []Diagnostic
	for _, d := range defs {
		diagnostics = append(diagnostics, Diagnostic{
			Code:    "emit_implicit_single_overload",
			Message: fmt.Sprintf("%s selected by implicit single-overload fallback", d.PrettyString()),
			Func:    d,
		})
	}
	return diagnostics
}

func unmappedOverloadError(namespace, name string, defs []*Func) error {
	var b strings.Builder
	fmt.Fprintf(&b, "unmapped overloads for %s::%s", namespace, name)
	for i, d := range defs {
		fmt.Fprintf(&b, "\n  %d %s", i, d.PrettyString())
	}
	return fmt.Errorf("%s", b.String())
}

func unmappedSignatureError(namespace, name string, f *Func, rules map[string]variantRule) error {
	var b strings.Builder
	fmt.Fprintf(&b, "unmapped overload signature for %s::%s: %s", namespace, name, f.Signature())
	fmt.Fprintf(&b, "\noverload: %s", f.PrettyString())
	writeKnownSignatures(&b, rules)
	return fmt.Errorf("%s", b.String())
}

func missingSignatureError(namespace, name string, seen map[string]bool, rules map[string]variantRule) error {
	var b strings.Builder
	fmt.Fprintf(&b, "variant mapping signatures missing from parsed overloads for %s::%s", namespace, name)
	for _, signature := range sortedRuleSignatures(rules) {
		if !seen[signature] {
			fmt.Fprintf(&b, "\n  %s", signature)
		}
	}
	return fmt.Errorf("%s", b.String())
}

func writeKnownSignatures(b *strings.Builder, rules map[string]variantRule) {
	fmt.Fprintf(b, "\nknown signatures:")
	for _, signature := range sortedRuleSignatures(rules) {
		fmt.Fprintf(b, "\n  %s", signature)
	}
}

func sortedRuleSignatures(rules map[string]variantRule) []string {
	var signatures []string
	for signature := range rules {
		signatures = append(signatures, signature)
	}
	sort.Strings(signatures)
	return signatures
}
