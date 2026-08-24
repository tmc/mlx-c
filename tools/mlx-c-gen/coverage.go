package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// runCoverage reports MLX_API symbols declared in the MLX source tree that
// have no generated binding: the blind-spot class where upstream moves or
// adds declarations outside the headers the manifest parses. A symbol is
// "covered" when its C++ name appears in any of: the tree's committed lock
// (bound), the manifest (mapped/skipped by policy), or the removal waivers
// (deliberately dropped). Everything else is listed for review.
func runCoverage(args []string) error {
	fs := flag.NewFlagSet("mlx-c-gen coverage", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	mlxSrc := fs.String("mlx-src", "", "MLX source directory")
	lockPath := fs.String("lock", "codegen/mlxc-capi.lock.json", "API lock JSON path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	root, rest, err := takeRoot(args)
	if err != nil {
		return err
	}
	_ = rest
	if err := fs.Parse(rest); err != nil {
		return err
	}

	src, err := resolveMLXSource(root, *mlxSrc)
	if err != nil {
		return err
	}
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("MLX source %s: %w", src, err)
	}

	bound := map[string]bool{}
	if data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(*lockPath))); err == nil {
		for name := range lockFunctionNames(data) {
			bound[name] = true
		}
	}
	policy := map[string]bool{}
	if data, err := os.ReadFile(filepath.Join(root, "codegen", "manifest.yaml")); err == nil {
		for _, m := range manifestNameRE.FindAllStringSubmatch(string(data), -1) {
			if m[1] != "" {
				policy[m[1]] = true
			} else if m[2] != "" {
				policy[m[2]] = true
			}
		}
	}
	waived := map[string]bool{}
	if data, err := os.ReadFile(filepath.Join(root, "codegen", "removals.yaml")); err == nil {
		for _, m := range yamlNameRE.FindAllStringSubmatch(string(data), -1) {
			waived[m[1]] = true
		}
	}

	var uncovered []string
	seenHeader := map[string][]string{}
	err = filepath.Walk(filepath.Join(src, "mlx"), func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".h") {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		if rel == filepath.FromSlash("mlx/api.h") {
			return nil // macro definition file; MLX_API decls here are the macros themselves
		}
		for _, m := range mlxAPIDeclRE.FindAllStringSubmatch(string(data), -1) {
			name := m[1]
			if bound[name] || bound["mlx_"+name] || policy[name] || waived[name] || waived["mlx_"+name] {
				continue
			}
			key := strings.Join(strings.Split(rel, string(filepath.Separator)), "/")
			seenHeader[key] = append(seenHeader[key], name)
			uncovered = append(uncovered, key+": "+name)
		}
		return nil
	})
	if err != nil {
		return err
	}
	sort.Strings(uncovered)

	headers := make([]string, 0, len(seenHeader))
	for h := range seenHeader {
		headers = append(headers, h)
	}
	sort.Strings(headers)
	fmt.Printf("MLX_API symbols without bindings: %d across %d headers\n", len(uncovered), len(seenHeader))
	for _, h := range headers {
		names := seenHeader[h]
		sort.Strings(names)
		fmt.Printf("  %s\n", h)
		for _, n := range names {
			fmt.Printf("    %s\n", n)
		}
	}
	return nil
}

var (
	// MLX_API <ret> name( — captures the declaration name.
	mlxAPIDeclRE = regexp.MustCompile(`MLX_API\s+(?:[A-Za-z_][\w:<>,\s&*.]*?\s)?([A-Za-z_]\w*)\s*\(`)
	// C++ function names in manifest variant mappings ("namespace": entries carry names implicitly via signature blocks; match bare identifiers after 'mlx_core' style keys is unreliable, so match quoted names in custom_hooks/detail lists and mapping keys).
	manifestNameRE = regexp.MustCompile(`(?m)^    ([a-z_][a-z0-9_]*):$|^\s+- ([a-z_][a-z0-9_]*)$`)
	yamlNameRE     = regexp.MustCompile(`name:\s*(\S+)`)
)

// lockFunctionNames extracts function names from a lock JSON by regex so the
// coverage verb does not depend on the apilock schema.
func lockFunctionNames(data []byte) map[string]bool {
	out := map[string]bool{}
	for _, m := range lockNameRE.FindAllStringSubmatch(string(data), -1) {
		out[m[1]] = true
	}
	return out
}

var lockNameRE = regexp.MustCompile(`"name":\s*"([A-Za-z_][A-Za-z0-9_]*)"`)
