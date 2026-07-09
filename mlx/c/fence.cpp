/* Copyright © 2023-2024 Apple Inc. */

#include "mlx/c/error.h"
#include "mlx/c/fence.h"

namespace {

struct fence_state {};

fence_state* get_fence(mlx_fence fence) {
  return static_cast<fence_state*>(fence.ctx);
}

int unsupported() {
  mlx_error("mlx_fence operations require exported MLX Fence symbols");
  return 1;
}

} // namespace

extern "C" mlx_fence mlx_fence_new(void) {
  return mlx_fence({new fence_state{}});
}

extern "C" mlx_fence mlx_fence_new_stream(mlx_stream) {
  unsupported();
  return mlx_fence({nullptr});
}

extern "C" int mlx_fence_free(mlx_fence fence) {
  delete get_fence(fence);
  return 0;
}

extern "C" int mlx_fence_update(mlx_fence fence, mlx_stream, const mlx_array, bool) {
  if (!fence.ctx) {
    mlx_error("expected a non-empty mlx_fence");
    return 1;
  }
  return unsupported();
}

extern "C" int mlx_fence_wait(mlx_fence fence, mlx_stream, const mlx_array) {
  if (!fence.ctx) {
    mlx_error("expected a non-empty mlx_fence");
    return 1;
  }
  return unsupported();
}
