/* Copyright © 2023-2024 Apple Inc.                   */
/*                                                    */
/* This file is auto-generated. Do not edit manually. */
/*                                                    */

#ifndef MLX_OPS_H
#define MLX_OPS_H

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
 * \defgroup ops Core array operations
 */
/**@{*/

/**
 * Absolute value of elements in an array.
 */
int mlx_abs(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Add two arrays.
 */
int mlx_add(
    mlx_array* res,
    const mlx_array a,
    const mlx_array b,
    const mlx_stream s);

/**
 * Compute D = beta * C + alpha * (A
 * @
 * B)
 */
int mlx_addmm(
    mlx_array* res,
    const mlx_array c,
    const mlx_array a,
    const mlx_array b,
    float alpha,
    float beta,
    const mlx_stream s);

/**
 * Reduces the input along the given axes. An output value is true
 * if all the corresponding inputs are true.
 */
int mlx_all_axes(
    mlx_array* res,
    const mlx_array a,
    const int* axes,
    size_t axes_num,
    bool keepdims,
    const mlx_stream s);

/**
 * Reduces the input along the given axis. An output value is true
 * if all the corresponding inputs are true.
 */
int mlx_all_axis(
    mlx_array* res,
    const mlx_array a,
    int axis,
    bool keepdims,
    const mlx_stream s);

/**
 * True if all elements in the array are true (or non-zero).
 */
int mlx_all(
    mlx_array* res,
    const mlx_array a,
    bool keepdims,
    const mlx_stream s);

/**
 * True if the two arrays are equal within the specified tolerance.
 */
int mlx_allclose(
    mlx_array* res,
    const mlx_array a,
    const mlx_array b,
    double rtol,
    double atol,
    bool equal_nan,
    const mlx_stream s);

/**
 * Reduces the input along the given axes. An output value is true
 * if any of the corresponding inputs are true.
 */
int mlx_any_axes(
    mlx_array* res,
    const mlx_array a,
    const int* axes,
    size_t axes_num,
    bool keepdims,
    const mlx_stream s);

/**
 * Reduces the input along the given axis. An output value is true
 * if any of the corresponding inputs are true.
 */
int mlx_any_axis(
    mlx_array* res,
    const mlx_array a,
    int axis,
    bool keepdims,
    const mlx_stream s);

/**
 * True if any elements in the array are true (or non-zero).
 */
int mlx_any(
    mlx_array* res,
    const mlx_array a,
    bool keepdims,
    const mlx_stream s);

/**
 * A 1D array of numbers starting at `start` (optional),
 * stopping at stop, stepping by `step` (optional).
 */
int mlx_arange(
    mlx_array* res,
    double start,
    double stop,
    double step,
    mlx_dtype dtype,
    const mlx_stream s);

/**
 * Arc Cosine of the elements of an array
 */
int mlx_arccos(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Inverse Hyperbolic Cosine of the elements of an array
 */
int mlx_arccosh(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Arc Sine of the elements of an array
 */
int mlx_arcsin(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Inverse Hyperbolic Sine of the elements of an array
 */
int mlx_arcsinh(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Arc Tangent of the elements of an array
 */
int mlx_arctan(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Inverse tangent of the ratio of two arrays
 */
int mlx_arctan2(
    mlx_array* res,
    const mlx_array a,
    const mlx_array b,
    const mlx_stream s);

/**
 * Inverse Hyperbolic Tangent of the elements of an array
 */
int mlx_arctanh(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Returns the indices of the maximum values along a given axis.
 */
int mlx_argmax_axis(
    mlx_array* res,
    const mlx_array a,
    int axis,
    bool keepdims,
    const mlx_stream s);

/**
 * Returns the index of the maximum value in the array.
 */
int mlx_argmax(
    mlx_array* res,
    const mlx_array a,
    bool keepdims,
    const mlx_stream s);

/**
 * Returns the indices of the minimum values along a given axis.
 */
int mlx_argmin_axis(
    mlx_array* res,
    const mlx_array a,
    int axis,
    bool keepdims,
    const mlx_stream s);

/**
 * Returns the index of the minimum value in the array.
 */
int mlx_argmin(
    mlx_array* res,
    const mlx_array a,
    bool keepdims,
    const mlx_stream s);

/**
 * Returns indices that partition the array along a given axis
 * such that the smaller kth elements are first.
 */
int mlx_argpartition_axis(
    mlx_array* res,
    const mlx_array a,
    int kth,
    int axis,
    const mlx_stream s);

/**
 * Returns indices that partition the flattened array
 * such that the smaller kth elements are first.
 */
int mlx_argpartition(
    mlx_array* res,
    const mlx_array a,
    int kth,
    const mlx_stream s);

/**
 * Returns indices that sort the array along a given axis.
 * The sort is stable and NaN values are placed at the end.
 */
int mlx_argsort_axis(
    mlx_array* res,
    const mlx_array a,
    int axis,
    const mlx_stream s);

/**
 * Returns indices that sort the flattened array.
 * The sort is stable and NaN values are placed at the end.
 */
int mlx_argsort(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * True if two arrays have the same shape and elements.
 */
int mlx_array_equal(
    mlx_array* res,
    const mlx_array a,
    const mlx_array b,
    bool equal_nan,
    const mlx_stream s);

/**
 * Create a view of an array with the given shape and strides.
 */
int mlx_as_strided(
    mlx_array* res,
    const mlx_array a,
    const int* shape,
    size_t shape_num,
    const int64_t* strides,
    size_t strides_num,
    size_t offset,
    const mlx_stream s);

/**
 * Convert an array to the given data type.
 */
int mlx_astype_copy(
    mlx_array* res,
    const mlx_array a,
    mlx_dtype dtype,
    mlx_optional_bool copy,
    const mlx_stream s);

/**
 * Cast an array to a dtype, using MLX's default copy behavior.
 */
int mlx_astype(
    mlx_array* res,
    const mlx_array a,
    mlx_dtype dtype,
    const mlx_stream s);

/**
 * convert an array to an atleast ndim array
 */
int mlx_atleast_1d(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Return a view of the input with at least two dimensions.
 */
int mlx_atleast_2d(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Return a view of the input with at least three dimensions.
 */
int mlx_atleast_3d(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Returns the bartlett window of size M.
 */
int mlx_bartlett(mlx_array* res, int M, const mlx_stream s);

/**
 * Bitwise and.
 */
int mlx_bitwise_and(
    mlx_array* res,
    const mlx_array a,
    const mlx_array b,
    const mlx_stream s);

/**
 * Invert the bits.
 */
int mlx_bitwise_invert(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Bitwise inclusive or.
 */
int mlx_bitwise_or(
    mlx_array* res,
    const mlx_array a,
    const mlx_array b,
    const mlx_stream s);

/**
 * Bitwise exclusive or.
 */
int mlx_bitwise_xor(
    mlx_array* res,
    const mlx_array a,
    const mlx_array b,
    const mlx_stream s);

/**
 * Returns the Blackmann window of size M.
 */
int mlx_blackman(mlx_array* res, int M, const mlx_stream s);

/**
 * Compute matrix product with block masking
 */
int mlx_block_masked_mm(
    mlx_array* res,
    const mlx_array a,
    const mlx_array b,
    int block_size,
    const mlx_array mask_out /* may be null */,
    const mlx_array mask_lhs /* may be null */,
    const mlx_array mask_rhs /* may be null */,
    const mlx_stream s);

/**
 * Broadcast a vector of arrays against one another.
 */
int mlx_broadcast_arrays(
    mlx_vector_array* res,
    const mlx_vector_array inputs,
    const mlx_stream s);

/**
 * Broadcast an array to a given shape.
 */
int mlx_broadcast_to(
    mlx_array* res,
    const mlx_array a,
    const int* shape,
    size_t shape_num,
    const mlx_stream s);

/**
 * Ceil the element of an array.
 */
int mlx_ceil(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Clip (limit) the values in an array.
 */
int mlx_clip(
    mlx_array* res,
    const mlx_array a,
    const mlx_array a_min /* may be null */,
    const mlx_array a_max /* may be null */,
    const mlx_stream s);

/**
 * Concatenate arrays along a given axis.
 */
int mlx_concatenate_axis(
    mlx_array* res,
    const mlx_vector_array arrays,
    int axis,
    const mlx_stream s);

/**
 * Concatenate arrays along the first axis.
 */
int mlx_concatenate(
    mlx_array* res,
    const mlx_vector_array arrays,
    const mlx_stream s);

/**
 * Return the complex conjugate of each element.
 */
int mlx_conjugate(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Return a contiguous array, optionally allowing column-major layout.
 */
int mlx_contiguous(
    mlx_array* res,
    const mlx_array a,
    bool allow_col_major,
    const mlx_stream s);

/**
 * 1D convolution with a filter
 */
int mlx_conv1d(
    mlx_array* res,
    const mlx_array input,
    const mlx_array weight,
    int stride,
    int padding,
    int dilation,
    int groups,
    const mlx_stream s);

/**
 * 2D convolution with a filter
 */
int mlx_conv2d(
    mlx_array* res,
    const mlx_array input,
    const mlx_array weight,
    int stride_0,
    int stride_1,
    int padding_0,
    int padding_1,
    int dilation_0,
    int dilation_1,
    int groups,
    const mlx_stream s);

/**
 * 3D convolution with a filter
 */
int mlx_conv3d(
    mlx_array* res,
    const mlx_array input,
    const mlx_array weight,
    int stride_0,
    int stride_1,
    int stride_2,
    int padding_0,
    int padding_1,
    int padding_2,
    int dilation_0,
    int dilation_1,
    int dilation_2,
    int groups,
    const mlx_stream s);

/**
 * General convolution with a filter
 */
int mlx_conv_general(
    mlx_array* res,
    const mlx_array input,
    const mlx_array weight,
    const int* stride,
    size_t stride_num,
    const int* padding_lo,
    size_t padding_lo_num,
    const int* padding_hi,
    size_t padding_hi_num,
    const int* kernel_dilation,
    size_t kernel_dilation_num,
    const int* input_dilation,
    size_t input_dilation_num,
    int groups,
    bool flip,
    const mlx_stream s);

/**
 * 1D transposed convolution with a filter
 */
int mlx_conv_transpose1d(
    mlx_array* res,
    const mlx_array input,
    const mlx_array weight,
    int stride,
    int padding,
    int dilation,
    int output_padding,
    int groups,
    const mlx_stream s);

/**
 * 2D transposed convolution with a filter
 */
int mlx_conv_transpose2d(
    mlx_array* res,
    const mlx_array input,
    const mlx_array weight,
    int stride_0,
    int stride_1,
    int padding_0,
    int padding_1,
    int dilation_0,
    int dilation_1,
    int output_padding_0,
    int output_padding_1,
    int groups,
    const mlx_stream s);

/**
 * 3D transposed convolution with a filter
 */
int mlx_conv_transpose3d(
    mlx_array* res,
    const mlx_array input,
    const mlx_array weight,
    int stride_0,
    int stride_1,
    int stride_2,
    int padding_0,
    int padding_1,
    int padding_2,
    int dilation_0,
    int dilation_1,
    int dilation_2,
    int output_padding_0,
    int output_padding_1,
    int output_padding_2,
    int groups,
    const mlx_stream s);

/**
 * Copy another array.
 */
int mlx_copy(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Cosine of the elements of an array
 */
int mlx_cos(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Hyperbolic Cosine of the elements of an array
 */
int mlx_cosh(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Count nonzero elements over multiple axes.
 */
int mlx_count_nonzero_axes(
    mlx_array* res,
    const mlx_array a,
    const int* axes,
    size_t axes_num,
    bool keepdims,
    const mlx_stream s);

/**
 * Count nonzero elements over one axis.
 */
int mlx_count_nonzero(
    mlx_array* res,
    const mlx_array a,
    int axis,
    bool keepdims,
    const mlx_stream s);

/**
 * Count the number of non-zero elements in an array.
 */
int mlx_count_nonzero_keepdims(
    mlx_array* res,
    const mlx_array a,
    bool keepdims,
    const mlx_stream s);

/**
 * Cumulative max of an array along the given axis.
 */
int mlx_cummax(
    mlx_array* res,
    const mlx_array a,
    int axis,
    bool reverse,
    bool inclusive,
    const mlx_stream s);

/**
 * Cumulative min of an array along the given axis.
 */
int mlx_cummin(
    mlx_array* res,
    const mlx_array a,
    int axis,
    bool reverse,
    bool inclusive,
    const mlx_stream s);

/**
 * Cumulative product of an array along the given axis.
 */
int mlx_cumprod_dtype(
    mlx_array* res,
    const mlx_array a,
    int axis,
    bool reverse,
    bool inclusive,
    mlx_optional_dtype dtype,
    const mlx_stream s);

/**
 * Return the cumulative product along an axis.
 */
int mlx_cumprod(
    mlx_array* res,
    const mlx_array a,
    int axis,
    bool reverse,
    bool inclusive,
    const mlx_stream s);

/**
 * Cumulative sum of an array along the given axis.
 */
int mlx_cumsum_dtype(
    mlx_array* res,
    const mlx_array a,
    int axis,
    bool reverse,
    bool inclusive,
    mlx_optional_dtype dtype,
    const mlx_stream s);

/**
 * Return the cumulative sum along an axis.
 */
int mlx_cumsum(
    mlx_array* res,
    const mlx_array a,
    int axis,
    bool reverse,
    bool inclusive,
    const mlx_stream s);

/**
 * Convert the elements of an array from Radians to Degrees
 */
int mlx_degrees(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Implements the identity function but allows injecting dependencies to other
 * arrays. This ensures that these other arrays will have been computed
 * when the outputs of this function are computed.
 */
int mlx_depends(
    mlx_vector_array* res,
    const mlx_vector_array inputs,
    const mlx_vector_array dependencies);

/**
 * Dequantize a matrix produced by quantize()
 */
int mlx_dequantize(
    mlx_array* res,
    const mlx_array w,
    const mlx_array scales,
    const mlx_array biases /* may be null */,
    mlx_optional_int group_size,
    mlx_optional_int bits,
    const char* mode,
    const mlx_array global_scale /* may be null */,
    mlx_optional_dtype dtype,
    const mlx_stream s);

/**
 * Extract diagonal from a 2d array or create a diagonal matrix.
 */
int mlx_diag(mlx_array* res, const mlx_array a, int k, const mlx_stream s);

/**
 * Extract a diagonal or construct a diagonal array
 */
int mlx_diagonal(
    mlx_array* res,
    const mlx_array a,
    int offset,
    int axis1,
    int axis2,
    const mlx_stream s);

/**
 * Return the n-th discrete difference along an axis.
 */
int mlx_diff(
    mlx_array* res,
    const mlx_array a,
    int n,
    int axis,
    const mlx_stream s);

/**
 * Divide two arrays.
 */
int mlx_divide(
    mlx_array* res,
    const mlx_array a,
    const mlx_array b,
    const mlx_stream s);

/**
 * Compute the element-wise quotient and remainder.
 */
int mlx_divmod(
    mlx_vector_array* res,
    const mlx_array a,
    const mlx_array b,
    const mlx_stream s);

/**
 * Evaluate an Einstein summation over the input arrays.
 */
int mlx_einsum(
    mlx_array* res,
    const char* subscripts,
    const mlx_vector_array operands,
    const mlx_stream s);

/**
 * Returns the bool array with (a == b) element-wise.
 */
int mlx_equal(
    mlx_array* res,
    const mlx_array a,
    const mlx_array b,
    const mlx_stream s);

/**
 * Computes the error function of the elements of an array.
 */
int mlx_erf(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Computes the inverse error function of the elements of an array.
 */
int mlx_erfinv(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Exponential of the elements of an array.
 */
int mlx_exp(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Add a singleton dimension at the given axes.
 */
int mlx_expand_dims_axes(
    mlx_array* res,
    const mlx_array a,
    const int* axes,
    size_t axes_num,
    const mlx_stream s);

/**
 * Add a singleton dimension at the given axis.
 */
int mlx_expand_dims(
    mlx_array* res,
    const mlx_array a,
    int axis,
    const mlx_stream s);

/**
 * Computes the expm1 function of the elements of an array.
 */
int mlx_expm1(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Fill an array of the given shape (n,m) with ones in the specified diagonal
 * k, and zeros everywhere else.
 */
int mlx_eye(
    mlx_array* res,
    int n,
    int m,
    int k,
    mlx_dtype dtype,
    const mlx_stream s);

/**
 * Flatten the dimensions in the range `[start_axis, end_axis]` .
 */
int mlx_flatten(
    mlx_array* res,
    const mlx_array a,
    int start_axis,
    int end_axis,
    const mlx_stream s);

/**
 * Reverse the order of the elements along the given axes.
 */
int mlx_flip(
    mlx_array* res,
    const mlx_array a,
    const int* axes,
    size_t axes_num,
    const mlx_stream s);

/**
 * Reverse the order of the elements along the given axis.
 */
int mlx_flip_axis(
    mlx_array* res,
    const mlx_array a,
    int axis,
    const mlx_stream s);

/**
 * Reverse the order of the elements along all axes.
 */
int mlx_flip_all(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Floor the element of an array.
 */
int mlx_floor(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Compute integer division. Equivalent to doing floor(a / x).
 */
int mlx_floor_divide(
    mlx_array* res,
    const mlx_array a,
    const mlx_array b,
    const mlx_stream s);

/**
 * Convert an E4M3 float8 to the given floating point dtype.
 */
int mlx_from_fp8(
    mlx_array* res,
    const mlx_array x,
    mlx_dtype dtype,
    const mlx_stream s);

/**
 * Fill an array of the given shape with the given value(s).
 */
int mlx_full(
    mlx_array* res,
    const int* shape,
    size_t shape_num,
    const mlx_array vals,
    mlx_dtype dtype,
    const mlx_stream s);

/**
 * Return an array with the same shape as the input and filled with a value.
 */
int mlx_full_like(
    mlx_array* res,
    const mlx_array a,
    const mlx_array vals,
    mlx_dtype dtype,
    const mlx_stream s);

/**
 * Gather array entries given indices and slices
 */
int mlx_gather(
    mlx_array* res,
    const mlx_array a,
    const mlx_vector_array indices,
    const int* axes,
    size_t axes_num,
    const int* slice_sizes,
    size_t slice_sizes_num,
    const mlx_stream s);

/**
 * Gather values from an array using indices along one axis.
 */
int mlx_gather_single(
    mlx_array* res,
    const mlx_array a,
    const mlx_array indices,
    int axis,
    const int* slice_sizes,
    size_t slice_sizes_num,
    const mlx_stream s);

/**
 * Compute matrix product with matrix-level gather
 */
int mlx_gather_mm(
    mlx_array* res,
    const mlx_array a,
    const mlx_array b,
    const mlx_array lhs_indices /* may be null */,
    const mlx_array rhs_indices /* may be null */,
    bool sorted_indices,
    const mlx_stream s);

/**
 * Compute matrix products with matrix-level gather.
 */
int mlx_gather_qmm(
    mlx_array* res,
    const mlx_array x,
    const mlx_array w,
    const mlx_array scales,
    const mlx_array biases /* may be null */,
    const mlx_array lhs_indices /* may be null */,
    const mlx_array rhs_indices /* may be null */,
    bool transpose,
    mlx_optional_int group_size,
    mlx_optional_int bits,
    const char* mode,
    bool sorted_indices,
    const mlx_stream s);

/**
 * Returns bool array with (a > b) element-wise.
 */
int mlx_greater(
    mlx_array* res,
    const mlx_array a,
    const mlx_array b,
    const mlx_stream s);

/**
 * Returns bool array with (a >= b) element-wise.
 */
int mlx_greater_equal(
    mlx_array* res,
    const mlx_array a,
    const mlx_array b,
    const mlx_stream s);

/**
 * Multiply the array by the Hadamard matrix of corresponding size.
 */
int mlx_hadamard_transform(
    mlx_array* res,
    const mlx_array a,
    mlx_optional_float scale,
    const mlx_stream s);

/**
 * Returns the Hamming window of size M.
 */
int mlx_hamming(mlx_array* res, int M, const mlx_stream s);

/**
 * Returns the Hanning window of size M.
 */
int mlx_hanning(mlx_array* res, int M, const mlx_stream s);

/**
 * Create a square matrix of shape (n,n) of zeros, and ones in the major
 * diagonal.
 */
int mlx_identity(mlx_array* res, int n, mlx_dtype dtype, const mlx_stream s);

/**
 * Return the imaginary component of each element.
 */
int mlx_imag(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Compute the inner product of two vectors.
 */
int mlx_inner(
    mlx_array* res,
    const mlx_array a,
    const mlx_array b,
    const mlx_stream s);

/**
 * Returns a boolean array where two arrays are element-wise equal within the
 * specified tolerance.
 */
int mlx_isclose(
    mlx_array* res,
    const mlx_array a,
    const mlx_array b,
    double rtol,
    double atol,
    bool equal_nan,
    const mlx_stream s);

/**
 * Return a boolean array indicating finite elements.
 */
int mlx_isfinite(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Return a boolean array indicating infinite elements.
 */
int mlx_isinf(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Return a boolean array indicating NaN elements.
 */
int mlx_isnan(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Return a boolean array indicating negative infinite elements.
 */
int mlx_isneginf(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Return a boolean array indicating positive infinite elements.
 */
int mlx_isposinf(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Compute the Kronecker product of two arrays.
 */
int mlx_kron(
    mlx_array* res,
    const mlx_array a,
    const mlx_array b,
    const mlx_stream s);

/**
 * Shift bits to the left.
 */
int mlx_left_shift(
    mlx_array* res,
    const mlx_array a,
    const mlx_array b,
    const mlx_stream s);

/**
 * Returns bool array with (a
 * <
 * b) element-wise.
 */
int mlx_less(
    mlx_array* res,
    const mlx_array a,
    const mlx_array b,
    const mlx_stream s);

/**
 * Returns bool array with (a
 * <
 * = b) element-wise.
 */
int mlx_less_equal(
    mlx_array* res,
    const mlx_array a,
    const mlx_array b,
    const mlx_stream s);

/**
 * A 1D array of `num` evenly spaced numbers in the range `[start, stop]`
 */
int mlx_linspace(
    mlx_array* res,
    double start,
    double stop,
    int num,
    mlx_dtype dtype,
    const mlx_stream s);

/**
 * Natural logarithm of the elements of an array.
 */
int mlx_log(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Log base 10 of the elements of an array.
 */
int mlx_log10(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Natural logarithm of one plus elements in the array: `log(1 + a)`.
 */
int mlx_log1p(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Log base 2 of the elements of an array.
 */
int mlx_log2(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Log-add-exp of one elements in the array: `log(exp(a) + exp(b))`.
 */
int mlx_logaddexp(
    mlx_array* res,
    const mlx_array a,
    const mlx_array b,
    const mlx_stream s);

/**
 * Cumulative logsumexp of an array along the given axis.
 */
int mlx_logcumsumexp(
    mlx_array* res,
    const mlx_array a,
    int axis,
    bool reverse,
    bool inclusive,
    const mlx_stream s);

/**
 * Logical and of two arrays
 */
int mlx_logical_and(
    mlx_array* res,
    const mlx_array a,
    const mlx_array b,
    const mlx_stream s);

/**
 * Logical not of an array
 */
int mlx_logical_not(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Logical or of two arrays
 */
int mlx_logical_or(
    mlx_array* res,
    const mlx_array a,
    const mlx_array b,
    const mlx_stream s);

/**
 * Logical exclusive or of two arrays
 */
int mlx_logical_xor(
    mlx_array* res,
    const mlx_array a,
    const mlx_array b,
    const mlx_stream s);

/**
 * The logsumexp of the elements of an array along the given axes.
 */
int mlx_logsumexp_axes(
    mlx_array* res,
    const mlx_array a,
    const int* axes,
    size_t axes_num,
    bool keepdims,
    const mlx_stream s);

/**
 * The logsumexp of the elements of an array along the given axis.
 */
int mlx_logsumexp_axis(
    mlx_array* res,
    const mlx_array a,
    int axis,
    bool keepdims,
    const mlx_stream s);

/**
 * The logsumexp of all elements of the array.
 */
int mlx_logsumexp(
    mlx_array* res,
    const mlx_array a,
    bool keepdims,
    const mlx_stream s);

/**
 * Scatter values into an array where the mask is true.
 */
int mlx_masked_scatter(
    mlx_array* res,
    const mlx_array a,
    const mlx_array mask,
    const mlx_array src,
    const mlx_stream s);

/**
 * Matrix-matrix multiplication.
 */
int mlx_matmul(
    mlx_array* res,
    const mlx_array a,
    const mlx_array b,
    const mlx_stream s);

/**
 * The maximum of the elements of an array along the given axes.
 */
int mlx_max_axes(
    mlx_array* res,
    const mlx_array a,
    const int* axes,
    size_t axes_num,
    bool keepdims,
    const mlx_stream s);

/**
 * The maximum of the elements of an array along the given axis.
 */
int mlx_max_axis(
    mlx_array* res,
    const mlx_array a,
    int axis,
    bool keepdims,
    const mlx_stream s);

/**
 * The maximum of all elements of the array.
 */
int mlx_max(
    mlx_array* res,
    const mlx_array a,
    bool keepdims,
    const mlx_stream s);

/**
 * Element-wise maximum between two arrays.
 */
int mlx_maximum(
    mlx_array* res,
    const mlx_array a,
    const mlx_array b,
    const mlx_stream s);

/**
 * Computes the mean of the elements of an array along the given axes
 */
int mlx_mean_axes(
    mlx_array* res,
    const mlx_array a,
    const int* axes,
    size_t axes_num,
    bool keepdims,
    const mlx_stream s);

/**
 * Computes the mean of the elements of an array along the given axis
 */
int mlx_mean_axis(
    mlx_array* res,
    const mlx_array a,
    int axis,
    bool keepdims,
    const mlx_stream s);

/**
 * Computes the mean of the elements of an array.
 */
int mlx_mean(
    mlx_array* res,
    const mlx_array a,
    bool keepdims,
    const mlx_stream s);

/**
 * Computes the median of the elements of an array along the given axes
 */
int mlx_median(
    mlx_array* res,
    const mlx_array a,
    const int* axes,
    size_t axes_num,
    bool keepdims,
    const mlx_stream s);

/**
 * A vector of coordinate arrays from coordinate vectors.
 */
int mlx_meshgrid(
    mlx_vector_array* res,
    const mlx_vector_array arrays,
    bool sparse,
    const char* indexing,
    const mlx_stream s);

/**
 * The minimum of the elements of an array along the given axes.
 */
int mlx_min_axes(
    mlx_array* res,
    const mlx_array a,
    const int* axes,
    size_t axes_num,
    bool keepdims,
    const mlx_stream s);

/**
 * The minimum of the elements of an array along the given axis.
 */
int mlx_min_axis(
    mlx_array* res,
    const mlx_array a,
    int axis,
    bool keepdims,
    const mlx_stream s);

/**
 * The minimum of all elements of the array.
 */
int mlx_min(
    mlx_array* res,
    const mlx_array a,
    bool keepdims,
    const mlx_stream s);

/**
 * Element-wise minimum between two arrays.
 */
int mlx_minimum(
    mlx_array* res,
    const mlx_array a,
    const mlx_array b,
    const mlx_stream s);

/**
 * Move an axis of an array.
 */
int mlx_moveaxis(
    mlx_array* res,
    const mlx_array a,
    int source,
    int destination,
    const mlx_stream s);

/**
 * Multiply two arrays.
 */
int mlx_multiply(
    mlx_array* res,
    const mlx_array a,
    const mlx_array b,
    const mlx_stream s);

/**
 * Replace NaN and infinities with finite numbers.
 */
int mlx_nan_to_num(
    mlx_array* res,
    const mlx_array a,
    float nan,
    mlx_optional_float posinf,
    mlx_optional_float neginf,
    const mlx_stream s);

/**
 * Negate an array.
 */
int mlx_negative(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Returns the bool array with (a != b) element-wise.
 */
int mlx_not_equal(
    mlx_array* res,
    const mlx_array a,
    const mlx_array b,
    const mlx_stream s);

/**
 * Extract the number of elements along some axes as a scalar array. Used to
 * allow shape dependent shapeless compilation (pun intended).
 */
int mlx_number_of_elements(
    mlx_array* res,
    const mlx_array a,
    const int* axes,
    size_t axes_num,
    bool inverted,
    mlx_dtype dtype,
    const mlx_stream s);

/**
 * Fill an array of the given shape with ones.
 */
int mlx_ones(
    mlx_array* res,
    const int* shape,
    size_t shape_num,
    mlx_dtype dtype,
    const mlx_stream s);

/**
 * Return an array of ones with the same shape and dtype as the input.
 */
int mlx_ones_like(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Compute the outer product of two vectors.
 */
int mlx_outer(
    mlx_array* res,
    const mlx_array a,
    const mlx_array b,
    const mlx_stream s);

/**
 * Pad an array with a constant value
 */
int mlx_pad(
    mlx_array* res,
    const mlx_array a,
    const int* axes,
    size_t axes_num,
    const int* low_pad_size,
    size_t low_pad_size_num,
    const int* high_pad_size,
    size_t high_pad_size_num,
    const mlx_array pad_value,
    const char* mode,
    const mlx_stream s);

/**
 * Pad an array with the same padding width before and after each axis.
 */
int mlx_pad_symmetric(
    mlx_array* res,
    const mlx_array a,
    int pad_width,
    const mlx_array pad_value,
    const char* mode,
    const mlx_stream s);

/**
 * Returns a partitioned copy of the array along a given axis
 * such that the smaller kth elements are first.
 */
int mlx_partition_axis(
    mlx_array* res,
    const mlx_array a,
    int kth,
    int axis,
    const mlx_stream s);

/**
 * Returns a partitioned copy of the flattened array
 * such that the smaller kth elements are first.
 */
int mlx_partition(
    mlx_array* res,
    const mlx_array a,
    int kth,
    const mlx_stream s);

/**
 * Unary plus — return a copy of the array unchanged.
 */
int mlx_positive(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Raise elements of a to the power of b element-wise
 */
int mlx_power(
    mlx_array* res,
    const mlx_array a,
    const mlx_array b,
    const mlx_stream s);

/**
 * The product of the elements of an array along the given axes.
 */
int mlx_prod_axes(
    mlx_array* res,
    const mlx_array a,
    const int* axes,
    size_t axes_num,
    bool keepdims,
    const mlx_stream s);

/**
 * The product of the elements of an array along the given axis.
 */
int mlx_prod_axis(
    mlx_array* res,
    const mlx_array a,
    int axis,
    bool keepdims,
    const mlx_stream s);

/**
 * The product of all elements of the array.
 */
int mlx_prod(
    mlx_array* res,
    const mlx_array a,
    bool keepdims,
    const mlx_stream s);

/**
 * Put the values into the array at the given indices along the axis
 */
int mlx_put_along_axis(
    mlx_array* res,
    const mlx_array a,
    const mlx_array indices,
    const mlx_array values,
    int axis,
    const mlx_stream s);

/**
 * Quantized matrix multiplication with optional scales, biases, and transpose.
 */
int mlx_qqmm(
    mlx_array* res,
    const mlx_array x,
    const mlx_array w,
    const mlx_array w_scales /* may be null */,
    mlx_optional_int group_size,
    mlx_optional_int bits,
    const char* mode,
    const mlx_array global_scale_x /* may be null */,
    const mlx_array global_scale_w /* may be null */,
    const mlx_stream s);

/**
 * Quantize a matrix along its last axis
 */
int mlx_quantize(
    mlx_vector_array* res,
    const mlx_array w,
    mlx_optional_int group_size,
    mlx_optional_int bits,
    const char* mode,
    const mlx_array global_scale /* may be null */,
    const mlx_stream s);

/**
 * Quantized matmul multiplies x with a quantized matrix w
 */
int mlx_quantized_matmul(
    mlx_array* res,
    const mlx_array x,
    const mlx_array w,
    const mlx_array scales,
    const mlx_array biases /* may be null */,
    bool transpose,
    mlx_optional_int group_size,
    mlx_optional_int bits,
    const char* mode,
    const mlx_stream s);

/**
 * Convert the elements of an array from Degrees to Radians
 */
int mlx_radians(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Return the real component of each element.
 */
int mlx_real(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * The reciprocal (1/x) of the elements in an array.
 */
int mlx_reciprocal(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Compute the element-wise remainder of division
 */
int mlx_remainder(
    mlx_array* res,
    const mlx_array a,
    const mlx_array b,
    const mlx_stream s);

/**
 * Repeat an array along an axis.
 */
int mlx_repeat_axis(
    mlx_array* res,
    const mlx_array arr,
    int repeats,
    int axis,
    const mlx_stream s);

/**
 * Repeat each element of the array.
 */
int mlx_repeat(
    mlx_array* res,
    const mlx_array arr,
    int repeats,
    const mlx_stream s);

/**
 * Reshape an array to the given shape.
 */
int mlx_reshape(
    mlx_array* res,
    const mlx_array a,
    const int* shape,
    size_t shape_num,
    const mlx_stream s);

/**
 * Shift bits to the right.
 */
int mlx_right_shift(
    mlx_array* res,
    const mlx_array a,
    const mlx_array b,
    const mlx_stream s);

/**
 * Roll array elements by shifts along one axis.
 */
int mlx_roll_axis(
    mlx_array* res,
    const mlx_array a,
    const int* shift,
    size_t shift_num,
    int axis,
    const mlx_stream s);

/**
 * Roll array elements by shifts along the given axes.
 */
int mlx_roll_axes(
    mlx_array* res,
    const mlx_array a,
    const int* shift,
    size_t shift_num,
    const int* axes,
    size_t axes_num,
    const mlx_stream s);

/**
 * Roll flattened array elements by shifts.
 */
int mlx_roll(
    mlx_array* res,
    const mlx_array a,
    const int* shift,
    size_t shift_num,
    const mlx_stream s);

/**
 * Round a floating point number
 */
int mlx_round(
    mlx_array* res,
    const mlx_array a,
    int decimals,
    const mlx_stream s);

/**
 * Square root and reciprocal the elements of an array.
 */
int mlx_rsqrt(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Scatter updates to the given indices.
 * The parameters ``indices`` and ``axes`` determine the locations of ``a``
 * that are updated with the values in ``updates``. Assuming 1-d ``indices``
 * for simplicity, ``indices[i]`` are the indices on axis ``axes[i]`` to which
 * the values in ``updates`` will be applied. Note each array in
 * ``indices`` is assigned to a corresponding axis and hence ``indices.size() ==
 * axes.size()``. If an index/axis pair is not provided then indices along that
 * axis are assumed to be zero.
 * Note the rank of ``updates`` must be equal to the sum of the rank of the
 * broadcasted ``indices`` and the rank of ``a``. In other words, assuming the
 * arrays in ``indices`` have the same shape, ``updates.ndim() ==
 * indices[0].ndim() + a.ndim()``. The leading dimensions of ``updates``
 * correspond to the indices, and the remaining ``a.ndim()`` dimensions are the
 * values that will be applied to the given location in ``a``.
 * For example:
 *
 * will produce:
 *
 * This scatters the two-element row vector ``[1, 2]`` starting at the ``(2,
 * 0)`` position of ``a``.
 * Adding another element to ``indices`` will scatter into another location of
 * ``a``. We also have to add an another update for the new index:
 *
 * will produce:
 *
 * To control the scatter location on an additional axis, add another index
 * array to ``indices`` and another axis to ``axes``:
 *
 * will produce:
 *
 * Items in indices are broadcasted together. This means:
 *
 * is equivalent to:
 *
 * Note, ``scatter`` does not perform bounds checking on the indices and
 * updates.  Out-of-bounds accesses on ``a`` are undefined and typically result
 * in unintended or invalid memory writes.
 */
int mlx_scatter(
    mlx_array* res,
    const mlx_array a,
    const mlx_vector_array indices,
    const mlx_array updates,
    const int* axes,
    size_t axes_num,
    const mlx_stream s);

/**
 * Scatter updates into an array using indices along one axis.
 */
int mlx_scatter_single(
    mlx_array* res,
    const mlx_array a,
    const mlx_array indices,
    const mlx_array updates,
    int axis,
    const mlx_stream s);

/**
 * Scatter and add updates to given indices
 */
int mlx_scatter_add(
    mlx_array* res,
    const mlx_array a,
    const mlx_vector_array indices,
    const mlx_array updates,
    const int* axes,
    size_t axes_num,
    const mlx_stream s);

/**
 * Add updates into an array using scatter indices along one axis.
 */
int mlx_scatter_add_single(
    mlx_array* res,
    const mlx_array a,
    const mlx_array indices,
    const mlx_array updates,
    int axis,
    const mlx_stream s);

/**
 * Add the values into the array at the given indices along the axis
 */
int mlx_scatter_add_axis(
    mlx_array* res,
    const mlx_array a,
    const mlx_array indices,
    const mlx_array values,
    int axis,
    const mlx_stream s);

/**
 * Scatter and max updates to given linear indices
 */
int mlx_scatter_max(
    mlx_array* res,
    const mlx_array a,
    const mlx_vector_array indices,
    const mlx_array updates,
    const int* axes,
    size_t axes_num,
    const mlx_stream s);

/**
 * Take the maximum with updates scattered along one axis.
 */
int mlx_scatter_max_single(
    mlx_array* res,
    const mlx_array a,
    const mlx_array indices,
    const mlx_array updates,
    int axis,
    const mlx_stream s);

/**
 * Scatter and min updates to given linear indices
 */
int mlx_scatter_min(
    mlx_array* res,
    const mlx_array a,
    const mlx_vector_array indices,
    const mlx_array updates,
    const int* axes,
    size_t axes_num,
    const mlx_stream s);

/**
 * Take the minimum with updates scattered along one axis.
 */
int mlx_scatter_min_single(
    mlx_array* res,
    const mlx_array a,
    const mlx_array indices,
    const mlx_array updates,
    int axis,
    const mlx_stream s);

/**
 * Scatter and prod updates to given indices
 */
int mlx_scatter_prod(
    mlx_array* res,
    const mlx_array a,
    const mlx_vector_array indices,
    const mlx_array updates,
    const int* axes,
    size_t axes_num,
    const mlx_stream s);

/**
 * Multiply updates into an array using scatter indices along one axis.
 */
int mlx_scatter_prod_single(
    mlx_array* res,
    const mlx_array a,
    const mlx_array indices,
    const mlx_array updates,
    int axis,
    const mlx_stream s);

/**
 * Compute a matrix product but segment the inner dimension and write the
 * result separately for each segment.
 */
int mlx_segmented_mm(
    mlx_array* res,
    const mlx_array a,
    const mlx_array b,
    const mlx_array segments,
    const mlx_stream s);

/**
 * Element-wise logistic sigmoid of the array: `1 / (1 + exp(-x)`.
 */
int mlx_sigmoid(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * The sign of the elements in an array.
 */
int mlx_sign(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Sine of the elements of an array
 */
int mlx_sin(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Hyperbolic Sine of the elements of an array
 */
int mlx_sinh(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Slice an array.
 */
int mlx_slice(
    mlx_array* res,
    const mlx_array a,
    const int* start,
    size_t start_num,
    const int* stop,
    size_t stop_num,
    const int* strides,
    size_t strides_num,
    const mlx_stream s);

/**
 * Slice an array with dynamic starting indices.
 */
int mlx_slice_dynamic(
    mlx_array* res,
    const mlx_array a,
    const mlx_array start,
    const int* axes,
    size_t axes_num,
    const int* slice_size,
    size_t slice_size_num,
    const mlx_stream s);

/**
 * Update a slice from the source array.
 */
int mlx_slice_update(
    mlx_array* res,
    const mlx_array src,
    const mlx_array update,
    const int* start,
    size_t start_num,
    const int* stop,
    size_t stop_num,
    const int* strides,
    size_t strides_num,
    const mlx_stream s);

/**
 * Update a slice from the source array with dynamic starting indices.
 */
int mlx_slice_update_dynamic(
    mlx_array* res,
    const mlx_array src,
    const mlx_array update,
    const mlx_array start,
    const int* axes,
    size_t axes_num,
    const mlx_stream s);

/**
 * Slice update and add updates to given slice.
 */
int mlx_slice_update_add(
    mlx_array* res,
    const mlx_array src,
    const mlx_array update,
    const int* start,
    size_t start_num,
    const int* stop,
    size_t stop_num,
    const int* strides,
    size_t strides_num,
    const mlx_stream s);

/**
 * Slice update and max updates to given slice.
 */
int mlx_slice_update_max(
    mlx_array* res,
    const mlx_array src,
    const mlx_array update,
    const int* start,
    size_t start_num,
    const int* stop,
    size_t stop_num,
    const int* strides,
    size_t strides_num,
    const mlx_stream s);

/**
 * Slice update and min updates to given slice.
 */
int mlx_slice_update_min(
    mlx_array* res,
    const mlx_array src,
    const mlx_array update,
    const int* start,
    size_t start_num,
    const int* stop,
    size_t stop_num,
    const int* strides,
    size_t strides_num,
    const mlx_stream s);

/**
 * Slice update and prod updates to given slice.
 */
int mlx_slice_update_prod(
    mlx_array* res,
    const mlx_array src,
    const mlx_array update,
    const int* start,
    size_t start_num,
    const int* stop,
    size_t stop_num,
    const int* strides,
    size_t strides_num,
    const mlx_stream s);

/**
 * Softmax of an array.
 */
int mlx_softmax_axes(
    mlx_array* res,
    const mlx_array a,
    const int* axes,
    size_t axes_num,
    bool precise,
    const mlx_stream s);

/**
 * Softmax of an array.
 */
int mlx_softmax_axis(
    mlx_array* res,
    const mlx_array a,
    int axis,
    bool precise,
    const mlx_stream s);

/**
 * Softmax of an array.
 */
int mlx_softmax(
    mlx_array* res,
    const mlx_array a,
    bool precise,
    const mlx_stream s);

/**
 * Returns a sorted copy of the array along a given axis.
 * The sort is stable and NaN values are placed at the end.
 */
int mlx_sort_axis(
    mlx_array* res,
    const mlx_array a,
    int axis,
    const mlx_stream s);

/**
 * Returns a sorted copy of the flattened array.
 * The sort is stable and NaN values are placed at the end.
 */
int mlx_sort(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Split an array into sub-arrays along a given axis.
 */
int mlx_split(
    mlx_vector_array* res,
    const mlx_array a,
    int num_splits,
    int axis,
    const mlx_stream s);

/**
 * Split an array at the given section boundaries along an axis.
 */
int mlx_split_sections(
    mlx_vector_array* res,
    const mlx_array a,
    const int* indices,
    size_t indices_num,
    int axis,
    const mlx_stream s);

/**
 * Square root the elements of an array.
 */
int mlx_sqrt(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Square the elements of an array.
 */
int mlx_square(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Remove singleton dimensions at the given axes.
 */
int mlx_squeeze_axes(
    mlx_array* res,
    const mlx_array a,
    const int* axes,
    size_t axes_num,
    const mlx_stream s);

/**
 * Remove singleton dimensions at the given axis.
 */
int mlx_squeeze_axis(
    mlx_array* res,
    const mlx_array a,
    int axis,
    const mlx_stream s);

/**
 * Remove all singleton dimensions.
 */
int mlx_squeeze(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Stack arrays along a new axis.
 */
int mlx_stack_axis(
    mlx_array* res,
    const mlx_vector_array arrays,
    int axis,
    const mlx_stream s);

/**
 * Stack arrays along a new first axis.
 */
int mlx_stack(
    mlx_array* res,
    const mlx_vector_array arrays,
    const mlx_stream s);

/**
 * Computes the standard deviation of the elements of an array along the given
 * axes
 */
int mlx_std_axes(
    mlx_array* res,
    const mlx_array a,
    const int* axes,
    size_t axes_num,
    bool keepdims,
    int ddof,
    const mlx_stream s);

/**
 * Computes the standard deviation of the elements of an array along the given
 * axis
 */
int mlx_std_axis(
    mlx_array* res,
    const mlx_array a,
    int axis,
    bool keepdims,
    int ddof,
    const mlx_stream s);

/**
 * Computes the standard deviation of the elements of an array.
 */
int mlx_std(
    mlx_array* res,
    const mlx_array a,
    bool keepdims,
    int ddof,
    const mlx_stream s);

/**
 * Stop the flow of gradients.
 */
int mlx_stop_gradient(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Subtract two arrays.
 */
int mlx_subtract(
    mlx_array* res,
    const mlx_array a,
    const mlx_array b,
    const mlx_stream s);

/**
 * Sums the elements of an array along the given axes.
 */
int mlx_sum_axes(
    mlx_array* res,
    const mlx_array a,
    const int* axes,
    size_t axes_num,
    bool keepdims,
    const mlx_stream s);

/**
 * Sums the elements of an array along the given axis.
 */
int mlx_sum_axis(
    mlx_array* res,
    const mlx_array a,
    int axis,
    bool keepdims,
    const mlx_stream s);

/**
 * Sums the elements of an array.
 */
int mlx_sum(
    mlx_array* res,
    const mlx_array a,
    bool keepdims,
    const mlx_stream s);

/**
 * Swap two axes of an array.
 */
int mlx_swapaxes(
    mlx_array* res,
    const mlx_array a,
    int axis1,
    int axis2,
    const mlx_stream s);

/**
 * Take array slices at the given indices of the specified axis.
 */
int mlx_take_axis(
    mlx_array* res,
    const mlx_array a,
    const mlx_array indices,
    int axis,
    const mlx_stream s);

/**
 * Take array entries at the given indices treating the array as flattened.
 */
int mlx_take(
    mlx_array* res,
    const mlx_array a,
    const mlx_array indices,
    const mlx_stream s);

/**
 * Take array entries given indices along the axis
 */
int mlx_take_along_axis(
    mlx_array* res,
    const mlx_array a,
    const mlx_array indices,
    int axis,
    const mlx_stream s);

/**
 * Tangent of the elements of an array
 */
int mlx_tan(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Hyperbolic Tangent of the elements of an array
 */
int mlx_tanh(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Contract two arrays over the given axes.
 */
int mlx_tensordot(
    mlx_array* res,
    const mlx_array a,
    const mlx_array b,
    const int* axes_a,
    size_t axes_a_num,
    const int* axes_b,
    size_t axes_b_num,
    const mlx_stream s);

/**
 * Returns a contraction of a and b over multiple dimensions.
 */
int mlx_tensordot_axis(
    mlx_array* res,
    const mlx_array a,
    const mlx_array b,
    int axis,
    const mlx_stream s);

/**
 * Repeat the array according to the given repetitions for each dimension.
 */
int mlx_tile(
    mlx_array* res,
    const mlx_array arr,
    const int* reps,
    size_t reps_num,
    const mlx_stream s);

/**
 * Convert a floating point matrix to E4M3 float8.
 */
int mlx_to_fp8(mlx_array* res, const mlx_array x, const mlx_stream s);

/**
 * Returns topk elements of the array along a given axis.
 */
int mlx_topk_axis(
    mlx_array* res,
    const mlx_array a,
    int k,
    int axis,
    const mlx_stream s);

/**
 * Returns topk elements of the flattened array.
 */
int mlx_topk(mlx_array* res, const mlx_array a, int k, const mlx_stream s);

/**
 * Return the sum along a specified diagonal in the given array.
 */
int mlx_trace(
    mlx_array* res,
    const mlx_array a,
    int offset,
    int axis1,
    int axis2,
    mlx_dtype dtype,
    const mlx_stream s);

/**
 * Permutes the dimensions according to the given axes.
 */
int mlx_transpose_axes(
    mlx_array* res,
    const mlx_array a,
    const int* axes,
    size_t axes_num,
    const mlx_stream s);

/**
 * Permutes the dimensions in reverse order.
 */
int mlx_transpose(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Return a matrix with ones at and below the given diagonal and zeros
 * elsewhere.
 */
int mlx_tri(
    mlx_array* res,
    int n,
    int m,
    int k,
    mlx_dtype type,
    const mlx_stream s);

/**
 * Return the lower triangular part of the array, zeroing entries above the kth
 * diagonal.
 */
int mlx_tril(mlx_array* res, const mlx_array x, int k, const mlx_stream s);

/**
 * Return the upper triangular part of the array, zeroing entries below the kth
 * diagonal.
 */
int mlx_triu(mlx_array* res, const mlx_array x, int k, const mlx_stream s);

/**
 * Truncate the elements of an array towards zero.
 */
int mlx_trunc(mlx_array* res, const mlx_array a, const mlx_stream s);

/**
 * Unflatten the axis to the given shape.
 */
int mlx_unflatten(
    mlx_array* res,
    const mlx_array a,
    int axis,
    const int* shape,
    size_t shape_num,
    const mlx_stream s);

/**
 * Split an array into a sequence of arrays along an axis, removing it.
 */
int mlx_unstack(
    mlx_vector_array* res,
    const mlx_array a,
    int axis,
    const mlx_stream s);

/**
 * Unstack an array along the default axis.
 */
int mlx_unstack_all(
    mlx_vector_array* res,
    const mlx_array a,
    const mlx_stream s);

/**
 * Computes the variance of the elements of an array along the given
 * axes
 */
int mlx_var_axes(
    mlx_array* res,
    const mlx_array a,
    const int* axes,
    size_t axes_num,
    bool keepdims,
    int ddof,
    const mlx_stream s);

/**
 * Computes the variance of the elements of an array along the given
 * axis
 */
int mlx_var_axis(
    mlx_array* res,
    const mlx_array a,
    int axis,
    bool keepdims,
    int ddof,
    const mlx_stream s);

/**
 * Computes the variance of the elements of an array.
 */
int mlx_var(
    mlx_array* res,
    const mlx_array a,
    bool keepdims,
    int ddof,
    const mlx_stream s);

/**
 * Compute a vector dot product along an axis.
 */
int mlx_vecdot(
    mlx_array* res,
    const mlx_array a,
    const mlx_array b,
    int axis,
    const mlx_stream s);

/**
 * Return a view of the array with the given data type.
 */
int mlx_view(
    mlx_array* res,
    const mlx_array a,
    mlx_dtype dtype,
    const mlx_stream s);

/**
 * Select from x or y depending on condition.
 */
int mlx_where(
    mlx_array* res,
    const mlx_array condition,
    const mlx_array x,
    const mlx_array y,
    const mlx_stream s);

/**
 * Fill an array of the given shape with zeros.
 */
int mlx_zeros(
    mlx_array* res,
    const int* shape,
    size_t shape_num,
    mlx_dtype dtype,
    const mlx_stream s);

/**
 * Return an array of zeros with the same shape and dtype as the input.
 */
int mlx_zeros_like(mlx_array* res, const mlx_array a, const mlx_stream s);

/**@}*/

#ifdef __cplusplus
}
#endif

#endif
