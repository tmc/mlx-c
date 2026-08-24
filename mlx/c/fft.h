/* Copyright © 2023-2024 Apple Inc.                   */
/*                                                    */
/* This file is auto-generated. Do not edit manually. */
/*                                                    */

#ifndef MLX_FFT_H
#define MLX_FFT_H

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
 * \defgroup fft FFT operations
 */
/**@{*/

typedef enum mlx_fft_norm_ {
  MLX_FFT_NORM_BACKWARD,
  MLX_FFT_NORM_ORTHO,
  MLX_FFT_NORM_FORWARD
} mlx_fft_norm;

/**
 * Compute the one-dimensional Fourier Transform.
 */
int mlx_fft_fft(
    mlx_array* res,
    const mlx_array a,
    int n,
    int axis,
    mlx_fft_norm norm,
    const mlx_stream s);

/**
 * Compute the two-dimensional Fourier Transform.
 */
int mlx_fft_fft2(
    mlx_array* res,
    const mlx_array a,
    const int* n,
    size_t n_num,
    const int* axes,
    size_t axes_num,
    mlx_fft_norm norm,
    const mlx_stream s);

/**
 * Compute the discrete Fourier Transform sample frequencies.
 */
int mlx_fft_fftfreq(mlx_array* res, int n, double d, const mlx_stream s);

/**
 * Compute the n-dimensional Fourier Transform.
 */
int mlx_fft_fftn(
    mlx_array* res,
    const mlx_array a,
    const int* n,
    size_t n_num,
    const int* axes,
    size_t axes_num,
    mlx_fft_norm norm,
    const mlx_stream s);

/**
 * Shift the zero-frequency component to the center of the spectrum along
 * specified axes.
 */
int mlx_fft_fftshift(
    mlx_array* res,
    const mlx_array a,
    const int* axes,
    size_t axes_num,
    const mlx_stream s);

/**
 * Compute the one-dimensional inverse Fourier Transform.
 */
int mlx_fft_ifft(
    mlx_array* res,
    const mlx_array a,
    int n,
    int axis,
    mlx_fft_norm norm,
    const mlx_stream s);

/**
 * Compute the two-dimensional inverse Fourier Transform.
 */
int mlx_fft_ifft2(
    mlx_array* res,
    const mlx_array a,
    const int* n,
    size_t n_num,
    const int* axes,
    size_t axes_num,
    mlx_fft_norm norm,
    const mlx_stream s);

/**
 * Compute the n-dimensional inverse Fourier Transform.
 */
int mlx_fft_ifftn(
    mlx_array* res,
    const mlx_array a,
    const int* n,
    size_t n_num,
    const int* axes,
    size_t axes_num,
    mlx_fft_norm norm,
    const mlx_stream s);

/**
 * The inverse of fftshift along specified axes.
 */
int mlx_fft_ifftshift(
    mlx_array* res,
    const mlx_array a,
    const int* axes,
    size_t axes_num,
    const mlx_stream s);

/**
 * Compute the one-dimensional inverse of `rfft`.
 */
int mlx_fft_irfft(
    mlx_array* res,
    const mlx_array a,
    int n,
    int axis,
    mlx_fft_norm norm,
    const mlx_stream s);

/**
 * Compute the two-dimensional inverse of `rfft2`.
 */
int mlx_fft_irfft2(
    mlx_array* res,
    const mlx_array a,
    const int* n,
    size_t n_num,
    const int* axes,
    size_t axes_num,
    mlx_fft_norm norm,
    const mlx_stream s);

/**
 * Compute the n-dimensional inverse of `rfftn`.
 */
int mlx_fft_irfftn(
    mlx_array* res,
    const mlx_array a,
    const int* n,
    size_t n_num,
    const int* axes,
    size_t axes_num,
    mlx_fft_norm norm,
    const mlx_stream s);

/**
 * Compute the one-dimensional Fourier Transform on a real input.
 */
int mlx_fft_rfft(
    mlx_array* res,
    const mlx_array a,
    int n,
    int axis,
    mlx_fft_norm norm,
    const mlx_stream s);

/**
 * Compute the two-dimensional Fourier Transform on a real input.
 */
int mlx_fft_rfft2(
    mlx_array* res,
    const mlx_array a,
    const int* n,
    size_t n_num,
    const int* axes,
    size_t axes_num,
    mlx_fft_norm norm,
    const mlx_stream s);

/**
 * Compute the discrete Fourier Transform sample frequencies for
 * `rfft`/`irfft`.
 */
int mlx_fft_rfftfreq(mlx_array* res, int n, double d, const mlx_stream s);

/**
 * Compute the n-dimensional Fourier Transform on a real input.
 */
int mlx_fft_rfftn(
    mlx_array* res,
    const mlx_array a,
    const int* n,
    size_t n_num,
    const int* axes,
    size_t axes_num,
    mlx_fft_norm norm,
    const mlx_stream s);

/**@}*/

#ifdef __cplusplus
}
#endif

#endif
