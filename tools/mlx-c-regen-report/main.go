// Command mlx-c-regen-report reports scratch-tree regeneration drift.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/tmc/mlx-c/internal/mlxcgen/regenreport"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "mlx-c-regen-report: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	repoRoot := flag.String("root", ".", "repository root")
	mlxSrc := flag.String("mlx-src", "", "MLX source directory")
	manifestPath := flag.String("manifest", "", "generator manifest path")
	customDir := flag.String("custom-dir", "", "custom generator spec directory")
	typePolicyPath := flag.String("types", "", "type policy path")
	compileCommandsPath := flag.String("compile-commands", "", "compile_commands.json path for parser flags")
	inventoryPath := flag.String("inventory", "codegen/generated-files.txt", "generated-file inventory path")
	workDir := flag.String("work-dir", "", "scratch work directory")
	astCacheDir := flag.String("ast-cache", "", "cache parsed clang AST results under directory")
	noASTCache := flag.Bool("no-ast-cache", false, "disable parsed clang AST cache")
	formatCacheDir := flag.String("format-cache", "", "cache clang-format output under directory")
	noFormatCache := flag.Bool("no-format-cache", false, "disable clang-format output cache")
	generator := flag.String("generator", "go run ./tools/mlx-c-gen", "generator command")
	noFormat := flag.Bool("no-format", false, "pass --no-format to mlx-c-gen")
	keepWork := flag.Bool("keep-work", false, "keep an auto-created scratch directory")
	check := flag.Bool("check", false, "exit non-zero when drift is detected")
	flag.Parse()

	report, err := regenreport.Run(regenreport.Options{
		RepoRoot:            *repoRoot,
		MLXSrc:              *mlxSrc,
		ManifestPath:        *manifestPath,
		CustomDir:           *customDir,
		TypePolicyPath:      *typePolicyPath,
		CompileCommandsPath: *compileCommandsPath,
		InventoryPath:       *inventoryPath,
		WorkDir:             *workDir,
		ASTCacheDir:         *astCacheDir,
		NoASTCache:          *noASTCache,
		FormatCacheDir:      *formatCacheDir,
		NoFormatCache:       *noFormatCache,
		Generator:           strings.Fields(*generator),
		NoFormat:            *noFormat,
		KeepWork:            *keepWork,
	})
	if err != nil {
		return err
	}
	data, err := report.JSON()
	if err != nil {
		return err
	}
	if _, err := os.Stdout.Write(data); err != nil {
		return fmt.Errorf("write stdout: %w", err)
	}
	if *check && !report.Clean() {
		return fmt.Errorf("regenerated files differ")
	}
	return nil
}
