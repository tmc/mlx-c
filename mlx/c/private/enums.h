/* Copyright © 2023-2024 Apple Inc. */

#ifndef MLX_ENUMS_PRIVATE_H
#define MLX_ENUMS_PRIVATE_H

#include "mlx/c/array.h"
#include "mlx/c/compile.h"
#include "mlx/c/fast.h"
#include "mlx/c/fft.h"
#include "mlx/mlx.h"

namespace {
#define MLX_ENUM_ASSERT(c_name, cpp_name)                     \
  static_assert(                                              \
      static_cast<int>(c_name) == static_cast<int>(cpp_name), \
      "MLX enum value mismatch: " #c_name)

MLX_ENUM_ASSERT(MLX_COMPILE_MODE_DISABLED, mlx::core::CompileMode::disabled);
MLX_ENUM_ASSERT(
    MLX_COMPILE_MODE_NO_SIMPLIFY,
    mlx::core::CompileMode::no_simplify);
MLX_ENUM_ASSERT(MLX_COMPILE_MODE_NO_FUSE, mlx::core::CompileMode::no_fuse);
MLX_ENUM_ASSERT(MLX_COMPILE_MODE_ENABLED, mlx::core::CompileMode::enabled);

inline mlx_compile_mode mlx_compile_mode_to_c(mlx::core::CompileMode type) {
  static mlx_compile_mode map[] = {
      MLX_COMPILE_MODE_DISABLED,
      MLX_COMPILE_MODE_NO_SIMPLIFY,
      MLX_COMPILE_MODE_NO_FUSE,
      MLX_COMPILE_MODE_ENABLED};
  return map[(int)type];
}
inline mlx::core::CompileMode mlx_compile_mode_to_cpp(mlx_compile_mode type) {
  static mlx::core::CompileMode map[] = {
      mlx::core::CompileMode::disabled,
      mlx::core::CompileMode::no_simplify,
      mlx::core::CompileMode::no_fuse,
      mlx::core::CompileMode::enabled};
  return map[(int)type];
}

MLX_ENUM_ASSERT(MLX_BOOL, mlx::core::Dtype::Val::bool_);
MLX_ENUM_ASSERT(MLX_UINT8, mlx::core::Dtype::Val::uint8);
MLX_ENUM_ASSERT(MLX_UINT16, mlx::core::Dtype::Val::uint16);
MLX_ENUM_ASSERT(MLX_UINT32, mlx::core::Dtype::Val::uint32);
MLX_ENUM_ASSERT(MLX_UINT64, mlx::core::Dtype::Val::uint64);
MLX_ENUM_ASSERT(MLX_INT8, mlx::core::Dtype::Val::int8);
MLX_ENUM_ASSERT(MLX_INT16, mlx::core::Dtype::Val::int16);
MLX_ENUM_ASSERT(MLX_INT32, mlx::core::Dtype::Val::int32);
MLX_ENUM_ASSERT(MLX_INT64, mlx::core::Dtype::Val::int64);
MLX_ENUM_ASSERT(MLX_FLOAT16, mlx::core::Dtype::Val::float16);
MLX_ENUM_ASSERT(MLX_FLOAT32, mlx::core::Dtype::Val::float32);
MLX_ENUM_ASSERT(MLX_FLOAT64, mlx::core::Dtype::Val::float64);
MLX_ENUM_ASSERT(MLX_BFLOAT16, mlx::core::Dtype::Val::bfloat16);
MLX_ENUM_ASSERT(MLX_COMPLEX64, mlx::core::Dtype::Val::complex64);

inline mlx_dtype mlx_dtype_to_c(mlx::core::Dtype type) {
  static mlx_dtype map[] = {
      MLX_BOOL,
      MLX_UINT8,
      MLX_UINT16,
      MLX_UINT32,
      MLX_UINT64,
      MLX_INT8,
      MLX_INT16,
      MLX_INT32,
      MLX_INT64,
      MLX_FLOAT16,
      MLX_FLOAT32,
      MLX_FLOAT64,
      MLX_BFLOAT16,
      MLX_COMPLEX64,
  };
  return map[(int)type.val()];
}
inline mlx::core::Dtype mlx_dtype_to_cpp(mlx_dtype type) {
  static mlx::core::Dtype map[] = {
      mlx::core::bool_,
      mlx::core::uint8,
      mlx::core::uint16,
      mlx::core::uint32,
      mlx::core::uint64,
      mlx::core::int8,
      mlx::core::int16,
      mlx::core::int32,
      mlx::core::int64,
      mlx::core::float16,
      mlx::core::float32,
      mlx::core::float64,
      mlx::core::bfloat16,
      mlx::core::complex64,
  };
  return map[(int)type];
}

MLX_ENUM_ASSERT(MLX_CPU, mlx::core::Device::DeviceType::cpu);
MLX_ENUM_ASSERT(MLX_GPU, mlx::core::Device::DeviceType::gpu);

inline mlx_device_type mlx_device_type_to_c(
    mlx::core::Device::DeviceType type) {
  static mlx_device_type map[] = {MLX_CPU, MLX_GPU};
  return map[(int)type];
}
inline mlx::core::Device::DeviceType mlx_device_type_to_cpp(
    mlx_device_type type) {
  static mlx::core::Device::DeviceType map[] = {
      mlx::core::Device::DeviceType::cpu, mlx::core::Device::DeviceType::gpu};
  return map[(int)type];
}

MLX_ENUM_ASSERT(MLX_FFT_NORM_BACKWARD, mlx::core::fft::FFTNorm::Backward);
MLX_ENUM_ASSERT(MLX_FFT_NORM_ORTHO, mlx::core::fft::FFTNorm::Ortho);
MLX_ENUM_ASSERT(MLX_FFT_NORM_FORWARD, mlx::core::fft::FFTNorm::Forward);

inline mlx_fft_norm mlx_fft_norm_to_c(mlx::core::fft::FFTNorm norm) {
  static mlx_fft_norm map[] = {
      MLX_FFT_NORM_BACKWARD, MLX_FFT_NORM_ORTHO, MLX_FFT_NORM_FORWARD};
  return map[(int)norm];
}
inline mlx::core::fft::FFTNorm mlx_fft_norm_to_cpp(mlx_fft_norm norm) {
  static mlx::core::fft::FFTNorm map[] = {
      mlx::core::fft::FFTNorm::Backward,
      mlx::core::fft::FFTNorm::Ortho,
      mlx::core::fft::FFTNorm::Forward};
  return map[(int)norm];
}

MLX_ENUM_ASSERT(MLX_MATH_MODE_SAFE, mlx::core::MathMode::Safe);
MLX_ENUM_ASSERT(MLX_MATH_MODE_RELAXED, mlx::core::MathMode::Relaxed);
MLX_ENUM_ASSERT(MLX_MATH_MODE_FAST, mlx::core::MathMode::Fast);

inline mlx::core::MathMode mlx_math_mode_to_cpp(mlx_math_mode mode) {
  static mlx::core::MathMode map[] = {
      mlx::core::MathMode::Safe,
      mlx::core::MathMode::Relaxed,
      mlx::core::MathMode::Fast};
  return map[(int)mode];
}

#undef MLX_ENUM_ASSERT
} // namespace

#endif
