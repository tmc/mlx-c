package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tmc/mlx-c/tools/mlx-c-gen/internal/mlxcgen/apilock"
)

// runLock regenerates (or verifies) the checked-in MLX C API lock from the
// generated headers. Folded from the former tools/mlx-c-lock and
// tools/lockgen scratch tools.
func runLock(args []string) error {
	fs := flag.NewFlagSet("mlx-c-gen lock", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	headers := fs.String("headers", "mlx/c", "directory containing MLX C headers")
	lockPath := fs.String("lock", "-", "path for JSON API lock, or - for stdout")
	tuPath := fs.String("tu", "", "path for generated C translation-unit lock")
	check := fs.Bool("check", false, "check generated outputs against existing files")
	if err := fs.Parse(args); err != nil {
		return err
	}

	lock, err := apilock.Generate(*headers)
	if err != nil {
		return err
	}
	jsonData, err := lock.JSON()
	if err != nil {
		return err
	}
	tuData, err := lock.LockC()
	if err != nil {
		return err
	}

	if *check {
		if *lockPath == "-" {
			return fmt.Errorf("-check requires -lock path")
		}
		if err := checkFile(*lockPath, jsonData); err != nil {
			return err
		}
		if *tuPath != "" {
			if err := checkFile(*tuPath, tuData); err != nil {
				return err
			}
		}
		return nil
	}

	if *lockPath == "-" {
		if _, err := os.Stdout.Write(jsonData); err != nil {
			return fmt.Errorf("write stdout: %w", err)
		}
	} else if err := writeLockFile(*lockPath, jsonData); err != nil {
		return err
	}
	if *tuPath != "" {
		if err := writeLockFile(*tuPath, tuData); err != nil {
			return err
		}
	}
	return nil
}

func writeLockFile(name string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(name), 0o777); err != nil {
		return fmt.Errorf("make %s: %w", filepath.Dir(name), err)
	}
	if err := os.WriteFile(name, data, 0o666); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	return nil
}
