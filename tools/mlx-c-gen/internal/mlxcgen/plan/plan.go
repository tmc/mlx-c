package plan

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"github.com/tmc/mlx-c/tools/mlx-c-gen/internal/mlxcgen/inventory"
	"gopkg.in/yaml.v3"
)

const (
	defaultManifestPath = "codegen/manifest.yaml"
	SchemaVersion       = 1
)

var (
	cmakeGitTagRE = regexp.MustCompile(`\bGIT_TAG\s+([^\s)]+)`)
	cIdentRE      = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	planNameRE    = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
)

// HeaderMapping defines a header-derived binding output.
type HeaderMapping struct {
	Name        string   `yaml:"name"`
	Headers     []string `yaml:"headers"`
	Docstring   string   `yaml:"doc"`
	PreIncludes []string `yaml:"pre_includes,omitempty"`
}

// Manifest describes the generator output plan.
type Manifest struct {
	SchemaVersion          int                             `yaml:"schema_version"`
	MLX                    MLXPolicy                       `yaml:"mlx,omitempty"`
	Upstream               UpstreamPolicy                  `yaml:"upstream,omitempty"`
	Report                 ReportPolicy                    `yaml:"report,omitempty"`
	GeneratedMarkers       GeneratedMarkerPolicy           `yaml:"generated_markers,omitempty"`
	ModuleFiles            []string                        `yaml:"module_files,omitempty"`
	Headers                []HeaderMapping                 `yaml:"headers"`
	Standalone             []string                        `yaml:"standalone"`
	CustomHooks            []CustomHook                    `yaml:"custom_hooks,omitempty"`
	HookAPI                []HookAPI                       `yaml:"hook_api,omitempty"`
	VariantMappings        map[string]map[string][]Variant `yaml:"variant_mappings,omitempty"`
	AllowedDetailFunctions []string                        `yaml:"allowed_detail_functions,omitempty"`
}

// MLXPolicy records the upstream MLX revision the manifest was reviewed
// against.
type MLXPolicy struct {
	ExpectedGitRef  string `yaml:"expected_git_ref,omitempty" json:"expected_git_ref,omitempty"`
	ReleaseRevision int    `yaml:"release_revision,omitempty" json:"release_revision,omitempty"`
}

// UpstreamPolicy records the upstream mlx-c surface that the shared-symbol
// parity gate compares against. Ref names a git ref in this repository, so the
// gate needs no checkout of its own; HeadersDir is the header directory within
// that ref.
type UpstreamPolicy struct {
	Ref        string `yaml:"ref,omitempty" json:"ref,omitempty"`
	HeadersDir string `yaml:"headers_dir,omitempty" json:"headers_dir,omitempty"`
}

// ReportPolicy records the report gates expected for this manifest.
type ReportPolicy struct {
	RequireCleanGenerated    bool `yaml:"require_clean_generated,omitempty" json:"require_clean_generated,omitempty"`
	RequireAPILock           bool `yaml:"require_api_lock,omitempty" json:"require_api_lock,omitempty"`
	RequireDocCoverage       bool `yaml:"require_doc_coverage,omitempty" json:"require_doc_coverage,omitempty"`
	RequireTypeCoverage      bool `yaml:"require_type_coverage,omitempty" json:"require_type_coverage,omitempty"`
	RequireDiagnosticReasons bool `yaml:"require_diagnostic_reasons,omitempty" json:"require_diagnostic_reasons,omitempty"`
	RequireExplicitVariants  bool `yaml:"require_explicit_variants,omitempty" json:"require_explicit_variants,omitempty"`
	RequireDecisionDeclIDs   bool `yaml:"require_decision_decl_ids,omitempty" json:"require_decision_decl_ids,omitempty"`
	RequireDecisionCoverage  bool `yaml:"require_decision_coverage,omitempty" json:"require_decision_coverage,omitempty"`
	RequireEmitAPILock       bool `yaml:"require_emit_api_lock,omitempty" json:"require_emit_api_lock,omitempty"`
	RequireUniqueCNames      bool `yaml:"require_unique_c_names,omitempty" json:"require_unique_c_names,omitempty"`
	RequireUpstreamParity    bool `yaml:"require_upstream_parity,omitempty" json:"require_upstream_parity,omitempty"`
	RequireLockMonotonicity  bool `yaml:"require_lock_monotonicity,omitempty" json:"require_lock_monotonicity,omitempty"`
	IncludeInventory         bool `yaml:"include_inventory,omitempty" json:"include_inventory,omitempty"`
}

