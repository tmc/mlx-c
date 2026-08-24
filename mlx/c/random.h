/* Copyright © 2023-2024 Apple Inc.                   */
/*                                                    */
/* This file is auto-generated. Do not edit manually. */
/*                                                    */

#ifndef MLX_RANDOM_H
#define MLX_RANDOM_H

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
 * \defgroup random Random number operations
 */
/**@{*/

/**
 * Generate binary variables with probability to be true equal to p
 */
int mlx_random_bernoulli(
    mlx_array* res,
    const mlx_array p,
    const int* shape,
    size_t shape_num,
    const mlx_array key /* may be null */,
    const mlx_stream s);

/**
 * Generate an array with type uint32 filled with random bits.
 */
int mlx_random_bits(
    mlx_array* res,
    const int* shape,
    size_t shape_num,
    int width,
    const mlx_array key /* may be null */,
    const mlx_stream s);

/**
 * Sample categorical values with the given output shape.
 */
int mlx_random_categorical_shape(
    mlx_array* res,
    const mlx_array logits,
    int axis,
    const int* shape,
    size_t shape_num,
    const mlx_array key /* may be null */,
    const mlx_stream s);

/**
 * Sample the given number of categorical values.
 */
int mlx_random_categorical_num_samples(
    mlx_array* res,
    const mlx_array logits_,
    int axis,
    int num_samples,
    const mlx_array key /* may be null */,
    const mlx_stream s);

/**
 * Sample categorical values from logits along an axis.
 */
int mlx_random_categorical(
    mlx_array* res,
    const mlx_array logits,
    int axis,
    const mlx_array key /* may be null */,
    const mlx_stream s);

/**
 * Generate samples from a Gumbel distribution.
 */
int mlx_random_gumbel(
    mlx_array* res,
    const int* shape,
    size_t shape_num,
    mlx_dtype dtype,
    const mlx_array key /* may be null */,
    const mlx_stream s);

/**
 * Get a PRNG key from a seed.
 */
int mlx_random_key(mlx_array* res, uint64_t seed);

/**
 * Generate samples from the laplace distribution.
 */
int mlx_random_laplace(
    mlx_array* res,
    const int* shape,
    size_t shape_num,
    mlx_dtype dtype,
    float loc,
    float scale,
    const mlx_array key /* may be null */,
    const mlx_stream s);

/**
 * Generate samples from a multivariate normal distribution.
 */
int mlx_random_multivariate_normal(
    mlx_array* res,
    const mlx_array mean,
    const mlx_array cov,
    const int* shape,
    size_t shape_num,
    mlx_dtype dtype,
    const mlx_array key /* may be null */,
    const mlx_stream s);

/**
 * Generate samples from the standard normal distribution.
 */
int mlx_random_normal_broadcast(
    mlx_array* res,
    const int* shape,
    size_t shape_num,
    mlx_dtype dtype,
    const mlx_array loc /* may be null */,
    const mlx_array scale /* may be null */,
    const mlx_array key /* may be null */,
    const mlx_stream s);

/**
 * Generate samples from a normal distribution with scalar mean and standard
 * deviation.
 */
int mlx_random_normal(
    mlx_array* res,
    const int* shape,
    size_t shape_num,
    mlx_dtype dtype,
    float loc,
    float scale,
    const mlx_array key /* may be null */,
    const mlx_stream s);

/**
 * Return a randomly permuted copy of an array along an axis.
 */
int mlx_random_permutation(
    mlx_array* res,
    const mlx_array x,
    int axis,
    const mlx_array key /* may be null */,
    const mlx_stream s);

/**
 * Return a random permutation of integers from zero to n.
 */
int mlx_random_permutation_arange(
    mlx_array* res,
    int x,
    const mlx_array key /* may be null */,
    const mlx_stream s);

/**
 * Generate integer samples uniformly at random
 */
int mlx_random_randint(
    mlx_array* res,
    const mlx_array low,
    const mlx_array high,
    const int* shape,
    size_t shape_num,
    mlx_dtype dtype,
    const mlx_array key /* may be null */,
    const mlx_stream s);

/**
 * Seed the default PRNG key.
 */
int mlx_random_seed(uint64_t seed);

/**
 * Split the rng key into `num` keys.
 */
int mlx_random_split_num(
    mlx_array* res,
    const mlx_array key,
    int num,
    const mlx_stream s);

/**
 * Split the rng key into a pair of keys.
 */
int mlx_random_split(
    mlx_array* res_0,
    mlx_array* res_1,
    const mlx_array key,
    const mlx_stream s);

/**
 * Generate samples from a normal distribution truncated between lower and upper
 * bounds.
 */
int mlx_random_truncated_normal(
    mlx_array* res,
    const mlx_array lower,
    const mlx_array upper,
    const int* shape,
    size_t shape_num,
    mlx_dtype dtype,
    const mlx_array key /* may be null */,
    const mlx_stream s);

/**
 * Generate uniform random numbers between low and high.
 */
int mlx_random_uniform(
    mlx_array* res,
    const mlx_array low,
    const mlx_array high,
    const int* shape,
    size_t shape_num,
    mlx_dtype dtype,
    const mlx_array key /* may be null */,
    const mlx_stream s);

/**@}*/

#ifdef __cplusplus
}
#endif

#endif
