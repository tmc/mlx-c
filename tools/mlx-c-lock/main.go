// Command mlx-c-lock writes the checked-in MLX C API lock.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tmc/mlx-c/internal/mlxcgen/apilock"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "mlx-c-lock: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	headers := flag.String("headers", "mlx/c", "directory containing MLX C headers")
	lockPath := flag.String("lock", "-", "path for JSON API lock, or - for stdout")
	tuPath := flag.String("tu", "", "path for generated C translation-unit lock")
	check := flag.Bool("check", false, "check generated outputs against existing files")
	flag.Parse()

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
	} else if err := writeFile(*lockPath, jsonData); err != nil {
		return err
	}
	if *tuPath != "" {
		if err := writeFile(*tuPath, tuData); err != nil {
			return err
		}
	}
	return nil
}

func writeFile(name string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(name), 0o777); err != nil {
		return fmt.Errorf("make %s: %w", filepath.Dir(name), err)
	}
	if err := os.WriteFile(name, data, 0o666); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	return nil
}

func checkFile(name string, want []byte) error {
	got, err := os.ReadFile(name)
	if err != nil {
		return fmt.Errorf("read %s: %w", name, err)
	}
	if !bytes.Equal(got, want) {
		return fmt.Errorf("%s is out of date", name)
	}
	return nil
}
