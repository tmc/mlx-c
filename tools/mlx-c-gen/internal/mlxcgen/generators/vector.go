package generators

import (
	"fmt"
	"io"
	"strings"
)

// VectorType defines a vector type to generate.
type VectorType struct {
	CppType    string
	CType      string // type in C function signatures
	ShortName  string // used in mlx_vector_<shortname>
	ReturnType string // return type for get function
	CToCpp     func(s string) string
	CAssign    func(dest, src string) string
}

var vectorTypes = []VectorType{
	{
		CppType:    "mlx::core::array",
		CType:      "const mlx_array",
		ShortName:  "array",
		ReturnType: "mlx_array*",
		CToCpp:     func(s string) string { return "mlx_array_get_(" + s + ")" },
		CAssign:    func(d, s string) string { return "mlx_array_set_(*" + d + ", " + s + ")" },
	},
	{
		CppType:    "std::vector<mlx::core::array>",
		CType:      "const mlx_vector_array",
		ShortName:  "vector_array",
		ReturnType: "mlx_vector_array*",
		CToCpp:     func(s string) string { return "mlx_vector_array_get_(" + s + ")" },
		CAssign:    func(d, s string) string { return "mlx_vector_array_set_(*" + d + ", " + s + ")" },
	},
	{
		CppType:    "int",
		CType:      "int",
		ShortName:  "int",
		ReturnType: "int*",
		CToCpp:     func(s string) string { return s },
		CAssign:    func(d, s string) string { return "*" + d + " = " + s },
	},
	{
		CppType:    "std::string",
		CType:      "const char*",
		ShortName:  "string",
		ReturnType: "char**",
		CToCpp:     func(s string) string { return s },
		CAssign:    func(d, s string) string { return "*" + d + " = " + s + ".data()" },
	},
}

// GenerateVector generates the vector.h, vector.cpp, or private/vector.h files.
func GenerateVector(w io.Writer, mode string) {
	switch mode {
	case "header":
		generateVectorHeader(w)
	case "impl":
		generateVectorImpl(w)
	case "private":
		generateVectorPrivate(w)
	}
}

func generateVectorHeader(w io.Writer) {
	fmt.Fprintf(w, `/* Copyright © 2023-2024 Apple Inc.                   */
/*                                                    */
/* This file is auto-generated. Do not edit manually. */
/*                                                    */

#ifndef MLX_VECTOR_H
#define MLX_VECTOR_H

#include "mlx/c/array.h"
#include "mlx/c/string.h"

#ifdef __cplusplus
extern "C" {
#endif

/**
 * \defgroup mlx_vector Vectors
 * MLX vector objects.
 */
/**@{*/
`)

	for _, vt := range vectorTypes {
		generateVectorDecl(w, vt)
	}

	fmt.Fprintf(w, `
/**@}*/

#ifdef __cplusplus
}
#endif

#endif
`)
}

func generateVectorDecl(w io.Writer, vt VectorType) {
	name := "mlx_vector_" + vt.ShortName
	ctype := vt.CType
	rctype := vt.ReturnType

	fmt.Fprintf(w, `
/**
 * A vector of %s.
 */
typedef struct %s_ {
  void* ctx;
} %s;
%s %s_new(void);
int %s_set(%s* vec, const %s src);
int %s_free(%s vec);
%s %s_new_data(%s* data, size_t size);
%s %s_new_value(%s val);
int %s_set_data(
    %s* vec,
    %s* data,
    size_t size);
int %s_set_value(%s* vec, %s val);
int %s_append_data(
    %s vec,
    %s* data,
    size_t size);
int %s_append_value(%s vec, %s val);
size_t %s_size(%s vec);
int %s_get(
    %s res,
    const %s vec,
    size_t idx);
`,
		vt.ShortName,
		name, name,
		name, name,
		name, name, name,
		name, name,
		name, name, ctype,
		name, name, ctype,
		name, name, ctype,
		name, name, ctype,
		name, name, ctype,
		name, name, ctype,
		name, name,
		name, rctype, name,
	)
}

func generateVectorImpl(w io.Writer) {
	fmt.Fprintf(w, `/* Copyright © 2023-2024 Apple Inc.                   */
/*                                                    */
/* This file is auto-generated. Do not edit manually. */
/*                                                    */

#include "mlx/c/error.h"
#include "mlx/c/private/mlx.h"
#include "mlx/c/vector.h"
`)

	for _, vt := range vectorTypes {
		generateVectorImplCode(w, vt)
	}
}