// GeneratedMarkerPolicy records generated marker invariants for this manifest.
type GeneratedMarkerPolicy struct {
	ForbidVolatileData bool `yaml:"forbid_volatile_data,omitempty" json:"forbid_volatile_data,omitempty"`
}

// CustomHook records a manifest-owned generated hook that is not selected by an
// upstream overload variant.
type CustomHook struct {
	CName  string `yaml:"c_name" json:"c_name"`
	Reason string `yaml:"reason,omitempty" json:"reason,omitempty"`
}

// HookAPI records the public C API names emitted by a special-case hook.
type HookAPI struct {
	CName string   `yaml:"c_name" json:"c_name"`
	Names []string `yaml:"names" json:"names"`
}

// Variant defines one overload selection rule.
type Variant struct {
	Signature string  `yaml:"signature"`
	Suffix    *string `yaml:"suffix,omitempty"`
	Skip      bool    `yaml:"skip,omitempty"`
	// Optional tolerates the overload being absent from the parsed headers,
	// for signatures whose presence varies across supported MLX versions.
	Optional bool   `yaml:"optional,omitempty"`
	Reason   string `yaml:"reason,omitempty" json:"reason,omitempty"`
	Doc      string `yaml:"doc,omitempty"`
}

// variantSkipReasons is the closed vocabulary for skip reasons.
//
// unbindable_signature and excluded_by_policy split the former not_c_api,
// which conflated two different claims: that a signature cannot be expressed
// in C at all, and that a perfectly bindable overload was dropped on purpose.
// Only the second is a decision anyone can revisit, so it needs its own word.
var variantSkipReasons = map[string]bool{
	"backend_not_supported":    true,
	"covered_by_other_variant": true,
	"excluded_by_policy":       true,
	"internal_namespace":       true,
	"manual_wrapper":           true,
	"name_collision":           true,
	"template_function":        true,
	"unbindable_signature":     true,
	"version_skew":             true,
	"unstable_upstream_api":    true,
	"unsupported_type":         true,
}

