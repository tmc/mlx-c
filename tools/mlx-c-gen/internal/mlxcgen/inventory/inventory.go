package inventory

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	includeRE     = regexp.MustCompile(`#include\s+"(mlx/c/[^"]+)"`)
	cmakeSourceRE = regexp.MustCompile(`mlx/c/[A-Za-z0-9_./-]+\.cpp`)
)

var validKinds = map[string]bool{
	"generated_header_api":  true,
	"generated_support":     true,
	"custom_spec_generated": true,
	"handwritten_runtime":   true,
	"not_owned_by_codegen":  true,
}

var validTargets = map[string]bool{
	"mlxc":   true,
	"jacclc": true,
}

// Entry classifies one source or header file.
type Entry struct {
	Kind   string
	Target string
	Path   string
}

// Read parses an inventory file.
func Read(r io.Reader) ([]Entry, error) {
	var entries []Entry
	seen := map[string]bool{}
	scan := bufio.NewScanner(r)
	for line := 1; scan.Scan(); line++ {
		text, _, _ := strings.Cut(scan.Text(), "#")
		fields := strings.Fields(text)
		if len(fields) == 0 {
			continue
		}
		if len(fields) != 3 {
			return nil, fmt.Errorf("line %d: expected 3 fields, got %d", line, len(fields))
		}
		entry := Entry{Kind: fields[0], Target: fields[1], Path: filepath.ToSlash(fields[2])}
		if !validKinds[entry.Kind] {
			return nil, fmt.Errorf("line %d: unknown kind %q", line, entry.Kind)
		}
		if !validTargets[entry.Target] {
			return nil, fmt.Errorf("line %d: unknown target %q", line, entry.Target)
		}
		if err := validatePath(entry.Path); err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		if seen[entry.Path] {
			return nil, fmt.Errorf("line %d: duplicate path %q", line, entry.Path)
		}
		seen[entry.Path] = true
		entries = append(entries, entry)
	}
	if err := scan.Err(); err != nil {
		return nil, fmt.Errorf("read inventory: %w", err)
	}
	return entries, nil
}

func validatePath(path string) error {
	if filepath.IsAbs(path) || strings.HasPrefix(path, "/") {
		return fmt.Errorf("inventory path %q must be relative", path)
	}
	if strings.Contains(path, `\`) {
		return fmt.Errorf("inventory path %q contains backslash", path)
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
	if clean != path || clean == "." || strings.HasPrefix(clean, "../") {
		return fmt.Errorf("inventory path %q must be clean and stay under the repository", path)
	}
	if !strings.HasPrefix(path, "mlx/c/") {
		return fmt.Errorf("inventory path %q must be under mlx/c", path)
	}
	ext := filepath.Ext(path)
	if ext != ".h" && ext != ".cpp" {
		return fmt.Errorf("inventory path %q must have .h or .cpp extension", path)
	}
	return nil
}

// Check verifies inventoryPath against root.
func Check(root, inventoryPath string) error {
	f, err := os.Open(inventoryPath)
	if err != nil {
		return fmt.Errorf("open %s: %w", inventoryPath, err)
	}
	defer f.Close()
	entries, err := Read(f)
	if err != nil {
		return err
	}
	byPath := map[string]Entry{}
	for _, entry := range entries {
		byPath[entry.Path] = entry
	}

	var problems []string
	for _, entry := range entries {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(entry.Path))); err != nil {
			problems = append(problems, fmt.Sprintf("inventory path %s: %v", entry.Path, err))
		}
		if strings.HasPrefix(entry.Path, "mlx/c/jaccl.") && entry.Target != "jacclc" {
			problems = append(problems, fmt.Sprintf("%s must belong to jacclc", entry.Path))
		}
		if entry.Target == "jacclc" && !strings.HasPrefix(entry.Path, "mlx/c/jaccl.") {
			problems = append(problems, fmt.Sprintf("%s is in jacclc but is not a jaccl file", entry.Path))
		}
	}

	files, err := cFiles(root)
	if err != nil {
		return err
	}
	for _, path := range files {
		if _, ok := byPath[path]; !ok {
			problems = append(problems, fmt.Sprintf("%s is not classified", path))
		}
	}

	cmakeSources, err := cmakeSources(root)
	if err != nil {
		return err
	}
	for _, path := range cmakeSources {
		entry, ok := byPath[path]
		if !ok {
			problems = append(problems, fmt.Sprintf("CMake source %s is not classified", path))
			continue
		}
		want := "mlxc"
		if path == "mlx/c/jaccl.cpp" {
			want = "jacclc"
		}
		if entry.Target != want {
			problems = append(problems, fmt.Sprintf("CMake source %s target = %s, want %s", path, entry.Target, want))
		}
	}

	if err := checkUmbrella(root, byPath, &problems); err != nil {
		return err
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("inventory check failed:\n%s", strings.Join(problems, "\n"))
	}
	return nil
}

func cFiles(root string) ([]string, error) {
	var files []string
	base := filepath.Join(root, "mlx", "c")
	err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := filepath.Ext(path)
		if ext != ".h" && ext != ".cpp" {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", base, err)
	}
	sort.Strings(files)
	return files, nil
}

func cmakeSources(root string) ([]string, error) {
	data, err := os.ReadFile(filepath.Join(root, "CMakeLists.txt"))
	if err != nil {
		return nil, fmt.Errorf("read CMakeLists.txt: %w", err)
	}
	seen := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		line, _, _ = strings.Cut(line, "#")
		for _, match := range cmakeSourceRE.FindAllString(line, -1) {
			seen[filepath.ToSlash(match)] = true
		}
	}
	var out []string
	for path := range seen {
		out = append(out, path)
	}
	sort.Strings(out)
	return out, nil
}

func checkUmbrella(root string, byPath map[string]Entry, problems *[]string) error {
	data, err := os.ReadFile(filepath.Join(root, "mlx", "c", "mlx.h"))
	if err != nil {
		return fmt.Errorf("read mlx/c/mlx.h: %w", err)
	}
	includes := map[string]bool{}
	for _, match := range includeRE.FindAllStringSubmatch(string(data), -1) {
		path := filepath.ToSlash(match[1])
		includes[path] = true
		entry, ok := byPath[path]
		if !ok {
			*problems = append(*problems, fmt.Sprintf("umbrella include %s is not classified", path))
			continue
		}
		if entry.Target != "mlxc" {
			*problems = append(*problems, fmt.Sprintf("umbrella include %s target = %s, want mlxc", path, entry.Target))
		}
	}
	if includes["mlx/c/jaccl.h"] {
		*problems = append(*problems, "mlx/c/mlx.h must not include mlx/c/jaccl.h")
	}
	for path, entry := range byPath {
		if entry.Target != "mlxc" || !isPublicHeader(path) || path == "mlx/c/mlx.h" {
			continue
		}
		if !includes[path] {
			*problems = append(*problems, fmt.Sprintf("public mlxc header %s is not included by mlx/c/mlx.h", path))
		}
	}
	return nil
}

func isPublicHeader(path string) bool {
	return strings.HasPrefix(path, "mlx/c/") &&
		!strings.HasPrefix(path, "mlx/c/private/") &&
		filepath.Ext(path) == ".h"
}