func generateVectorImplCode(w io.Writer, vt VectorType) {
	name := "mlx_vector_" + vt.ShortName
	ctype := vt.CType
	cpptype := vt.CppType
	rctype := vt.ReturnType
	ctoCpp := vt.CToCpp
	cAssign := vt.CAssign

	fmt.Fprintf(w, `
extern "C" %s %s_new(void) {
  try {
    return %s_new_({});
  } catch (std::exception& e) {
    mlx_error(e.what());
    return %s_new_();
  }
}

extern "C" int %s_set(
    %s* vec,
    const %s src) {
  try {
    %s_set_(*vec, %s_get_(src));
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
  return 0;
}

extern "C" int %s_free(%s vec) {
  try {
    %s_free_(vec);
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
  return 0;
}

extern "C" %s %s_new_data(
    %s* data,
    size_t size) {
  try {
    auto vec = %s_new();
    for (size_t i = 0; i < size; i++) {
      %s_get_(vec).push_back(%s);
    }
    return vec;
  } catch (std::exception& e) {
    mlx_error(e.what());
    return %s_new_();
  }
}

extern "C" %s %s_new_value(%s val) {
  try {
    return %s_new_({%s});
  } catch (std::exception& e) {
    mlx_error(e.what());
    return %s_new_();
  }
}

extern "C" int
%s_set_data(%s* vec_, %s* data, size_t size) {
  try {
    std::vector<%s> cpp_arrs;
    for (size_t i = 0; i < size; i++) {
      cpp_arrs.push_back(%s);
    }
    %s_set_(*vec_, cpp_arrs);
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
  return 0;
}

extern "C" int %s_set_value(%s* vec_, %s val) {
  try {
    %s_set_(*vec_, std::vector<%s>({%s}));
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
  return 0;
}

extern "C" int
%s_append_data(%s vec, %s* data, size_t size) {
  try {
    for (size_t i = 0; i < size; i++) {
      %s_get_(vec).push_back(%s);
    }
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
  return 0;
}

extern "C" int %s_append_value(
    %s vec,
    %s value) {
  try {
    %s_get_(vec).push_back(%s);
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
  return 0;
}

extern "C" int %s_get(
    %s res,
    const %s vec,
    size_t index) {
  try {
    %s;
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
  return 0;
}

extern "C" size_t %s_size(%s vec) {
  try {
    return %s_get_(vec).size();
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 0;
  }
}
`,
		// new
		name, name, name, name,
		// set
		name, name, name, name, name,
		// free
		name, name, name,
		// new_data
		name, name, ctype, name, name, ctoCpp("data[i]"), name,
		// new_value
		name, name, ctype, name, ctoCpp("val"), name,
		// set_data
		name, name, ctype, cpptype, ctoCpp("data[i]"), name,
		// set_value
		name, name, ctype, name, cpptype, ctoCpp("val"),
		// append_data
		name, name, ctype, name, ctoCpp("data[i]"),
		// append_value
		name, name, ctype, name, ctoCpp("value"),
		// get
		name, rctype, name, cAssign("res", name+"_get_(vec).at(index)"),
		// size
		name, name, name,
	)
}

func generateVectorPrivate(w io.Writer) {
	fmt.Fprintf(w, `/* Copyright © 2023-2024 Apple Inc.                   */
/*                                                    */
/* This file is auto-generated. Do not edit manually. */
/*                                                    */

#ifndef MLX_VECTOR_PRIVATE_H
#define MLX_VECTOR_PRIVATE_H

#include "mlx/c/vector.h"
#include "mlx/mlx.h"
`)

	for _, vt := range vectorTypes {
		generateTypePrivate(w, "mlx_vector_"+vt.ShortName, "std::vector<"+vt.CppType+">", true, false)
	}

	fmt.Fprintf(w, `
#endif
`)
}

// generateTypePrivate generates the private helper code for a type.
func generateTypePrivate(w io.Writer, ctype, cpptype string, ctor, noCopy bool) {
	ctorCopyCode := ""
	if !noCopy {
		ctorCopyCode = fmt.Sprintf(`
inline %s %s_new_(const %s& s) {
  return %s({new %s(s)});
}
`, ctype, ctype, cpptype, ctype, cpptype)
	}

	ctorCode := ""
	if ctor {
		ctorCode = fmt.Sprintf(`
inline %s %s_new_() {
  return %s({nullptr});
}
%s
inline %s %s_new_(%s&& s) {
  return %s({new %s(std::move(s))});
}
`, ctype, ctype, ctype, ctorCopyCode, ctype, ctype, cpptype, ctype, cpptype)
	}

	setCode := ""
	if noCopy {
		setCode = fmt.Sprintf(`
inline %s& %s_set_(%s& d, %s&& s) {
  if (d.ctx) {
    delete static_cast<%s*>(d.ctx);
  }
  d.ctx = new %s(std::move(s));
  return d;
}
`, ctype, ctype, ctype, cpptype, cpptype, cpptype)
	} else {
		setCode = fmt.Sprintf(`
inline %s& %s_set_(%s& d, const %s& s) {
  if (d.ctx) {
    *static_cast<%s*>(d.ctx) = s;
  } else {
    d.ctx = new %s(s);
  }
  return d;
}

inline %s& %s_set_(%s& d, %s&& s) {
  if (d.ctx) {
    *static_cast<%s*>(d.ctx) = std::move(s);
  } else {
    d.ctx = new %s(std::move(s));
  }
  return d;
}
`, ctype, ctype, ctype, cpptype, cpptype, cpptype, ctype, ctype, ctype, cpptype, cpptype, cpptype)
	}

	mainCode := fmt.Sprintf(`
%s

inline %s& %s_get_(%s d) {
  if (!d.ctx) {
    throw std::runtime_error("expected a non-empty %s");
  }
  return *static_cast<%s*>(d.ctx);
}

inline void %s_free_(%s d) {
  if (d.ctx) {
    delete static_cast<%s*>(d.ctx);
  }
}
`, setCode, cpptype, ctype, ctype, ctype, cpptype, ctype, ctype, cpptype)

	// Write constructor code first if applicable
	fmt.Fprint(w, ctorCode)
	fmt.Fprint(w, mainCode)
}

// GenerateTypePrivateHeader generates a standalone private header for a type.
func GenerateTypePrivateHeader(w io.Writer, shortName, ctype, cpptype string, noCopy bool) {
	upperName := strings.ToUpper(shortName)

	fmt.Fprintf(w, `/* Copyright © 2023-2024 Apple Inc.                   */
/*                                                    */
/* This file is auto-generated. Do not edit manually. */
/*                                                    */

#ifndef MLX_%s_PRIVATE_H
#define MLX_%s_PRIVATE_H

#include "mlx/c/%s.h"
#include "mlx/mlx.h"
`, upperName, upperName, shortName)

	generateTypePrivate(w, ctype, cpptype, true, noCopy)

	fmt.Fprintf(w, "#endif\n")
}
