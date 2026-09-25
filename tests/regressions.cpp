#include "mlx/mlx.h"
#include "mlx/c/mlx.h"
#include "mlx/allocator.h"
#include "mlx/memory.h"
#include <chrono>
#include <cmath>
#include <cstring>
#include <filesystem>
#include <future>
#include <fstream>
#include <iostream>
#include <memory>
#include <string>
#include <thread>
#include <vector>
#ifdef __linux__
#include <malloc.h>
#endif
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

// A reader whose offset reads block until a gate opens, or give up after a
// timeout so a failing run still ends.
struct GatedReader : io::Reader {
  std::string bytes;
  size_t pos=0;
  std::shared_future<void> gate;
  GatedReader(std::string b,std::shared_future<void> g) : bytes(std::move(b)),gate(std::move(g)) {}
  bool is_open() const override { return true; }
  bool good() const override { return pos<=bytes.size(); }
  size_t tell() override { return pos; }
  void seek(int64_t off,std::ios_base::seekdir way) override {
    pos = way==std::ios_base::beg ? off : way==std::ios_base::end ? bytes.size()+off : pos+off;
  }
  void read(char* data,size_t n) override {
    if(pos+n>bytes.size()) throw std::runtime_error("gated reader: eof");
    std::memcpy(data,bytes.data()+pos,n);
    pos+=n;
  }
  void read(char* data,size_t n,size_t off) override {
    gate.wait_for(std::chrono::seconds(3));
    std::memcpy(data,bytes.data()+off,n);
  }
  std::string label() const override { return "gated reader"; }
};

// A path load must not wait behind reads through a custom reader that block.
// Eight blocked reads fill the four-thread io pool that every Load used, so
// the path load's read ran only after they gave up.
int reader_pool(const std::string& dir) {
  set_default_device(Device::cpu);
  std::unordered_map<std::string,array> tensors;
  for(int i=0;i<8;++i) tensors.insert({"s"+std::to_string(i),full({16},float(i),float32)});
  auto blocked_path=dir+"/reader_pool_blocked.safetensors";
  auto path=dir+"/reader_pool_path.safetensors";
  save_safetensors(blocked_path,tensors);
  save_safetensors(path,{{"f",full({16},1.f,float32)}});
  std::string bytes;
  {
    std::ifstream in(blocked_path,std::ios::binary);
    bytes.assign(std::istreambuf_iterator<char>(in),{});
  }
  std::promise<void> open;
  auto reader=std::make_shared<GatedReader>(bytes,open.get_future().share());
  auto blocked=load_safetensors(reader,new_stream(Device::cpu)).first;
  auto f=load_safetensors(path).first.at("f");
  std::vector<array> all;
  for(auto& [k,v] : blocked) all.push_back(v);
  async_eval(all);
  std::this_thread::sleep_for(std::chrono::milliseconds(100));
  auto start=std::chrono::steady_clock::now();
  eval(f);
  auto ms=std::chrono::duration_cast<std::chrono::milliseconds>(
      std::chrono::steady_clock::now()-start).count();
  open.set_value();
  eval(all);
  std::filesystem::remove(blocked_path);
  std::filesystem::remove(path);
  std::cout << "reader_pool path_eval_ms=" << ms << std::endl;
  return ms<1000 ? 0 : 1;
}

// Reports whether f raises, counting a success as a failure of the test.
template <typename F>
void must_raise(const char* what,int& bad,F f) {
  try { f(); } catch(const std::exception& e) {
    std::cout << "failed_load " << what << " error=" << e.what() << std::endl;
    return;
  }
  std::cout << "failed_load " << what << " succeeded" << std::endl;
  ++bad;
}

// An in-memory C reader over a one-element npy, for the C API check.
struct CBytes {
  std::string bytes;
  size_t pos=0;
};
bool cbytes_true(void*) { return true; }
size_t cbytes_tell(void* d) { return static_cast<CBytes*>(d)->pos; }
int cbytes_seek(void* d,int64_t off,int whence) {
  auto* b=static_cast<CBytes*>(d);
  b->pos = whence==SEEK_SET ? off : whence==SEEK_END ? b->bytes.size()+off : b->pos+off;
  return 0;
}
size_t cbytes_read(void* d,char* data,size_t n) {
  auto* b=static_cast<CBytes*>(d);
  n=std::min(n,b->bytes.size()-b->pos);
  std::memcpy(data,b->bytes.data()+b->pos,n);
  b->pos+=n;
  return n;
}
// Header reads succeed; a read of the trailing float32 fails.
size_t cbytes_read_at(void* d,char* data,size_t n,size_t off) {
  auto* b=static_cast<CBytes*>(d);
  if(off+n>b->bytes.size()-sizeof(float)) return 0;
  std::memcpy(data,b->bytes.data()+off,n);
  return n;
}
const char* cbytes_label(void*) { return "failing c reader"; }
void cbytes_free(void* d) { delete static_cast<CBytes*>(d); }
void count_error(const char* msg,void* data) {
  ++*static_cast<int*>(data);
  std::cout << "failed_load c error=" << msg << std::endl;
}

