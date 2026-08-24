package generators

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tmc/mlx-c/tools/mlx-c-gen/internal/mlxcgen/parser"
)

func TestGenerateMetalSkipsDeviceInfo(t *testing.T) {
	result := &parser.ParseResult{
		Functions: map[string][]*parser.Function{
			"mlx::core::metal::device_info": {{
				Name:       "device_info",
				Namespace:  "mlx::core::metal",
				ReturnType: "std::unordered_map<std::string, std::variant<std::string, size_t>>",
			}},
			"mlx::core::metal::is_available": {{
				Name:       "is_available",
				Namespace:  "mlx::core::metal",
				ReturnType: "bool",
				Doc:        "Return true when Metal is available.",
			}},
			"mlx::core::metal::start_capture": {{
				Name:       "start_capture",
				Namespace:  "mlx::core::metal",
				ReturnType: "void",
				ParamTypes: []string{"std::string"},
				ParamNames: []string{"path"},
			}},
			"mlx::core::metal::stop_capture": {{
				Name:       "stop_capture",
				Namespace:  "mlx::core::metal",
				ReturnType: "void",
			}},
		},
		Enums: map[string]*parser.Enum{},
	}

	var header bytes.Buffer
	if err := New().Generate(&header, result, "metal", nil, false, "Metal specific operations"); err != nil {
		t.Fatalf("Generate header: %v", err)
	}
	text := header.String()
	for _, want := range []string{
		" * Return true when Metal is available.",
		"int mlx_metal_is_available(bool* res);",
		"int mlx_metal_start_capture(const char* path);",
		"int mlx_metal_stop_capture(void);",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("header missing %q\n%s", want, text)
		}
	}
	if strings.Contains(text, "metal_device_info") {
		t.Fatalf("header contains obsolete metal_device_info\n%s", text)
	}
}
