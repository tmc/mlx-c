/* Copyright © 2023-2024 Apple Inc.                   */
/*                                                    */
/* This file is auto-generated. Do not edit manually. */
/*                                                    */

#ifndef MLX_MEMORY_H
#define MLX_MEMORY_H

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
 * \defgroup memory Memory operations
 */
/**@{*/

/**
 * Clear the memory cache.
 */
int mlx_clear_cache(void);

/**
 * Return actively used memory in bytes, excluding cached buffers.
 */
int mlx_get_active_memory(size_t* res);

/**
 * Return the memory cache size in bytes.
 */
int mlx_get_cache_memory(size_t* res);

/**
 * Return the current memory limit in bytes.
 */
int mlx_get_memory_limit(size_t* res);

/**
 * Return the peak active memory in bytes since startup or the last reset.
 */
int mlx_get_peak_memory(size_t* res);

/**
 * Reset the recorded peak active memory to zero.
 */
int mlx_reset_peak_memory(void);

/**
 * Set the cache limit in bytes and return the previous limit.
 */
int mlx_set_cache_limit(size_t* res, size_t limit);

/**
 * Set the memory limit in bytes and return the previous limit.
 */
int mlx_set_memory_limit(size_t* res, size_t limit);

/**
 * Set the wired memory limit in bytes and return the previous limit.
 */
int mlx_set_wired_limit(size_t* res, size_t limit);

/**@}*/

#ifdef __cplusplus
}
#endif

#endif
