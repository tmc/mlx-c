package main

import (
	"testing"

	"github.com/tmc/mlx-c/tools/mlx-c-gen/internal/mlxcgen/parser"
)

func TestDbgExtractScatter(t *testing.T) {
	parser.SetIncludePaths([]string{"/Users/tmc/ml-explore/mlx-src-0321"})
	res, err := parser.ParseFiles([]string{"/Users/tmc/ml-explore/mlx-src-0321/mlx/ops.h"})
	if err != nil {
		t.Fatal(err)
	}
	for k, fs := range res.Functions {
		if len(fs) > 0 && fs[0].Name == "scatter" {
			t.Logf("KEY=%s DOC=%q", k, fs[0].Doc)
		}
	}
}
