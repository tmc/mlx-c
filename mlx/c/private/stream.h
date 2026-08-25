/* Copyright © 2023-2024 Apple Inc.                   */
/*                                                    */
/* This file is auto-generated. Do not edit manually. */
/*                                                    */

#ifndef MLX_STREAM_PRIVATE_H
#define MLX_STREAM_PRIVATE_H

#include "mlx/c/stream.h"
#include "mlx/mlx.h"

inline mlx_stream mlx_stream_new_() {
  return mlx_stream({nullptr});
}

inline mlx_stream mlx_stream_new_(const mlx::core::Stream& s) {
  return mlx_stream({new mlx::core::Stream(s)});
}

inline mlx_stream mlx_stream_new_(mlx::core::Stream&& s) {
  return mlx_stream({new mlx::core::Stream(std::move(s))});
}

inline mlx_stream& mlx_stream_set_(mlx_stream& d, const mlx::core::Stream& s) {
  if (d.ctx) {
    *static_cast<mlx::core::Stream*>(d.ctx) = s;
  } else {
    d.ctx = new mlx::core::Stream(s);
  }
  return d;
}

inline mlx_stream& mlx_stream_set_(mlx_stream& d, mlx::core::Stream&& s) {
  if (d.ctx) {
    *static_cast<mlx::core::Stream*>(d.ctx) = std::move(s);
  } else {
    d.ctx = new mlx::core::Stream(std::move(s));
  }
  return d;
}

inline mlx::core::Stream& mlx_stream_get_(mlx_stream d) {
  if (!d.ctx) {
    throw std::runtime_error("expected a non-empty mlx_stream");
  }
  return *static_cast<mlx::core::Stream*>(d.ctx);
}

inline void mlx_stream_free_(mlx_stream d) {
  if (d.ctx) {
    delete static_cast<mlx::core::Stream*>(d.ctx);
  }
}

// The descriptor a failed mlx_thread_local_stream_new returns. A negative
// index is not a stream MLX can produce, which is what makes it a sentinel;
// mlx_thread_local_stream_is_valid is the C-visible spelling of this test.
inline mlx_thread_local_stream mlx_thread_local_stream_invalid_() {
  return mlx_thread_local_stream{-1, MLX_CPU, 0};
}

inline mlx_thread_local_stream mlx_thread_local_stream_new_(
    mlx::core::ThreadLocalStream s) {
  return mlx_thread_local_stream{
      static_cast<int>(s.index),
      static_cast<mlx_device_type>(s.device.type),
      static_cast<int>(s.device.index)};
}

// Reconstructs the C++ ThreadLocalStream value from its plain-struct form.
// stream_from_thread_local_stream resolves it to the owning thread's Stream.
inline mlx::core::ThreadLocalStream mlx_thread_local_stream_get_(
    mlx_thread_local_stream d) {
  return mlx::core::ThreadLocalStream(
      d.index,
      mlx::core::Device(
          static_cast<mlx::core::Device::DeviceType>(d.device_type),
          d.device_index));
}

#endif
