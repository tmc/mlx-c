// Package parser provides C++ header parsing for MLX binding generation using clang AST JSON.
package parser

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"unicode"
)

var parsedCacheVersion = "mlxcgen-ast-v2"

// IncludePaths holds include paths for parsing.
var IncludePaths []string

// PreIncludes holds headers to include before the target headers.
// This is needed for headers that don't include their dependencies.
var PreIncludes []string

// CompileCommandsPath holds the optional compile_commands.json path for parser flags.
var CompileCommandsPath string

// ASTCacheDir holds the optional directory for parsed clang AST cache entries.
var ASTCacheDir string

var clangVersionCache struct {
	once    sync.Once
	version string
	err     error
}

// SetIncludePaths sets the include paths to use when parsing headers.
func SetIncludePaths(paths []string) {
	IncludePaths = paths
}

// SetPreIncludes sets headers to include before target headers.
func SetPreIncludes(includes []string) {
	PreIncludes = includes
}

// SetCompileCommandsPath sets an optional compile_commands.json path.
func SetCompileCommandsPath(path string) {
	CompileCommandsPath = path
}

// SetASTCacheDir sets an optional cache directory for parsed clang AST results.
func SetASTCacheDir(path string) {
	ASTCacheDir = path
}

// ResolveASTCacheDir returns the parsed clang AST cache directory.
func ResolveASTCacheDir(explicit string, disabled bool) string {
	if disabled {
		return ""
	}
	if explicit != "" {
		return explicit
	}
	if env := os.Getenv("MLX_C_AST_CACHE"); env != "" {
		return env
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(base, "mlx-c", "mlxcgen", "ast")
}

// Function represents a parsed C++ function declaration.
type Function struct {
	Name         string
	Namespace    string
	ReturnType   string
	ParamTypes   []string
	ParamNames   []string
	ParamDefault []string
	Doc          string // Doxygen comment
	Variant      string // Set later by variant selection
	File         string
	Line         int
	Col          int
}

// Enum represents a parsed C++ enum declaration.
type Enum struct {
	Name      string
	Namespace string
	Values    []string
}

// Diagnostic records a generation-pipeline decision that can hide source API surface.
type Diagnostic struct {
	Code    string
	Message string
	Reason  string
	File    string
	Line    int
	Col     int
}

// ParseResult holds the results of parsing a header file.
type ParseResult struct {
	Functions   map[string][]*Function // namespace::name -> list of overloads
	Enums       map[string]*Enum       // namespace::name -> enum
	Diagnostics []Diagnostic
}

// clangNode represents a node in the clang AST JSON.
type clangNode struct {
	ID                  string      `json:"id"`
	Kind                string      `json:"kind"`
	Name                string      `json:"name"`
	MangledName         string      `json:"mangledName"`
	Type                *clangType  `json:"type"`
	Inner               []clangNode `json:"inner"`
	Loc                 *clangLoc   `json:"loc"`
	Range               *clangRange `json:"range"`
	IsUsed              bool        `json:"isUsed"`
	ReferencedDecl      *clangRef   `json:"referencedDecl"`
	Value               interface{} `json:"value"` // Can be string, number, or bool
	ValueCategory       string      `json:"valueCategory"`
	StorageClass        string      `json:"storageClass"`
	Inline              bool        `json:"inline"`
	Constexpr           bool        `json:"constexpr"`
	QualType            string      `json:"qualType"`
	DesugaredQualType   string      `json:"desugaredQualType"`
	TagUsed             string      `json:"tagUsed"`
	CompleteDefinition  bool        `json:"completeDefinition"`
	FixedUnderlyingType *clangType  `json:"fixedUnderlyingType"`
	ScopedEnumTag       string      `json:"scopedEnumTag"`
	Text                string      `json:"text"` // For TextComment

}

type clangType struct {
	QualType          string `json:"qualType"`
	DesugaredQualType string `json:"desugaredQualType"`
}

type clangLoc struct {
	File         string       `json:"file"`
	Line         int          `json:"line"`
	Col          int          `json:"col"`
	Offset       int          `json:"offset"`
	IncludedFrom *clangIncLoc `json:"includedFrom"`
}

type clangIncLoc struct {
	File string `json:"file"`
}

type clangRange struct {
	Begin clangLoc `json:"begin"`
	End   clangLoc `json:"end"`
}

type clangRef struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
	Name string `json:"name"`
}

// ParseFile parses a C++ header file using clang AST JSON.
func ParseFile(path string) (*ParseResult, error) {
	return ParseFiles([]string{path})
}

