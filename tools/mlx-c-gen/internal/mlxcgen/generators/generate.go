// Package generators provides code generation for MLX C bindings.
package generators

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/tmc/mlx-c/tools/mlx-c-gen/internal/mlxcgen/hooks"
	"github.com/tmc/mlx-c/tools/mlx-c-gen/internal/mlxcgen/ir"
	"github.com/tmc/mlx-c/tools/mlx-c-gen/internal/mlxcgen/parser"
	"github.com/tmc/mlx-c/tools/mlx-c-gen/internal/mlxcgen/plan"
	"github.com/tmc/mlx-c/tools/mlx-c-gen/internal/mlxcgen/types"
	"github.com/tmc/mlx-c/tools/mlx-c-gen/internal/mlxcgen/variants"
)

// Generator generates C bindings from parsed C++ headers.
type Generator struct {
	types    *types.Registry
	variants *variants.Selector

	// emitDocs controls whether doc comments are written above generated
	// declarations. The zero value emits docs; use SetEmitDocs(false) to omit them.
	emitDocs bool
}

var (
	snakeCapRe   = regexp.MustCompile(`([A-Z]+)([A-Z][a-z])`)
	snakeLowerRe = regexp.MustCompile(`([a-z0-9])([A-Z])`)
)

// New creates a new Generator.
func New() *Generator {
	return &Generator{
		types:    types.NewRegistry(),
		emitDocs: true,
	}
}

// SetEmitDocs controls whether doc comments are emitted above declarations.
func (g *Generator) SetEmitDocs(v bool) { g.emitDocs = v }

// NewWithManifest creates a generator that uses manifest policy.
func NewWithManifest(manifest plan.Manifest) *Generator {
	return &Generator{
		types:    types.NewRegistry(),
		variants: variants.NewSelector(manifest),
	}
}

// Generate generates C bindings for the given parsed result.
func (g *Generator) Generate(w io.Writer, result *parser.ParseResult, headerName string, headers []string, impl bool, docstring string) error {
	allFuncs, _, err := g.selectFunctions(result)
	if err != nil {
		return err
	}

	// Write header
	g.writeHeader(w, headerName, headers, impl, docstring)

	// Write enums (only in header)
	if !impl {
		for _, enum := range result.Enums {
			g.writeEnum(w, enum)
		}
	}

	// Write functions
	for _, f := range allFuncs {
		g.writeFunction(w, f, impl)
	}

	// Write footer
	g.writeFooter(w, impl, docstring)

	return nil
}

// Diagnostics returns metadata diagnostics for selected functions the generator
// cannot emit with the current type registry.
func (g *Generator) Diagnostics(result *parser.ParseResult) []parser.Diagnostic {
	allFuncs, diagnostics, err := g.selectFunctions(result)
	if err != nil {
		diagnostics = append(diagnostics, parser.Diagnostic{
			Code:    "variant_selection_error",
			Message: err.Error(),
		})
		return diagnostics
	}
	for _, f := range allFuncs {
		funcName := cNamespace(f.Namespace) + "_" + f.Name
		if f.Variant != "" {
			funcName += "_" + f.Variant
		}
		if hooks.GetHook(funcName) != nil {
			continue
		}
		for _, unsupported := range g.unsupportedTypes(f) {
			diagnostics = append(diagnostics, parser.Diagnostic{
				Code:    unsupported.code,
				Message: unsupported.message,
				Reason:  "unsupported_type",
				File:    f.File,
				Line:    f.Line,
				Col:     f.Col,
			})
		}
	}
	return diagnostics
}

// SelectedGeneratedIR returns selected declarations emitted through the normal
// type registry. Hook-backed declarations are intentionally excluded.
func (g *Generator) SelectedGeneratedIR(module string, result *parser.ParseResult) (ir.Result, error) {
	allFuncs, _, err := g.selectFunctions(result)
	if err != nil {
		return ir.Result{}, err
	}
	var out ir.Result
	for _, f := range allFuncs {
		funcName := cFuncName(f.Namespace, f.Name, f.Variant)
		if hooks.GetHook(funcName) != nil {
			continue
		}
		out.Functions = append(out.Functions, ir.NewFuncDecl(
			module,
			f.File,
			f.Namespace,
			f.Name,
			f.ReturnType,
			f.ParamTypes,
			f.ParamNames,
			f.ParamDefault,
			f.Doc,
			f.Line,
			f.Col,
		))
	}
	out.Sort()
	return out, nil
}

