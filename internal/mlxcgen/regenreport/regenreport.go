package regenreport

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/tmc/mlx-c/internal/mlxcgen/customspec"
	"github.com/tmc/mlx-c/internal/mlxcgen/doccoverage"
	"github.com/tmc/mlx-c/internal/mlxcgen/inventory"
	"github.com/tmc/mlx-c/internal/mlxcgen/ir"
	"github.com/tmc/mlx-c/internal/mlxcgen/parser"
	"github.com/tmc/mlx-c/internal/mlxcgen/plan"
	"github.com/tmc/mlx-c/internal/mlxcgen/types"
	"gopkg.in/yaml.v3"
)

const SchemaVersion = 1

// Options controls a regeneration report run.
type Options struct {
	RepoRoot            string
	MLXSrc              string
	ManifestPath        string
	CustomDir           string
	TypePolicyPath      string
	CompileCommandsPath string
	InventoryPath       string
	WorkDir             string
	ASTCacheDir         string
	NoASTCache          bool
	FormatCacheDir      string
	NoFormatCache       bool
	Generator           []string
	NoFormat            bool
	KeepWork            bool
}

// Report records the result of a scratch-tree regeneration.
type Report struct {
	SchemaVersion             int                        `json:"schema_version"`
	RepoRoot                  string                     `json:"repo_root"`
	MLXSrc                    string                     `json:"mlx_src"`
	MLXRevision               string                     `json:"mlx_revision,omitempty"`
	MLXRef                    MLXRefStatus               `json:"mlx_ref"`
	ClangVersion              string                     `json:"clang_version,omitempty"`
	CompileCommandsPath       string                     `json:"compile_commands_path,omitempty"`
	ASTCacheDir               string                     `json:"ast_cache_dir,omitempty"`
	FormatCacheDir            string                     `json:"format_cache_dir,omitempty"`
	ManifestPath              string                     `json:"manifest_path,omitempty"`
	CustomDir                 string                     `json:"custom_dir,omitempty"`
	TypePolicyPath            string                     `json:"type_policy_path,omitempty"`
	InputDigests              InputDigests               `json:"input_digests"`
	TypePolicy                TypePolicy                 `json:"type_policy"`
	MissingTypes              []types.MissingType        `json:"missing_types,omitempty"`
	DocCoverage               doccoverage.Coverage       `json:"doc_coverage"`
	MissingDocs               []doccoverage.MissingDoc   `json:"missing_docs,omitempty"`
	GeneratedMarkerViolations []GeneratedMarkerViolation `json:"generated_marker_violations,omitempty"`
	Manifest                  ManifestInfo               `json:"manifest"`
	Modules                   []Module                   `json:"modules,omitempty"`
	CustomSpecs               []CustomSpec               `json:"custom_specs,omitempty"`
	InventoryPath             string                     `json:"inventory_path,omitempty"`
	Inventory                 []Inventory                `json:"inventory,omitempty"`
	WorkDir                   string                     `json:"work_dir"`
	OutputDir                 string                     `json:"output_dir"`
	MetadataPath              string                     `json:"metadata_path"`
	IR                        ir.Result                  `json:"ir,omitempty"`
	TypePolicyIR              ir.Result                  `json:"type_policy_ir,omitempty"`
	Diagnostics               []Diagnostic               `json:"diagnostics,omitempty"`
	SymbolChecks              []SymbolCheck              `json:"symbol_checks,omitempty"`
	Command                   []string                   `json:"command"`
	GeneratorOut              string                     `json:"generator_output,omitempty"`
	GeneratorErr              string                     `json:"generator_error,omitempty"`
	Summary                   Summary                    `json:"summary"`
	Files                     []FileReport               `json:"files"`
	GeneratedOnly             []string                   `json:"generated_only,omitempty"`
}

// MLXRefStatus records whether the parsed MLX checkout matches the reviewed ref.
type MLXRefStatus struct {
	ExpectedGitRef   string `json:"expected_git_ref,omitempty"`
	ExpectedRevision string `json:"expected_revision,omitempty"`
	ActualRevision   string `json:"actual_revision,omitempty"`
	MatchesExpected  bool   `json:"matches_expected"`
	Error            string `json:"error,omitempty"`
}

