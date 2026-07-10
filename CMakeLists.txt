# Copyright © 2023-2024 Apple Inc.

cmake_minimum_required(VERSION 3.16)

# ----------------------------- Setup -----------------------------
project(mlx.data LANGUAGES CXX C)
include(CTest)
set(CMAKE_CXX_STANDARD 20)
set(CMAKE_EXPORT_COMPILE_COMMANDS
    ON
    CACHE BOOL "Export compile_commands.json for generator parsing")

# ----------------------------- Configuration -----------------------------
if(NOT MLX_C_VERSION)
  set(MLX_C_VERSION 0.6.0)
endif()

option(BUILD_SHARED_LIBS "Build mlx C as a shared library" OFF)
option(MLX_C_BUILD_EXAMPLES "Build examples for mlx C" ON)
option(MLX_C_BUILD_JACCL "Build the standalone JACCL C API when available" ON)
option(MLX_C_USE_SYSTEM_MLX "Use system MLX" OFF)
set(MLX_C_COMPILER_LAUNCHER
    "auto"
    CACHE STRING
          "Compiler launcher for C/C++ builds: auto, ccache, sccache, a path, or empty to disable")
set_property(CACHE MLX_C_COMPILER_LAUNCHER PROPERTY STRINGS "" auto ccache
                                                      sccache)

if(MLX_C_COMPILER_LAUNCHER)
  if(MLX_C_COMPILER_LAUNCHER STREQUAL "auto")
    find_program(MLX_C_DETECTED_COMPILER_LAUNCHER sccache)
    if(NOT MLX_C_DETECTED_COMPILER_LAUNCHER)
      find_program(MLX_C_DETECTED_COMPILER_LAUNCHER ccache)
    endif()
  else()
    find_program(MLX_C_DETECTED_COMPILER_LAUNCHER
                 ${MLX_C_COMPILER_LAUNCHER})
    if(NOT MLX_C_DETECTED_COMPILER_LAUNCHER
       AND EXISTS "${MLX_C_COMPILER_LAUNCHER}")
      set(MLX_C_DETECTED_COMPILER_LAUNCHER ${MLX_C_COMPILER_LAUNCHER})
    endif()
    if(NOT MLX_C_DETECTED_COMPILER_LAUNCHER)
      message(FATAL_ERROR "compiler launcher not found: "
                          "${MLX_C_COMPILER_LAUNCHER}")
    endif()
  endif()

  if(MLX_C_DETECTED_COMPILER_LAUNCHER)
    set(CMAKE_C_COMPILER_LAUNCHER ${MLX_C_DETECTED_COMPILER_LAUNCHER})
    set(CMAKE_CXX_COMPILER_LAUNCHER ${MLX_C_DETECTED_COMPILER_LAUNCHER})
    message(STATUS "Using compiler launcher: "
                   "${MLX_C_DETECTED_COMPILER_LAUNCHER}")
  endif()
endif()

# ----------------------------- mlx -----------------------------

include(FetchContent)
# Avoid warning about DOWNLOAD_EXTRACT_TIMESTAMP in CMake 3.24:
if(POLICY CMP0135)
  cmake_policy(SET CMP0135 NEW)
endif()

# mlx
set(MLX_BUILD_TESTS OFF)
set(MLX_BUILD_EXAMPLES OFF)
set(MLX_BUILD_BENCHMARKS OFF)
set(MLX_BUILD_PYTHON_BINDINGS OFF)

if(MLX_C_USE_SYSTEM_MLX)
  find_package(MLX REQUIRED)
else()
  set(MLX_C_FETCH_MLX_ARGS
      GIT_REPOSITORY "https://github.com/ml-explore/mlx.git"
      GIT_TAG v0.32.0
      PATCH_COMMAND
        ${CMAKE_COMMAND} -DMLX_SOURCE_DIR=<SOURCE_DIR> -P
        ${CMAKE_CURRENT_LIST_DIR}/cmake/export-mlx-event.cmake)
  if(CMAKE_VERSION VERSION_GREATER_EQUAL 3.28)
    list(APPEND MLX_C_FETCH_MLX_ARGS EXCLUDE_FROM_ALL)
    FetchContent_Declare(
      mlx
      ${MLX_C_FETCH_MLX_ARGS})
    FetchContent_MakeAvailable(mlx)
  else()
    FetchContent_Declare(
      mlx
      ${MLX_C_FETCH_MLX_ARGS})
    FetchContent_GetProperties(mlx)
    if(NOT mlx_POPULATED)
      FetchContent_Populate(mlx)
      add_subdirectory(${mlx_SOURCE_DIR} ${mlx_BINARY_DIR} EXCLUDE_FROM_ALL)
    endif()
  endif()
endif()

# ----------------------------- lib -----------------------------