func (g *Generator) selectFunctions(result *parser.ParseResult) ([]*variants.Func, []parser.Diagnostic, error) {
	var allFuncs []*variants.Func
	var diagnostics []parser.Diagnostic
	var firstErr error
	var names []string
	for nsName := range result.Functions {
		names = append(names, nsName)
	}
	sort.Strings(names)

	for _, nsName := range names {
		defs := result.Functions[nsName]
		parts := strings.Split(nsName, "::")
		namespace := strings.Join(parts[:len(parts)-1], "::")
		name := parts[len(parts)-1]

		// Convert parser.Function to variants.Func
		var vFuncs []*variants.Func
		for _, d := range defs {
			vFuncs = append(vFuncs, &variants.Func{
				Name:         d.Name,
				Namespace:    d.Namespace,
				ReturnType:   d.ReturnType,
				ParamTypes:   d.ParamTypes,
				ParamNames:   d.ParamNames,
				ParamDefault: d.ParamDefault,
				Doc:          d.Doc,
				File:         d.File,
				Line:         d.Line,
				Col:          d.Col,
			})
		}

		// Sort by parameter count (descending) to match Python behavior
		sort.Slice(vFuncs, func(i, j int) bool {
			return len(vFuncs[i].ParamNames) > len(vFuncs[j].ParamNames)
		})

		// Apply variant selection
		var selected []*variants.Func
		var variantDiagnostics []variants.Diagnostic
		var err error
		if g.variants != nil {
			selected, variantDiagnostics, err = g.variants.SelectWithDiagnostics(namespace, name, vFuncs)
		} else {
			selected, variantDiagnostics, err = variants.SelectVariantsWithDiagnostics(namespace, name, vFuncs)
		}
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("select variants for %s: %w", nsName, err)
			}
			continue
		}
		for _, d := range variantDiagnostics {
			diagnostics = append(diagnostics, parser.Diagnostic{
				Code:    d.Code,
				Message: d.Message,
				Reason:  d.Reason,
				File:    d.Func.File,
				Line:    d.Func.Line,
				Col:     d.Func.Col,
			})
		}

		// Deduplicate by variant
		seen := make(map[string]bool)
		for _, f := range selected {
			if !seen[f.Variant] {
				seen[f.Variant] = true
				allFuncs = append(allFuncs, f)
			}
		}
	}

	// Sort functions: first by base C name, then by VariantIndex (to preserve variant order)
	sort.Slice(allFuncs, func(i, j int) bool {
		baseI := cNamespace(allFuncs[i].Namespace) + "_" + allFuncs[i].Name
		baseJ := cNamespace(allFuncs[j].Namespace) + "_" + allFuncs[j].Name
		if baseI != baseJ {
			return baseI < baseJ
		}
		// Same base name: sort by variant index to preserve variant order
		return allFuncs[i].VariantIndex < allFuncs[j].VariantIndex
	})
	return allFuncs, diagnostics, firstErr
}