// Load reads a generator plan manifest.
func Load(r io.Reader) (Manifest, error) {
	var m Manifest
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err := dec.Decode(&m); err != nil {
		return Manifest{}, fmt.Errorf("parse plan manifest: %w", err)
	}
	if len(m.ModuleFiles) > 0 {
		return Manifest{}, fmt.Errorf("plan manifest module_files require LoadFile")
	}
	if err := m.validate(); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

// LoadFile reads a generator plan manifest and any referenced module files.
func LoadFile(path string) (Manifest, error) {
	f, err := os.Open(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("open plan manifest: %w", err)
	}
	defer f.Close()
	var m Manifest
	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)
	if err := dec.Decode(&m); err != nil {
		return Manifest{}, fmt.Errorf("parse plan manifest: %w", err)
	}
	if err := m.loadModuleFiles(filepath.Dir(path)); err != nil {
		return Manifest{}, err
	}
	if err := m.validate(); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

// LoadPath reads path, or the default repository manifest if path is empty.
func LoadPath(path string) (Manifest, error) {
	if path == "" {
		return Default()
	}
	return LoadFile(path)
}

// Default returns the repository generator output plan.
func Default() (Manifest, error) {
	path, err := findDefaultManifest()
	if err != nil {
		return Manifest{}, err
	}
	return LoadFile(path)
}

// HeaderMappings returns the current header-derived binding plan.
func HeaderMappings() ([]HeaderMapping, error) {
	m, err := Default()
	if err != nil {
		return nil, err
	}
	return copyHeaderMappings(m.Headers), nil
}

// StandaloneNames returns the current standalone binding plan.
func StandaloneNames() ([]string, error) {
	m, err := Default()
	if err != nil {
		return nil, err
	}
	return append([]string(nil), m.Standalone...), nil
}

// VariantMappings returns the current overload variant selection policy.
func VariantMappings() (map[string]map[string][]Variant, error) {
	m, err := Default()
	if err != nil {
		return nil, err
	}
	return copyVariantMappings(m.VariantMappings), nil
}

// AllowedDetailFunctions returns the current detail namespace allowlist.
func AllowedDetailFunctions() (map[string]bool, error) {
	m, err := Default()
	if err != nil {
		return nil, err
	}
	return m.AllowedDetailFunctionsSet(), nil
}

// GeneratedOutputs returns all files currently produced by the Go generator.
func GeneratedOutputs() ([]string, error) {
	m, err := Default()
	if err != nil {
		return nil, err
	}
	return m.GeneratedOutputs(), nil
}

// GeneratedOutputs returns all files produced by m.
func (m Manifest) GeneratedOutputs() []string {
	var out []string
	for _, hm := range m.Headers {
		out = append(out,
			"mlx/c/"+hm.Name+".h",
			"mlx/c/"+hm.Name+".cpp",
		)
	}
	for _, name := range m.Standalone {
		out = append(out,
			"mlx/c/"+name+".h",
			"mlx/c/"+name+".cpp",
			"mlx/c/private/"+name+".h",
		)
	}
	sort.Strings(out)
	return out
}

// AllowedDetailFunctionsSet returns the detail namespace allowlist as a set.
func (m Manifest) AllowedDetailFunctionsSet() map[string]bool {
	out := map[string]bool{}
	for _, name := range m.AllowedDetailFunctions {
		out[name] = true
	}
	return out
}

// CheckInventory verifies that generated inventory entries match the plan.
func CheckInventory(entries []inventory.Entry) error {
	m, err := Default()
	if err != nil {
		return err
	}
	return m.CheckInventory(entries)
}

// CheckInventory verifies that generated inventory entries match m.
func (m Manifest) CheckInventory(entries []inventory.Entry) error {
	return m.CheckInventoryWithGenerated(entries, nil)
}

// CheckInventoryWithGenerated verifies inventory entries against m and generated custom files.
func (m Manifest) CheckInventoryWithGenerated(entries []inventory.Entry, customGenerated []string) error {
	outputs := m.GeneratedOutputs()
	planned := map[string]map[string]bool{}
	addPlanned := func(path string, kinds ...string) {
		if planned[path] == nil {
			planned[path] = map[string]bool{}
		}
		for _, kind := range kinds {
			planned[path][kind] = true
		}
	}
	for _, out := range outputs {
		addPlanned(out, "generated_header_api", "generated_support")
	}
	for _, out := range customGenerated {
		addPlanned(out, "custom_spec_generated")
	}
	entriesByPath := map[string]inventory.Entry{}
	for _, entry := range entries {
		entriesByPath[entry.Path] = entry
	}

	var problems []string
	for path := range planned {
		entry, ok := entriesByPath[path]
		if !ok {
			problems = append(problems, fmt.Sprintf("%s is generated by plan but missing from inventory", path))
			continue
		}
		if !planned[path][entry.Kind] {
			problems = append(problems, fmt.Sprintf("%s inventory kind = %s, want %s", path, entry.Kind, plannedKinds(planned[path])))
		}
	}
	for _, entry := range entries {
		if entry.Kind != "generated_header_api" && entry.Kind != "generated_support" {
			continue
		}
		if planned[entry.Path] == nil || (!planned[entry.Path]["generated_header_api"] && !planned[entry.Path]["generated_support"]) {
			problems = append(problems, fmt.Sprintf("%s is %s but is not generated by the Go plan", entry.Path, entry.Kind))
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("plan check failed:\n%s", strings.Join(problems, "\n"))
	}
	return nil
}

func plannedKinds(kinds map[string]bool) string {
	var out []string
	for kind := range kinds {
		out = append(out, kind)
	}
	sort.Strings(out)
	return strings.Join(out, " or ")
}

// CheckCMakeMLXRef verifies that CMake fetches the MLX ref recorded in m.
func (m Manifest) CheckCMakeMLXRef(root string) error {
	got, err := cmakeMLXGitTag(root)
	if err != nil {
		return err
	}
	if got != m.MLX.ExpectedGitRef {
		return fmt.Errorf("CMake MLX GIT_TAG = %s, want manifest mlx expected_git_ref %s", got, m.MLX.ExpectedGitRef)
	}
	return nil
}

func cmakeMLXGitTag(root string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, "CMakeLists.txt"))
	if err != nil {
		return "", fmt.Errorf("read CMakeLists.txt: %w", err)
	}
	match := cmakeGitTagRE.FindSubmatch(data)
	if match == nil {
		return "", fmt.Errorf("CMakeLists.txt has no MLX GIT_TAG")
	}
	return string(match[1]), nil
}

