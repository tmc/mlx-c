package generators

import (
	"fmt"
	"io"
	"strings"
)

// ClosureType defines a closure type to generate.
type ClosureType struct {
	Name      string   // mlx_closure, mlx_closure_value_and_grad, etc.
	ReturnCpp string   // C++ return type
	ParamsCpp []string // C++ parameter types
}

var closureTypes = []ClosureType{
	{
		Name:      "mlx_closure",
		ReturnCpp: "std::vector<mlx::core::array>",
		ParamsCpp: []string{"std::vector<mlx::core::array>"},
	},
	{
		Name:      "mlx_closure_kwargs",
		ReturnCpp: "std::vector<mlx::core::array>",
		ParamsCpp: []string{"std::vector<mlx::core::array>", "std::unordered_map<std::string, mlx::core::array>"},
	},
	{
		Name:      "mlx_closure_value_and_grad",
		ReturnCpp: "std::pair<std::vector<mlx::core::array>, std::vector<mlx::core::array>>",
		ParamsCpp: []string{"std::vector<mlx::core::array>"},
	},
	{
		Name:      "mlx_closure_custom",
		ReturnCpp: "std::vector<mlx::core::array>",
		ParamsCpp: []string{"std::vector<mlx::core::array>", "std::vector<mlx::core::array>", "std::vector<mlx::core::array>"},
	},
	{
		Name:      "mlx_closure_custom_jvp",
		ReturnCpp: "std::vector<mlx::core::array>",
		ParamsCpp: []string{"std::vector<mlx::core::array>", "std::vector<mlx::core::array>", "std::vector<int>"},
	},
	{
		Name:      "mlx_closure_custom_vmap",
		ReturnCpp: "std::pair<std::vector<mlx::core::array>, @std::vector<int>>",
		ParamsCpp: []string{"std::vector<mlx::core::array>", "std::vector<int>"},
	},
}

// closureTypeInfo contains type conversion info for closure parameters.
type closureTypeInfo struct {
	CArg           func(name string) string
	CArgUntyped    func(name string) string
	CToCpp         func(name string) string
	CppArg         func(name string) string
	CNew           func(name string) string
	Free           func(name string) string
	CAssignFromCpp func(dest, src string, returned bool) string
	CReturnArg     func(name string) string
}

