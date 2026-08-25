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

func TestABIFromFunctions(t *testing.T) {
	fns := []Function{{
		Name:   "mlx_fast_scaled_dot_product_attention",
		Return: "int",
		Parameters: []string{
			"mlx_array* res", "const mlx_array queries", "const mlx_array keys",
			"const mlx_array values", "float scale", "const char* mask_mode",
			"const mlx_array mask_arr", "const mlx_array sinks",
			"bool force_fused", "const mlx_stream s",
		},
	}}
	m := ABIFromFunctions(fns)
	e := m.Functions["mlx_fast_scaled_dot_product_attention"]
	if e.IntegerClassCount != 9 || e.FloatClassCount != 1 {
		t.Errorf("got %d/%d, want 9/1", e.IntegerClassCount, e.FloatClassCount)
	}
	data, err := m.JSON()
	if err != nil {
		t.Fatal(err)
	}
	var back ABIMap
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if back.SchemaVersion != ABISchemaVersion || len(back.Functions) != 1 {
		t.Errorf("roundtrip mismatch: %+v", back)
	}
}
