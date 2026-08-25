/* Copyright © 2023-2024 Apple Inc. */

#include <cstring>
#include <map>
#include <mutex>
#include <optional>
#include <utility>

#include "mlx/c/device.h"
#include "mlx/c/error.h"
#include "mlx/c/private/mlx.h"
#include "mlx/c/stream.h"

namespace {

// default_stream(d) is thread-local: it lazily creates a stream whose
// command encoder is registered only in the creating thread, and using that
// stream from any other thread throws "There is no Stream(...) in current
// thread". The C API does not tie stream handles to threads, so its default
// streams are process-global streams created with new_thread_unsafe_stream,
// which registers globally. Callers must serialize work on a stream -- the
// same contract new_thread_unsafe_stream has.
mlx::core::Stream default_stream_for(mlx::core::Device d) {
  static std::mutex mtx;
  static std::map<std::pair<int, int>, mlx::core::Stream> defaults;
  std::lock_guard<std::mutex> lock(mtx);
  auto key = std::make_pair(static_cast<int>(d.type), d.index);
  auto it = defaults.find(key);
  if (it == defaults.end()) {
    it = defaults.emplace(key, mlx::core::new_thread_unsafe_stream(d)).first;
  }
  return it->second;
}

std::mutex& default_override_mutex() {
  static std::mutex mtx;
  return mtx;
}

std::map<std::pair<int, int>, mlx::core::Stream>& default_overrides() {
  static std::map<std::pair<int, int>, mlx::core::Stream> overrides;
  return overrides;
}

mlx::core::Stream effective_default_stream(mlx::core::Device d) {
  auto resolve = [&]() {
    {
      std::lock_guard<std::mutex> lock(default_override_mutex());
      auto& overrides = default_overrides();
      auto it =
          overrides.find(std::make_pair(static_cast<int>(d.type), d.index));
      if (it != overrides.end()) {
        return it->second;
      }
    }
    return default_stream_for(d);
  };
  auto s = resolve();
  // Keep the core-internal (thread-local) default in sync: some ops call
  // default_stream() directly for intermediate computations (e.g.
  // categorical's expand_dims), and without this those nodes would land on a
  // lazily created thread-local stream instead.
  mlx::core::set_default_stream(s);
  return s;
}

} // namespace

int mlx_stream_tostring(mlx_string* str_, mlx_stream stream) {
  try {
    std::ostringstream os;
    os << mlx_stream_get_(stream);
    std::string str = os.str();
    mlx_string_set_(*str_, str);
    return 0;
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
}

extern "C" mlx_stream mlx_stream_new(void) {
  return mlx_stream_new_();
}

extern "C" mlx_stream mlx_stream_new_device(mlx_device dev) {
  // Delegates to the thread-unsafe constructor: stream handles returned by
  // the C API are not tied to the creating thread (see default_stream_for).
  // Keeping one implementation means a future revert of this divergence to
  // upstream's new_stream semantics touches exactly this function, while
  // mlx_stream_new_thread_unsafe_device stays the explicit spelling of the
  // parallelism contract.
  return mlx_stream_new_thread_unsafe_device(dev);
}

extern "C" mlx_stream mlx_stream_new_thread_unsafe_device(mlx_device dev) {
  try {
    return mlx_stream_new_(
        mlx::core::new_thread_unsafe_stream(mlx_device_get_(dev)));
  } catch (std::exception& e) {
    mlx_error(e.what());
    return mlx_stream_new_();
  }
}

extern "C" int mlx_thread_local_stream_new(
    mlx_thread_local_stream* stream,
    mlx_device dev) {
  try {
    *stream = mlx_thread_local_stream_new_(
        mlx::core::new_thread_local_stream(mlx_device_get_(dev)));
  } catch (std::exception& e) {
    *stream = mlx_thread_local_stream{-1, MLX_CPU, 0};
    mlx_error(e.what());
    return 1;
  }
  return 0;
}

extern "C" int mlx_thread_local_stream_resolve(
    mlx_stream* stream,
    const mlx_thread_local_stream* thread_local_stream) {
  try {
    mlx_stream_set_(
        *stream,
        mlx::core::stream_from_thread_local_stream(
            mlx_thread_local_stream_get_(*thread_local_stream)));
  } catch (std::exception& e) {
    mlx_stream_free_(*stream);
    *stream = mlx_stream_new_();
    mlx_error(e.what());
    return 1;
  }
  return 0;
}

extern "C" int mlx_stream_set(mlx_stream* stream, const mlx_stream src) {
  try {
    mlx_stream_set_(*stream, mlx_stream_get_(src));
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
  return 0;
}
extern "C" int mlx_stream_free(mlx_stream stream) {
  try {
    mlx_stream_free_(stream);
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
  return 0;
}
extern "C" bool mlx_stream_equal(mlx_stream lhs, mlx_stream rhs) {
  return mlx_stream_get_(lhs) == mlx_stream_get_(rhs);
}
extern "C" int mlx_stream_get_device(mlx_device* dev, mlx_stream stream) {
  try {
    mlx_device_set_(*dev, mlx_stream_get_(stream).device);
    return 0;
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
}
extern "C" int mlx_stream_get_index(int* index, mlx_stream stream) {
  try {
    *index = mlx_stream_get_(stream).index;
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
  return 0;
}
extern "C" int mlx_synchronize(mlx_stream stream) {
  try {
    mlx::core::synchronize(mlx_stream_get_(stream));
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
  return 0;
}

extern "C" int mlx_synchronize_default(void) {
  try {
    mlx::core::synchronize();
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
  return 0;
}

extern "C" int mlx_thread_local_stream_synchronize(
    const mlx_thread_local_stream* stream) {
  try {
    mlx::core::synchronize(mlx_thread_local_stream_get_(*stream));
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
  return 0;
}

extern "C" int mlx_get_default_stream(mlx_stream* stream, mlx_device dev) {
  try {
    mlx_stream_set_(*stream, effective_default_stream(mlx_device_get_(dev)));
    return 0;
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
}
extern "C" int mlx_set_default_stream(mlx_stream stream) {
  try {
    auto s = mlx_stream_get_(stream);
    {
      std::lock_guard<std::mutex> lock(default_override_mutex());
      default_overrides().insert_or_assign(
          std::make_pair(static_cast<int>(s.device.type), s.device.index), s);
    }
    // Keep the core-internal (thread-local) default in sync for code inside
    // MLX that consults default_stream() directly on this thread.
    mlx::core::set_default_stream(s);
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
  return 0;
}
extern "C" mlx_stream mlx_default_cpu_stream_new(void) {
  try {
    return mlx_stream_new_(
        effective_default_stream(mlx::core::Device::DeviceType::cpu));
  } catch (std::exception& e) {
    mlx_error(e.what());
    return mlx_stream_new_();
  }
}

extern "C" mlx_stream mlx_default_gpu_stream_new(void) {
  try {
    return mlx_stream_new_(
        effective_default_stream(mlx::core::Device::DeviceType::gpu));
  } catch (std::exception& e) {
    mlx_error(e.what());
    return mlx_stream_new_();
  }
}
