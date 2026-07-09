package customspec

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/tmc/mlx-c/internal/mlxcgen/apilock"
)

func TestLoad(t *testing.T) {
	spec, err := Load(strings.NewReader(`
schema_version: 1
name: jaccl
target: jacclc
header: mlx/c/jaccl.h
ownership: handwritten_runtime
items:
  - kind: struct
    name: mlx_jaccl_group
    action: handwritten
    reason: runtime_handle
  - kind: function
    name: mlx_jaccl_group_free
    action: handwritten
    reason: runtime_lifetime
    signature: int mlx_jaccl_group_free(mlx_jaccl_group group)
`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if spec.Name != "jaccl" || spec.Target != "jacclc" || len(spec.Items) != 2 {
		t.Fatalf("spec = %#v", spec)
	}
}

func TestGeneratedHeaders(t *testing.T) {
	got := GeneratedHeaders([]Spec{
		{Header: "mlx/c/ops.h"},
		{Header: "mlx/c/jaccl.h", Generate: GenerateSpec{Header: true}},
		{Header: "mlx/c/fft.h", Generate: GenerateSpec{Header: true}},
	})
	want := "mlx/c/fft.h\nmlx/c/jaccl.h"
	if strings.Join(got, "\n") != want {
		t.Fatalf("headers = %#v, want %s", got, want)
	}
}

func TestLoadDirRejectsDuplicateTargetHeader(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"one.yaml", "two.yaml"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(`
schema_version: 1
name: jaccl
target: jacclc
header: mlx/c/jaccl.h
ownership: handwritten_runtime
items:
  - kind: function
    name: mlx_jaccl_group_free
    action: handwritten
    reason: runtime_lifetime
    signature: int mlx_jaccl_group_free(mlx_jaccl_group group)
`), 0o666); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	_, err := LoadDir(dir)
	if err == nil {
		t.Fatal("LoadDir succeeded, want error")
	}
	if !strings.Contains(err.Error(), "duplicate custom spec for jacclc:mlx/c/jaccl.h") {
		t.Fatalf("error = %v, want duplicate target/header", err)
	}
}

func TestRenderHeader(t *testing.T) {
	spec := Spec{
		SchemaVersion: 1,
		Name:          "jaccl",
		Target:        "jacclc",
		Header:        "mlx/c/jaccl.h",
		Ownership:     "handwritten_runtime",
		Generate:      GenerateSpec{Header: true},
		Copyright:     "Copyright \u00a9 2026 Apple Inc.",
		IncludeGuard:  "MLX_JACCL_H",
		Includes:      []string{"stdbool.h"},
		Group: Group{
			Name:  "mlx_jaccl",
			Title: "JACCL",
			Doc:   "Standalone C API for libjaccl.",
		},
		Items: []Item{
			{
				Kind:   "struct",
				Name:   "mlx_jaccl_group",
				Action: "custom_spec",
				Reason: "runtime_handle",
				Doc:    "A JACCL communication group.",
				Opaque: true,
			},
			{
				Kind:      "function",
				Name:      "mlx_jaccl_is_available",
				Action:    "handwritten",
				Reason:    "optional_runtime",
				Doc:       "Check if JACCL is available on this system.",
				Signature: "bool mlx_jaccl_is_available(void)",
			},
		},
	}
	got, err := RenderHeader(spec)
	if err != nil {
		t.Fatalf("RenderHeader: %v", err)
	}
	for _, want := range []string{
		"/* This file is auto-generated. Do not edit manually. */",
		"#ifndef MLX_JACCL_H",
		"#include <stdbool.h>",
		"\\defgroup mlx_jaccl JACCL",
		"typedef struct mlx_jaccl_group_ { void* ctx; } mlx_jaccl_group;",
		"bool mlx_jaccl_is_available(void);",
	} {
		if !strings.Contains(string(got), want) {
			t.Fatalf("header missing %q:\n%s", want, got)
		}
	}
}

func TestCheckLock(t *testing.T) {
	lock := &apilock.Lock{
		Targets: map[string]apilock.Target{
			"jacclc": {
				Structs: []apilock.Struct{{
					Name:   "mlx_jaccl_group",
					Header: "mlx/c/jaccl.h",
				}},
				Functions: []apilock.Function{{
					Name:      "mlx_jaccl_group_free",
					Header:    "mlx/c/jaccl.h",
					Signature: "int mlx_jaccl_group_free(mlx_jaccl_group group)",
				}},
			},
		},
	}
	specs := []Spec{{
		SchemaVersion: 1,
		Name:          "jaccl",
		Target:        "jacclc",
		Header:        "mlx/c/jaccl.h",
		Ownership:     "handwritten_runtime",
		Items: []Item{
			{Kind: "struct", Name: "mlx_jaccl_group", Action: "handwritten", Reason: "runtime_handle"},
			{Kind: "function", Name: "mlx_jaccl_group_free", Action: "handwritten", Reason: "runtime_lifetime", Signature: "int mlx_jaccl_group_free(mlx_jaccl_group group)"},
		},
	}}
	if err := CheckLock(lock, specs); err != nil {
		t.Fatalf("CheckLock: %v", err)
	}
}

func TestCheckImplementations(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "mlx/c"), 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "mlx/c/jaccl.cpp"), []byte(`
extern "C" int mlx_jaccl_group_free(mlx_jaccl_group group) {
  return 0;
}

extern "C" int
mlx_jaccl_recv(mlx_jaccl_group group, void* output, size_t n_bytes, int src) {
  return 0;
}

extern "C" int mlx_jaccl_declaration_only(void);
`), 0o666); err != nil {
		t.Fatal(err)
	}
	specs := []Spec{{
		SchemaVersion:  1,
		Name:           "jaccl",
		Target:         "jacclc",
		Header:         "mlx/c/jaccl.h",
		Implementation: "mlx/c/jaccl.cpp",
		Ownership:      "handwritten_runtime",
		Items: []Item{
			{Kind: "function", Name: "mlx_jaccl_group_free", Action: "handwritten", Reason: "runtime_lifetime", Signature: "int mlx_jaccl_group_free(mlx_jaccl_group group)"},
			{Kind: "function", Name: "mlx_jaccl_recv", Action: "handwritten", Reason: "point_to_point_wrapper", Signature: "int mlx_jaccl_recv(mlx_jaccl_group group, void* output, size_t n_bytes, int src)"},
		},
	}}
	if err := CheckImplementations(root, specs); err != nil {
		t.Fatalf("CheckImplementations: %v", err)
	}
}