var closureTypeMap = map[string]closureTypeInfo{
	"std::vector<mlx::core::array>": {
		CArg:        func(s string) string { return "const mlx_vector_array " + s },
		CArgUntyped: func(s string) string { return s },
		CToCpp:      func(s string) string { return "mlx_vector_array_get_(" + s + ")" },
		CppArg:      func(s string) string { return "const std::vector<mlx::core::array>& " + s },
		CNew:        func(s string) string { return "auto " + s + " = mlx_vector_array_new_()" },
		Free:        func(s string) string { return "mlx_vector_array_free(" + s + ")" },
		CAssignFromCpp: func(d, s string, returned bool) string {
			prefix := ""
			if returned {
				prefix = "*"
			}
			return "mlx_vector_array_set_(" + prefix + d + ", " + s + ")"
		},
		CReturnArg: func(s string) string { return "mlx_vector_array* " + s },
	},
	"std::vector<int>": {
		CArg: func(s string) string {
			if s == "" {
				return "const int*, size_t _num"
			}
			return "const int* " + s + ", size_t " + s + "_num"
		},
		CArgUntyped: func(s string) string { return s + ", " + s + "_num" },
		CToCpp:      func(s string) string { return "std::vector<int>(" + s + ", " + s + " + " + s + "_num)" },
		CppArg:      func(s string) string { return "const std::vector<int>& " + s },
		CNew: func(s string) string {
			return "const int* " + s + " = nullptr;\nsize_t " + s + "_num = 0"
		},
		Free: func(string) string { return "" },
		CAssignFromCpp: func(d, s string, returned bool) string {
			return d + " = " + s + ".data();\n" + d + "_num = " + s + ".size()"
		},
		CReturnArg: func(s string) string { return "mlx_vector_int* " + s },
	},
	"@std::vector<int>": {
		CArg:        func(s string) string { return "const mlx_vector_int " + s },
		CArgUntyped: func(s string) string { return s },
		CToCpp:      func(s string) string { return "mlx_vector_int_get_(" + s + ")" },
		CppArg:      func(s string) string { return "const std::vector<int>& " + s },
		CNew:        func(s string) string { return "auto " + s + " = mlx_vector_int_new_()" },
		Free:        func(s string) string { return "mlx_vector_int_free(" + s + ")" },
		CAssignFromCpp: func(d, s string, returned bool) string {
			prefix := ""
			if returned {
				prefix = "*"
			}
			return "mlx_vector_int_set_(" + prefix + d + ", " + s + ")"
		},
		CReturnArg: func(s string) string { return "mlx_vector_int* " + s },
	},
	"std::unordered_map<std::string, mlx::core::array>": {
		CArg:        func(s string) string { return "const mlx_map_string_to_array " + s },
		CArgUntyped: func(s string) string { return s },
		CToCpp:      func(s string) string { return "mlx_map_string_to_array_get_(" + s + ")" },
		CppArg:      func(s string) string { return "const std::unordered_map<std::string, mlx::core::array>& " + s },
		CNew:        func(s string) string { return "auto " + s + " = mlx_map_string_to_array_new_()" },
		Free:        func(s string) string { return "mlx_map_string_to_array_free(" + s + ")" },
		CAssignFromCpp: func(d, s string, returned bool) string {
			prefix := ""
			if returned {
				prefix = "*"
			}
			return "mlx_map_string_to_array_set_(" + prefix + d + ", " + s + ")"
		},
		CReturnArg: func(s string) string { return "mlx_map_string_to_array* " + s },
	},
	"std::pair<std::vector<mlx::core::array>, std::vector<mlx::core::array>>": {
		CArg:        func(s string) string { return "mlx_vector_array* " + s + "_0, mlx_vector_array* " + s + "_1" },
		CArgUntyped: func(s string) string { return "&" + s + "_0, &" + s + "_1" },
		CToCpp: func(s string) string {
			return "std::make_pair(mlx_vector_array_get_(" + s + "_0), mlx_vector_array_get_(" + s + "_1))"
		},
		CppArg: func(s string) string { return "" },
		CNew: func(s string) string {
			return "auto " + s + "_0 = mlx_vector_array_new_();\nauto " + s + "_1 = mlx_vector_array_new_();"
		},
		Free: func(s string) string {
			return "mlx_vector_array_free(" + s + "_0);\nmlx_vector_array_free(" + s + "_1);"
		},
		CAssignFromCpp: func(d, s string, returned bool) string {
			prefix := ""
			if returned {
				prefix = "*"
			}
			return "{ auto [tpl_0, tpl_1] = " + s + ";\nmlx_vector_array_set_(" + prefix + d + "_0,tpl_0);\nmlx_vector_array_set_(" + prefix + d + "_1,tpl_1);}"
		},
		CReturnArg: func(s string) string {
			if s == "" {
				return "mlx_vector_array*, mlx_vector_array*"
			}
			return "mlx_vector_array* " + s + "_0, mlx_vector_array* " + s + "_1"
		},
	},
	"std::pair<std::vector<mlx::core::array>, @std::vector<int>>": {
		CArg:        func(s string) string { return "mlx_vector_array* " + s + "_0, mlx_vector_int* " + s + "_1" },
		CArgUntyped: func(s string) string { return "&" + s + "_0, &" + s + "_1" },
		CToCpp: func(s string) string {
			return "std::make_pair(mlx_vector_array_get_(" + s + "_0), mlx_vector_int_get_(" + s + "_1))"
		},
		CppArg: func(s string) string { return "" },
		CNew: func(s string) string {
			return "auto " + s + "_0 = mlx_vector_array_new_();\nauto " + s + "_1 = mlx_vector_int_new_();"
		},
		Free: func(s string) string { return "mlx_vector_array_free(" + s + "_0);\nmlx_vector_int_free(" + s + "_1);" },
		CAssignFromCpp: func(d, s string, returned bool) string {
			prefix := ""
			if returned {
				prefix = "*"
			}
			return "{ auto [tpl_0, tpl_1] = " + s + ";\nmlx_vector_array_set_(" + prefix + d + "_0,tpl_0);\nmlx_vector_int_set_(" + prefix + d + "_1,tpl_1);}"
		},
		CReturnArg: func(s string) string {
			if s == "" {
				return "mlx_vector_array*, mlx_vector_int*"
			}
			return "mlx_vector_array* " + s + "_0, mlx_vector_int* " + s + "_1"
		},
	},
}

