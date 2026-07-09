// Command mlxcgen generates C bindings for MLX from C++ headers.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/tmc/mlx-c/internal/mlxcgen/apilock"
	"github.com/tmc/mlx-c/internal/mlxcgen/customspec"
	"github.com/tmc/mlx-c/internal/mlxcgen/doccoverage"
	"github.com/tmc/mlx-c/internal/mlxcgen/generators"
	"github.com/tmc/mlx-c/internal/mlxcgen/hooks"
	"github.com/tmc/mlx-c/internal/mlxcgen/inventory"
	"github.com/tmc/mlx-c/internal/mlxcgen/ir"
	"github.com/tmc/mlx-c/internal/mlxcgen/parser"
	"github.com/tmc/mlx-c/internal/mlxcgen/plan"
	"github.com/tmc/mlx-c/internal/mlxcgen/regenreport"
	"github.com/tmc/mlx-c/internal/mlxcgen/symbols"
	"github.com/tmc/mlx-c/internal/mlxcgen/types"
)

var standaloneGenerators = map[string]func(mode string) string{
	"vector": func(mode string) string {
		var buf bytes.Buffer
		generators.GenerateVector(&buf, mode)
		return buf.String()
	},
	"closure": func(mode string) string {
		var buf bytes.Buffer
		generators.GenerateClosure(&buf, mode)
		return buf.String()
	},
	"map": func(mode string) string {
		var buf bytes.Buffer
		generators.GenerateMap(&buf, mode)
		return buf.String()
	},
}