set(mlxc-src
    ${CMAKE_CURRENT_LIST_DIR}/mlx/c/array.cpp
    ${CMAKE_CURRENT_LIST_DIR}/mlx/c/closure.cpp
    ${CMAKE_CURRENT_LIST_DIR}/mlx/c/compile.cpp
    ${CMAKE_CURRENT_LIST_DIR}/mlx/c/cuda.cpp
    ${CMAKE_CURRENT_LIST_DIR}/mlx/c/device.cpp
    ${CMAKE_CURRENT_LIST_DIR}/mlx/c/distributed.cpp
    ${CMAKE_CURRENT_LIST_DIR}/mlx/c/distributed_group.cpp
    ${CMAKE_CURRENT_LIST_DIR}/mlx/c/error.cpp
    ${CMAKE_CURRENT_LIST_DIR}/mlx/c/event.cpp
    ${CMAKE_CURRENT_LIST_DIR}/mlx/c/export.cpp
    ${CMAKE_CURRENT_LIST_DIR}/mlx/c/fast.cpp
    ${CMAKE_CURRENT_LIST_DIR}/mlx/c/fft.cpp
    ${CMAKE_CURRENT_LIST_DIR}/mlx/c/graph_utils.cpp
    ${CMAKE_CURRENT_LIST_DIR}/mlx/c/io.cpp
    ${CMAKE_CURRENT_LIST_DIR}/mlx/c/io_types.cpp
    ${CMAKE_CURRENT_LIST_DIR}/mlx/c/linalg.cpp
    ${CMAKE_CURRENT_LIST_DIR}/mlx/c/map.cpp
    ${CMAKE_CURRENT_LIST_DIR}/mlx/c/memory.cpp
    ${CMAKE_CURRENT_LIST_DIR}/mlx/c/metal.cpp
    # ${CMAKE_CURRENT_LIST_DIR}/mlx/c/object.cpp
    ${CMAKE_CURRENT_LIST_DIR}/mlx/c/ops.cpp
    ${CMAKE_CURRENT_LIST_DIR}/mlx/c/random.cpp
    ${CMAKE_CURRENT_LIST_DIR}/mlx/c/stream.cpp
    ${CMAKE_CURRENT_LIST_DIR}/mlx/c/string.cpp
    ${CMAKE_CURRENT_LIST_DIR}/mlx/c/transforms.cpp
    ${CMAKE_CURRENT_LIST_DIR}/mlx/c/transforms_impl.cpp
    ${CMAKE_CURRENT_LIST_DIR}/mlx/c/vector.cpp
    ${CMAKE_CURRENT_LIST_DIR}/mlx/c/version.cpp)

add_library(mlxc ${mlxc-src})

target_link_libraries(mlxc PRIVATE mlx)
set_property(TARGET mlxc PROPERTY POSITION_INDEPENDENT_CODE ON)

set(MLX_C_INSTALL_TARGETS mlxc)
set(MLX_C_HAS_JACCL OFF)

if(MLX_C_BUILD_JACCL AND TARGET jaccl)
  add_library(jacclc ${CMAKE_CURRENT_LIST_DIR}/mlx/c/jaccl.cpp)
  target_link_libraries(jacclc PRIVATE jaccl)
  FetchContent_GetProperties(json)
  if(json_POPULATED
     AND EXISTS "${json_SOURCE_DIR}/single_include/nlohmann/json.hpp")
    target_include_directories(
      jacclc
      PRIVATE $<BUILD_INTERFACE:${json_SOURCE_DIR}/single_include/nlohmann>)
  endif()
  set_property(TARGET jacclc PROPERTY POSITION_INDEPENDENT_CODE ON)
  list(APPEND MLX_C_INSTALL_TARGETS jacclc)
  set(MLX_C_HAS_JACCL ON)
elseif(MLX_C_BUILD_JACCL)
  message(STATUS "JACCL C API disabled: jaccl target is not available")
endif()

set(MLX_C_HAS_JACCL_DEFINE 0)
if(MLX_C_HAS_JACCL)
  set(MLX_C_HAS_JACCL_DEFINE 1)
endif()
set(MLX_C_GENERATED_INCLUDE_DIR ${CMAKE_CURRENT_BINARY_DIR}/include)
set(MLX_C_CONFIG_HEADER ${MLX_C_GENERATED_INCLUDE_DIR}/mlx/c/config.h)
file(MAKE_DIRECTORY ${MLX_C_GENERATED_INCLUDE_DIR}/mlx/c)
configure_file(${CMAKE_CURRENT_LIST_DIR}/mlx/c/config.h.in
               ${MLX_C_CONFIG_HEADER} @ONLY)

target_include_directories(
  mlxc
  PUBLIC $<BUILD_INTERFACE:${MLX_C_GENERATED_INCLUDE_DIR}>
         $<BUILD_INTERFACE:${CMAKE_CURRENT_LIST_DIR}>
         $<INSTALL_INTERFACE:${CMAKE_INSTALL_INCLUDEDIR}>)
if(TARGET jacclc)
  target_include_directories(
    jacclc
    PUBLIC $<BUILD_INTERFACE:${MLX_C_GENERATED_INCLUDE_DIR}>
           $<BUILD_INTERFACE:${CMAKE_CURRENT_LIST_DIR}>
           $<INSTALL_INTERFACE:${CMAKE_INSTALL_INCLUDEDIR}>)
endif()

