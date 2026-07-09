// Command mlx-c-inventory-check verifies the MLX C generated-file inventory.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/tmc/mlx-c/internal/mlxcgen/inventory"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "mlx-c-inventory-check: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	root := flag.String("root", ".", "repository root")
	inventoryPath := flag.String("inventory", "codegen/generated-files.txt", "generated-file inventory path")
	flag.Parse()
	return inventory.Check(*root, *inventoryPath)
}
