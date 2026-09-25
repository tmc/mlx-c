#include <functional>
#include <string>
#include <thread>
#include <unordered_set>
#include "check.h"
#include "mlx/c/mlx.h"
#include "mlx/c/private/array.h"
#include "mlx/c/private/stream.h"
#include "mlx/primitives.h"

using namespace mlx::core;

namespace {

// Imports f and applies it to x on the calling thread. Every primitive of the
// result must use one of want; loads counts the lazily loaded constants.
float apply(
    mlx_imported_function f,
    const std::vector<Stream>& want,
    const std::string& mode) {
  auto x = mlx_array_new_(array({3.f, 4.f}));
  auto args = mlx_vector_array_new_value(x);
  auto out = mlx_vector_array_new();
  CHECK(mlx_imported_function_apply(&out, f, args) == 0);
  auto y = mlx_array_new();
  CHECK(mlx_vector_array_get(&y, out, 0) == 0);
  int loads = 0, primitives = 0;
  std::unordered_set<uintptr_t> seen;
  std::function<void(const array&)> visit = [&](const array& a) {
    if (!seen.insert(a.id()).second) return;
    if (a.has_primitive()) {
      auto s = a.primitive().stream();
      bool found = false;
      for (auto w : want) found |= s == w;
      CHECK(found);
      ++primitives;
      if (std::string(a.primitive().name()) == "Load") ++loads;
    }
    for (const auto& in : a.inputs()) visit(in);
  };
  visit(mlx_array_get_(y));
  CHECK(loads > 0 && primitives > loads);
  CHECK(mlx_array_eval(y) == 0);
  const float* values = mlx_array_data_float32(y);
  CHECK(values);
  CHECK(values[0] == (mode == "mixed" ? 5.f : 4.f));
  CHECK(values[1] == (mode == "mixed" ? 8.f : 6.f));
  float first = values[0];
  CHECK(mlx_array_free(y) == 0);
  CHECK(mlx_vector_array_free(out) == 0);
  CHECK(mlx_vector_array_free(args) == 0);
  CHECK(mlx_array_free(x) == 0);
  return first;
}

} // namespace

int main(int argc, char** argv) {
  CHECK(argc == 3);
  std::string mode = argv[2];
  CHECK(mode == "cpu" || mode == "gpu" || mode == "mixed");
  set_default_device(Device::cpu);
  auto cpu = new_stream(Device::cpu);
  set_default_stream(cpu);
  auto operation = mode == "cpu" ? cpu : new_stream(Device::gpu);
  auto constant = array({1.f, 2.f});
  export_function(argv[1], [&](const Args& xs) {
    auto y = add(xs[0], constant, operation);
    return Args{mode == "mixed" ? add(y, constant, cpu) : y};
  }, Args{array({3.f, 4.f})});

  // Through mlx-c, so the global setter recognizes the streams as portable.
  std::vector<Stream> portable;
  for (auto type : {Device::cpu, Device::gpu}) {
    for (int i = 0, n = device_count(type); i < n; ++i) {
      auto d = mlx_device_new_type(type == Device::cpu ? MLX_CPU : MLX_GPU, i);
      auto h = mlx_stream_new_thread_unsafe(d);
      CHECK(mlx_device_free(d) == 0);
      portable.push_back(mlx_stream_get_(h));
      CHECK(mlx_set_default_stream_global(h) == 0);
      CHECK(mlx_stream_free(h) == 0);
    }
  }
  std::string error;
  mlx_set_error_handler([](const char* s, void* p) {
    *static_cast<std::string*>(p) = s;
  }, &error, [](void*) {});

  // A thread with no default of its own imports onto the global defaults,
  // and neither importing nor evaluating there creates a stream.
  auto streams = get_streams().size();
  std::thread importer([&] {
    auto importer_id = std::this_thread::get_id();
    auto bad = mlx_imported_function_new((std::string(argv[1]) + ".missing").c_str());
    CHECK(!bad.ctx && !error.empty());
    error.clear();
    auto imported = mlx_imported_function_new(argv[1]);
    CHECK(imported.ctx && error.empty());
    CHECK(get_streams().size() == streams);
    for (auto s : portable) CHECK(default_stream(s.device) == s);
    std::thread evaluator([&] {
      apply(imported, portable, mode);
      std::cout << "PASS mode=" << mode << " importer=" << importer_id
                << " evaluator=" << std::this_thread::get_id() << std::endl;
    });
    evaluator.join();
    CHECK(mlx_imported_function_free(imported) == 0);
  });
  importer.join();
  CHECK(get_streams().size() == streams);

  // A thread's own defaults are used as for any other operation and are left
  // in place.
  std::thread owner([&] {
    std::vector<Stream> own;
    for (auto s : portable) {
      auto local = new_stream(s.device);
      set_default_stream(local);
      own.push_back(local);
    }
    auto imported = mlx_imported_function_new(argv[1]);
    CHECK(imported.ctx && error.empty());
    for (auto s : own) CHECK(default_stream(s.device) == s);
    apply(imported, own, mode);
    for (auto s : own) CHECK(default_stream(s.device) == s);
    CHECK(mlx_imported_function_free(imported) == 0);
  });
  owner.join();
  mlx_set_error_handler(nullptr, nullptr, nullptr);
}
