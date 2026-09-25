#pragma once

#include <vector>
#include "mlx/stream.h"

// Imports retain their streams. Select the wrapper global defaults while the
// graph (including lazy CPU constants) is read, then restore this thread.
// Snapshotting an unset default may create a thread-local stream.
class mlx_import_stream_guard_ {
 public:
  mlx_import_stream_guard_();
  ~mlx_import_stream_guard_();
  mlx_import_stream_guard_(const mlx_import_stream_guard_&) = delete;
  mlx_import_stream_guard_& operator=(const mlx_import_stream_guard_&) = delete;

 private:
  void restore();
  std::vector<mlx::core::Stream> defaults_;
};
