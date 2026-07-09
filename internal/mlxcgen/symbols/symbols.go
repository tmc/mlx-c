package symbols

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/tmc/mlx-c/internal/mlxcgen/apilock"
)

// TargetLibrary maps one lock target to a built library path.
type TargetLibrary struct {
	Target string
	Path   string
}

// TargetSymbols maps one lock target to an actual symbol-list file.
type TargetSymbols struct {
	Target string
	Path   string
}

// Options controls symbol checking.
type Options struct {
	LockPath string
	NM       string
	Targets  []TargetLibrary
	Actuals  []TargetSymbols
}

// Result records the symbol check result for one target.
type Result struct {
	Target          string   `json:"target"`
	Path            string   `json:"path"`
	Source          string   `json:"source"`
	LockedFunctions int      `json:"locked_functions"`
	DefinedSymbols  int      `json:"defined_symbols"`
	PublicSymbols   int      `json:"public_symbols"`
	Problems        []string `json:"problems,omitempty"`
}

// Check verifies built library symbols against the API lock.
func Check(opts Options) error {
	_, err := Report(opts)
	return err
}

// Report verifies built library symbols and returns structured results.
func Report(opts Options) ([]Result, error) {
	if opts.LockPath == "" {
		return nil, fmt.Errorf("missing lock path")
	}
	if opts.NM == "" {
		opts.NM = "nm"
	}
	data, err := os.ReadFile(opts.LockPath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", opts.LockPath, err)
	}
	var lock apilock.Lock
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, fmt.Errorf("parse %s: %w", opts.LockPath, err)
	}
	if len(opts.Targets) == 0 && len(opts.Actuals) == 0 {
		return nil, fmt.Errorf("no target libraries or actual symbol files provided")
	}

	var problems []string
	var results []Result
	for _, tl := range opts.Targets {
		target, ok := lock.Targets[tl.Target]
		if !ok {
			problems = append(problems, fmt.Sprintf("unknown target %q", tl.Target))
			results = append(results, Result{
				Target:   tl.Target,
				Path:     tl.Path,
				Source:   "library",
				Problems: []string{fmt.Sprintf("unknown target %q", tl.Target)},
			})
			continue
		}
		syms, err := definedSymbols(opts.NM, tl.Path)
		if err != nil {
			problems = append(problems, err.Error())
			results = append(results, Result{
				Target:   tl.Target,
				Path:     tl.Path,
				Source:   "library",
				Problems: []string{err.Error()},
			})
			continue
		}
		result := checkResult(tl.Target, tl.Path, "library", target, syms)
		problems = append(problems, result.Problems...)
		results = append(results, result)
	}
	for _, actual := range opts.Actuals {
		target, ok := lock.Targets[actual.Target]
		if !ok {
			problems = append(problems, fmt.Sprintf("unknown target %q", actual.Target))
			results = append(results, Result{
				Target:   actual.Target,
				Path:     actual.Path,
				Source:   "symbol_list",
				Problems: []string{fmt.Sprintf("unknown target %q", actual.Target)},
			})
			continue
		}
		syms, err := readSymbolList(actual.Path)
		if err != nil {
			problems = append(problems, err.Error())
			results = append(results, Result{
				Target:   actual.Target,
				Path:     actual.Path,
				Source:   "symbol_list",
				Problems: []string{err.Error()},
			})
			continue
		}
		result := checkResult(actual.Target, actual.Path, "symbol_list", target, syms)
		problems = append(problems, result.Problems...)
		results = append(results, result)
	}
	sortResults(results)
	if len(problems) > 0 {
		sort.Strings(problems)
		return results, fmt.Errorf("symbol check failed:\n%s", strings.Join(problems, "\n"))
	}
	return results, nil
}

