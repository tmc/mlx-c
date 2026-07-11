/* Copyright © 2026 Apple Inc. */

#include "mlx/c/private/stream_debug_trace.h"
#ifdef MLX_STREAM_TRACE_BUILD
#include "mlx/c/array.h"
#include "mlx/array.h"
#endif

#include <pthread.h>

#include <atomic>
#include <chrono>
#include <cerrno>
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <mutex>
#include <string>
#include <utility>
#include <thread>
#include <unordered_map>
#include <unistd.h>

#ifdef __APPLE__
#include <limits.h>
#include <mach-o/dyld.h>
#else
#include <limits.h>
#endif

namespace {

constexpr const char* kTraceEnv = "MLX_STREAM_TRACE";
constexpr const char* kTraceRunIdEnv = "MLX_STREAM_TRACE_RUN_ID";
constexpr uint32_t kSchemaVersion = 1;
constexpr size_t kMaxRecordLength = 4096;

struct HandleState {
  uint64_t desc;
  uint64_t desc_gen;
  uint64_t handle_gen;
};

struct TraceState {
  bool enabled = false;
  bool initialized = false;
  FILE* out = nullptr;
  bool owns_output = false;
  uint64_t run_id = 0;
  uint64_t sequence = 0;
  uint64_t next_handle_generation = 1;
  std::mutex init_mu;
  std::mutex state_mu;
  std::unordered_map<uint64_t, HandleState> handle_state;
};

TraceState& state() {
  static TraceState s;
  return s;
}

uint64_t ThreadId() {
#if defined(__APPLE__)
  uint64_t tid = 0;
  if (pthread_threadid_np(nullptr, &tid) != 0) {
    return 0;
  }
  return tid;
#else
  return static_cast<uint64_t>(std::hash<std::thread::id>{}(std::this_thread::get_id()));
#endif
}

uint64_t GetArrayHandleGeneration(uint64_t c_handle) {
  auto& s = state();
  auto it = s.handle_state.find(c_handle);
  if (it == s.handle_state.end()) {
    return 0;
  }
  return it->second.handle_gen;
}

std::string EscapeJson(const char* value) {
  if (value == nullptr) {
    return "";
  }
  std::string out;
  for (const unsigned char* it =
           reinterpret_cast<const unsigned char*>(value);
       *it;
       ++it) {
    if (*it == '\\') {
      out += "\\\\";
    } else if (*it == '"') {
      out += "\\\"";
    } else if (*it == '\n') {
      out += "\\n";
    } else if (*it == '\r') {
      out += "\\r";
    } else if (*it == '\t') {
      out += "\\t";
    } else {
      out.push_back(static_cast<char>(*it));
    }
  }
  return out;
}

uint64_t Fnv1a64(const std::string& value) {
  uint64_t hash = 0xcbf29ce484222325ULL;
  for (unsigned char ch : value) {
    hash ^= ch;
    hash *= 0x100000001b3ULL;
  }
  return hash;
}

uint64_t FallbackRunId() {
#ifdef __APPLE__
  char path[PATH_MAX] = {};
  uint32_t path_len = sizeof(path);
  if (_NSGetExecutablePath(path, &path_len) == 0) {
    return Fnv1a64(path) ^ (static_cast<uint64_t>(getpid()) << 16);
  }
  return static_cast<uint64_t>(getpid()) << 16;
#else
  char path[PATH_MAX] = {};
  const ssize_t n = ::readlink("/proc/self/exe", path, sizeof(path) - 1);
  if (n > 0) {
    path[n] = '\0';
    return Fnv1a64(path) ^ (static_cast<uint64_t>(getpid()) << 16);
  }
  return static_cast<uint64_t>(getpid()) << 16;
#endif
}

uint64_t ParseRunId(const char* value) {
  if (value == nullptr || *value == '\0') {
    return 0;
  }
  errno = 0;
  char* end = nullptr;
  const uint64_t parsed = std::strtoull(value, &end, 0);
  if (errno != 0 || end == value || *end != '\0') {
    return 0;
  }
  return parsed;
}

void EnsureEnabled() {
  auto& s = state();
  if (s.initialized) {
    return;
  }

  std::lock_guard<std::mutex> lock(s.init_mu);
  if (s.initialized) {
    return;
  }

  const char* trace_env = std::getenv(kTraceEnv);
  if (trace_env != nullptr && *trace_env != '\0') {
    if (std::strcmp(trace_env, "1") == 0) {
      s.out = stderr;
      s.enabled = true;
    } else {
      FILE* out = std::fopen(trace_env, "a");
      if (out != nullptr) {
        s.out = out;
        s.owns_output = true;
        s.enabled = true;
      }
    }
  }

  if (s.enabled) {
    const char* run_id_env = std::getenv(kTraceRunIdEnv);
    s.run_id = ParseRunId(run_id_env);
    if (s.run_id == 0) {
      s.run_id = FallbackRunId();
    }
  }
  s.initialized = true;
}

std::pair<uint64_t, uint64_t> RegisterArrayHandleAssignment(
    uint64_t c_handle,
    uint64_t desc,
    uint64_t desc_gen) {
  auto& s = state();
  std::lock_guard<std::mutex> lock(s.state_mu);

  const uint64_t c_handle_gen = s.next_handle_generation++;
  HandleState next_state{desc, desc_gen, c_handle_gen};
  s.handle_state[c_handle] = next_state;
  return {desc_gen, c_handle_gen};
}

void ReleaseArrayHandle(uint64_t c_handle) {
  auto& s = state();
  std::lock_guard<std::mutex> lock(s.state_mu);
  s.handle_state.erase(c_handle);
}

void WriteRecord(const std::string& record) {
  auto& s = state();
  if (s.out == nullptr) {
    return;
  }
  std::fwrite(record.data(), 1, record.size(), s.out);
  if (s.owns_output) {
    std::fflush(s.out);
  }
}

}  // namespace

