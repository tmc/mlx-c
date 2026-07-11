find_package(Threads REQUIRED)

add_executable(mlx-c-stream-test stream_test.cpp)
target_link_libraries(mlx-c-stream-test PRIVATE mlxc Threads::Threads)
add_test(NAME mlx-c-stream COMMAND mlx-c-stream-test)

add_executable(mlx-c-stream-trace-test stream_trace_test.cpp)
target_link_libraries(mlx-c-stream-trace-test PRIVATE mlxc Threads::Threads)
if(MLX_STREAM_TRACE_BUILD)
  target_compile_definitions(mlx-c-stream-trace-test PRIVATE MLX_STREAM_TRACE_BUILD)
endif()
add_test(NAME mlx-c-stream-trace COMMAND mlx-c-stream-trace-test)

add_executable(mlx-c-event-test event_test.cpp)
target_link_libraries(mlx-c-event-test PRIVATE mlxc mlx Threads::Threads)
add_test(NAME mlx-c-event COMMAND mlx-c-event-test)

add_executable(mlx-c-event-consumer event_consumer.c)
set_property(TARGET mlx-c-event-consumer PROPERTY LINKER_LANGUAGE CXX)
target_link_libraries(mlx-c-event-consumer PRIVATE mlxc)
add_test(NAME mlx-c-event-consumer COMMAND mlx-c-event-consumer)

add_executable(mlx-c-event-benchmark event_benchmark.cpp)
target_link_libraries(mlx-c-event-benchmark PRIVATE mlxc)
