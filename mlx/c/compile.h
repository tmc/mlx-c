/* Copyright © 2023-2024 Apple Inc.                   */
/*                                                    */
/* This file is auto-generated. Do not edit manually. */
/*                                                    */

#ifndef MLX_COMPILE_H
#define MLX_COMPILE_H

#include <stdbool.h>
#include <stdint.h>
#include <stdio.h>

#include "mlx/c/array.h"
#include "mlx/c/closure.h"
#include "mlx/c/distributed_group.h"
#include "mlx/c/io_types.h"
#include "mlx/c/map.h"
#include "mlx/c/stream.h"
#include "mlx/c/string.h"
#include "mlx/c/vector.h"

#ifdef __cplusplus
extern "C" {
#endif

/**
 * \defgroup compile Compilation operations
 */
/**@{*/

/* Compile caches are PER-OS-THREAD: mlx_compile_cache_current() returns the
 * calling thread's cache view. The handle-taking erase/clear API arrived in
 * MLX 0.32.1; mlx_detail_compile_erase and mlx_detail_compile_clear_cache act
 * ONLY on the cache view of the passed handle -- they are not global
 * operations. */
typedef struct mlx_compile_cache_ {
  void* ctx; /* heap-allocated std::weak_ptr<CompileCache> */
} mlx_compile_cache;
mlx_compile_cache mlx_compile_cache_current(void);
int mlx_compile_cache_free(mlx_compile_cache cache);
typedef enum mlx_compile_mode_ {
  MLX_COMPILE_MODE_DISABLED,
  MLX_COMPILE_MODE_NO_SIMPLIFY,
  MLX_COMPILE_MODE_NO_FUSE,
  MLX_COMPILE_MODE_ENABLED
} mlx_compile_mode;

/**
 * Compile takes a function and returns a compiled function.
 */
int mlx_compile(mlx_closure* res, const mlx_closure fun, bool shapeless);
int mlx_detail_compile(
    mlx_closure* res,
    const mlx_closure fun,
    uintptr_t fun_id,
    bool shapeless,
    const uint64_t* constants,
    size_t constants_num);
int mlx_detail_compile_clear_cache(mlx_compile_cache cache);
int mlx_detail_compile_erase(mlx_compile_cache cache, uintptr_t fun_id);

/**
 * Globally disable compilation.
 * Setting the environment variable ``MLX_DISABLE_COMPILE`` can also
 * be used to disable compilation.
 */
int mlx_disable_compile(void);

/**
 * Globally enable compilation.
 * This will override the environment variable ``MLX_DISABLE_COMPILE``.
 */
int mlx_enable_compile(void);

/**
 * Set the compiler mode to the given value.
 */
int mlx_set_compile_mode(mlx_compile_mode mode);

/**@}*/

#ifdef __cplusplus
}
#endif

#endif
