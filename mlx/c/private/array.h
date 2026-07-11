/* Copyright © 2023-2024 Apple Inc.                   */
/*                                                    */
/* This file is auto-generated. Do not edit manually. */
/*                                                    */

#ifndef MLX_ARRAY_PRIVATE_H
#define MLX_ARRAY_PRIVATE_H

#include "mlx/c/array.h"
#include "mlx/mlx.h"
#include "mlx/c/private/stream_debug_trace.h"

namespace {

void mlx_trace_array_set_event_(mlx_array d) {
#ifdef MLX_STREAM_TRACE_BUILD
  if (d.ctx == nullptr) {
    return;
  }
#if defined(__GNUC__) || defined(__clang__)
  const void* caller_pc = __builtin_return_address(0);
#else
  const void* caller_pc = nullptr;
#endif
  ::mlx::detail::mlx_trace_array_handle_assignment(
      "array_set",
      "mlx_array_set",
      "private_array_set",
      reinterpret_cast<uint64_t>(caller_pc),
      d.ctx,
      -1,
      -1,
      -1);
#else
  (void)d;
#endif
}

void mlx_trace_array_new_event_(mlx_array arr, const char* c_function) {
#ifdef MLX_STREAM_TRACE_BUILD
  if (arr.ctx == nullptr) {
    return;
  }
#if defined(__GNUC__) || defined(__clang__)
  const void* caller_pc = __builtin_return_address(0);
#else
  const void* caller_pc = nullptr;
#endif
  ::mlx::detail::mlx_trace_array_handle_assignment(
      "array_new",
      c_function,
      "private_array_new",
      reinterpret_cast<uint64_t>(caller_pc),
      arr.ctx,
      -1,
      -1,
      -1);
#else
  (void)arr;
  (void)c_function;
#endif
}

}  // namespace

inline mlx_array mlx_array_new_() {
  return mlx_array({nullptr});
}

inline mlx_array mlx_array_new_(const mlx::core::array& s) {
  mlx_array arr({new mlx::core::array(s)});
  mlx_trace_array_new_event_(arr, "mlx_array_new_(const array&)");
  return arr;
}

inline mlx_array mlx_array_new_(mlx::core::array&& s) {
  mlx_array arr({new mlx::core::array(std::move(s))});
  mlx_trace_array_new_event_(arr, "mlx_array_new_(array&&)");
  return arr;
}

inline mlx_array& mlx_array_set_(mlx_array& d, const mlx::core::array& s) {
#ifdef MLX_STREAM_TRACE_BUILD
  std::uintptr_t previous_native_desc = 0;
#endif
  if (d.ctx) {
#ifdef MLX_STREAM_TRACE_BUILD
    previous_native_desc = static_cast<mlx::core::array*>(d.ctx)->id();
#endif
    *static_cast<mlx::core::array*>(d.ctx) = s;
  } else {
    d.ctx = new mlx::core::array(s);
  }
  if (d.ctx) {
#ifdef MLX_STREAM_TRACE_BUILD
    const auto native_desc = static_cast<mlx::core::array*>(d.ctx)->id();
    if (previous_native_desc != native_desc) {
      ::mlx::detail::mlx_trace_array_handle_release(d.ctx);
    }
#endif
    mlx_trace_array_set_event_(d);
  }
  return d;
}

inline mlx_array& mlx_array_set_(mlx_array& d, mlx::core::array&& s) {
#ifdef MLX_STREAM_TRACE_BUILD
  std::uintptr_t previous_native_desc = 0;
#endif
  if (d.ctx) {
#ifdef MLX_STREAM_TRACE_BUILD
    previous_native_desc = static_cast<mlx::core::array*>(d.ctx)->id();
#endif
    *static_cast<mlx::core::array*>(d.ctx) = std::move(s);
  } else {
    d.ctx = new mlx::core::array(std::move(s));
  }
  if (d.ctx) {
#ifdef MLX_STREAM_TRACE_BUILD
    const auto native_desc = static_cast<mlx::core::array*>(d.ctx)->id();
    if (previous_native_desc != native_desc) {
      ::mlx::detail::mlx_trace_array_handle_release(d.ctx);
    }
#endif
    mlx_trace_array_set_event_(d);
  }
  return d;
}

inline mlx::core::array& mlx_array_get_(mlx_array d) {
  if (!d.ctx) {
    throw std::runtime_error("expected a non-empty mlx_array");
  }
  return *static_cast<mlx::core::array*>(d.ctx);
}

inline void mlx_array_free_(mlx_array d) {
  if (d.ctx) {
#ifdef MLX_STREAM_TRACE_BUILD
    ::mlx::detail::mlx_trace_array_handle_release(d.ctx);
#endif
    delete static_cast<mlx::core::array*>(d.ctx);
  }
}

#endif
