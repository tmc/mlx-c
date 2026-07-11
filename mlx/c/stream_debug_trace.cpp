/* Copyright © 2026 Apple Inc. */

#include "mlx/c/private/stream_debug_trace.h"
#ifdef MLX_STREAM_TRACE_BUILD
#include "mlx/c/array.h"
#include "mlx/array.h"
#endif

#include <pthread.h>

#include <chrono>
#include <cstdlib>
#include <cstring>
#include <fcntl.h>
#include <functional>
#include <mutex>
#include <sstream>
#include <string>
#include <thread>
#include <unordered_map>
#include <unistd.h>

namespace {

constexpr const char* kTraceEnv = "MLX_STREAM_TRACE";
constexpr const char* kTraceFileEnv = "MLX_STREAM_TRACE_FILE";
constexpr const char* kTraceRunIdEnv = "MLX_STREAM_TRACE_RUN_ID";
constexpr size_t kTraceRecordCap = 4096;
constexpr uint32_t kSchemaVersion = 1;

void EmitRecordOrTruncated(
    const std::string& event,
    const std::string& record,
    uint64_t ts_ns,
    uint64_t os_tid,
    uint64_t pid,
    const std::string& run_id,
    uint64_t seq);

struct HandleState {
  uint64_t desc;
  uint64_t desc_gen;
  uint64_t handle_gen;
};

struct TraceState {
  bool enabled = false;
  bool transport_probe_emitted = false;
  std::once_flag init_once;
  int out_fd = -1;
  std::string run_id;
  uint64_t sequence = 0;
  uint64_t next_handle_generation = 1;
  std::mutex trace_mu;
  std::unordered_map<uint64_t, HandleState> handle_state;
};

TraceState& state() {
  static TraceState s;
  return s;
}

void EmitTraceDiagnostic(const std::string& message) {
  const auto msg = message.c_str();
  (void)::write(STDERR_FILENO, msg, std::strlen(msg));
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
    } else if (*it == '\"') {
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

std::string EscapeJson(const std::string& value) {
  return EscapeJson(value.c_str());
}

std::string TruncateForTraceField(const std::string& value, size_t max_len) {
  if (max_len == 0) {
    return "";
  }
  if (value.size() <= max_len) {
    return value;
  }
  if (max_len <= 3) {
    return value.substr(0, max_len);
  }
  return value.substr(0, max_len - 3) + "...";
}

void EmitTraceTransportFailure(const char* reason, uint64_t seq) {
  std::ostringstream msg;
  msg << "MLX_STREAM_TRACE transport failure seq=" << seq << " reason="
      << (reason == nullptr ? "" : reason) << "\n";
  EmitTraceDiagnostic(msg.str());
}

void DisableTracingAfterFailure(const char* reason, uint64_t seq) {
  auto& s = state();
  s.enabled = false;
  EmitTraceTransportFailure(reason, seq);
}

bool IsLocalTraceFile(const char* path) {
  if (path == nullptr || path[0] == '\0') {
    return false;
  }

  return std::strncmp(path, "/tmp/", 5) == 0 ||
      std::strncmp(path, "/private/tmp/", 13) == 0;
}

int OpenTraceFile(const char* path) {
  int fd = ::open(path, O_CREAT | O_WRONLY | O_APPEND | O_CLOEXEC, 0600);
  if (fd < 0) {
    std::ostringstream msg;
    msg << "MLX_STREAM_TRACE_FILE open failed: " << path << "\n";
    EmitTraceDiagnostic(msg.str());
    return -1;
  }
  return fd;
}

int ResolveTraceFD() {
  const char* trace_env = std::getenv(kTraceEnv);
  if (trace_env == nullptr || trace_env[0] == '\0') {
    return -1;
  }

  const char* trace_file = std::getenv(kTraceFileEnv);
  if (trace_file != nullptr && trace_file[0] != '\0') {
    if (!IsLocalTraceFile(trace_file)) {
      EmitTraceDiagnostic("MLX_STREAM_TRACE_FILE must be under /tmp\n");
      return -1;
    }
    const int fd = OpenTraceFile(trace_file);
    return fd;
  }

  if (std::strcmp(trace_env, "1") == 0) {
    return STDERR_FILENO;
  }

  if (!IsLocalTraceFile(trace_env)) {
    EmitTraceDiagnostic("MLX_STREAM_TRACE path must be under /tmp\n");
    return -1;
  }

  const int fd = OpenTraceFile(trace_env);
  return fd;
}

void EnsureEnabled() {
  auto& s = state();
  std::call_once(s.init_once, []() {
    auto& inner = state();
    const char* trace_env = std::getenv(kTraceEnv);
    if (trace_env == nullptr || trace_env[0] == '\0') {
      inner.enabled = false;
      return;
    }

    const char* run_id_env = std::getenv(kTraceRunIdEnv);
    inner.run_id = run_id_env == nullptr ? "" : run_id_env;

    inner.out_fd = ResolveTraceFD();
    if (inner.out_fd >= 0) {
      inner.enabled = true;
    } else {
      inner.enabled = false;
    }
  });
}

uint64_t NextSeq() {
  auto& s = state();
  return ++s.sequence;
}

void WriteRecordLocked(const std::string& record, uint64_t seq) {
  if (record.size() > kTraceRecordCap) {
    DisableTracingAfterFailure("record exceeds cap", seq);
    return;
  }

  auto& s = state();
  if (!s.enabled || s.out_fd < 0) {
    return;
  }

  const ssize_t rc = ::write(s.out_fd, record.data(), record.size());
  if (rc != static_cast<ssize_t>(record.size())) {
    DisableTracingAfterFailure("short/failed write", seq);
  }
}

void EmitTransportProbeLocked() {
  auto& s = state();
  if (s.transport_probe_emitted) {
    return;
  }
  s.transport_probe_emitted = true;

  const uint64_t ts_ns = static_cast<uint64_t>(
      std::chrono::high_resolution_clock::now().time_since_epoch().count());
  const uint64_t os_tid = ThreadId();
  const uint64_t pid = static_cast<uint64_t>(getpid());
  const uint64_t seq = NextSeq();
  const std::string run_id = EscapeJson(s.run_id.c_str());

  std::ostringstream trace;
  trace << '{' << "\"schema_version\":" << kSchemaVersion << ','
        << "\"source\":\"mlxc\",";
  trace << "\"run_id\":\"" << run_id << "\",";
  trace << "\"event\":\"transport_probe\",";
  trace << "\"seq\":" << seq << ',';
  trace << "\"mono_ns\":" << ts_ns << ',';
  trace << "\"os_tid\":" << os_tid << ',';
  trace << "\"pid\":" << pid << ',';
  trace << "\"truncated\":false}" << '\n';
  EmitRecordOrTruncated(
      "transport_probe",
      trace.str(),
      ts_ns,
      os_tid,
      pid,
      s.run_id,
      seq);
}

void EmitRecordOrTruncated(
    const std::string& event,
    const std::string& record,
    uint64_t ts_ns,
    uint64_t os_tid,
    uint64_t pid,
    const std::string& run_id,
    uint64_t seq) {
  if (record.size() <= kTraceRecordCap) {
    WriteRecordLocked(record, seq);
    return;
  }

  const std::string run_id_preview = EscapeJson(TruncateForTraceField(run_id, 64));
  const std::string event_preview = EscapeJson(TruncateForTraceField(event, 64));

  std::ostringstream truncated;
  truncated << '{' << "\"schema_version\":" << kSchemaVersion << ','
            << "\"source\":\"mlxc\",";
  truncated << "\"run_id\":\"" << run_id_preview << "\",";
  truncated << "\"run_id_len\":" << run_id.size() << ',';
  truncated << "\"event\":\"transport_truncated\",";
  truncated << "\"seq\":" << seq << ',';
  truncated << "\"mono_ns\":" << ts_ns << ',';
  truncated << "\"os_tid\":" << os_tid << ',';
  truncated << "\"pid\":" << pid << ',';
  truncated << "\"truncated\":true,";
  truncated << "\"overflow_event\":\"" << event_preview << "\",";
  truncated << "\"overflow_bytes\":" << record.size() << '}' << '\n';

  const std::string overflow_record = truncated.str();
  if (overflow_record.size() > kTraceRecordCap) {
    DisableTracingAfterFailure("truncated summary exceeds cap", seq);
    return;
  }

  WriteRecordLocked(overflow_record, seq);
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
  if (c_handle == nullptr) {
    return;
  }

  EnsureEnabled();
  if (!state().enabled) {
    return;
  }

  std::lock_guard<std::mutex> lock(s.trace_mu);
  if (s.out_fd < 0) {
    return;
  }

  EmitTransportProbeLocked();

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
  const uint64_t seq = NextSeq();
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
  trace << '{' << "\"schema_version\":" << kSchemaVersion << ',';
  trace << "\"source\":\"mlxc\",";
  trace << "\"run_id\":\"" << run_id << "\",";
  trace << "\"event\":\"" << ev << "\",";
  trace << "\"seq\":" << seq << ',';
  trace << "\"mono_ns\":" << ts_ns << ',';
  trace << "\"os_tid\":" << os_tid << ',';
  trace << "\"pid\":" << pid << ',';
  trace << "\"array_data_ptr\":\"0x" << std::hex << native_desc << std::dec << "\",";
  trace << "\"array_wrapper_ptr\":\"0x" << std::hex << handle << std::dec << "\",";
  trace << "\"call_site\":\"private_array\",";
  trace << "\"caller_pc\":\"0x" << std::hex << caller_pc << std::dec << "\",";
  trace << "\"stream_idx\":-1,";
  trace << "\"stream_domain\":0,";
  trace << "\"pool_gen\":0,";
  trace << "\"native_desc_gen\":" << native_desc_gen << ',';
  trace << "\"c_handle_gen\":" << c_handle_gen << ',';
  trace << "\"output_position\":" << output_position << ',';
  trace << "\"output_count\":" << output_count << ',';
  trace << "\"requested_stream_index\":" << requested_stream_index << ',';
  trace << "\"c_function\":\"" << fn << "\",";
  trace << "\"c_path\":\"" << path << "\"}" << '\n';

  const std::string record = trace.str();
  EmitRecordOrTruncated(ev, record, ts_ns, os_tid, pid, s.run_id, seq);
}

void ReleaseArrayHandle(uint64_t c_handle) {
  auto& s = state();
  std::lock_guard<std::mutex> lock(s.trace_mu);
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
  auto& s = state();
  if (!s.enabled) {
    return 0;
  }

  std::lock_guard<std::mutex> lock(s.trace_mu);
  return GetArrayHandleGeneration(
      reinterpret_cast<uint64_t>(arr.ctx), s);
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
  if (!state().enabled || c_handle == nullptr) {
    return;
  }

  ReleaseArrayHandle(reinterpret_cast<uint64_t>(c_handle));
}

}  // namespace detail
}  // namespace mlx
