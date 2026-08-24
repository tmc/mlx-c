/* Copyright © 2023-2024 Apple Inc.                   */
/*                                                    */
/* This file is auto-generated. Do not edit manually. */
/*                                                    */

#ifndef MLX_DISTRIBUTED_H
#define MLX_DISTRIBUTED_H

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
 * \defgroup distributed Distributed collectives
 */
/**@{*/

/**
 * Gather arrays from all ranks in the group.
 */
int mlx_distributed_all_gather(
    mlx_array* res,
    const mlx_array x,
    const mlx_distributed_group group /* may be null */,
    const mlx_stream S);

/**
 * Compute the element-wise maximum across all ranks in the group.
 */
int mlx_distributed_all_max(
    mlx_array* res,
    const mlx_array x,
    const mlx_distributed_group group /* may be null */,
    const mlx_stream s);

/**
 * Compute the element-wise minimum across all ranks in the group.
 */
int mlx_distributed_all_min(
    mlx_array* res,
    const mlx_array x,
    const mlx_distributed_group group /* may be null */,
    const mlx_stream s);

/**
 * Compute the element-wise sum across all ranks in the group.
 */
int mlx_distributed_all_sum(
    mlx_array* res,
    const mlx_array x,
    const mlx_distributed_group group /* may be null */,
    const mlx_stream s);

/**
 * Receive an array with the given shape and dtype from a source rank.
 */
int mlx_distributed_recv(
    mlx_array* res,
    const int* shape,
    size_t shape_num,
    mlx_dtype dtype,
    int src,
    const mlx_distributed_group group /* may be null */,
    const mlx_stream s);

/**
 * Receive an array shaped and typed like the input from a source rank.
 */
int mlx_distributed_recv_like(
    mlx_array* res,
    const mlx_array x,
    int src,
    const mlx_distributed_group group /* may be null */,
    const mlx_stream s);

/**
 * Send an array to a destination rank.
 */
int mlx_distributed_send(
    mlx_array* res,
    const mlx_array x,
    int dst,
    const mlx_distributed_group group /* may be null */,
    const mlx_stream s);

/**
 * Sum arrays across the group and scatter the result by rank.
 */
int mlx_distributed_sum_scatter(
    mlx_array* res,
    const mlx_array x,
    const mlx_distributed_group group /* may be null */,
    const mlx_stream s);

/**@}*/

#ifdef __cplusplus
}
#endif

#endif
