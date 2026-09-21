/* Copyright © 2023-2024 Apple Inc. */

#ifndef MLX_EVENT_PRIVATE_H
#define MLX_EVENT_PRIVATE_H

#include "mlx/c/event.h"
#include "mlx/event.h"

inline mlx_event mlx_event_new_() {
  return mlx_event({new mlx::core::Event()});
}

inline mlx_event mlx_event_new_(const mlx::core::Event& e) {
  return mlx_event({new mlx::core::Event(e)});
}

inline mlx_event mlx_event_new_(mlx::core::Event&& e) {
  return mlx_event({new mlx::core::Event(std::move(e))});
}

inline mlx_event& mlx_event_set_(mlx_event& d, const mlx::core::Event& e) {
  if (d.ctx) {
    *static_cast<mlx::core::Event*>(d.ctx) = e;
  } else {
    d.ctx = new mlx::core::Event(e);
  }
  return d;
}

inline mlx_event& mlx_event_set_(mlx_event& d, mlx::core::Event&& e) {
  if (d.ctx) {
    *static_cast<mlx::core::Event*>(d.ctx) = std::move(e);
  } else {
    d.ctx = new mlx::core::Event(std::move(e));
  }
  return d;
}

inline mlx::core::Event& mlx_event_get_(mlx_event e) {
  if (!e.ctx) {
    throw std::runtime_error("expected a non-empty mlx_event");
  }
  return *static_cast<mlx::core::Event*>(e.ctx);
}

inline void mlx_event_free_(mlx_event e) {
  if (e.ctx) {
    delete static_cast<mlx::core::Event*>(e.ctx);
  }
}

#endif
