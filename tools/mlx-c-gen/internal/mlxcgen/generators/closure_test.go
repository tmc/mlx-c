package generators

import (
	"bytes"
	"strings"
	"testing"
)

func TestGenerateClosurePreservesCallbackABI(t *testing.T) {
	var header bytes.Buffer
	GenerateClosure(&header, "header")
	headerText := header.String()
	for _, want := range []string{
		"mlx_closure mlx_closure_new_unary",
		"int (*fun)(mlx_vector_array*, mlx_vector_array*, const mlx_vector_array )",
		"int (*fun)(mlx_vector_array* , const mlx_vector_array , const mlx_vector_array , const int*, size_t _num)",
		"int (*fun)(mlx_vector_array*, mlx_vector_int*, const mlx_vector_array , const int*, size_t _num)",
		"int mlx_closure_custom_vmap_apply(mlx_vector_array* res_0, mlx_vector_int* res_1, mlx_closure_custom_vmap cls, const mlx_vector_array input_0, const int* input_1, size_t input_1_num)",
	} {
		if !strings.Contains(headerText, want) {
			t.Fatalf("header missing %q\n%s", want, headerText)
		}
	}

	var impl bytes.Buffer
	GenerateClosure(&impl, "impl")
	implText := impl.String()
	for _, want := range []string{
		"auto status = fun(&res, input);",
		"auto status = fun(&res, input_0, input_1, input_2, input_2_num);",
		"std::vector<int>(input_1, input_1 + input_1_num)",
	} {
		if !strings.Contains(implText, want) {
			t.Fatalf("implementation missing %q\n%s", want, implText)
		}
	}
}
