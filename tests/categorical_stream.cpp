#include <functional>
#include <future>
#include <thread>
#include <unordered_set>
#include "check.h"
#include "mlx/c/mlx.h"
#include "mlx/primitives.h"
#include "mlx/random.h"

int main() {
  using namespace mlx::core;
  auto stream = new_thread_unsafe_stream(Device::cpu);
  std::promise<std::thread::id> ready;
  std::promise<void> release;
  auto done = release.get_future();
  std::thread worker([&] {
    CHECK(stream != default_stream(Device::cpu));
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