// A lazily loaded array whose read fails must keep failing. The first eval
// raises the read error, but the array is then evaluated, so without the fix
// a second eval returns success and its data is uninitialized memory.
int failed_load(const std::string& path) {
  set_default_device(Device::cpu);
  std::vector<float> values(4096,7.f);
  save_safetensors(path,{{"x",array(values.data(),{4096})}});
  std::string bytes;
  {
    std::ifstream in(path,std::ios::binary);
    bytes.assign(std::istreambuf_iterator<char>(in),{});
  }
  std::filesystem::remove(path);
  // Leave a freed buffer of other values for the load's allocation to reuse,
  // so uninitialized data is visible.
  eval(full({4096},-1.f));
  auto load=[&] {
    return load_safetensors(std::make_shared<FailingReader>(bytes),Device::cpu).first.at("x");
  };
  int bad=0;
  auto x=load();
  must_raise("eval",bad,[&] { eval(x); });
  must_raise("re-eval",bad,[&] { eval(x); });
  must_raise("item",bad,[&] { slice(x,{0},{1}).item<float>(); });
  must_raise("data",bad,[&] {
    auto p=x.data<float>();
    int wrong=0;
    for(size_t i=0;i<values.size();++i) wrong+=p[i]!=values[i];
    std::cout << "failed_load data wrong=" << wrong << "/" << values.size() << std::endl;
  });
  must_raise("derived eval",bad,[&] { eval(add(x,array(1.f))); });
  // An array computed in the eval that failed.
  auto y=add(load(),array(1.f));
  must_raise("output eval",bad,[&] { eval(y); });
  must_raise("output re-eval",bad,[&] { eval(y); });
  must_raise("output data",bad,[&] { y.data<float>(); });
  // An array evaluated asynchronously. The error surfaces at wait, or at
  // async_eval itself if the read has already failed when it enqueues.
  auto z=load();
  must_raise("async wait",bad,[&] { async_eval({z}); z.wait(); });
  must_raise("async re-wait",bad,[&] { z.wait(); });

  // An array already evaluated before a failing eval keeps its data.
  auto good=full({4},2.f);
  eval(good);
  must_raise("mixed eval",bad,[&] { eval({good,load()}); });
  try {
    eval(good);
    if(good.data<float>()[3]!=2.f) throw std::runtime_error("wrong value");
  } catch(const std::exception& e) {
    std::cout << "failed_load good array error=" << e.what() << std::endl;
    ++bad;
  }

  // The same through the C API, loading a one-element npy.
  auto npy=path+".npy";
  save(npy,array({7.f}));
  auto* cb=new CBytes;
  {
    std::ifstream in(npy,std::ios::binary);
    cb->bytes.assign(std::istreambuf_iterator<char>(in),{});
  }
  std::filesystem::remove(npy);
  int errors=0;
  // The handler data is kept only with a destructor.
  mlx_set_error_handler(count_error,&errors,[](void*) {});
  mlx_io_vtable vt{cbytes_true,cbytes_true,cbytes_tell,cbytes_seek,cbytes_read,
                   cbytes_read_at,nullptr,cbytes_label,cbytes_free};
  auto reader=mlx_io_reader_new(cb,vt);
  auto s=mlx_default_cpu_stream_new();
  auto c=mlx_array_new();
  if(mlx_load_reader(&c,reader,s)!=0) {
    std::cout << "failed_load c load failed" << std::endl;
    return 1;
  }
  mlx_io_reader_free(reader);
  mlx_stream_free(s);
  int cbad=0;
  if(mlx_array_eval(c)==0) ++cbad, std::cout << "failed_load c eval succeeded" << std::endl;
  if(mlx_array_eval(c)==0) ++cbad, std::cout << "failed_load c re-eval succeeded" << std::endl;
  float v=0;
  if(mlx_array_item_float32(&v,c)==0) ++cbad, std::cout << "failed_load c item succeeded: " << v << std::endl;
  if(mlx_array_data_float32(c)!=nullptr) ++cbad, std::cout << "failed_load c data succeeded" << std::endl;
  mlx_array_free(c);
  mlx_set_error_handler(nullptr,nullptr,nullptr);
  std::cout << "failed_load bad=" << bad << " c_bad=" << cbad << " c_errors=" << errors << std::endl;
  return bad==0 && cbad==0 ? 0 : 1;
}

// A lazy load on a CUDA stream whose read fails must not leak the host buffer
// that Load::eval_gpu mallocs to stage the read.
int load_leak(const std::string& path) {
#ifdef __linux__
  constexpr int n=1<<22; // 16 MiB of float32
  std::vector<float> values(n,1.f);
  save_safetensors(path,{{"x",array(values.data(),{n})}});
  std::string bytes;
  {
    std::ifstream in(path,std::ios::binary);
    bytes.assign(std::istreambuf_iterator<char>(in),{});
  }
  std::filesystem::remove(path);
  auto gpu=default_stream(Device::gpu);
  auto run=[&]{
    auto x=load_safetensors(std::make_shared<FailingReader>(bytes),gpu).first.at("x");
    try { eval(x); } catch(const std::exception&) {}
  };
  auto in_use=[]{ auto m=mallinfo2(); return m.uordblks+m.hblkhd; };
  run();
  auto before=in_use();
  int runs=8;
  for(int i=0;i<runs;++i) run();
  double leaked=(double(in_use())-double(before))/(1<<20);
  std::cout << "load_leak runs=" << runs << " heap_growth_mib=" << leaked << std::endl;
  return leaked < 16 ? 0 : 1;
#else
  return 2;
#endif
}

int main(int argc,char**argv) {
  if(argc<2) return 2;
  std::string mode=argv[1];
  if(mode=="short-read" && argc==3) return short_read(argv[2]);
  if(mode=="derived-error" && argc==3) return derived_error(argv[2]);
  if(mode=="reader-pool" && argc==3) return reader_pool(argv[2]);
  if(mode=="failed-load" && argc==3) return failed_load(argv[2]);
  if(mode=="load-leak" && argc==3) return load_leak(argv[2]);
  if(mode=="assignment") return assignment();
  if(mode=="attention") return attention();
  if(mode=="scan") return scan(false);
  if(mode=="scan-axis0") return scan(true);
  return 2;
}
