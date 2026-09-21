#include <chrono>
#include <future>
#include <thread>
#include "check.h"
#include "mlx/c/mlx.h"

namespace {
int errors = 0;
void record_error(const char*, void*) {
  ++errors;
}
} // namespace

int main() {
  mlx_set_error_handler(record_error, nullptr, nullptr);
  auto dev = mlx_device_new_type(MLX_CPU, 0);
  auto producer = mlx_stream_new_thread_unsafe(dev);
  auto consumer = mlx_stream_new_thread_unsafe(dev);
  CHECK(dev.ctx && producer.ctx && consumer.ctx);
  auto event = mlx_event_new(producer);
  auto completed = mlx_event_new(consumer);
  CHECK(event.ctx && completed.ctx);
  bool signaled = true;
  CHECK(mlx_event_is_signaled(&signaled, event) == 0 && !signaled);
  CHECK(mlx_event_wait_stream(event, consumer) == 0);
  CHECK(mlx_event_signal(completed, consumer) == 0);
  auto waiting =
      std::async(std::launch::async, [&] { return mlx_event_wait(completed); });
  CHECK(
      waiting.wait_for(std::chrono::milliseconds(100)) ==
      std::future_status::timeout);
  CHECK(mlx_event_signal(event, producer) == 0);
  CHECK(waiting.wait_for(std::chrono::seconds(2)) == std::future_status::ready);
  CHECK(waiting.get() == 0);
  CHECK(mlx_event_is_signaled(&signaled, event) == 0 && signaled);
  CHECK(mlx_synchronize(producer) == 0);
  CHECK(mlx_synchronize(consumer) == 0);
  CHECK(mlx_event_is_signaled(nullptr, event) != 0);
  auto empty = mlx_event{nullptr};
  CHECK(mlx_event_is_signaled(&signaled, empty) != 0);
  CHECK(mlx_event_wait(empty) != 0);
  CHECK(mlx_event_signal(empty, producer) != 0);
  CHECK(mlx_event_wait_stream(empty, consumer) != 0);
  CHECK(!mlx_event_new(mlx_stream{nullptr}).ctx);
  CHECK(errors == 6);
  CHECK(mlx_event_free(completed) == 0);
  CHECK(mlx_event_free(event) == 0);
  CHECK(mlx_event_free(mlx_event{nullptr}) == 0);
  CHECK(mlx_stream_free(consumer) == 0);
  CHECK(mlx_stream_free(producer) == 0);
  CHECK(mlx_device_free(dev) == 0);
}