// GenerateClosure generates the closure.h, closure.cpp, or private/closure.h files.
func GenerateClosure(w io.Writer, mode string) {
	switch mode {
	case "header":
		generateClosureHeader(w)
	case "impl":
		generateClosureImpl(w)
	case "private":
		generateClosurePrivate(w)
	}
}

func generateClosureHeader(w io.Writer) {
	fmt.Fprintf(w, `/* Copyright © 2023-2024 Apple Inc.                   */
/*                                                    */
/* This file is auto-generated. Do not edit manually. */
/*                                                    */

#ifndef MLX_CLOSURE_H
#define MLX_CLOSURE_H

#include "mlx/c/array.h"
#include "mlx/c/map.h"
#include "mlx/c/optional.h"
#include "mlx/c/stream.h"
#include "mlx/c/vector.h"

#ifdef __cplusplus
extern "C" {
#endif

/**
 * \defgroup mlx_closure Closures
 * MLX closure objects.
 */
/**@{*/
`)

	for _, ct := range closureTypes {
		generateClosureDecl(w, ct)
		if ct.Name == "mlx_closure" {
			writeUnaryClosureHeader(w)
		}
	}

	fmt.Fprintf(w, `
/**@}*/

#ifdef __cplusplus
}
#endif

#endif
`)
}

func writeUnaryClosureHeader(w io.Writer) {
	fmt.Fprintf(w, `
mlx_closure mlx_closure_new_unary(int (*fun)(mlx_array*, const mlx_array));
`)
}

func generateClosureDecl(w io.Writer, ct ClosureType) {
	name := ct.Name
	retInfo := closureTypeMap[ct.ReturnCpp]

	// Build parameter type declarations
	var cArgsUnnamed []string
	for _, pt := range ct.ParamsCpp {
		info := closureTypeMap[pt]
		cArgsUnnamed = append(cArgsUnnamed, info.CArg(""))
	}

	var cArgs []string
	for i, pt := range ct.ParamsCpp {
		info := closureTypeMap[pt]
		suffix := ""
		if len(ct.ParamsCpp) > 1 {
			suffix = fmt.Sprintf("_%d", i)
		}
		cArgs = append(cArgs, info.CArg("input"+suffix))
	}

	rcArgsUnnamed := retInfo.CReturnArg("")
	rcArgs := retInfo.CReturnArg("res")

	fmt.Fprintf(w, `
typedef struct %s_ {
  void* ctx;
} %s;
%s %s_new(void);
int %s_free(%s cls);
%s %s_new_func(int (*fun)(%s, %s));
%s %s_new_func_payload(
    int (*fun)(%s, %s, void*),
    void* payload,
    void (*dtor)(void*));
int %s_set(%s *cls, const %s src);
int %s_apply(%s, %s cls, %s);
`,
		name, name,
		name, name,
		name, name,
		name, name, rcArgsUnnamed, strings.Join(cArgsUnnamed, ", "),
		name, name, rcArgsUnnamed, strings.Join(cArgsUnnamed, ", "),
		name, name, name,
		name, rcArgs, name, strings.Join(cArgs, ", "),
	)
}

func generateClosureImpl(w io.Writer) {
	fmt.Fprintf(w, `/* Copyright © 2023-2024 Apple Inc.                   */
/*                                                    */
/* This file is auto-generated. Do not edit manually. */
/*                                                    */

#include "mlx/c/closure.h"
#include "mlx/c/error.h"
#include "mlx/c/private/mlx.h"
`)

	for _, ct := range closureTypes {
		generateClosureImplCode(w, ct)
		if ct.Name == "mlx_closure" {
			writeUnaryClosureImpl(w)
		}
	}
}

