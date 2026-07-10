/* Copyright © 2023-2024 Apple Inc. */

#include "mlx/c/event.h"
#include "mlx/backend/gpu/eval.h"
#include "mlx/c/error.h"
#include "mlx/c/private/event.h"
#include "mlx/c/private/stream.h"

extern "C" int mlx_event_new(mlx_event* event, mlx_stream stream) {
  if (!event) {
    mlx_error("expected a non-null mlx_event output");
    return 1;
  }
  try {
    auto value = mlx::core::Event(mlx_stream_get_(stream));
    value.set_value(1);
    mlx_event_set_(*event, std::move(value));
  } catch (std::exception& e) {
    mlx_event_free_(*event);
    *event = mlx_event({nullptr});
    mlx_error(e.what());
    return 1;
  }
  return 0;
}

extern "C" int mlx_event_free(mlx_event event) {
  try {
    mlx_event_free_(event);
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
  return 0;
}

extern "C" int mlx_event_is_signaled(bool* result, mlx_event event) {
  if (!result) {
    mlx_error("expected a non-null bool output");
    return 1;
  }
  try {
    *result = mlx_event_get_(event).is_signaled();
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
  return 0;
}

extern "C" int mlx_event_signal(mlx_event event, mlx_stream stream) {
  try {
    auto& value = mlx_stream_get_(stream);
    mlx_event_get_(event).signal(value);
    if (value.device == mlx::core::Device::gpu) {
      mlx::core::gpu::finalize(value);
    }
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
  return 0;
}

extern "C" int mlx_event_wait(mlx_event event) {
  try {
    mlx_event_get_(event).wait();
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
  return 0;
}

extern "C" int mlx_event_wait_stream(mlx_event event, mlx_stream stream) {
  try {
    auto& value = mlx_stream_get_(stream);
    mlx_event_get_(event).wait(value);
    if (value.device == mlx::core::Device::gpu) {
      mlx::core::gpu::finalize(value);
    }
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
  return 0;
}
