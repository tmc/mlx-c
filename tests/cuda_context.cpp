// A foreign cuCtxSetCurrent on a thread MLX has already used must not leave
// MLX working in that context: the next op must run in the device's primary
// context and give the right answer, and destroying the foreign context must
// not break process exit (CUDA graphs are instantiated in the current
// context).
#include <cuda.h>
#include <cstring>
#include "check.h"
#include "mlx/c/mlx.h"

namespace {

CUcontext current() {
  CUcontext c = nullptr;
  CHECK(cuCtxGetCurrent(&c) == CUDA_SUCCESS);
  return c;
}

mlx_array full(float v, mlx_stream s) {
  int shape[] = {64, 64};
  auto val = mlx_array_new_float32(v);
  auto out = mlx_array_new();
  CHECK(mlx_full(&out, shape, 2, val, MLX_FLOAT32, s) == 0);
  CHECK(mlx_array_free(val) == 0);
  return out;
}

// all_equal reports whether every element of a equals v.
bool all_equal(mlx_array a, float v, mlx_stream s) {
  auto val = mlx_array_new_float32(v);
  auto eq = mlx_array_new();
  auto all = mlx_array_new();
  CHECK(mlx_equal(&eq, a, val, s) == 0);
  CHECK(mlx_all(&all, eq, false, s) == 0);
  bool ok = false;
  CHECK(mlx_array_item_bool(&ok, all) == 0);
  mlx_array_free(all);
  mlx_array_free(eq);
  mlx_array_free(val);
  return ok;
}

} // namespace

int main(int argc, char** argv) {
  CHECK(argc == 2);
  bool matmul = std::strcmp(argv[1], "matmul") == 0;
  CHECK(matmul || std::strcmp(argv[1], "ew") == 0);

  auto dev = mlx_device_new_type(MLX_GPU, 0);
  auto s = mlx_stream_new_device(dev);
  CHECK(s.ctx);

  // Use the GPU once, so this thread has made the primary context current.
  auto x = full(1.f, s);
  auto y = mlx_array_new();
  CHECK(mlx_matmul(&y, x, x, s) == 0);
  CHECK(all_equal(y, 64.f, s));
  CUcontext primary = current();
  CUdevice cu_dev;
  CHECK(cuDeviceGet(&cu_dev, 0) == CUDA_SUCCESS);
  CUcontext retained = nullptr;
  CHECK(cuDevicePrimaryCtxRetain(&retained, cu_dev) == CUDA_SUCCESS);
  CHECK(primary == retained);

  // Switch this thread to a context MLX does not own.
  CUcontext other = nullptr;
  CHECK(cuCtxCreate(&other, nullptr, 0, cu_dev) == CUDA_SUCCESS);
  CHECK(cuCtxSetCurrent(other) == CUDA_SUCCESS);
  CHECK(current() == other);

  // 2*64+1 = 129, through cuBLAS or through elementwise kernels only.
  auto a = full(2.f, s);
  auto one = mlx_array_new_float32(1.f);
  auto t = mlx_array_new();
  auto b = mlx_array_new();
  if (matmul) {
    CHECK(mlx_matmul(&t, a, x, s) == 0);
  } else {
    auto k = full(64.f, s);
    CHECK(mlx_multiply(&t, a, k, s) == 0);
    CHECK(mlx_array_free(k) == 0);
  }
  CHECK(mlx_add(&b, t, one, s) == 0);
  CHECK(mlx_array_eval(b) == 0);
  CHECK(mlx_synchronize(s) == 0);
  CHECK(all_equal(b, 129.f, s));
  CHECK(current() == primary);

  CHECK(cuCtxDestroy(other) == CUDA_SUCCESS);
  CHECK(cuCtxSetCurrent(primary) == CUDA_SUCCESS);
  CHECK(cuDevicePrimaryCtxRelease(cu_dev) == CUDA_SUCCESS);
  mlx_array_free(b);
  mlx_array_free(t);
  mlx_array_free(one);
  mlx_array_free(a);
  mlx_array_free(y);
  mlx_array_free(x);
  mlx_stream_free(s);
  mlx_device_free(dev);
  std::cout << "PASS " << argv[1] << "\n";
  return 0;
}
