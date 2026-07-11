/* Copyright © 2023-2024 Apple Inc. */

#include <algorithm>
#include <cerrno>
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <fcntl.h>
#include <fstream>
#include <iostream>
#include <sstream>
#include <cstddef>
#include <string>
#include <sys/stat.h>
#include <sys/wait.h>
#include <thread>
#include <unordered_map>
#include <unordered_set>
#include <unistd.h>
#include <vector>

#include "mlx/c/array.h"
#include "mlx/c/error.h"

namespace {

constexpr char kTraceEnv[] = "MLX_STREAM_TRACE";
constexpr char kTraceFileEnv[] = "MLX_STREAM_TRACE_FILE";
constexpr char kTraceRunIdEnv[] = "MLX_STREAM_TRACE_RUN_ID";
constexpr int kConcurrentThreads = 4;
constexpr int kConcurrentIterations = 32;

void fail(const char* message) {
  std::cerr << message << '\n';
  std::exit(1);
}

bool parse_u64_hex_field(
    const std::string& line,
    const char* key,
    uint64_t& value) {
  const std::string token = std::string("\"") + key + "\":\"0x";
  const size_t pos = line.find(token);
  if (pos == std::string::npos) {
    return false;
  }
  size_t start = pos + token.size();
  const size_t end = line.find('"', start);
  if (end == std::string::npos || end <= start) {
    return false;
  }
  try {
    value = std::stoull(line.substr(start, end - start), nullptr, 16);
  } catch (const std::exception&) {
    return false;
  }
  return true;
}

bool parse_u64_dec_field(
    const std::string& line,
    const char* key,
    uint64_t& value) {
  const std::string token = std::string("\"") + key + "\":";
  const size_t pos = line.find(token);
  if (pos == std::string::npos) {
    return false;
  }
  size_t start = pos + token.size();
  const size_t end = line.find_first_of(",}", start);
  if (end == std::string::npos || end <= start) {
    return false;
  }
  try {
    value = std::stoull(line.substr(start, end - start));
  } catch (const std::exception&) {
    return false;
  }
  return true;
}

bool parse_string_field(
    const std::string& line,
    const char* key,
    std::string& value) {
  const std::string token = std::string("\"") + key + "\":\"";
  const size_t pos = line.find(token);
  if (pos == std::string::npos) {
    return false;
  }
  const size_t start = pos + token.size();
  const size_t end = line.find('"', start);
  if (end == std::string::npos) {
    return false;
  }
  value = line.substr(start, end - start);
  return true;
}

bool parse_optional_bool_field(
    const std::string& line,
    const char* key,
    bool& value) {
  const std::string token = std::string("\"") + key + "\":";
  const size_t pos = line.find(token);
  if (pos == std::string::npos) {
    return false;
  }
  const size_t start = pos + token.size();
  if (line.compare(start, 4, "true") == 0) {
    value = true;
    return true;
  }
  if (line.compare(start, 5, "false") == 0) {
    value = false;
    return true;
  }
  return false;
}

struct TraceRecord {
  uint64_t seq = 0;
  std::string run_id;
  uint64_t run_id_len = 0;
  uint64_t c_handle_gen = 0;
  uint64_t native_desc_gen = 0;
  uint64_t array_data_ptr = 0;
  uint64_t array_wrapper_ptr = 0;
  uint64_t caller_pc = 0;
  std::string event;
  bool truncated = false;
  std::string overflow_event;
};

extern "C" uint64_t mlx_stream_trace_array_handle_generation(mlx_array arr);

std::vector<TraceRecord> parse_trace_file(const std::string& path) {
  std::vector<TraceRecord> out;
  std::ifstream in(path);
  if (!in.is_open()) {
    return out;
  }

  std::string line;
  while (std::getline(in, line)) {
    if (line.empty()) {
      continue;
    }
    if (line.front() != '{' || line.back() != '}') {
      fail("trace file contains non-json line");
    }
    TraceRecord r;
    if (!parse_string_field(line, "run_id", r.run_id) ||
        !parse_u64_dec_field(line, "seq", r.seq) ||
        !parse_string_field(line, "event", r.event)) {
      fail("trace file contains unparsable line");
    }
    parse_u64_dec_field(line, "run_id_len", r.run_id_len);
    parse_u64_dec_field(line, "c_handle_gen", r.c_handle_gen);
    parse_u64_dec_field(line, "native_desc_gen", r.native_desc_gen);
    parse_u64_hex_field(line, "array_data_ptr", r.array_data_ptr);
    parse_u64_hex_field(line, "array_wrapper_ptr", r.array_wrapper_ptr);
    parse_u64_hex_field(line, "caller_pc", r.caller_pc);
    parse_string_field(line, "overflow_event", r.overflow_event);
    parse_optional_bool_field(line, "truncated", r.truncated);
    out.push_back(r);
  }
  return out;
}

void validate_records_common(
    const std::vector<TraceRecord>& records,
    const std::string& expected_run_id) {
  for (const auto& r : records) {
    if (r.run_id != expected_run_id) {
      fail("trace run_id changed across records");
    }
    if (r.event.empty()) {
      fail("trace record missing event");
    }
  }
}

void emit_trace_operation() {
  mlx_array a = mlx_array_new_int(23);
  mlx_array b = mlx_array_new();
  if (mlx_array_set(&b, a)) {
    fail("path-security trace operation failed");
  }
  mlx_array_free(a);
  mlx_array_free(b);
}

std::string direct_trace_path(const char* root, const char* kind) {
  std::ostringstream path;
  path << root << "/mlx-c-stream-trace-" << kind << "-" << getpid();
  return path.str();
}

void create_empty_file(const std::string& path) {
  const int fd = ::open(path.c_str(), O_CREAT | O_EXCL | O_RDWR, 0600);
  if (fd < 0 || ::close(fd) != 0) {
    fail("unable to create path-security file");
  }
}

off_t file_size(const std::string& path) {
  struct stat status {};
  if (::stat(path.c_str(), &status) != 0) {
    fail("unable to stat path-security file");
  }
  return status.st_size;
}

void run_path_security_mode(const char* kind) {
  setenv(kTraceEnv, "1", 1);
  setenv(kTraceRunIdEnv, "stream-trace-path-security", 1);

  std::string path;
  std::string target;
  std::string directory;
  int held_fd = -1;
  bool expect_trace = false;

  if (std::strcmp(kind, "tmp") == 0) {
    path = direct_trace_path("/tmp", kind);
    std::remove(path.c_str());
    create_empty_file(path);
    expect_trace = true;
  } else if (std::strcmp(kind, "private-tmp") == 0) {
    path = direct_trace_path("/private/tmp", kind);
    std::remove(path.c_str());
    create_empty_file(path);
    expect_trace = true;
  } else if (std::strcmp(kind, "traversal") == 0) {
    target = direct_trace_path("/tmp", kind);
    std::remove(target.c_str());
    create_empty_file(target);
    const auto name = target.substr(target.find_last_of('/') + 1);
    path = "/private/tmp/../tmp/" + name;
  } else if (std::strcmp(kind, "nested") == 0) {
    char dir[] = "/tmp/mlx-c-stream-trace-nested-XXXXXX";
    if (::mkdtemp(dir) == nullptr) {
      fail("unable to create nested path directory");
    }
    directory = dir;
    path = directory + "/trace.jsonl";
  } else if (std::strcmp(kind, "dot") == 0) {
    path = "/tmp/.";
  } else if (std::strcmp(kind, "parent") == 0) {
    path = "/private/tmp/..";
  } else if (std::strcmp(kind, "relative") == 0) {
    path = direct_trace_path(".", kind);
    std::remove(path.c_str());
  } else if (std::strcmp(kind, "other-root") == 0) {
    path = direct_trace_path("/var/tmp", kind);
    std::remove(path.c_str());
  } else if (std::strcmp(kind, "symlink") == 0) {
    target = direct_trace_path("/tmp", "symlink-target");
    path = direct_trace_path("/tmp", kind);
    std::remove(target.c_str());
    std::remove(path.c_str());
    create_empty_file(target);
    if (::symlink(target.c_str(), path.c_str()) != 0) {
      fail("unable to create path-security symlink");
    }
  } else if (std::strcmp(kind, "directory") == 0) {
    char dir[] = "/tmp/mlx-c-stream-trace-directory-XXXXXX";
    if (::mkdtemp(dir) == nullptr) {
      fail("unable to create path-security directory");
    }
    path = dir;
  } else if (std::strcmp(kind, "fifo") == 0) {
    path = direct_trace_path("/tmp", kind);
    std::remove(path.c_str());
    if (::mkfifo(path.c_str(), 0600) != 0) {
      fail("unable to create path-security fifo");
    }
    held_fd = ::open(path.c_str(), O_RDONLY | O_NONBLOCK);
    if (held_fd < 0) {
      fail("unable to open path-security fifo reader");
    }
  } else if (std::strcmp(kind, "hardlink") == 0) {
    target = direct_trace_path("/tmp", "hardlink-target");
    path = direct_trace_path("/tmp", kind);
    std::remove(target.c_str());
    std::remove(path.c_str());
    create_empty_file(target);
    if (::link(target.c_str(), path.c_str()) != 0) {
      if (errno == EPERM || errno == ENOTSUP) {
        std::remove(target.c_str());
        return;
      }
      fail("unable to create path-security hardlink");
    }
  } else {
    fail("unknown path-security case");
  }

  setenv(kTraceFileEnv, path.c_str(), 1);
  emit_trace_operation();

  if (expect_trace) {
    if (file_size(path) == 0) {
      fail("valid direct trace path produced no output");
    }
  } else if (!target.empty()) {
    if (file_size(target) != 0) {
      fail("unsafe trace path modified target");
    }
  } else if (std::strcmp(kind, "nested") == 0 ||
             std::strcmp(kind, "relative") == 0 ||
             std::strcmp(kind, "other-root") == 0) {
    if (::access(path.c_str(), F_OK) == 0) {
      fail("unsafe trace path created output");
    }
  }

  if (held_fd >= 0) {
    ::close(held_fd);
  }
  if (!path.empty() && std::strcmp(kind, "dot") != 0 &&
      std::strcmp(kind, "parent") != 0 &&
      std::strcmp(kind, "directory") != 0) {
    std::remove(path.c_str());
  }
  if (!target.empty()) {
    std::remove(target.c_str());
  }
  if (!directory.empty()) {
    ::rmdir(directory.c_str());
  }
  if (std::strcmp(kind, "directory") == 0) {
    ::rmdir(path.c_str());
  }
}

void run_path_security_processes(const char* exe_path) {
  const char* cases[] = {
      "tmp",
      "private-tmp",
      "traversal",
      "nested",
      "dot",
      "parent",
      "relative",
      "other-root",
      "symlink",
      "directory",
      "fifo",
      "hardlink",
  };
  for (const char* path_case : cases) {
    const pid_t pid = fork();
    if (pid < 0) {
      fail("unable to fork path-security worker");
    }
    if (pid == 0) {
      execl(exe_path, exe_path, "--path-security", path_case, nullptr);
      std::_Exit(1);
    }
    int status = 0;
    if (waitpid(pid, &status, 0) != pid || !WIFEXITED(status) ||
        WEXITSTATUS(status) != 0) {
      fail("path-security worker failed");
    }
  }
}

void run_concurrent_trace_worker() {
  char path[256];
  const char* trace_path = std::getenv(kTraceEnv);
  if (trace_path == nullptr || trace_path[0] == '\0') {
    fail("trace path missing in concurrent worker");
  }
  std::snprintf(path, sizeof(path), "%s", trace_path);
  const auto base_records = parse_trace_file(path);

  auto worker = []() {
    for (int i = 0; i < kConcurrentIterations; ++i) {
      mlx_array a = mlx_array_new_int(i);
      mlx_array b = mlx_array_new();
      if (mlx_array_set(&b, a)) {
        fail("concurrent trace test failed to copy array");
      }
      mlx_array_free(a);
      mlx_array_free(b);
    }
  };

  std::vector<std::thread> threads;
  threads.reserve(kConcurrentThreads);
  for (int i = 0; i < kConcurrentThreads; ++i) {
    threads.emplace_back(worker);
  }
  for (auto& t : threads) {
    t.join();
  }

  auto assert_worker_records = [&](const std::vector<TraceRecord>& records) {
    if (records.size() < static_cast<size_t>(
            kConcurrentThreads * kConcurrentIterations * 2)) {
      fail("concurrent trace test lost records");
    }
  };
  auto records = parse_trace_file(path);
  if (records.size() <= base_records.size()) {
    fail("concurrent trace test produced no new records");
  }

  std::vector<TraceRecord> appended_records(
      records.begin() + static_cast<long long>(base_records.size()), records.end());
  assert_worker_records(appended_records);
  const std::string expected_run_id =
      std::getenv(kTraceRunIdEnv) == nullptr ? "" : std::getenv(kTraceRunIdEnv);
  validate_records_common(appended_records, expected_run_id);

  for (size_t i = 1; i < appended_records.size(); ++i) {
    if (appended_records[i].seq <= appended_records[i - 1].seq) {
      fail("trace sequence is not strictly increasing under concurrency");
    }
  }

  uint64_t previous_seq = 0;
  for (const auto& r : appended_records) {
    if (r.seq == 0 || r.seq == previous_seq) {
      fail("trace sequence value missing or duplicated");
    }
    previous_seq = r.seq;
  }
}

void run_concurrent_trace_process(const char* exe_path, const std::string& path) {
  const size_t pre_size = parse_trace_file(path).size();

  pid_t pid = fork();
  if (pid < 0) {
    fail("unable to fork concurrent trace worker");
  }
  if (pid == 0) {
    execl(exe_path, exe_path, "--concurrent", nullptr);
    std::_Exit(1);
  }

  int status = 0;
  if (waitpid(pid, &status, 0) != pid) {
    fail("failed to wait for concurrent trace worker");
  }
  if (!WIFEXITED(status) || WEXITSTATUS(status) != 0) {
    fail("concurrent trace worker failed");
  }

  auto all_records = parse_trace_file(path);
  if (all_records.size() <= pre_size) {
    fail("concurrent trace test lost records");
  }
  std::vector<TraceRecord> records(
      all_records.begin() + pre_size, all_records.end());
  const std::string expected_run_id =
      std::getenv(kTraceRunIdEnv) == nullptr ? "" : std::getenv(kTraceRunIdEnv);
  validate_records_common(records, expected_run_id);

  for (size_t i = 1; i < records.size(); ++i) {
    if (records[i].seq <= records[i - 1].seq) {
      fail("trace sequence is not strictly increasing under concurrency");
    }
  }

  uint64_t previous_seq = 0;
  for (const auto& r : records) {
    if (r.seq == 0 || r.seq == previous_seq) {
      fail("trace sequence value missing or duplicated");
    }
    previous_seq = r.seq;
  }
}

void run_transport_probe_mode() {
  setenv(kTraceRunIdEnv, "stream-trace-probe", 1);

  char path[256];
  std::snprintf(path, sizeof(path), "/tmp/mlx-c-stream-trace-probe-%d.log", getpid());
  std::remove(path);
  setenv(kTraceEnv, path, 1);

  auto records = parse_trace_file(path);
  if (!records.empty()) {
    fail("probe mode trace file should be empty");
  }

  mlx_array a = mlx_array_new_int(13);
  mlx_array b = mlx_array_new();
  if (mlx_array_set(&b, a)) {
    fail("probe mode failed to copy array");
  }
  mlx_array_free(a);
  mlx_array_free(b);

  records = parse_trace_file(path);
  if (records.empty()) {
    fail("probe mode produced no records");
  }

  uint64_t probe_seq = records.front().seq;
  if (records[0].event != "transport_probe") {
    fail("first event was not transport_probe");
  }
  if (probe_seq != 1) {
    fail("transport probe did not emit seq 1");
  }

  uint64_t previous_seq = 0;
  size_t probe_count = 0;
  bool saw_duplicate_probe_seq = false;
  for (const auto& r : records) {
    if (r.seq <= previous_seq) {
      fail("trace sequence not strictly increasing");
    }
    if (r.seq == probe_seq) {
      if (r.event == "transport_probe") {
        ++probe_count;
      } else {
        saw_duplicate_probe_seq = true;
      }
    }
    previous_seq = r.seq;
  }
  if (probe_count != 1) {
    fail("transport probe not emitted exactly once");
  }
  if (saw_duplicate_probe_seq) {
    fail("non-probe event reused probe sequence");
  }
}

void run_transport_oversized_probe_mode() {
  const size_t run_id_size = 5000;
  const std::string oversized_run_id(run_id_size, 'y');
  setenv(kTraceRunIdEnv, oversized_run_id.c_str(), 1);

  char path[256];
  std::snprintf(path, sizeof(path), "/tmp/mlx-c-stream-trace-oversized-probe-%d.log", getpid());
  std::remove(path);
  setenv(kTraceEnv, path, 1);

  auto records = parse_trace_file(path);
  if (!records.empty()) {
    fail("oversized probe mode trace file should be empty");
  }

  mlx_array a = mlx_array_new_int(17);
  mlx_array b = mlx_array_new();
  if (mlx_array_set(&b, a)) {
    fail("oversized probe mode failed to copy array");
  }
  mlx_array_free(a);
  mlx_array_free(b);

  records = parse_trace_file(path);
  if (records.empty()) {
    fail("oversized probe mode produced no records");
  }

  if (records[0].event != "transport_truncated") {
    fail("oversized run id did not generate transport_truncated");
  }
  if (records[0].overflow_event != "transport_probe") {
    fail("transport_truncated event did not identify probe overflow");
  }
  if (!records[0].truncated) {
    fail("transport_truncated missing truncated=true");
  }
  if (records[0].run_id_len != oversized_run_id.size()) {
    fail("transport_truncated did not preserve oversized run id length");
  }
  if (records[0].seq != 1) {
    fail("oversized probe did not emit seq 1");
  }
}

void run_transport_oversized_probe_process(const char* exe_path) {
  pid_t pid = fork();
  if (pid < 0) {
    fail("unable to fork oversized probe worker");
  }
  if (pid == 0) {
    execl(exe_path, exe_path, "--oversized-probe", nullptr);
    std::_Exit(1);
  }
  int status = 0;
  if (waitpid(pid, &status, 0) != pid) {
    fail("failed to wait for oversized probe worker");
  }
  if (!WIFEXITED(status) || WEXITSTATUS(status) != 0) {
    fail("oversized probe worker failed");
  }
}

void run_transport_probe_process(const char* exe_path) {
  pid_t pid = fork();
  if (pid < 0) {
    fail("unable to fork transport probe worker");
  }
  if (pid == 0) {
    execl(exe_path, exe_path, "--probe", nullptr);
    std::_Exit(1);
  }
  int status = 0;
  if (waitpid(pid, &status, 0) != pid) {
    fail("failed to wait for transport probe worker");
  }
  if (!WIFEXITED(status) || WEXITSTATUS(status) != 0) {
    fail("transport probe worker failed");
  }
}

void run_oversized_run_id_mode() {
  const size_t run_id_size = 2048;
  const std::string oversized_run_id(run_id_size, 'x');
  setenv(kTraceRunIdEnv, oversized_run_id.c_str(), 1);

  char path[256];
  std::snprintf(path, sizeof(path), "/tmp/mlx-c-stream-trace-oversized-%d.log", getpid());
  std::remove(path);
  setenv(kTraceEnv, path, 1);

  auto records = parse_trace_file(path);
  if (!records.empty()) {
    fail("trace file should be empty before oversized run_id trace test");
  }

  mlx_array a = mlx_array_new_int(11);
  mlx_array b = mlx_array_new();
  if (mlx_array_set(&b, a)) {
    fail("oversized run_id test failed to copy array");
  }
  mlx_array_free(a);
  mlx_array_free(b);

  records = parse_trace_file(path);
  if (records.empty()) {
    fail("oversized run_id test produced no records");
  }
  for (const auto& r : records) {
    if (r.seq == 0) {
      fail("invalid sequence in oversized run_id mode");
    }
    if (r.seq == 1 && r.event != "transport_truncated" && r.event != "transport_probe") {
      fail("first oversized event was not probe");
    }
  }
  for (const auto& r : records) {
    if (r.run_id_len != 0 && r.run_id_len != run_id_size) {
      fail("oversized run_id length did not preserve exact size in overflow record");
    }
    if (r.event == "transport_truncated") {
      continue;
    }
    if (r.run_id != oversized_run_id) {
      fail("oversized run_id test changed non-truncated run_id value");
    }
  }
}

void run_oversized_run_id_process(const char* exe_path) {
  pid_t pid = fork();
  if (pid < 0) {
    fail("unable to fork oversized run_id test");
  }
  if (pid == 0) {
    execl(exe_path, exe_path, "--oversized-run-id", nullptr);
    std::_Exit(1);
  }
  int status = 0;
  if (waitpid(pid, &status, 0) != pid) {
    fail("failed to wait for oversized run_id worker");
  }
  if (!WIFEXITED(status) || WEXITSTATUS(status) != 0) {
    fail("oversized run_id worker failed");
  }
}

}  // namespace

