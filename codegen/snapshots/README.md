# API migration snapshots

These public C declarations were extracted before candidate edits. They are independent of the canonical API lock.

| Input | Revision | Functions | JSON SHA-256 |
|---|---|---:|---|
| Upstream | ebc88f10caa1b625e6b581437a8dea6df8a70085 | 674 | ca1ec35701c03e7f2455a0e9f649a73a2f09e4b6c5e2d8e3cd0a20c90b570237 |
| Downstream | 78f68783f44f90a7aac853d34c7679e4a3c8cd93 | 652 | 0af5af0689ece5c68e25706d6d4951e9ef71ee5ab0e8fc4946206a4622293fdb |

Generator: in-repository tools/mlx-c-gen at 461029853235639b05d75d7ab59d887b2b535dda. Build from an isolated checkout using `GOWORK=off go build -o "$HOME/tmp/mlx-c-gen-snapshot" ./tools/mlx-c-gen`, then run `mlx-c-gen-snapshot lock -headers /absolute/pinned/checkout/mlx/c -lock output.json`. The executable used here has SHA-256 81bd41e7912feda7761a1c06ac071186ff2b88ca0d719aa4326590db32ce0472. This CLI uses apilock.Generate, the same header parser as GenerateTarget; jaccl is optional in this tree. Do not substitute gen/main's incompatible parser.

baseline-delta.json compares function names and complete signature strings. Thirteen same-name strings differ: ten operation argument layouts, Metal path output ownership, and two top-level by-value const qualifiers on compile-cache arguments. The qualifiers are not ABI layout changes. Thirty-four upstream names are added and twelve downstream names are absent upstream. The final candidate snapshot is recorded below, separately from these two baselines.

## Required checks outside this lock

- Five same-name defaults have changed scope without signature changes: mlx_get_default_stream, mlx_set_default_stream, mlx_synchronize_default, mlx_default_cpu_stream_new and mlx_default_gpu_stream_new. Candidate Global extensions preserve downstream behavior; upstream originals retain upstream scope.
- Ordinary stream creation has changed cross-thread behavior with the same signature; Go must select upstream mlx_stream_new_thread_unsafe explicitly.
- private/stream.h C++ conversion, ownership and validation helpers are outside the public umbrella and parser.
- Nested callback prototypes and actual returned int/size_t ABI require independent C invocation probes. A struct made of same-sized function pointers does not prove callback compatibility.
- Opaque handles do not describe pointee ownership or compile-cache capture semantics. Metal string copying/freeing, TLS shared owner and cache capture need lifetime tests.
- Resolving a default on one native thread does not establish the thread used by the next FFI call. Test implicit intermediate placement, including categorical expand_dims, and lazy evaluation.
- Event host publication, GPU progress and free/use safety require backend-specific bounded native tests with consequential negative controls.
- Locks describe declarations, not actual loaded bytes. Mach-O/ELF export checks and actual library paths/hashes are separate evidence.

No native runtime acceptance is inferred from these snapshots.

## Candidate

Wrapper implementation 18501621c3dd6bed63c2910481a1b40ba7b21e6e adds exactly six event functions and five Global default extensions. All 674 upstream function signature strings remain unchanged. Candidate snapshot contains 685 functions, SHA-256 e19c622cf34fef1a478fc848dc0c97bb1a0479d0487d6b58e412f7b255afcb50.

The original core implementation was 8af4c14ae1cd89c741e409defb101f51bcc53df9, with checked patch SHA-256 c7e4bc0745d8da285ac5097f34cc508cdc217d32574847b28defd7514c020b69. Historical native receipts retain that source identity.

The previous checked patch added the reviewed CUDA worker shutdown changes, producing core 3d974d880c8cc224c270bbdd1d7acd1d09b3a7c9 with SHA-256 f0395620ec82d78e6520ae46ddda7dacdf0e3d7f9d6c5d00eba39a5b0ba8adcb.

The current checked patch also gives CUDA RoPE a row contiguous output for inputs whose layout is not dense, and produces core tree b76656e61d0aed0cd9fb74ae7554ad08429de97e from pristine v0.32.2. Its SHA-256 is 45d96fd272cc8f1918f497c0e1df4ca0570fec0c89e14e4874c5d0c1bbced95f. Default wrapper builds still fetch the pristine upstream revision and apply the checked patch; they do not depend on fetching an unpublished core commit. The public C headers and the 685-function candidate snapshot are unchanged. This source relationship does not relabel earlier native qualification.