func checkResult(name, path, source string, target apilock.Target, syms map[string]bool) Result {
	problems := checkTarget(name, target, syms)
	sort.Strings(problems)
	return Result{
		Target:          name,
		Path:            path,
		Source:          source,
		LockedFunctions: len(target.Functions),
		DefinedSymbols:  len(syms),
		PublicSymbols:   countPublicSymbols(syms),
		Problems:        problems,
	}
}

func countPublicSymbols(syms map[string]bool) int {
	n := 0
	for sym := range syms {
		if isPublicCAPISymbol(canonicalName(sym)) {
			n++
		}
	}
	return n
}

func sortResults(results []Result) {
	sort.Slice(results, func(i, j int) bool {
		if results[i].Target != results[j].Target {
			return results[i].Target < results[j].Target
		}
		if results[i].Source != results[j].Source {
			return results[i].Source < results[j].Source
		}
		return results[i].Path < results[j].Path
	})
}

func checkTarget(name string, target apilock.Target, syms map[string]bool) []string {
	var problems []string
	for _, fn := range target.Functions {
		if !hasSymbol(syms, fn.Name) {
			problems = append(problems, fmt.Sprintf("%s: missing %s", name, fn.Name))
		}
	}
	for sym := range syms {
		canon := canonicalName(sym)
		if isPublicCAPISymbol(canon) && !lockedSymbol(target, sym) {
			problems = append(problems, fmt.Sprintf("%s: unexpected public C API symbol %s", name, sym))
		}
		switch name {
		case "mlxc":
			if strings.HasPrefix(canon, "mlx_jaccl_") {
				problems = append(problems, fmt.Sprintf("mlxc: forbidden JACCL symbol %s", sym))
			}
		case "jacclc":
			if isPublicCAPISymbol(canon) && !strings.HasPrefix(canon, "mlx_jaccl_") {
				problems = append(problems, fmt.Sprintf("jacclc: forbidden non-JACCL C API symbol %s", sym))
			}
		}
	}
	return problems
}

func lockedSymbol(target apilock.Target, sym string) bool {
	for _, fn := range target.Functions {
		if hasSymbol(map[string]bool{sym: true}, fn.Name) {
			return true
		}
	}
	return false
}

func definedSymbols(nm, path string) (map[string]bool, error) {
	cmd := exec.Command(nm, "-g", path)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("run nm on %s: %s", path, msg)
	}
	return ParseNM(out), nil
}

func readSymbolList(path string) (map[string]bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read symbols %s: %w", path, err)
	}
	return ParseSymbolList(data), nil
}

// ParseSymbolList returns symbol names from a whitespace-separated symbol list.
func ParseSymbolList(out []byte) map[string]bool {
	syms := map[string]bool{}
	for _, name := range strings.Fields(string(out)) {
		syms[name] = true
	}
	return syms
}

// ParseNM returns the defined symbol names from nm output.
func ParseNM(out []byte) map[string]bool {
	syms := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if len(fields) > 1 && fields[len(fields)-2] == "U" {
			continue
		}
		name := fields[len(fields)-1]
		if name == "" || strings.HasSuffix(name, ":") {
			continue
		}
		syms[name] = true
	}
	return syms
}

func hasSymbol(syms map[string]bool, want string) bool {
	for _, name := range normalizedNames(want) {
		if syms[name] {
			return true
		}
	}
	for sym := range syms {
		if canonicalName(sym) == want {
			return true
		}
	}
	return false
}

func normalizedNames(name string) []string {
	if strings.HasPrefix(name, "_") {
		return []string{name, strings.TrimPrefix(name, "_")}
	}
	return []string{name, "_" + name}
}

func canonicalName(name string) string {
	if strings.HasPrefix(name, "__mlx_") {
		return strings.TrimPrefix(name, "_")
	}
	if strings.HasPrefix(name, "_mlx_") {
		return strings.TrimPrefix(name, "_")
	}
	return name
}

func isPublicCAPISymbol(name string) bool {
	return strings.HasPrefix(name, "mlx_") || strings.HasPrefix(name, "_mlx_")
}