func main() {
	generateArgs := os.Args[1:]
	defaultGenerateNoArgs := len(os.Args) == 1
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "check":
			if err := runCheck(os.Args[2:]); err != nil {
				if errors.Is(err, flag.ErrHelp) {
					return
				}
				fmt.Fprintf(os.Stderr, "mlx-c-gen check: %v\n", err)
				os.Exit(1)
			}
			return
		case "parse":
			if err := runParse(os.Args[2:]); err != nil {
				if errors.Is(err, flag.ErrHelp) {
					return
				}
				fmt.Fprintf(os.Stderr, "mlx-c-gen parse: %v\n", err)
				os.Exit(1)
			}
			return
		case "generate":
			generateArgs = os.Args[2:]
			os.Args = append([]string{os.Args[0]}, os.Args[2:]...)
		}
	}

	mlxSrc := flag.String("mlx-src", "", "Path to MLX source directory")
	outputRoot := flag.String("output-root", ".", "Repository root for generated files")
	outputDir := flag.String("output-dir", "", "Output directory for generated files")
	metadataPath := flag.String("metadata", "", "Path to output YAML metadata file")
	reportPath := flag.String("report", "", "Path to output generation report JSON file")
	manifestPath := flag.String("manifest", "", "Path to generator manifest")
	customDir := flag.String("custom-dir", "", "Path to custom generator specs")
	compileCommandsPath := flag.String("compile-commands", "", "Path to compile_commands.json for parser flags")
	astCacheDir := flag.String("ast-cache", "", "Cache parsed clang AST results under directory")
	noASTCache := flag.Bool("no-ast-cache", false, "Disable parsed clang AST cache")
	formatCacheDir := flag.String("format-cache", "", "Cache clang-format output under directory")
	noFormatCache := flag.Bool("no-format-cache", false, "Disable clang-format output cache")
	dryRun := flag.Bool("dry-run", false, "Print what would be done without doing it")
	noFormat := flag.Bool("no-format", false, "Skip running clang-format on generated files")
	flag.Parse()

	mlxSrcPath, err := discoverMLXSource(*mlxSrc)
	if err != nil {
		if defaultGenerateNoArgs && *mlxSrc == "" {
			flag.Usage()
			return
		}
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Using MLX source: %s\n", mlxSrcPath)

	// Set up include paths for the parser
	absMlxSrc, _ := filepath.Abs(mlxSrcPath)
	parser.SetIncludePaths([]string{absMlxSrc})
	parser.SetCompileCommandsPath(*compileCommandsPath)
	resolvedASTCacheDir := resolveASTCacheDir(*astCacheDir, *noASTCache)
	parser.SetASTCacheDir(resolvedASTCacheDir)
	resolvedFormatCacheDir := resolveFormatCacheDir(*formatCacheDir, *noFormatCache || *noFormat)

	// Output directory
	outDir := generateOutputDir(*outputDir, *outputRoot)
	privateDir := filepath.Join(outDir, "private")

	if err := prepareOutputDir(outDir, *dryRun); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	manifest, err := plan.LoadPath(*manifestPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
	headerMappings := manifest.Headers
	standaloneNames := manifest.Standalone
	customSpecs, err := customspec.LoadDir(*customDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Output directory: %s\n\n", outDir)

	if *dryRun {
		fmt.Println("DRY RUN - no files will be written")
		fmt.Println()
	}

	success := true

	// Generate header-based bindings
	fmt.Println("Generating header-based bindings...")

	// Initialize the generator
	gen := generators.NewWithManifest(manifest)

	// Parse all headers first to build a complete result for metadata
	// We need to collect all headers to parse them in one go if we want a single metadata file
	// But current structure parses per mapping.
	// For now, we'll collect results after parsing loop, or simpler:
	// We just used existing loop. To support metadata better, we might want to aggregate.
	// However, distinct mappings might have distinct needs.
	// Let's rely on the parser accumulating if we passed a single context, but we don't.
	//
	// Better approach for metadata:
	// Create a combined result holder if metadata is requested.
	combinedResult := &parser.ParseResult{
		Functions: make(map[string][]*parser.Function),
		Enums:     make(map[string]*parser.Enum),
	}
	combinedIR := ir.Result{}
	typePolicyIR := ir.Result{}

	for _, hm := range headerMappings {
		if *dryRun {
			fmt.Printf("  Would generate %s.h and %s.cpp from %v\n", hm.Name, hm.Name, hm.Headers)
			continue
		}

		// Build full paths
		var fullPaths []string
		allExist := true
		for _, h := range hm.Headers {
			fullPath := filepath.Join(mlxSrcPath, h)
			if _, err := os.Stat(fullPath); os.IsNotExist(err) {
				fmt.Printf("  ERROR: Header not found: %s\n", fullPath)
				allExist = false
				break
			}
			fullPaths = append(fullPaths, fullPath)
		}
		if !allExist {
			success = false
			continue
		}

		// Set pre-includes for this header (may be nil)
		parser.SetPreIncludes(hm.PreIncludes)

		// Parse headers
		result, err := parser.ParseFiles(fullPaths)
		if err != nil {
			fmt.Printf("  ERROR parsing %s: %v\n", hm.Name, err)
			success = false
			continue
		}

		// Aggregate for metadata
		if *metadataPath != "" {
			result.Diagnostics = append(result.Diagnostics, gen.Diagnostics(result)...)
			combinedIR = ir.Merge(combinedIR, ir.FromParseResult(hm.Name, result))
			selected, err := gen.SelectedGeneratedIR(hm.Name, result)
			if err == nil {
				typePolicyIR = ir.Merge(typePolicyIR, selected)
			}
			for k, v := range result.Functions {
				combinedResult.Functions[k] = v
			}
			for k, v := range result.Enums {
				combinedResult.Enums[k] = v
			}
			combinedResult.Diagnostics = append(combinedResult.Diagnostics, result.Diagnostics...)
		}

		// Generate .h file
		fmt.Printf("  Generating %s.h...\n", hm.Name)
		var hBuf bytes.Buffer
		if err := gen.Generate(&hBuf, result, hm.Name, fullPaths, false, hm.Docstring); err != nil {
			fmt.Printf("    ERROR: %v\n", err)
			success = false
			continue
		}
		hPath := filepath.Join(outDir, hm.Name+".h")
		if err := os.WriteFile(hPath, hBuf.Bytes(), 0644); err != nil {
			fmt.Printf("    ERROR writing %s: %v\n", hPath, err)
			success = false
			continue
		}

		// Generate .cpp file
		fmt.Printf("  Generating %s.cpp...\n", hm.Name)
		var cppBuf bytes.Buffer
		if err := gen.Generate(&cppBuf, result, hm.Name, fullPaths, true, hm.Docstring); err != nil {
			fmt.Printf("    ERROR: %v\n", err)
			success = false
			continue
		}
		cppPath := filepath.Join(outDir, hm.Name+".cpp")
		if err := os.WriteFile(cppPath, cppBuf.Bytes(), 0644); err != nil {
			fmt.Printf("    ERROR writing %s: %v\n", cppPath, err)
			success = false
			continue
		}
	}

	fmt.Println()

	// Generate metadata if requested
	if *metadataPath != "" && success {
		fmt.Printf("Generating metadata to %s...\n", *metadataPath)
		f, err := os.Create(*metadataPath)
		if err != nil {
			fmt.Printf("  ERROR creating metadata file: %v\n", err)
			success = false
		} else {
			defer f.Close()
			yg := generators.NewYaml()
			if err := yg.GenerateYamlWithTypePolicyIR(f, combinedResult, combinedIR, typePolicyIR); err != nil {
				fmt.Printf("  ERROR generating metadata: %v\n", err)
				success = false
			}
		}
	}

	// Generate standalone bindings
	fmt.Println("Generating standalone bindings...")
	for _, name := range standaloneNames {
		generate := standaloneGenerators[name]
		if generate == nil {
			fmt.Printf("  ERROR: no standalone generator for %s\n", name)
			success = false
			continue
		}
		if *dryRun {
			fmt.Printf("  Would generate %s.h, %s.cpp, private/%s.h\n", name, name, name)
			continue
		}

		// Generate .h file
		fmt.Printf("  Generating %s.h...\n", name)
		hContent := generate("header")
		hPath := filepath.Join(outDir, name+".h")
		if err := os.WriteFile(hPath, []byte(hContent), 0644); err != nil {
			fmt.Printf("    ERROR writing %s: %v\n", hPath, err)
			success = false
			continue
		}

		// Generate .cpp file
		fmt.Printf("  Generating %s.cpp...\n", name)
		cppContent := generate("impl")
		cppPath := filepath.Join(outDir, name+".cpp")
		if err := os.WriteFile(cppPath, []byte(cppContent), 0644); err != nil {
			fmt.Printf("    ERROR writing %s: %v\n", cppPath, err)
			success = false
			continue
		}

		// Generate private .h file
		fmt.Printf("  Generating private/%s.h...\n", name)
		privContent := generate("private")
		privPath := filepath.Join(privateDir, name+".h")
		if err := os.WriteFile(privPath, []byte(privContent), 0644); err != nil {
			fmt.Printf("    ERROR writing %s: %v\n", privPath, err)
			success = false
			continue
		}
	}

	fmt.Println()

	if len(customspec.GeneratedHeaders(customSpecs)) > 0 {
		fmt.Println("Generating custom spec bindings...")
		for _, spec := range customSpecs {
			if !spec.Generate.Header {
				continue
			}
			outPath, err := customHeaderOutputPath(outDir, spec.Header)
			if err != nil {
				fmt.Printf("  ERROR: %v\n", err)
				success = false
				continue
			}
			if *dryRun {
				fmt.Printf("  Would generate %s from custom spec %s\n", spec.Header, spec.Name)
				continue
			}
			fmt.Printf("  Generating %s...\n", spec.Header)
			data, err := customspec.RenderHeader(spec)
			if err != nil {
				fmt.Printf("    ERROR: %v\n", err)
				success = false
				continue
			}
			if err := os.MkdirAll(filepath.Dir(outPath), 0o777); err != nil {
				fmt.Printf("    ERROR creating %s: %v\n", filepath.Dir(outPath), err)
				success = false
				continue
			}
			if err := os.WriteFile(outPath, data, 0644); err != nil {
				fmt.Printf("    ERROR writing %s: %v\n", outPath, err)
				success = false
				continue
			}
		}
		fmt.Println()
	}

	// Run clang-format
	if success && !*dryRun && !*noFormat {
		fmt.Println("Running clang-format on generated files...")
		var files []string
		for _, hm := range headerMappings {
			files = append(files, filepath.Join(outDir, hm.Name+".h"))
			files = append(files, filepath.Join(outDir, hm.Name+".cpp"))
		}
		for _, name := range standaloneNames {
			files = append(files, filepath.Join(outDir, name+".h"))
			files = append(files, filepath.Join(outDir, name+".cpp"))
			files = append(files, filepath.Join(privateDir, name+".h"))
		}
		for _, out := range customspec.GeneratedHeaders(customSpecs) {
			path, err := customHeaderOutputPath(outDir, out)
			if err != nil {
				fmt.Printf("  WARNING: could not resolve custom header %s: %v\n", out, err)
				continue
			}
			files = append(files, path)
		}

		for _, f := range files {
			// Read, format via stdin with --assume-filename, and write back
			// The assume-filename makes clang-format look for .clang-format
			// based on that path, so we use mlx/c/<basename> to find our config
			content, err := os.ReadFile(f)
			if err != nil {
				fmt.Printf("  WARNING: could not read %s: %v\n", f, err)
				continue
			}

			// Build assumed filename relative to repo root
			base := filepath.Base(f)
			assumedPath := filepath.Join("mlx", "c", base)
			if strings.Contains(f, "private") {
				assumedPath = filepath.Join("mlx", "c", "private", base)
			}

			formatted, err := formatContent(content, assumedPath, resolvedFormatCacheDir)
			if err != nil {
				fmt.Printf("  WARNING: clang-format failed for %s: %v\n", f, err)
				continue
			}

			if err := os.WriteFile(f, formatted, 0644); err != nil {
				fmt.Printf("  WARNING: could not write %s: %v\n", f, err)
			}
		}
		fmt.Println()
	}

	if success && *reportPath != "" {
		report := newGenerateReport(generateReportOptions{
			Args:                generateArgs,
			OutputRoot:          *outputRoot,
			OutputDir:           outDir,
			MLXSrc:              mlxSrcPath,
			ManifestPath:        *manifestPath,
			CustomDir:           *customDir,
			CompileCommandsPath: *compileCommandsPath,
			ASTCacheDir:         resolvedASTCacheDir,
			FormatCacheDir:      resolvedFormatCacheDir,
			MetadataPath:        *metadataPath,
			Manifest:            manifest,
			CustomSpecs:         customSpecs,
			DryRun:              *dryRun,
			NoFormat:            *noFormat,
		})
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Printf("  ERROR marshaling report: %v\n", err)
			success = false
		} else if err := writeCheckReport(*reportPath, append(data, '\n')); err != nil {
			fmt.Printf("  ERROR writing report: %v\n", err)
			success = false
		}
	}

	if success {
		fmt.Println("Done!")
	} else {
		fmt.Println("Completed with errors.")
		os.Exit(1)
	}
}

type generateReport struct {
	SchemaVersion       int                      `json:"schema_version"`
	OutputRoot          string                   `json:"output_root"`
	OutputDir           string                   `json:"output_dir"`
	MLXSrc              string                   `json:"mlx_src"`
	MLXRevision         string                   `json:"mlx_revision,omitempty"`
	ClangVersion        string                   `json:"clang_version,omitempty"`
	ManifestPath        string                   `json:"manifest_path,omitempty"`
	CustomDir           string                   `json:"custom_dir,omitempty"`
	CompileCommandsPath string                   `json:"compile_commands_path,omitempty"`
	ASTCacheDir         string                   `json:"ast_cache_dir,omitempty"`
	FormatCacheDir      string                   `json:"format_cache_dir,omitempty"`
	MetadataPath        string                   `json:"metadata_path,omitempty"`
	Manifest            regenreport.ManifestInfo `json:"manifest"`
	Modules             []generateReportModule   `json:"modules,omitempty"`
	Standalone          []string                 `json:"standalone,omitempty"`
	GeneratedFiles      []string                 `json:"generated_files,omitempty"`
	Command             []string                 `json:"command"`
	Summary             generateReportSummary    `json:"summary"`
}

type generateReportModule struct {
	Name    string   `json:"name"`
	Headers []string `json:"headers"`
	Outputs []string `json:"outputs"`
}

type generateReportSummary struct {
	HeaderModules  int  `json:"header_modules"`
	Standalone     int  `json:"standalone"`
	GeneratedFiles int  `json:"generated_files"`
	DryRun         bool `json:"dry_run,omitempty"`
	NoFormat       bool `json:"no_format,omitempty"`
}

type generateReportOptions struct {
	Args                []string
	OutputRoot          string
	OutputDir           string
	MLXSrc              string
	ManifestPath        string
	CustomDir           string
	CompileCommandsPath string
	ASTCacheDir         string
	FormatCacheDir      string
	MetadataPath        string
	Manifest            plan.Manifest
	CustomSpecs         []customspec.Spec
	DryRun              bool
	NoFormat            bool
}

func generateOutputDir(outputDir, outputRoot string) string {
	if outputDir != "" {
		return outputDir
	}
	if outputRoot == "" || outputRoot == "." {
		return filepath.Join("mlx", "c")
	}
	return filepath.Join(outputRoot, "mlx", "c")
}

func customHeaderOutputPath(outDir, header string) (string, error) {
	header = filepath.ToSlash(header)
	rel, ok := strings.CutPrefix(header, "mlx/c/")
	if !ok || rel == "" {
		return "", fmt.Errorf("custom header %s is outside mlx/c", header)
	}
	relPath := filepath.Clean(filepath.FromSlash(rel))
	if relPath == "." || relPath == ".." || strings.HasPrefix(relPath, ".."+string(os.PathSeparator)) || filepath.IsAbs(relPath) {
		return "", fmt.Errorf("custom header %s is outside mlx/c", header)
	}
	return filepath.Join(outDir, relPath), nil
}

func newGenerateReport(opts generateReportOptions) generateReport {
	outputs := append([]string{}, opts.Manifest.GeneratedOutputs()...)
	outputs = append(outputs, customspec.GeneratedHeaders(opts.CustomSpecs)...)
	sort.Strings(outputs)
	report := generateReport{
		SchemaVersion:       regenreport.SchemaVersion,
		OutputRoot:          normalizedReportPath(opts.OutputRoot),
		OutputDir:           normalizedReportPath(opts.OutputDir),
		MLXSrc:              opts.MLXSrc,
		ManifestPath:        opts.ManifestPath,
		CustomDir:           opts.CustomDir,
		CompileCommandsPath: opts.CompileCommandsPath,
		ASTCacheDir:         opts.ASTCacheDir,
		FormatCacheDir:      normalizedReportPath(opts.FormatCacheDir),
		MetadataPath:        normalizedReportPath(opts.MetadataPath),
		Manifest: regenreport.ManifestInfo{
			SchemaVersion:    opts.Manifest.SchemaVersion,
			MLX:              opts.Manifest.MLX,
			Report:           opts.Manifest.Report,
			GeneratedMarkers: opts.Manifest.GeneratedMarkers,
			CustomHooks:      append([]plan.CustomHook(nil), opts.Manifest.CustomHooks...),
			HookAPI:          append([]plan.HookAPI(nil), opts.Manifest.HookAPI...),
		},
		Modules:        generateReportModules(opts.Manifest),
		Standalone:     append([]string(nil), opts.Manifest.Standalone...),
		GeneratedFiles: outputs,
		Command:        append([]string{"mlx-c-gen", "generate"}, normalizedGenerateCommandArgs(opts.Args)...),
		Summary: generateReportSummary{
			HeaderModules:  len(opts.Manifest.Headers),
			Standalone:     len(opts.Manifest.Standalone),
			GeneratedFiles: len(outputs),
			DryRun:         opts.DryRun,
			NoFormat:       opts.NoFormat,
		},
	}
	if clangVersion, err := commandOutputLine("clang++", "--version"); err == nil {
		report.ClangVersion = clangVersion
	}
	if mlxRevision, err := commandOutput("git", "-C", opts.MLXSrc, "rev-parse", "HEAD"); err == nil {
		report.MLXRevision = mlxRevision
	}
	return report
}

func generateReportModules(manifest plan.Manifest) []generateReportModule {
	modules := make([]generateReportModule, 0, len(manifest.Headers))
	for _, hm := range manifest.Headers {
		modules = append(modules, generateReportModule{
			Name:    hm.Name,
			Headers: append([]string(nil), hm.Headers...),
			Outputs: []string{
				"mlx/c/" + hm.Name + ".cpp",
				"mlx/c/" + hm.Name + ".h",
			},
		})
	}
	return modules
}

func normalizedGenerateCommandArgs(args []string) []string {
	return normalizedCommandPathArgs(args, map[string]bool{
		"--format-cache": true,
		"--metadata":     true,
		"--output-dir":   true,
		"--output-root":  true,
		"--report":       true,
	})
}

func normalizedCommandPathArgs(args []string, pathFlags map[string]bool) []string {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if pathFlags[arg] {
			out = append(out, arg)
			if i+1 < len(args) {
				out = append(out, normalizedReportPath(args[i+1]))
				i++
			}
			continue
		}
		name, value, ok := strings.Cut(arg, "=")
		if ok && pathFlags[name] {
			out = append(out, name+"="+normalizedReportPath(value))
			continue
		}
		out = append(out, arg)
	}
	return out
}

