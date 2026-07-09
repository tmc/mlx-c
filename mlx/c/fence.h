/* Copyright © 2023-2024 Apple Inc. */

#ifndef MLX_FENCE_H
#define MLX_FENCE_H

#include <stdbool.h>

#include "mlx/c/array.h"
#include "mlx/c/stream.h"

#ifdef __cplusplus
extern "C" {
#endif

/**
 * \defgroup fence Fence synchronization
 */
/**@{*/

/**
 * A fence for synchronizing work between streams.
 */
typedef struct mlx_fence_ {
  void* ctx;
} mlx_fence;

/**
 * Return a new invalid fence.
 */
mlx_fence mlx_fence_new(void);

/**
 * Return a new fence associated with stream.
 */
mlx_fence mlx_fence_new_stream(mlx_stream stream);

/**
 * Free a fence.
 */
int mlx_fence_free(mlx_fence fence);

/**
 * Update the fence after computing x in stream.
 */
int mlx_fence_update(
    mlx_fence fence,
    mlx_stream stream,
    const mlx_array x,
    bool cross_device);

/**
 * Wait in stream until previous fence updates have completed.
 */
int mlx_fence_wait(mlx_fence fence, mlx_stream stream, const mlx_array x);

/**@}*/

#ifdef __cplusplus
}
#endif

#endif
