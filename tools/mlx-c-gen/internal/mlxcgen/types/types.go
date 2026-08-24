// Package types provides C++ to C type mappings for MLX binding generation.
package types

import (
	"fmt"
	"strings"
)

// TypeMapping defines how to convert between C++ and C types.
type TypeMapping struct {
	CType   string   // C type name (e.g., "mlx_array")
	CppType string   // C++ type name (e.g., "mlx::core::array")
	Alt     []string // Alternative type names that should map to this type

	// Conversion functions - these return code snippets
	CToCpp         func(name string) string // Convert C to C++ (e.g., "mlx_array_get_(x)")
	CppToC         func(name string) string // Convert C++ to C (e.g., "mlx_array_new_(x)")
	CArg           func(name string) string // C function argument (e.g., "const mlx_array x")
	CArgUntyped    func(name string) string // Just the variable name(s)
	CReturnArg     func(name string) string // C return argument (e.g., "mlx_array* res")
	CNew           func(name string) string // Declare new C variable
	Free           func(name string) string // Free C variable
	CAssignFromCpp func(dest, src string, returned bool) string
	CppArg         func(name string) string // C++ function argument
}

// Registry holds all type mappings.
type Registry struct {
	byC   map[string]*TypeMapping
	byCpp map[string]*TypeMapping
	byAlt map[string]*TypeMapping
	all   []*TypeMapping
}

// NewRegistry creates a new type registry with all built-in types.
func NewRegistry() *Registry {
	r := &Registry{
		byC:   make(map[string]*TypeMapping),
		byCpp: make(map[string]*TypeMapping),
		byAlt: make(map[string]*TypeMapping),
	}
	r.registerAll()
	return r
}

// Register adds a type mapping to the registry.
func (r *Registry) Register(t *TypeMapping) {
	r.all = append(r.all, t)
	if t.CType != "" {
		r.byC[t.CType] = t
	}
	if t.CppType != "" {
		r.byCpp[t.CppType] = t
	}
	for _, alt := range t.Alt {
		if alt != "" {
			r.byAlt[alt] = t
		}
	}
}

// FindByCpp looks up a type by its C++ name.
func (r *Registry) FindByCpp(cppType string) *TypeMapping {
	if t, ok := r.byCpp[cppType]; ok {
		return t
	}
	if t, ok := r.byAlt[cppType]; ok {
		return t
	}
	return nil
}

// FindByC looks up a type by its C name.
func (r *Registry) FindByC(cType string) *TypeMapping {
	return r.byC[cType]
}