# Windows DLL symbol exports
if(WIN32)
  set_target_properties(mlxc PROPERTIES WINDOWS_EXPORT_ALL_SYMBOLS TRUE)
  if(TARGET jacclc)
    set_target_properties(jacclc PROPERTIES WINDOWS_EXPORT_ALL_SYMBOLS TRUE)
  endif()
endif()

if(MLX_C_BUILD_EXAMPLES)
  add_subdirectory(${CMAKE_CURRENT_LIST_DIR}/examples)
endif()

if(BUILD_TESTING)
  add_subdirectory(${CMAKE_CURRENT_LIST_DIR}/tests)
endif()

# ----------------------------- Installation -----------------------------
include(GNUInstallDirs)

# Install library
install(
  TARGETS ${MLX_C_INSTALL_TARGETS}
  EXPORT MLXCTargets
  LIBRARY DESTINATION ${CMAKE_INSTALL_LIBDIR}
  ARCHIVE DESTINATION ${CMAKE_INSTALL_LIBDIR}
  RUNTIME DESTINATION ${CMAKE_INSTALL_BINDIR}
  INCLUDES
  DESTINATION ${CMAKE_INSTALL_INCLUDEDIR})

# Install headers
install(
  DIRECTORY ${CMAKE_CURRENT_LIST_DIR}/mlx
  DESTINATION ${CMAKE_INSTALL_INCLUDEDIR}
  COMPONENT headers
  FILES_MATCHING
  PATTERN "*.h"
  PATTERN "config.h" EXCLUDE
  PATTERN "jaccl.h" EXCLUDE
  PATTERN "private" EXCLUDE)

install(FILES ${MLX_C_CONFIG_HEADER}
        DESTINATION ${CMAKE_INSTALL_INCLUDEDIR}/mlx/c COMPONENT headers)

if(MLX_C_HAS_JACCL)
  install(FILES ${CMAKE_CURRENT_LIST_DIR}/mlx/c/jaccl.h
          DESTINATION ${CMAKE_INSTALL_INCLUDEDIR}/mlx/c COMPONENT headers)
endif()

if(NOT MLX_C_USE_SYSTEM_MLX AND TARGET mlx)
  install(
    TARGETS mlx
    LIBRARY DESTINATION ${CMAKE_INSTALL_LIBDIR}
    ARCHIVE DESTINATION ${CMAKE_INSTALL_LIBDIR}
    RUNTIME DESTINATION ${CMAKE_INSTALL_BINDIR})
  if(MLX_BUILD_METAL)
    install(FILES ${mlx_BINARY_DIR}/mlx/backend/metal/kernels/mlx.metallib
            DESTINATION ${CMAKE_INSTALL_LIBDIR} COMPONENT metallib)
  endif()
endif()

if(TARGET jaccl)
  install(
    TARGETS jaccl
    LIBRARY DESTINATION ${CMAKE_INSTALL_LIBDIR}
    ARCHIVE DESTINATION ${CMAKE_INSTALL_LIBDIR}
    RUNTIME DESTINATION ${CMAKE_INSTALL_BINDIR}
    INCLUDES
    DESTINATION ${CMAKE_INSTALL_INCLUDEDIR})
  if(NOT MLX_C_USE_SYSTEM_MLX)
    install(
      DIRECTORY ${mlx_SOURCE_DIR}/mlx/distributed/jaccl/lib/jaccl/
      DESTINATION ${CMAKE_INSTALL_INCLUDEDIR}/jaccl
      FILES_MATCHING
      PATTERN "*.h")
  endif()
endif()

# Install cmake config
set(MLX_C_CMAKE_BUILD_CONFIG ${CMAKE_BINARY_DIR}/MLXCConfig.cmake)
set(MLX_C_CMAKE_BUILD_VERSION_CONFIG
    ${CMAKE_BINARY_DIR}/MLXCConfigVersion.cmake)
set(MLX_C_CMAKE_INSTALL_MODULE_DIR share/cmake/MLXC)

install(
  EXPORT MLXCTargets
  FILE MLXCTargets.cmake
  DESTINATION ${MLX_C_CMAKE_INSTALL_MODULE_DIR})

include(CMakePackageConfigHelpers)

write_basic_package_version_file(
  ${MLX_C_CMAKE_BUILD_VERSION_CONFIG}
  COMPATIBILITY SameMajorVersion
  VERSION ${MLX_C_VERSION})

configure_package_config_file(
  ${CMAKE_CURRENT_LIST_DIR}/mlx-c.pc.in ${MLX_C_CMAKE_BUILD_CONFIG}
  INSTALL_DESTINATION ${MLX_C_CMAKE_INSTALL_MODULE_DIR}
  NO_CHECK_REQUIRED_COMPONENTS_MACRO
  PATH_VARS CMAKE_INSTALL_LIBDIR CMAKE_INSTALL_INCLUDEDIR
            MLX_C_CMAKE_INSTALL_MODULE_DIR)

install(FILES ${MLX_C_CMAKE_BUILD_CONFIG} ${MLX_C_CMAKE_BUILD_VERSION_CONFIG}
        DESTINATION ${MLX_C_CMAKE_INSTALL_MODULE_DIR})
