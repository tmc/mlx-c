/* Copyright © 2023-2024 Apple Inc.                   */
/*                                                    */
/* This file is auto-generated. Do not edit manually. */
/*                                                    */

#ifndef MLX_TRANSFORMS_H
#define MLX_TRANSFORMS_H

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
 * \defgroup transforms Transform operations
 */
/**@{*/

/**
 * Evaluate the arrays asynchronously.
 */
int mlx_async_eval(const mlx_vector_array outputs);

/**
 * Checkpoint the gradient of a function. Namely, discard all intermediate
 * state and recalculate it when we need to compute the gradient.
 */
int mlx_checkpoint(mlx_closure* res, const mlx_closure fun);

/**
 * Redefine the transformations of `fun` according to the provided functions.
 * Namely when calling the vjp of `fun` then `fun_vjp` will be called,
 * `fun_jvp` for the jvp and `fun_vmap` for vmap.
 * If any transformation is not provided, then a default one is created by
 * calling `vjp`, `jvp` and `vmap` on the function directly.
 */
int mlx_custom_function(
    mlx_closure* res,
    const mlx_closure fun,
    const mlx_closure_custom fun_vjp /* may be null */,
    const mlx_closure_custom_jvp fun_jvp /* may be null */,
    const mlx_closure_custom_vmap fun_vmap /* may be null */);

/**
 * Return a function that behaves exactly like `fun` but if the vjp of the
 * results is computed `fun_vjp` will be used instead of `vjp(fun, ...)` .
 */
int mlx_custom_vjp(
    mlx_closure* res,
    const mlx_closure fun,
    const mlx_closure_custom fun_vjp);

/**
 * Evaluate the arrays synchronously.
 */
int mlx_eval(const mlx_vector_array outputs);

/**
 * Computes the output and Jacobian-vector product (JVP) of a function.
 * Computes the Jacobian-vector product of the Jacobian of the function
 * evaluated at the primals with the vector of tangents. Returns a pair of
 * vectors of output arrays and JVP arrays.
 */
int mlx_jvp(
    mlx_vector_array* res_0,
    mlx_vector_array* res_1,
    const mlx_closure fun,
    const mlx_vector_array primals,
    const mlx_vector_array tangents);

/**
 * Returns a function which computes the value and gradient of the input
 * function with respect to a vector of input arrays.
 */
int mlx_value_and_grad(
    mlx_closure_value_and_grad* res,
    const mlx_closure fun,
    const int* argnums,
    size_t argnums_num);

/**
 * Computes the output and vector-Jacobian product (VJP) of a function.
 * Computes the vector-Jacobian product of the vector of cotangents with the
 * Jacobian of the function evaluated at the primals. Returns a pair of
 * vectors of output arrays and VJP arrays.
 */
int mlx_vjp(
    mlx_vector_array* res_0,
    mlx_vector_array* res_1,
    const mlx_closure fun,
    const mlx_vector_array primals,
    const mlx_vector_array cotangents);

/**@}*/

#ifdef __cplusplus
}
#endif

#endif
