/* Copyright © 2026 Apple Inc. */

#include "mlx/c/private/stream_debug_trace.h"
#ifdef MLX_STREAM_TRACE_BUILD
#include "mlx/c/array.h"
#include "mlx/array.h"
#endif

#include <pthread.h>

#include <atomic>
#include <chrono>
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <functional>
#include <mutex>
#include <sstream>
#include <string>
#include <thread>
#include <unordered_map>
#include <unistd.h>

namespace {

constexpr const char* kTraceEnv = "MLX_STREAM_TRACE";
constexpr const char* kTraceRunIdEnv = "MLX_STREAM_TRACE_RUN_ID";
constexpr uint32_t kSchemaVersion = 1;

struct HandleState {
  uint64_t desc;
  uint64_t desc_gen;
  uint64_t handle_gen;
};

struct TraceState {
  bool enabled = false;
  std::once_flag init_once;
  FILE* out = nullptr;
  bool owns_output = false;
  std::string run_id;
  uint64_t sequence = 0;
  uint64_t next_handle_generation = 1;
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

uint64_t GetArrayHandleGeneration(uint64_t c_handle, const TraceState& s) {
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

void EnsureEnabled() {
  auto& s = state();
  std::call_once(s.init_once, []() {
    auto& inner = state();
    const char* trace_env = std::getenv(kTraceEnv);
    if (trace_env != nullptr && *trace_env != '\0') {
      if (std::strcmp(trace_env, "1") == 0) {
        inner.out = stderr;
        inner.enabled = true;
      } else {
        FILE* out = std::fopen(trace_env, "a");
        if (out != nullptr) {
          inner.out = out;
          inner.owns_output = true;
          inner.enabled = true;
        }
      }
    }

    if (inner.enabled) {
      const char* run_id_env = std::getenv(kTraceRunIdEnv);
      inner.run_id = run_id_env == nullptr ? "" : run_id_env;
    }
  });
}

void EmitArrayHandleAssignmentRecord(
    uint64_t handle,
    void* c_handle,
    int64_t output_position,
    int64_t output_count,
    int64_t requested_stream_index,
    const char* event,
    const char* c_function,
    const char* c_path,
    uint64_t caller_pc) {
  auto& s = state();
  std::lock_guard<std::mutex> lock(s.state_mu);
  if (!s.enabled || s.out == nullptr || c_handle == nullptr) {
    return;
  }

  uint64_t native_desc = 0;
  uint64_t native_desc_gen = 0;
#ifdef MLX_STREAM_TRACE_BUILD
  auto* array_obj = static_cast<mlx::core::array*>(c_handle);
  if (array_obj != nullptr) {
    native_desc = array_obj->id();
    native_desc_gen = array_obj->trace_generation();
  }
#endif

  const uint64_t c_handle_gen = s.next_handle_generation++;
  const uint64_t seq = ++s.sequence;
  s.handle_state[handle] = HandleState{native_desc, native_desc_gen, c_handle_gen};

  const std::string ev = EscapeJson(event == nullptr ? "" : event);
  const std::string fn = EscapeJson(c_function == nullptr ? "" : c_function);
  const std::string path = EscapeJson(c_path == nullptr ? "" : c_path);
  const uint64_t ts_ns = static_cast<uint64_t>(
      std::chrono::high_resolution_clock::now().time_since_epoch().count());
  const uint64_t os_tid = ThreadId();
  const uint64_t pid = static_cast<uint64_t>(getpid());
  const std::string run_id = EscapeJson(s.run_id.c_str());

  std::ostringstream trace;
  trace << '{' << "\"schema_version\":" << kSchemaVersion << ','
        << "\"run_id\":\"" << run_id << "\","
        << "\"version\":" << kSchemaVersion << ','
        << "\"event\":\"" << ev << "\","
        << "\"seq\":" << seq << ','
        << "\"ts_ns\":" << ts_ns << ','
        << "\"os_tid\":" << os_tid << ','
        << "\"pid\":" << pid << ','
        << "\"array_data_ptr\":\"0x" << std::hex << native_desc << std::dec
        << "\","
        << "\"array_wrapper_ptr\":\"0x" << std::hex << handle << std::dec << "\","
        << "\"call_site\":\"private_array\","
        << "\"caller_pc\":\"0x" << std::hex << caller_pc << std::dec << "\","
        << "\"stream_idx\":-1,"
        << "\"stream_domain\":0,"
        << "\"pool_gen\":0,"
        << "\"native_desc_gen\":" << native_desc_gen << ','
        << "\"c_handle_gen\":" << c_handle_gen << ','
        << "\"output_position\":" << output_position << ','
        << "\"output_count\":" << output_count << ','
        << "\"requested_stream_index\":" << requested_stream_index << ','
        << "\"c_function\":\"" << fn << "\","
        << "\"c_path\":\"" << path << "\"}\n";
  const std::string record = trace.str();
  if (record.empty()) {
    return;
  }
  if (s.out != nullptr) {
    std::fwrite(record.data(), 1, record.size(), s.out);
    if (s.owns_output) {
      std::fflush(s.out);
    }
  }
}

void ReleaseArrayHandle(uint64_t c_handle) {
  auto& s = state();
  std::lock_guard<std::mutex> lock(s.state_mu);
  s.handle_state.erase(c_handle);
}

}  // namespace

extern "C" uint64_t mlx_stream_trace_array_handle_generation(mlx_array arr) {
#ifdef MLX_STREAM_TRACE_BUILD
  if (arr.ctx == nullptr) {
    return 0;
  }
#endif
  EnsureEnabled();
  if (!state().enabled) {
    return 0;
  }
  std::lock_guard<std::mutex> lock(state().state_mu);
  return GetArrayHandleGeneration(reinterpret_cast<uint64_t>(arr.ctx), state());
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
  EmitArrayHandleAssignmentRecord(
      reinterpret_cast<uint64_t>(c_handle),
      c_handle,
      output_position,
      output_count,
      requested_stream_index,
      event,
      c_function,
      c_path,
      caller_pc);
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
