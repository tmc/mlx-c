# Fix MLX v0.32.0's Stream::operator< to be a strict weak ordering.
#
# Upstream v0.32.0's comparator is `device < rhs.device || index < rhs.index`,
# which is not transitive: (cpu,5) < (gpu,3) and (gpu,3) < (cpu,1) hold while
# (cpu,5) < (cpu,1) does not. mlx-c's thread-local stream token resolution
# keys a std::map on Stream, so resolving a CPU and a GPU token on the same
# thread is UB until this is fixed. Ratified 2026-08-23 as a mandatory carry-in
# alongside the MLX_API event export patch.

set(stream_header "${MLX_SOURCE_DIR}/mlx/stream.h")
file(READ "${stream_header}" contents)

if(contents MATCHES "std::tie\\(device, index\\)")
  # Already patched (or fixed upstream) — nothing to do.
  return()
endif()

if(NOT contents MATCHES "return device < rhs.device \\|\\| index < rhs.index;")
  message(FATAL_ERROR "cannot find Stream::operator< body in ${stream_header}; \
upstream may have fixed the ordering itself — re-evaluate this patch")
endif()

if(NOT contents MATCHES "#include <tuple>")
  string(REPLACE "#include <vector>" "#include <tuple>\n#include <vector>"
         contents "${contents}")
  if(NOT contents MATCHES "#include <tuple>")
    message(FATAL_ERROR "cannot find an include anchor in ${stream_header}")
  endif()
endif()

string(REPLACE
       "return device < rhs.device || index < rhs.index;"
       "return std::tie(device, index) < std::tie(rhs.device, rhs.index);"
       contents
       "${contents}")
file(WRITE "${stream_header}" "${contents}")
