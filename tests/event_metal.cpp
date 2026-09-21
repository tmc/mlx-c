#define NS_PRIVATE_IMPLEMENTATION
#define MTL_PRIVATE_IMPLEMENTATION
#include <Metal/Metal.hpp>

#include <chrono>
#include <cstring>
#include <future>
#include <memory>

#include "check.h"
#include "mlx/backend/metal/device.h"
#include "mlx/c/mlx.h"
#include "mlx/c/private/stream.h"

using namespace std::chrono_literals;

static std::future<MTL::CommandBufferStatus> completion(mlx_stream stream) {
  auto promise = std::make_shared<std::promise<MTL::CommandBufferStatus>>();
  auto future = promise->get_future();
  auto& encoder =
      mlx::core::metal::get_command_encoder(mlx_stream_get_(stream));
  encoder.get_command_buffer()->addCompletedHandler(
      [promise](MTL::CommandBuffer* buffer) {
        promise->set_value(buffer->status());
      });
  return future;
}

int main(int argc, char** argv) {
  CHECK(argc == 2);
  bool signal_only = std::strcmp(argv[1], "signal_finalize") == 0;
  bool wait_first = std::strcmp(argv[1], "wait_first") == 0;
  bool recorded_pending = std::strcmp(argv[1], "recorded_pending") == 0;
  CHECK(signal_only || wait_first || recorded_pending);
  auto pool = NS::TransferPtr(NS::AutoreleasePool::alloc()->init());
  bool metal = false, cuda = true;
  CHECK(mlx_metal_is_available(&metal) == 0 && metal);
  CHECK(mlx_cuda_is_available(&cuda) == 0 && !cuda);
  auto device = mlx_device_new_type(MLX_GPU, 0);
  auto producer = mlx_stream_new_thread_unsafe(device);
  auto consumer = mlx_stream_new_thread_unsafe(device);
  CHECK(device.ctx && producer.ctx && consumer.ctx);
  CHECK(!mlx_stream_equal(producer, consumer));
  int producer_index = -1, consumer_index = -1;
  CHECK(mlx_stream_get_index(&producer_index, producer) == 0);
  CHECK(mlx_stream_get_index(&consumer_index, consumer) == 0);
  auto& native_device =
      mlx::core::metal::device(mlx_stream_get_(producer).device);
  std::cout << "device=" << native_device.mtl_device()->name()->utf8String()
            << " producer=" << producer_index << " consumer=" << consumer_index
            << "\n";
  auto event = mlx_event_new(producer);
  CHECK(event.ctx);

  if (signal_only) {
    CHECK(mlx_event_signal(event, producer) == 0);
    auto waited =
        std::async(std::launch::async, [&] { return mlx_event_wait(event); });
    auto progress = waited.wait_for(2s);
    // Recovery comes after the oracle and cannot turn a failed oracle into
    // success.
    if (progress != std::future_status::ready) {
      CHECK(mlx_synchronize(producer) == 0);
    }
    CHECK(waited.get() == 0);
    CHECK(progress == std::future_status::ready);
  } else {
    auto gate = NS::TransferPtr(native_device.mtl_device()->newSharedEvent());
    CHECK(gate.get());
    if (recorded_pending) {
      auto& encoder =
          mlx::core::metal::get_command_encoder(mlx_stream_get_(producer));
      encoder.get_command_buffer()->encodeWait(gate.get(), 1);
      CHECK(mlx_event_signal(event, producer) == 0);
      bool signaled = true;
      CHECK(mlx_event_is_signaled(&signaled, event) == 0 && !signaled);
    }
    auto finished = completion(consumer);
    CHECK(mlx_event_wait_stream(event, consumer) == 0);
    auto before = finished.wait_for(100ms);
    if (recorded_pending) {
      gate->setSignaledValue(1);
    } else {
      CHECK(mlx_event_signal(event, producer) == 0);
    }
    auto after = finished.wait_for(2s);
    if (after != std::future_status::ready) {
      CHECK(mlx_synchronize(consumer) == 0);
    }
    CHECK(finished.get() == MTL::CommandBufferStatusCompleted);
    CHECK(before == std::future_status::timeout);
    CHECK(after == std::future_status::ready);
  }
  CHECK(mlx_synchronize(producer) == 0);
  CHECK(mlx_synchronize(consumer) == 0);
  CHECK(mlx_event_free(event) == 0);
  CHECK(mlx_stream_free(consumer) == 0);
  CHECK(mlx_stream_free(producer) == 0);
  CHECK(mlx_device_free(device) == 0);
  std::cout << argv[1] << " passed on two Metal GPU streams\n";
}