func (m Manifest) validate() error {
	if m.SchemaVersion != SchemaVersion {
		return fmt.Errorf("plan manifest schema_version = %d, want %d", m.SchemaVersion, SchemaVersion)
	}
	if m.MLX.ExpectedGitRef == "" {
		return fmt.Errorf("plan manifest has empty mlx expected_git_ref")
	}
	if len(m.Headers) == 0 {
		return fmt.Errorf("plan manifest has no header mappings")
	}
	if len(m.Standalone) == 0 {
		return fmt.Errorf("plan manifest has no standalone generators")
	}
	headerNames := map[string]bool{}
	for _, hm := range m.Headers {
		if hm.Name == "" {
			return fmt.Errorf("plan manifest has header mapping with empty name")
		}
		if !planNameRE.MatchString(hm.Name) {
			return fmt.Errorf("plan manifest header mapping %q has invalid name", hm.Name)
		}
		if headerNames[hm.Name] {
			return fmt.Errorf("plan manifest has duplicate header mapping %q", hm.Name)
		}
		headerNames[hm.Name] = true
		if len(hm.Headers) == 0 {
			return fmt.Errorf("plan manifest header mapping %q has no headers", hm.Name)
		}
		for _, header := range hm.Headers {
			if header == "" {
				return fmt.Errorf("plan manifest header mapping %q has empty header", hm.Name)
			}
			if err := validateHeaderPath(hm.Name, "header", header); err != nil {
				return err
			}
		}
		for _, header := range hm.PreIncludes {
			if header == "" {
				return fmt.Errorf("plan manifest header mapping %q has empty pre-include", hm.Name)
			}
			if err := validateHeaderPath(hm.Name, "pre-include", header); err != nil {
				return err
			}
		}
	}
	standaloneNames := map[string]bool{}
	for _, name := range m.Standalone {
		if name == "" {
			return fmt.Errorf("plan manifest has empty standalone generator")
		}
		if !planNameRE.MatchString(name) {
			return fmt.Errorf("plan manifest standalone generator %q has invalid name", name)
		}
		if standaloneNames[name] {
			return fmt.Errorf("plan manifest has duplicate standalone generator %q", name)
		}
		standaloneNames[name] = true
	}
	customHooks := map[string]bool{}
	for _, hook := range m.CustomHooks {
		if hook.CName == "" {
			return fmt.Errorf("plan manifest has custom hook with empty c_name")
		}
		if !cIdentRE.MatchString(hook.CName) {
			return fmt.Errorf("plan manifest custom hook %q has invalid c_name", hook.CName)
		}
		if hook.Reason == "" {
			return fmt.Errorf("plan manifest custom hook %q has empty reason", hook.CName)
		}
		if customHooks[hook.CName] {
			return fmt.Errorf("plan manifest has duplicate custom hook %q", hook.CName)
		}
		customHooks[hook.CName] = true
	}
	hookAPIs := map[string]bool{}
	for _, api := range m.HookAPI {
		if api.CName == "" {
			return fmt.Errorf("plan manifest has hook api with empty c_name")
		}
		if !cIdentRE.MatchString(api.CName) {
			return fmt.Errorf("plan manifest hook api %q has invalid c_name", api.CName)
		}
		if hookAPIs[api.CName] {
			return fmt.Errorf("plan manifest has duplicate hook api %q", api.CName)
		}
		if len(api.Names) == 0 {
			return fmt.Errorf("plan manifest hook api %q has no names", api.CName)
		}
		names := map[string]bool{}
		for _, name := range api.Names {
			if name == "" {
				return fmt.Errorf("plan manifest hook api %q has empty name", api.CName)
			}
			if !cIdentRE.MatchString(name) {
				return fmt.Errorf("plan manifest hook api %q has invalid name %q", api.CName, name)
			}
			if names[name] {
				return fmt.Errorf("plan manifest hook api %q has duplicate name %q", api.CName, name)
			}
			names[name] = true
		}
		hookAPIs[api.CName] = true
	}
	for namespace, funcs := range m.VariantMappings {
		if namespace == "" {
			return fmt.Errorf("plan manifest has variant mapping with empty namespace")
		}
		if len(funcs) == 0 {
			return fmt.Errorf("plan manifest variant namespace %q has no functions", namespace)
		}
		for name, variants := range funcs {
			if name == "" {
				return fmt.Errorf("plan manifest variant namespace %q has empty function name", namespace)
			}
			if len(variants) == 0 {
				return fmt.Errorf("plan manifest variant mapping %q.%s has no entries", namespace, name)
			}
			seenSignatures := map[string]bool{}
			for _, variant := range variants {
				if variant.Signature == "" {
					return fmt.Errorf("plan manifest variant mapping %q.%s has empty signature", namespace, name)
				}
				if seenSignatures[variant.Signature] {
					return fmt.Errorf("plan manifest variant mapping %q.%s has duplicate signature %q", namespace, name, variant.Signature)
				}
				seenSignatures[variant.Signature] = true
				if variant.Skip == (variant.Suffix != nil) {
					return fmt.Errorf("plan manifest variant mapping %q.%s signature %q must set exactly one of suffix or skip", namespace, name, variant.Signature)
				}
				if variant.Skip && variant.Reason == "" {
					return fmt.Errorf("plan manifest variant mapping %q.%s signature %q has skip without reason", namespace, name, variant.Signature)
				}
				if variant.Reason != "" && !variant.Skip {
					return fmt.Errorf("plan manifest variant mapping %q.%s signature %q has reason without skip", namespace, name, variant.Signature)
				}
				if variant.Reason != "" && !variantSkipReasons[variant.Reason] {
					return fmt.Errorf("plan manifest variant mapping %q.%s signature %q has unknown skip reason %q", namespace, name, variant.Signature, variant.Reason)
				}
			}
		}
	}
	allowedDetail := map[string]bool{}
	for _, name := range m.AllowedDetailFunctions {
		if name == "" {
			return fmt.Errorf("plan manifest has empty allowed detail function")
		}
		if !cIdentRE.MatchString(name) {
			return fmt.Errorf("plan manifest allowed detail function %q has invalid name", name)
		}
		if allowedDetail[name] {
			return fmt.Errorf("plan manifest has duplicate allowed detail function %q", name)
		}
		allowedDetail[name] = true
	}
	return nil
}

