/* Copyright © 2023-2024 Apple Inc. */

#include "mlx/c/error.h"
#include "mlx/c/fence.h"
#include "mlx/c/private/array.h"
#include "mlx/c/private/fence.h"
#include "mlx/c/private/stream.h"

extern "C" mlx_fence mlx_fence_new(void) {
  try {
    return mlx_fence_new_();
  } catch (std::exception& e) {
    mlx_error(e.what());
    return mlx_fence({nullptr});
  }
}

extern "C" mlx_fence mlx_fence_new_stream(mlx_stream stream) {
  try {
    return mlx_fence_new_(mlx::core::Fence(mlx_stream_get_(stream)));
  } catch (std::exception& e) {
    mlx_error(e.what());
    return mlx_fence({nullptr});
  }
}

extern "C" int mlx_fence_free(mlx_fence fence) {
  try {
    mlx_fence_free_(fence);
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
  return 0;
}

extern "C" int mlx_fence_update(
    mlx_fence fence,
    mlx_stream stream,
    const mlx_array x,
    bool cross_device) {
  try {
    mlx_fence_get_(fence).update(
        mlx_stream_get_(stream), mlx_array_get_(x), cross_device);
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
  return 0;
}

extern "C" int mlx_fence_wait(
    mlx_fence fence,
    mlx_stream stream,
    const mlx_array x) {
  try {
    mlx_fence_get_(fence).wait(mlx_stream_get_(stream), mlx_array_get_(x));
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
  return 0;
}
