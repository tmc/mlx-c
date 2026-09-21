#include <array>
#include <condition_variable>
#include <functional>
#include <future>
#include <mutex>
#include <thread>

#include "check.h"
#include "mlx/c/mlx.h"
#include "mlx/c/private/compile.h"

namespace {

// Only one command runs at a time, but both native owner threads stay alive.
// This separates OS-thread cache identity from stream and function identity.
class Worker {
 public:
  explicit Worker(int owner) : thread_([this, owner] { loop(owner); }) {}
  ~Worker() { stop(); }

  void run(std::function<void()> fn) {
    std::packaged_task<void()> task(std::move(fn));
    auto done = task.get_future();
    {
      std::lock_guard<std::mutex> lock(mutex_);
      CHECK(!stop_ && !task_.valid());
      task_ = std::move(task);
    }
    ready_.notify_one();
    done.get();
  }

  void stop() {
    if (!thread_.joinable()) {
      return;
    }
    {
      std::lock_guard<std::mutex> lock(mutex_);
      stop_ = true;
    }
    ready_.notify_one();
    thread_.join();
  }

 private:
  void loop(int owner);
  std::mutex mutex_;
  std::condition_variable ready_;
  std::packaged_task<void()> task_;
  bool stop_ = false;
  std::thread thread_;
};

thread_local int current_owner = -1;
std::array<std::array<int, 2>, 2> traces{};
mlx_stream stream;

void Worker::loop(int owner) {
  current_owner = owner;
  for (;;) {
    std::packaged_task<void()> task;
    {
      std::unique_lock<std::mutex> lock(mutex_);
      ready_.wait(lock, [this] { return stop_ || task_.valid(); });
      if (stop_) {
        return;
      }
      task = std::move(task_);
    }
    task();
  }
}

int trace(mlx_vector_array* result, mlx_vector_array inputs, void* payload) {
  CHECK(current_owner == 0 || current_owner == 1);
  ++traces[current_owner][*static_cast<int*>(payload)];
  auto input = mlx_array_new();
  auto output = mlx_array_new();
  CHECK(mlx_vector_array_get(&input, inputs, 0) == 0);
  CHECK(mlx_add(&output, input, input, stream) == 0);
  CHECK(mlx_vector_array_set_value(result, output) == 0);
  CHECK(mlx_array_free(output) == 0);
  CHECK(mlx_array_free(input) == 0);
  return 0;
}

void apply(mlx_closure fun) {
  // Fixed shape, dtype, stream and constants: only cache eviction can explain
  // a later trace. Non-scalar inputs avoid constant-scalar specialization.
  const float data[] = {3.f, 7.f};
  const int shape[] = {2};
  auto input = mlx_array_new_data(data, shape, 1, MLX_FLOAT32);
  auto inputs = mlx_vector_array_new_value(input);
  auto outputs = mlx_vector_array_new();
  CHECK(mlx_closure_apply(&outputs, fun, inputs) == 0);
  auto output = mlx_array_new();
  CHECK(mlx_vector_array_get(&output, outputs, 0) == 0);
  CHECK(mlx_array_eval(output) == 0);
  CHECK(mlx_array_size(output) == 2);
  const auto* values = mlx_array_data_float32(output);
  CHECK(values && values[0] == 6.f && values[1] == 14.f);
  CHECK(mlx_array_free(output) == 0);
  CHECK(mlx_vector_array_free(outputs) == 0);
  CHECK(mlx_vector_array_free(inputs) == 0);
  CHECK(mlx_array_free(input) == 0);
}

void expect(int owner, int first, int second) {
  if (traces[owner][0] != first || traces[owner][1] != second) {
    std::cerr << "owner " << owner << " traces=" << traces[owner][0] << ","
              << traces[owner][1] << " expected=" << first << "," << second
              << "\n";
    std::exit(1);
  }
}

} // namespace