func (r *Registry) registerAll() {
	// MLX handle types
	for _, t := range []struct {
		cType   string
		cppType string
		alt     []string
	}{
		{"mlx_array", "mlx::core::array", []string{"array"}},
		{"mlx_vector_int", "@std::vector<int>", []string{"@std::vector<int>"}},
		{"mlx_vector_string", "std::vector<std::string>", []string{"std::vector<std::string>"}},
		{"mlx_vector_array", "std::vector<mlx::core::array>", []string{"std::vector<array>"}},
		{"mlx_stream", "mlx::core::Stream", []string{"StreamOrDevice"}},
		{"mlx_map_string_to_array", "std::unordered_map<std::string, mlx::core::array>", []string{"std::unordered_map<std::string, array>"}},
		{"mlx_map_string_to_string", "std::unordered_map<std::string, std::string>", nil},
		{"mlx_distributed_group", "mlx::core::distributed::Group", []string{"Group"}},
		{"mlx_node_namer", "mlx::core::NodeNamer", []string{"NodeNamer"}},
		{"mlx_closure", "std::function<std::vector<array>(std::vector<array>)>", []string{
			"std::function<std::vector<array>(const std::vector<array>&)>",
			"std::function<std::vector<mlx::core::array>(std::vector<mlx::core::array>)>",
			"std::function<std::vector<mlx::core::array>(const std::vector<mlx::core::array>&)>",
		}},
		{"mlx_closure_value_and_grad", "std::function<std::pair<std::vector<array>, std::vector<array>>(const std::vector<array>&)>", []string{
			"ValueAndGradFn",
			"std::function<std::pair<std::vector<array>, std::vector<array>>(std::vector<array>)>",
			"std::function<std::pair<std::vector<mlx::core::array>, std::vector<mlx::core::array>>(const std::vector<mlx::core::array>&)>",
		}},
		{"mlx_closure_custom", "std::function<std::vector<mlx::core::array>(std::vector<mlx::core::array>,std::vector<mlx::core::array>,std::vector<mlx::core::array>)>", []string{
			"std::function<std::vector<array>(std::vector<array>,std::vector<array>,std::vector<array>)>",
			"std::function<std::vector<array>(const std::vector<array>&, const std::vector<array>&, const std::vector<array>&)>",
		}},
		{"mlx_closure_custom_jvp", "std::function<std::vector<mlx::core::array>(std::vector<mlx::core::array>,std::vector<mlx::core::array>,std::vector<int>)>", []string{
			"std::function<std::vector<array>(std::vector<array>,std::vector<array>,std::vector<int>)>",
			"std::function<std::vector<array>(const std::vector<array>&, const std::vector<array>&, const std::vector<int>&)>",
		}},
		{"mlx_closure_custom_vmap", "std::function<std::pair<std::vector<mlx::core::array>, std::vector<int>>(std::vector<mlx::core::array>,std::vector<int>)>", []string{
			"std::function<std::pair<std::vector<array>, std::vector<int>>(std::vector<array>,std::vector<int>)>",
			"std::function<std::pair<std::vector<array>, std::vector<int>>(const std::vector<array>&, const std::vector<int>&)>",
		}},
	} {
		cType := t.cType
		r.Register(&TypeMapping{
			CType:   cType,
			CppType: t.cppType,
			Alt:     t.alt,
			Free:    func(s string) string { return cType + "_free(" + s + ")" },
			CppToC:  func(s string) string { return cType + "_new_(" + s + ")" },
			CToCpp:  func(s string) string { return cType + "_get_(" + s + ")" },
			CAssignFromCpp: func(dest, src string, returned bool) string {
				prefix := ""
				if returned {
					prefix = "*"
				}
				return cType + "_set_(" + prefix + dest + ", " + src + ")"
			},
			CArg: func(s string) string {
				if s == "" {
					return "const " + cType
				}
				return "const " + cType + " " + s
			},
			CArgUntyped: func(s string) string { return s },
			CReturnArg: func(s string) string {
				if s == "" {
					return cType + "*"
				}
				return cType + "* " + s
			},
			CNew: func(s string) string { return "auto " + s + " = " + cType + "_new_()" },
			CppArg: func(s string) string {
				cpp := strings.Replace(t.cppType, "@", "", 1)
				if s == "" {
					return "const " + cpp + "&"
				}
				return "const " + cpp + "& " + s
			},
		})
	}

	// Small vector types (Shape, Strides)
	r.registerSmallVectorType("int", "mlx::core::Shape", "Shape")
	r.registerSmallVectorType("int64_t", "mlx::core::Strides", "Strides")

	// Raw vector types
	r.registerRawVectorType("int", nil)
	r.registerRawVectorType("size_t", nil)
	r.registerRawVectorType("uint64_t", nil)

	// Optional raw vector types
	r.registerOptionalRawVectorType("int")

	// Return tuple types
	r.registerReturnTupleType([]string{"mlx::core::array", "mlx::core::array"}, nil)
	r.registerReturnTupleType([]string{"mlx::core::array", "mlx::core::array", "mlx::core::array"}, nil)
	r.registerReturnTupleType([]string{"std::vector<mlx::core::array>", "std::vector<mlx::core::array>"}, nil)
	r.registerReturnTupleType([]string{"std::vector<mlx::core::array>", "@std::vector<int>"}, nil)
	r.registerReturnTupleType(
		[]string{"std::unordered_map<std::string, mlx::core::array>", "std::unordered_map<std::string, std::string>"},
		[]string{"SafetensorsLoad"},
	)

	// void type
	r.Register(&TypeMapping{
		CppType:    "void",
		CReturnArg: func(s string) string { return "" },
		CAssignFromCpp: func(dest, src string, returned bool) string {
			return src
		},
	})

	// Dtype
	r.Register(&TypeMapping{
		CType:   "mlx_dtype",
		CppType: "mlx::core::Dtype",
		Alt:     []string{"Dtype"},
		CToCpp:  func(s string) string { return "mlx_dtype_to_cpp(" + s + ")" },
		CArg: func(s string) string {
			if s == "" {
				return "mlx_dtype"
			}
			return "mlx_dtype " + s
		},
		CArgUntyped: func(s string) string { return s },
		CReturnArg: func(s string) string {
			if s == "" {
				return "mlx_dtype*"
			}
			return "mlx_dtype* " + s
		},
		CNew: func(s string) string { return "mlx_dtype " + s },
		Free: func(s string) string { return "" },
		CAssignFromCpp: func(dest, src string, returned bool) string {
			return dest + " = mlx_dtype_to_c((int)((" + src + ").val))"
		},
	})

	// CompileMode
	r.Register(&TypeMapping{
		CppType: "mlx::core::CompileMode",
		Alt:     []string{"CompileMode"},
		CToCpp:  func(s string) string { return "mlx_compile_mode_to_cpp(" + s + ")" },
		CArg: func(s string) string {
			if s == "" {
				return "mlx_compile_mode"
			}
			return "mlx_compile_mode " + s
		},
		CArgUntyped: func(s string) string { return s },
		CReturnArg: func(s string) string {
			if s == "" {
				return "mlx_compile_mode*"
			}
			return "mlx_compile_mode* " + s
		},
		CNew: func(s string) string { return "mlx_dtype " + s },
		Free: func(s string) string { return "" },
		CAssignFromCpp: func(dest, src string, returned bool) string {
			return dest + " = mlx_compile_mode_to_c((int)((" + src + ").val))"
		},
	})

	// FFTNorm
	r.Register(&TypeMapping{
		CType:   "mlx_fft_norm",
		CppType: "mlx::core::fft::FFTNorm",
		Alt:     []string{"FFTNorm"},
		CToCpp:  func(s string) string { return "mlx_fft_norm_to_cpp(" + s + ")" },
		CArg: func(s string) string {
			if s == "" {
				return "mlx_fft_norm"
			}
			return "mlx_fft_norm " + s
		},
		CArgUntyped: func(s string) string { return s },
		CReturnArg: func(s string) string {
			if s == "" {
				return "mlx_fft_norm*"
			}
			return "mlx_fft_norm* " + s
		},
		CNew: func(s string) string { return "mlx_fft_norm " + s },
		Free: func(s string) string { return "" },
		CAssignFromCpp: func(dest, src string, returned bool) string {
			return dest + " = mlx_fft_norm_to_c(" + src + ")"
		},
	})

	// std::string
	r.Register(&TypeMapping{
		CppType: "std::string",
		Alt:     []string{"std::string"},
		CToCpp:  func(s string) string { return "std::string(" + s + ")" },
		CArg: func(s string) string {
			if s == "" {
				return "const char*"
			}
			return "const char* " + s
		},
		CArgUntyped: func(s string) string { return s },
		CReturnArg: func(s string) string {
			if s == "" {
				return "const char**"
			}
			return "const char** " + s
		},
		CAssignFromCpp: func(dest, src string, returned bool) string {
			prefix := ""
			if returned {
				prefix = "*"
			}
			return prefix + dest + " = " + src + ".c_str()"
		},
	})

	// IO types
	r.Register(&TypeMapping{
		CppType: "std::shared_ptr<io::Reader>",
		CToCpp:  func(s string) string { return "mlx_io_reader_get_(" + s + ")" },
		CArg: func(s string) string {
			if s == "" {
				return "mlx_io_reader"
			}
			return "mlx_io_reader " + s
		},
		CArgUntyped: func(s string) string { return s },
	})

	r.Register(&TypeMapping{
		CppType: "std::shared_ptr<io::Writer>",
		CToCpp:  func(s string) string { return "mlx_io_writer_get_(" + s + ")" },
		CArg: func(s string) string {
			if s == "" {
				return "mlx_io_writer"
			}
			return "mlx_io_writer " + s
		},
		CArgUntyped: func(s string) string { return s },
	})

	// Compile cache handle (MLX >= 0.32.1: caches are per-OS-thread;
	// erase/clear act only on the passing handle's view).
	r.Register(&TypeMapping{
		CppType: "CompileCacheWeakPtr",
		Alt: []string{
			"const CompileCacheWeakPtr&",
			"const CompileCacheWeakPtr &",
			"std::weak_ptr<mlx::core::CompileCache>",
		},
		CToCpp:  func(s string) string { return "mlx_compile_cache_weakptr_get_(" + s + ")" },
		CArg: func(s string) string {
			if s == "" {
				return "mlx_compile_cache"
			}
			return "mlx_compile_cache " + s
		},
		CArgUntyped: func(s string) string { return s },
	})

	r.Register(&TypeMapping{
		CppType: "std::ostream",
		CToCpp: func(s string) string {
			return "CFileOutputStream::as_lvalue(CFileOutputStream(" + s + "))"
		},
		CArg: func(s string) string {
			if s == "" {
				return "FILE*"
			}
			return "FILE* " + s
		},
		CArgUntyped: func(s string) string { return s },
	})

	// Primitive types
	for _, ctype := range []string{"int", "size_t", "float", "double", "bool", "uint64_t", "uintptr_t"} {
		ct := ctype // capture for closures
		alt := []string(nil)
		if ct == "uintptr_t" {
			alt = []string{"std::uintptr_t"}
		}
		r.Register(&TypeMapping{
			CType:       ct,
			CppType:     ct,
			Alt:         alt,
			Free:        func(s string) string { return "" },
			CppToC:      func(s string) string { return s },
			CToCpp:      func(s string) string { return s },
			CArg:        func(s string) string { return ct + " " + s },
			CArgUntyped: func(s string) string { return s },
			CppArg:      func(s string) string { return ct + " " + s },
			CReturnArg:  func(s string) string { return ct + "* " + s },
			CAssignFromCpp: func(dest, src string, returned bool) string {
				return "*" + dest + " = " + src
			},
		})
	}

	// Optional primitive types. std::optional<bool> backs upstream's
	// astype(a, dtype, copy, s) overload (bound as mlx_astype_copy).
	for _, cppType := range []string{"float", "int", "bool", "mlx::core::Dtype"} {
		r.registerOptionalPrimitiveType(cppType)
	}

	// std::pair<int, int>
	r.Register(&TypeMapping{
		CppType: "std::pair<int, int>",
		Alt:     []string{"std::pair<int, int>"},
		CToCpp:  func(s string) string { return "std::make_pair(" + s + "_0, " + s + "_1)" },
		CArg: func(s string) string {
			if s == "" {
				return "int , int"
			}
			return "int " + s + "_0, int " + s + "_1"
		},
		CArgUntyped: func(s string) string { return s + "_0, " + s + "_1" },
		CReturnArg: func(s string) string {
			if s == "" {
				return "int* , int*"
			}
			return "int* " + s + "_0, int* " + s + "_1"
		},
		CAssignFromCpp: func(dest, src string, returned bool) string {
			return "std::tie(" + dest + "_0, " + dest + "_1) = " + src
		},
	})

	// std::tuple<int, int, int>
	r.Register(&TypeMapping{
		CppType: "std::tuple<int, int, int>",
		Alt:     []string{"std::tuple<int, int, int>"},
		CToCpp:  func(s string) string { return "std::make_tuple(" + s + "_0, " + s + "_1," + s + "_2)" },
		CArg: func(s string) string {
			if s == "" {
				return "int , int , int"
			}
			return "int " + s + "_0, int " + s + "_1, int " + s + "_2"
		},
		CArgUntyped: func(s string) string { return s + "_0, " + s + "_1, " + s + "_2" },
		CReturnArg: func(s string) string {
			if s == "" {
				return "int* , int* , int"
			}
			return "int* " + s + "_0, int* " + s + "_1, int " + s + "_2"
		},
		CAssignFromCpp: func(dest, src string, returned bool) string {
			return "std::tie(" + dest + "_0, " + dest + "_1, " + dest + "_2) = " + src
		},
	})

	// Optional handle types
	r.registerOptionalHandleType("mlx::core::array")
	r.registerOptionalHandleType("mlx::core::distributed::Group")
	// Closure optional types - use normalized forms that match parser output
	r.registerOptionalHandleType("std::function<std::vector<array>(const std::vector<array>&, const std::vector<array>&, const std::vector<array>&)>")
	r.registerOptionalHandleType("std::function<std::vector<array>(const std::vector<array>&, const std::vector<array>&, const std::vector<int>&)>")
	r.registerOptionalHandleType("std::function<std::pair<std::vector<array>, std::vector<int>>(const std::vector<array>&, const std::vector<int>&)>")
}