// ParseFiles parses multiple header files using clang AST JSON.
// Each file is parsed individually to preserve location information.
func ParseFiles(paths []string) (*ParseResult, error) {
	if len(paths) == 0 {
		return &ParseResult{
			Functions: make(map[string][]*Function),
			Enums:     make(map[string]*Enum),
		}, nil
	}

	result := &ParseResult{
		Functions: make(map[string][]*Function),
		Enums:     make(map[string]*Enum),
	}

	// Parse each file individually to preserve location information
	for _, p := range paths {
		absPath, err := filepath.Abs(p)
		if err != nil {
			return nil, fmt.Errorf("getting absolute path for %s: %w", p, err)
		}
		if ASTCacheDir != "" {
			cached, ok, err := readCachedParseResult(ASTCacheDir, []string{p})
			if err != nil {
				return nil, fmt.Errorf("read parsed cache for %s: %w", p, err)
			}
			if ok {
				mergeParseResult(result, cached)
				continue
			}
		}

		// Get the AST JSON using clang for this single file
		astJSON, wrapperPath, deps, err := runClangAST([]string{p})
		if err != nil {
			return nil, fmt.Errorf("running clang for %s: %w", p, err)
		}

		// Parse JSON
		root, err := parseClangASTJSON(astJSON)
		if err != nil {
			return nil, fmt.Errorf("parsing clang AST JSON for %s: %w", p, err)
		}

		headerResult := &ParseResult{
			Functions: make(map[string][]*Function),
			Enums:     make(map[string]*Enum),
		}

		// Extract the directory containing the header for filtering
		headerDirs := make(map[string]bool)
		headerDirs[filepath.Dir(absPath)] = true

		// Walk the AST for this file
		// Initialize currentFile to empty - it will be set by loc.File as we walk
		currentFile := ""
		walkAST(root, "", headerResult, []string{absPath}, headerDirs, wrapperPath, &currentFile)
		if ASTCacheDir != "" {
			if err := writeCachedParseResult(ASTCacheDir, []string{p}, headerResult, deps); err != nil {
				return nil, fmt.Errorf("write parsed cache for %s: %w", p, err)
			}
		}
		mergeParseResult(result, headerResult)
	}

	// Deduplicate functions that differ only in namespace qualification of types
	// (e.g., "array" vs "mlx::core::array")
	for key, funcs := range result.Functions {
		result.Functions[key] = deduplicateFunctions(funcs)
	}

	return result, nil
}

func parseClangASTJSON(data []byte) (*clangNode, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	var nodes []clangNode
	for {
		var node clangNode
		if err := dec.Decode(&node); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		nodes = append(nodes, node)
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("empty clang AST JSON")
	}
	if len(nodes) == 1 {
		return &nodes[0], nil
	}
	return &clangNode{
		Kind:  "TranslationUnitDecl",
		Inner: nodes,
	}, nil
}

func mergeParseResult(dst, src *ParseResult) {
	for name, funcs := range src.Functions {
		dst.Functions[name] = append(dst.Functions[name], funcs...)
	}
	for name, enum := range src.Enums {
		dst.Enums[name] = enum
	}
	dst.Diagnostics = append(dst.Diagnostics, src.Diagnostics...)
}

// deduplicateFunctions removes duplicate functions that differ only in type namespace qualification.
func deduplicateFunctions(funcs []*Function) []*Function {
	seen := make(map[string]bool)
	var unique []*Function

	for _, f := range funcs {
		// Create a signature key from normalized parameter types
		sig := f.Name + "(" + strings.Join(f.ParamTypes, ",") + ")->" + f.ReturnType
		if !seen[sig] {
			seen[sig] = true
			unique = append(unique, f)
		}
	}

	return unique
}

