/* Copyright © 2023-2024 Apple Inc.                   */
/*                                                    */
/* This file is auto-generated. Do not edit manually. */
/*                                                    */

#ifndef MLX_STREAM_PRIVATE_H
#define MLX_STREAM_PRIVATE_H

#include "mlx/c/private/enums.h"
#include "mlx/c/stream.h"
#include "mlx/mlx.h"

inline mlx::core::Device mlx_thread_local_stream_device_(
    mlx_thread_local_stream stream) {
  if (stream.index < 0) {
    throw std::invalid_argument(
        "thread-local stream index must be non-negative");
  }
  if (stream.device_type != MLX_CPU && stream.device_type != MLX_GPU) {
    throw std::invalid_argument("invalid thread-local stream device type");
  }
  if (stream.device_index < 0) {
    throw std::invalid_argument(
        "thread-local stream device index must be non-negative");
  }
  auto type = mlx_device_type_to_cpp(stream.device_type);
  if (stream.device_index >= mlx::core::device_count(type)) {
    throw std::invalid_argument(
        "thread-local stream device index out of range");
  }
  return mlx::core::Device(type, stream.device_index);
}

inline mlx_stream mlx_stream_new_() {
  return mlx_stream({nullptr});
}

inline mlx_stream mlx_stream_new_(const mlx::core::Stream& s) {
  return mlx_stream({new mlx::core::Stream(s)});
}

inline mlx_stream mlx_stream_new_(mlx::core::Stream&& s) {
  return mlx_stream({new mlx::core::Stream(std::move(s))});
}

inline mlx_thread_local_stream mlx_thread_local_stream_new_(
    const mlx::core::ThreadLocalStream& s) {
  return mlx_thread_local_stream{
      s.index,
      mlx_device_type_to_c(s.device.type),
      s.device.index,
  };
}

inline mlx_thread_local_stream mlx_thread_local_stream_new_(
    mlx::core::ThreadLocalStream&& s) {
  return mlx_thread_local_stream_new_(s);
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

inline mlx::core::ThreadLocalStream& mlx_thread_local_stream_get_(
    const mlx_thread_local_stream* d) {
  if (!d) {
    throw std::invalid_argument("expected a thread-local stream token");
  }
  auto device = mlx_thread_local_stream_device_(*d);
  static thread_local mlx::core::ThreadLocalStream s(d->index, device);
  s.index = d->index;
  s.device = device;
  return s;
}

inline void mlx_stream_free_(mlx_stream d) {
  if (d.ctx) {
    delete static_cast<mlx::core::Stream*>(d.ctx);
  }
}

#endif
