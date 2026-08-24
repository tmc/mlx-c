// Package doccoverage reports missing documentation sources for generated API.
package doccoverage

import (
	"sort"
	"strings"

	"github.com/tmc/mlx-c/tools/mlx-c-gen/internal/mlxcgen/hooks"
	"github.com/tmc/mlx-c/tools/mlx-c-gen/internal/mlxcgen/ir"
	"github.com/tmc/mlx-c/tools/mlx-c-gen/internal/mlxcgen/plan"
	"github.com/tmc/mlx-c/tools/mlx-c-gen/internal/mlxcgen/types"
)

// Coverage counts generated C declarations with and without documentation.
type Coverage struct {
	Exported int `json:"exported"`
	WithDoc  int `json:"with_doc"`
	Missing  int `json:"missing"`
}

// MissingDoc records one generated C declaration without a doc source.
type MissingDoc struct {
	Module    string `json:"module"`
	Header    string `json:"header,omitempty"`
	Namespace string `json:"namespace"`
	Function  string `json:"function"`
	Signature string `json:"signature"`
	CName     string `json:"c_name"`
	Action    string `json:"action"`
	File      string `json:"file,omitempty"`
	Line      int    `json:"line,omitempty"`
	Col       int    `json:"col,omitempty"`
}

// Analyze reports selected generated function declarations that have no IR
// documentation source.
func Analyze(manifest plan.Manifest, result ir.Result) (Coverage, []MissingDoc) {
	decls := funcsByDecisionKey(result.Functions)
	registry := types.NewRegistry()
	var coverage Coverage
	var missing []MissingDoc

	namespaces := make([]string, 0, len(manifest.VariantMappings))
	for namespace := range manifest.VariantMappings {
		namespaces = append(namespaces, namespace)
	}
	sort.Strings(namespaces)

	for _, namespace := range namespaces {
		funcs := manifest.VariantMappings[namespace]
		names := make([]string, 0, len(funcs))
		for name := range funcs {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			for _, variant := range funcs[name] {
				if variant.Skip {
					continue
				}
				suffix := ""
				if variant.Suffix != nil {
					suffix = *variant.Suffix
				}
				cname := cName(namespace, name, suffix)
				action := "emit"
				if hooks.HasHook(cname) {
					action = "hook"
				}
				for _, fn := range decls[decisionKey(namespace, name, variant.Signature)] {
					if action != "hook" && !supported(registry, fn) {
						continue
					}
					coverage.Exported++
					if strings.TrimSpace(variant.Doc) != "" || strings.TrimSpace(fn.Comment) != "" {
						coverage.WithDoc++
						continue
					}
					coverage.Missing++
					missing = append(missing, MissingDoc{
						Module:    fn.Module,
						Header:    fn.Header,
						Namespace: fn.Namespace,
						Function:  fn.Name,
						Signature: variant.Signature,
						CName:     cname,
						Action:    action,
						File:      fn.Loc.File,
						Line:      fn.Loc.Line,
						Col:       fn.Loc.Col,
					})
				}
			}
		}
	}

	sort.Slice(missing, func(i, j int) bool {
		if missing[i].CName != missing[j].CName {
			return missing[i].CName < missing[j].CName
		}
		if missing[i].File != missing[j].File {
			return missing[i].File < missing[j].File
		}
		return missing[i].Signature < missing[j].Signature
	})
	return coverage, missing
}

func funcsByDecisionKey(funcs []ir.FuncDecl) map[string][]ir.FuncDecl {
	out := map[string][]ir.FuncDecl{}
	for _, fn := range funcs {
		key := decisionKey(strings.ReplaceAll(fn.Namespace, "::", "_"), fn.Name, signature(fn))
		out[key] = append(out[key], fn)
	}
	return out
}

func decisionKey(namespace, name, signature string) string {
	return namespace + "|" + name + "|" + signature
}

func signature(fn ir.FuncDecl) string {
	params := make([]string, 0, len(fn.Params))
	for _, param := range fn.Params {
		params = append(params, param.Type)
	}
	return fn.Return + "(" + strings.Join(params, ", ") + ")"
}

func supported(registry *types.Registry, fn ir.FuncDecl) bool {
	if registry.FindByCpp(fn.Return) == nil {
		return false
	}
	for _, param := range fn.Params {
		if registry.FindByCpp(param.Type) == nil {
			return false
		}
	}
	return true
}

func cName(namespace, name, suffix string) string {
	prefix := cPrefix(namespace)
	cname := prefix + "_" + name
	if suffix != "" {
		cname += "_" + suffix
	}
	return cname
}

func cPrefix(namespace string) string {
	parts := strings.Split(namespace, "_")
	if len(parts) >= 2 && parts[0] == "mlx" && parts[1] == "core" {
		parts = append(parts[:1], parts[2:]...)
		if len(parts) == 2 && parts[1] == "cu" {
			parts[1] = "cuda"
		}
	}
	return strings.Join(parts, "_")
}
