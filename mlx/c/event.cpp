/* Copyright © 2023-2024 Apple Inc. */

#include "mlx/c/event.h"
#include "mlx/backend/gpu/eval.h"
#include "mlx/c/error.h"
#include "mlx/c/private/event.h"
#include "mlx/c/private/stream.h"

extern "C" mlx_event mlx_event_new(mlx_stream stream) {
  try {
    auto value = mlx::core::Event(mlx_stream_get_(stream));
    // Signalling is at the event's current value, so an event left at 0 would
    // start out signaled. The C API exposes no way to move the value, which is
    // what makes the event one-shot.
    value.set_value(1);
    return mlx_event_new_(std::move(value));
  } catch (std::exception& e) {
    mlx_error(e.what());
    return mlx_event({nullptr});
  }
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
