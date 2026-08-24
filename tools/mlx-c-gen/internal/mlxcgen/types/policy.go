package types

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/tmc/mlx-c/tools/mlx-c-gen/internal/mlxcgen/ir"
	"gopkg.in/yaml.v3"
)

const (
	SchemaVersion         = 1
	defaultTypePolicyPath = "codegen/types.yaml"
)

// MappingInfo describes a registered type mapping without code templates.
type MappingInfo struct {
	CType   string
	CppType string
	Alt     []string
}

// Policy records the reviewable type mapping inventory.
type Policy struct {
	SchemaVersion int        `yaml:"schema_version" json:"schema_version"`
	Types         []TypeSpec `yaml:"types" json:"types"`
}

// TypeSpec records one reviewable type mapping.
type TypeSpec struct {
	CPP         string   `yaml:"cpp" json:"cpp"`
	C           string   `yaml:"c,omitempty" json:"c,omitempty"`
	Alternates  []string `yaml:"alternates,omitempty" json:"alternates,omitempty"`
	Class       string   `yaml:"class" json:"class"`
	Ownership   string   `yaml:"ownership" json:"ownership"`
	Nullability string   `yaml:"nullability" json:"nullability"`
	Conversion  string   `yaml:"conversion" json:"conversion"`
}

// MissingType records one IR type use that is not covered by the policy.
type MissingType struct {
	Type       string       `json:"type"`
	Role       string       `json:"role"`
	ParamIndex int          `json:"param_index,omitempty"`
	ParamName  string       `json:"param_name,omitempty"`
	Function   ir.DeclID    `json:"function"`
	Module     string       `json:"module"`
	Header     string       `json:"header,omitempty"`
	Namespace  string       `json:"namespace"`
	Name       string       `json:"name"`
	Loc        ir.SourceLoc `json:"loc,omitempty"`
}

// Mappings returns the registered mappings without conversion callbacks.
func (r *Registry) Mappings() []MappingInfo {
	out := make([]MappingInfo, 0, len(r.all))
	for _, tm := range r.all {
		out = append(out, MappingInfo{
			CType:   tm.CType,
			CppType: tm.CppType,
			Alt:     append([]string(nil), tm.Alt...),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CppType < out[j].CppType
	})
	return out
}

// LoadPolicy reads a type policy manifest.
func LoadPolicy(r io.Reader) (Policy, error) {
	var p Policy
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err := dec.Decode(&p); err != nil {
		return Policy{}, fmt.Errorf("parse type policy: %w", err)
	}
	if err := p.validate(); err != nil {
		return Policy{}, err
	}
	return p, nil
}

// LoadPolicyFile reads a type policy manifest from path.
func LoadPolicyFile(path string) (Policy, error) {
	f, err := os.Open(path)
	if err != nil {
		return Policy{}, fmt.Errorf("open type policy: %w", err)
	}
	defer f.Close()
	return LoadPolicy(f)
}

// LoadPolicyPath reads path, or the default repository type policy if path is
// empty.
func LoadPolicyPath(path string) (Policy, error) {
	if path == "" {
		var err error
		path, err = findDefaultTypePolicy()
		if err != nil {
			return Policy{}, err
		}
	}
	return LoadPolicyFile(path)
}

// CheckRegistry verifies that p matches r's registered type mappings.
func (p Policy) CheckRegistry(r *Registry) error {
	if err := p.validate(); err != nil {
		return err
	}
	specs := map[string]TypeSpec{}
	for _, spec := range p.Types {
		specs[spec.CPP] = spec
	}
	var problems []string
	for _, mapping := range r.Mappings() {
		spec, ok := specs[mapping.CppType]
		if !ok {
			problems = append(problems, fmt.Sprintf("type policy missing %q", mapping.CppType))
			continue
		}
		if spec.C != mapping.CType {
			problems = append(problems, fmt.Sprintf("type policy %q c = %q, want %q", mapping.CppType, spec.C, mapping.CType))
		}
		if !sameStrings(spec.Alternates, mapping.Alt) {
			problems = append(problems, fmt.Sprintf("type policy %q alternates = %#v, want %#v", mapping.CppType, spec.Alternates, mapping.Alt))
		}
		delete(specs, mapping.CppType)
	}
	for cpp := range specs {
		problems = append(problems, fmt.Sprintf("type policy %q is not registered", cpp))
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("type policy check failed:\n%s", strings.Join(problems, "\n"))
	}
	return nil
}

// MissingIRTypes returns IR type uses not covered by p.
func (p Policy) MissingIRTypes(result ir.Result) []MissingType {
	covered := p.coveredTypes()
	var missing []MissingType
	for _, fn := range result.Functions {
		if fn.Return != "" && !covered[fn.Return] {
			missing = append(missing, missingType(fn, "return", 0, "", fn.Return))
		}
		for i, param := range fn.Params {
			if param.Type == "" || covered[param.Type] {
				continue
			}
			missing = append(missing, missingType(fn, "param", i, param.Name, param.Type))
		}
	}
	sort.Slice(missing, func(i, j int) bool {
		a, b := missing[i], missing[j]
		if a.Function != b.Function {
			return a.Function < b.Function
		}
		if a.Role != b.Role {
			return a.Role < b.Role
		}
		if a.ParamIndex != b.ParamIndex {
			return a.ParamIndex < b.ParamIndex
		}
		return a.Type < b.Type
	})
	return missing
}

func (p Policy) coveredTypes() map[string]bool {
	covered := map[string]bool{}
	for _, spec := range p.Types {
		covered[spec.CPP] = true
		for _, alt := range spec.Alternates {
			covered[alt] = true
		}
	}
	return covered
}

func missingType(fn ir.FuncDecl, role string, paramIndex int, paramName, typ string) MissingType {
	return MissingType{
		Type:       typ,
		Role:       role,
		ParamIndex: paramIndex,
		ParamName:  paramName,
		Function:   fn.ID,
		Module:     fn.Module,
		Header:     fn.Header,
		Namespace:  fn.Namespace,
		Name:       fn.Name,
		Loc:        fn.Loc,
	}
}

func (p Policy) validate() error {
	if p.SchemaVersion != SchemaVersion {
		return fmt.Errorf("type policy schema_version = %d, want %d", p.SchemaVersion, SchemaVersion)
	}
	if len(p.Types) == 0 {
		return fmt.Errorf("type policy has no types")
	}
	seen := map[string]bool{}
	for _, spec := range p.Types {
		if spec.CPP == "" {
			return fmt.Errorf("type policy has empty cpp type")
		}
		if seen[spec.CPP] {
			return fmt.Errorf("type policy has duplicate cpp type %q", spec.CPP)
		}
		seen[spec.CPP] = true
		if spec.Class == "" {
			return fmt.Errorf("type policy %q has empty class", spec.CPP)
		}
		if spec.Ownership == "" {
			return fmt.Errorf("type policy %q has empty ownership", spec.CPP)
		}
		if spec.Nullability == "" {
			return fmt.Errorf("type policy %q has empty nullability", spec.CPP)
		}
		if spec.Conversion == "" {
			return fmt.Errorf("type policy %q has empty conversion", spec.CPP)
		}
	}
	return nil
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func findDefaultTypePolicy() (string, error) {
	if path, ok := findUp(".", defaultTypePolicyPath); ok {
		return path, nil
	}
	_, file, _, ok := runtime.Caller(0)
	if ok {
		if path, ok := findUp(filepath.Dir(file), defaultTypePolicyPath); ok {
			return path, nil
		}
	}
	return "", fmt.Errorf("find default type policy %s", defaultTypePolicyPath)
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
