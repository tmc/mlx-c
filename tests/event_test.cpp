#include <atomic>
#include <chrono>
#include <cstdlib>
#include <iostream>
#include <string>
#include <thread>

#include "mlx/c/device.h"
#include "mlx/c/error.h"
#include "mlx/c/event.h"
#include "mlx/c/private/stream.h"
#include "mlx/c/stream.h"
#include "mlx/scheduler.h"

namespace {

std::string last_error;

void record_error(const char* message, void*) {
  last_error = message;
}

void check(bool condition, const char* message) {
  if (!condition) {
    std::cerr << message << '\n';
    std::exit(1);
  }
}

mlx_stream new_stream(mlx_device_type type) {
  mlx_device device = mlx_device_new_type(type, 0);
  mlx_stream stream = mlx_stream_new_device(device);
  mlx_device_free(device);
  check(stream.ctx != nullptr, "failed to create stream");
  return stream;
}

void check_invalid() {
  mlx_event empty{nullptr};
  mlx_stream stream{nullptr};
  bool signaled = false;

  last_error.clear();
  check(mlx_event_new(nullptr, stream) != 0, "new accepted a null output");
  check(!last_error.empty(), "new did not report a null output");
  last_error.clear();
  check(mlx_event_new(&empty, stream) != 0, "new accepted an empty stream");
  check(empty.ctx == nullptr, "failed new returned an event");
  check(!last_error.empty(), "new did not report an empty stream");
  last_error.clear();
  check(mlx_event_signal(empty, stream) != 0, "signal accepted empty handles");
  check(!last_error.empty(), "signal did not report empty handles");
  last_error.clear();
  check(
      mlx_event_wait_stream(empty, stream) != 0,
      "stream wait accepted empty handles");
  check(!last_error.empty(), "stream wait did not report empty handles");
  last_error.clear();
  check(mlx_event_wait(empty) != 0, "host wait accepted an empty event");
  check(!last_error.empty(), "host wait did not report an empty event");
  last_error.clear();
  check(
      mlx_event_is_signaled(&signaled, empty) != 0,
      "query accepted an empty event");
  check(!last_error.empty(), "query did not report an empty event");
  last_error.clear();
  check(
      mlx_event_is_signaled(nullptr, empty) != 0,
      "query accepted a null output");
  check(!last_error.empty(), "query did not report a null output");
  check(mlx_event_free(empty) == 0, "free rejected an empty event");
  check(mlx_event_free(empty) == 0, "repeated empty free failed");
}

void check_cpu_ordering() {
  mlx_stream producer = new_stream(MLX_CPU);
  mlx_stream consumer = new_stream(MLX_CPU);
  mlx_event event{nullptr};
  check(mlx_event_new(&event, producer) == 0, "failed to create CPU event");

  bool signaled = true;
  check(
      mlx_event_is_signaled(&signaled, event) == 0,
      "failed to query CPU event");
  check(!signaled, "new event is already signaled");

  std::atomic<int> produced{0};
  std::atomic<int> observed{0};
  mlx::core::scheduler::enqueue(mlx_stream_get_(producer), [&] {
    std::this_thread::sleep_for(std::chrono::milliseconds(1));
    produced.store(1, std::memory_order_release);
  });
  check(mlx_event_signal(event, producer) == 0, "failed to signal CPU event");
  check(
      mlx_event_wait_stream(event, consumer) == 0,
      "failed to wait on CPU stream");
  mlx::core::scheduler::enqueue(mlx_stream_get_(consumer), [&] {
    observed.store(produced.load(std::memory_order_acquire));
  });
  check(mlx_synchronize(consumer) == 0, "failed to synchronize CPU consumer");
  check(observed.load() == 1, "CPU consumer ran before producer");
  check(mlx_event_wait(event) == 0, "CPU host wait failed");
  check(
      mlx_event_is_signaled(&signaled, event) == 0 && signaled,
      "CPU event not signaled");

  check(mlx_event_free(event) == 0, "failed to free CPU event");
  mlx_stream_free(producer);
  mlx_stream_free(consumer);
}

void check_submission_lifetime(mlx_device_type type) {
  mlx_device device = mlx_device_new_type(type, 0);
  bool available = false;
  check(
      mlx_device_is_available(&available, device) == 0,
      "failed to query device");
  mlx_device_free(device);
  if (!available) {
    return;
  }
  mlx_stream producer = new_stream(type);
  mlx_stream consumer = new_stream(type);
  for (int i = 0; i < 1000; i++) {
    mlx_event event{nullptr};
    check(
        mlx_event_new(&event, producer) == 0, "lifetime event creation failed");
    check(mlx_event_signal(event, producer) == 0, "lifetime signal failed");
    check(mlx_event_wait_stream(event, consumer) == 0, "lifetime wait failed");
    check(mlx_event_free(event) == 0, "lifetime free failed");
  }
  check(mlx_synchronize(consumer) == 0, "lifetime consumer synchronize failed");
  mlx_stream_free(producer);
  mlx_stream_free(consumer);
}

void check_gpu_ordering() {
  mlx_device gpu = mlx_device_new_type(MLX_GPU, 0);
  bool available = false;
  check(mlx_device_is_available(&available, gpu) == 0, "failed to query GPU");
  mlx_device_free(gpu);
  if (!available) {
    return;
  }

  mlx_stream producer = new_stream(MLX_GPU);
  mlx_stream consumer = new_stream(MLX_GPU);
  mlx_event ready{nullptr};
  mlx_event done{nullptr};
  check(mlx_event_new(&ready, producer) == 0, "failed to create GPU event");
  check(
      mlx_event_new(&done, consumer) == 0,
      "failed to create GPU completion event");
  check(mlx_event_signal(ready, producer) == 0, "failed to signal GPU event");
  check(
      mlx_event_wait_stream(ready, consumer) == 0,
      "failed to wait on GPU stream");
  check(
      mlx_event_signal(done, consumer) == 0, "failed to signal GPU completion");
  check(mlx_event_free(ready) == 0, "failed to free submitted GPU event");
  check(mlx_event_wait(done) == 0, "GPU event chain did not complete");
  check(mlx_event_free(done) == 0, "failed to free GPU completion event");
  mlx_stream_free(producer);
  mlx_stream_free(consumer);
}

} // namespace

int main() {
  mlx_set_error_handler(record_error, nullptr, nullptr);
  check_invalid();
  check_cpu_ordering();
  check_submission_lifetime(MLX_CPU);
  check_gpu_ordering();
  check_submission_lifetime(MLX_GPU);
  mlx_set_error_handler(nullptr, nullptr, nullptr);
  return 0;
}