func normalizedReportPath(path string) string {
	if path == "" || path == "." {
		return path
	}
	if filepath.IsAbs(path) {
		return "<path>"
	}
	return path
}

var clangFormatVersionCache struct {
	once    sync.Once
	version string
	err     error
}

func formatContent(content []byte, assumedPath, cacheDir string) ([]byte, error) {
	key := ""
	if cacheDir != "" {
		var err error
		key, err = formatCacheKey(content, assumedPath)
		if err != nil {
			return nil, err
		}
		if data, ok, err := readFormatCache(cacheDir, key); err != nil {
			return nil, err
		} else if ok {
			return data, nil
		}
	}

	formatted, err := runClangFormat(content, assumedPath)
	if err != nil {
		return nil, err
	}
	if cacheDir != "" {
		if err := writeFormatCache(cacheDir, key, formatted); err != nil {
			return nil, err
		}
	}
	return formatted, nil
}

func runClangFormat(content []byte, assumedPath string) ([]byte, error) {
	cmd := exec.Command("clang-format", "--assume-filename="+assumedPath)
	cmd.Stdin = bytes.NewReader(content)
	var formatted bytes.Buffer
	cmd.Stdout = &formatted
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return formatted.Bytes(), nil
}

func formatCacheKey(content []byte, assumedPath string) (string, error) {
	version, err := clangFormatVersion()
	if err != nil {
		return "", err
	}
	contentSHA := sha256.Sum256(content)
	styleSHA, err := formatStyleSHA()
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(struct {
		Version    string `json:"version"`
		Clang      string `json:"clang"`
		Assumed    string `json:"assumed"`
		ContentSHA string `json:"content_sha"`
		StyleSHA   string `json:"style_sha"`
	}{
		Version:    "mlxcgen-format-v1",
		Clang:      version,
		Assumed:    assumedPath,
		ContentSHA: fmt.Sprintf("%x", contentSHA[:]),
		StyleSHA:   styleSHA,
	})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return fmt.Sprintf("%x", sum[:]), nil
}

func clangFormatVersion() (string, error) {
	clangFormatVersionCache.once.Do(func() {
		out, err := exec.Command("clang-format", "--version").Output()
		if err != nil {
			clangFormatVersionCache.err = fmt.Errorf("run clang-format --version: %w", err)
			return
		}
		clangFormatVersionCache.version = strings.TrimSpace(string(out))
	})
	return clangFormatVersionCache.version, clangFormatVersionCache.err
}

func formatStyleSHA() (string, error) {
	data, err := os.ReadFile(".clang-format")
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("read .clang-format: %w", err)
	}
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum[:]), nil
}

func readFormatCache(dir, key string) ([]byte, bool, error) {
	path := formatCachePath(dir, key)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("read format cache: %w", err)
	}
	return data, true, nil
}

func writeFormatCache(dir, key string, data []byte) error {
	path := formatCachePath(dir, key)
	if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
		return fmt.Errorf("create format cache dir: %w", err)
	}
	if err := writeFileAtomic(path, data, 0o666); err != nil {
		return fmt.Errorf("write format cache: %w", err)
	}
	return nil
}

func formatCachePath(dir, key string) string {
	return filepath.Join(dir, key[:2], key+".format")
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

type parseOptions struct {
	RepoRoot            string
	MLXSrc              string
	ManifestPath        string
	CustomDir           string
	TypePolicyPath      string
	CompileCommandsPath string
	InventoryPath       string
	ASTCacheDir         string
	NoASTCache          bool
	OutPath             string
	ReportPath          string
}

type parseReport struct {
	SchemaVersion  int                      `json:"schema_version"`
	RepoRoot       string                   `json:"repo_root"`
	MLXSrc         string                   `json:"mlx_src"`
	MLXRevision    string                   `json:"mlx_revision,omitempty"`
	ClangVersion   string                   `json:"clang_version,omitempty"`
	ASTCacheDir    string                   `json:"ast_cache_dir,omitempty"`
	ManifestPath   string                   `json:"manifest_path,omitempty"`
	CustomDir      string                   `json:"custom_dir,omitempty"`
	TypePolicyPath string                   `json:"type_policy_path,omitempty"`
	InventoryPath  string                   `json:"inventory_path,omitempty"`
	TypePolicy     regenreport.TypePolicy   `json:"type_policy"`
	DocCoverage    doccoverage.Coverage     `json:"doc_coverage"`
	Modules        []parseModule            `json:"modules,omitempty"`
	Summary        parseSummary             `json:"summary"`
	Decisions      []parseDecision          `json:"decisions,omitempty"`
	FileDecisions  []parseFileDecision      `json:"file_decisions,omitempty"`
	Diagnostics    []parseDiagnostic        `json:"diagnostics,omitempty"`
	MissingTypes   []types.MissingType      `json:"missing_types,omitempty"`
	MissingDocs    []doccoverage.MissingDoc `json:"missing_docs,omitempty"`
	Command        []string                 `json:"command"`
	IR             ir.Result                `json:"ir"`
}

type parseModule struct {
	Name      string   `json:"name"`
	Headers   []string `json:"headers"`
	Functions int      `json:"functions"`
	Enums     int      `json:"enums"`
}

type parseSummary struct {
	Functions     int `json:"functions"`
	Enums         int `json:"enums"`
	Diagnostics   int `json:"diagnostics"`
	MissingTypes  int `json:"missing_types"`
	MissingDocs   int `json:"missing_docs"`
	Decisions     int `json:"decisions"`
	FileDecisions int `json:"file_decisions"`
	Emits         int `json:"emits"`
	Hooks         int `json:"hooks"`
	Handwritten   int `json:"handwritten"`
	CustomSpecs   int `json:"custom_specs"`
	NotOwned      int `json:"not_owned"`
	Skips         int `json:"skips"`
}

type parseDiagnostic struct {
	Code    string    `json:"code"`
	DeclID  ir.DeclID `json:"decl_id,omitempty"`
	Message string    `json:"message"`
	Reason  string    `json:"reason,omitempty"`
	File    string    `json:"file,omitempty"`
	Line    int       `json:"line,omitempty"`
	Col     int       `json:"col,omitempty"`
}

type parseDecision struct {
	Source    string    `json:"source"`
	DeclID    ir.DeclID `json:"decl_id,omitempty"`
	Namespace string    `json:"namespace"`
	Function  string    `json:"function"`
	Signature string    `json:"signature"`
	Action    string    `json:"action"`
	CName     string    `json:"c_name,omitempty"`
	Suffix    string    `json:"suffix,omitempty"`
	Reason    string    `json:"reason,omitempty"`
	File      string    `json:"file,omitempty"`
	Line      int       `json:"line,omitempty"`
	Col       int       `json:"col,omitempty"`
}

type parseFileDecision struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Path   string `json:"path"`
	Action string `json:"action"`
	Reason string `json:"reason,omitempty"`
}

