package generators

import (
	"fmt"
	"io"
)

// MapType defines a map type to generate.
type MapType struct {
	KeyType struct {
		C              string
		Cpp            string
		Nick           string
		CReturn        string
		CToCpp         func(s string) string
		CAssignFromCpp func(d, s string) string
	}
	ValueType struct {
		C              string
		Cpp            string
		Nick           string
		CReturn        string
		CToCpp         func(s string) string
		CAssignFromCpp func(d, s string) string
	}
}

var mapTypes = []MapType{
	// string -> array
	{
		KeyType: struct {
			C              string
			Cpp            string
			Nick           string
			CReturn        string
			CToCpp         func(s string) string
			CAssignFromCpp func(d, s string) string
		}{
			C:              "const char*",
			Cpp:            "std::string",
			Nick:           "string",
			CReturn:        "const char**",
			CToCpp:         func(s string) string { return "std::string(" + s + ")" },
			CAssignFromCpp: func(d, s string) string { return "*" + d + " = " + s + ".data()" },
		},
		ValueType: struct {
			C              string
			Cpp            string
			Nick           string
			CReturn        string
			CToCpp         func(s string) string
			CAssignFromCpp func(d, s string) string
		}{
			C:              "const mlx_array",
			Cpp:            "mlx::core::array",
			Nick:           "array",
			CReturn:        "mlx_array*",
			CToCpp:         func(s string) string { return "mlx_array_get_(" + s + ")" },
			CAssignFromCpp: func(d, s string) string { return "mlx_array_set_(*" + d + ", " + s + ")" },
		},
	},
	// string -> string
	{
		KeyType: struct {
			C              string
			Cpp            string
			Nick           string
			CReturn        string
			CToCpp         func(s string) string
			CAssignFromCpp func(d, s string) string
		}{
			C:              "const char*",
			Cpp:            "std::string",
			Nick:           "string",
			CReturn:        "const char**",
			CToCpp:         func(s string) string { return "std::string(" + s + ")" },
			CAssignFromCpp: func(d, s string) string { return "*" + d + " = " + s + ".data()" },
		},
		ValueType: struct {
			C              string
			Cpp            string
			Nick           string
			CReturn        string
			CToCpp         func(s string) string
			CAssignFromCpp func(d, s string) string
		}{
			C:              "const char*",
			Cpp:            "std::string",
			Nick:           "string",
			CReturn:        "const char**",
			CToCpp:         func(s string) string { return "std::string(" + s + ")" },
			CAssignFromCpp: func(d, s string) string { return "*" + d + " = " + s + ".data()" },
		},
	},
}

// GenerateMap generates the map.h, map.cpp, or private/map.h files.
func GenerateMap(w io.Writer, mode string) {
	switch mode {
	case "header":
		generateMapHeader(w)
	case "impl":
		generateMapImpl(w)
	case "private":
		generateMapPrivate(w)
	}
}

func generateMapHeader(w io.Writer) {
	fmt.Fprintf(w, `/* Copyright © 2023-2024 Apple Inc.                   */
/*                                                    */
/* This file is auto-generated. Do not edit manually. */
/*                                                    */

#ifndef MLX_MAP_H
#define MLX_MAP_H

#include "mlx/c/array.h"
#include "mlx/c/string.h"

#ifdef __cplusplus
extern "C" {
#endif

/**
 * \defgroup mlx_map Maps
 * MLX map objects.
 */
/**@{*/
`)

	for _, mt := range mapTypes {
		generateMapDecl(w, mt)
	}

	fmt.Fprintf(w, `
/**@}*/

#ifdef __cplusplus
}
#endif

#endif
`)
}

func generateMapDecl(w io.Writer, mt MapType) {
	nick1 := mt.KeyType.Nick
	nick2 := mt.ValueType.Nick
	ctype1 := mt.KeyType.C
	ctype2 := mt.ValueType.C
	rctype1 := mt.KeyType.CReturn
	rctype2 := mt.ValueType.CReturn
	name := "mlx_map_" + nick1 + "_to_" + nick2

	fmt.Fprintf(w, `
/**
 * A %s-to-%s map
 */
typedef struct %s_ {
  void* ctx;
} %s;

/**
 * Returns a new empty %s-to-%s map.
 */
%s %s_new(void);
/**
 * Set map to provided src map.
 */
int %s_set(
    %s* map,
    const %s src);
/**
 * Free a %s-to-%s map.
 */
int %s_free(%s map);
/**
 * Insert a new `+"`value`"+` at the specified `+"`key`"+` in the map.
 */
int %s_insert(
    %s map,
    %s key,
    %s value);
/**
 * Returns the value indexed at the specified `+"`key`"+` in the map.
 */
int %s_get(
    %s value,
    const %s map,
    %s key);

/**
 * An iterator over a %s-to-%s map.
 */
typedef struct %s_iterator_ {
  void* ctx;
  void* map_ctx;
} %s_iterator;
/**
 * Returns a new iterator over the given map.
 */
%s_iterator %s_iterator_new(
    %s map);
/**
 * Free iterator.
 */
int %s_iterator_free(
    %s_iterator it);
/**
 * Increment iterator.
 */
int %s_iterator_next(
    %s key,
    %s value,
    %s_iterator it);
`,
		nick1, nick2,
		name, name,
		nick1, nick2,
		name, name,
		name, name, name,
		nick1, nick2,
		name, name,
		name, name, ctype1, ctype2,
		name, rctype2, name, ctype1,
		nick1, nick2,
		name, name,
		name, name, name,
		name, name,
		name, rctype1, rctype2, name,
	)
}

