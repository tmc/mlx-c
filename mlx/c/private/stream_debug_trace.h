/* Copyright © 2026 Apple Inc. */

#ifndef MLX_STREAM_DEBUG_TRACE_H
#define MLX_STREAM_DEBUG_TRACE_H

#include <cstddef>
#include <cstdint>

#include "mlx/c/array.h"

// Returns the trace generation of an array handle, or 0 when the trace runtime
// is not built. Exported so a trace harness can correlate handles without
// linking the runtime's internals; this header is its only declaration.
extern "C" uint64_t mlx_stream_trace_array_handle_generation(mlx_array arr);

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
    int64_t requested_stream_index);

void mlx_trace_array_handle_release(void* c_handle);

}  // namespace detail
}  // namespace mlx

#endif