// InputDigests records the source-of-truth files that drove regeneration.
type InputDigests struct {
	Manifest    *PathDigest  `json:"manifest,omitempty"`
	ModuleFiles []PathDigest `json:"module_files,omitempty"`
	TypePolicy  *PathDigest  `json:"type_policy,omitempty"`
	CustomSpecs []PathDigest `json:"custom_specs,omitempty"`
	Inventory   *PathDigest  `json:"inventory,omitempty"`
}

// SymbolCheck records a shared-library symbol check result.
type SymbolCheck struct {
	Target          string   `json:"target"`
	Path            string   `json:"path"`
	Source          string   `json:"source"`
	LockedFunctions int      `json:"locked_functions"`
	DefinedSymbols  int      `json:"defined_symbols"`
	PublicSymbols   int      `json:"public_symbols"`
	Problems        []string `json:"problems,omitempty"`
}

// PathDigest records the SHA-256 digest of one input file.
type PathDigest struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// Module records one planned header-derived generator module.
type Module struct {
	Name    string   `json:"name"`
	Headers []string `json:"headers"`
	Outputs []string `json:"outputs"`
}

// ManifestInfo records review policy from the generator manifest.
type ManifestInfo struct {
	SchemaVersion    int                        `json:"schema_version"`
	MLX              plan.MLXPolicy             `json:"mlx,omitempty"`
	Report           plan.ReportPolicy          `json:"report,omitempty"`
	GeneratedMarkers plan.GeneratedMarkerPolicy `json:"generated_markers,omitempty"`
	CustomHooks      []plan.CustomHook          `json:"custom_hooks,omitempty"`
	HookAPI          []plan.HookAPI             `json:"hook_api,omitempty"`
}

// TypePolicy records the loaded generator type policy.
type TypePolicy struct {
	SchemaVersion int `json:"schema_version"`
	Types         int `json:"types"`
	MissingTypes  int `json:"missing_types"`
}

// Inventory records one generated-file inventory entry in the report.
type Inventory struct {
	Kind   string `json:"kind"`
	Target string `json:"target"`
	Path   string `json:"path"`
}

// CustomSpec records a loaded custom generation policy file.
type CustomSpec struct {
	Name            string           `json:"name"`
	Target          string           `json:"target"`
	Header          string           `json:"header"`
	Implementation  string           `json:"implementation,omitempty"`
	Ownership       string           `json:"ownership"`
	GeneratedHeader bool             `json:"generated_header,omitempty"`
	Items           int              `json:"items"`
	ActionCounts    []Count          `json:"action_counts,omitempty"`
	KindCounts      []Count          `json:"kind_counts,omitempty"`
	ReasonCounts    []Count          `json:"reason_counts,omitempty"`
	ItemDecisions   []CustomSpecItem `json:"item_decisions,omitempty"`
}

// Count records a named count in a report.
type Count struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// CustomSpecItem records one custom-spec declaration decision.
type CustomSpecItem struct {
	Kind      string                `json:"kind"`
	Name      string                `json:"name"`
	Action    string                `json:"action"`
	Reason    string                `json:"reason,omitempty"`
	Signature string                `json:"signature,omitempty"`
	Opaque    bool                  `json:"opaque,omitempty"`
	Values    []CustomSpecEnumValue `json:"values,omitempty"`
}

// CustomSpecEnumValue records one custom-spec enum constant.
type CustomSpecEnumValue struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

// Diagnostic records a generator diagnostic included in metadata.yaml.
type Diagnostic struct {
	Code    string `json:"code" yaml:"code"`
	Message string `json:"message" yaml:"message"`
	Reason  string `json:"reason,omitempty" yaml:"reason,omitempty"`
	File    string `json:"file,omitempty" yaml:"file,omitempty"`
	Line    int    `json:"line,omitempty" yaml:"line,omitempty"`
	Col     int    `json:"col,omitempty" yaml:"col,omitempty"`
}

