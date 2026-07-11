#include <cstdlib>
#include <iostream>
#include <mutex>
#include <set>
#include <string>
#include <thread>
#include <vector>

#include "mlx/c/error.h"
#include "mlx/c/stream.h"

namespace {

std::string last_error;
std::mutex error_mutex;
std::vector<std::string> errors;

void record_error(const char* message, void*) {
  std::lock_guard lock(error_mutex);
  last_error = message;
  errors.emplace_back(message);
}

void clear_errors() {
  std::lock_guard lock(error_mutex);
  last_error.clear();
  errors.clear();
}

bool has_error() {
  std::lock_guard lock(error_mutex);
  return !last_error.empty();
}

void fail(const char* message) {
  std::cerr << message << '\n';
  std::exit(1);
}

void check(bool condition, const char* message) {
  if (!condition) {
    fail(message);
  }
}

void check_invalid(mlx_thread_local_stream token) {
  mlx_device cpu = mlx_device_new_type(MLX_CPU, 0);
  mlx_stream output = mlx_stream_new_device(cpu);
  check(output.ctx != nullptr, "failed to create initial resolver output");

  clear_errors();
  check(
      mlx_thread_local_stream_resolve(&output, &token) != 0,
      "resolver accepted an invalid token");
  check(output.ctx == nullptr, "resolver retained a valid output");
  check(has_error(), "resolver did not report an error");
  clear_errors();
  check(
      mlx_thread_local_stream_synchronize(&token) != 0,
      "synchronize accepted an invalid token");
  check(has_error(), "synchronize did not report an error");
  mlx_device_free(cpu);
}

int stream_index(mlx_stream stream) {
  int index = -1;
  check(
      mlx_stream_get_index(&index, stream) == 0, "failed to get stream index");
  return index;
}

} // namespace

int main() {
  mlx_set_error_handler(record_error, nullptr, nullptr);
  const bool ubsan_safe = std::getenv("MLX_C_UBSAN_SAFE_INPUTS") != nullptr;

  int cpu_count = 0;
  check(
      mlx_device_count(&cpu_count, MLX_CPU) == 0,
      "failed to count CPU devices");
  check(cpu_count > 0, "no CPU device available");

  check_invalid({-1, MLX_CPU, 0});
  if (!ubsan_safe) {
    check_invalid({0, static_cast<mlx_device_type>(-1), 0});
    check_invalid({0, static_cast<mlx_device_type>(2), 0});
    check_invalid({0, static_cast<mlx_device_type>(1000000), 0});
  }
  check_invalid({0, MLX_CPU, -1});
  check_invalid({0, MLX_CPU, cpu_count});

  int gpu_count = 0;
  check(
      mlx_device_count(&gpu_count, MLX_GPU) == 0,
      "failed to count GPU devices");
  check_invalid({0, MLX_GPU, -1});
  check_invalid({0, MLX_GPU, gpu_count});

  mlx_device cpu = mlx_device_new_type(MLX_CPU, 0);
  mlx_thread_local_stream token = {-1, MLX_CPU, -1};
  check(
      mlx_thread_local_stream_new(&token, cpu) == 0,
      "constructor rejected a valid device");
  check(token.index >= 0, "constructor returned an invalid token");
  check(token.device_type == MLX_CPU, "constructor lost device type");
  check(token.device_index == 0, "constructor lost device index");

  mlx_device nonzero_cpu = mlx_device_new_type(MLX_CPU, 7);
  mlx_thread_local_stream nonzero = {-1, MLX_CPU, -1};
  check(
      mlx_thread_local_stream_new(&nonzero, nonzero_cpu) == 0,
      "constructor rejected a nonzero device index");
  check(nonzero.device_index == 7, "constructor lost nonzero device index");
  mlx_device_free(nonzero_cpu);

  mlx_stream first = mlx_stream_new();
  check(
      mlx_thread_local_stream_resolve(&first, &token) == 0,
      "resolver rejected a valid token");
  check(first.ctx != nullptr, "resolver returned an empty stream");
  check(
      mlx_thread_local_stream_synchronize(&token) == 0,
      "synchronize rejected a valid token");

  clear_errors();
  mlx_stream checked = mlx_stream_new();
  check(
      mlx_thread_local_stream_resolve(&checked, nullptr) != 0,
      "resolver accepted a null token");
  check(checked.ctx == nullptr, "null token produced a stream");
  check(has_error(), "null token did not report an error");

  mlx_stream second = mlx_stream_new();
  check(
      mlx_thread_local_stream_resolve(&second, &token) == 0,
      "second resolution failed");
  check(
      first.ctx != nullptr && second.ctx != nullptr, "failed to resolve token");
  check(mlx_stream_equal(first, second), "same-thread resolutions differ");
  check(
      stream_index(first) == stream_index(second),
      "same-thread indices differ");
  check(
      mlx_thread_local_stream_synchronize(&token) == 0,
      "CPU synchronize failed");
  mlx_stream_free(first);
  mlx_stream_free(second);

  int thread_indices[2] = {-1, -1};
  std::thread threads[2];
  for (int i = 0; i < 2; i++) {
    threads[i] = std::thread([&, i] {
      mlx_stream stream = mlx_stream_new();
      if (mlx_thread_local_stream_resolve(&stream, &token) != 0) {
        return;
      }
      thread_indices[i] = stream_index(stream);
      mlx_stream_free(stream);
    });
  }
  for (auto& thread : threads) {
    thread.join();
  }
  check(
      thread_indices[0] >= 0 && thread_indices[1] >= 0,
      "thread resolution failed");
  check(
      thread_indices[0] != thread_indices[1], "cross-thread indices are equal");

  clear_errors();
  const mlx_thread_local_stream invalid_tokens[] = {
      {-1, MLX_CPU, 0},
      {-2, MLX_CPU, 0},
      {0, MLX_CPU, -1},
      {0, MLX_CPU, cpu_count},
      {0, MLX_CPU, cpu_count + 1},
      {0, MLX_GPU, -1},
      {0, MLX_GPU, gpu_count},
      {0, MLX_GPU, gpu_count + 1},
  };
  std::thread error_threads[8];
  for (int i = 0; i < 8; ++i) {
    error_threads[i] = std::thread([&, i] {
      mlx_stream output = mlx_stream_new();
      check(
          mlx_thread_local_stream_resolve(&output, &invalid_tokens[i]) != 0,
          "concurrent resolver accepted an invalid token");
      check(output.ctx == nullptr, "concurrent resolver retained output");
    });
  }
  for (auto& thread : error_threads) {
    thread.join();
  }
  {
    std::lock_guard lock(error_mutex);
    check(errors.size() == 8, "concurrent errors were lost");
    std::set<std::string> distinct(errors.begin(), errors.end());
    check(distinct.size() >= 3, "concurrent errors were not distinguishable");
  }

  mlx_device_free(cpu);
  mlx_set_error_handler(nullptr, nullptr, nullptr);
  return 0;
}
