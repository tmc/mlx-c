// Command mlx-c-plan-check verifies generator outputs against the inventory.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/tmc/mlx-c/internal/mlxcgen/inventory"
	"github.com/tmc/mlx-c/internal/mlxcgen/plan"
	"github.com/tmc/mlx-c/internal/mlxcgen/types"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "mlx-c-plan-check: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	inventoryPath := flag.String("inventory", "codegen/generated-files.txt", "generated-file inventory path")
	typePolicyPath := flag.String("types", "codegen/types.yaml", "type policy path")
	flag.Parse()

	f, err := os.Open(*inventoryPath)
	if err != nil {
		return fmt.Errorf("open %s: %w", *inventoryPath, err)
	}
	defer f.Close()
	entries, err := inventory.Read(f)
	if err != nil {
		return err
	}
	if err := plan.CheckInventory(entries); err != nil {
		return err
	}
	policy, err := types.LoadPolicyPath(*typePolicyPath)
	if err != nil {
		return err
	}
	return policy.CheckRegistry(types.NewRegistry())
}
