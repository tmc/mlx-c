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
 * A MLX thread-local stream token.
 *
 * This is a plain value naming a logical stream. Resolving it on an OS thread
 * returns that thread's concrete stream for this token.
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
 * The stream is registered for the calling thread. Work evaluated on another
 * thread may require mlx_stream_new_thread_unsafe_device or a thread-local
 * stream token.
 */
mlx_stream mlx_stream_new_device(mlx_device dev);

/**
 * Returns a new stream on a device that can be used from any thread.
 */
mlx_stream mlx_stream_new_thread_unsafe_device(mlx_device dev);

/**
 * Returns a new thread-local stream token on a device.
 *
 * Resolve the token to a concrete stream for the current thread with
 * mlx_stream_from_thread_local_stream.
 */
mlx_thread_local_stream mlx_new_thread_local_stream(mlx_device dev);

/**
 * Write a new thread-local stream token for a device.
 *
 * This form avoids platform-specific aggregate calling conventions and is
 * preferred by foreign-function interfaces.
 */
int mlx_new_thread_local_stream_checked(
    mlx_thread_local_stream* stream,
    mlx_device dev);

/**
 * Returns a new thread-local stream token on a device.
 *
 * Deprecated: use mlx_new_thread_local_stream.
 */
mlx_thread_local_stream mlx_thread_local_stream_new_device(mlx_device dev);

/**
 * Resolve a thread-local stream token to the stream for the current thread.
 */
mlx_stream mlx_stream_from_thread_local_stream(mlx_thread_local_stream stream);

/**
 * Resolve a thread-local stream token passed by pointer.
 *
 * This form avoids platform-specific aggregate calling conventions and is
 * preferred by foreign-function interfaces.
 */
int mlx_stream_from_thread_local_stream_checked(
    mlx_stream* stream,
    const mlx_thread_local_stream* thread_local_stream);

/**
 * Resolve a thread-local stream token to the stream for the current thread.
 *
 * Deprecated: use mlx_stream_from_thread_local_stream.
 */
int mlx_stream_from_thread_local(
    mlx_stream* stream,
    mlx_thread_local_stream thread_local_stream);

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
 */
int mlx_synchronize_default(void);

/**
 * Synchronize with the stream corresponding to the current thread.
 */
int mlx_thread_local_stream_synchronize(mlx_thread_local_stream stream);

/**
 * Synchronize with a thread-local stream token passed by pointer.
 *
 * This form avoids platform-specific aggregate calling conventions and is
 * preferred by foreign-function interfaces.
 */
int mlx_thread_local_stream_synchronize_checked(
    const mlx_thread_local_stream* stream);

/**
 * Synchronize with the stream corresponding to the current thread.
 *
 * Deprecated: use mlx_thread_local_stream_synchronize.
 */
int mlx_synchronize_thread_local(mlx_thread_local_stream stream);

/**
 * Returns the default stream on the given device.
 */
int mlx_get_default_stream(mlx_stream* stream, mlx_device dev);
/**
 * Set default stream.
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