func (m *Manifest) loadModuleFiles(dir string) error {
	if len(m.ModuleFiles) == 0 {
		return nil
	}
	if len(m.Headers) != 0 {
		return fmt.Errorf("plan manifest must not set both headers and module_files")
	}
	for _, name := range m.ModuleFiles {
		if name == "" {
			return fmt.Errorf("plan manifest has empty module file")
		}
		if err := validateModuleFilePath(name); err != nil {
			return err
		}
		path := name
		path = filepath.Join(dir, filepath.FromSlash(name))
		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("open module file %s: %w", name, err)
		}
		var hm HeaderMapping
		dec := yaml.NewDecoder(f)
		dec.KnownFields(true)
		if err := dec.Decode(&hm); err != nil {
			f.Close()
			return fmt.Errorf("parse module file %s: %w", name, err)
		}
		if err := f.Close(); err != nil {
			return fmt.Errorf("close module file %s: %w", name, err)
		}
		m.Headers = append(m.Headers, hm)
	}
	if err := m.checkModuleFileInventory(dir); err != nil {
		return err
	}
	return nil
}

func (m *Manifest) checkModuleFileInventory(dir string) error {
	listed := map[string]bool{}
	dirs := map[string]bool{}
	for _, name := range m.ModuleFiles {
		listed[name] = true
		dirs[path.Dir(name)] = true
	}
	var problems []string
	for moduleDir := range dirs {
		entries, err := os.ReadDir(filepath.Join(dir, filepath.FromSlash(moduleDir)))
		if err != nil {
			return fmt.Errorf("read module file directory %s: %w", moduleDir, err)
		}
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
				continue
			}
			name := path.Join(moduleDir, entry.Name())
			if !listed[name] {
				problems = append(problems, fmt.Sprintf("module file %s exists but is not listed in manifest", name))
			}
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("plan manifest module file inventory failed:\n%s", strings.Join(problems, "\n"))
	}
	return nil
}

