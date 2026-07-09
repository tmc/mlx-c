// Command mlx-c-symbol-check compares a C API lock with built library symbols.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/tmc/mlx-c/internal/mlxcgen/symbols"
)

type targetFlags []symbols.TargetLibrary
type actualFlags []symbols.TargetSymbols

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

func (f *actualFlags) String() string {
	var parts []string
	for _, actual := range *f {
		parts = append(parts, actual.Target+"="+actual.Path)
	}
	return strings.Join(parts, ",")
}

func (f *actualFlags) Set(s string) error {
	target, path, ok := strings.Cut(s, "=")
	if !ok || target == "" || path == "" {
		return fmt.Errorf("expected target=symbol-file")
	}
	*f = append(*f, symbols.TargetSymbols{Target: target, Path: path})
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "mlx-c-symbol-check: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var targets targetFlags
	var actuals actualFlags
	lockPath := flag.String("lock", "codegen/mlxc-capi.lock.json", "API lock JSON path")
	nm := flag.String("nm", "nm", "nm command for --target library checks")
	flag.Var(&targets, "target", "target=library path to check; may be repeated")
	flag.Var(&actuals, "actual", "target=symbol-file path to check; may be repeated")
	flag.Parse()
	return symbols.Check(symbols.Options{
		LockPath: *lockPath,
		NM:       *nm,
		Targets:  targets,
		Actuals:  actuals,
	})
}
