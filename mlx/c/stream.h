/* Copyright © 2023-2024 Apple Inc. */

#ifndef MLX_STREAM_H
#define MLX_STREAM_H

#include <stdbool.h>

#include "mlx/c/device.h"

#ifdef __cplusplus
extern "C" {
#endif

/**
 * \defgroup mlx_stream Stream
 * MLX stream object.
 */
/**@{*/

/**
 * A MLX stream object.
 */
typedef struct mlx_stream_ {
  void* ctx;
} mlx_stream;

/**
 * A MLX thread-local stream descriptor.
 *
 * This is a plain value naming a logical stream. Resolving it on an OS thread
 * returns that thread's concrete stream for this descriptor.
 *
 * The ABI stores the device fields directly instead of embedding mlx_device,
 * because mlx_device is an owning opaque handle while MLX ThreadLocalStream is
 * a small copyable value.
 */
typedef struct mlx_thread_local_stream_ {
  int index;
  mlx_device_type device_type;
  int device_index;
} mlx_thread_local_stream;

/**
 * Returns a new empty stream.
 */
mlx_stream mlx_stream_new(void);

/**
 * Returns a new stream on a device.
 *
 * The stream is usable from any thread: this delegates to
 * mlx_stream_new_thread_unsafe_device, because stream handles crossing the C
 * API are not tied to the thread that created them. This diverges from
 * mlx::core::new_stream, which registers only for the calling thread.
 */
mlx_stream mlx_stream_new_device(mlx_device dev);

/**
 * Returns a new stream on a device that can be used from any thread.
 *
 * "Thread unsafe" is upstream's name for the registration, not a warning
 * about the handle: the stream is registered process-globally rather than per
 * thread, and the caller is responsible for serializing work submitted to it.
 */
mlx_stream mlx_stream_new_thread_unsafe_device(mlx_device dev);

/**
 * Returns a new thread-local stream descriptor for a device.
 *
 * The descriptor names a stream rather than being one, and is meant to be
 * carried across threads: mlx_thread_local_stream_resolve maps it to a stream
 * belonging to whichever thread resolves it, creating that thread's stream on
 * first use. Threads therefore do not share ordering through one descriptor.
 *
 * Prefer mlx_stream_new_device for ordinary parallel work: a fixed set of
 * streams, one thread submitting to each, reaches the same concurrency without
 * pinning threads or accumulating streams. Reach for a descriptor when a
 * per-thread stream is what you actually want.
 *
 * Per descriptor, one stream is allocated per resolving thread. Across D
 * descriptors the population is bounded by D times the number of threads, and
 * in practice lands under it, since threads are reused and not every thread
 * resolves every descriptor. Nothing reclaims those streams: mlx_stream_free
 * releases a handle, not the registration behind it.
 *
 * On failure, returns an invalid descriptor and sets the error handler.
 * Use mlx_thread_local_stream_is_valid to test the result.
 */
mlx_thread_local_stream mlx_thread_local_stream_new(mlx_device dev);

/**
 * Returns true when the descriptor names a stream.
 *
 * A descriptor returned by a failed mlx_thread_local_stream_new is invalid;
 * resolving or synchronizing one is an error.
 */
bool mlx_thread_local_stream_is_valid(mlx_thread_local_stream stream);

/**
 * Resolve a thread-local stream descriptor for the current thread.
 *
 * Unlike the descriptor, the resolved stream is confined to the calling
 * thread: using it from another one fails with "There is no Stream(...) in
 * current thread". Resolve again on each thread, or use
 * mlx_stream_new_thread_unsafe_device for a stream that may cross threads.
 *
 * As with mlx_stream_set, stream must already hold a stream handle, e.g. one
 * from mlx_stream_new; its previous value is overwritten.
 */
int mlx_thread_local_stream_resolve(
    mlx_stream* stream,
    const mlx_thread_local_stream* thread_local_stream);

/**
 * Set stream to provided src stream.
 */
int mlx_stream_set(mlx_stream* stream, const mlx_stream src);
/**
 * Free a stream.
 */
int mlx_stream_free(mlx_stream stream);
/**
 * Get stream description.
 */
int mlx_stream_tostring(mlx_string* str, mlx_stream stream);
/**
 * Check if streams are the same.
 */
bool mlx_stream_equal(mlx_stream lhs, mlx_stream rhs);
/**
 * Return the device of the stream.
 */
int mlx_stream_get_device(mlx_device* dev, mlx_stream stream);
/**
 * Return the index of the stream.
 */
int mlx_stream_get_index(int* index, mlx_stream stream);
/**
 * Synchronize with the provided stream.
 */
int mlx_synchronize(mlx_stream stream);

/**
 * Synchronize with the default stream.
 *
 * This is the same default mlx_get_default_stream reports for the default
 * device, including any mlx_set_default_stream override, rather than the
 * per-thread default mlx::core::synchronize() would resolve.
 */
int mlx_synchronize_default(void);

/**
 * Synchronize with the stream corresponding to the current thread.
 */
int mlx_thread_local_stream_synchronize(const mlx_thread_local_stream* stream);

/**
 * Returns the default stream on the given device.
 *
 * The C API's default streams are process-global and usable from any thread,
 * where mlx::core::default_stream is per thread. As a side effect this also
 * points the calling thread's core-internal default at the returned stream,
 * so ops that consult default_stream() for intermediate work agree with it.
 */
int mlx_get_default_stream(mlx_stream* stream, mlx_device dev);
/**
 * Set default stream.
 *
 * The override is process-global: unlike mlx::core::set_default_stream, this
 * affects every thread, not only the caller. The calling thread's
 * core-internal default is pointed at stream as well.
 */
int mlx_set_default_stream(mlx_stream stream);
/**
 * Returns the current default CPU stream.
 */
mlx_stream mlx_default_cpu_stream_new(void);

/**
 * Returns the current default GPU stream.
 */
mlx_stream mlx_default_gpu_stream_new(void);

/**@}*/

#ifdef __cplusplus
}
#endif

#endif