func TestCheckImplementationsReportsMissingFunction(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "mlx/c"), 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "mlx/c/jaccl.cpp"), []byte(`
extern "C" int mlx_jaccl_group_free(mlx_jaccl_group group);
`), 0o666); err != nil {
		t.Fatal(err)
	}
	specs := []Spec{{
		SchemaVersion:  1,
		Name:           "jaccl",
		Target:         "jacclc",
		Header:         "mlx/c/jaccl.h",
		Implementation: "mlx/c/jaccl.cpp",
		Ownership:      "handwritten_runtime",
		Items: []Item{
			{Kind: "function", Name: "mlx_jaccl_group_free", Action: "handwritten", Reason: "runtime_lifetime", Signature: "int mlx_jaccl_group_free(mlx_jaccl_group group)"},
		},
	}}
	err := CheckImplementations(root, specs)
	if err == nil {
		t.Fatal("CheckImplementations succeeded, want error")
	}
	if !strings.Contains(err.Error(), `missing extern "C" definition for function:mlx_jaccl_group_free`) {
		t.Fatalf("error = %v, want missing definition", err)
	}
}

func TestCheckImplementationsReportsUnexpectedFunction(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "mlx/c"), 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "mlx/c/jaccl.cpp"), []byte(`
extern "C" int mlx_jaccl_group_free(mlx_jaccl_group group) {
  return 0;
}

extern "C" int mlx_jaccl_extra(void) {
  return 0;
}
`), 0o666); err != nil {
		t.Fatal(err)
	}
	specs := []Spec{{
		SchemaVersion:  1,
		Name:           "jaccl",
		Target:         "jacclc",
		Header:         "mlx/c/jaccl.h",
		Implementation: "mlx/c/jaccl.cpp",
		Ownership:      "handwritten_runtime",
		Items: []Item{
			{Kind: "function", Name: "mlx_jaccl_group_free", Action: "handwritten", Reason: "runtime_lifetime", Signature: "int mlx_jaccl_group_free(mlx_jaccl_group group)"},
		},
	}}
	err := CheckImplementations(root, specs)
	if err == nil {
		t.Fatal("CheckImplementations succeeded, want error")
	}
	if !strings.Contains(err.Error(), `unexpected extern "C" definition for function:mlx_jaccl_extra`) {
		t.Fatalf("error = %v, want unexpected definition", err)
	}
}