int main() {
  mlx_set_error_handler(
      [](const char* message, void*) { std::cerr << message << "\n"; },
      nullptr,
      nullptr);
  auto device = mlx_device_new_type(MLX_CPU, 0);
  CHECK(device.ctx);
  CHECK(mlx_set_default_device(device) == 0);
  stream = mlx_stream_new_thread_unsafe(device);
  CHECK(stream.ctx);
  // Exercise graph caching without requiring JIT fusion or GPU execution.
  CHECK(mlx_set_compile_mode(MLX_COMPILE_MODE_NO_FUSE) == 0);

  auto empty = mlx_compile_cache_new();
  CHECK(!empty.ctx);
  CHECK(mlx_detail_compile_clear_cache(empty) != 0);
  CHECK(mlx_detail_compile_erase(empty, 1) != 0);
  CHECK(mlx_compile_cache_free(empty) == 0);

  auto constructor_cache = mlx_compile_cache_new();
  CHECK(mlx_detail_compile_cache(&constructor_cache) == 0);
  CHECK(constructor_cache.ctx);
  int payload[] = {0, 1};
  // Explicit IDs belong to this fixture, not closure pointer guesses.
  const uintptr_t ids[] = {0x4341434801ULL, 0x4341434802ULL};
  const uint64_t constants[] = {19};
  mlx_closure fun[2];
  for (int i = 0; i < 2; ++i) {
    auto source = mlx_closure_new_func_payload(trace, &payload[i], nullptr);
    fun[i] = mlx_closure_new();
    CHECK(source.ctx);
    CHECK(mlx_detail_compile(&fun[i], source, ids[i], false, constants, 1) == 0);
    CHECK(mlx_closure_free(source) == 0);
  }

  Worker a(0), b(1);
  mlx_compile_cache ha = mlx_compile_cache_new();
  mlx_compile_cache hb = mlx_compile_cache_new();
  a.run([&] {
    CHECK(mlx_detail_compile_cache(&ha) == 0 && ha.ctx);
    for (auto f : fun) {
      apply(f);
      apply(f);
    }
    expect(0, 1, 1); // Positive control: compilation must actually cache.
  });
  b.run([&] {
    CHECK(mlx_detail_compile_cache(&hb) == 0 && hb.ctx);
    for (auto f : fun) {
      apply(f);
      apply(f);
    }
    expect(1, 1, 1);
  });
  std::cout << "positive control: one trace per function per live owner\n";

  // Construction happened on main, invocation on A/B. Main's capture cannot
  // erase their entries, even though closures, shapes and stream are shared.
  CHECK(mlx_detail_compile_clear_cache(constructor_cache) == 0);
  auto apply_a = [&] { for (auto f : fun) apply(f); };
  auto apply_b = apply_a;
  a.run([&] { apply_a(); expect(0, 1, 1); });
  b.run([&] { apply_b(); expect(1, 1, 1); });

  b.run([&] { CHECK(mlx_detail_compile_erase(ha, ids[0]) == 0); });
  a.run([&] { apply_a(); expect(0, 2, 1); });
  b.run([&] { apply_b(); expect(1, 1, 1); });
  b.run([&] { CHECK(mlx_detail_compile_clear_cache(ha) == 0); });
  a.run([&] { apply_a(); expect(0, 3, 2); });
  b.run([&] { apply_b(); expect(1, 1, 1); });
  std::cout << "cross-thread erase/clear: only captured owner retraces\n";

  b.run([&] { CHECK(mlx_compile_cache_free(ha) == 0); ha = {nullptr}; });
  a.run([&] {
    apply_a();
    expect(0, 3, 2); // Freeing a weak holder must not clear its owner's cache.
    CHECK(mlx_detail_compile_cache(&ha) == 0 && ha.ctx);
  });
  b.run([&] { CHECK(mlx_detail_compile_erase(ha, ids[0]) == 0); });
  a.run([&] { apply_a(); expect(0, 4, 2); });

  a.stop();
  // The C API intentionally treats expired captures as no-ops. A core-backed
  // weak-pointer observation distinguishes that from a still-live empty cache.
  CHECK(ha.ctx && mlx_compile_cache_get_(ha).expired());
  b.run([&] {
    CHECK(mlx_detail_compile_erase(ha, ids[0]) == 0);
    CHECK(mlx_detail_compile_clear_cache(ha) == 0);
    apply_b();
    expect(1, 1, 1);
    CHECK(mlx_compile_cache_free(ha) == 0);
    ha = {nullptr};
  });
  b.stop();
  CHECK(hb.ctx && mlx_compile_cache_get_(hb).expired());
  CHECK(mlx_compile_cache_free(hb) == 0);
  CHECK(mlx_compile_cache_free(constructor_cache) == 0);
  for (auto f : fun) CHECK(mlx_closure_free(f) == 0);
  CHECK(mlx_stream_free(stream) == 0);
  CHECK(mlx_device_free(device) == 0);
  std::cout << "free preserves entries; exited owner expires weak capture: pass\n";
}
