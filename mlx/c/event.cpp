/* Copyright © 2023-2024 Apple Inc. */

#include "mlx/c/error.h"
#include "mlx/c/event.h"
#include "mlx/c/private/event.h"
#include "mlx/c/private/stream.h"

extern "C" mlx_event mlx_event_new(void) {
  try {
    return mlx_event_new_();
  } catch (std::exception& e) {
    mlx_error(e.what());
    return mlx_event({nullptr});
  }
}

extern "C" mlx_event mlx_event_new_stream(mlx_stream stream) {
  try {
    return mlx_event_new_(mlx::core::Event(mlx_stream_get_(stream)));
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

extern "C" int mlx_event_is_signaled(bool* res, mlx_event event) {
  try {
    *res = mlx_event_get_(event).is_signaled();
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
  return 0;
}

extern "C" int mlx_event_stream(mlx_stream* res, mlx_event event) {
  try {
    mlx_stream_set_(*res, mlx_event_get_(event).stream());
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
  return 0;
}

extern "C" int mlx_event_value(uint64_t* res, mlx_event event) {
  try {
    *res = mlx_event_get_(event).value();
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
  return 0;
}

extern "C" int mlx_event_set_value(mlx_event event, uint64_t value) {
  try {
    mlx_event_get_(event).set_value(value);
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
  return 0;
}

extern "C" int mlx_event_valid(bool* res, mlx_event event) {
  try {
    *res = mlx_event_get_(event).valid();
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
  return 0;
}

extern "C" int mlx_event_signal(mlx_event event, mlx_stream stream) {
  try {
    mlx_event_get_(event).signal(mlx_stream_get_(stream));
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
    mlx_event_get_(event).wait(mlx_stream_get_(stream));
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
  return 0;
}