// runClangAST runs clang to produce AST JSON for header files.
// It returns the AST JSON, the main target file path for location matching,
// and dependency stats for parsed-result cache validation.
func runClangAST(paths []string) ([]byte, string, []astCacheDep, error) {
	if len(paths) == 0 {
		return nil, "", nil, fmt.Errorf("no paths provided")
	}

	_, wrapperText, args, _, err := clangASTCacheInput(paths)
	if err != nil {
		return nil, "", nil, err
	}

	tmpFile, err := os.CreateTemp("", "mlxcgen-*.cpp")
	if err != nil {
		return nil, "", nil, err
	}
	mainFile := tmpFile.Name()

	if _, err := tmpFile.WriteString(wrapperText); err != nil {
		tmpFile.Close()
		os.Remove(mainFile)
		return nil, "", nil, err
	}
	tmpFile.Close()
	defer os.Remove(mainFile)

	baseArgs := append([]string(nil), args...)
	depFile := ""
	if ASTCacheDir != "" {
		dep, err := os.CreateTemp("", "mlxcgen-*.d")
		if err == nil {
			depFile = dep.Name()
			dep.Close()
			defer os.Remove(depFile)
			args = append(args, "-MD", "-MF", depFile)
		}
	}
	args = append(args, mainFile)

	cmd := exec.Command("clang++", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()

	// Clang may return non-zero even on success with AST dump
	// Check if we got JSON output
	if stdout.Len() == 0 {
		if runErr != nil {
			return nil, "", nil, fmt.Errorf("clang failed: %v\nstderr: %s", runErr, stderr.String())
		}
		return nil, "", nil, fmt.Errorf("clang produced no output")
	}

	var deps []astCacheDep
	if ASTCacheDir != "" {
		depPaths, err := readClangDepFile(depFile)
		if err != nil {
			depPaths, err = clangDeps(baseArgs, mainFile)
		}
		if err == nil {
			deps, _ = statASTCacheDeps(depPaths, mainFile)
		}
	}

	return stdout.Bytes(), mainFile, deps, nil
}

func clangASTCacheInput(paths []string) ([]string, string, []string, string, error) {
	var absPaths []string
	for _, p := range paths {
		absPath, err := filepath.Abs(p)
		if err != nil {
			return nil, "", nil, "", fmt.Errorf("getting absolute path for %s: %w", p, err)
		}
		absPaths = append(absPaths, absPath)
	}

	wrapperText := clangWrapper(absPaths)
	args, err := clangASTArgs(absPaths)
	if err != nil {
		return nil, "", nil, "", err
	}
	cacheKey := ""
	if ASTCacheDir != "" {
		cacheKey, err = astCacheKey(wrapperText, args)
		if err != nil {
			return nil, "", nil, "", err
		}
	}
	return absPaths, wrapperText, args, cacheKey, nil
}

func clangWrapper(absPaths []string) string {
	var wrapper strings.Builder
	for _, inc := range PreIncludes {
		wrapper.WriteString(fmt.Sprintf("#include \"%s\"\n", inc))
	}
	for _, p := range absPaths {
		wrapper.WriteString(fmt.Sprintf("#include \"%s\"\n", p))
	}
	return wrapper.String()
}

type astCacheMeta struct {
	Deps []astCacheDep `json:"deps"`
}

type astCacheDep struct {
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"mod_time"`
}

func readCachedParseResult(dir string, paths []string) (*ParseResult, bool, error) {
	_, _, _, key, err := clangASTCacheInput(paths)
	if err != nil {
		return nil, false, err
	}
	if key == "" {
		return nil, false, nil
	}
	dataPath, metaPath := parseCachePaths(dir, key)
	meta, err := readASTCacheMeta(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if !astCacheDepsFresh(meta.Deps) {
		return nil, false, nil
	}
	data, err := os.ReadFile(dataPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("read parsed cache: %w", err)
	}
	result := &ParseResult{}
	if err := json.Unmarshal(data, result); err != nil {
		return nil, false, fmt.Errorf("parse parsed cache: %w", err)
	}
	if result.Functions == nil {
		result.Functions = make(map[string][]*Function)
	}
	if result.Enums == nil {
		result.Enums = make(map[string]*Enum)
	}
	return result, true, nil
}

func writeCachedParseResult(dir string, paths []string, result *ParseResult, deps []astCacheDep) error {
	_, _, _, key, err := clangASTCacheInput(paths)
	if err != nil {
		return err
	}
	if key == "" || len(deps) == 0 {
		return nil
	}
	dataPath, metaPath := parseCachePaths(dir, key)
	if err := os.MkdirAll(filepath.Dir(dataPath), 0o777); err != nil {
		return fmt.Errorf("create parsed cache dir: %w", err)
	}
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal parsed cache: %w", err)
	}
	if err := writeFileAtomic(dataPath, append(data, '\n'), 0o666); err != nil {
		return fmt.Errorf("write parsed cache: %w", err)
	}
	meta, err := json.Marshal(astCacheMeta{Deps: deps})
	if err != nil {
		return fmt.Errorf("marshal parsed cache metadata: %w", err)
	}
	if err := writeFileAtomic(metaPath, meta, 0o666); err != nil {
		return fmt.Errorf("write parsed cache metadata: %w", err)
	}
	return nil
}

func astCacheKey(wrapper string, args []string) (string, error) {
	version, err := clangVersion()
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(struct {
		Version string   `json:"version"`
		Clang   string   `json:"clang"`
		Args    []string `json:"args"`
		Wrapper string   `json:"wrapper"`
	}{
		Version: parsedCacheVersion,
		Clang:   version,
		Args:    args,
		Wrapper: wrapper,
	})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return fmt.Sprintf("%x", sum[:]), nil
}

func clangVersion() (string, error) {
	clangVersionCache.once.Do(func() {
		out, err := exec.Command("clang++", "--version").Output()
		if err != nil {
			clangVersionCache.err = fmt.Errorf("run clang++ --version: %w", err)
			return
		}
		clangVersionCache.version = strings.TrimSpace(string(out))
	})
	return clangVersionCache.version, clangVersionCache.err
}

func parseCachePaths(dir, key string) (dataPath, metaPath string) {
	subdir := filepath.Join(dir, key[:2])
	return filepath.Join(subdir, key+".parse.json"), filepath.Join(subdir, key+".parse.meta.json")
}

func readASTCacheMeta(path string) (astCacheMeta, error) {
	var meta astCacheMeta
	data, err := os.ReadFile(path)
	if err != nil {
		return meta, err
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		return meta, fmt.Errorf("parse cache metadata: %w", err)
	}
	return meta, nil
}

func astCacheDepsFresh(deps []astCacheDep) bool {
	if len(deps) == 0 {
		return false
	}
	for _, dep := range deps {
		info, err := os.Stat(dep.Path)
		if err != nil {
			return false
		}
		if info.Size() != dep.Size || info.ModTime().UnixNano() != dep.ModTime {
			return false
		}
	}
	return true
}

func clangDeps(args []string, mainFile string) ([]string, error) {
	depArgs := append([]string{"-M"}, clangDependencyArgs(args)...)
	depArgs = append(depArgs, mainFile)
	cmd := exec.Command("clang++", depArgs...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("run clang dependency scan: %w\n%s", err, strings.TrimSpace(stderr.String()))
	}
	return parseMakeDeps(string(out)), nil
}

func clangDependencyArgs(args []string) []string {
	var out []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-Xclang" && i+1 < len(args) {
			next := args[i+1]
			if next == "-ast-dump=json" || strings.HasPrefix(next, "-ast-dump-filter=") {
				i++
				continue
			}
		}
		if arg == "-fsyntax-only" {
			continue
		}
		out = append(out, arg)
	}
	return out
}

func parseMakeDeps(out string) []string {
	out = strings.ReplaceAll(out, "\\\n", " ")
	colon := strings.Index(out, ":")
	if colon < 0 {
		return nil
	}
	fields := strings.Fields(out[colon+1:])
	deps := make([]string, 0, len(fields))
	for _, dep := range fields {
		deps = append(deps, filepath.Clean(dep))
	}
	return deps
}

func readClangDepFile(path string) ([]string, error) {
	if path == "" {
		return nil, fmt.Errorf("missing dependency file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	deps := parseMakeDeps(string(data))
	if len(deps) == 0 {
		return nil, fmt.Errorf("empty dependency file")
	}
	return deps, nil
}

func statASTCacheDeps(paths []string, skip string) ([]astCacheDep, error) {
	stats := make([]astCacheDep, 0, len(paths))
	skip = filepath.Clean(skip)
	for _, path := range paths {
		path = filepath.Clean(path)
		if path == skip {
			continue
		}
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		stats = append(stats, astCacheDep{
			Path:    path,
			Size:    info.Size(),
			ModTime: info.ModTime().UnixNano(),
		})
	}
	return stats, nil
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func clangASTArgs(absPaths []string) ([]string, error) {
	args := []string{
		"-Xclang",
		"-ast-dump=json",
		"-Xclang",
		"-ast-dump-filter=mlx::core",
		"-fsyntax-only",
	}
	if CompileCommandsPath != "" {
		compileArgs, err := compileCommandArgs(CompileCommandsPath, absPaths)
		if err != nil {
			return nil, err
		}
		args = append(args, compileArgs...)
	}
	if !hasStandardArg(args) {
		args = append(args, "-std=c++20")
	}
	if !hasXArg(args) {
		args = append(args, "-x", "c++")
	}

	// Add configured include paths (e.g., MLX source directories)
	for _, inc := range IncludePaths {
		args = append(args, "-I"+inc)
	}

	// Add include paths based on input file locations
	includeDirs := make(map[string]bool)
	for _, p := range absPaths {
		// Add the directory containing the header and its parents
		dir := filepath.Dir(p)
		for dir != "/" && dir != "." {
			includeDirs[dir] = true
			dir = filepath.Dir(dir)
		}
	}
	var dirs []string
	for dir := range includeDirs {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	for _, dir := range dirs {
		args = append(args, "-I"+dir)
	}

	return args, nil
}

type compileCommand struct {
	Directory string   `json:"directory"`
	Command   string   `json:"command"`
	Arguments []string `json:"arguments"`
	File      string   `json:"file"`
}

func compileCommandArgs(path string, targetPaths []string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read compile commands: %w", err)
	}
	var commands []compileCommand
	if err := json.Unmarshal(data, &commands); err != nil {
		return nil, fmt.Errorf("parse compile commands: %w", err)
	}
	cmd := selectCompileCommand(commands, targetPaths)
	if cmd == nil {
		return nil, fmt.Errorf("compile commands %s has no entries", path)
	}
	argv := append([]string(nil), cmd.Arguments...)
	if len(argv) == 0 {
		argv, err = splitCommand(cmd.Command)
		if err != nil {
			return nil, err
		}
	}
	if len(argv) == 0 {
		return nil, fmt.Errorf("compile command for %s is empty", cmd.File)
	}
	dir := compileCommandDir(filepath.Dir(path), cmd.Directory)
	return filterCompileArgs(argv[1:], compileCommandFile(dir, cmd.File), dir), nil
}

func selectCompileCommand(commands []compileCommand, targetPaths []string) *compileCommand {
	for _, target := range targetPaths {
		targetDir := filepath.Clean(filepath.Dir(target))
		stem := fileStem(target)
		for i := range commands {
			cmdDir := compileCommandDir("", commands[i].Directory)
			cmdFile := compileCommandFile(cmdDir, commands[i].File)
			if fileStem(cmdFile) == stem && filepath.Clean(filepath.Dir(cmdFile)) == targetDir {
				return &commands[i]
			}
		}
	}
	for _, target := range targetPaths {
		stem := fileStem(target)
		for i := range commands {
			cmdDir := compileCommandDir("", commands[i].Directory)
			cmdFile := compileCommandFile(cmdDir, commands[i].File)
			if fileStem(cmdFile) == stem {
				return &commands[i]
			}
		}
	}
	if len(commands) == 0 {
		return nil
	}
	return &commands[0]
}

func compileCommandDir(base, dir string) string {
	if dir == "" {
		if base == "" {
			return "."
		}
		return filepath.Clean(base)
	}
	if filepath.IsAbs(dir) || base == "" {
		return filepath.Clean(dir)
	}
	return filepath.Clean(filepath.Join(base, dir))
}

func compileCommandFile(dir, file string) string {
	if file == "" || filepath.IsAbs(file) {
		return filepath.Clean(file)
	}
	return filepath.Clean(filepath.Join(dir, file))
}

func fileStem(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func filterCompileArgs(args []string, sourceFile, dir string) []string {
	var out []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-c":
			continue
		case arg == "-o" || arg == "-MF" || arg == "-MT" || arg == "-MQ" || arg == "-MJ":
			i++
			continue
		case strings.HasPrefix(arg, "-o") && arg != "-ObjC":
			continue
		case samePath(compileCommandFile(dir, arg), sourceFile), isSourcePath(arg):
			continue
		}
		if compilePathFlagTakesNext(arg) {
			out = append(out, arg)
			if i+1 < len(args) {
				i++
				out = append(out, compileArgPath(dir, args[i]))
			}
			continue
		}
		if rewritten, ok := rewriteJoinedCompilePathFlag(arg, dir); ok {
			out = append(out, rewritten)
			continue
		}
		if rewritten, ok := rewriteEqualCompilePathFlag(arg, dir); ok {
			out = append(out, rewritten)
			continue
		}
		out = append(out, arg)
	}
	return out
}

func compilePathFlagTakesNext(arg string) bool {
	switch arg {
	case "-I", "-F", "-isystem", "-iquote", "-idirafter", "-iframework", "-include", "-imacros", "-isysroot", "-resource-dir":
		return true
	default:
		return false
	}
}

func rewriteJoinedCompilePathFlag(arg, dir string) (string, bool) {
	for _, prefix := range []string{"-I", "-F", "-isystem", "-iquote", "-idirafter", "-iframework", "-isysroot"} {
		if strings.HasPrefix(arg, prefix) && arg != prefix {
			return prefix + compileArgPath(dir, strings.TrimPrefix(arg, prefix)), true
		}
	}
	return "", false
}

func rewriteEqualCompilePathFlag(arg, dir string) (string, bool) {
	for _, prefix := range []string{"--sysroot="} {
		if strings.HasPrefix(arg, prefix) {
			return prefix + compileArgPath(dir, strings.TrimPrefix(arg, prefix)), true
		}
	}
	return "", false
}

func compileArgPath(dir, path string) string {
	if path == "" || filepath.IsAbs(path) || strings.HasPrefix(path, "-") {
		return path
	}
	return filepath.Clean(filepath.Join(dir, path))
}

func samePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

func isSourcePath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".c", ".cc", ".cpp", ".cxx", ".c++", ".m", ".mm":
		return true
	}
	return false
}

func splitCommand(command string) ([]string, error) {
	var args []string
	var b strings.Builder
	var quote rune
	escaped := false
	flush := func() {
		if b.Len() == 0 {
			return
		}
		args = append(args, b.String())
		b.Reset()
	}
	for _, r := range command {
		if escaped {
			b.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
				continue
			}
			b.WriteRune(r)
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			continue
		}
		if unicode.IsSpace(r) {
			flush()
			continue
		}
		b.WriteRune(r)
	}
	if escaped {
		b.WriteRune('\\')
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quote in compile command")
	}
	flush()
	return args, nil
}

func hasStandardArg(args []string) bool {
	for _, arg := range args {
		if strings.HasPrefix(arg, "-std=") {
			return true
		}
	}
	return false
}

func hasXArg(args []string) bool {
	for i, arg := range args {
		if arg == "-x" && i+1 < len(args) {
			return true
		}
		if strings.HasPrefix(arg, "-x") && arg != "-Xclang" {
			return true
		}
	}
	return false
}

// walkAST recursively walks the clang AST and extracts declarations.
// currentFile tracks the most recently seen file for location filtering.
func walkAST(node *clangNode, namespace string, result *ParseResult, targetPaths []string, headerDirs map[string]bool, wrapperPath string, currentFile *string) {
	// Track the current file context - update when we see a loc.File
	if node.Loc != nil && node.Loc.File != "" {
		*currentFile = node.Loc.File
	}

	switch node.Kind {
	case "NamespaceDecl":
		newNs := namespace
		if node.Name != "" {
			if newNs != "" {
				newNs += "::" + node.Name
			} else {
				newNs = node.Name
			}
		}
		for i := range node.Inner {
			walkAST(&node.Inner[i], newNs, result, targetPaths, headerDirs, wrapperPath, currentFile)
		}

	case "FunctionDecl":
		funcNS := normalizeNamespace(namespace)
		// Skip if not in a target header
		if !isInTargetHeaders(node.Loc, targetPaths, headerDirs, wrapperPath, *currentFile) {
			return
		}
		// Skip operator overloads
		if strings.HasPrefix(node.Name, "operator") {
			addDiagnostic(result, "skip_operator", node.Loc, *currentFile, "%s is an operator overload", qualifiedName(funcNS, node.Name))
			return
		}
		// Skip compiler builtins
		if strings.HasPrefix(node.Name, "__") {
			addDiagnostic(result, "skip_builtin", node.Loc, *currentFile, "%s is a compiler builtin", qualifiedName(funcNS, node.Name))
			return
		}
		// Skip if no type info
		if node.Type == nil {
			addDiagnostic(result, "skip_missing_type", node.Loc, *currentFile, "%s has no function type", qualifiedName(funcNS, node.Name))
			return
		}

		f := extractFunction(node, funcNS, *currentFile)
		if f != nil {
			// Skip Stream return type
			if f.ReturnType == "Stream" {
				addDiagnostic(result, "skip_stream_return", node.Loc, *currentFile, "%s returns Stream", qualifiedName(funcNS, f.Name))
				return
			}
			// Skip template functions (have template type parameters like T, U)
			if isTemplateFunction(f) {
				addDiagnostic(result, "skip_template_function", node.Loc, *currentFile, "%s uses template parameters", qualifiedName(funcNS, f.Name))
				return
			}

			key := funcNS + "::" + f.Name
			result.Functions[key] = append(result.Functions[key], f)
		} else {
			addDiagnostic(result, "skip_unparsed_function", node.Loc, *currentFile, "%s could not be parsed from clang type %q", qualifiedName(funcNS, node.Name), node.Type.QualType)
		}

	case "FullComment":
		// This case might not be reached directly if comments are attached to Decls
		// but we handle inner comments in extractFunction

	case "EnumDecl":
		enumNS := normalizeNamespace(namespace)
		if !isInTargetHeaders(node.Loc, targetPaths, headerDirs, wrapperPath, *currentFile) {
			return
		}
		if node.Name == "" {
			return
		}
		// Skip system/standard library enums
		if strings.HasPrefix(enumNS, "std") {
			return
		}

		e := extractEnum(node, enumNS)
		if e != nil {
			key := enumNS + "::" + e.Name
			result.Enums[key] = e
		}

	case "CXXRecordDecl", "ClassTemplateDecl", "TypedefDecl", "TypeAliasDecl":
		// Skip these but process their children for nested namespaces
		for i := range node.Inner {
			walkAST(&node.Inner[i], namespace, result, targetPaths, headerDirs, wrapperPath, currentFile)
		}

	default:
		// Recurse into children for TranslationUnitDecl and other containers
		for i := range node.Inner {
			walkAST(&node.Inner[i], namespace, result, targetPaths, headerDirs, wrapperPath, currentFile)
		}
	}
}

func normalizeNamespace(namespace string) string {
	if namespace == "core" || strings.HasPrefix(namespace, "core::") {
		return "mlx::" + namespace
	}
	return namespace
}

func addDiagnostic(result *ParseResult, code string, loc *clangLoc, currentFile string, format string, args ...interface{}) {
	file := currentFile
	line, col := 0, 0
	if loc != nil {
		if loc.File != "" {
			file = loc.File
		}
		line = loc.Line
		col = loc.Col
	}
	result.Diagnostics = append(result.Diagnostics, Diagnostic{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
		Reason:  diagnosticReason(code),
		File:    file,
		Line:    line,
		Col:     col,
	})
}

func diagnosticReason(code string) string {
	switch code {
	case "skip_operator", "skip_builtin":
		return "unbindable_signature"
	case "skip_template_function":
		return "template_function"
	case "skip_missing_type", "skip_stream_return", "skip_unparsed_function":
		return "unsupported_type"
	default:
		return ""
	}
}

func qualifiedName(namespace, name string) string {
	if namespace == "" {
		return name
	}
	return namespace + "::" + name
}

// isInTargetHeaders checks if a location is in one of the target headers.
// mainFilePath is the file passed to clang (either a single header or a wrapper).
// currentFile is the most recently seen file from loc.File, used as context when loc.File is empty.
func isInTargetHeaders(loc *clangLoc, targetPaths []string, headerDirs map[string]bool, mainFilePath string, currentFile string) bool {
	// Determine the effective file - either from loc.File or from tracked context
	effectiveFile := currentFile
	if loc != nil && loc.File != "" {
		effectiveFile = loc.File
	}
	effectiveFile = filepath.Clean(effectiveFile)

	// Check if the effective file matches a target path
	for _, p := range targetPaths {
		if effectiveFile == filepath.Clean(p) {
			return true
		}
	}

	return false
}

// extractFunction extracts a Function from a FunctionDecl node.
func extractFunction(node *clangNode, namespace, currentFile string) *Function {
	if node.Type == nil {
		return nil
	}

	// Parse the function type string
	// Format is typically: "returnType (paramType1, paramType2, ...)"
	typeStr := node.Type.QualType
	if node.Type.DesugaredQualType != "" {
		typeStr = node.Type.DesugaredQualType
	}

	// Find the parameter list start - must be outside angle brackets
	// e.g., for "std::function<array(array)> (int)" we want the last "(" not inside <>
	parenIdx := findFunctionParenthesis(typeStr)
	if parenIdx == -1 {
		return nil
	}

	returnType := strings.TrimSpace(typeStr[:parenIdx])

	// Extract parameter information from inner nodes
	var paramTypes, paramNames, paramDefaults []string
	for _, inner := range node.Inner {
		if inner.Kind == "ParmVarDecl" {
			pt := ""
			if inner.Type != nil {
				pt = inner.Type.QualType
			}
			paramTypes = append(paramTypes, normalizeType(pt))
			paramNames = append(paramNames, inner.Name)

			// Check for default value
			hasDefault := false
			for _, innerInner := range inner.Inner {
				if isExprNode(innerInner.Kind) {
					hasDefault = true
					break
				}
			}
			if hasDefault {
				paramDefaults = append(paramDefaults, "default") // Mark as having default
			} else {
				paramDefaults = append(paramDefaults, "")
			}
		}
	}

	file := currentFile
	line, col := 0, 0
	if node.Loc != nil {
		if node.Loc.File != "" {
			file = node.Loc.File
		}
		line = node.Loc.Line
		col = node.Loc.Col
	}

	return &Function{
		Name:         node.Name,
		Namespace:    namespace,
		ReturnType:   normalizeType(returnType),
		ParamTypes:   paramTypes,
		ParamNames:   paramNames,
		ParamDefault: paramDefaults,
		Doc:          extractDoc(node),
		File:         file,
		Line:         line,
		Col:          col,
	}
}

// extractDoc extracts documentation from a node's children.
//
// Clang's comment lexer splits text at '<' (treated as the start of an HTML
// tag), so a comment like "(a <= b)" arrives as multiple TextComment nodes on
// the same source line. Fragments whose byte ranges are contiguous belong to
// the same line and are joined directly; others are separated by newlines.
func extractDoc(node *clangNode) string {
	var docs []string
	var lastEnd int = -1
	part := func(text string, start int) {
		if len(docs) > 0 && start == lastEnd {
			docs[len(docs)-1] += text
		} else {
			docs = append(docs, strings.TrimSpace(text))
		}
		lastEnd = start + len(text)
	}
	for _, inner := range node.Inner {
		if inner.Kind != "FullComment" {
			continue
		}
		for _, commentPart := range inner.Inner {
			switch commentPart.Kind {
			case "ParagraphComment":
				for _, textPart := range commentPart.Inner {
					if textPart.Kind != "TextComment" {
						continue
					}
					start := -1
					if textPart.Loc != nil {
						start = textPart.Loc.Offset
					}
					part(textPart.Text, start)
				}
			case "VerbatimBlockComment":
				// @code/@endcode blocks: flush pending text so the sanitizer
				// sees the block boundary, then keep or drop the block.
				var lines []string
				cpp := false
				for _, line := range commentPart.Inner {
					if line.Kind != "VerbatimBlockLineComment" {
						continue
					}
					if cppOnlyTokenRE.MatchString(line.Text) {
						cpp = true
					}
					lines = append(lines, strings.TrimSpace(line.Text))
				}
				docs = append(docs, "") // block boundary marker
				if cpp {
					docs = append(docs, docCodeOmittedSentinel)
				} else {
					docs = append(docs, lines...)
				}
			}
		}
	}
	for i, d := range docs {
		docs[i] = strings.TrimSpace(d)
	}
	return sanitizeDocLines(docs)
}

// C++-only tokens whose presence makes a @code block unusable in a C header.
var cppOnlyTokenRE = regexp.MustCompile(
	`\bauto\s|std::|->|::|array\{|vector\{|new\b|\.c_str\(\)`)

const docCodeOmittedSentinel = "\x00"

// sanitizeDocLines mirrors the ratified doc-comment cleanup: drop C++ code
// blocks in favor of a single visible note (merging consecutive notes),
// remove dangling lead-in lines ending in ':' above omitted blocks, and
// collapse blank runs.
func sanitizeDocLines(lines []string) string {
	var out []string
	pendingNote := false
	for _, ln := range lines {
		if pendingNote {
			if ln == "" || (!strings.HasSuffix(strings.TrimSpace(ln), ":")) {
				if strings.TrimSpace(ln) != "" {
					pendingNote = false
				}
			} else {
				continue // follow-on lead-in like "will produce:" after a note
			}
		}
		if ln == docCodeOmittedSentinel {
			for len(out) > 0 && out[len(out)-1] == "" {
				out = out[:len(out)-1]
			}
			if n := len(out); n > 0 && strings.HasSuffix(out[n-1], ":") {
				out = out[:n-1]
				for len(out) > 0 && out[len(out)-1] == "" {
					out = out[:len(out)-1]
				}
			}
			note := "Code example omitted: written in C++."
			if len(out) == 0 || out[len(out)-1] != note {
				if len(out) > 0 && out[len(out)-1] != "" {
					out = append(out, "")
				}
				out = append(out, note)
			}
			pendingNote = true
			continue
		}
		if ln == "" && len(out) > 0 && out[len(out)-1] == "" {
			continue
		}
		out = append(out, ln)
	}
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return strings.Join(out, "\n")
}

// isTemplateFunction returns true if the function has template type parameters.
// Template functions have single-letter type names or contain template syntax.
func isTemplateFunction(f *Function) bool {
	templateTypes := map[string]bool{
		"T": true, "U": true, "V": true, "W": true,
		"T1": true, "T2": true, "T3": true, "T4": true,
		"...": true,
	}

	// Check return type
	if templateTypes[f.ReturnType] {
		return true
	}

	// Check parameter types
	for _, pt := range f.ParamTypes {
		if templateTypes[pt] {
			return true
		}
		// Also check for template syntax like std::enable_if, etc.
		if strings.Contains(pt, "enable_if") || strings.Contains(pt, "type-parameter") {
			return true
		}
	}
	return false
}

// isExprNode returns true if the kind represents an expression (potential default value).
func isExprNode(kind string) bool {
	exprKinds := map[string]bool{
		"IntegerLiteral":           true,
		"FloatingLiteral":          true,
		"StringLiteral":            true,
		"CXXBoolLiteralExpr":       true,
		"CXXNullPtrLiteralExpr":    true,
		"CXXDefaultArgExpr":        true,
		"CXXConstructExpr":         true,
		"CXXTemporaryObjectExpr":   true,
		"CallExpr":                 true,
		"DeclRefExpr":              true,
		"ImplicitCastExpr":         true,
		"MaterializeTemporaryExpr": true,
		"ExprWithCleanups":         true,
		"InitListExpr":             true,
		"UnaryOperator":            true,
		"BinaryOperator":           true,
	}
	return exprKinds[kind]
}

// extractEnum extracts an Enum from an EnumDecl node.
func extractEnum(node *clangNode, namespace string) *Enum {
	var values []string
	for _, inner := range node.Inner {
		if inner.Kind == "EnumConstantDecl" && inner.Name != "" {
			values = append(values, inner.Name)
		}
	}

	return &Enum{
		Name:      node.Name,
		Namespace: namespace,
		Values:    values,
	}
}

// findFunctionParenthesis finds the index of the '(' that starts the function parameter list.
// This handles cases like "std::function<array(array)> (int)" where the first '(' is inside a template.
func findFunctionParenthesis(typeStr string) int {
	depth := 0
	for i, c := range typeStr {
		switch c {
		case '<':
			depth++
		case '>':
			depth--
		case '(':
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// normalizeType normalizes C++ type names for consistent lookup.
func normalizeType(t string) string {
	t = strings.TrimSpace(t)

	// Remove const at the beginning
	if strings.HasPrefix(t, "const ") {
		t = strings.TrimPrefix(t, "const ")
	}

	// Remove reference at the end
	t = strings.TrimSuffix(t, " &")
	t = strings.TrimSuffix(t, "&")

	// Normalize spacing around punctuation for consistent matching
	// Remove space before >
	t = strings.ReplaceAll(t, " >", ">")
	// Remove space before &
	t = strings.ReplaceAll(t, " &", "&")
	// Remove space after <
	t = strings.ReplaceAll(t, "< ", "<")
	// Remove space before (
	t = strings.ReplaceAll(t, " (", "(")
	// Remove space after )
	t = strings.ReplaceAll(t, ") ", ")")

	// Normalize multiple spaces to single
	t = strings.Join(strings.Fields(t), " ")

	// Common type normalizations
	t = strings.ReplaceAll(t, "class ", "")
	t = strings.ReplaceAll(t, "struct ", "")

	// Remove mlx::core:: namespace prefix for MLX types
	t = strings.ReplaceAll(t, "mlx::core::", "")

	return strings.TrimSpace(t)
}

// ParseString parses C++ header content directly (without clang).
// This is a fallback for when clang isn't available.
func ParseString(content string) (*ParseResult, error) {
	// Create a temp file and use clang
	tmpFile, err := os.CreateTemp("", "mlxcgen-*.h")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		return nil, err
	}
	tmpFile.Close()

	return ParseFile(tmpFile.Name())
}
