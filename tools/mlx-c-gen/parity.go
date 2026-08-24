package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/tmc/mlx-c/tools/mlx-c-gen/internal/mlxcgen/apilock"
	"github.com/tmc/mlx-c/tools/mlx-c-gen/internal/mlxcgen/compat"
)

// runParity runs the D5 shared-symbol parity gate standalone: ours-lock vs
// upstream-headers, without the rest of check. Folded from the former
// tools/paritycheck scratch tool.
func runParity(args []string) error {
	fs := flag.NewFlagSet("mlx-c-gen parity", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	lockPath := fs.String("lock", "codegen/mlxc-capi.lock.json", "ours API lock JSON")
	upstreamDir := fs.String("upstream-headers", "", "upstream mlx-c header directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *upstreamDir == "" {
		return fmt.Errorf("usage: mlx-c-gen parity [--lock FILE] --upstream-headers DIR")
	}
	data, err := os.ReadFile(*lockPath)
	if err != nil {
		return fmt.Errorf("read lock: %w", err)
	}
	ours, err := apilock.Decode(data)
	if err != nil {
		return fmt.Errorf("decode lock: %w", err)
	}
	upstream, err := apilock.GenerateTarget(*upstreamDir, "mlx.h")
	if err != nil {
		return fmt.Errorf("parse upstream: %w", err)
	}
	res := compat.CheckParity(ours.Targets["mlxc"], upstream, compat.ParityWaivers{})
	fmt.Printf("shared=%d ours-only=%d divergent=%d\n", res.Shared, res.OursOnly, len(res.Divergent))
	for _, name := range res.Divergent {
		fmt.Println("DIVERGENT:", name)
	}
	if err := res.Err(); err != nil {
		return err
	}
	fmt.Println("PASS")
	return nil
}