func TestCheckImplementationsReportsMissingImplementation(t *testing.T) {
	specs := []Spec{{
		SchemaVersion: 1,
		Name:          "jaccl",
		Target:        "jacclc",
		Header:        "mlx/c/jaccl.h",
		Ownership:     "handwritten_runtime",
		Items: []Item{
			{Kind: "function", Name: "mlx_jaccl_group_free", Action: "handwritten", Reason: "runtime_lifetime", Signature: "int mlx_jaccl_group_free(mlx_jaccl_group group)"},
		},
	}}
	err := CheckImplementations("", specs)
	if err == nil {
		t.Fatal("CheckImplementations succeeded, want error")
	}
	if !strings.Contains(err.Error(), "missing implementation") {
		t.Fatalf("error = %v, want missing implementation", err)
	}
}

func TestCheckLockReportsSignatureMismatch(t *testing.T) {
	lock := &apilock.Lock{
		Targets: map[string]apilock.Target{
			"jacclc": {
				Functions: []apilock.Function{{
					Name:      "mlx_jaccl_group_free",
					Header:    "mlx/c/jaccl.h",
					Signature: "int mlx_jaccl_group_free(mlx_jaccl_group group)",
				}},
			},
		},
	}
	specs := []Spec{{
		SchemaVersion: 1,
		Name:          "jaccl",
		Target:        "jacclc",
		Header:        "mlx/c/jaccl.h",
		Ownership:     "handwritten_runtime",
		Items: []Item{
			{Kind: "function", Name: "mlx_jaccl_group_free", Action: "handwritten", Reason: "runtime_lifetime", Signature: "void mlx_jaccl_group_free(mlx_jaccl_group group)"},
		},
	}}
	err := CheckLock(lock, specs)
	if err == nil {
		t.Fatal("CheckLock succeeded, want error")
	}
	if !strings.Contains(err.Error(), "signature =") {
		t.Fatalf("error = %v, want signature mismatch", err)
	}
}

func TestLoadRejectsUnknownKindAndAction(t *testing.T) {
	_, err := Load(strings.NewReader(`
schema_version: 1
name: bad
target: jacclc
header: mlx/c/jaccl.h
ownership: handwritten_runtime
items:
  - kind: thing
    name: mlx_jaccl_group
    action: magic
`))
	if err == nil {
		t.Fatal("Load succeeded, want error")
	}
	for _, want := range []string{"unknown kind thing", "unknown action magic"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %v, want %q", err, want)
		}
	}
}

func TestLoadRejectsUnknownTarget(t *testing.T) {
	_, err := Load(strings.NewReader(`
schema_version: 1
name: bad
target: other
header: mlx/c/jaccl.h
ownership: handwritten_runtime
items:
  - kind: function
    name: mlx_jaccl_group_free
    action: handwritten
    reason: runtime_lifetime
    signature: int mlx_jaccl_group_free(mlx_jaccl_group group)
`))
	if err == nil {
		t.Fatal("Load succeeded, want error")
	}
	if !strings.Contains(err.Error(), "unknown target other") {
		t.Fatalf("error = %v, want unknown target", err)
	}
}

func TestLoadRejectsUnknownOwnership(t *testing.T) {
	_, err := Load(strings.NewReader(`
schema_version: 1
name: bad
target: jacclc
header: mlx/c/jaccl.h
ownership: generated_elsewhere
items:
  - kind: function
    name: mlx_jaccl_group_free
    action: handwritten
    reason: runtime_lifetime
    signature: int mlx_jaccl_group_free(mlx_jaccl_group group)
`))
	if err == nil {
		t.Fatal("Load succeeded, want error")
	}
	if !strings.Contains(err.Error(), "unknown ownership generated_elsewhere") {
		t.Fatalf("error = %v, want unknown ownership", err)
	}
}