func (r *Registry) registerRawVectorType(cppType string, alt []string) {
	fullCppType := "std::vector<" + cppType + ">"
	r.Register(&TypeMapping{
		CppType: fullCppType,
		Alt:     alt,
		Free:    func(s string) string { return "" },
		CToCpp: func(s string) string {
			return "std::vector<" + cppType + ">(" + s + ", " + s + " + " + s + "_num)"
		},
		CAssignFromCpp: func(dest, src string, returned bool) string {
			return dest + " = " + src + ".data(); " + dest + "_num = " + src + ".size()"
		},
		CArg: func(s string) string {
			if s == "" {
				return "const " + cppType + "* , size_t"
			}
			return "const " + cppType + "* " + s + ", size_t " + s + "_num"
		},
		CArgUntyped: func(s string) string { return s + ", " + s + "_num" },
		CNew: func(s string) string {
			return "const " + cppType + "* " + s + "= nullptr;  size_t " + s + "_num = 0"
		},
		CppArg: func(s string) string {
			if s == "" {
				return "const std::vector<" + cppType + ">&"
			}
			return "const std::vector<" + cppType + ">& " + s
		},
	})
}

func (r *Registry) registerSmallVectorType(elemType, cppType, alt string) {
	r.Register(&TypeMapping{
		CppType: cppType,
		Alt:     []string{alt},
		Free:    func(s string) string { return "" },
		CToCpp: func(s string) string {
			return cppType + "(" + s + ", " + s + " + " + s + "_num)"
		},
		CAssignFromCpp: func(dest, src string, returned bool) string {
			return dest + " = " + src + ".data(); " + dest + "_num = " + src + ".size()"
		},
		CArg: func(s string) string {
			if s == "" {
				return "const " + elemType + "* , size_t"
			}
			return "const " + elemType + "* " + s + ", size_t " + s + "_num"
		},
		CArgUntyped: func(s string) string { return s + ", " + s + "_num" },
		CNew: func(s string) string {
			return "const " + elemType + "* " + s + "= nullptr;  size_t " + s + "_num = 0"
		},
		CppArg: func(s string) string {
			if s == "" {
				return "const " + cppType + "&"
			}
			return "const " + cppType + "& " + s
		},
	})
}

