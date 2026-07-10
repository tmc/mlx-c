find_package(Threads REQUIRED)

add_executable(mlx-c-stream-test stream_test.cpp)
target_link_libraries(mlx-c-stream-test PRIVATE mlxc Threads::Threads)
add_test(NAME mlx-c-stream COMMAND mlx-c-stream-test)
