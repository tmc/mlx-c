#include <future>
#include <thread>

#include "check.h"
#include "mlx/c/mlx.h"
#include "mlx/c/private/array.h"
#include "mlx/c/private/stream.h"
#include "mlx/primitives.h"

static mlx_stream resolve(mlx_stream_thread_local tls) {
  auto stream = mlx_stream_new();
  CHECK(mlx_stream_from_thread_local(&stream, tls) == 0);
  CHECK(stream.ctx);
  return stream;
}

static void evaluate(mlx_stream stream) {
  auto input = mlx_array_new_float32(21.f);
  auto output = mlx_array_new();
  CHECK(input.ctx);
  CHECK(mlx_add(&output, input, input, stream) == 0);
  // Inspect the lazy primitive before evaluation: a logical TLS index is not
  // a concrete stream index, even though both private types inherit Stream.
  auto& array = mlx_array_get_(output);
  CHECK(array.has_primitive());
  CHECK(array.primitive().stream() == mlx_stream_get_(stream));
  CHECK(mlx_array_eval(output) == 0);
  float value = 0;
  CHECK(mlx_array_item_float32(&value, output) == 0 && value == 42.f);
  CHECK(mlx_array_free(output) == 0);
  CHECK(mlx_array_free(input) == 0);
}

struct Witness {
  std::thread::id thread;
  int stream;
};

int main() {
  auto device = mlx_device_new_type(MLX_CPU, 0);
  CHECK(device.ctx);
  // Separate the concrete and logical index spaces before creating TLS.
  for (int i = 0; i < 3; ++i) {
    auto spare = mlx_stream_new_device(device);
    CHECK(spare.ctx);
    CHECK(mlx_stream_free(spare) == 0);
  }
  auto original = mlx_stream_thread_local_new(device);
  mlx_stream_thread_local copy = {nullptr};
  CHECK(original.ctx);
  CHECK(mlx_stream_thread_local_set(&copy, original) == 0);
  CHECK(copy.ctx && copy.ctx != original.ctx);
  CHECK(
      mlx_stream_thread_local_get_(copy) ==
      mlx_stream_thread_local_get_(original));
  auto assigned = mlx_stream_thread_local_new(device);
  CHECK(assigned.ctx);
  auto holder = assigned.ctx;
  CHECK(mlx_stream_thread_local_set(&assigned, original) == 0);
  CHECK(assigned.ctx == holder && assigned.ctx != original.ctx);
  CHECK(
      mlx_stream_thread_local_get_(assigned) ==
      mlx_stream_thread_local_get_(original));
  CHECK(mlx_stream_thread_local_free(assigned) == 0);
  auto logical = mlx_stream_thread_local_get_(original);
  auto owner = resolve(original);
  auto again = resolve(copy);
  CHECK(mlx_stream_equal(owner, again));
  CHECK(mlx_stream_get_(owner).index != logical.index);
  CHECK(mlx_stream_free(again) == 0);
  CHECK(mlx_stream_thread_local_free(original) == 0);
  original = {nullptr};
  // Freeing the source holder does not invalidate its independently set copy.
  again = resolve(copy);
  CHECK(mlx_stream_equal(owner, again));
  CHECK(mlx_stream_free(again) == 0);

  std::promise<Witness> ready;
  std::promise<void> release;
  auto done = release.get_future();
  std::thread worker([&] {
    auto local = resolve(copy);
    auto repeated = resolve(copy);
    CHECK(mlx_stream_equal(local, repeated));
    int index = -1;
    CHECK(mlx_stream_get_index(&index, local) == 0);
    evaluate(local);
    CHECK(mlx_synchronize_thread_local(copy) == 0);
    ready.set_value({std::this_thread::get_id(), index});
    done.wait();
    CHECK(mlx_stream_free(repeated) == 0);
    CHECK(mlx_stream_free(local) == 0);
  });
  auto witness = ready.get_future().get();
  int owner_index = -1;
  CHECK(mlx_stream_get_index(&owner_index, owner) == 0);
  CHECK(witness.thread != std::this_thread::get_id());
  CHECK(witness.stream != owner_index);
  std::cout << "owner=" << std::this_thread::get_id()
            << " worker=" << witness.thread << " owner_stream=" << owner_index
            << " worker_stream=" << witness.stream << std::endl;
  evaluate(owner);
  CHECK(mlx_synchronize_thread_local(copy) == 0);
  release.set_value();
  worker.join();
  CHECK(mlx_stream_thread_local_free(copy) == 0);
  copy = {nullptr};
  // The concrete stream's backend remains registered after both TLS holders
  // are freed. No freed holder is accessed and no explicit clear is requested.
  evaluate(owner);
  CHECK(mlx_synchronize(owner) == 0);
  CHECK(mlx_stream_free(owner) == 0);
  CHECK(mlx_device_free(device) == 0);
}
