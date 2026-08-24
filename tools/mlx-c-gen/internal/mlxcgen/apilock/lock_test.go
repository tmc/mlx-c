package apilock

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateSeparatesTargets(t *testing.T) {
	dir := t.TempDir()
	writeHeader(t, dir, "mlx.h", `
#include "mlx/c/config.h"
#include "mlx/c/array.h"
`)
	writeHeader(t, dir, "config.h", `#define MLX_C_HAS_JACCL 0`)
	writeHeader(t, dir, "array.h", `
typedef struct mlx_array_ {
  void* ctx;
} mlx_array;
typedef enum mlx_dtype_ {
  MLX_BOOL = 0,
  MLX_FLOAT32,
} mlx_dtype;
int mlx_array_free(mlx_array arr);
`)
	writeHeader(t, dir, "jaccl.h", `
typedef struct mlx_jaccl_group_ {
  void* ctx;
} mlx_jaccl_group;
typedef enum mlx_jaccl_dtype_ {
  MLX_JACCL_BOOL = 0,
  MLX_JACCL_FLOAT32,
} mlx_jaccl_dtype;
int mlx_jaccl_group_free(mlx_jaccl_group group);
`)

	lock, err := Generate(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(lock.Targets["mlxc"].Functions) != 1 {
		t.Fatalf("mlxc functions = %d, want 1", len(lock.Targets["mlxc"].Functions))
	}
	if got := lock.Targets["mlxc"].Functions[0].Name; got != "mlx_array_free" {
		t.Fatalf("mlxc function = %q", got)
	}
	if got := lock.Targets["jacclc"].Functions[0].Name; got != "mlx_jaccl_group_free" {
		t.Fatalf("jacclc function = %q", got)
	}
	if len(lock.Targets["mlxc"].Macros) != 1 ||
		lock.Targets["mlxc"].Macros[0].Name != "MLX_C_HAS_JACCL" ||
		lock.Targets["mlxc"].Macros[0].Definition != "0" {
		t.Fatalf("mlxc macros = %#v, want MLX_C_HAS_JACCL", lock.Targets["mlxc"].Macros)
	}

	tu, err := lock.LockC()
	if err != nil {
		t.Fatal(err)
	}
	text := string(tu)
	for _, want := range []string{
		`#if !defined(MLX_C_HAS_JACCL)`,
		`#elif MLX_C_HAS_JACCL != 0 && MLX_C_HAS_JACCL != 1`,
		`_Static_assert(MLX_FLOAT32 == 1, "mlx_dtype.MLX_FLOAT32 ABI break");`,
		`_Static_assert(sizeof(mlx_jaccl_group) == sizeof(void *), "mlx_jaccl_group ABI break");`,
		`extern int mlx_array_free(mlx_array arr);`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("lock.c missing %q\n%s", want, text)
		}
	}
}

func TestParseFunctionPointerTypedefAndStructField(t *testing.T) {
	dir := t.TempDir()
	writeHeader(t, dir, "mlx.h", `#include "mlx/c/io.h"`)
	writeHeader(t, dir, "io.h", `
typedef void (*mlx_error_handler_func)(const char* msg, void* data);
typedef struct mlx_io_vtable_ {
  bool (*is_open)(void*);
  void (*read)(void*, char* data, size_t n);
} mlx_io_vtable;
void mlx_set_error_handler(mlx_error_handler_func handler, void* data);
`)
	writeHeader(t, dir, "jaccl.h", `int mlx_jaccl_is_available(void);`)

	lock, err := Generate(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := lock.Targets["mlxc"].Typedefs[0].Name; got != "mlx_error_handler_func" {
		t.Fatalf("typedef = %q", got)
	}
	fields := lock.Targets["mlxc"].Structs[0].Fields
	if len(fields) != 2 || fields[0].Name != "is_open" || fields[1].Name != "read" {
		t.Fatalf("fields = %#v", fields)
	}
}

func TestFunctionPointerArgumentDoesNotFuseDeclarations(t *testing.T) {
	dir := t.TempDir()
	writeHeader(t, dir, "mlx.h", `
#include "mlx/c/closure.h"
#include "mlx/c/random.h"
`)
	writeHeader(t, dir, "closure.h", `
typedef struct mlx_closure_custom_jvp_ {
  void* ctx;
} mlx_closure_custom_jvp;
typedef struct mlx_vector_array_ {
  void* ctx;
} mlx_vector_array;
mlx_closure_custom_jvp mlx_closure_custom_jvp_new_func(
    int (*fun)(
        mlx_vector_array*,
        const mlx_vector_array,
        const mlx_vector_array,
        const int*,
        size_t _num));
`)
	writeHeader(t, dir, "random.h", `
typedef struct mlx_array_ {
  void* ctx;
} mlx_array;
typedef struct mlx_stream_ {
  void* ctx;
} mlx_stream;
int mlx_random_categorical_num_samples(
    mlx_array* res,
    const mlx_array logits_,
    int axis,
    int num_samples,
    const mlx_array key,
    const mlx_stream s);
`)
	writeHeader(t, dir, "jaccl.h", `int mlx_jaccl_is_available(void);`)

	lock, err := Generate(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, fn := range lock.Targets["mlxc"].Functions {
		got[fn.Name] = fn.Signature
	}
	if strings.Contains(got["mlx_closure_custom_jvp_new_func"], "num_samples") {
		t.Fatalf("fused closure signature: %s", got["mlx_closure_custom_jvp_new_func"])
	}
	if !strings.Contains(got["mlx_random_categorical_num_samples"], "num_samples") {
		t.Fatalf("lost random signature: %s", got["mlx_random_categorical_num_samples"])
	}
}

func TestParseHeaderContent(t *testing.T) {
	target, err := ParseHeaderContent("hook.h", []byte(`
typedef struct mlx_node_namer_ {
  void* ctx;
} mlx_node_namer;
mlx_node_namer mlx_node_namer_new(void);
int mlx_export_to_dot(FILE* os, const mlx_node_namer namer);
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(target.Structs) != 1 || target.Structs[0].Name != "mlx_node_namer" {
		t.Fatalf("structs = %#v", target.Structs)
	}
	if len(target.Functions) != 2 || target.Functions[0].Name != "mlx_export_to_dot" || target.Functions[1].Name != "mlx_node_namer_new" {
		t.Fatalf("functions = %#v", target.Functions)
	}
}

func writeHeader(t *testing.T, dir, name, data string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o666); err != nil {
		t.Fatal(err)
	}
}
