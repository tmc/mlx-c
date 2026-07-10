#include <chrono>
#include <cstdint>
#include <iostream>
#include <vector>

#include "mlx/c/device.h"
#include "mlx/c/event.h"
#include "mlx/c/stream.h"

using clock_type = std::chrono::steady_clock;

int benchmark(mlx_device_type type, const char* name) {
  constexpr int iterations = 1000;
  mlx_device device = mlx_device_new_type(type, 0);
  bool available = false;
  if (mlx_device_is_available(&available, device) != 0) {
    return 1;
  }
  if (!available) {
    mlx_device_free(device);
    return 0;
  }
  mlx_stream producer = mlx_stream_new_device(device);
  mlx_stream consumer = mlx_stream_new_device(device);
  for (int i = 0; i < 100; i++) {
    mlx_event event{nullptr};
    if (mlx_event_new(&event, producer) != 0) {
      return 1;
    }
    mlx_event_free(event);
  }
  std::vector<mlx_event> events(iterations, mlx_event{nullptr});

  auto start = clock_type::now();
  for (auto& event : events) {
    if (mlx_event_new(&event, producer) != 0) {
      return 1;
    }
  }
  auto allocated = clock_type::now();
  for (auto event : events) {
    if (mlx_event_signal(event, producer) != 0 ||
        mlx_event_wait_stream(event, consumer) != 0) {
      return 1;
    }
  }
  auto submitted = clock_type::now();
  if (mlx_synchronize(consumer) != 0) {
    return 1;
  }
  for (auto event : events) {
    mlx_event_free(event);
  }

  auto allocation_ns =
      std::chrono::duration_cast<std::chrono::nanoseconds>(allocated - start)
          .count() /
      iterations;
  auto submission_ns = std::chrono::duration_cast<std::chrono::nanoseconds>(
                           submitted - allocated)
                           .count() /
      iterations;
  std::cout << name << " event allocation: " << allocation_ns << " ns/op\n";
  std::cout << name << " signal+wait_stream submission: " << submission_ns
            << " ns/op\n";

  mlx_stream_free(producer);
  mlx_stream_free(consumer);
  mlx_device_free(device);
  return 0;
}

int main() {
  return benchmark(MLX_CPU, "CPU") || benchmark(MLX_GPU, "GPU");
}