// Summary counts per-file comparison states.
type Summary struct {
	Equal            int `json:"equal"`
	Different        int `json:"different"`
	MissingGenerated int `json:"missing_generated"`
	MissingCheckedIn int `json:"missing_checked_in"`
}

// FileReport records the comparison for one planned generated file.
type FileReport struct {
	Path            string `json:"path"`
	Status          string `json:"status"`
	CheckedBytes    int64  `json:"checked_bytes,omitempty"`
	GeneratedBytes  int64  `json:"generated_bytes,omitempty"`
	CheckedSHA256   string `json:"checked_sha256,omitempty"`
	GeneratedSHA256 string `json:"generated_sha256,omitempty"`
}

// GeneratedMarkerViolation records volatile data in a generated-file marker.
type GeneratedMarkerViolation struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
	Marker string `json:"marker"`
}

// Run creates a scratch output tree, runs the generator, and compares outputs.
func Run(opts Options) (*Report, error) {
	opts = resolveOptions(opts)
	inventoryPath := repoPath(opts.RepoRoot, opts.InventoryPath)
	if opts.MLXSrc == "" {
		return nil, fmt.Errorf("missing mlx source path")
	}
	manifest, err := plan.LoadPath(opts.ManifestPath)
	if err != nil {
		return nil, err
	}
	if err := manifest.CheckCMakeMLXRef(opts.RepoRoot); err != nil {
		return nil, err
	}
	typePolicyPath := repoPath(opts.RepoRoot, opts.TypePolicyPath)
	typePolicy, err := types.LoadPolicyPath(typePolicyPath)
	if err != nil {
		return nil, err
	}
	if err := typePolicy.CheckRegistry(types.NewRegistry()); err != nil {
		return nil, err
	}
	customSpecs, err := loadCustomSpecs(opts.RepoRoot, opts.CustomDir)
	if err != nil {
		return nil, err
	}
	inputDigests, err := reportInputDigests(
		opts.RepoRoot,
		manifestFilePath(opts.RepoRoot, opts.ManifestPath),
		typePolicyPath,
		inventoryPath,
		repoPath(opts.RepoRoot, opts.CustomDir),
		manifest,
	)
	if err != nil {
		return nil, err
	}
	inventoryEntries, err := checkInventory(opts.RepoRoot, inventoryPath, manifest, customSpecs)
	if err != nil {
		return nil, err
	}

	workDir := opts.WorkDir
	transientWorkDir := workDir == "" && !opts.KeepWork
	if workDir == "" {
		tmp, err := os.MkdirTemp("", "mlx-c-regen-*")
		if err != nil {
			return nil, fmt.Errorf("make temp dir: %w", err)
		}
		workDir = tmp
	}
	if !opts.KeepWork && opts.WorkDir == "" {
		defer os.RemoveAll(workDir)
	}
	outputDir := filepath.Join(workDir, "mlx", "c")
	if err := os.MkdirAll(filepath.Join(outputDir, "private"), 0o777); err != nil {
		return nil, fmt.Errorf("make output dir: %w", err)
	}
	metadataPath := filepath.Join(workDir, "metadata.yaml")

	args := generatorArgs(opts, outputDir, metadataPath)
	cmd := exec.Command(opts.Generator[0], args...)
	cmd.Dir = opts.RepoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("run generator: %w\n%s", err, strings.TrimSpace(string(out)))
	}

	outputs := generatedOutputs(manifest, customSpecs)
	modules := reportModules(manifest)
	clangVersion, err := commandOutputLine("clang++", "--version")
	if err != nil {
		clangVersion = ""
	}
	mlxRef := reportMLXRef(opts.MLXSrc, manifest.MLX)
	mlxRevision := mlxRef.ActualRevision
	report, err := Compare(opts.RepoRoot, outputDir, outputs)
	if err != nil {
		return nil, err
	}
	markerViolations, err := checkGeneratedMarkers(outputDir, outputs, manifest.GeneratedMarkers)
	if err != nil {
		return nil, err
	}
	metadata, err := readMetadata(metadataPath)
	if err != nil {
		return nil, err
	}
	typePolicyIR := metadata.TypePolicyIR
	if typePolicyIR.Empty() {
		typePolicyIR = metadata.IR
	}
	missingTypes := typePolicy.MissingIRTypes(typePolicyIR)
	docCoverage, missingDocs := doccoverage.Analyze(manifest, metadata.IR)
	report.SchemaVersion = SchemaVersion
	report.RepoRoot = opts.RepoRoot
	report.MLXSrc = opts.MLXSrc
	report.MLXRevision = mlxRevision
	report.MLXRef = mlxRef
	report.ClangVersion = clangVersion
	report.CompileCommandsPath = opts.CompileCommandsPath
	report.ASTCacheDir = opts.ASTCacheDir
	report.FormatCacheDir = opts.FormatCacheDir
	report.ManifestPath = opts.ManifestPath
	report.CustomDir = opts.CustomDir
	report.TypePolicyPath = typePolicyPath
	report.InputDigests = inputDigests
	report.TypePolicy = reportTypePolicy(typePolicy, missingTypes)
	report.MissingTypes = missingTypes
	report.DocCoverage = docCoverage
	report.MissingDocs = missingDocs
	report.GeneratedMarkerViolations = markerViolations
	report.Manifest = reportManifest(manifest)
	report.Modules = modules
	report.CustomSpecs = reportCustomSpecs(customSpecs)
	report.InventoryPath = inventoryPath
	report.Inventory = reportInventory(inventoryEntries)
	report.WorkDir = workDir
	report.OutputDir = outputDir
	report.MetadataPath = metadataPath
	report.IR = metadata.IR
	report.TypePolicyIR = typePolicyIR
	report.Diagnostics = metadata.Diagnostics
	report.Command = append([]string{opts.Generator[0]}, args...)
	report.GeneratorOut = string(out)
	if transientWorkDir {
		normalizeTransientWorkDir(report, workDir)
	}
	return report, nil
}

