/* Copyright © 2023-2024 Apple Inc. */

#include <cstdint>

#include "mlx/c/error.h"
#include "mlx/c/event.h"

namespace {

struct event_state {
  uint64_t value;
};

event_state* get_event(mlx_event event) {
  return static_cast<event_state*>(event.ctx);
}

int unsupported() {
  mlx_error("mlx_event stream operations require exported MLX Event symbols");
  return 1;
}

} // namespace

extern "C" mlx_event mlx_event_new(void) {
  return mlx_event({new event_state{0}});
}

extern "C" mlx_event mlx_event_new_stream(mlx_stream) {
  unsupported();
  return mlx_event({nullptr});
}

extern "C" int mlx_event_free(mlx_event event) {
  delete get_event(event);
  return 0;
}

extern "C" int mlx_event_is_signaled(bool* res, mlx_event event) {
  if (!event.ctx) {
    mlx_error("expected a non-empty mlx_event");
    return 1;
  }
  *res = false;
  return 0;
}

extern "C" int mlx_event_stream(mlx_stream*, mlx_event) {
  return unsupported();
}

extern "C" int mlx_event_value(uint64_t* res, mlx_event event) {
  auto* state = get_event(event);
  if (!state) {
    mlx_error("expected a non-empty mlx_event");
    return 1;
  }
  *res = state->value;
  return 0;
}

extern "C" int mlx_event_set_value(mlx_event event, uint64_t value) {
  auto* state = get_event(event);
  if (!state) {
    mlx_error("expected a non-empty mlx_event");
    return 1;
  }
  state->value = value;
  return 0;
}

extern "C" int mlx_event_valid(bool* res, mlx_event event) {
  *res = event.ctx != nullptr;
  return 0;
}

extern "C" int mlx_event_signal(mlx_event, mlx_stream) {
  return unsupported();
}

extern "C" int mlx_event_wait(mlx_event) {
  return unsupported();
}

extern "C" int mlx_event_wait_stream(mlx_event, mlx_stream) {
  return unsupported();
}