func (g *Generator) writeHeader(w io.Writer, headerName string, headers []string, impl bool, docstring string) {
	fmt.Fprintf(w, `/* Copyright © 2023-2024 Apple Inc.                   */
/*                                                    */
/* This file is auto-generated. Do not edit manually. */
/*                                                    */

`)
	if impl {
		fmt.Fprintf(w, "#include \"mlx/c/%s.h\"\n", headerName)
		for _, header := range headers {
			// Extract path from header (e.g., mlx/ops.h)
			// Handle paths like "/Users/foo/mlx/mlx/ops.h" -> "mlx/ops.h"
			// The MLX repo has structure: <repo>/mlx/<files>, so we may see double "mlx/"
			idx := strings.LastIndex(header, "/mlx/")
			if idx != -1 {
				include := header[idx+1:] // Start from "mlx/..."
				// If include starts with "mlx/mlx/", we have a double - remove the first one
				if strings.HasPrefix(include, "mlx/mlx/") {
					include = include[4:] // Skip the first "mlx/"
				}
				// Skip headers that match the base headerName (e.g., skip mlx/ops.h for "ops")
				// but only when there are multiple headers - the base is redundant
				base := filepath.Base(include)
				if len(headers) > 1 && base == headerName+".h" {
					continue
				}
				fmt.Fprintf(w, "#include \"%s\"\n", include)
			}
		}
		fmt.Fprintf(w, "#include \"mlx/c/error.h\"\n")
		fmt.Fprintf(w, "#include \"mlx/c/private/mlx.h\"\n")
		fmt.Fprintf(w, "\n")
	} else {
		fmt.Fprintf(w, "#ifndef MLX_%s_H\n", strings.ToUpper(headerName))
		fmt.Fprintf(w, "#define MLX_%s_H\n\n", strings.ToUpper(headerName))
		fmt.Fprintf(w, "#include <stdbool.h>\n")
		fmt.Fprintf(w, "#include <stdint.h>\n")
		fmt.Fprintf(w, "#include <stdio.h>\n\n")
		fmt.Fprintf(w, "#include \"mlx/c/array.h\"\n")
		fmt.Fprintf(w, "#include \"mlx/c/closure.h\"\n")
		fmt.Fprintf(w, "#include \"mlx/c/distributed_group.h\"\n")
		fmt.Fprintf(w, "#include \"mlx/c/io_types.h\"\n")
		fmt.Fprintf(w, "#include \"mlx/c/map.h\"\n")
		fmt.Fprintf(w, "#include \"mlx/c/stream.h\"\n")
		fmt.Fprintf(w, "#include \"mlx/c/string.h\"\n")
		fmt.Fprintf(w, "#include \"mlx/c/vector.h\"\n\n")
		fmt.Fprintf(w, "#ifdef __cplusplus\n")
		fmt.Fprintf(w, "extern \"C\" {\n")
		fmt.Fprintf(w, "#endif\n\n")
		if docstring != "" {
			fmt.Fprintf(w, "/**\n")
			fmt.Fprintf(w, " * \\defgroup %s %s\n", headerName, docstring)
			fmt.Fprintf(w, " */\n")
			fmt.Fprintf(w, "/**@{*/\n\n")
		}
	}
}

func (g *Generator) writeFooter(w io.Writer, impl bool, docstring string) {
	if impl {
		// Nothing for implementation
	} else {
		if docstring != "" {
			fmt.Fprintf(w, "\n/**@}*/\n")
		}
		fmt.Fprintf(w, "\n#ifdef __cplusplus\n")
		fmt.Fprintf(w, "}\n")
		fmt.Fprintf(w, "#endif\n\n")
		fmt.Fprintf(w, "#endif\n")
	}
}

func (g *Generator) writeEnum(w io.Writer, enum *parser.Enum) {
	cTypename := "mlx_" + toSnakeCase(enum.Name)
	var cVals []string
	for _, v := range enum.Values {
		cVals = append(cVals, "MLX_"+strings.ToUpper(toSnakeCase(enum.Name))+"_"+strings.ToUpper(v))
	}

	fmt.Fprintf(w, "typedef enum %s_ {%s} %s;\n\n",
		cTypename, strings.Join(cVals, ", "), cTypename)
}

