package inventory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadRejectsDuplicate(t *testing.T) {
	_, err := Read(strings.NewReader(`
generated_header_api mlxc mlx/c/ops.h
generated_header_api mlxc mlx/c/ops.h
`))
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("Read duplicate error = %v", err)
	}
}

func TestReadRejectsInvalidPaths(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "absolute", path: "/tmp/ops.h", want: "must be relative"},
		{name: "traversal", path: "../mlx/c/ops.h", want: "must be clean and stay under the repository"},
		{name: "unclean", path: "mlx/c/../ops.h", want: "must be clean and stay under the repository"},
		{name: "backslash", path: `mlx\c\ops.h`, want: "contains backslash"},
		{name: "outside", path: "include/ops.h", want: "must be under mlx/c"},
		{name: "extension", path: "mlx/c/ops.txt", want: "must have .h or .cpp extension"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Read(strings.NewReader("generated_header_api mlxc " + tt.path + "\n"))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Read error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestCheckCoversFilesAndUmbrella(t *testing.T) {
	root := t.TempDir()
	write(t, root, "CMakeLists.txt", `
set(mlxc-src
  ${CMAKE_CURRENT_LIST_DIR}/mlx/c/ops.cpp)
add_library(jacclc ${CMAKE_CURRENT_LIST_DIR}/mlx/c/jaccl.cpp)
`)
	write(t, root, "mlx/c/mlx.h", `#include "mlx/c/ops.h"`)
	write(t, root, "mlx/c/ops.h", "")
	write(t, root, "mlx/c/ops.cpp", "")
	write(t, root, "mlx/c/jaccl.h", "")
	write(t, root, "mlx/c/jaccl.cpp", "")
	write(t, root, "codegen/generated-files.txt", `
not_owned_by_codegen mlxc mlx/c/mlx.h
generated_header_api mlxc mlx/c/ops.h
generated_header_api mlxc mlx/c/ops.cpp
handwritten_runtime jacclc mlx/c/jaccl.h
handwritten_runtime jacclc mlx/c/jaccl.cpp
`)
	if err := Check(root, filepath.Join(root, "codegen/generated-files.txt")); err != nil {
		t.Fatal(err)
	}
}

func TestCheckFailsForUnclassifiedFile(t *testing.T) {
	root := t.TempDir()
	write(t, root, "CMakeLists.txt", "")
	write(t, root, "mlx/c/mlx.h", "")
	write(t, root, "mlx/c/extra.h", "")
	write(t, root, "codegen/generated-files.txt", `not_owned_by_codegen mlxc mlx/c/mlx.h`)
	err := Check(root, filepath.Join(root, "codegen/generated-files.txt"))
	if err == nil || !strings.Contains(err.Error(), "mlx/c/extra.h is not classified") {
		t.Fatalf("Check error = %v", err)
	}
}

func write(t *testing.T, root, name, data string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o666); err != nil {
		t.Fatal(err)
	}
}
