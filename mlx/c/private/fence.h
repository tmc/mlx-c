/* Copyright © 2023-2024 Apple Inc. */

#ifndef MLX_FENCE_PRIVATE_H
#define MLX_FENCE_PRIVATE_H

#include "mlx/c/fence.h"
#include "mlx/fence.h"

inline mlx_fence mlx_fence_new_() {
  return mlx_fence({new mlx::core::Fence()});
}

inline mlx_fence mlx_fence_new_(const mlx::core::Fence& f) {
  return mlx_fence({new mlx::core::Fence(f)});
}

inline mlx_fence mlx_fence_new_(mlx::core::Fence&& f) {
  return mlx_fence({new mlx::core::Fence(std::move(f))});
}

inline mlx_fence& mlx_fence_set_(mlx_fence& d, const mlx::core::Fence& f) {
  if (d.ctx) {
    *static_cast<mlx::core::Fence*>(d.ctx) = f;
  } else {
    d.ctx = new mlx::core::Fence(f);
  }
  return d;
}

inline mlx_fence& mlx_fence_set_(mlx_fence& d, mlx::core::Fence&& f) {
  if (d.ctx) {
    *static_cast<mlx::core::Fence*>(d.ctx) = std::move(f);
  } else {
    d.ctx = new mlx::core::Fence(std::move(f));
  }
  return d;
}

inline mlx::core::Fence& mlx_fence_get_(mlx_fence f) {
  if (!f.ctx) {
    throw std::runtime_error("expected a non-empty mlx_fence");
  }
  return *static_cast<mlx::core::Fence*>(f.ctx);
}

inline void mlx_fence_free_(mlx_fence f) {
  if (f.ctx) {
    delete static_cast<mlx::core::Fence*>(f.ctx);
  }
}

#endif