func (g *Generator) writeFunction(w io.Writer, f *variants.Func, impl bool) {
	funcName := cNamespace(f.Namespace) + "_" + f.Name
	if f.Variant != "" {
		funcName += "_" + f.Variant
	}

	// Check for hooks
	if hook := hooks.GetHook(funcName); hook != nil {
		buf := &bytes.Buffer{}
		if hook(buf, funcName, impl) {
			w.Write(buf.Bytes())
			return
		}
	}

	// Find return type
	returnType := g.types.FindByCpp(f.ReturnType)
	if returnType == nil {
		return
	}

	// Build signature
	var cCallParts []string
	var cppCallParts []string

	// Return value as first argument
	resArg := ""
	if returnType.CReturnArg != nil {
		resArg = returnType.CReturnArg("res")
	}
	if resArg != "" {
		cCallParts = append(cCallParts, resArg)
	}

	// Process parameters
	unsupported := false
	for i, pt := range f.ParamTypes {
		pn := f.ParamNames[i]
		if pn == "" {
			pn = "param"
		}

		paramType := g.types.FindByCpp(pt)
		if paramType == nil {
			unsupported = true
			break
		}

		if paramType.CArg != nil {
			cCallParts = append(cCallParts, paramType.CArg(pn))
		}
		if paramType.CToCpp != nil {
			cppCallParts = append(cppCallParts, paramType.CToCpp(pn))
		}
	}

	if unsupported {
		return
	}

	// Build final signature
	cCall := "void"
	if len(cCallParts) > 0 {
		cCall = strings.Join(cCallParts, ", ")
	}
	cppCall := strings.Join(cppCallParts, ", ")

	signature := fmt.Sprintf("int %s(%s)", funcName, cCall)

	if impl {
		// Write implementation
		fmt.Fprintf(w, "extern \"C\" %s {\n", signature)
		fmt.Fprintf(w, "try {\n")

		cppCallExpr := fmt.Sprintf("%s::%s(%s)", f.Namespace, f.Name, cppCall)
		if returnType.CAssignFromCpp != nil {
			fmt.Fprintf(w, "%s;\n", returnType.CAssignFromCpp("res", cppCallExpr, true))
		}

		fmt.Fprintf(w, "} catch (std::exception & e) {\n")
		fmt.Fprintf(w, "mlx_error(e.what());\n")
		fmt.Fprintf(w, "return 1;\n")
		fmt.Fprintf(w, "}\n")
		fmt.Fprintf(w, "return 0;\n")
		fmt.Fprintf(w, "}\n")
	} else {
		// Write declaration
		if g.emitDocs {
			writeFunctionDoc(w, f.Doc)
		}
		fmt.Fprintf(w, "%s;\n", signature)
	}
}

func writeFunctionDoc(w io.Writer, doc string) {
	doc = strings.TrimSpace(doc)
	if doc == "" {
		return
	}
	fmt.Fprintf(w, "\n/**\n")
	for _, line := range strings.Split(doc, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasSuffix(line, " *") {
			line = strings.TrimSpace(strings.TrimSuffix(line, " *"))
		}
		if line == "*" {
			line = ""
		}
		if line == "" {
			fmt.Fprintf(w, " *\n")
			continue
		}
		line = strings.ReplaceAll(line, "*/", "* /")
		fmt.Fprintf(w, " * %s\n", line)
	}
	fmt.Fprintf(w, " */\n")
}

type unsupportedType struct {
	code    string
	message string
}

func (g *Generator) unsupportedTypes(f *variants.Func) []unsupportedType {
	var unsupported []unsupportedType
	if g.types.FindByCpp(f.ReturnType) == nil {
		return append(unsupported, unsupportedType{
			code:    "skip_unsupported_return_type",
			message: fmt.Sprintf("%s has unsupported return type %q", f.PrettyString(), f.ReturnType),
		})
	}
	for i, pt := range f.ParamTypes {
		if g.types.FindByCpp(pt) != nil {
			continue
		}
		name := ""
		if i < len(f.ParamNames) {
			name = f.ParamNames[i]
		}
		unsupported = append(unsupported, unsupportedType{
			code:    "skip_unsupported_param_type",
			message: fmt.Sprintf("%s has unsupported parameter %q of type %q", f.PrettyString(), name, pt),
		})
	}
	return unsupported
}

// toSnakeCase converts CamelCase to snake_case.
func toSnakeCase(name string) string {
	snake := snakeCapRe.ReplaceAllString(name, "${1}_${2}")
	snake = snakeLowerRe.ReplaceAllString(snake, "${1}_${2}")
	return strings.ToLower(snake)
}

// cNamespace converts C++ namespace to C prefix.
func cNamespace(namespace string) string {
	parts := strings.Split(namespace, "::")
	if len(parts) >= 2 && parts[0] == "mlx" && parts[1] == "core" {
		// Remove "core" from mlx::core
		parts = append(parts[:1], parts[2:]...)
		if len(parts) == 2 && parts[1] == "cu" {
			parts[1] = "cuda"
		}
	}
	return strings.Join(parts, "_")
}
