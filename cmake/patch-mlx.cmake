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
if(NOT patch_hash STREQUAL "5ecfb69a46beadb8536010ebecc627e2e4ad63996a7603963ce007d91ee57bc3")
  message(FATAL_ERROR "MLX patch checksum mismatch")
endif()
# The first eight files match core b76656e61d0a, except that device.cpp also
# checks the current CUDA context in make_current; mlx/export.cpp resolves
# imported streams on the importing thread. The rest carry upstream #4431,
# #4453 and #4420; the last three add an opt-in process-wide default stream
# and a way to clear it; mlx/io/load.cpp advances the pread offset after a
# short read.
# The patch header gives per-file provenance.
set(paths
  mlx/backend/cuda/device.cpp
  mlx/backend/cuda/rope.cu
  mlx/backend/cuda/worker.cpp
  mlx/backend/cuda/worker.h
  mlx/backend/gpu/eval.h
  mlx/event.h
  mlx/random.cpp
  tests/random_tests.cpp
  mlx/export.cpp
  mlx/backend/metal/kernels/sdpa_vector.h
  python/tests/test_fast_sdpa.py
  mlx/array.cpp
  mlx/array.h
  tests/array_tests.cpp
  tests/autograd_tests.cpp
  mlx/backend/common/utils.cpp
  tests/gpu_tests.cpp
  mlx/stream.cpp
  mlx/stream.h
  tests/export_import_tests.cpp
  mlx/io/load.cpp)
set(pristine
  6a5033019724d0c8e5282744f2e7f22427e9e6aa7a824f1b32d85f87c804594b
  79cb9ec596574b06aaec6e804dde9b4ef661b72522adc712545424ffbf894879
  408b47b67f6d6ae8afe59f3b51c8d28b0488d6b779f207e7101fbe9d9d9e093d
  9dc4107113c697184241d7023d3eaf4237c3a7447ba59f3390d231fe150b0a22
  8c18ac14ca85348cda4ccf6fb87badf30fbc5723b2f35254145871c2444bdbe3
  56d49e3c5d71ffa94e484ef55762b25e627b4a9114ad1c138534b7a83826c8fb
  d4ef694bedccbc9735cfe12918f676378cf351ce87a056c12821982c0a61cde6
  3fc4e7fb0481fff61dbe6b3776b152ef018e142db89bdc7b9bedbcc9c2374e9d
  7ccca449d5b98149da4a3be7d0f4305c7db7145e521cf81d31f91228620d8b69
  de098d50a67a865e2e64fb1700fc4819307861f009a65642679777a9a9cec7cd
  f5f93a35ad3626f0329c20627e16118d4259590b31d6e1e1a58ca51494a179a3
  6e1fb0a6b398f7dd95355f7f5a2338addbfc4a8c0f18137252315b2dfa25cb72
  5c90807aa44226c557c5bda740bdc1732109810e8862697ad5d374092669db65
  e378f29bd0ad8df6d854572d172be453d184cb374239a01a1ab8ae391f9eb8d5
  116b0179a28fae664b8cf9d24c09b85f90b69a8d9ea8a92bcecfe02bd7a55e52
  f831b4e6576cb76519fddcbab2e424dc53e8f75811cce448f4a877ec1885331e
  05447fe0435e81ffc269284723d51a8cdee8f79916fff38703eccb453d951986
  042c5cb2b06743a507ac42f8888c001ef461cd6498c33d5b80d2bbb79ba05998
  93e08b1178009b07f9a2a4758c03c1bca9ee1886d7ef6b733cc23bd867268d2e
  3ad2d40d2dbf0e8a15d7eba5885ce67b1e35cbe5cc951d18a5579955e5cea770
  54f08df8acedd2f925e4bf22db9bd401d5d4e2966d9b67dc649d1e9228d1ad0a)
set(patched
  037fd9ba2e38936e9334fa542a5612a2b78f1493bbb5e85dfdb1edd7afc9d575
  f3d7b0c4f2beafe8f84d1fa727271c0efc4fc4f3eb1035e498d8dd46a3b16c08
  253f932f0a3c20f6b690c0f36a015e2de22e030f63ce62baee9f24e5605b4179
  490b1d11914bca6506bf69fefa5f2369f5fc589ccf1ea82515333e90ccb9e27c
  681239fe618b4183107641a7a414c936b4fb95ccc508992f53982d978ea44d76
  cede132b4ec128eaab040f0ba577155bb032fea2f94b430bc5ac3d5861cfff5d
  44923f1f6f9c01f7a99db6952897618080a8a26825a7eff45ae2ef7dc0f8a44e
  cedfb7fb2854668228be2d561a231fe9dcd6445d825aacd762945300de6a2931
  e1acdaf61472ea2e44196798d34a621fd76fd52a3a76f4c33052727d4f93f8a5
  f52c8da21f2c31f3f77168d326a49815fc29bde4b573883896a8eb3298f11508
  b0fbf0ddf8b883105a04009a0a6437c3812e4b1341571200d68a798cb10dd5b9
  92d0e3d686eee37ead527e92aa4fc2d9fd893a8c039e7d4f18c404edc2ffbfac
  543d9096f3ea52870087059db5f793925dfb5e00097638776bc52b01e6189c3b
  2becb80ae139a874564624b688b2c77c119968846c0b26d4b1a7de7a01472ca7
  fed4e051fc804217ddbc24f7d4d2568dfcbfc58b0b2eba48cfc24d9527ec4ebb
  7704128ac8a95546585d01d6d2834b16f29cc865b82ea11a89c4674783c9752a
  490d32046744384471f9881aadc8b89c3673c4269ecaf9c0bf546f631dfce9d7
  386b73b7e25ced86c0be9f9af8a9ed00ebcddd9ca760c4ab0bc5a400eb79d271
  e4858dbb70984f81f4e8e6b9468f7bb74baf687b845cb968617662c82de69ff8
  6389a758ef907a3a5bc3bfea1dbffa7c711305bc95fd1f4cfbd3ca98433b56f7
  d07b768d44fbb31327020fcf11fd6a9f196b5b9d55edd3803fada198c764ad4f)
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
list(LENGTH paths path_count)
list(LENGTH pristine pristine_count)
list(LENGTH patched patched_count)
if(NOT path_count EQUAL pristine_count OR NOT path_count EQUAL patched_count)
  message(FATAL_ERROR "MLX patch checksum lists differ in length")
endif()
math(EXPR last_path "${path_count} - 1")
set(all_pristine TRUE)
set(all_patched TRUE)
foreach(i RANGE 0 ${last_path})
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
foreach(i RANGE 0 ${last_path})
  list(GET paths ${i} path)
  list(GET patched ${i} expected)
  file(SHA256 "${MLX_SOURCE_DIR}/${path}" actual)
  if(NOT actual STREQUAL expected)
    message(FATAL_ERROR "MLX patched file checksum mismatch: ${path}")
  endif()
endforeach()
message(STATUS "Applied checked MLX patch ${patch_hash}")