func TestLoadRejectsMissingReason(t *testing.T) {
	_, err := Load(strings.NewReader(`
schema_version: 1
name: bad
target: jacclc
header: mlx/c/jaccl.h
ownership: handwritten_runtime
items:
  - kind: function
    name: mlx_jaccl_group_free
    action: handwritten
    signature: int mlx_jaccl_group_free(mlx_jaccl_group group)
`))
	if err == nil {
		t.Fatal("Load succeeded, want error")
	}
	if !strings.Contains(err.Error(), "missing reason") {
		t.Fatalf("error = %v, want missing reason", err)
	}
}

func TestLoadRejectsUnknownReason(t *testing.T) {
	_, err := Load(strings.NewReader(`
schema_version: 1
name: bad
target: jacclc
header: mlx/c/jaccl.h
ownership: handwritten_runtime
items:
  - kind: function
    name: mlx_jaccl_group_free
    action: handwritten
    reason: maybe_later
    signature: int mlx_jaccl_group_free(mlx_jaccl_group group)
`))
	if err == nil {
		t.Fatal("Load succeeded, want error")
	}
	if !strings.Contains(err.Error(), "unknown reason maybe_later") {
		t.Fatalf("error = %v, want unknown reason", err)
	}
}

func TestLoadRejectsHeaderOutsideMLXC(t *testing.T) {
	for _, header := range []string{"include/jaccl.h", "mlx/c/../jaccl.h"} {
		_, err := Load(strings.NewReader(`
schema_version: 1
name: bad
target: jacclc
header: ` + header + `
ownership: handwritten_runtime
items:
  - kind: function
    name: mlx_jaccl_group_free
    action: handwritten
    reason: runtime_lifetime
    signature: int mlx_jaccl_group_free(mlx_jaccl_group group)
`))
		if err == nil {
			t.Fatalf("Load accepted header %q", header)
		}
		if !strings.Contains(err.Error(), "must be under mlx/c") {
			t.Fatalf("error = %v, want header error", err)
		}
	}
}

func TestLoadRejectsImplementationOutsideMLXC(t *testing.T) {
	for _, implementation := range []string{"src/jaccl.cpp", "mlx/c/../jaccl.cpp"} {
		_, err := Load(strings.NewReader(`
schema_version: 1
name: bad
target: jacclc
header: mlx/c/jaccl.h
implementation: ` + implementation + `
ownership: handwritten_runtime
items:
  - kind: function
    name: mlx_jaccl_group_free
    action: handwritten
    reason: runtime_lifetime
    signature: int mlx_jaccl_group_free(mlx_jaccl_group group)
`))
		if err == nil {
			t.Fatalf("Load accepted implementation %q", implementation)
		}
		if !strings.Contains(err.Error(), "must be under mlx/c") {
			t.Fatalf("error = %v, want implementation error", err)
		}
	}
}

func TestLoadRejectsInvalidIncludes(t *testing.T) {
	for _, include := range []string{"", "../private.h", "stdio.h>", "sys/types.h ", "sys types.h"} {
		_, err := Load(strings.NewReader(`
schema_version: 1
name: bad
target: jacclc
header: mlx/c/jaccl.h
ownership: handwritten_runtime
generate:
  header: true
copyright: Copyright
include_guard: MLX_JACCL_H
includes:
  - "` + include + `"
group:
  name: mlx_jaccl
  title: JACCL
  doc: Standalone C API for libjaccl.
items:
  - kind: function
    name: mlx_jaccl_group_free
    action: handwritten
    reason: runtime_lifetime
    doc: Free a group.
    signature: int mlx_jaccl_group_free(mlx_jaccl_group group)
`))
		if err == nil {
			t.Fatalf("Load accepted include %q", include)
		}
		if !strings.Contains(err.Error(), "is not a valid header include") {
			t.Fatalf("error = %v, want include error", err)
		}
	}
}

