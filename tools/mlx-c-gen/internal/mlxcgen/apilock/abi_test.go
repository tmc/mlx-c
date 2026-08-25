package apilock

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

func TestExtractImplBodies(t *testing.T) {
	impl := `extern "C" mlx_event mlx_event_new(mlx_stream stream) {
  try { return w(); } catch (std::exception& e) {
    mlx_error(e.what());
    return mlx_event({nullptr});
  }
}

extern "C" int mlx_event_free(mlx_event event);

void f(void) { mlx_event_new(s); }
`
	got := ExtractImplBodies(impl, "mlx_event_new")
	if len(got) != 1 || !strings.Contains(got[0], "mlx_error") {
		t.Errorf("bodies = %q, want one body with error path", got)
	}
	if got := ExtractImplBodies(impl, "mlx_event_free"); got != nil {
		t.Errorf("declaration-only function returned %v", got)
	}
	if got := ExtractImplBodies(impl, "f"); got != nil {
		t.Errorf("unrelated function matched: %v", got)
	}
}

func TestExtractImplBodiesMultiple(t *testing.T) {
	impl := `extern "C" uint64_t f(void) { return 1; }
extern "C" uint64_t f(void) { mlx_error("x"); return 2; }
`
	bodies := ExtractImplBodies(impl, "f")
	if len(bodies) != 2 {
		t.Fatalf("got %d bodies, want 2", len(bodies))
	}
	if got := ExtractImplBodies(impl, "g"); got != nil {
		t.Errorf("unknown function returned %v", got)
	}
}

func TestFallibleTransitivity(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("stream.h", `#ifndef S_H
#define S_H
mlx_stream mlx_stream_new(void);
mlx_stream mlx_stream_new_thread_unsafe_device(mlx_device dev);
mlx_stream mlx_stream_new_device(mlx_device dev);
#endif
`)
	write("stream.cpp", `extern "C" mlx_stream mlx_stream_new(void) {
  return mlx_stream_new_();
}

extern "C" mlx_stream mlx_stream_new_thread_unsafe_device(mlx_device dev) {
  try {
    return mlx_stream_new_(new_stream(dev));
  } catch (std::exception& e) {
    mlx_error(e.what());
    return mlx_stream_new_();
  }
}

extern "C" mlx_stream mlx_stream_new_device(mlx_device dev) {
  return mlx_stream_new_thread_unsafe_device(dev);
}
`)
	abi, err := BuildABI(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]bool{
		"mlx_stream_new":                      false,
		"mlx_stream_new_thread_unsafe_device": true,
		"mlx_stream_new_device":               true,
	} {
		e, ok := abi.Functions[name]
		if !ok {
			t.Fatalf("%s missing from manifest", name)
		}
		if e.Fallible != want {
			t.Errorf("%s fallible = %v, want %v", name, e.Fallible, want)
		}
	}
}

func TestEntryClassification(t *testing.T) {
	src := abiSource{fn: Function{
		Name:   "mlx_fast_scaled_dot_product_attention",
		Return: "int",
		Parameters: []string{
			"mlx_array* res", "const mlx_array queries", "const mlx_array keys",
			"const mlx_array values", "float scale", "const char* mask_mode",
			"const mlx_array mask_arr", "const mlx_array sinks",
			"bool force_fused", "const mlx_stream s",
		},
	}}
	src.fallible = isStatusReturn(src.fn.Return)
	e := src.entry()
	if e.IntegerClassCount != 9 || e.FloatClassCount != 1 {
		t.Errorf("got %d/%d, want 9/1", e.IntegerClassCount, e.FloatClassCount)
	}
	if !e.Fallible {
		t.Error("int-returning function must be fallible")
	}
	data, err := ABIMap{SchemaVersion: ABISchemaVersion,
		Functions: map[string]ABIEntry{"a": e}}.JSON()
	if err != nil {
		t.Fatal(err)
	}
	var back struct {
		SchemaVersion int                     `json:"schema_version"`
		Functions     map[string]ABIEntryJSON `json:"functions"`
	}
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if back.SchemaVersion != ABISchemaVersion {
		t.Errorf("schema version = %d", back.SchemaVersion)
	}
	fall, present := back.Functions["a"]["fallible"]
	if !present {
		t.Error("fallible must be emitted even when false")
	}
	if v, ok := fall.(bool); !ok || !v {
		t.Errorf("fallible = %v, want true", fall)
	}
}

type ABIEntryJSON = map[string]interface{}
