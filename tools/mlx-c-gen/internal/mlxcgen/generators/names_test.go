package generators

import "testing"

func TestCNamespace(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		want      string
	}{
		{"core", "mlx::core", "mlx"},
		{"cuda alias", "mlx::core::cu", "mlx_cuda"},
		{"nested cuda namespace", "mlx::core::cu::detail", "mlx_cu_detail"},
		{"metal", "mlx::core::metal", "mlx_metal"},
		{"jaccl", "mlx::core::distributed::jaccl", "mlx_distributed_jaccl"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cNamespace(tt.namespace); got != tt.want {
				t.Fatalf("cNamespace(%q) = %q, want %q", tt.namespace, got, tt.want)
			}
		})
	}
}

func TestCFuncNameUsesNamespaceAlias(t *testing.T) {
	if got, want := cFuncName("mlx::core::cu", "is_available", ""), "mlx_cuda_is_available"; got != want {
		t.Fatalf("cFuncName(cuda) = %q, want %q", got, want)
	}
	if got, want := cFuncName("mlx::core::fft", "fft", "axis"), "mlx_fft_fft_axis"; got != want {
		t.Fatalf("cFuncName(variant) = %q, want %q", got, want)
	}
}

func TestToSnakeCase(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "CompileMode", "compile_mode"},
		{"acronym prefix", "FFTNorm", "fft_norm"},
		{"short word", "Dtype", "dtype"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := toSnakeCase(tt.in); got != tt.want {
				t.Fatalf("toSnakeCase(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
