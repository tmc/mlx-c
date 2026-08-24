/* Copyright © 2023-2024 Apple Inc.                   */
/*                                                    */
/* This file is auto-generated. Do not edit manually. */
/*                                                    */

#ifndef MLX_LINALG_H
#define MLX_LINALG_H

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
 * \defgroup linalg Linear algebra operations
 */
/**@{*/

/**
 * Compute the Cholesky decomposition of a positive definite matrix.
 */
int mlx_linalg_cholesky(
    mlx_array* res,
    const mlx_array a,
    bool upper,
    const mlx_stream s);

/**
 * Compute the inverse from a Cholesky factorization.
 */
int mlx_linalg_cholesky_inv(
    mlx_array* res,
    const mlx_array a,
    bool upper,
    const mlx_stream s);

/**
 * Compute the cross product of two arrays along the given axis.
 */
int mlx_linalg_cross(
    mlx_array* res,
    const mlx_array a,
    const mlx_array b,
    int axis,
    const mlx_stream s);

/**
 * Compute the determinant of a square matrix.
 */
int mlx_linalg_det(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Compute eigenvalues and right eigenvectors of a square matrix.
 */
int mlx_linalg_eig(
    mlx_array* res_0,
    mlx_array* res_1,
    const mlx_array a,
    const mlx_stream s);

/**
 * Compute eigenvalues and eigenvectors of a Hermitian matrix.
 */
int mlx_linalg_eigh(
    mlx_array* res_0,
    mlx_array* res_1,
    const mlx_array a,
    const char* UPLO,
    const mlx_stream s);

/**
 * Compute eigenvalues of a square matrix.
 */
int mlx_linalg_eigvals(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Compute eigenvalues of a Hermitian matrix.
 */
int mlx_linalg_eigvalsh(
    mlx_array* res,
    const mlx_array a,
    const char* UPLO,
    const mlx_stream s);

/**
 * Compute the inverse of a square matrix.
 */
int mlx_linalg_inv(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Compute the LU factorization of a matrix.
 */
int mlx_linalg_lu(mlx_vector_array* res, const mlx_array a, const mlx_stream s);

/**
 * Compute packed LU factors and pivots for a matrix.
 */
int mlx_linalg_lu_factor(
    mlx_array* res_0,
    mlx_array* res_1,
    const mlx_array a,
    const mlx_stream s);

/**
 * Compute vector or matrix norms.
 * - If axis and ord are both unspecified, computes the 2-norm of flatten(x).
 * - If axis is not provided but ord is, then x must be either 1D or 2D.
 * - If axis is provided, but ord is not, then the 2-norm (or Frobenius norm
 * for matrices) is computed along the given axes. At most 2 axes can be
 * specified.
 * - If both axis and ord are provided, then the corresponding matrix or vector
 * norm is computed. At most 2 axes can be specified.
 */
int mlx_linalg_norm(
    mlx_array* res,
    const mlx_array a,
    double ord,
    const int* axis /* may be null */,
    size_t axis_num,
    bool keepdims,
    const mlx_stream s);

/**
 * Compute a matrix norm over the given axes.
 */
int mlx_linalg_norm_matrix(
    mlx_array* res,
    const mlx_array a,
    const char* ord,
    const int* axis /* may be null */,
    size_t axis_num,
    bool keepdims,
    const mlx_stream s);

/**
 * Compute the L2 norm over the given axes.
 */
int mlx_linalg_norm_l2(
    mlx_array* res,
    const mlx_array a,
    const int* axis /* may be null */,
    size_t axis_num,
    bool keepdims,
    const mlx_stream s);

/**
 * Compute the Moore-Penrose pseudoinverse of a matrix.
 */
int mlx_linalg_pinv(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Compute the QR factorization of a matrix.
 */
int mlx_linalg_qr(
    mlx_array* res_0,
    mlx_array* res_1,
    const mlx_array a,
    const mlx_stream s);

/**
 * Compute the sign and logarithm of the determinant.
 */
int mlx_linalg_slogdet(
    mlx_array* res_0,
    mlx_array* res_1,
    const mlx_array a,
    const mlx_stream s);

/**
 * Solve a linear system.
 */
int mlx_linalg_solve(
    mlx_array* res,
    const mlx_array a,
    const mlx_array b,
    const mlx_stream s);

/**
 * Solve a triangular linear system.
 */
int mlx_linalg_solve_triangular(
    mlx_array* res,
    const mlx_array a,
    const mlx_array b,
    bool upper,
    const mlx_stream s);

/**
 * Compute the singular value decomposition of a matrix.
 */
int mlx_linalg_svd(
    mlx_vector_array* res,
    const mlx_array a,
    bool compute_uv,
    const mlx_stream s);

/**
 * Compute the inverse of a triangular matrix.
 */
int mlx_linalg_tri_inv(
    mlx_array* res,
    const mlx_array a,
    bool upper,
    const mlx_stream s);

/**@}*/

#ifdef __cplusplus
}
#endif

#endif