func runParse(args []string) error {
	opts, err := parseOptionsFromArgs(args)
	if err != nil {
		return err
	}
	if opts.MLXSrc == "" {
		mlxSrc, err := discoverMLXSource("")
		if err != nil {
			return err
		}
		opts.MLXSrc = mlxSrc
	}
	parsed, typePolicyIR, modules, diagnostics, err := parseIR(opts)
	if err != nil {
		return err
	}
	typePolicy, missingTypes, typeDiagnostics, err := parseTypePolicy(opts, typePolicyIR)
	if err != nil {
		return err
	}
	manifest, err := plan.LoadPath(opts.ManifestPath)
	if err != nil {
		return err
	}
	decisions, decisionSummary := parseVariantDecisions(manifest, parsed)
	detailDecisions, detailSummary := parseDetailDecisions(typePolicyIR)
	decisions = append(decisions, detailDecisions...)
	decisionSummary.Add(detailSummary)
	diagnosticDecisions, diagnosticSummary, err := parseDiagnosticDecisions(parsed, diagnostics)
	if err != nil {
		return err
	}
	decisions = append(decisions, diagnosticDecisions...)
	decisionSummary.Add(diagnosticSummary)
	if err := checkDecisionDeclIDs(manifest, decisions); err != nil {
		return err
	}
	if err := checkDecisionCoverage(manifest, parsed, decisions); err != nil {
		return err
	}
	customSpecs, err := loadCustomSpecs(opts.RepoRoot, opts.CustomDir)
	if err != nil {
		return err
	}
	if err := checkCNameUniqueness(manifest, decisions, customSpecs); err != nil {
		return err
	}
	fileDecisions, fileDecisionSummary, err := parseInventoryDecisions(opts, manifest)
	if err != nil {
		return err
	}
	docCoverage, missingDocs := doccoverage.Analyze(manifest, parsed)
	diagnostics = append(diagnostics, typeDiagnostics...)
	sortParseDiagnostics(diagnostics)
	if err := checkParseReportPolicy(manifest, docCoverage, missingTypes, diagnostics); err != nil {
		return err
	}
	data, err := json.MarshalIndent(parsed, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal parse IR: %w", err)
	}
	if err := writeCheckReport(opts.OutPath, append(data, '\n')); err != nil {
		return err
	}
	if opts.ReportPath == "" {
		return nil
	}
	report := parseReport{
		SchemaVersion:  regenreport.SchemaVersion,
		RepoRoot:       opts.RepoRoot,
		MLXSrc:         opts.MLXSrc,
		ASTCacheDir:    opts.ASTCacheDir,
		ManifestPath:   opts.ManifestPath,
		CustomDir:      opts.CustomDir,
		TypePolicyPath: opts.TypePolicyPath,
		InventoryPath:  opts.InventoryPath,
		TypePolicy:     typePolicy,
		DocCoverage:    docCoverage,
		Modules:        modules,
		Summary: parseSummary{
			Functions:     len(parsed.Functions),
			Enums:         len(parsed.Enums),
			Diagnostics:   len(diagnostics),
			MissingTypes:  len(missingTypes),
			MissingDocs:   len(missingDocs),
			Decisions:     len(decisions),
			FileDecisions: len(fileDecisions),
			Emits:         decisionSummary.Emits,
			Hooks:         decisionSummary.Hooks,
			Handwritten:   fileDecisionSummary.Handwritten,
			CustomSpecs:   fileDecisionSummary.CustomSpecs,
			NotOwned:      fileDecisionSummary.NotOwned,
			Skips:         decisionSummary.Skips,
		},
		Decisions:     decisions,
		FileDecisions: fileDecisions,
		Diagnostics:   diagnostics,
		MissingTypes:  missingTypes,
		MissingDocs:   missingDocs,
		Command:       append([]string{"mlx-c-gen", "parse"}, normalizedParseCommandArgs(args)...),
		IR:            parsed,
	}
	if clangVersion, err := commandOutputLine("clang++", "--version"); err == nil {
		report.ClangVersion = clangVersion
	}
	if mlxRevision, err := commandOutput("git", "-C", opts.MLXSrc, "rev-parse", "HEAD"); err == nil {
		report.MLXRevision = mlxRevision
	}
	reportData, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal parse report: %w", err)
	}
	return writeCheckReport(opts.ReportPath, append(reportData, '\n'))
}

func parseOptionsFromArgs(args []string) (parseOptions, error) {
	var opts parseOptions
	fs := flag.NewFlagSet("mlx-c-gen parse", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: mlx-c-gen parse [options]")
		fmt.Fprintln(os.Stderr)
		fs.PrintDefaults()
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Environment:")
		fmt.Fprintln(os.Stderr, "  MLX_C_AST_CACHE sets the default parsed clang AST cache directory.")
		fmt.Fprintln(os.Stderr, "  MLX_C_FORMAT_CACHE sets the default clang-format output cache directory.")
	}
	fs.StringVar(&opts.RepoRoot, "root", ".", "repository root")
	fs.StringVar(&opts.RepoRoot, "output-root", ".", "repository root (alias for --root)")
	fs.StringVar(&opts.MLXSrc, "mlx-src", "", "MLX source directory")
	fs.StringVar(&opts.ManifestPath, "manifest", "", "generator manifest path")
	fs.StringVar(&opts.CustomDir, "custom-dir", "", "custom generator spec directory")
	fs.StringVar(&opts.TypePolicyPath, "types", "", "type policy path")
	fs.StringVar(&opts.CompileCommandsPath, "compile-commands", "", "compile_commands.json path for parser flags")
	fs.StringVar(&opts.InventoryPath, "inventory", "codegen/generated-files.txt", "generated-file inventory path")
	fs.StringVar(&opts.InventoryPath, "generated-files", "codegen/generated-files.txt", "generated-file inventory path (alias for --inventory)")
	fs.StringVar(&opts.ASTCacheDir, "ast-cache", "", "cache parsed clang AST results under directory")
	fs.BoolVar(&opts.NoASTCache, "no-ast-cache", false, "disable parsed clang AST cache")
	fs.StringVar(&opts.OutPath, "out", "-", "write normalized IR JSON to path or - for stdout")
	fs.StringVar(&opts.ReportPath, "report", "", "write parse report JSON to path")
	if err := fs.Parse(args); err != nil {
		return opts, err
	}
	opts.ASTCacheDir = resolveASTCacheDir(opts.ASTCacheDir, opts.NoASTCache)
	if opts.TypePolicyPath == "" {
		opts.TypePolicyPath = filepath.Join(opts.RepoRoot, "codegen", "types.yaml")
	}
	return opts, nil
}

func normalizedParseCommandArgs(args []string) []string {
	return normalizedCommandPathArgs(args, map[string]bool{
		"--out":    true,
		"--report": true,
	})
}

type parseDecisionSummary struct {
	Emits int
	Hooks int
	Skips int
}

func (s *parseDecisionSummary) Add(other parseDecisionSummary) {
	s.Emits += other.Emits
	s.Hooks += other.Hooks
	s.Skips += other.Skips
}

type parseFileDecisionSummary struct {
	Handwritten int
	CustomSpecs int
	NotOwned    int
}

func parseVariantDecisions(manifest plan.Manifest, parsed ir.Result) ([]parseDecision, parseDecisionSummary) {
	var decisions []parseDecision
	var summary parseDecisionSummary
	seenHooks := map[string]bool{}
	declIDs := declIDsByDecisionKey(parsed)
	var namespaces []string
	for namespace := range manifest.VariantMappings {
		namespaces = append(namespaces, namespace)
	}
	sort.Strings(namespaces)
	for _, namespace := range namespaces {
		funcs := manifest.VariantMappings[namespace]
		var names []string
		for name := range funcs {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			for _, variant := range funcs[name] {
				decision := parseDecision{
					Source:    "variant_mapping",
					DeclID:    declIDs[parseDecisionKey(namespace, name, variant.Signature)],
					Namespace: namespace,
					Function:  name,
					Signature: variant.Signature,
				}
				if variant.Skip {
					decision.Action = "skip"
					decision.Reason = variant.Reason
					if decision.Reason == "" {
						decision.Reason = "variant_mapping"
					}
					summary.Skips++
				} else {
					suffix := ""
					if variant.Suffix != nil {
						suffix = *variant.Suffix
					}
					decision.Suffix = suffix
					decision.CName = variantCName(namespace, name, suffix)
					if hooks.HasHook(decision.CName) {
						decision.Action = "hook"
						decision.Reason = "custom_hook"
						seenHooks[decision.CName] = true
						summary.Hooks++
					} else {
						decision.Action = "emit"
						summary.Emits++
					}
				}
				decisions = append(decisions, decision)
			}
		}
	}
	for _, name := range hooks.Names() {
		if seenHooks[name] {
			continue
		}
		if hook, ok := customHookByCName(manifest.CustomHooks, name); ok {
			decisions = append(decisions, parseDecision{
				Source:   "custom_hook",
				Function: strings.TrimPrefix(name, "mlx_"),
				Action:   "hook",
				CName:    name,
				Reason:   hook.Reason,
			})
			seenHooks[name] = true
			summary.Hooks++
			continue
		}
		decisions = append(decisions, parseDecision{
			Source:   "hook_registry",
			Function: strings.TrimPrefix(name, "mlx_"),
			Action:   "hook",
			CName:    name,
			Reason:   "custom_hook_unmatched",
		})
		summary.Hooks++
	}
	return decisions, summary
}

func parseDetailDecisions(selected ir.Result) ([]parseDecision, parseDecisionSummary) {
	var decisions []parseDecision
	var summary parseDecisionSummary
	for _, fn := range selected.Functions {
		if fn.Namespace != "mlx::core::detail" {
			continue
		}
		namespace := strings.ReplaceAll(fn.Namespace, "::", "_")
		decisions = append(decisions, parseDecision{
			Source:    "allowed_detail_function",
			DeclID:    fn.ID,
			Namespace: namespace,
			Function:  fn.Name,
			Signature: parseDecisionSignature(fn),
			Action:    "emit",
			CName:     variantCName(namespace, fn.Name, ""),
		})
		summary.Emits++
	}
	return decisions, summary
}

func parseDiagnosticDecisions(parsed ir.Result, diagnostics []parseDiagnostic) ([]parseDecision, parseDecisionSummary, error) {
	decls := funcDeclsByID(parsed)
	var decisions []parseDecision
	var summary parseDecisionSummary
	for _, diagnostic := range diagnostics {
		if parserSkipDiagnostic(diagnostic.Code) {
			decision, ok := parserDiagnosticDecision(diagnostic)
			if !ok {
				return nil, parseDecisionSummary{}, fmt.Errorf("parser diagnostic %s missing qualified name: %s", diagnostic.Code, diagnostic.Message)
			}
			decisions = append(decisions, decision)
			summary.Skips++
			continue
		}
		source, ok := detailDiagnosticSource(diagnostic.Code)
		if !ok || diagnostic.DeclID == "" {
			continue
		}
		fn, ok := decls[diagnostic.DeclID]
		if !ok {
			continue
		}
		decisions = append(decisions, parseDecision{
			Source:    source,
			DeclID:    fn.ID,
			Namespace: strings.ReplaceAll(fn.Namespace, "::", "_"),
			Function:  fn.Name,
			Signature: parseDecisionSignature(fn),
			Action:    "skip",
			Reason:    diagnostic.Reason,
		})
		summary.Skips++
	}
	return decisions, summary, nil
}