func TestLoadRejectsDuplicateIncludes(t *testing.T) {
	_, err := Load(strings.NewReader(`
schema_version: 1
name: bad
target: jacclc
header: mlx/c/jaccl.h
ownership: handwritten_runtime
generate:
  header: true
copyright: Copyright
include_guard: MLX_JACCL_H
includes:
  - stdbool.h
  - stdbool.h
group:
  name: mlx_jaccl
  title: JACCL
  doc: Standalone C API for libjaccl.
items:
  - kind: function
    name: mlx_jaccl_group_free
    action: handwritten
    reason: runtime_lifetime
    doc: Free a group.
    signature: int mlx_jaccl_group_free(mlx_jaccl_group group)
`))
	if err == nil {
		t.Fatal("Load succeeded, want error")
	}
	if !strings.Contains(err.Error(), `duplicate "stdbool.h"`) {
		t.Fatalf("error = %v, want duplicate include", err)
	}
}

func TestLoadRejectsInvalidGeneratedIdentifiers(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{
			name: "include guard",
			body: `
include_guard: MLX-JACCL-H
group:
  name: mlx_jaccl
  title: JACCL
  doc: Standalone C API for libjaccl.
items:
  - kind: function
    name: mlx_jaccl_group_free
    action: handwritten
    reason: runtime_lifetime
    doc: Free a group.
    signature: int mlx_jaccl_group_free(mlx_jaccl_group group)
`,
			want: "include_guard MLX-JACCL-H is not a valid C identifier",
		},
		{
			name: "group name",
			body: `
include_guard: MLX_JACCL_H
group:
  name: mlx-jaccl
  title: JACCL
  doc: Standalone C API for libjaccl.
items:
  - kind: function
    name: mlx_jaccl_group_free
    action: handwritten
    reason: runtime_lifetime
    doc: Free a group.
    signature: int mlx_jaccl_group_free(mlx_jaccl_group group)
`,
			want: "group name mlx-jaccl is not a valid C identifier",
		},
		{
			name: "item name",
			body: `
include_guard: MLX_JACCL_H
group:
  name: mlx_jaccl
  title: JACCL
  doc: Standalone C API for libjaccl.
items:
  - kind: function
    name: 1mlx_jaccl_group_free
    action: handwritten
    reason: runtime_lifetime
    doc: Free a group.
    signature: int mlx_jaccl_group_free(mlx_jaccl_group group)
`,
			want: "items[0]: name 1mlx_jaccl_group_free is not a valid C identifier",
		},
		{
			name: "enum value",
			body: `
include_guard: MLX_JACCL_H
group:
  name: mlx_jaccl
  title: JACCL
  doc: Standalone C API for libjaccl.
items:
  - kind: enum
    name: mlx_jaccl_dtype
    action: custom_spec
    reason: dtype_table
    doc: Element type.
    values:
      - name: MLX-JACCL-FLOAT32
        value: 11
`,
			want: "items[0].values[0]: name MLX-JACCL-FLOAT32 is not a valid C identifier",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load(strings.NewReader(`
schema_version: 1
name: bad
target: jacclc
header: mlx/c/jaccl.h
ownership: handwritten_runtime
generate:
  header: true
copyright: Copyright
` + tc.body))
			if err == nil {
				t.Fatal("Load succeeded, want error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestLoadRejectsDuplicateEnumValues(t *testing.T) {
	for _, tc := range []struct {
		name   string
		values string
		want   string
	}{
		{
			name: "name",
			values: `
      - name: MLX_JACCL_FLOAT32
        value: 11
      - name: MLX_JACCL_FLOAT32
        value: 12
`,
			want: "items[0].values[1]: duplicate name MLX_JACCL_FLOAT32",
		},
		{
			name: "value",
			values: `
      - name: MLX_JACCL_FLOAT32
        value: 11
      - name: MLX_JACCL_FLOAT64
        value: 11
`,
			want: "items[0].values[1]: duplicate value 11",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load(strings.NewReader(`
schema_version: 1
name: bad
target: jacclc
header: mlx/c/jaccl.h
ownership: handwritten_runtime
generate:
  header: true
copyright: Copyright
include_guard: MLX_JACCL_H
group:
  name: mlx_jaccl
  title: JACCL
  doc: Standalone C API for libjaccl.
items:
  - kind: enum
    name: mlx_jaccl_dtype
    action: custom_spec
    reason: dtype_table
    doc: Element type.
    values:
` + tc.values))
			if err == nil {
				t.Fatal("Load succeeded, want error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestLoadRejectsInvalidGeneratedDocs(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{
			name: "group title",
			body: `
group:
  name: mlx_jaccl
  title: JACCL */ break
  doc: Standalone C API for libjaccl.
items:
  - kind: function
    name: mlx_jaccl_group_free
    action: handwritten
    reason: runtime_lifetime
    doc: Free a group.
    signature: int mlx_jaccl_group_free(mlx_jaccl_group group)
`,
			want: "group title contains invalid comment text",
		},
		{
			name: "group doc",
			body: `
group:
  name: mlx_jaccl
  title: JACCL
  doc: Standalone */ C API for libjaccl.
items:
  - kind: function
    name: mlx_jaccl_group_free
    action: handwritten
    reason: runtime_lifetime
    doc: Free a group.
    signature: int mlx_jaccl_group_free(mlx_jaccl_group group)
`,
			want: "group doc contains invalid comment text",
		},
		{
			name: "item doc",
			body: `
group:
  name: mlx_jaccl
  title: JACCL
  doc: Standalone C API for libjaccl.
items:
  - kind: function
    name: mlx_jaccl_group_free
    action: handwritten
    reason: runtime_lifetime
    doc: Free */ a group.
    signature: int mlx_jaccl_group_free(mlx_jaccl_group group)
`,
			want: "items[0]: doc contains invalid comment text",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load(strings.NewReader(`
schema_version: 1
name: bad
target: jacclc
header: mlx/c/jaccl.h
ownership: handwritten_runtime
generate:
  header: true
copyright: Copyright
include_guard: MLX_JACCL_H
` + tc.body))
			if err == nil {
				t.Fatal("Load succeeded, want error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestLoadRejectsInvalidCopyright(t *testing.T) {
	for _, copyright := range []string{
		"Copyright */ break",
		"Copyright\nBad",
		" Copyright",
		"Copyright ",
	} {
		_, err := Load(strings.NewReader(`
schema_version: 1
name: bad
target: jacclc
header: mlx/c/jaccl.h
ownership: handwritten_runtime
generate:
  header: true
copyright: ` + strconv.Quote(copyright) + `
include_guard: MLX_JACCL_H
group:
  name: mlx_jaccl
  title: JACCL
  doc: Standalone C API for libjaccl.
items:
  - kind: function
    name: mlx_jaccl_group_free
    action: handwritten
    reason: runtime_lifetime
    doc: Free a group.
    signature: int mlx_jaccl_group_free(mlx_jaccl_group group)
`))
		if err == nil {
			t.Fatalf("Load accepted copyright %q", copyright)
		}
		if !strings.Contains(err.Error(), "copyright contains invalid comment text") {
			t.Fatalf("error = %v, want copyright error", err)
		}
	}
}

func TestLoadRejectsKindSpecificFieldMismatches(t *testing.T) {
	for _, tc := range []struct {
		name string
		item string
		want string
	}{
		{
			name: "enum opaque",
			item: `
  - kind: enum
    name: mlx_jaccl_dtype
    action: custom_spec
    reason: dtype_table
    doc: Element type.
    opaque: true
    values:
      - name: MLX_JACCL_FLOAT32
        value: 11
`,
			want: "items[0]: enum must not be opaque",
		},
		{
			name: "enum signature",
			item: `
  - kind: enum
    name: mlx_jaccl_dtype
    action: custom_spec
    reason: dtype_table
    doc: Element type.
    signature: int mlx_jaccl_dtype(void)
    values:
      - name: MLX_JACCL_FLOAT32
        value: 11
`,
			want: "items[0]: enum must not have signature",
		},
		{
			name: "function opaque",
			item: `
  - kind: function
    name: mlx_jaccl_group_free
    action: handwritten
    reason: runtime_lifetime
    doc: Free a group.
    signature: int mlx_jaccl_group_free(mlx_jaccl_group group)
    opaque: true
`,
			want: "items[0]: function must not be opaque",
		},
		{
			name: "function enum values",
			item: `
  - kind: function
    name: mlx_jaccl_group_free
    action: handwritten
    reason: runtime_lifetime
    doc: Free a group.
    signature: int mlx_jaccl_group_free(mlx_jaccl_group group)
    values:
      - name: MLX_JACCL_FLOAT32
        value: 11
`,
			want: "items[0]: function must not have enum values",
		},
		{
			name: "struct missing opaque",
			item: `
  - kind: struct
    name: mlx_jaccl_group
    action: custom_spec
    reason: runtime_handle
    doc: A group.
`,
			want: "items[0]: struct must be opaque",
		},
		{
			name: "struct signature",
			item: `
  - kind: struct
    name: mlx_jaccl_group
    action: custom_spec
    reason: runtime_handle
    doc: A group.
    signature: int mlx_jaccl_group(void)
    opaque: true
`,
			want: "items[0]: struct must not have signature",
		},
		{
			name: "struct enum values",
			item: `
  - kind: struct
    name: mlx_jaccl_group
    action: custom_spec
    reason: runtime_handle
    doc: A group.
    opaque: true
    values:
      - name: MLX_JACCL_FLOAT32
        value: 11
`,
			want: "items[0]: struct must not have enum values",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load(strings.NewReader(`
schema_version: 1
name: bad
target: jacclc
header: mlx/c/jaccl.h
ownership: handwritten_runtime
generate:
  header: true
copyright: Copyright
include_guard: MLX_JACCL_H
group:
  name: mlx_jaccl
  title: JACCL
  doc: Standalone C API for libjaccl.
items:
` + tc.item))
			if err == nil {
				t.Fatal("Load succeeded, want error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestLoadRejectsFunctionSignatureNameMismatch(t *testing.T) {
	_, err := Load(strings.NewReader(`
schema_version: 1
name: bad
target: jacclc
header: mlx/c/jaccl.h
ownership: handwritten_runtime
items:
  - kind: function
    name: mlx_jaccl_group_free
    action: handwritten
    reason: runtime_lifetime
    signature: int mlx_jaccl_group_delete(mlx_jaccl_group group)
`))
	if err == nil {
		t.Fatal("Load succeeded, want error")
	}
	if !strings.Contains(err.Error(), `signature name = "mlx_jaccl_group_delete", want "mlx_jaccl_group_free"`) {
		t.Fatalf("error = %v, want signature name mismatch", err)
	}
}

func TestLoadRejectsInvalidFunctionSignatureText(t *testing.T) {
	for _, signature := range []string{
		"int mlx_jaccl_group_free(mlx_jaccl_group group);",
		"int mlx_jaccl_group_free(mlx_jaccl_group group) { return 0; }",
		"int mlx_jaccl_group_free(mlx_jaccl_group group)\n#define BAD 1",
		"int mlx_jaccl_group_free(mlx_jaccl_group group) /* bad */",
		" int mlx_jaccl_group_free(mlx_jaccl_group group)",
	} {
		_, err := Load(strings.NewReader(`
schema_version: 1
name: bad
target: jacclc
header: mlx/c/jaccl.h
ownership: handwritten_runtime
items:
  - kind: function
    name: mlx_jaccl_group_free
    action: handwritten
    reason: runtime_lifetime
    signature: ` + strconv.Quote(signature) + `
`))
		if err == nil {
			t.Fatalf("Load accepted signature %q", signature)
		}
		if !strings.Contains(err.Error(), "signature contains invalid C declaration text") {
			t.Fatalf("error = %v, want invalid signature text", err)
		}
	}
}

func TestCheckLockReportsMissingAndExtra(t *testing.T) {
	lock := &apilock.Lock{
		Targets: map[string]apilock.Target{
			"jacclc": {
				Functions: []apilock.Function{{
					Name:   "mlx_jaccl_group_free",
					Header: "mlx/c/jaccl.h",
				}},
			},
		},
	}
	specs := []Spec{{
		SchemaVersion: 1,
		Name:          "jaccl",
		Target:        "jacclc",
		Header:        "mlx/c/jaccl.h",
		Ownership:     "handwritten_runtime",
		Items: []Item{
			{Kind: "function", Name: "mlx_jaccl_group_new", Action: "handwritten", Reason: "runtime_lifetime", Signature: "mlx_jaccl_group mlx_jaccl_group_new(void)"},
		},
	}}
	err := CheckLock(lock, specs)
	if err == nil {
		t.Fatal("CheckLock succeeded, want error")
	}
	for _, want := range []string{
		"missing custom spec item function:mlx_jaccl_group_free",
		"custom spec item function:mlx_jaccl_group_new is not in API lock",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %v, want %q", err, want)
		}
	}
}
