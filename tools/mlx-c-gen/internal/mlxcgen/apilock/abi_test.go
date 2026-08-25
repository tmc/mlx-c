package apilock

import (
	"encoding/json"
	"testing"
)

func TestClassifyParam(t *testing.T) {
	tests := []struct {
		param string
		want  string
	}{
		{"mlx_array* res", "integer"},
		{"const mlx_array queries", "integer"},
		{"float scale", "float"},
		{"double x", "float"},
		{"double* out", "integer"},
		{"bool force_fused", "integer"},
		{"const char* mask_mode", "integer"},
		{"const mlx_stream s", "integer"},
		{"uint64_t n", "integer"},
	}
	for _, tt := range tests {
		if got := ClassifyParam(tt.param); got != tt.want {
			t.Errorf("ClassifyParam(%q) = %q, want %q", tt.param, got, tt.want)
		}
	}
}

func TestExtractImplBody(t *testing.T) {
	impl := `extern "C" mlx_event mlx_event_new(mlx_stream stream) {
  try {
    return mlx_event_new_(x);
  } catch (std::exception& e) {
    mlx_error(e.what());
    return mlx_event({nullptr});
  }
}

extern "C" int mlx_event_free(mlx_event event);
// a call site, not a definition:
void f(void) { mlx_event_new(s); }
`
	if got := ExtractImplBody(impl, "mlx_event_new"); got == "" {
		t.Error("mlx_event_new body not found")
	} else if !contains(got, "mlx_error") {
		t.Errorf("body missing error path: %q", got)
	}
	if got := ExtractImplBody(impl, "mlx_event_free"); got != "" {
		t.Errorf("declaration-only function returned body %q", got)
	}
}

func contains(s, sub string) bool { return len(sub) == 0 || indexOf(s, sub) >= 0 }
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestEntryFromFunction(t *testing.T) {
	fn := Function{
		Name:   "mlx_fast_scaled_dot_product_attention",
		Return: "int",
		Parameters: []string{
			"mlx_array* res", "const mlx_array queries", "const mlx_array keys",
			"const mlx_array values", "float scale", "const char* mask_mode",
			"const mlx_array mask_arr", "const mlx_array sinks",
			"bool force_fused", "const mlx_stream s",
		},
	}
	e := entryFromFunction(fn, ABIClassAPI, "")
	if e.IntegerClassCount != 9 || e.FloatClassCount != 1 {
		t.Errorf("got %d/%d, want 9/1", e.IntegerClassCount, e.FloatClassCount)
	}
	if !e.Fallible {
		t.Error("int-returning function must be fallible")
	}
	if e.Class != ABIClassAPI {
		t.Errorf("class = %q, want api", e.Class)
	}

	byValue := Function{Name: "mlx_event_new", Return: "mlx_event",
		Parameters: []string{"mlx_stream stream"}}
	e2 := entryFromFunction(byValue, ABIClassAPI, "mlx_error(e.what());")
	if !e2.Fallible {
		t.Error("by-value with error path must be fallible")
	}
	e3 := entryFromFunction(Function{Name: "mlx_stream_new", Return: "mlx_stream"},
		ABIClassAPI, "return mlx_stream_new_();")
	if e3.Fallible {
		t.Error("by-value without error path must not be fallible")
	}
	data, err := ABIMap{SchemaVersion: ABISchemaVersion,
		Functions: map[string]ABIEntry{"a": e}}.JSON()
	if err != nil {
		t.Fatal(err)
	}
	var back ABIMap
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if back.SchemaVersion != ABISchemaVersion || back.Functions["a"].Fallible != true {
		t.Errorf("roundtrip mismatch: %+v", back)
	}
}