func detailDiagnosticSource(code string) (string, bool) {
	switch code {
	case "skip_allowed_detail_overload":
		return "allowed_detail_overload", true
	case "skip_unallowed_detail_function":
		return "unallowed_detail_function", true
	default:
		return "", false
	}
}

func parserSkipDiagnostic(code string) bool {
	switch code {
	case "skip_builtin",
		"skip_missing_type",
		"skip_operator",
		"skip_stream_return",
		"skip_template_function",
		"skip_unparsed_function":
		return true
	default:
		return false
	}
}

func parserDiagnosticDecision(diagnostic parseDiagnostic) (parseDecision, bool) {
	namespace, name, ok := diagnosticQualifiedName(diagnostic.Message)
	if !ok {
		return parseDecision{}, false
	}
	return parseDecision{
		Source:    "parser_diagnostic",
		Namespace: strings.ReplaceAll(namespace, "::", "_"),
		Function:  name,
		Action:    "skip",
		Reason:    diagnostic.Reason,
		File:      parseDiagnosticFile(diagnostic.File),
		Line:      diagnostic.Line,
		Col:       diagnostic.Col,
	}, true
}

func diagnosticQualifiedName(message string) (namespace, name string, ok bool) {
	qualified, _, found := strings.Cut(message, " ")
	if !found {
		return "", "", false
	}
	i := strings.LastIndex(qualified, "::")
	if i < 0 {
		return "", "", false
	}
	return qualified[:i], qualified[i+2:], true
}

func funcDeclsByID(parsed ir.Result) map[ir.DeclID]ir.FuncDecl {
	decls := map[ir.DeclID]ir.FuncDecl{}
	for _, fn := range parsed.Functions {
		decls[fn.ID] = fn
	}
	return decls
}

func declIDsByDecisionKey(parsed ir.Result) map[string]ir.DeclID {
	out := map[string]ir.DeclID{}
	for _, fn := range parsed.Functions {
		namespace := strings.ReplaceAll(fn.Namespace, "::", "_")
		out[parseDecisionKey(namespace, fn.Name, parseDecisionSignature(fn))] = fn.ID
	}
	return out
}

func parseDecisionKey(namespace, name, signature string) string {
	return namespace + "|" + name + "|" + signature
}

func parseDecisionSignature(fn ir.FuncDecl) string {
	params := make([]string, 0, len(fn.Params))
	for _, param := range fn.Params {
		params = append(params, param.Type)
	}
	return fn.Return + "(" + strings.Join(params, ", ") + ")"
}

func checkDecisionDeclIDs(manifest plan.Manifest, decisions []parseDecision) error {
	if !manifest.Report.RequireDecisionDeclIDs {
		return nil
	}
	for _, decision := range decisions {
		if decisionNeedsDeclID(decision) && decision.DeclID == "" {
			return fmt.Errorf("variant decision missing declaration id for %s %s", decision.Function, decision.Signature)
		}
	}
	return nil
}