func (r *Registry) registerOptionalRawVectorType(cppType string) {
	fullCppType := "std::optional<std::vector<" + cppType + ">>"

	r.Register(&TypeMapping{
		CppType: fullCppType,
		Free:    func(s string) string { return "" },
		CToCpp: func(s string) string {
			return "(" + s + "? std::make_optional(std::vector<" + cppType + ">(" +
				s + ", " + s + " + " + s + "_num)) : std::nullopt)"
		},
		CAssignFromCpp: func(dest, src string, returned bool) string {
			return "if(" + src + ".has_value()) {" +
				dest + " = " + src + ".data();" +
				dest + "_num = " + src + ".size();" +
				"} else {" +
				dest + " = nullptr;" +
				dest + "_num = 0;" +
				"}"
		},
		CArg: func(s string) string {
			if s == "" {
				return "const " + cppType + "* /* may be null */, size_t"
			}
			return "const " + cppType + "*" + s + "/* may be null */, size_t " + s + "_num"
		},
		CArgUntyped: func(s string) string { return s + ", " + s + "_num" },
	})
}

func (r *Registry) registerReturnTupleType(cppTypes []string, alts []string) {
	n := len(cppTypes)
	cTypes := make([]string, n)
	altTypes := make([]string, n)
	ctoCpps := make([]func(string) string, n)

	for i, cppType := range cppTypes {
		typedef := r.FindByCpp(cppType)
		if typedef == nil {
			panic("unknown type: " + cppType)
		}
		cTypes[i] = typedef.CType
		if len(typedef.Alt) > 0 {
			altTypes[i] = typedef.Alt[0]
		}
		ctoCpps[i] = typedef.CToCpp
	}

	cppMakeTuple := "std::make_pair"
	cppTuple := "std::pair"
	if n > 2 {
		cppMakeTuple = "std::tie"
		cppTuple = "std::tuple"
	}

	fullCppType := cppTuple + "<" + strings.Join(cppTypes, ", ") + ">"
	fullAltType := cppTuple + "<" + strings.Join(altTypes, ", ") + ">"
	allAlts := append([]string{fullAltType}, alts...)

	r.Register(&TypeMapping{
		CppType: fullCppType,
		Alt:     allAlts,
		CToCpp: func(s string) string {
			parts := make([]string, n)
			for i := 0; i < n; i++ {
				parts[i] = ctoCpps[i](s + "_" + fmt.Sprintf("%d", i))
			}
			return cppMakeTuple + "(" + strings.Join(parts, ", ") + ")"
		},
		CReturnArg: func(s string) string {
			parts := make([]string, n)
			for i := 0; i < n; i++ {
				if s == "" {
					parts[i] = cTypes[i] + "*"
				} else {
					parts[i] = cTypes[i] + "* " + s + "_" + fmt.Sprintf("%d", i)
				}
			}
			return strings.Join(parts, ", ")
		},
		CNew: func(s string) string {
			parts := make([]string, n)
			for i := 0; i < n; i++ {
				parts[i] = "auto " + s + "_" + fmt.Sprintf("%d", i) + " = " + cTypes[i] + "_new_();"
			}
			return strings.Join(parts, "\n")
		},
		Free: func(s string) string {
			parts := make([]string, n)
			for i := 0; i < n; i++ {
				parts[i] = cTypes[i] + "_free(" + s + "_" + fmt.Sprintf("%d", i) + ");"
			}
			return strings.Join(parts, "\n")
		},
		CAssignFromCpp: func(dest, src string, returned bool) string {
			prefix := ""
			if returned {
				prefix = "*"
			}
			tplVars := make([]string, n)
			assignments := make([]string, n)
			for i := 0; i < n; i++ {
				tplVars[i] = "tpl_" + fmt.Sprintf("%d", i)
				assignments[i] = cTypes[i] + "_set_(" + prefix + dest + "_" + fmt.Sprintf("%d", i) + "," + tplVars[i] + ");"
			}
			return "{ auto [" + strings.Join(tplVars, ", ") + "] = " + src + ";" +
				strings.Join(assignments, "\n") + "}"
		},
	})
}

