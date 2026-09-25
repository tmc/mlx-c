// Entry points that take no stream reach MLX's thread-local default. With
// the global defaults set, calling them on a fresh thread must not create a
// stream. get_streams() lists every stream ever created, so its size is the
// tripwire; a new entry point that leaks fails here once it is listed.
#include <cstring>
#include <functional>
#include <string>
#include <thread>
#include <vector>
#include "check.h"
#include "mlx/c/mlx.h"
#include "mlx/stream.h"

namespace {

mlx_stream global_cpu;

int square(mlx_vector_array* res, const mlx_vector_array in) {
  auto x = mlx_array_new();
  auto y = mlx_array_new();
  if (mlx_vector_array_get(&x, in, 0) != 0 ||
      mlx_multiply(&y, x, x, global_cpu) != 0 ||
      mlx_vector_array_set_value(res, y) != 0) {
    return 1;
  }
  mlx_array_free(y);
  mlx_array_free(x);
  return 0;
}

// lazy returns an unevaluated array whose primitive uses the global stream.
mlx_array lazy() {
  auto a = mlx_array_new_float32(3.f);
  auto y = mlx_array_new();
  CHECK(mlx_add(&y, a, a, global_cpu) == 0);
  CHECK(mlx_array_free(a) == 0);
  return y;
}

void apply(mlx_closure f) {
  auto x = lazy();
  auto in = mlx_vector_array_new_value(x);
  auto out = mlx_vector_array_new();
  CHECK(mlx_closure_apply(&out, f, in) == 0);
  CHECK(mlx_eval(out) == 0);
  CHECK(mlx_vector_array_free(out) == 0);
  CHECK(mlx_vector_array_free(in) == 0);
  CHECK(mlx_array_free(x) == 0);
}

mlx_stream set_global(mlx_device_type type) {
  auto d = mlx_device_new_type(type, 0);
  auto s = mlx_stream_new_thread_unsafe(d);
  CHECK(s.ctx && mlx_set_default_stream_global(s) == 0);
  CHECK(mlx_device_free(d) == 0);
  return s;
}

size_t streams() {
  return mlx::core::get_streams().size();
}

void tripwire(bool gpu, const char* file) {
  global_cpu = set_global(MLX_CPU);
  if (gpu) {
    CHECK(mlx_stream_free(set_global(MLX_GPU)) == 0);
  }
  auto closure = mlx_closure_new_func(square);
  auto compiled = mlx_closure_new();
  CHECK(mlx_compile(&compiled, closure, false) == 0);
  CHECK(
      mlx_export_function(
          file,
          closure,
          [] {
            auto x = lazy();
            auto v = mlx_vector_array_new_value(x);
            mlx_array_free(x);
            return v;
          }(),
          false) == 0);

  std::vector<std::pair<const char*, std::function<void()>>> cases = {
      {"eval",
       [] {
         auto x = lazy();
         auto v = mlx_vector_array_new_value(x);
         CHECK(mlx_eval(v) == 0);
         mlx_vector_array_free(v);
         mlx_array_free(x);
       }},
      {"async_eval",
       [] {
         auto x = lazy();
         auto v = mlx_vector_array_new_value(x);
         CHECK(mlx_async_eval(v) == 0);
         CHECK(mlx_array_eval(x) == 0);
         mlx_vector_array_free(v);
         mlx_array_free(x);
       }},
      {"array_eval",
       [] {
         auto x = lazy();
         CHECK(mlx_array_eval(x) == 0);
         mlx_array_free(x);
       }},
      {"array_item",
       [] {
         auto x = lazy();
         float v = 0;
         CHECK(mlx_array_item_float32(&v, x) == 0 && v == 6.f);
         mlx_array_free(x);
       }},
      {"array_tostring",
       [] {
         auto x = lazy();
         auto s = mlx_string_new();
         CHECK(mlx_array_tostring(&s, x) == 0);
         mlx_string_free(s);
         mlx_array_free(x);
       }},
      {"synchronize_default", [] { CHECK(mlx_synchronize_default() == 0); }},
      {"synchronize_default_global",
       [] { CHECK(mlx_synchronize_default_global() == 0); }},
      {"default_cpu_stream",
       [] {
         auto s = mlx_default_cpu_stream_new();
         CHECK(mlx_stream_equal(s, global_cpu));
         mlx_stream_free(s);
       }},
      {"get_default_stream",
       [] {
         auto d = mlx_device_new_type(MLX_CPU, 0);
         auto s = mlx_stream_new();
         CHECK(mlx_get_default_stream(&s, d) == 0);
         CHECK(mlx_stream_equal(s, global_cpu));
         mlx_stream_free(s);
         mlx_device_free(d);
       }},
      {"closure_apply", [&] { apply(closure); }},
      {"compiled_apply", [&] { apply(compiled); }},
      {"vjp",
       [&] {
         auto x = lazy();
         auto one = mlx_array_new_float32(1.f);
         auto primals = mlx_vector_array_new_value(x);
         auto cotangents = mlx_vector_array_new_value(one);
         auto out = mlx_vector_array_new();
         auto grads = mlx_vector_array_new();
         CHECK(mlx_vjp(&out, &grads, closure, primals, cotangents) == 0);
         CHECK(mlx_eval(grads) == 0);
         mlx_vector_array_free(grads);
         mlx_vector_array_free(out);
         mlx_vector_array_free(cotangents);
         mlx_vector_array_free(primals);
         mlx_array_free(one);
         mlx_array_free(x);
       }},
      {"import",
       [&] {
         auto f = mlx_imported_function_new(file);
         CHECK(f.ctx);
         auto x = lazy();
         auto in = mlx_vector_array_new_value(x);
         auto out = mlx_vector_array_new();
         CHECK(mlx_imported_function_apply(&out, f, in) == 0);
         CHECK(mlx_eval(out) == 0);
         mlx_vector_array_free(out);
         mlx_vector_array_free(in);
         mlx_array_free(x);
         mlx_imported_function_free(f);
       }},
  };
  if (gpu) {
    cases.push_back({"default_gpu_stream", [] {
                       auto s = mlx_default_gpu_stream_new();
                       mlx_stream_free(s);
                     }});
  }
  for (auto& [name, run] : cases) {
    auto before = streams();
    std::thread t(run);
    t.join();
    auto after = streams();
    std::cout << name << " created=" << after - before << "\n";
    CHECK(after == before);
  }
  mlx_closure_free(compiled);
  mlx_closure_free(closure);
}

// Only the CPU global is set: a GPU default on a fresh thread falls through
// to a stream of its own, and the CPU default still does not.
void never_set() {
  global_cpu = set_global(MLX_CPU);
  auto before = streams();
  std::thread t([] {
    auto c = mlx_default_cpu_stream_new();
    CHECK(mlx_stream_equal(c, global_cpu));
    auto g = mlx_default_gpu_stream_new();
    CHECK(g.ctx && !mlx_stream_equal(g, global_cpu));
    auto a = mlx_array_new_float32(3.f);
    auto y = mlx_array_new();
    CHECK(mlx_add(&y, a, a, g) == 0);
    float v = 0;
    CHECK(mlx_array_item_float32(&v, y) == 0 && v == 6.f);
    mlx_array_free(y);
    mlx_array_free(a);
    mlx_stream_free(g);
    mlx_stream_free(c);
  });
  t.join();
  auto created = streams() - before;
  std::cout << "never_set created=" << created << "\n";
  CHECK(created == 1);
}

// fresh runs a stream-less computation on a new thread and returns how many
// streams that created.
size_t fresh() {
  auto before = streams();
  std::thread t([] {
    auto c = mlx_default_cpu_stream_new();
    CHECK(c.ctx);
    CHECK(!global_cpu.ctx || !mlx_stream_equal(c, global_cpu));
    auto a = mlx_array_new_float32(3.f);
    auto y = mlx_array_new();
    CHECK(mlx_add(&y, a, a, c) == 0);
    float v = 0;
    CHECK(mlx_array_item_float32(&v, y) == 0 && v == 6.f);
    mlx_array_free(y);
    mlx_array_free(a);
    mlx_stream_free(c);
  });
  t.join();
  return streams() - before;
}

// Set, then clear: a fresh thread behaves as it did before any global was
// set, creating the same streams of its own.
void clear() {
  auto baseline = fresh();
  global_cpu = set_global(MLX_CPU);
  CHECK(mlx_clear_default_stream_global() == 0);
  auto created = fresh();
  std::cout << "clear baseline=" << baseline << " created=" << created << "\n";
  CHECK(baseline >= 1 && created == baseline);

  // The global getter no longer returns the cleared stream either; it falls
  // back to the wrapper's own portable default.
  std::thread u([] {
    auto d = mlx_device_new_type(MLX_CPU, 0);
    auto g = mlx_stream_new();
    CHECK(mlx_get_default_stream_global(&g, d) == 0);
    CHECK(g.ctx && !mlx_stream_equal(g, global_cpu));
    mlx_stream_free(g);
    mlx_device_free(d);
  });
  u.join();
}

} // namespace

int main(int argc, char** argv) {
  CHECK(argc >= 2);
  std::string mode = argv[1];
  bool gpu = argc == 4 && std::strcmp(argv[3], "gpu") == 0;
  if (mode == "tripwire") {
    CHECK(argc >= 3);
    tripwire(gpu, argv[2]);
  } else if (mode == "never-set") {
    never_set();
  } else if (mode == "clear") {
    clear();
  } else {
    CHECK(false);
  }
  CHECK(mlx_stream_free(global_cpu) == 0);
  std::cout << "PASS " << mode << "\n";
}
