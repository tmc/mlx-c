set(event_header "${MLX_SOURCE_DIR}/mlx/event.h")
file(READ "${event_header}" contents)

if(NOT contents MATCHES "class MLX_API Event")
  if(NOT contents MATCHES "class Event")
    message(FATAL_ERROR "cannot find MLX Event declaration in ${event_header}")
  endif()
  string(REPLACE "class Event" "class MLX_API Event" contents "${contents}")
  file(WRITE "${event_header}" "${contents}")
endif()

set(eval_header "${MLX_SOURCE_DIR}/mlx/backend/gpu/eval.h")
file(READ "${eval_header}" contents)
if(NOT contents MATCHES "MLX_API void finalize")
  if(NOT contents MATCHES "void finalize")
    message(FATAL_ERROR "cannot find MLX GPU finalize declaration in ${eval_header}")
  endif()
  string(REPLACE
         "void finalize(Stream s);"
         "MLX_API void finalize(Stream s);"
         contents
         "${contents}")
  file(WRITE "${eval_header}" "${contents}")
endif()
