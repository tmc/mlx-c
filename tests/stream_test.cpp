#include <cstdlib>
#include <iostream>
#include <string>
#include <thread>

#include "mlx/c/error.h"
#include "mlx/c/stream.h"

namespace {

std::string last_error;

void record_error(const char* message, void*) {
  last_error = message;
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
  last_error.clear();
  mlx_stream stream = mlx_stream_from_thread_local_stream(token);
  check(stream.ctx == nullptr, "invalid token resolved to a stream");
  check(!last_error.empty(), "invalid token did not report an error");

  mlx_device cpu = mlx_device_new_type(MLX_CPU, 0);
  mlx_stream output = mlx_stream_new_device(cpu);
  check(output.ctx != nullptr, "failed to create initial checked output");
  last_error.clear();
  check(
      mlx_stream_from_thread_local(&output, token) != 0,
      "checked resolver accepted an invalid token");
  check(output.ctx == nullptr, "checked resolver retained a valid output");
  check(!last_error.empty(), "checked resolver did not report an error");

  last_error.clear();
  check(
      mlx_thread_local_stream_synchronize(token) != 0,
      "synchronize accepted an invalid token");
  check(!last_error.empty(), "synchronize did not report an error");
  last_error.clear();
  check(
      mlx_synchronize_thread_local(token) != 0,
      "compatibility synchronize accepted an invalid token");
  check(
      !last_error.empty(), "compatibility synchronize did not report an error");
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

  int cpu_count = 0;
  check(
      mlx_device_count(&cpu_count, MLX_CPU) == 0,
      "failed to count CPU devices");
  check(cpu_count > 0, "no CPU device available");

  check_invalid({-1, MLX_CPU, 0});
  check_invalid({0, static_cast<mlx_device_type>(-1), 0});
  check_invalid({0, static_cast<mlx_device_type>(2), 0});
  check_invalid({0, static_cast<mlx_device_type>(1000000), 0});
  check_invalid({0, MLX_CPU, -1});
  check_invalid({0, MLX_CPU, cpu_count});

  int gpu_count = 0;
  check(
      mlx_device_count(&gpu_count, MLX_GPU) == 0,
      "failed to count GPU devices");
  check_invalid({0, MLX_GPU, -1});
  check_invalid({0, MLX_GPU, gpu_count});

  mlx_device cpu = mlx_device_new_type(MLX_CPU, 0);
  mlx_thread_local_stream token = mlx_new_thread_local_stream(cpu);
  check(token.index >= 0, "failed to create thread-local stream token");

  mlx_stream first = mlx_stream_from_thread_local_stream(token);
  mlx_stream second = mlx_stream_from_thread_local_stream(token);
  check(
      first.ctx != nullptr && second.ctx != nullptr, "failed to resolve token");
  check(mlx_stream_equal(first, second), "same-thread resolutions differ");
  check(
      stream_index(first) == stream_index(second),
      "same-thread indices differ");
  check(
      mlx_thread_local_stream_synchronize(token) == 0,
      "CPU synchronize failed");
  mlx_stream_free(first);
  mlx_stream_free(second);

  int thread_indices[2] = {-1, -1};
  std::thread threads[2];
  for (int i = 0; i < 2; i++) {
    threads[i] = std::thread([&, i] {
      mlx_stream stream = mlx_stream_from_thread_local_stream(token);
      if (stream.ctx == nullptr) {
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

  mlx_device_free(cpu);
  mlx_set_error_handler(nullptr, nullptr, nullptr);
  return 0;
}