func (r *Registry) registerOptionalPrimitiveType(cppType string) {
	typedef := r.FindByCpp(cppType)
	if typedef == nil {
		return
	}
	cType := typedef.CType
	altType := ""
	if len(typedef.Alt) > 0 {
		altType = typedef.Alt[0]
	}

	optCType := "mlx_optional_" + strings.Replace(cType, "mlx_", "", 1)
	optCppType := "std::optional<" + cppType + ">"

	ctoCpp := typedef.CToCpp
	if ctoCpp == nil {
		ctoCpp = func(s string) string { return s }
	}
	cppToC := typedef.CppToC
	if cppToC == nil {
		cppToC = func(s string) string { return s }
	}

	var optAlt []string
	if altType != "" {
		optAlt = []string{"std::optional<" + altType + ">"}
	}

	r.Register(&TypeMapping{
		CType:   optCType,
		CppType: optCppType,
		Alt:     optAlt,
		Free:    func(s string) string { return "" },
		CppToC: func(s string) string {
			return "(" + s + ".has_value() ? " + optCType + "_({" + cppToC(s+".value()") + ", true}) : " + optCType + "_({0, false}))"
		},
		CToCpp: func(s string) string {
			return "(" + s + ".has_value ? std::make_optional<" + cppType + ">(" + ctoCpp(s+".value") + ") : std::nullopt)"
		},
		CArg: func(s string) string {
			if s == "" {
				return optCType
			}
			return optCType + " " + s
		},
		CArgUntyped: func(s string) string { return s },
		CppArg: func(s string) string {
			return optCppType + s
		},
	})
}

func (r *Registry) registerOptionalHandleType(cppType string) {
	typedef := r.FindByCpp(cppType)
	if typedef == nil {
		return
	}

	optCppType := "std::optional<" + cppType + ">"
	var optAlt []string
	if len(typedef.Alt) > 0 {
		optAlt = []string{"std::optional<" + typedef.Alt[0] + ">"}
	}

	r.Register(&TypeMapping{
		CType:   typedef.CType,
		CppType: optCppType,
		Alt:     optAlt,
		CArg: func(s string) string {
			arg := typedef.CArg(s)
			return arg + " /* may be null */"
		},
		CArgUntyped: typedef.CArgUntyped,
		CToCpp: func(s string) string {
			return "(" + s + ".ctx ? std::make_optional(" + typedef.CToCpp(s) + ") : std::nullopt)"
		},
		CAssignFromCpp: func(dest, src string, returned bool) string {
			return "(" + src + ".has_value() ? " + src + ".value() : nullptr)"
		},
	})
}