func validateModuleFilePath(name string) error {
	if filepath.IsAbs(name) || strings.HasPrefix(name, "/") {
		return fmt.Errorf("plan manifest module file %q must be relative", name)
	}
	if strings.Contains(name, `\`) {
		return fmt.Errorf("plan manifest module file %q contains backslash", name)
	}
	clean := pathClean(name)
	if clean != name || clean == "." || strings.HasPrefix(clean, "../") {
		return fmt.Errorf("plan manifest module file %q must be clean and stay under the manifest directory", name)
	}
	if filepath.Ext(name) != ".yaml" {
		return fmt.Errorf("plan manifest module file %q must have .yaml extension", name)
	}
	return nil
}

func validateHeaderPath(module, kind, header string) error {
	if filepath.IsAbs(header) || strings.HasPrefix(header, "/") {
		return fmt.Errorf("plan manifest header mapping %q %s %q must be relative", module, kind, header)
	}
	if strings.Contains(header, `\`) {
		return fmt.Errorf("plan manifest header mapping %q %s %q contains backslash", module, kind, header)
	}
	clean := pathClean(header)
	if clean != header || clean == "." || strings.HasPrefix(clean, "../") {
		return fmt.Errorf("plan manifest header mapping %q %s %q must be clean and stay under the include root", module, kind, header)
	}
	if filepath.Ext(header) != ".h" {
		return fmt.Errorf("plan manifest header mapping %q %s %q must have .h extension", module, kind, header)
	}
	return nil
}

func pathClean(name string) string {
	return filepath.ToSlash(filepath.Clean(filepath.FromSlash(name)))
}

func copyHeaderMappings(in []HeaderMapping) []HeaderMapping {
	out := make([]HeaderMapping, len(in))
	for i, hm := range in {
		out[i] = HeaderMapping{
			Name:        hm.Name,
			Headers:     append([]string(nil), hm.Headers...),
			Docstring:   hm.Docstring,
			PreIncludes: append([]string(nil), hm.PreIncludes...),
		}
	}
	return out
}

func copyVariantMappings(in map[string]map[string][]Variant) map[string]map[string][]Variant {
	out := make(map[string]map[string][]Variant, len(in))
	for namespace, funcs := range in {
		out[namespace] = make(map[string][]Variant, len(funcs))
		for name, variants := range funcs {
			out[namespace][name] = copyVariants(variants)
		}
	}
	return out
}

func copyVariants(in []Variant) []Variant {
	out := make([]Variant, len(in))
	for i, variant := range in {
		out[i] = Variant{
			Signature: variant.Signature,
			Skip:      variant.Skip,
			Optional:  variant.Optional,
			Reason:    variant.Reason,
			Doc:       variant.Doc,
		}
		if variant.Suffix != nil {
			s := *variant.Suffix
			out[i].Suffix = &s
		}
	}
	return out
}

func findDefaultManifest() (string, error) {
	if path, ok := findUp(".", defaultManifestPath); ok {
		return path, nil
	}
	_, file, _, ok := runtime.Caller(0)
	if ok {
		if path, ok := findUp(filepath.Dir(file), defaultManifestPath); ok {
			return path, nil
		}
	}
	return "", fmt.Errorf("find default plan manifest %s", defaultManifestPath)
}

func findUp(start, rel string) (string, bool) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", false
	}
	for {
		path := filepath.Join(dir, rel)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, true
		}
		next := filepath.Dir(dir)
		if next == dir {
			return "", false
		}
		dir = next
	}
}