func writeUnaryClosureImpl(w io.Writer) {
	fmt.Fprintf(w, `
extern "C" mlx_closure mlx_closure_new_unary(
    int (*fun)(mlx_array*, const mlx_array)) {
  try {
    auto cpp_closure = [fun](const std::vector<mlx::core::array>& cpp_input) {
      if (cpp_input.size() != 1) {
        throw std::runtime_error("closure: expected unary input");
      }
      auto input = mlx_array_new_(cpp_input[0]);
      auto res = mlx_array_new_();
      auto status = fun(&res, input);
      if(status) {
        mlx_array_free_(res);
        throw std::runtime_error("mlx_closure returned a non-zero value");
      }
      mlx_array_free(input);
      std::vector<mlx::core::array> cpp_res = {mlx_array_get_(res)};
      mlx_array_free(res);
      return cpp_res;
    };
    return mlx_closure_new_(cpp_closure);
  } catch (std::exception& e) {
    mlx_error(e.what());
    return mlx_closure_new_();
  }
}
`)
}

func generateClosureImplCode(w io.Writer, ct ClosureType) {
	name := ct.Name
	retInfo := closureTypeMap[ct.ReturnCpp]

	// Build parameter info
	var cArgsUntyped, cArgs, cppArgsTypeName, cArgsFree, cArgsCtx, cppArgsToCArgs []string
	for i, pt := range ct.ParamsCpp {
		info := closureTypeMap[pt]
		suffix := ""
		if len(ct.ParamsCpp) > 1 {
			suffix = fmt.Sprintf("_%d", i)
		}
		cArgsUntyped = append(cArgsUntyped, info.CArgUntyped("input"+suffix))
		cArgs = append(cArgs, info.CArg("input"+suffix))
		cppArgsTypeName = append(cppArgsTypeName, info.CppArg("cpp_input"+suffix))
		cArgsFree = append(cArgsFree, info.Free("input"+suffix)+";")
		cArgsCtx = append(cArgsCtx, info.CToCpp("input"+suffix))
		cppArgsToCArgs = append(cppArgsToCArgs, info.CNew("input"+suffix)+";")
		cppArgsToCArgs = append(cppArgsToCArgs, info.CAssignFromCpp("input"+suffix, "cpp_input"+suffix, false)+";")
	}

	rcArgsNew := retInfo.CNew("res") + ";"
	rcArgsFree := retInfo.Free("res") + ";"
	rcArgsToCpp := "auto cpp_res = " + retInfo.CToCpp("res") + ";"

	cArgsUntypedStr := strings.Join(cArgsUntyped, ", ")
	cArgsStr := strings.Join(cArgs, ", ")
	cppArgsTypeNameStr := strings.Join(cppArgsTypeName, ", ")
	cppArgsToCArgsStr := strings.Join(cppArgsToCArgs, "\n")
	cArgsFreeStr := strings.Join(cArgsFree, "\n")
	cArgsCtxStr := strings.Join(cArgsCtx, ", ")

	rcArgsUnnamed := retInfo.CReturnArg("")
	rcArgs := retInfo.CReturnArg("res")
	rcArgsUntyped := closureReturnArg(retInfo, "res")

	var cArgsUnnamed []string
	for _, pt := range ct.ParamsCpp {
		info := closureTypeMap[pt]
		cArgsUnnamed = append(cArgsUnnamed, info.CArg(""))
	}
	cArgsUnnamedStr := strings.Join(cArgsUnnamed, ", ")

	// Build C++ args string for function type
	var cppArgs []string
	for _, pt := range ct.ParamsCpp {
		info := closureTypeMap[pt]
		arg := info.CppArg("")
		if arg != "" {
			cppArgs = append(cppArgs, strings.TrimSpace(arg))
		}
	}
	cppArgsStr := strings.Join(cppArgs, ", ")
	rcppArg := strings.Replace(ct.ReturnCpp, "@", "", 1)

	assignClsToRcArgs := retInfo.CAssignFromCpp("res", name+"_get_(cls)("+cArgsCtxStr+")", true) + ";"

	fmt.Fprintf(w, `
extern "C" %s %s_new(void) {
  try {
    return %s_new_();
  } catch (std::exception& e) {
    mlx_error(e.what());
    return %s_new_();
  }
}

extern "C" int %s_set(%s *cls, const %s src) {
  try {
    %s_set_(*cls, %s_get_(src));
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
  return 0;
}

extern "C" int %s_free(%s cls) {
  try {
    %s_free_(cls);
    return 0;
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
}

extern "C" %s %s_new_func(int (*fun)(%s, %s)) {
  try {
    auto cpp_closure = [fun](%s) {
      %s
      %s
      auto status = fun(%s, %s);
      %s
      if(status) {
        %s
        throw std::runtime_error("%s returned a non-zero value");
      }
      %s
      %s
      return cpp_res;
    };
    return %s_new_(cpp_closure);
  } catch (std::exception& e) {
    mlx_error(e.what());
    return %s_new_();
  }
}

extern "C" %s %s_new_func_payload(
    int (*fun)(%s, %s, void*),
    void* payload,
    void (*dtor)(void*)) {
  try {
    std::shared_ptr<void> cpp_payload = nullptr;
    if (dtor) {
      cpp_payload = std::shared_ptr<void>(payload, dtor);
    } else {
      cpp_payload = std::shared_ptr<void>(payload, [](void*) {});
    }
    auto cpp_closure = [fun, cpp_payload](%s) {
      %s
      %s
      auto status = fun(%s, %s, cpp_payload.get());
      %s
      if(status) {
        %s
        throw std::runtime_error("%s returned a non-zero value");
      }
      %s
      %s
      return cpp_res;
    };
    return %s_new_(cpp_closure);
  } catch (std::exception& e) {
    mlx_error(e.what());
    return %s_new_();
  }
}

extern "C" int %s_apply(%s, %s cls, %s) {
  try {
    %s
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
  return 0;
}
`,
		// new
		name, name, name, name,
		// set
		name, name, name, name, name,
		// free
		name, name, name,
		// new_func
		name, name, rcArgsUnnamed, cArgsUnnamedStr,
		cppArgsTypeNameStr, cppArgsToCArgsStr, rcArgsNew, rcArgsUntyped, cArgsUntypedStr,
		cArgsFreeStr, rcArgsFree, name, rcArgsToCpp, rcArgsFree, name, name,
		// new_func_payload
		name, name, rcArgsUnnamed, cArgsUnnamedStr,
		cppArgsTypeNameStr, cppArgsToCArgsStr, rcArgsNew, rcArgsUntyped, cArgsUntypedStr,
		cArgsFreeStr, rcArgsFree, name, rcArgsToCpp, rcArgsFree, name, name,
		// apply
		name, rcArgs, name, cArgsStr, assignClsToRcArgs,
	)

	// For reference, show what C++ function type this closure represents
	_ = rcppArg
	_ = cppArgsStr
}

