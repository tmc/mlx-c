# Migrating the events-and-threads wrapper

This candidate pairs the upstream `ebc88f10` C API with downstream Event and
global-default extensions on patched MLX v0.32.2. It requires matching bindings.
It is not a binary-compatible replacement for the wrapper at `78f68783`.

## Streams and ownership

The upstream default-stream functions retain their thread-local scope. The five
`*_global` extensions preserve the downstream process-wide default policy.
MLX-Go maps its existing public default-stream API to these explicit extensions
and uses `mlx_stream_new_thread_unsafe` for portable streams. A Go goroutine alone
does not establish native thread affinity: thread-local graph creation, use and
lazy evaluation must stay on the owning OS thread.

The old plain-value TLS descriptor is replaced by opaque
`mlx_stream_thread_local` handles. Copying a C handle aliases its allocation;
copying descriptor content through the setter is a separate operation. Freeing
the wrapper does not promise reclamation of every native stream resolved from
it. MLX-Go uses a shared private owner and exposes no public descriptor `Close`.

`mlx_clear_streams` remains part of the upstream C surface but is deliberately
excluded from the Go bindings. It is not a replacement for descriptor cleanup;
GPU cleanup can affect global encoder state.

## Other binding changes

- Compile-cache construction is separate from invocation-thread capture through
  `mlx_detail_compile_cache`; it is not a rename of the old current-cache call.
- Seek callbacks return `int`; read, read-at and write callbacks return `size_t`.
  Old void-returning callbacks cannot be used with this wrapper.
- Ten operation signatures and the explicit axis/axes variants require matching
  generated argument layouts. See `codegen/snapshots` for the declaration diff.
- The Metal path getter returns an owned `mlx_string`; callers must copy and free
  it according to that API.
- Event usage is one-shot. On CUDA builds, signal must return before host wait
  or stream wait, including waits on CPU streams. Metal supports enqueueing a
  wait before signal with explicit caller ordering. These APIs do
  not imply concurrent Go evaluation.
- `mlx_node_namer_get_name` borrows the namer's stored string. Keep the namer alive
  and do not replace that name while using the pointer. The old wrapper's
  thread-local string copy had a different lifetime.

## Examples and build changes

The old `example-threads` target is removed. Its plain-value TLS calls no longer
exist, and its unconditional wait-before-signal example was unsuitable for
CUDA. `example-thread-stream` and `example-stream-tls`, together with the native
stream/Event tests, cover the replacement APIs; they do not preserve the old
example's filename or every behavior.

The old string-replacement export hook is replaced by a checked patch against
an exact pristine v0.32.2 revision. The patch verifies source state and hashes,
and includes the reviewed CUDA worker shutdown changes. Native qualification
is backend-specific; a successful source patch or build is not runtime proof.
The public snapshots do not describe private C++ ownership, callback ABI,
unchanged-signature semantics, or loaded library identity. Their separate native
checks remain necessary. Upstream-generated header prose also differs from the
old branch; unchanged declarations do not imply identical documentation.
