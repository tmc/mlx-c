# Apply only the reviewed patch to the pinned pristine MLX source.
if(NOT DEFINED MLX_SOURCE_DIR)
  message(FATAL_ERROR "MLX_SOURCE_DIR is required")
endif()
find_package(Git REQUIRED)
execute_process(COMMAND "${GIT_EXECUTABLE}" rev-parse HEAD
  WORKING_DIRECTORY "${MLX_SOURCE_DIR}" OUTPUT_VARIABLE revision
  OUTPUT_STRIP_TRAILING_WHITESPACE RESULT_VARIABLE status)
if(NOT status EQUAL 0 OR NOT revision STREQUAL "1f8e74e3f12f31365464a6867c6579f0e9b29d85")
  message(FATAL_ERROR "MLX patch requires exact MLX v0.32.2 source")
endif()
set(patch "${CMAKE_CURRENT_LIST_DIR}/mlx-v0.32.2.patch")
file(SHA256 "${patch}" patch_hash)
if(NOT patch_hash STREQUAL "45d96fd272cc8f1918f497c0e1df4ca0570fec0c89e14e4874c5d0c1bbced95f")
  message(FATAL_ERROR "MLX patch checksum mismatch")
endif()
# Resulting tracked source matches core b76656e61d0aed0cd9fb74ae7554ad08429de97e.
set(paths
  mlx/backend/cuda/device.cpp
  mlx/backend/cuda/rope.cu
  mlx/backend/cuda/worker.cpp
  mlx/backend/cuda/worker.h
  mlx/backend/gpu/eval.h
  mlx/event.h
  mlx/random.cpp
  tests/random_tests.cpp)
set(pristine
  6a5033019724d0c8e5282744f2e7f22427e9e6aa7a824f1b32d85f87c804594b
  79cb9ec596574b06aaec6e804dde9b4ef661b72522adc712545424ffbf894879
  408b47b67f6d6ae8afe59f3b51c8d28b0488d6b779f207e7101fbe9d9d9e093d
  9dc4107113c697184241d7023d3eaf4237c3a7447ba59f3390d231fe150b0a22
  8c18ac14ca85348cda4ccf6fb87badf30fbc5723b2f35254145871c2444bdbe3
  56d49e3c5d71ffa94e484ef55762b25e627b4a9114ad1c138534b7a83826c8fb
  d4ef694bedccbc9735cfe12918f676378cf351ce87a056c12821982c0a61cde6
  3fc4e7fb0481fff61dbe6b3776b152ef018e142db89bdc7b9bedbcc9c2374e9d)
set(patched
  35705bc794d7cf49d1351771ee80ca7fa8c5083ca6b931eaf691616adf54d28e
  f3d7b0c4f2beafe8f84d1fa727271c0efc4fc4f3eb1035e498d8dd46a3b16c08
  253f932f0a3c20f6b690c0f36a015e2de22e030f63ce62baee9f24e5605b4179
  490b1d11914bca6506bf69fefa5f2369f5fc589ccf1ea82515333e90ccb9e27c
  681239fe618b4183107641a7a414c936b4fb95ccc508992f53982d978ea44d76
  cede132b4ec128eaab040f0ba577155bb032fea2f94b430bc5ac3d5861cfff5d
  44923f1f6f9c01f7a99db6952897618080a8a26825a7eff45ae2ef7dc0f8a44e
  cedfb7fb2854668228be2d561a231fe9dcd6445d825aacd762945300de6a2931)
# Only the checked patch may differ from the pinned tracked source.
execute_process(COMMAND "${GIT_EXECUTABLE}" diff --cached --quiet
  WORKING_DIRECTORY "${MLX_SOURCE_DIR}" RESULT_VARIABLE status)
if(NOT status EQUAL 0)
  message(FATAL_ERROR "MLX patch requires an unchanged index")
endif()
execute_process(COMMAND "${GIT_EXECUTABLE}" ls-files --others --exclude-standard
  WORKING_DIRECTORY "${MLX_SOURCE_DIR}" OUTPUT_VARIABLE untracked
  RESULT_VARIABLE status)
if(NOT status EQUAL 0 OR NOT untracked STREQUAL "")
  message(FATAL_ERROR "MLX patch source contains untracked files")
endif()
execute_process(COMMAND "${GIT_EXECUTABLE}" diff --name-only --no-renames HEAD
  WORKING_DIRECTORY "${MLX_SOURCE_DIR}" OUTPUT_VARIABLE changed
  OUTPUT_STRIP_TRAILING_WHITESPACE RESULT_VARIABLE status)
if(NOT status EQUAL 0)
  message(FATAL_ERROR "MLX patch source status check failed")
endif()
string(REPLACE "\n" ";" changed "${changed}")
foreach(path IN LISTS changed)
  list(FIND paths "${path}" path_index)
  if(path_index EQUAL -1)
    message(FATAL_ERROR "MLX patch source has unrelated changes: ${path}")
  endif()
endforeach()
execute_process(COMMAND "${GIT_EXECUTABLE}" diff --summary HEAD
  WORKING_DIRECTORY "${MLX_SOURCE_DIR}" OUTPUT_VARIABLE metadata
  RESULT_VARIABLE status)
if(NOT status EQUAL 0 OR NOT metadata STREQUAL "")
  message(FATAL_ERROR "MLX patch source has file mode or type changes")
endif()
set(all_pristine TRUE)
set(all_patched TRUE)
foreach(i RANGE 0 7)
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
foreach(i RANGE 0 7)
  list(GET paths ${i} path)
  list(GET patched ${i} expected)
  file(SHA256 "${MLX_SOURCE_DIR}/${path}" actual)
  if(NOT actual STREQUAL expected)
    message(FATAL_ERROR "MLX patched file checksum mismatch: ${path}")
  endif()
endforeach()
message(STATUS "Applied checked MLX patch ${patch_hash}")