func checkDecisionCoverage(manifest plan.Manifest, parsed ir.Result, decisions []parseDecision) error {
	if !manifest.Report.RequireDecisionCoverage {
		return nil
	}
	want := map[ir.DeclID]bool{}
	for _, fn := range parsed.Functions {
		want[fn.ID] = true
	}
	seen := map[ir.DeclID]int{}
	var problems []string
	for _, decision := range decisions {
		if decision.DeclID == "" {
			continue
		}
		if !want[decision.DeclID] {
			problems = append(problems, fmt.Sprintf("decision for unknown declaration id %s", decision.DeclID))
			continue
		}
		seen[decision.DeclID]++
	}
	for id := range want {
		if seen[id] == 0 {
			problems = append(problems, fmt.Sprintf("missing decision for declaration id %s", id))
		}
	}
	for id, count := range seen {
		if count > 1 {
			problems = append(problems, fmt.Sprintf("declaration id %s has %d decisions", id, count))
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("decision coverage check failed:\n%s", strings.Join(problems, "\n"))
	}
	return nil
}

func checkCNameUniqueness(manifest plan.Manifest, decisions []parseDecision, customSpecs []customspec.Spec) error {
	if !manifest.Report.RequireUniqueCNames {
		return nil
	}
	owners := map[string][]string{}
	add := func(name, owner string) {
		if name == "" {
			return
		}
		owners[name] = append(owners[name], owner)
	}
	for _, decision := range decisions {
		if decision.Action != "emit" {
			continue
		}
		add(decision.CName, decisionCNameOwner(decision))
	}
	for _, api := range manifest.HookAPI {
		for _, name := range api.Names {
			add(name, "hook_api "+api.CName)
		}
	}
	for _, spec := range customSpecs {
		for _, item := range spec.Items {
			add(item.Name, customSpecCNameOwner(spec.Name, item))
			if item.Kind == "enum" {
				for _, value := range item.Values {
					add(value.Name, "custom_spec "+spec.Name+" enum:"+item.Name+" value:"+value.Name)
				}
			}
		}
	}
	var problems []string
	for name, nameOwners := range owners {
		if len(nameOwners) > 1 {
			sort.Strings(nameOwners)
			problems = append(problems, fmt.Sprintf("public C name %s has multiple owners: %s", name, strings.Join(nameOwners, ", ")))
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("public C name uniqueness check failed:\n%s", strings.Join(problems, "\n"))
	}
	return nil
}

func customSpecCNameOwner(specName string, item customspec.Item) string {
	return "custom_spec " + specName + " " + item.Kind + ":" + item.Name
}

func decisionCNameOwner(decision parseDecision) string {
	if decision.Namespace != "" && decision.Function != "" {
		if decision.Signature != "" {
			return decision.Source + " " + decision.Namespace + "." + decision.Function + " " + decision.Signature
		}
		return decision.Source + " " + decision.Namespace + "." + decision.Function
	}
	if decision.Function != "" {
		return decision.Source + " " + decision.Function
	}
	return decision.Source
}

func reportCNameDecisions(manifest plan.Manifest, report *regenreport.Report) []parseDecision {
	if report == nil {
		return nil
	}
	decisions, _ := parseVariantDecisions(manifest, report.IR)
	detailDecisions, _ := parseDetailDecisions(report.TypePolicyIR)
	return append(decisions, detailDecisions...)
}

func checkReportCNameUniqueness(opts checkOptions, report *regenreport.Report) error {
	if report == nil || !report.Manifest.Report.RequireUniqueCNames {
		return nil
	}
	manifest, err := plan.LoadPath(opts.Options.ManifestPath)
	if err != nil {
		return err
	}
	customSpecs, err := loadCustomSpecs(opts.Options.RepoRoot, opts.Options.CustomDir)
	if err != nil {
		return err
	}
	return checkCNameUniqueness(manifest, reportCNameDecisions(manifest, report), customSpecs)
}

func loadCustomSpecs(root, dir string) ([]customspec.Spec, error) {
	if dir == "" {
		return nil, nil
	}
	return customspec.LoadDir(repoPath(root, dir))
}

func decisionNeedsDeclID(decision parseDecision) bool {
	return decision.Source == "variant_mapping" ||
		decision.Source == "allowed_detail_function" ||
		decision.Source == "allowed_detail_overload" ||
		decision.Source == "unallowed_detail_function"
}

func customHookByCName(customHooks []plan.CustomHook, cName string) (plan.CustomHook, bool) {
	for _, hook := range customHooks {
		if hook.CName == cName {
			return hook, true
		}
	}
	return plan.CustomHook{}, false
}

func parseInventoryDecisions(opts parseOptions, manifest plan.Manifest) ([]parseFileDecision, parseFileDecisionSummary, error) {
	path := repoPath(opts.RepoRoot, opts.InventoryPath)
	var customGenerated []string
	if manifest.Report.IncludeInventory {
		if err := inventory.Check(opts.RepoRoot, path); err != nil {
			return nil, parseFileDecisionSummary{}, err
		}
		if opts.CustomDir != "" {
			customSpecs, err := customspec.LoadDir(repoPath(opts.RepoRoot, opts.CustomDir))
			if err != nil {
				return nil, parseFileDecisionSummary{}, err
			}
			customGenerated = customspec.GeneratedHeaders(customSpecs)
		}
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, parseFileDecisionSummary{}, fmt.Errorf("open inventory: %w", err)
	}
	defer f.Close()
	entries, err := inventory.Read(f)
	if err != nil {
		return nil, parseFileDecisionSummary{}, err
	}
	if manifest.Report.IncludeInventory {
		if err := manifest.CheckInventoryWithGenerated(entries, customGenerated); err != nil {
			return nil, parseFileDecisionSummary{}, err
		}
	}
	decisions := make([]parseFileDecision, 0, len(entries))
	var summary parseFileDecisionSummary
	for _, entry := range entries {
		decision := parseFileDecision{
			Source: "inventory",
			Target: entry.Target,
			Path:   entry.Path,
			Action: inventoryAction(entry.Kind),
			Reason: entry.Kind,
		}
		switch decision.Action {
		case "handwritten":
			summary.Handwritten++
		case "custom_spec":
			summary.CustomSpecs++
		case "not_owned":
			summary.NotOwned++
		}
		decisions = append(decisions, decision)
	}
	sort.Slice(decisions, func(i, j int) bool {
		if decisions[i].Path != decisions[j].Path {
			return decisions[i].Path < decisions[j].Path
		}
		if decisions[i].Target != decisions[j].Target {
			return decisions[i].Target < decisions[j].Target
		}
		return decisions[i].Action < decisions[j].Action
	})
	return decisions, summary, nil
}

func inventoryAction(kind string) string {
	switch kind {
	case "generated_header_api", "generated_support":
		return "emit"
	case "custom_spec_generated":
		return "custom_spec"
	case "handwritten_runtime":
		return "handwritten"
	case "not_owned_by_codegen":
		return "not_owned"
	default:
		return kind
	}
}

func variantCName(namespace, name, suffix string) string {
	cname := variantCPrefix(namespace) + "_" + name
	if suffix != "" {
		cname += "_" + suffix
	}
	return cname
}

func variantCPrefix(namespace string) string {
	parts := strings.Split(namespace, "_")
	if len(parts) >= 2 && parts[0] == "mlx" && parts[1] == "core" {
		parts = append(parts[:1], parts[2:]...)
		if len(parts) == 2 && parts[1] == "cu" {
			parts[1] = "cuda"
		}
	}
	return strings.Join(parts, "_")
}

func parseIR(opts parseOptions) (ir.Result, ir.Result, []parseModule, []parseDiagnostic, error) {
	manifest, err := plan.LoadPath(opts.ManifestPath)
	if err != nil {
		return ir.Result{}, ir.Result{}, nil, nil, err
	}
	if err := manifest.CheckCMakeMLXRef(opts.RepoRoot); err != nil {
		return ir.Result{}, ir.Result{}, nil, nil, err
	}
	absMlxSrc, err := filepath.Abs(opts.MLXSrc)
	if err != nil {
		return ir.Result{}, ir.Result{}, nil, nil, fmt.Errorf("resolve MLX source: %w", err)
	}
	parser.SetIncludePaths([]string{absMlxSrc})
	parser.SetCompileCommandsPath(opts.CompileCommandsPath)
	parser.SetASTCacheDir(opts.ASTCacheDir)

	gen := generators.NewWithManifest(manifest)
	var parsed ir.Result
	var typePolicyIR ir.Result
	var modules []parseModule
	var diagnostics []parseDiagnostic
	for _, hm := range manifest.Headers {
		fullPaths, err := headerPaths(opts.MLXSrc, hm.Headers)
		if err != nil {
			return ir.Result{}, ir.Result{}, nil, nil, err
		}
		parser.SetPreIncludes(hm.PreIncludes)
		result, err := parser.ParseFiles(fullPaths)
		if err != nil {
			return ir.Result{}, ir.Result{}, nil, nil, fmt.Errorf("parse %s: %w", hm.Name, err)
		}
		moduleIR := ir.FromParseResult(hm.Name, result)
		parsed = ir.Merge(parsed, moduleIR)
		selected, err := gen.SelectedGeneratedIR(hm.Name, result)
		if err == nil {
			typePolicyIR = ir.Merge(typePolicyIR, selected)
		}
		modules = append(modules, parseModule{
			Name:      hm.Name,
			Headers:   append([]string(nil), hm.Headers...),
			Functions: len(moduleIR.Functions),
			Enums:     len(moduleIR.Enums),
		})
		diagnostics = append(diagnostics, convertParserDiagnostics(result.Diagnostics)...)
		diagnostics = append(diagnostics, generatorDiagnostics(gen, result)...)
	}
	parsed.Sort()
	typePolicyIR.Sort()
	enrichDiagnosticsWithDeclIDs(diagnostics, parsed)
	sortParseDiagnostics(diagnostics)
	return parsed, typePolicyIR, modules, diagnostics, nil
}

func generatorDiagnostics(gen *generators.Generator, result *parser.ParseResult) []parseDiagnostic {
	return convertParserDiagnostics(gen.Diagnostics(result))
}

func convertParserDiagnostics(diagnostics []parser.Diagnostic) []parseDiagnostic {
	out := make([]parseDiagnostic, 0, len(diagnostics))
	for _, d := range diagnostics {
		out = append(out, parseDiagnostic{
			Code:    d.Code,
			Message: d.Message,
			Reason:  d.Reason,
			File:    d.File,
			Line:    d.Line,
			Col:     d.Col,
		})
	}
	return out
}

func enrichDiagnosticsWithDeclIDs(diagnostics []parseDiagnostic, parsed ir.Result) {
	ids := declIDsByLocation(parsed)
	for i := range diagnostics {
		if diagnostics[i].DeclID != "" {
			continue
		}
		if id := ids[diagnosticLocationKey(diagnostics[i])]; id != "" {
			diagnostics[i].DeclID = id
		}
	}
}

func declIDsByLocation(parsed ir.Result) map[string]ir.DeclID {
	ids := map[string]ir.DeclID{}
	for _, fn := range parsed.Functions {
		ids[locationKey(fn.Loc.File, fn.Loc.Line, fn.Loc.Col)] = fn.ID
	}
	return ids
}

func diagnosticLocationKey(diagnostic parseDiagnostic) string {
	return locationKey(parseDiagnosticFile(diagnostic.File), diagnostic.Line, diagnostic.Col)
}

func locationKey(file string, line, col int) string {
	if file == "" || line == 0 || col == 0 {
		return ""
	}
	return fmt.Sprintf("%s:%d:%d", file, line, col)
}

func parseDiagnosticFile(file string) string {
	file = filepath.ToSlash(filepath.Clean(file))
	if file == "." {
		return ""
	}
	if strings.HasPrefix(file, "mlx/") {
		return file
	}
	if i := strings.LastIndex(file, "/mlx/"); i >= 0 {
		return file[i+1:]
	}
	return file
}

func parseTypePolicy(opts parseOptions, parsed ir.Result) (regenreport.TypePolicy, []types.MissingType, []parseDiagnostic, error) {
	path := repoPath(opts.RepoRoot, opts.TypePolicyPath)
	policy, err := types.LoadPolicyPath(path)
	if err != nil {
		return regenreport.TypePolicy{}, nil, nil, err
	}
	if err := policy.CheckRegistry(types.NewRegistry()); err != nil {
		return regenreport.TypePolicy{}, nil, nil, err
	}
	missing := policy.MissingIRTypes(parsed)
	diagnostics := make([]parseDiagnostic, 0, len(missing))
	for _, miss := range missing {
		diagnostics = append(diagnostics, parseDiagnostic{
			Code: "missing_type_policy",
			Message: fmt.Sprintf("%s %s type %q has no type policy mapping",
				miss.Function, miss.Role, miss.Type),
			File: miss.Loc.File,
			Line: miss.Loc.Line,
			Col:  miss.Loc.Col,
		})
	}
	return regenreport.TypePolicy{
		SchemaVersion: policy.SchemaVersion,
		Types:         len(policy.Types),
		MissingTypes:  len(missing),
	}, missing, diagnostics, nil
}

func checkParseReportPolicy(manifest plan.Manifest, docCoverage doccoverage.Coverage, missingTypes []types.MissingType, diagnostics []parseDiagnostic) error {
	report := &regenreport.Report{
		Manifest: regenreport.ManifestInfo{
			Report: manifest.Report,
		},
		DocCoverage:  docCoverage,
		MissingTypes: missingTypes,
		Diagnostics:  reportDiagnostics(diagnostics),
	}
	if err := checkDocCoverage(report, false); err != nil {
		return err
	}
	if err := checkTypeCoverage(report); err != nil {
		return err
	}
	if err := checkDiagnosticReasons(report); err != nil {
		return err
	}
	if err := checkExplicitVariants(report); err != nil {
		return err
	}
	return nil
}

func reportDiagnostics(diagnostics []parseDiagnostic) []regenreport.Diagnostic {
	out := make([]regenreport.Diagnostic, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		out = append(out, regenreport.Diagnostic{
			Code:    diagnostic.Code,
			Message: diagnostic.Message,
			Reason:  diagnostic.Reason,
			File:    diagnostic.File,
			Line:    diagnostic.Line,
			Col:     diagnostic.Col,
		})
	}
	return out
}

func headerPaths(mlxSrc string, headers []string) ([]string, error) {
	paths := make([]string, 0, len(headers))
	for _, header := range headers {
		path := filepath.Join(mlxSrc, header)
		if _, err := os.Stat(path); err != nil {
			return nil, fmt.Errorf("header %s: %w", path, err)
		}
		paths = append(paths, path)
	}
	return paths, nil
}

func sortParseDiagnostics(diagnostics []parseDiagnostic) {
	sort.Slice(diagnostics, func(i, j int) bool {
		a, b := diagnostics[i], diagnostics[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Col != b.Col {
			return a.Col < b.Col
		}
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		return a.Message < b.Message
	})
}

func runCheck(args []string) error {
	opts, err := parseCheckOptions(args)
	if err != nil {
		return err
	}
	if opts.Options.MLXSrc == "" {
		mlxSrc, err := discoverMLXSource("")
		if err != nil {
			return err
		}
		opts.Options.MLXSrc = mlxSrc
	}
	report, err := regenreport.Run(opts.Options)
	if err != nil {
		return err
	}
	data, err := report.JSON()
	if err != nil {
		return err
	}
	if err := writeCheckReport(opts.ReportPath, data); err != nil {
		return err
	}
	if err := checkReportCNameUniqueness(opts, report); err != nil {
		return err
	}
	if err := checkAPILock(opts, report); err != nil {
		return err
	}
	if len(opts.Symbols) > 0 {
		results, err := symbols.Report(symbols.Options{
			LockPath: opts.LockPath,
			NM:       opts.NM,
			Targets:  opts.Symbols,
		})
		report.SymbolChecks = reportSymbolChecks(results)
		if len(results) > 0 {
			data, jsonErr := report.JSON()
			if jsonErr != nil {
				return jsonErr
			}
			if writeErr := writeCheckReport(opts.ReportPath, data); writeErr != nil {
				return writeErr
			}
		}
		if err != nil {
			return err
		}
	}
	if err := checkGeneratedClean(report, opts.StrictGenerated); err != nil {
		return err
	}
	if err := checkDocCoverage(report, opts.StrictDocs); err != nil {
		return err
	}
	if err := checkTypeCoverage(report); err != nil {
		return err
	}
	if err := checkDiagnosticReasons(report); err != nil {
		return err
	}
	if err := checkExplicitVariants(report); err != nil {
		return err
	}
	if err := checkGeneratedMarkers(report); err != nil {
		return err
	}
	if err := checkHookManifest(opts.Options.ManifestPath); err != nil {
		return err
	}
	return nil
}

func checkGeneratedClean(report *regenreport.Report, strict bool) error {
	if report == nil {
		return nil
	}
	if !strict && !report.Manifest.Report.RequireCleanGenerated {
		return nil
	}
	if !report.Clean() {
		return fmt.Errorf("regenerated files differ")
	}
	return nil
}

func reportSymbolChecks(results []symbols.Result) []regenreport.SymbolCheck {
	out := make([]regenreport.SymbolCheck, 0, len(results))
	for _, result := range results {
		out = append(out, regenreport.SymbolCheck{
			Target:          result.Target,
			Path:            result.Path,
			Source:          result.Source,
			LockedFunctions: result.LockedFunctions,
			DefinedSymbols:  result.DefinedSymbols,
			PublicSymbols:   result.PublicSymbols,
			Problems:        append([]string(nil), result.Problems...),
		})
	}
	return out
}

func checkDocCoverage(report *regenreport.Report, strict bool) error {
	if report == nil {
		return nil
	}
	if !strict && !report.Manifest.Report.RequireDocCoverage {
		return nil
	}
	if report.DocCoverage.Missing != 0 {
		return fmt.Errorf("generated declarations missing doc source")
	}
	return nil
}

func checkTypeCoverage(report *regenreport.Report) error {
	if report == nil || !report.Manifest.Report.RequireTypeCoverage {
		return nil
	}
	if len(report.MissingTypes) != 0 {
		return fmt.Errorf("selected declarations missing type policy entries")
	}
	for _, diagnostic := range report.Diagnostics {
		if isUnsupportedTypeDiagnostic(diagnostic.Code) {
			return fmt.Errorf("selected declarations have unsupported types")
		}
	}
	return nil
}

func checkDiagnosticReasons(report *regenreport.Report) error {
	if report == nil || !report.Manifest.Report.RequireDiagnosticReasons {
		return nil
	}
	for _, diagnostic := range report.Diagnostics {
		if strings.HasPrefix(diagnostic.Code, "skip_") && diagnostic.Reason == "" {
			return fmt.Errorf("skip diagnostic %s missing reason", diagnostic.Code)
		}
	}
	return nil
}

func checkExplicitVariants(report *regenreport.Report) error {
	if report == nil || !report.Manifest.Report.RequireExplicitVariants {
		return nil
	}
	for _, diagnostic := range report.Diagnostics {
		if diagnostic.Code == "emit_implicit_single_overload" {
			return fmt.Errorf("implicit variant selection remains")
		}
	}
	return nil
}

func isUnsupportedTypeDiagnostic(code string) bool {
	return code == "skip_unsupported_param_type" || code == "skip_unsupported_return_type"
}

func checkGeneratedMarkers(report *regenreport.Report) error {
	if report == nil || !report.Manifest.GeneratedMarkers.ForbidVolatileData {
		return nil
	}
	if len(report.GeneratedMarkerViolations) != 0 {
		return fmt.Errorf("generated markers contain volatile data")
	}
	return nil
}

func checkHookManifest(manifestPath string) error {
	manifest, err := plan.LoadPath(manifestPath)
	if err != nil {
		return err
	}
	return checkHookManifestPolicy(manifest)
}

func checkHookManifestPolicy(manifest plan.Manifest) error {
	declared := map[string]bool{}
	for namespace, funcs := range manifest.VariantMappings {
		for name, variants := range funcs {
			for _, variant := range variants {
				if variant.Skip || variant.Suffix == nil {
					continue
				}
				cName := variantCName(namespace, name, *variant.Suffix)
				if hooks.HasHook(cName) {
					declared[cName] = true
				}
			}
		}
	}
	var problems []string
	for _, hook := range manifest.CustomHooks {
		if !hooks.HasHook(hook.CName) {
			problems = append(problems, fmt.Sprintf("manifest declares unknown custom hook %s", hook.CName))
			continue
		}
		declared[hook.CName] = true
	}
	apiDeclared := map[string]bool{}
	for _, api := range manifest.HookAPI {
		if !hooks.HasHook(api.CName) {
			problems = append(problems, fmt.Sprintf("manifest declares API for unknown hook %s", api.CName))
			continue
		}
		if !declared[api.CName] {
			problems = append(problems, fmt.Sprintf("manifest declares API for undeclared hook %s", api.CName))
		}
		if err := checkHookAPIOutput(api); err != nil {
			problems = append(problems, err.Error())
		}
		apiDeclared[api.CName] = true
	}
	for _, name := range hooks.Names() {
		if !declared[name] {
			problems = append(problems, fmt.Sprintf("hook %s is registered but not declared in manifest", name))
		}
		if declared[name] && !apiDeclared[name] {
			problems = append(problems, fmt.Sprintf("hook %s is declared but has no hook_api entry", name))
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("hook manifest check failed:\n%s", strings.Join(problems, "\n"))
	}
	return nil
}

func checkHookAPIOutput(api plan.HookAPI) error {
	got, err := hookPublicNames(api.CName)
	if err != nil {
		return err
	}
	want := map[string]bool{}
	for _, name := range api.Names {
		want[name] = true
	}
	gotSet := map[string]bool{}
	var problems []string
	for _, name := range got {
		gotSet[name] = true
		if !want[name] {
			problems = append(problems, fmt.Sprintf("hook %s emits public API name %s not listed in hook_api", api.CName, name))
		}
	}
	for _, name := range api.Names {
		if !gotSet[name] {
			problems = append(problems, fmt.Sprintf("hook %s hook_api name %s is not emitted by hook", api.CName, name))
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("%s", strings.Join(problems, "\n"))
	}
	return nil
}

func hookPublicNames(cName string) ([]string, error) {
	hook := hooks.GetHook(cName)
	if hook == nil {
		return nil, fmt.Errorf("unknown hook %s", cName)
	}
	var buf bytes.Buffer
	if !hook(&buf, cName, false) {
		return nil, fmt.Errorf("hook %s did not handle header generation", cName)
	}
	target, err := apilock.ParseHeaderContent(cName+".h", buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("parse hook %s API: %w", cName, err)
	}
	return lockTargetNames(target), nil
}

type checkOptions struct {
	Options         regenreport.Options
	LockPath        string
	LockTUPath      string
	ReportPath      string
	NM              string
	Symbols         []symbols.TargetLibrary
	StrictGenerated bool
	StrictDocs      bool
}

func parseCheckOptions(args []string) (checkOptions, error) {
	var opts checkOptions
	fs := flag.NewFlagSet("mlx-c-gen check", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: mlx-c-gen check [options]")
		fmt.Fprintln(os.Stderr)
		fs.PrintDefaults()
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Environment:")
		fmt.Fprintln(os.Stderr, "  MLX_C_AST_CACHE sets the default parsed clang AST cache directory.")
		fmt.Fprintln(os.Stderr, "  MLX_C_FORMAT_CACHE sets the default clang-format output cache directory.")
	}
	var symbolTargets targetFlags
	var repoRoot string
	var inventoryPath string
	fs.StringVar(&repoRoot, "root", ".", "repository root")
	fs.StringVar(&repoRoot, "output-root", ".", "repository root (alias for --root)")
	mlxSrc := fs.String("mlx-src", "", "MLX source directory")
	manifestPath := fs.String("manifest", "", "generator manifest path")
	customDir := fs.String("custom-dir", "", "custom generator spec directory")
	compileCommandsPath := fs.String("compile-commands", "", "compile_commands.json path for parser flags")
	fs.StringVar(&inventoryPath, "inventory", "codegen/generated-files.txt", "generated-file inventory path")
	fs.StringVar(&inventoryPath, "generated-files", "codegen/generated-files.txt", "generated-file inventory path (alias for --inventory)")
	typePolicyPath := fs.String("types", "", "type policy path")
	lockPath := fs.String("lock", "codegen/mlxc-capi.lock.json", "API lock JSON path")
	lockTUPath := fs.String("lock-tu", "codegen/lock.c", "generated API lock translation unit path")
	reportPath := fs.String("report", "", "write regeneration report JSON to path instead of stdout")
	workDir := fs.String("work-dir", "", "scratch work directory")
	astCacheDir := fs.String("ast-cache", "", "cache parsed clang AST results under directory")
	noASTCache := fs.Bool("no-ast-cache", false, "disable parsed clang AST cache")
	formatCacheDir := fs.String("format-cache", "", "cache clang-format output under directory")
	noFormatCache := fs.Bool("no-format-cache", false, "disable clang-format output cache")
	generator := fs.String("generator", defaultGeneratorCommand(), "generator command")
	nm := fs.String("nm", "nm", "nm command for optional symbol checks")
	noFormat := fs.Bool("no-format", false, "pass --no-format to mlx-c-gen")
	keepWork := fs.Bool("keep-work", false, "keep an auto-created scratch directory")
	strictGenerated := fs.Bool("strict-generated", false, "fail when generated files differ from checked-in artifacts")
	strictDocs := fs.Bool("strict-docs", false, "fail when generated declarations have no doc source")
	fs.Var(&symbolTargets, "symbol", "optional symbol check target=library; may be repeated")
	if err := fs.Parse(args); err != nil {
		return opts, err
	}
	opts.Options = regenreport.Options{
		RepoRoot:            repoRoot,
		MLXSrc:              *mlxSrc,
		ManifestPath:        *manifestPath,
		CustomDir:           *customDir,
		TypePolicyPath:      *typePolicyPath,
		CompileCommandsPath: *compileCommandsPath,
		InventoryPath:       inventoryPath,
		WorkDir:             *workDir,
		ASTCacheDir:         resolveASTCacheDir(*astCacheDir, *noASTCache),
		NoASTCache:          *noASTCache,
		FormatCacheDir:      resolveFormatCacheDir(*formatCacheDir, *noFormatCache || *noFormat),
		NoFormatCache:       *noFormatCache,
		Generator:           strings.Fields(*generator),
		NoFormat:            *noFormat,
		KeepWork:            *keepWork,
	}
	opts.LockPath = *lockPath
	opts.LockTUPath = *lockTUPath
	opts.ReportPath = *reportPath
	opts.NM = *nm
	opts.Symbols = symbolTargets
	opts.StrictGenerated = *strictGenerated
	opts.StrictDocs = *strictDocs
	return opts, nil
}

func writeCheckReport(path string, data []byte) error {
	if path == "" || path == "-" {
		if _, err := os.Stdout.Write(data); err != nil {
			return fmt.Errorf("write stdout: %w", err)
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
		return fmt.Errorf("create report directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create report temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write report temp file: %w", err)
	}
	if err := tmp.Chmod(0o666); err != nil {
		tmp.Close()
		return fmt.Errorf("chmod report temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close report temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("write report %s: %w", path, err)
	}
	return nil
}

func commandOutput(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func commandOutputLine(name string, args ...string) (string, error) {
	out, err := commandOutput(name, args...)
	if err != nil {
		return "", err
	}
	line, _, _ := strings.Cut(out, "\n")
	return line, nil
}

func defaultGeneratorCommand() string {
	exe, err := os.Executable()
	if err == nil && exe != "" {
		return exe
	}
	return "go run ./tools/mlx-c-gen"
}

func resolveASTCacheDir(explicit string, disabled bool) string {
	return parser.ResolveASTCacheDir(explicit, disabled)
}

func resolveFormatCacheDir(explicit string, disabled bool) string {
	return regenreport.ResolveFormatCacheDir(explicit, disabled)
}

type targetFlags []symbols.TargetLibrary

func (f *targetFlags) String() string {
	var parts []string
	for _, tl := range *f {
		parts = append(parts, tl.Target+"="+tl.Path)
	}
	return strings.Join(parts, ",")
}

func (f *targetFlags) Set(s string) error {
	target, path, ok := strings.Cut(s, "=")
	if !ok || target == "" || path == "" {
		return fmt.Errorf("expected target=library")
	}
	*f = append(*f, symbols.TargetLibrary{Target: target, Path: path})
	return nil
}

func checkAPILock(opts checkOptions, report *regenreport.Report) error {
	if opts.LockPath == "" {
		if report != nil && report.Manifest.Report.RequireAPILock {
			return fmt.Errorf("manifest requires API lock but no lock path is configured")
		}
		return nil
	}
	headersDir := filepath.Join(opts.Options.RepoRoot, "mlx", "c")
	lock, err := apilock.Generate(headersDir)
	if err != nil {
		return err
	}
	if err := checkCustomHooksLocked(report, lock); err != nil {
		return err
	}
	if err := checkHookAPILocked(report, lock); err != nil {
		return err
	}
	manifest, err := plan.LoadPath(opts.Options.ManifestPath)
	if err != nil {
		return err
	}
	if err := checkEmittedAPILocked(report, manifest, lock); err != nil {
		return err
	}
	jsonData, err := lock.JSON()
	if err != nil {
		return err
	}
	if err := checkFile(repoPath(opts.Options.RepoRoot, opts.LockPath), jsonData); err != nil {
		return err
	}
	if err := checkCustomSpecs(opts, lock); err != nil {
		return err
	}
	if opts.LockTUPath != "" {
		tuData, err := lock.LockC()
		if err != nil {
			return err
		}
		if err := checkFile(repoPath(opts.Options.RepoRoot, opts.LockTUPath), tuData); err != nil {
			return err
		}
	}
	return nil
}

func checkEmittedAPILocked(report *regenreport.Report, manifest plan.Manifest, lock *apilock.Lock) error {
	if report == nil || lock == nil || !manifest.Report.RequireEmitAPILock {
		return nil
	}
	decisions, _ := parseVariantDecisions(manifest, report.IR)
	detailDecisions, _ := parseDetailDecisions(report.TypePolicyIR)
	decisions = append(decisions, detailDecisions...)
	var problems []string
	for _, decision := range decisions {
		if decision.Action != "emit" || decision.CName == "" {
			continue
		}
		targetName := customHookLockTarget(decision.CName)
		target, ok := lock.Targets[targetName]
		if !ok {
			problems = append(problems, fmt.Sprintf("emit %s requires missing %s API lock target", decision.CName, targetName))
			continue
		}
		if !lockTargetHasFunction(target, decision.CName) {
			problems = append(problems, fmt.Sprintf("emit %s missing from %s API lock functions", decision.CName, targetName))
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("api lock emitted function check failed:\n%s", strings.Join(problems, "\n"))
	}
	return nil
}

func checkCustomHooksLocked(report *regenreport.Report, lock *apilock.Lock) error {
	if report == nil || lock == nil {
		return nil
	}
	var problems []string
	for _, hook := range report.Manifest.CustomHooks {
		targetName := customHookLockTarget(hook.CName)
		target, ok := lock.Targets[targetName]
		if !ok {
			problems = append(problems, fmt.Sprintf("custom hook %s requires missing %s API lock target", hook.CName, targetName))
			continue
		}
		if !lockTargetHasName(target, hook.CName) {
			problems = append(problems, fmt.Sprintf("custom hook %s missing from %s API lock", hook.CName, targetName))
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("api lock custom hook check failed:\n%s", strings.Join(problems, "\n"))
	}
	return nil
}

func checkHookAPILocked(report *regenreport.Report, lock *apilock.Lock) error {
	if report == nil || lock == nil {
		return nil
	}
	var problems []string
	for _, api := range report.Manifest.HookAPI {
		for _, name := range api.Names {
			targetName := customHookLockTarget(name)
			target, ok := lock.Targets[targetName]
			if !ok {
				problems = append(problems, fmt.Sprintf("hook %s API name %s requires missing %s API lock target", api.CName, name, targetName))
				continue
			}
			if !lockTargetHasName(target, name) {
				problems = append(problems, fmt.Sprintf("hook %s API name %s missing from %s API lock", api.CName, name, targetName))
			}
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("api lock hook api check failed:\n%s", strings.Join(problems, "\n"))
	}
	return nil
}

func customHookLockTarget(name string) string {
	if strings.HasPrefix(name, "mlx_jaccl_") {
		return "jacclc"
	}
	return "mlxc"
}

func lockTargetHasFunction(target apilock.Target, name string) bool {
	for _, fn := range target.Functions {
		if fn.Name == name {
			return true
		}
	}
	return false
}

func lockTargetHasName(target apilock.Target, name string) bool {
	names := lockTargetNames(target)
	i := sort.SearchStrings(names, name)
	return i < len(names) && names[i] == name
}

func lockTargetNames(target apilock.Target) []string {
	seen := map[string]bool{}
	var names []string
	add := func(name string) {
		if name != "" && !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	for _, macro := range target.Macros {
		add(macro.Name)
	}
	for _, typedef := range target.Typedefs {
		add(typedef.Name)
	}
	for _, st := range target.Structs {
		add(st.Name)
	}
	for _, enum := range target.Enums {
		add(enum.Name)
		for _, value := range enum.Values {
			add(value.Name)
		}
	}
	for _, fn := range target.Functions {
		add(fn.Name)
	}
	sort.Strings(names)
	return names
}

func checkCustomSpecs(opts checkOptions, lock *apilock.Lock) error {
	if opts.Options.CustomDir == "" {
		return nil
	}
	specs, err := customspec.LoadDir(repoPath(opts.Options.RepoRoot, opts.Options.CustomDir))
	if err != nil {
		return err
	}
	if err := customspec.CheckLock(lock, specs); err != nil {
		return err
	}
	if err := customspec.CheckImplementations(opts.Options.RepoRoot, specs); err != nil {
		return err
	}
	for _, spec := range specs {
		if !spec.Generate.Header {
			continue
		}
		data, err := customspec.RenderHeader(spec)
		if err != nil {
			return err
		}
		data, err = formatContent(data, spec.Header, opts.Options.FormatCacheDir)
		if err != nil {
			return fmt.Errorf("format custom header %s: %w", spec.Header, err)
		}
		if err := checkFile(repoPath(opts.Options.RepoRoot, spec.Header), data); err != nil {
			return err
		}
	}
	return nil
}

func repoPath(root, path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
}

func checkFile(path string, want []byte) error {
	got, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if !bytes.Equal(got, want) {
		return fmt.Errorf("%s is out of date", path)
	}
	return nil
}

func prepareOutputDir(outDir string, dryRun bool) error {
	if dryRun {
		return nil
	}
	if outDir == "" {
		return fmt.Errorf("missing output directory")
	}
	if err := os.MkdirAll(filepath.Join(outDir, "private"), 0o777); err != nil {
		return fmt.Errorf("create output directory %s: %w", outDir, err)
	}
	return nil
}

func discoverMLXSource(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	candidates := []string{
		"build/_deps/mlx-src",
		"build-Debug/_deps/mlx-src",
		"build-Release/_deps/mlx-src",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	return "", fmt.Errorf("could not find MLX source directory; please build the project first or specify --mlx-src")
}

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "Generate C bindings for MLX from C++ headers.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Subcommands:")
		fmt.Fprintln(os.Stderr, "  generate   Generate bindings. This is the default.")
		fmt.Fprintln(os.Stderr, "  parse      Parse upstream headers into normalized IR JSON.")
		fmt.Fprintln(os.Stderr, "  check      Regenerate into a scratch tree and verify API locks.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Options:")
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Environment:")
		fmt.Fprintln(os.Stderr, "  MLX_C_AST_CACHE sets the default parsed clang AST cache directory.")
		fmt.Fprintln(os.Stderr, "  MLX_C_FORMAT_CACHE sets the default clang-format output cache directory.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Example:")
		fmt.Fprintln(os.Stderr, "  mlxcgen --mlx-src=build/_deps/mlx-src")
		fmt.Fprintln(os.Stderr, "  mlxcgen check --mlx-src=build/_deps/mlx-src")
	}
}