extern "C" uint64_t mlx_stream_trace_array_handle_generation(mlx_array arr) {
#ifdef MLX_STREAM_TRACE_BUILD
  if (arr.ctx == nullptr) {
    return 0;
  }
  EnsureEnabled();
  if (!state().enabled) {
    return 0;
  }
  std::lock_guard<std::mutex> lock(state().state_mu);
  return GetArrayHandleGeneration(reinterpret_cast<uint64_t>(arr.ctx));
#else
  (void)arr;
  return 0;
#endif
}

namespace mlx {
namespace detail {

void mlx_trace_array_handle_assignment(
    const char* event,
    const char* c_function,
    const char* c_path,
    uint64_t caller_pc,
    void* c_handle,
    int64_t output_position,
    int64_t output_count,
    int64_t requested_stream_index) {
  EnsureEnabled();
  auto& s = state();
  if (!s.enabled || c_handle == nullptr) {
    return;
  }

  const uint64_t handle = reinterpret_cast<uint64_t>(c_handle);
  auto* array_obj = static_cast<mlx::core::array*>(c_handle);
  const uint64_t native_desc = array_obj->id();
#ifdef MLX_STREAM_TRACE_BUILD
  const uint64_t native_desc_gen = array_obj->trace_generation();
#else
  const uint64_t native_desc_gen = 0;
#endif
  const auto generations = RegisterArrayHandleAssignment(handle, native_desc, native_desc_gen);
  const uint64_t handle_gen = generations.second;

  const std::string ev = EscapeJson(event == nullptr ? "" : event);
  const std::string fn = EscapeJson(c_function == nullptr ? "" : c_function);
  const std::string path = EscapeJson(c_path == nullptr ? "" : c_path);
  const uint64_t seq = ++s.sequence;
  const uint64_t ts_ns = static_cast<uint64_t>(
      std::chrono::high_resolution_clock::now().time_since_epoch().count());
  const uint64_t os_tid = ThreadId();
  const uint64_t pid = static_cast<uint64_t>(getpid());

  char buffer[kMaxRecordLength];
  const int rc = std::snprintf(
      buffer,
      sizeof(buffer),
      "{\"schema_version\":%u,"
      "\"run_id\":%llu,"
      "\"version\":%u,"
      "\"event\":\"%s\","
      "\"seq\":%llu,"
      "\"ts_ns\":%llu,"
      "\"os_tid\":\"0x%llx\","
      "\"pid\":%llu,"
      "\"array_data_ptr\":\"0x%llx\","
      "\"array_wrapper_ptr\":\"0x%llx\","
      "\"call_site\":\"private_array\","
      "\"caller_pc\":\"0x%llx\","
      "\"stream_idx\":%lld,"
      "\"stream_domain\":0,"
      "\"pool_gen\":0,"
      "\"native_desc_gen\":%llu,"
      "\"c_handle_gen\":%llu,"
      "\"output_position\":%lld,"
      "\"output_count\":%lld,"
      "\"requested_stream_index\":%lld,"
      "\"c_function\":\"%s\","
      "\"c_path\":\"%s\"}\n",
      kSchemaVersion,
      static_cast<unsigned long long>(s.run_id),
      kSchemaVersion,
      ev.c_str(),
      static_cast<unsigned long long>(seq),
      static_cast<unsigned long long>(ts_ns),
      static_cast<unsigned long long>(os_tid),
      static_cast<unsigned long long>(pid),
      static_cast<unsigned long long>(native_desc),
      static_cast<unsigned long long>(handle),
      static_cast<unsigned long long>(caller_pc),
      static_cast<long long>(-1),
      static_cast<unsigned long long>(native_desc_gen),
      static_cast<unsigned long long>(handle_gen),
      static_cast<long long>(output_position),
      static_cast<long long>(output_count),
      static_cast<long long>(requested_stream_index),
      fn.c_str(),
      path.c_str());
  if (rc > 0) {
    WriteRecord(std::string(buffer, static_cast<size_t>(rc)));
  }
}

void mlx_trace_array_handle_release(void* c_handle) {
  EnsureEnabled();
  if (!state().enabled) {
    return;
  }
  if (c_handle == nullptr) {
    return;
  }
  ReleaseArrayHandle(reinterpret_cast<uint64_t>(c_handle));
}

}  // namespace detail
}  // namespace mlx
