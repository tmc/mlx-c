/* Copyright © 2023-2024 Apple Inc. */

#include <cstdio>
#include <cstdlib>
#include <fstream>
#include <iostream>
#include <algorithm>
#include <sstream>
#include <string>
#include <unordered_map>
#include <unordered_set>
#include <vector>
#include <unistd.h>

#include "mlx/c/array.h"
#include "mlx/c/error.h"

namespace {

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
  uint64_t run_id = 0;
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
    if (!parse_u64_dec_field(line, "run_id", r.run_id) ||
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
    uint64_t expected_run_id) {
  for (const auto& r : records) {
    if (r.run_id != expected_run_id) {
      fail("trace run_id changed across records");
    }
    if (r.event.empty()) {
      fail("trace record missing event");
    }
  }
}

}  // namespace

int main() {
  char path[256];
  std::snprintf(
      path, sizeof(path), "/tmp/mlx-c-stream-trace-%d.log", static_cast<int>(getpid()));
  std::remove(path);
  setenv("MLX_STREAM_TRACE", path, 1);
  setenv("MLX_STREAM_TRACE_RUN_ID", "123456", 1);

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
  validate_records_common(records, 123456);

  if (records.front().run_id != 123456) {
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

  return 0;
}
