#include <cstring>
#include <functional>
#include <future>
#include <thread>
#include <unordered_set>
#include "check.h"
#include "mlx/c/mlx.h"
#include "mlx/primitives.h"
#include "mlx/random.h"

int main(int argc, char** argv) {
  using namespace mlx::core;
  CHECK(argc == 1 || (argc == 2 && std::strcmp(argv[1], "gpu") == 0));
  auto device = argc == 2 ? Device::gpu : Device::cpu;
  auto stream = new_thread_unsafe_stream(device);
  std::promise<std::thread::id> ready;
  std::promise<void> release;
  auto done = release.get_future();
  std::thread worker([&] {
    CHECK(stream != default_stream(device));
    auto logits = array({1.f, 2.f, 3.f, 4.f, 5.f, 6.f}, {2, 3});
    auto out = random::categorical(logits, 1, 4, random::key(0), stream);
    int expanded = 0;
    std::unordered_set<uintptr_t> visited;
    std::function<void(const array&)> visit = [&](const array& a) {
      if (!visited.insert(a.id()).second)
        return;
      if (a.has_primitive()) {
        CHECK(a.primitive().stream() == stream);
        if (a.shape() == Shape{2, 3, 1})
          ++expanded;
      }
      for (const auto& input : a.inputs())
        visit(input);
    };
    visit(out);
    CHECK(expanded > 0);
    out.eval();
    CHECK(out.shape() == Shape({2, 4}));
    ready.set_value(std::this_thread::get_id());
    done.wait();
  });
  auto id = ready.get_future().get();
  CHECK(id != std::this_thread::get_id());
  std::cout << "creator=" << std::this_thread::get_id() << " worker=" << id
            << "\n";
  release.set_value();
  worker.join();
}
