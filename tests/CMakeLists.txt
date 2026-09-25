find_package(Threads REQUIRED)
foreach(name stream_global categorical_stream)
  add_executable(mlxc_test_${name} ${name}.cpp)
  target_link_libraries(mlxc_test_${name} PRIVATE mlxc mlx Threads::Threads)
  add_test(NAME mlxc_${name} COMMAND mlxc_test_${name})
  set_tests_properties(mlxc_${name} PROPERTIES TIMEOUT 15)
endforeach()

# CPU streams in a GPU build use that GPU backend's event implementation.
if(NOT MLX_BUILD_METAL AND NOT MLX_BUILD_CUDA)
  add_executable(mlxc_test_event event.cpp)
  target_link_libraries(mlxc_test_event PRIVATE mlxc Threads::Threads)
  add_test(NAME mlxc_event_cpu COMMAND mlxc_test_event)
  set_tests_properties(mlxc_event_cpu PROPERTIES TIMEOUT 15)
endif()

if(MLX_BUILD_METAL)
  add_executable(mlxc_test_event_metal event_metal.cpp)
  target_link_libraries(mlxc_test_event_metal PRIVATE mlxc mlx Threads::Threads)
  foreach(mode signal_finalize wait_first recorded_pending)
    add_test(NAME mlxc_metal_${mode} COMMAND mlxc_test_event_metal ${mode})
    set_tests_properties(mlxc_metal_${mode} PROPERTIES TIMEOUT 15)
  endforeach()
endif()

if(MLX_BUILD_METAL OR MLX_BUILD_CUDA)
  add_test(NAME mlxc_stream_global_gpu COMMAND mlxc_test_stream_global gpu)
  add_test(NAME mlxc_categorical_stream_gpu COMMAND mlxc_test_categorical_stream gpu)
  set_tests_properties(mlxc_stream_global_gpu mlxc_categorical_stream_gpu PROPERTIES TIMEOUT 30)
endif()

# Compile-cache metadata is exercised with explicit CPU streams and no fusion.
if(MLX_BUILD_CPU)
  add_executable(mlxc_test_compile_cache compile_cache.cpp)
  target_link_libraries(mlxc_test_compile_cache PRIVATE mlxc mlx Threads::Threads)
  add_test(NAME mlxc_compile_cache_cpu COMMAND mlxc_test_compile_cache)
  set_tests_properties(mlxc_compile_cache_cpu PROPERTIES TIMEOUT 15)
endif()

if(MLX_BUILD_CPU)
  add_executable(mlxc_test_stream_thread_local stream_thread_local.cpp)
  target_link_libraries(mlxc_test_stream_thread_local PRIVATE mlxc mlx Threads::Threads)
  add_test(NAME mlxc_stream_thread_local_cpu COMMAND mlxc_test_stream_thread_local)
  set_tests_properties(mlxc_stream_thread_local_cpu PROPERTIES TIMEOUT 15)
endif()

# Imported graphs retain streams, so each mode exports and imports its own file.
if(MLX_BUILD_CPU)
  add_executable(mlxc_test_import_stream import_stream.cpp)
  target_link_libraries(mlxc_test_import_stream PRIVATE mlxc mlx Threads::Threads)
  set(import_stream_modes cpu)
  if(MLX_BUILD_METAL OR MLX_BUILD_CUDA)
    list(APPEND import_stream_modes gpu mixed)
  endif()
  foreach(mode ${import_stream_modes})
    add_test(NAME mlxc_import_stream_${mode}
             COMMAND mlxc_test_import_stream
                     ${CMAKE_CURRENT_BINARY_DIR}/import_stream_${mode}.mlxfn ${mode})
    set_tests_properties(mlxc_import_stream_${mode} PROPERTIES TIMEOUT 60)
  endforeach()
endif()

# Calls without a stream on fresh threads must not create one.
if(MLX_BUILD_CPU)
  add_executable(mlxc_test_fresh_thread fresh_thread.cpp)
  target_link_libraries(mlxc_test_fresh_thread PRIVATE mlxc mlx Threads::Threads)
  set(fresh_thread_device cpu)
  if(MLX_BUILD_METAL OR MLX_BUILD_CUDA)
    set(fresh_thread_device gpu)
    add_test(NAME mlxc_fresh_thread_never_set
             COMMAND mlxc_test_fresh_thread never-set)
    set_tests_properties(mlxc_fresh_thread_never_set PROPERTIES TIMEOUT 60)
  endif()
  add_test(NAME mlxc_fresh_thread_tripwire
           COMMAND mlxc_test_fresh_thread tripwire
                   ${CMAKE_CURRENT_BINARY_DIR}/fresh_thread.mlxfn
                   ${fresh_thread_device})
  set_tests_properties(mlxc_fresh_thread_tripwire PROPERTIES TIMEOUT 60)
  add_test(NAME mlxc_fresh_thread_clear COMMAND mlxc_test_fresh_thread clear)
  set_tests_properties(mlxc_fresh_thread_clear PROPERTIES TIMEOUT 60)
endif()

# make_current restores the primary context after a foreign cuCtxSetCurrent.
if(MLX_BUILD_CUDA)
  find_package(CUDAToolkit REQUIRED)
  add_executable(mlxc_test_cuda_context cuda_context.cpp)
  target_link_libraries(mlxc_test_cuda_context PRIVATE mlxc CUDA::cuda_driver)
  foreach(mode matmul ew)
    add_test(NAME mlxc_cuda_context_${mode}
             COMMAND mlxc_test_cuda_context ${mode})
    set_tests_properties(mlxc_cuda_context_${mode} PROPERTIES TIMEOUT 60
                         RESOURCE_LOCK gpu)
  endforeach()
endif()

# Upstream 0.32.2 regressions (#4431 sdpa, #4453 array assignment, #4420 scan).
# Each mode is red on the pristine pin and green with the matching fix. The
# scan modes assert an unrelated canary allocation, because the defect is an
# out-of-bounds write whose own numerical output is correct.
if(MLX_BUILD_CPU)
  add_executable(mlxc_test_regressions regressions.cpp)
  target_link_libraries(mlxc_test_regressions PRIVATE mlx Threads::Threads)
  add_test(NAME mlxc_regression_assignment COMMAND mlxc_test_regressions assignment)
  set_tests_properties(mlxc_regression_assignment PROPERTIES TIMEOUT 60)
  if(MLX_BUILD_METAL OR MLX_BUILD_CUDA)
    foreach(mode attention scan scan-axis0)
      add_test(NAME mlxc_regression_${mode} COMMAND mlxc_test_regressions ${mode})
    endforeach()
    # The attention case allocates 2x4x8192x128 K and V twice over.
    set_tests_properties(mlxc_regression_attention PROPERTIES TIMEOUT 300)
    set_tests_properties(mlxc_regression_scan mlxc_regression_scan-axis0
                         PROPERTIES TIMEOUT 60)
    # One GPU at a time: these are correctness canaries, not throughput tests.
    set_tests_properties(mlxc_regression_attention mlxc_regression_scan
                         mlxc_regression_scan-axis0 PROPERTIES RESOURCE_LOCK gpu)
  endif()
endif()
