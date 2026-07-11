/* Copyright © 2023-2024 Apple Inc. */

#include <algorithm>
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <fstream>
#include <iostream>
#include <sstream>
#include <cstddef>
#include <string>
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

struct TraceRecord {
  uint64_t seq = 0;
  std::string run_id;
  uint64_t c_handle_gen = 0;
  uint64_t native_desc_gen = 0;
  uint64_t array_data_ptr = 0;
  uint64_t array_wrapper_ptr = 0;
  uint64_t caller_pc = 0;
  std::string event;
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
        !parse_u64_dec_field(line, "c_handle_gen", r.c_handle_gen) ||
        !parse_u64_dec_field(line, "native_desc_gen", r.native_desc_gen) ||
        !parse_u64_hex_field(line, "array_data_ptr", r.array_data_ptr) ||
        !parse_u64_hex_field(line, "array_wrapper_ptr", r.array_wrapper_ptr) ||
        !parse_u64_hex_field(line, "caller_pc", r.caller_pc) ||
        !parse_string_field(line, "event", r.event)) {
      fail("trace file contains unparsable line");
    }
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
  validate_records_common(records, oversized_run_id);
  for (const auto& r : records) {
    if (r.run_id != oversized_run_id) {
      fail("oversized run_id test did not preserve exact run_id");
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
  if (argc > 1 && std::strcmp(argv[1], "--concurrent") == 0) {
    run_concurrent_trace_worker();
    return 0;
  }
  if (argc > 1 && std::strcmp(argv[1], "--oversized-run-id") == 0) {
    run_oversized_run_id_mode();
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

  run_oversized_run_id_process(argv[0]);
  run_concurrent_trace_process(argv[0], path);
  return 0;
}
