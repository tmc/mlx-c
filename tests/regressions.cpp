#include "mlx/mlx.h"
#include "mlx/allocator.h"
#include "mlx/memory.h"
#include <cmath>
#include <iostream>
#include <memory>
#include <string>
#include <vector>
using namespace mlx::core;

int assignment() {
  set_default_device(Device::cpu);
  { auto warm = allocator::malloc(4096); allocator::free(warm); }
  auto before = get_active_memory();
  std::weak_ptr<array::Data> tracker;
  {
    array sink({3, 4});
    array key({1, 2});
    auto parts = split(key, 2);
    parts[0].set_data(allocator::malloc(4096));
    tracker = parts[0].data_shared_ptr();
    parts[0] = sink;
    parts[1] = array({5, 6});
  }
  auto after = get_active_memory();
  std::cout << "assignment expired=" << tracker.expired()
            << " active_before=" << before << " active_after=" << after << std::endl;
  return tracker.expired() && before == after ? 0 : 1;
}

int attention() {
  auto gpu = default_stream(Device::gpu);
  std::vector<float> qv(2*32*128, 0.0f), kv(2*4*8192*128, 0.0f);
  std::vector<float> vv(kv.size());
  for (size_t i=0; i<vv.size(); ++i) vv[i] = i < vv.size()/2 ? 1.f : 7.f;
  array q(qv.data(), {2,32,1,128});
  array k(kv.data(), {2,4,8192,128});
  array v(vv.data(), {2,4,8192,128});
  auto fused = fast::scaled_dot_product_attention(q,k,v,1.f,"",{},{},true,gpu);
  auto kr = repeat(k,8,1,gpu), vr = repeat(v,8,1,gpu);
  auto scores = matmul(q,swapaxes(kr,-1,-2,gpu),gpu);
  auto reference = matmul(softmax(scores,-1,true,gpu),vr,gpu);
  eval(fused,reference);
  float error=0;
  for (size_t i=0;i<fused.size();++i)
    error=std::max(error,std::abs(fused.data<float>()[i]-reference.data<float>()[i]));
  std::cout << "attention max_error=" << error << " batch0=" << fused.data<float>()[0]
            << " batch1=" << fused.data<float>()[32*128] << std::endl;
  return error < 1e-4f ? 0 : 1;
}

int scan(bool axis0) {
  auto gpu=default_stream(Device::gpu);
  std::vector<float> values(axis0?144:72,1.f);
  auto input=array(values.data(),{static_cast<int>(values.size())});
  auto x=axis0 ? transpose(reshape(input,{2,2,9,4},gpu),{1,0,3,2},gpu)
               : transpose(reshape(input,{2,9,4},gpu),{0,2,1},gpu);
  auto out=cumsum(x,axis0?0:-1,false,true,gpu);
  // The upstream regression keeps this unrelated allocation live as a canary.
  auto canary=tril(ones({9,9},float32,gpu),0,gpu);
  auto relative=subtract(expand_dims(out,-1,gpu),expand_dims(out,-2,gpu),gpu);
  eval(relative,canary);
  auto expected=tri(9,9,0,float32,Device::cpu);
  bool intact=array_equal(canary,expected,Device::cpu).item<bool>();
  auto ref=cumsum(x,axis0?0:-1,false,true,Device::cpu);
  bool numeric=array_equal(out,ref,Device::cpu).item<bool>();
  std::cout << "scan axis0=" << axis0 << " canary_intact=" << intact << " numeric=" << numeric << std::endl;
  return intact && numeric ? 0 : 1;
}

int main(int argc,char**argv) {
  if(argc!=2) return 2;
  std::string mode=argv[1];
  if(mode=="assignment") return assignment();
  if(mode=="attention") return attention();
  if(mode=="scan") return scan(false);
  if(mode=="scan-axis0") return scan(true);
  return 2;
}
