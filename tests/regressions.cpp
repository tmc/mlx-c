#include "mlx/mlx.h"
#include "mlx/allocator.h"
#include "mlx/memory.h"
#include <cmath>
#include <cstring>
#include <filesystem>
#include <fstream>
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

// A lazily loaded tensor whose file shrinks before eval must fail. Without the
// fix a short pread is retried at the same offset, filling the rest of the
// buffer with a repeat of the bytes already read.
int short_read(const std::string& path) {
  set_default_device(Device::cpu);
  std::vector<float> values(4096);
  for (size_t i=0;i<values.size();++i) values[i]=static_cast<float>(i);
  save_safetensors(path,{{"x",array(values.data(),{4096})}});
  auto x=load_safetensors(path).first.at("x");
  auto full=std::filesystem::file_size(path);
  std::filesystem::resize_file(path,full-values.size()*sizeof(float)/2);
  bool failed=false;
  try { eval(x); } catch(const std::exception& e) {
    failed=true;
    std::cout << "short_read error=" << e.what() << std::endl;
  }
  if(!failed) {
    int wrong=0;
    for (size_t i=0;i<values.size();++i) wrong+=x.data<float>()[i]!=values[i];
    std::cout << "short_read eval succeeded, wrong=" << wrong << std::endl;
  }
  std::filesystem::remove(path);
  return failed ? 0 : 1;
}

// An in-memory reader whose offset reads fail, as a Go reader that returns an
// error does. The header is read through the cursor and succeeds.
struct FailingReader : io::Reader {
  std::string bytes;
  size_t pos=0;
  explicit FailingReader(std::string b) : bytes(std::move(b)) {}
  bool is_open() const override { return true; }
  bool good() const override { return pos<=bytes.size(); }
  size_t tell() override { return pos; }
  void seek(int64_t off,std::ios_base::seekdir way) override {
    pos = way==std::ios_base::beg ? off : way==std::ios_base::end ? bytes.size()+off : pos+off;
  }
  void read(char* data,size_t n) override {
    if(pos+n>bytes.size()) throw std::runtime_error("failing reader: eof");
    std::memcpy(data,bytes.data()+pos,n);
    pos+=n;
  }
  void read(char*,size_t,size_t) override {
    throw std::runtime_error("failing reader: read failed");
  }
  std::string label() const override { return "failing reader"; }
};

// An array computed on the GPU from a lazy load whose read fails must fail to
// evaluate. The Metal command buffer that waits on the failed CPU stream
// signals the eval's event on the GPU, before its completion handler attaches
// the error to that event, so a host waiter that wakes in between returns
// without an error.
int derived_error(const std::string& path) {
  std::vector<float> values(256,1.f);
  save_safetensors(path,{{"x",array(values.data(),{256})}});
  std::string bytes;
  {
    std::ifstream in(path,std::ios::binary);
    bytes.assign(std::istreambuf_iterator<char>(in),{});
  }
  std::filesystem::remove(path);
  auto gpu=default_stream(Device::gpu);
  int runs=500, silent=0;
  for(int i=0;i<runs;++i) {
    auto x=load_safetensors(std::make_shared<FailingReader>(bytes),Device::cpu).first.at("x");
    auto y=add(x,array(1.f),gpu);
    try { eval(y); ++silent; } catch(const std::exception&) {}
  }
  std::cout << "derived_error silent=" << silent << "/" << runs << std::endl;
  return silent==0 ? 0 : 1;
}

int main(int argc,char**argv) {
  if(argc<2) return 2;
  std::string mode=argv[1];
  if(mode=="short-read" && argc==3) return short_read(argv[2]);
  if(mode=="derived-error" && argc==3) return derived_error(argv[2]);
  if(mode=="assignment") return assignment();
  if(mode=="attention") return attention();
  if(mode=="scan") return scan(false);
  if(mode=="scan-axis0") return scan(true);
  return 2;
}