func resolveOptions(opts Options) Options {
	if opts.RepoRoot == "" {
		opts.RepoRoot = "."
	}
	if opts.InventoryPath == "" {
		opts.InventoryPath = filepath.Join(opts.RepoRoot, "codegen", "generated-files.txt")
	}
	if opts.TypePolicyPath == "" {
		opts.TypePolicyPath = filepath.Join(opts.RepoRoot, "codegen", "types.yaml")
	}
	if len(opts.Generator) == 0 {
		opts.Generator = []string{"go", "run", "./tools/mlx-c-gen"}
	}
	opts.ASTCacheDir = parser.ResolveASTCacheDir(opts.ASTCacheDir, opts.NoASTCache)
	opts.FormatCacheDir = ResolveFormatCacheDir(opts.FormatCacheDir, opts.NoFormatCache || opts.NoFormat)
	return opts
}

// ResolveFormatCacheDir returns the clang-format output cache directory.
func ResolveFormatCacheDir(explicit string, disabled bool) string {
	if disabled {
		return ""
	}
	if explicit != "" {
		return explicit
	}
	if env := os.Getenv("MLX_C_FORMAT_CACHE"); env != "" {
		return env
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(base, "mlx-c", "mlxcgen", "format")
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

func reportModules(manifest plan.Manifest) []Module {
	modules := make([]Module, 0, len(manifest.Headers))
	for _, hm := range manifest.Headers {
		modules = append(modules, Module{
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

func reportManifest(manifest plan.Manifest) ManifestInfo {
	return ManifestInfo{
		SchemaVersion:    manifest.SchemaVersion,
		MLX:              manifest.MLX,
		Report:           manifest.Report,
		GeneratedMarkers: manifest.GeneratedMarkers,
		CustomHooks:      append([]plan.CustomHook(nil), manifest.CustomHooks...),
		HookAPI:          append([]plan.HookAPI(nil), manifest.HookAPI...),
	}
}

func generatedOutputs(manifest plan.Manifest, specs []customspec.Spec) []string {
	outputs := append([]string{}, manifest.GeneratedOutputs()...)
	outputs = append(outputs, customspec.GeneratedHeaders(specs)...)
	sort.Strings(outputs)
	return outputs
}

func reportTypePolicy(policy types.Policy, missingTypes []types.MissingType) TypePolicy {
	return TypePolicy{
		SchemaVersion: policy.SchemaVersion,
		Types:         len(policy.Types),
		MissingTypes:  len(missingTypes),
	}
}

func reportInventory(entries []inventory.Entry) []Inventory {
	out := make([]Inventory, 0, len(entries))
	for _, entry := range entries {
		out = append(out, Inventory{
			Kind:   entry.Kind,
			Target: entry.Target,
			Path:   entry.Path,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		if out[i].Target != out[j].Target {
			return out[i].Target < out[j].Target
		}
		return out[i].Kind < out[j].Kind
	})
	return out
}

func loadCustomSpecs(root, dir string) ([]customspec.Spec, error) {
	if dir == "" {
		return nil, nil
	}
	specs, err := customspec.LoadDir(repoPath(root, dir))
	if err != nil {
		return nil, err
	}
	return specs, nil
}

func reportMLXRef(mlxSrc string, policy plan.MLXPolicy) MLXRefStatus {
	status := MLXRefStatus{
		ExpectedGitRef: policy.ExpectedGitRef,
	}
	if mlxSrc == "" {
		status.Error = "missing mlx source path"
		return status
	}
	actual, err := commandOutput("git", "-C", mlxSrc, "rev-parse", "HEAD")
	if err != nil {
		status.Error = err.Error()
		return status
	}
	status.ActualRevision = actual
	if policy.ExpectedGitRef == "" {
		return status
	}
	expected, err := commandOutput("git", "-C", mlxSrc, "rev-parse", policy.ExpectedGitRef+"^{commit}")
	if err != nil {
		status.Error = err.Error()
		return status
	}
	status.ExpectedRevision = expected
	status.MatchesExpected = actual == expected
	return status
}

func reportInputDigests(root, manifestPath, typePolicyPath, inventoryPath, customDir string, manifest plan.Manifest) (InputDigests, error) {
	var out InputDigests
	var err error
	out.Manifest, err = fileDigest(root, manifestPath)
	if err != nil {
		return InputDigests{}, err
	}
	for _, name := range manifest.ModuleFiles {
		path := filepath.Join(filepath.Dir(manifestPath), filepath.FromSlash(name))
		digest, err := fileDigest(root, path)
		if err != nil {
			return InputDigests{}, err
		}
		if digest != nil {
			out.ModuleFiles = append(out.ModuleFiles, *digest)
		}
	}
	out.TypePolicy, err = fileDigest(root, typePolicyPath)
	if err != nil {
		return InputDigests{}, err
	}
	out.Inventory, err = fileDigest(root, inventoryPath)
	if err != nil {
		return InputDigests{}, err
	}
	customSpecs, err := customSpecDigests(root, customDir)
	if err != nil {
		return InputDigests{}, err
	}
	out.CustomSpecs = customSpecs
	return out, nil
}

func customSpecDigests(root, dir string) ([]PathDigest, error) {
	if dir == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read custom spec dir for digests: %w", err)
	}
	var out []PathDigest
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		digest, err := fileDigest(root, filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		if digest != nil {
			out = append(out, *digest)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Path < out[j].Path
	})
	return out, nil
}

func manifestFilePath(root, path string) string {
	if path == "" {
		return filepath.Join(root, "codegen", "manifest.yaml")
	}
	return repoPath(root, path)
}

func fileDigest(root, path string) (*PathDigest, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("digest %s: %w", displayPath(root, path), err)
	}
	return &PathDigest{
		Path:   displayPath(root, path),
		SHA256: hash(data),
	}, nil
}

func displayPath(root, path string) string {
	absRoot, rootErr := filepath.Abs(root)
	absPath, pathErr := filepath.Abs(path)
	if rootErr == nil && pathErr == nil {
		if rel, err := filepath.Rel(absRoot, absPath); err == nil && rel != "." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".." && !filepath.IsAbs(rel) {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(filepath.Clean(path))
}

func reportCustomSpecs(specs []customspec.Spec) []CustomSpec {
	out := make([]CustomSpec, 0, len(specs))
	for _, spec := range specs {
		out = append(out, CustomSpec{
			Name:            spec.Name,
			Target:          spec.Target,
			Header:          spec.Header,
			Implementation:  spec.Implementation,
			Ownership:       spec.Ownership,
			GeneratedHeader: spec.Generate.Header,
			Items:           len(spec.Items),
			ActionCounts:    countCustomSpecItems(spec.Items, func(item customspec.Item) string { return item.Action }),
			KindCounts:      countCustomSpecItems(spec.Items, func(item customspec.Item) string { return item.Kind }),
			ReasonCounts:    countCustomSpecItems(spec.Items, func(item customspec.Item) string { return item.Reason }),
			ItemDecisions:   reportCustomSpecItems(spec.Items),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Target != out[j].Target {
			return out[i].Target < out[j].Target
		}
		if out[i].Header != out[j].Header {
			return out[i].Header < out[j].Header
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func reportCustomSpecItems(items []customspec.Item) []CustomSpecItem {
	out := make([]CustomSpecItem, 0, len(items))
	for _, item := range items {
		var values []CustomSpecEnumValue
		for _, value := range item.Values {
			values = append(values, CustomSpecEnumValue{
				Name:  value.Name,
				Value: value.Value,
			})
		}
		out = append(out, CustomSpecItem{
			Kind:      item.Kind,
			Name:      item.Name,
			Action:    item.Action,
			Reason:    item.Reason,
			Signature: item.Signature,
			Opaque:    item.Opaque,
			Values:    values,
		})
	}
	return out
}

func countCustomSpecItems(items []customspec.Item, key func(customspec.Item) string) []Count {
	counts := map[string]int{}
	for _, item := range items {
		name := key(item)
		if name != "" {
			counts[name]++
		}
	}
	out := make([]Count, 0, len(counts))
	for name, count := range counts {
		out = append(out, Count{Name: name, Count: count})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

func generatorArgs(opts Options, outputDir, metadataPath string) []string {
	args := append([]string{}, opts.Generator[1:]...)
	args = append(args,
		"--mlx-src", opts.MLXSrc,
		"--output-dir", outputDir,
		"--metadata", metadataPath,
	)
	if opts.ManifestPath != "" {
		args = append(args, "--manifest", opts.ManifestPath)
	}
	if opts.CustomDir != "" {
		args = append(args, "--custom-dir", opts.CustomDir)
	}
	if opts.CompileCommandsPath != "" {
		args = append(args, "--compile-commands", opts.CompileCommandsPath)
	}
	if opts.ASTCacheDir != "" {
		args = append(args, "--ast-cache", opts.ASTCacheDir)
	}
	if opts.NoASTCache {
		args = append(args, "--no-ast-cache")
	}
	if opts.FormatCacheDir != "" {
		args = append(args, "--format-cache", opts.FormatCacheDir)
	}
	if opts.NoFormatCache {
		args = append(args, "--no-format-cache")
	}
	if opts.NoFormat {
		args = append(args, "--no-format")
	}
	return args
}

type metadataReport struct {
	IR           ir.Result    `yaml:"ir"`
	TypePolicyIR ir.Result    `yaml:"type_policy_ir"`
	Diagnostics  []Diagnostic `yaml:"diagnostics"`
}

func readMetadata(path string) (metadataReport, error) {
	var meta metadataReport
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return meta, nil
	}
	if err != nil {
		return meta, fmt.Errorf("read metadata: %w", err)
	}
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return meta, fmt.Errorf("parse metadata: %w", err)
	}
	meta.IR.Sort()
	meta.TypePolicyIR.Sort()
	return meta, nil
}

func readMetadataDiagnostics(path string) ([]Diagnostic, error) {
	meta, err := readMetadata(path)
	if err != nil {
		return nil, err
	}
	return meta.Diagnostics, nil
}

// Compare compares planned generator outputs in repoRoot and outputDir.
func Compare(repoRoot, outputDir string, outputs []string) (*Report, error) {
	report := &Report{}
	outputSet := map[string]bool{}
	for _, path := range outputs {
		outputSet[path] = true
		file, err := compareOne(repoRoot, outputDir, path)
		if err != nil {
			return nil, err
		}
		report.Files = append(report.Files, file)
		switch file.Status {
		case "equal":
			report.Summary.Equal++
		case "different":
			report.Summary.Different++
		case "missing_generated":
			report.Summary.MissingGenerated++
		case "missing_checked_in":
			report.Summary.MissingCheckedIn++
		}
	}
	sort.Slice(report.Files, func(i, j int) bool {
		return report.Files[i].Path < report.Files[j].Path
	})
	generatedOnly, err := generatedOnlyFiles(outputDir, outputSet)
	if err != nil {
		return nil, err
	}
	report.GeneratedOnly = generatedOnly
	return report, nil
}

// JSON returns an indented JSON representation of r.
func (r *Report) JSON() ([]byte, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal report: %w", err)
	}
	return append(data, '\n'), nil
}

// Clean reports whether all planned generated files match.
func (r *Report) Clean() bool {
	return r.Summary.Different == 0 &&
		r.Summary.MissingGenerated == 0 &&
		r.Summary.MissingCheckedIn == 0 &&
		len(r.GeneratedOnly) == 0
}

func checkInventory(root, path string, manifest plan.Manifest, customSpecs []customspec.Spec) ([]inventory.Entry, error) {
	if manifest.Report.IncludeInventory {
		if err := inventory.Check(root, path); err != nil {
			return nil, err
		}
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	entries, err := inventory.Read(f)
	if err != nil {
		return nil, err
	}
	if manifest.Report.IncludeInventory {
		if err := manifest.CheckInventoryWithGenerated(entries, customspec.GeneratedHeaders(customSpecs)); err != nil {
			return nil, err
		}
	}
	return entries, nil
}

func repoPath(root, path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
}

func compareOne(repoRoot, outputDir, path string) (FileReport, error) {
	rel := strings.TrimPrefix(path, "mlx/c/")
	checkedPath := filepath.Join(repoRoot, filepath.FromSlash(path))
	generatedPath := filepath.Join(outputDir, filepath.FromSlash(rel))

	checked, checkedErr := os.ReadFile(checkedPath)
	generated, generatedErr := os.ReadFile(generatedPath)

	file := FileReport{Path: path}
	checkedMissing := os.IsNotExist(checkedErr)
	generatedMissing := os.IsNotExist(generatedErr)
	if checkedErr != nil && !checkedMissing {
		return file, fmt.Errorf("read %s: %w", checkedPath, checkedErr)
	}
	if generatedErr != nil && !generatedMissing {
		return file, fmt.Errorf("read %s: %w", generatedPath, generatedErr)
	}
	if !checkedMissing {
		file.CheckedBytes = int64(len(checked))
		file.CheckedSHA256 = hash(checked)
	}
	if !generatedMissing {
		file.GeneratedBytes = int64(len(generated))
		file.GeneratedSHA256 = hash(generated)
	}
	switch {
	case checkedMissing:
		file.Status = "missing_checked_in"
	case generatedMissing:
		file.Status = "missing_generated"
	default:
		if bytes.Equal(checked, generated) {
			file.Status = "equal"
		} else {
			file.Status = "different"
		}
	}
	return file, nil
}

func generatedOnlyFiles(outputDir string, outputs map[string]bool) ([]string, error) {
	var out []string
	err := filepath.WalkDir(outputDir, func(path string, d os.DirEntry, err error) error {
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
		rel, err := filepath.Rel(outputDir, path)
		if err != nil {
			return err
		}
		inventoryPath := "mlx/c/" + filepath.ToSlash(rel)
		if !outputs[inventoryPath] {
			out = append(out, inventoryPath)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk generated output: %w", err)
	}
	sort.Strings(out)
	return out, nil
}

var (
	generatedMarkerDateTimeRE = regexp.MustCompile(`\b[0-9]{4}-[0-9]{2}-[0-9]{2}([T ][0-9]{2}:[0-9]{2}(:[0-9]{2})?(Z|[-+][0-9]{2}:[0-9]{2})?)?\b`)
	generatedMarkerClockRE    = regexp.MustCompile(`\b[0-9]{2}:[0-9]{2}:[0-9]{2}\b`)
	generatedMarkerWinPathRE  = regexp.MustCompile(`[A-Za-z]:\\`)
)

func checkGeneratedMarkers(outputDir string, outputs []string, policy plan.GeneratedMarkerPolicy) ([]GeneratedMarkerViolation, error) {
	if !policy.ForbidVolatileData {
		return nil, nil
	}
	var out []GeneratedMarkerViolation
	for _, path := range outputs {
		rel := strings.TrimPrefix(path, "mlx/c/")
		data, err := os.ReadFile(filepath.Join(outputDir, filepath.FromSlash(rel)))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read generated marker %s: %w", path, err)
		}
		out = append(out, generatedMarkerViolations(path, data)...)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		if out[i].Reason != out[j].Reason {
			return out[i].Reason < out[j].Reason
		}
		return out[i].Marker < out[j].Marker
	})
	return out, nil
}

func generatedMarkerViolations(path string, data []byte) []GeneratedMarkerViolation {
	lines := leadingGeneratedMarkerLines(data)
	out := make([]GeneratedMarkerViolation, 0, len(lines))
	for _, line := range lines {
		if reason := generatedMarkerVolatileReason(line); reason != "" {
			out = append(out, GeneratedMarkerViolation{
				Path:   path,
				Reason: reason,
				Marker: line,
			})
		}
	}
	return out
}

func leadingGeneratedMarkerLines(data []byte) []string {
	var out []string
	inBlock := false
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if inBlock {
			out = append(out, line)
			if strings.Contains(line, "*/") {
				inBlock = false
			}
			continue
		}
		if strings.HasPrefix(line, "/*") {
			out = append(out, line)
			if !strings.Contains(line, "*/") {
				inBlock = true
			}
			continue
		}
		if strings.HasPrefix(line, "//") {
			out = append(out, line)
			continue
		}
		break
	}
	return out
}

func generatedMarkerVolatileReason(line string) string {
	lower := strings.ToLower(line)
	switch {
	case strings.Contains(line, "/private/tmp/") ||
		strings.Contains(line, "/tmp/") ||
		strings.Contains(line, "/var/folders/"):
		return "temp_path"
	case strings.Contains(line, "/Users/") ||
		strings.Contains(line, "/Volumes/") ||
		strings.Contains(line, "/home/") ||
		generatedMarkerWinPathRE.MatchString(line):
		return "host_path"
	case generatedMarkerDateTimeRE.MatchString(line) ||
		generatedMarkerClockRE.MatchString(line):
		return "timestamp"
	case strings.Contains(lower, "hostname") ||
		strings.Contains(lower, "host:") ||
		strings.Contains(lower, "host=") ||
		strings.Contains(lower, " host "):
		return "hostname"
	}
	return ""
}

func normalizeTransientWorkDir(report *Report, workDir string) {
	if report == nil || workDir == "" {
		return
	}
	normalize := func(s string) string {
		return strings.ReplaceAll(s, workDir, "<workdir>")
	}
	report.WorkDir = normalize(report.WorkDir)
	report.OutputDir = normalize(report.OutputDir)
	report.MetadataPath = normalize(report.MetadataPath)
	for i := range report.Command {
		report.Command[i] = normalize(report.Command[i])
	}
	report.GeneratorOut = normalize(report.GeneratorOut)
	report.GeneratorErr = normalize(report.GeneratorErr)
}

func hash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