func closureReturnArg(info closureTypeInfo, name string) string {
	arg := info.CArgUntyped(name)
	if strings.HasPrefix(arg, "&") || strings.Contains(arg, ",") {
		return arg
	}
	return "&" + arg
}

func generateClosurePrivate(w io.Writer) {
	fmt.Fprintf(w, `/* Copyright © 2023-2024 Apple Inc.                   */
/*                                                    */
/* This file is auto-generated. Do not edit manually. */
/*                                                    */

#ifndef MLX_CLOSURE_PRIVATE_H
#define MLX_CLOSURE_PRIVATE_H

#include "mlx/c/closure.h"
#include "mlx/mlx.h"

`)

	for _, ct := range closureTypes {
		// Build C++ function type
		var cppArgs []string
		for _, pt := range ct.ParamsCpp {
			info := closureTypeMap[pt]
			arg := info.CppArg("")
			if arg != "" {
				cppArgs = append(cppArgs, strings.TrimSpace(arg))
			}
		}
		rcppArg := strings.Replace(ct.ReturnCpp, "@", "", 1)
		cppType := "std::function<" + rcppArg + "(" + strings.Join(cppArgs, ", ") + ")>"

		generateTypePrivate(w, ct.Name, cppType, true, false)
	}

	fmt.Fprintf(w, `
#endif
`)
}
