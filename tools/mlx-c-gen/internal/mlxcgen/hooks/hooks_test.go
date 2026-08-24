package hooks

import (
	"bytes"
	"strings"
	"testing"
)

func TestGraphUtilsHooks(t *testing.T) {
	tests := []struct {
		name     string
		funcName string
		impl     bool
		want     []string
	}{
		{
			name:     "export header",
			funcName: "mlx_export_to_dot",
			want: []string{
				"typedef struct mlx_node_namer_",
				"mlx_node_namer mlx_node_namer_new()",
				"int mlx_export_to_dot(",
			},
		},
		{
			name:     "export implementation",
			funcName: "mlx_export_to_dot",
			impl:     true,
			want: []string{
				"extern \"C\" mlx_node_namer mlx_node_namer_new()",
				"extern \"C\" int mlx_export_to_dot(",
				"CFileOutputStream::as_lvalue(CFileOutputStream(os))",
			},
		},
		{
			name:     "print header",
			funcName: "mlx_print_graph",
			want:     []string{"int mlx_print_graph("},
		},
		{
			name:     "print implementation",
			funcName: "mlx_print_graph",
			impl:     true,
			want: []string{
				"extern \"C\" int mlx_print_graph(",
				"CFileOutputStream::as_lvalue(CFileOutputStream(os))",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hook := GetHook(tt.funcName)
			if hook == nil {
				t.Fatalf("GetHook(%q) = nil", tt.funcName)
			}
			var buf bytes.Buffer
			if !hook(&buf, tt.funcName, tt.impl) {
				t.Fatalf("hook(%q) returned false", tt.funcName)
			}
			got := buf.String()
			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Fatalf("hook output missing %q\n%s", want, got)
				}
			}
		})
	}
}

func TestGGUFHooks(t *testing.T) {
	tests := []struct {
		name     string
		funcName string
		impl     bool
		want     []string
	}{
		{
			name:     "load header",
			funcName: "mlx_load_gguf",
			want:     []string{"int mlx_load_gguf(mlx_io_gguf* gguf"},
		},
		{
			name:     "load implementation",
			funcName: "mlx_load_gguf",
			impl:     true,
			want: []string{
				"extern \"C\" int",
				"mlx::core::load_gguf(file, mlx_stream_get_(s))",
				"mlx_io_gguf_set_(*gguf",
			},
		},
		{
			name:     "save header",
			funcName: "mlx_save_gguf",
			want:     []string{"int mlx_save_gguf(const char* file, mlx_io_gguf gguf)"},
		},
		{
			name:     "save implementation",
			funcName: "mlx_save_gguf",
			impl:     true,
			want: []string{
				"extern \"C\" int mlx_save_gguf",
				"mlx_io_gguf_get_(gguf)",
				"mlx::core::save_gguf(file, cpp_gguf.first, cpp_gguf.second)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hook := GetHook(tt.funcName)
			if hook == nil {
				t.Fatalf("GetHook(%q) = nil", tt.funcName)
			}
			var buf bytes.Buffer
			if !hook(&buf, tt.funcName, tt.impl) {
				t.Fatalf("hook(%q) returned false", tt.funcName)
			}
			got := buf.String()
			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Fatalf("hook output missing %q\n%s", want, got)
				}
			}
		})
	}
}

func TestMetalDeviceInfoHookRemoved(t *testing.T) {
	if HasHook("mlx_metal_device_info") {
		t.Fatal("mlx_metal_device_info hook is obsolete; device_info should be skipped by variants")
	}
}

func TestNamesSorted(t *testing.T) {
	got := Names()
	if len(got) == 0 {
		t.Fatal("Names returned no hooks")
	}
	for i := 1; i < len(got); i++ {
		if got[i-1] > got[i] {
			t.Fatalf("Names = %#v, want sorted", got)
		}
	}
	for _, want := range []string{"mlx_export_to_dot", "mlx_print_graph"} {
		found := false
		for _, name := range got {
			if name == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("Names missing %s: %#v", want, got)
		}
	}
}
