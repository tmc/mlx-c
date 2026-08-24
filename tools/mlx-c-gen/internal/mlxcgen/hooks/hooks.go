// Package hooks provides special-case handlers for specific functions in MLX C bindings.
package hooks

import (
	"io"
	"sort"
)

// Hook is a function that handles special cases in code generation.
// It returns true if it handled the function (caller should skip normal generation),
// or false if the caller should proceed with normal generation.
type Hook func(w io.Writer, funcName string, impl bool) bool

// hooks maps function names to their special handlers.
var hooks = map[string]Hook{
	"mlx_export_to_dot":              mlxExportToDot,
	"mlx_fast_metal_kernel":          mlxFastMetalKernel,
	"mlx_fast_cuda_kernel":           mlxFastCudaKernel,
	"mlx_load_gguf":                  mlxLoadGGUF,
	"mlx_print_graph":                mlxPrintGraph,
	"mlx_save_gguf":                  mlxSaveGGUF,
}

// GetHook returns the hook for a function name, or nil if none exists.
func GetHook(funcName string) Hook {
	return hooks[funcName]
}

// HasHook returns true if a hook exists for the given function name.
func HasHook(funcName string) bool {
	_, ok := hooks[funcName]
	return ok
}

// Names returns the registered hook function names in lexical order.
func Names() []string {
	names := make([]string, 0, len(hooks))
	for name := range hooks {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func mlxFastMetalKernel(w io.Writer, funcName string, impl bool) bool {
	writeCustomKernel(w, "metal", impl, metalKernelNew)
	return true
}

func mlxFastCudaKernel(w io.Writer, funcName string, impl bool) bool {
	writeCustomKernel(w, "cuda", impl, cudaKernelNew)
	return true
}

func mlxExportToDot(w io.Writer, funcName string, impl bool) bool {
	if impl {
		io.WriteString(w, graphUtilsNodeNamerImpl)
		io.WriteString(w, graphUtilsExportToDotImpl)
	} else {
		io.WriteString(w, graphUtilsNodeNamerHeader)
		io.WriteString(w, graphUtilsExportToDotHeader)
	}
	return true
}

func mlxPrintGraph(w io.Writer, funcName string, impl bool) bool {
	if impl {
		io.WriteString(w, graphUtilsPrintGraphImpl)
	} else {
		io.WriteString(w, graphUtilsPrintGraphHeader)
	}
	return true
}

func mlxLoadGGUF(w io.Writer, funcName string, impl bool) bool {
	if impl {
		io.WriteString(w, loadGGUFImpl)
	} else {
		io.WriteString(w, loadGGUFHeader)
	}
	return true
}

func mlxSaveGGUF(w io.Writer, funcName string, impl bool) bool {
	if impl {
		io.WriteString(w, saveGGUFImpl)
	} else {
		io.WriteString(w, saveGGUFHeader)
	}
	return true
}

// Compile caches are PER-OS-THREAD in MLX >= 0.32.1: compile_cache() returns
// the calling thread's cache, and erase/clear act only on the passing
// handle's view. The emitted docs below say this loudly because naive global
// eviction silently no-ops across threads.

// EmitCompileCache writes the compile-cache handle glue once per phase.
// Exported for the generator: the compile module emits it before the standard
// binding of erase/clear.
var compileCacheHeaderDone, compileCacheImplDone bool

func EmitCompileCache(w io.Writer, impl bool) {
	if impl {
		if !compileCacheImplDone {
			compileCacheImplDone = true
			io.WriteString(w, compileCacheImpl)
		}
		return
	}
	if !compileCacheHeaderDone {
		compileCacheHeaderDone = true
		io.WriteString(w, compileCacheHeader)
	}
}

const compileCacheHeader = `
/* Compile caches are PER-OS-THREAD in MLX >= 0.32.1: mlx_compile_cache_current()
 * returns the calling thread's cache view. mlx_detail_compile_erase and
 * mlx_detail_compile_clear_cache act ONLY on the cache view of the passed
 * handle -- they are not global operations. */
typedef struct mlx_compile_cache_ mlx_compile_cache;
mlx_compile_cache* mlx_compile_cache_current(void);
int mlx_compile_cache_free(mlx_compile_cache* cache);
`

const compileCacheImpl = `
struct mlx_compile_cache_ {
  std::weak_ptr<mlx::core::detail::CompileCache> ctx;
};

std::weak_ptr<mlx::core::detail::CompileCache> mlx_compile_cache_weakptr_get_(
    mlx_compile_cache cache) {
  if (cache.ctx.expired()) {
    throw std::runtime_error("expected a live mlx_compile_cache");
  }
  return cache.ctx;
}

extern "C" mlx_compile_cache* mlx_compile_cache_current(void) {
  try {
    return new mlx_compile_cache_{mlx::core::detail::compile_cache()};
  } catch (std::exception& e) {
    mlx_error(e.what());
  }
  return nullptr;
}

extern "C" int mlx_compile_cache_free(mlx_compile_cache* cache) {
  try {
    delete cache;
    return 0;
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
}
`

const graphUtilsNodeNamerHeader = `
typedef struct mlx_node_namer_ {
  void* ctx;
} mlx_node_namer;

mlx_node_namer mlx_node_namer_new();
int mlx_node_namer_free(mlx_node_namer namer);
int mlx_node_namer_set_name(
    mlx_node_namer namer,
    const mlx_array arr,
    const char* name);
int mlx_node_namer_get_name(
    const char** name,
    mlx_node_namer namer,
    const mlx_array arr);
`

const graphUtilsExportToDotHeader = `
int mlx_export_to_dot(
    FILE* os,
    const mlx_node_namer namer,
    const mlx_vector_array outputs);
`

const graphUtilsPrintGraphHeader = `int mlx_print_graph(
    FILE* os,
    const mlx_node_namer namer,
    const mlx_vector_array outputs);

`

const graphUtilsNodeNamerImpl = `
extern "C" mlx_node_namer mlx_node_namer_new() {
  try {
    return mlx_node_namer_new_(mlx::core::NodeNamer());
  } catch (std::exception& e) {
    mlx_error(e.what());
  }
  return {nullptr};
}
extern "C" int mlx_node_namer_free(mlx_node_namer namer) {
  try {
    mlx_node_namer_free_(namer);
    return 0;
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
}
extern "C" int mlx_node_namer_set_name(
    mlx_node_namer namer,
    const mlx_array arr,
    const char* name) {
  try {
    mlx_node_namer_get_(namer).set_name(mlx_array_get_(arr), name);
    return 0;
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
}
extern "C" int mlx_node_namer_get_name(
    const char** name,
    mlx_node_namer namer,
    const mlx_array arr) {
  try {
    *name = mlx_node_namer_get_(namer).get_name(mlx_array_get_(arr)).c_str();
    return 0;
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
}
`

const graphUtilsExportToDotImpl = `extern "C" int mlx_export_to_dot(
    FILE* os,
    const mlx_node_namer namer,
    const mlx_vector_array outputs) {
  try {
    mlx::core::export_to_dot(
        CFileOutputStream::as_lvalue(CFileOutputStream(os)),
        mlx_node_namer_get_(namer),
        mlx_vector_array_get_(outputs));
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
  return 0;
}
`

const graphUtilsPrintGraphImpl = `extern "C" int mlx_print_graph(
    FILE* os,
    const mlx_node_namer namer,
    const mlx_vector_array outputs) {
  try {
    mlx::core::print_graph(
        CFileOutputStream::as_lvalue(CFileOutputStream(os)),
        mlx_node_namer_get_(namer),
        mlx_vector_array_get_(outputs));
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
  return 0;
}
`

const loadGGUFHeader = `
int mlx_load_gguf(mlx_io_gguf* gguf, const char* file, const mlx_stream s);

`

const loadGGUFImpl = `extern "C" int
mlx_load_gguf(mlx_io_gguf* gguf, const char* file, const mlx_stream s) {
  try {
    auto cpp_gguf = mlx::core::load_gguf(file, mlx_stream_get_(s));
    mlx_io_gguf_set_(*gguf, std::move(cpp_gguf));
    return 0;
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
}

`

const saveGGUFHeader = `int mlx_save_gguf(const char* file, mlx_io_gguf gguf);

`

const saveGGUFImpl = `extern "C" int mlx_save_gguf(const char* file, mlx_io_gguf gguf) {
  try {
    auto cpp_gguf = mlx_io_gguf_get_(gguf);
    mlx::core::save_gguf(file, cpp_gguf.first, cpp_gguf.second);
    return 0;
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
}

`

const metalKernelNew = `
mlx_fast_metal_kernel mlx_fast_metal_kernel_new(
    const char* name,
    const mlx_vector_string input_names,
    const mlx_vector_string output_names,
    const char* source,
    const char* header,
    bool ensure_row_contiguous,
    bool atomic_outputs);
`

const metalKernelNewImpl = `
inline mlx_fast_metal_kernel mlx_fast_metal_kernel_new_(
    const std::string& name,
    const std::vector<std::string>& input_names,
    const std::vector<std::string>& output_names,
    const std::string& source,
    const std::string& header,
    bool ensure_row_contiguous,
    bool atomic_outputs) {
  return mlx_fast_metal_kernel({new mlx_fast_metal_kernel_cpp_(
      mlx::core::fast::metal_kernel(
          name,
          input_names,
          output_names,
          source,
          header,
          ensure_row_contiguous,
          atomic_outputs))});
}

extern "C" mlx_fast_metal_kernel mlx_fast_metal_kernel_new(
    const char* name,
    const mlx_vector_string input_names,
    const mlx_vector_string output_names,
    const char* source,
    const char* header,
    bool ensure_row_contiguous,
    bool atomic_outputs) {
  try {
    return mlx_fast_metal_kernel_new_(
        name,
        mlx_vector_string_get_(input_names),
        mlx_vector_string_get_(output_names),
        source,
        header,
        ensure_row_contiguous,
        atomic_outputs);
  } catch (std::exception& e) {
    mlx_error(e.what());
  }
  return {nullptr};
}
`

const cudaKernelNew = `
mlx_fast_cuda_kernel mlx_fast_cuda_kernel_new(
    const char* name,
    const mlx_vector_string input_names,
    const mlx_vector_string output_names,
    const char* source,
    const char* header,
    bool ensure_row_contiguous,
    int shared_memory);
`

const cudaKernelNewImpl = `
inline mlx_fast_cuda_kernel mlx_fast_cuda_kernel_new_(
    const std::string& name,
    const std::vector<std::string>& input_names,
    const std::vector<std::string>& output_names,
    const std::string& source,
    const std::string& header,
    bool ensure_row_contiguous,
    int shared_memory) {
  return mlx_fast_cuda_kernel(
      {new mlx_fast_cuda_kernel_cpp_(mlx::core::fast::cuda_kernel(
          name,
          input_names,
          output_names,
          source,
          header,
          ensure_row_contiguous,
          shared_memory))});
}

extern "C" mlx_fast_cuda_kernel mlx_fast_cuda_kernel_new(
    const char* name,
    const mlx_vector_string input_names,
    const mlx_vector_string output_names,
    const char* source,
    const char* header,
    bool ensure_row_contiguous,
    int shared_memory) {
  try {
    return mlx_fast_cuda_kernel_new_(
        name,
        mlx_vector_string_get_(input_names),
        mlx_vector_string_get_(output_names),
        source,
        header,
        ensure_row_contiguous,
        shared_memory);
  } catch (std::exception& e) {
    mlx_error(e.what());
  }
  return {nullptr};
}
`

func writeCustomKernel(w io.Writer, backend string, impl bool, kernelNew string) {
	var codeConfig, codeDef, code string

	if impl {
		codeConfig = `
struct mlx_fast_custom_kernel_config_cpp_ {
  std::vector<mlx::core::Shape> output_shapes;
  std::vector<mlx::core::Dtype> output_dtypes;
  std::tuple<int, int, int> grid;
  std::tuple<int, int, int> thread_group;
  std::vector<std::pair<std::string, mlx::core::fast::TemplateArg>>
      template_args;
  std::optional<float> init_value;
  bool verbose;
};

inline mlx_fast_custom_kernel_config mlx_fast_custom_kernel_config_new_() {
  return mlx_fast_custom_kernel_config(
      {new mlx_fast_custom_kernel_config_cpp_()});
}

inline mlx_fast_custom_kernel_config_cpp_& mlx_fast_custom_kernel_config_get_(
    mlx_fast_custom_kernel_config d) {
  if (!d.ctx) {
    throw std::runtime_error(
        "expected a non-empty mlx_fast_custom_kernel_config");
  }
  return *static_cast<mlx_fast_custom_kernel_config_cpp_*>(d.ctx);
}

inline void mlx_fast_custom_kernel_config_free_(mlx_fast_custom_kernel_config d) {
  if (d.ctx) {
    delete static_cast<mlx_fast_custom_kernel_config_cpp_*>(d.ctx);
  }
}

extern "C" mlx_fast_custom_kernel_config mlx_fast_custom_kernel_config_new(void) {
  try {
    return mlx_fast_custom_kernel_config_new_();
  } catch (std::exception& e) {
    mlx_error(e.what());
  }
  return {nullptr};
}

extern "C" void mlx_fast_custom_kernel_config_free(
    mlx_fast_custom_kernel_config cls) {
  mlx_fast_custom_kernel_config_free_(cls);
}

extern "C" int mlx_fast_custom_kernel_config_add_output_arg(
    mlx_fast_custom_kernel_config cls,
    const int* shape,
    size_t size,
    mlx_dtype dtype) {
  try {
    mlx_fast_custom_kernel_config_get_(cls).output_shapes.push_back(
        mlx::core::Shape(shape, shape + size));
    mlx_fast_custom_kernel_config_get_(cls).output_dtypes.push_back(
        mlx_dtype_to_cpp(dtype));
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
  return 0;
}
extern "C" int mlx_fast_custom_kernel_config_set_grid(
    mlx_fast_custom_kernel_config cls,
    int grid1,
    int grid2,
    int grid3) {
  try {
    mlx_fast_custom_kernel_config_get_(cls).grid =
        std::make_tuple(grid1, grid2, grid3);
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
  return 0;
}
extern "C" int mlx_fast_custom_kernel_config_set_thread_group(
    mlx_fast_custom_kernel_config cls,
    int thread1,
    int thread2,
    int thread3) {
  try {
    mlx_fast_custom_kernel_config_get_(cls).thread_group =
        std::make_tuple(thread1, thread2, thread3);
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
  return 0;
}
extern "C" int mlx_fast_custom_kernel_config_set_init_value(
    mlx_fast_custom_kernel_config cls,
    float value) {
  try {
    mlx_fast_custom_kernel_config_get_(cls).init_value = value;
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
  return 0;
}
extern "C" int mlx_fast_custom_kernel_config_set_verbose(
    mlx_fast_custom_kernel_config cls,
    bool verbose) {
  try {
    mlx_fast_custom_kernel_config_get_(cls).verbose = verbose;
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
  return 0;
}
extern "C" int mlx_fast_custom_kernel_config_add_template_arg_dtype(
    mlx_fast_custom_kernel_config cls,
    const char* name,
    mlx_dtype dtype) {
  try {
    mlx_fast_custom_kernel_config_get_(cls).template_args.push_back(
        std::make_pair(std::string(name), mlx_dtype_to_cpp(dtype)));
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
  return 0;
}
extern "C" int mlx_fast_custom_kernel_config_add_template_arg_int(
    mlx_fast_custom_kernel_config cls,
    const char* name,
    int value) {
  try {
    mlx_fast_custom_kernel_config_get_(cls).template_args.push_back(
        std::make_pair(std::string(name), value));
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
  return 0;
}
extern "C" int mlx_fast_custom_kernel_config_add_template_arg_bool(
    mlx_fast_custom_kernel_config cls,
    const char* name,
    bool value) {
  try {
    mlx_fast_custom_kernel_config_get_(cls).template_args.push_back(
        std::make_pair(std::string(name), value));
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
  return 0;
}
`
		codeDef = `
struct mlx_fast_custom_kernel_cpp_ {
  mlx::core::fast::CustomKernelFunction mkf;
  mlx_fast_custom_kernel_cpp_(mlx::core::fast::CustomKernelFunction mkf)
      : mkf(mkf) {};
};
`
		code = `
inline mlx::core::fast::CustomKernelFunction& mlx_fast_custom_kernel_get_(
    mlx_fast_custom_kernel d) {
  if (!d.ctx) {
    throw std::runtime_error("expected a non-empty mlx_fast_custom_kernel");
  }
  return static_cast<mlx_fast_custom_kernel_cpp_*>(d.ctx)->mkf;
}

inline void mlx_fast_custom_kernel_free_(mlx_fast_custom_kernel d) {
  if (d.ctx) {
    delete static_cast<mlx_fast_custom_kernel_cpp_*>(d.ctx);
  }
}

extern "C" void mlx_fast_custom_kernel_free(mlx_fast_custom_kernel cls) {
  mlx_fast_custom_kernel_free_(cls);
}

extern "C" int mlx_fast_custom_kernel_apply(
    mlx_vector_array* outputs,
    mlx_fast_custom_kernel cls,
    const mlx_vector_array inputs,
    const mlx_fast_custom_kernel_config config,
    const mlx_stream stream) {
  try {
    auto config_ctx = mlx_fast_custom_kernel_config_get_(config);
    mlx_vector_array_set_(
        *outputs,
        mlx_fast_custom_kernel_get_(cls)(
            mlx_vector_array_get_(inputs),
            config_ctx.output_shapes,
            config_ctx.output_dtypes,
            config_ctx.grid,
            config_ctx.thread_group,
            config_ctx.template_args,
            config_ctx.init_value,
            config_ctx.verbose,
            mlx_stream_get_(stream)));
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
  return 0;
}
`
		// Add backend-specific kernel new
		if backend == "metal" {
			kernelNew = metalKernelNewImpl
		} else {
			kernelNew = cudaKernelNewImpl
		}
	} else {
		codeConfig = `
typedef struct mlx_fast_custom_kernel_config_ {
  void* ctx;
} mlx_fast_custom_kernel_config;
mlx_fast_custom_kernel_config mlx_fast_custom_kernel_config_new(void);
void mlx_fast_custom_kernel_config_free(mlx_fast_custom_kernel_config cls);

int mlx_fast_custom_kernel_config_add_output_arg(
    mlx_fast_custom_kernel_config cls,
    const int* shape,
    size_t size,
    mlx_dtype dtype);
int mlx_fast_custom_kernel_config_set_grid(
    mlx_fast_custom_kernel_config cls,
    int grid1,
    int grid2,
    int grid3);
int mlx_fast_custom_kernel_config_set_thread_group(
    mlx_fast_custom_kernel_config cls,
    int thread1,
    int thread2,
    int thread3);
int mlx_fast_custom_kernel_config_set_init_value(
    mlx_fast_custom_kernel_config cls,
    float value);
int mlx_fast_custom_kernel_config_set_verbose(
    mlx_fast_custom_kernel_config cls,
    bool verbose);
int mlx_fast_custom_kernel_config_add_template_arg_dtype(
    mlx_fast_custom_kernel_config cls,
    const char* name,
    mlx_dtype dtype);
int mlx_fast_custom_kernel_config_add_template_arg_int(
    mlx_fast_custom_kernel_config cls,
    const char* name,
    int value);
int mlx_fast_custom_kernel_config_add_template_arg_bool(
    mlx_fast_custom_kernel_config cls,
    const char* name,
    bool value);
`
		codeDef = `
typedef struct mlx_fast_custom_kernel_ {
  void* ctx;
} mlx_fast_custom_kernel;
`
		code = `
void mlx_fast_custom_kernel_free(mlx_fast_custom_kernel cls);

int mlx_fast_custom_kernel_apply(
    mlx_vector_array* outputs,
    mlx_fast_custom_kernel cls,
    const mlx_vector_array inputs,
    const mlx_fast_custom_kernel_config config,
    const mlx_stream stream);
`
	}

	// Replace "custom" with backend name
	codeConfig = replaceAll(codeConfig, "custom", backend)
	codeDef = replaceAll(codeDef, "custom", backend)
	code = replaceAll(code, "custom", backend)

	io.WriteString(w, codeConfig)
	io.WriteString(w, codeDef)
	io.WriteString(w, kernelNew)
	io.WriteString(w, code)
	io.WriteString(w, "\n") // Blank line after hook output to match Python
}

func replaceAll(s, old, new string) string {
	result := s
	for {
		updated := replaceOne(result, old, new)
		if updated == result {
			break
		}
		result = updated
	}
	return result
}

func replaceOne(s, old, new string) string {
	idx := findString(s, old)
	if idx == -1 {
		return s
	}
	return s[:idx] + new + s[idx+len(old):]
}

func findString(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