int main(int argc, char** argv) {
  if (argc > 2 && std::strcmp(argv[1], "--path-security") == 0) {
    run_path_security_mode(argv[2]);
    return 0;
  }
  if (argc > 1 && std::strcmp(argv[1], "--concurrent") == 0) {
    run_concurrent_trace_worker();
    return 0;
  }
  if (argc > 1 && std::strcmp(argv[1], "--oversized-run-id") == 0) {
    run_oversized_run_id_mode();
    return 0;
  }
  if (argc > 1 && std::strcmp(argv[1], "--probe") == 0) {
    run_transport_probe_mode();
    return 0;
  }
  if (argc > 1 && std::strcmp(argv[1], "--oversized-probe") == 0) {
    run_transport_oversized_probe_mode();
    return 0;
  }

  char path[256];
  std::snprintf(
      path, sizeof(path), "/tmp/mlx-c-stream-trace-%d.log", static_cast<int>(getpid()));
  std::remove(path);
  setenv(kTraceEnv, path, 1);
  setenv(kTraceRunIdEnv, "stream-trace-run-1", 1);

  auto records = parse_trace_file(path);
  if (!records.empty()) {
    fail("trace file should be empty before trace operations");
  }

  mlx_array a = mlx_array_new_int(1);
  const uint64_t a_wrapper = reinterpret_cast<uint64_t>(a.ctx);
  mlx_array b = mlx_array_new();
  uint64_t b_wrapper = reinterpret_cast<uint64_t>(b.ctx);
  if (mlx_array_set(&b, a)) {
    fail("failed to copy array");
  }
  b_wrapper = reinterpret_cast<uint64_t>(b.ctx);
  if (mlx_array_set_int(&b, 2)) {
    fail("failed to mutate copied array");
  }
  mlx_array c = mlx_array_new();
  uint64_t c_wrapper = reinterpret_cast<uint64_t>(c.ctx);
  if (mlx_array_set_bool(&c, true)) {
    fail("failed to initialize third array");
  }
  c_wrapper = reinterpret_cast<uint64_t>(c.ctx);

#ifndef MLX_STREAM_TRACE_BUILD
  records = parse_trace_file(path);
  if (!records.empty()) {
    fail("trace output present while MLX_STREAM_TRACE_BUILD is off");
  }
  return 0;
#endif

  records = parse_trace_file(path);
  if (records.empty()) {
    fail("trace output missing in trace build");
  }
  validate_records_common(records, "stream-trace-run-1");

  if (records.front().run_id != "stream-trace-run-1") {
    fail("trace run_id did not inherit MLX_STREAM_TRACE_RUN_ID");
  }

  const std::unordered_set<uint64_t> owned_wrappers = {
      a_wrapper,
      b_wrapper,
      c_wrapper,
  };

  std::unordered_map<uint64_t, std::vector<uint64_t>> handle_gens;
  uint64_t base_gen = 0;
  uint64_t base_desc = 0;
  uint64_t copy_desc = 0;
  uint64_t expected_copy_gen = 0;
  bool saw_base = false;
  bool saw_copy = false;
  std::unordered_map<uint64_t, uint64_t> retired_desc_generations;
  for (const auto& r : records) {
    if (r.event == "array_new" && r.array_wrapper_ptr == a_wrapper) {
      if (!saw_base) {
        base_gen = r.native_desc_gen;
        base_desc = r.array_data_ptr;
        saw_base = true;
      }
    }
    if (r.event == "array_set" && r.array_wrapper_ptr == b_wrapper) {
      if (!saw_copy) {
        copy_desc = r.array_data_ptr;
        expected_copy_gen = r.native_desc_gen;
        saw_copy = true;
      }
    }
    if (owned_wrappers.count(r.array_wrapper_ptr) && r.array_data_ptr != 0) {
      retired_desc_generations[r.array_data_ptr] =
          std::max(retired_desc_generations[r.array_data_ptr], r.native_desc_gen);
    }
    handle_gens[r.array_wrapper_ptr].push_back(r.c_handle_gen);
  }

  if (!saw_base) {
    fail("missing source array_new record");
  }
  if (!saw_copy) {
    fail("missing copy array_set record");
  }
  if (base_desc != copy_desc || base_gen != expected_copy_gen) {
    fail("copy did not retain source native_desc/gen");
  }

  for (const auto& [wrapper, gens] : handle_gens) {
    if (gens.size() < 2) {
      continue;
    }
    for (size_t i = 1; i < gens.size(); ++i) {
      if (gens[i] <= gens[i - 1]) {
        fail("handle generations did not advance");
      }
    }
  }

  if (mlx_stream_trace_array_handle_generation(mlx_array{nullptr}) != 0) {
    fail("empty wrapper generation query must return 0");
  }

  if (mlx_stream_trace_array_handle_generation(a) == 0) {
    fail("base array generation query returned 0");
  }
  if (base_gen != 0 && mlx_stream_trace_array_handle_generation(a) != base_gen) {
    fail("base array generation query did not match emitted record");
  }

  uint64_t latest_b_set = 0;
  for (size_t i = records.size(); i > 0; --i) {
    const auto& r = records[i - 1];
    if (r.array_wrapper_ptr == b_wrapper && r.event == "array_set") {
      latest_b_set = r.c_handle_gen;
      break;
    }
  }
  if (latest_b_set == 0) {
    fail("missing array_set trace record for b wrapper");
  }
  if (mlx_stream_trace_array_handle_generation(b) != latest_b_set) {
    fail("array_set generation query mismatch");
  }

  mlx_array_free(a);
  mlx_array_free(b);
  mlx_array_free(c);

  for (int i = 0; i < 128; ++i) {
    mlx_array tmp = mlx_array_new_int(i);
    mlx_array_free(tmp);
  }

  auto all_records = parse_trace_file(path);
  if (all_records.size() < records.size()) {
    fail("trace file shrank between pre/post phases");
  }
  std::vector<TraceRecord> post_records;
  for (size_t i = records.size(); i < all_records.size(); ++i) {
    post_records.push_back(all_records[i]);
  }
  bool saw_reused_desc = false;
  for (const auto& r : post_records) {
    if (r.event != "array_new") {
      continue;
    }
    auto it = retired_desc_generations.find(r.array_data_ptr);
    if (it == retired_desc_generations.end()) {
      continue;
    }
    if (r.native_desc_gen <= it->second) {
      fail("retired descriptor reused with non-increasing generation");
    }
    saw_reused_desc = true;
    it->second = r.native_desc_gen;
  }
  if (!saw_reused_desc) {
    std::fprintf(
        stderr,
        "descriptor-generation force path not observed; live/reuse scenario not induced\n");
  }

  run_transport_probe_process(argv[0]);
  run_transport_oversized_probe_process(argv[0]);
  run_oversized_run_id_process(argv[0]);
  run_concurrent_trace_process(argv[0], path);
  run_path_security_processes(argv[0]);
  return 0;
}
