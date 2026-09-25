#include <cstring>
#include <future>
#include <thread>
#include "check.h"
#include "mlx/c/mlx.h"

int main(int argc, char** argv) {
  CHECK(argc == 1 || (argc == 2 && std::strcmp(argv[1], "gpu") == 0));
  bool gpu = argc == 2;
  auto local_default =
      gpu ? mlx_default_gpu_stream_new : mlx_default_cpu_stream_new;
  auto global_default = gpu ? mlx_default_gpu_stream_new_global
                            : mlx_default_cpu_stream_new_global;
  auto dev = mlx_device_new_type(gpu ? MLX_GPU : MLX_CPU, 0);
  CHECK(dev.ctx);
  CHECK(mlx_set_default_device(dev) == 0);
  auto local = local_default();
  auto portable = mlx_stream_new_thread_unsafe(dev);
  CHECK(local.ctx && portable.ctx);
  CHECK(mlx_set_default_stream_global(portable) == 0);
  auto global = global_default();
  CHECK(mlx_stream_equal(global, portable));
  CHECK(mlx_set_default_stream(local) == 0);
  std::promise<std::thread::id> ready;
  std::promise<void> release;
  auto done = release.get_future();
  std::thread worker([&] {
    // A thread without a default of its own falls back to the global one
    // instead of creating a stream.
    auto other = local_default();
    CHECK(!mlx_stream_equal(other, local));
    CHECK(mlx_stream_equal(other, portable));
    mlx_stream result = mlx_stream_new();
    CHECK(mlx_get_default_stream_global(&result, dev) == 0);
    CHECK(mlx_stream_equal(result, portable));
    auto mirror = local_default();
    CHECK(mlx_stream_equal(mirror, portable));
    auto input = mlx_array_new_float32(21.f);
    auto output = mlx_array_new();
    CHECK(input.ctx);
    CHECK(mlx_add(&output, input, input, portable) == 0);
    CHECK(mlx_array_eval(output) == 0);
    float value = 0;
    CHECK(mlx_array_item_float32(&value, output) == 0 && value == 42.f);
    CHECK(mlx_array_free(output) == 0);
    CHECK(mlx_array_free(input) == 0);
    CHECK(mlx_synchronize(portable) == 0);
    CHECK(mlx_synchronize_default_global() == 0);
    ready.set_value(std::this_thread::get_id());
    done.wait();
    CHECK(mlx_stream_free(mirror) == 0);
    CHECK(mlx_stream_free(result) == 0);
    CHECK(mlx_stream_free(other) == 0);
  });
  auto id = ready.get_future().get();
  CHECK(id != std::this_thread::get_id());
  std::cout << "creator=" << std::this_thread::get_id() << " worker=" << id
            << "\n";
  auto still_local = local_default();
  CHECK(mlx_stream_equal(still_local, local));
  release.set_value();
  worker.join();
  CHECK(mlx_stream_free(still_local) == 0);
  CHECK(mlx_stream_free(global) == 0);
  CHECK(mlx_stream_free(portable) == 0);
  CHECK(mlx_stream_free(local) == 0);
  CHECK(mlx_device_free(dev) == 0);
}