func generateMapImpl(w io.Writer) {
	fmt.Fprintf(w, `/* Copyright © 2023-2024 Apple Inc.                   */
/*                                                    */
/* This file is auto-generated. Do not edit manually. */
/*                                                    */

#include "mlx/c/error.h"
#include "mlx/c/map.h"
#include "mlx/c/private/mlx.h"
`)

	for _, mt := range mapTypes {
		generateMapImplCode(w, mt)
	}
}

func generateMapImplCode(w io.Writer, mt MapType) {
	nick1 := mt.KeyType.Nick
	nick2 := mt.ValueType.Nick
	ctype1 := mt.KeyType.C
	ctype2 := mt.ValueType.C
	cpptype1 := mt.KeyType.Cpp
	cpptype2 := mt.ValueType.Cpp
	rctype1 := mt.KeyType.CReturn
	rctype2 := mt.ValueType.CReturn
	ctype1ToCpp := mt.KeyType.CToCpp
	ctype2ToCpp := mt.ValueType.CToCpp
	ctype1AssignFromCpp := mt.KeyType.CAssignFromCpp
	ctype2AssignFromCpp := mt.ValueType.CAssignFromCpp
	name := "mlx_map_" + nick1 + "_to_" + nick2

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
    %s* map,
    const %s src) {
  try {
    %s_set_(*map, %s_get_(src));
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
  return 0;
}

extern "C" int %s_free(%s map) {
  try {
    %s_free_(map);
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
  return 0;
}

extern "C" int %s_insert(
    %s map,
    %s key,
    %s value) {
  try {
    %s_get_(map).insert_or_assign(
        %s, %s);
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
  return 0;
}

extern "C" int %s_get(
    %s value,
    const %s map,
    %s key) {
  try {
    auto search = %s_get_(map).find(%s);
    if (search == %s_get_(map).end()) {
      return 2;
    } else {
      %s;
      return 0;
    }
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
  return 0;
}

extern "C" %s_iterator
%s_iterator_new(%s map) {
  auto& cpp_map = %s_get_(map);
  try {
    return %s_iterator{
        new std::unordered_map<%s, %s>::iterator(cpp_map.begin()),
        &cpp_map};
  } catch (std::exception& e) {
    mlx_error(e.what());
    return %s_iterator{0};
  }
}

extern "C" int %s_iterator_next(
    %s key,
    %s value,
    %s_iterator it) {
  try {
    if (%s_iterator_get_(it) ==
        %s_iterator_get_map_(it).end()) {
      return 2;
    } else {
      %s;
      %s;
      %s_iterator_get_(it)++;
      return 0;
    }
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
}

extern "C" int %s_iterator_free(
    %s_iterator it) {
  try {
    %s_iterator_free_(it);
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
		// insert
		name, name, ctype1, ctype2, name, ctype1ToCpp("key"), ctype2ToCpp("value"),
		// get
		name, rctype2, name, ctype1, name, ctype1ToCpp("key"), name, ctype2AssignFromCpp("value", "search->second"),
		// iterator_new
		name, name, name, name, name, cpptype1, cpptype2, name,
		// iterator_next
		name, rctype1, rctype2, name, name, name,
		ctype1AssignFromCpp("key", name+"_iterator_get_(it)->first"),
		ctype2AssignFromCpp("value", name+"_iterator_get_(it)->second"),
		name,
		// iterator_free
		name, name, name,
	)
}

func generateMapPrivate(w io.Writer) {
	fmt.Fprintf(w, `/* Copyright © 2023-2024 Apple Inc.                   */
/*                                                    */
/* This file is auto-generated. Do not edit manually. */
/*                                                    */

#ifndef MLX_MAP_PRIVATE_H
#define MLX_MAP_PRIVATE_H

#include "mlx/c/map.h"
#include "mlx/mlx.h"
`)

	for _, mt := range mapTypes {
		ctype := "mlx_map_" + mt.KeyType.Nick + "_to_" + mt.ValueType.Nick
		cpptype := "std::unordered_map<" + mt.KeyType.Cpp + ", " + mt.ValueType.Cpp + ">"

		generateTypePrivate(w, ctype, cpptype, true, false)

		// Also generate iterator private helpers
		iterCtype := ctype + "_iterator"
		iterCpptype := cpptype + "::iterator"
		generateTypePrivate(w, iterCtype, iterCpptype, false, false)

		// Add get_map_ helper
		fmt.Fprintf(w, `
inline %s& %s_get_map_(%s d) {
  return *static_cast<%s*>(d.map_ctx);
}
`, cpptype, iterCtype, iterCtype, cpptype)
	}

	fmt.Fprintf(w, `
#endif
`)
}
