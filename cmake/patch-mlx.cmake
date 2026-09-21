# Apply only the reviewed patch to the pinned pristine MLX source.
if(NOT DEFINED MLX_SOURCE_DIR)
  message(FATAL_ERROR "MLX_SOURCE_DIR is required")
endif()
find_package(Git REQUIRED)
execute_process(COMMAND "${GIT_EXECUTABLE}" rev-parse HEAD
  WORKING_DIRECTORY "${MLX_SOURCE_DIR}" OUTPUT_VARIABLE revision
  OUTPUT_STRIP_TRAILING_WHITESPACE RESULT_VARIABLE status)
if(NOT status EQUAL 0 OR NOT revision STREQUAL "1f8e74e3f12f31365464a6867c6579f0e9b29d85")
  message(FATAL_ERROR "event/stream patch requires exact MLX v0.32.2 source")
endif()
set(patch "${CMAKE_CURRENT_LIST_DIR}/mlx-v0.32.2.patch")
file(SHA256 "${patch}" patch_hash)
if(NOT patch_hash STREQUAL "c7e4bc0745d8da285ac5097f34cc508cdc217d32574847b28defd7514c020b69")
  message(FATAL_ERROR "MLX patch checksum mismatch")
endif()
set(paths
  mlx/event.h
  mlx/backend/gpu/eval.h
  mlx/random.cpp
  tests/random_tests.cpp)
set(pristine
  56d49e3c5d71ffa94e484ef55762b25e627b4a9114ad1c138534b7a83826c8fb
  8c18ac14ca85348cda4ccf6fb87badf30fbc5723b2f35254145871c2444bdbe3
  d4ef694bedccbc9735cfe12918f676378cf351ce87a056c12821982c0a61cde6
  3fc4e7fb0481fff61dbe6b3776b152ef018e142db89bdc7b9bedbcc9c2374e9d)
set(patched
  cede132b4ec128eaab040f0ba577155bb032fea2f94b430bc5ac3d5861cfff5d
  681239fe618b4183107641a7a414c936b4fb95ccc508992f53982d978ea44d76
  44923f1f6f9c01f7a99db6952897618080a8a26825a7eff45ae2ef7dc0f8a44e
  cedfb7fb2854668228be2d561a231fe9dcd6445d825aacd762945300de6a2931)
set(all_pristine TRUE)
set(all_patched TRUE)
foreach(i RANGE 0 3)
  list(GET paths ${i} path)
  list(GET pristine ${i} before)
  list(GET patched ${i} after)
  file(SHA256 "${MLX_SOURCE_DIR}/${path}" actual)
  if(NOT actual STREQUAL before)
    set(all_pristine FALSE)
  endif()
  if(NOT actual STREQUAL after)
    set(all_patched FALSE)
  endif()
endforeach()
if(all_patched)
  message(STATUS "MLX patch ${patch_hash} already applied")
  return()
endif()
if(NOT all_pristine)
  message(FATAL_ERROR "MLX patch input differs from pristine or fully patched files")
endif()
execute_process(COMMAND "${GIT_EXECUTABLE}" apply --check "${patch}"
  WORKING_DIRECTORY "${MLX_SOURCE_DIR}" RESULT_VARIABLE status)
if(NOT status EQUAL 0)
  message(FATAL_ERROR "MLX patch context check failed")
endif()
execute_process(COMMAND "${GIT_EXECUTABLE}" apply "${patch}"
  WORKING_DIRECTORY "${MLX_SOURCE_DIR}" RESULT_VARIABLE status)
if(NOT status EQUAL 0)
  message(FATAL_ERROR "MLX patch application failed")
endif()
foreach(i RANGE 0 3)
  list(GET paths ${i} path)
  list(GET patched ${i} expected)
  file(SHA256 "${MLX_SOURCE_DIR}/${path}" actual)
  if(NOT actual STREQUAL expected)
    message(FATAL_ERROR "MLX patched file checksum mismatch: ${path}")
  endif()
endforeach()
message(STATUS "Applied checked MLX patch ${patch_hash}")
