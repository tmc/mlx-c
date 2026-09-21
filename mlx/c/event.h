/* Copyright © 2023-2024 Apple Inc. */

#ifndef MLX_EVENT_H
#define MLX_EVENT_H

#include <stdbool.h>
#include "mlx/c/stream.h"

#ifdef __cplusplus
extern "C" {
#endif

/**
 * \defgroup event Event synchronization
 */
/**@{*/

/**
 * A synchronization event.
 */
typedef struct mlx_event_ {
  void* ctx;
} mlx_event;

/**
 * Returns a new event associated with stream.
 *
 * The event is created unsignaled. Call mlx_event_signal exactly once;
 * repeated signaling is not supported.
 * Serialize submissions to each stream. On CUDA, signal must return before
 * wait or wait_stream is called, even when using a CPU stream. CPU-only and
 * Metal builds permit waiting before signaling from another stream.
 *
 * On failure, returns an empty event and sets the error handler. An empty
 * event has a null ctx; every operation below except mlx_event_free fails on
 * one, so a caller that ignores the check still gets an error rather than
 * silent misbehaviour.
 */
mlx_event mlx_event_new(mlx_stream stream);

/**
 * Free an event.
 *
 * Freeing an empty event is allowed. Handle copies alias one allocation;
 * free it exactly once after all host calls using it have returned.
 * This does not cancel an outstanding wait. Keep the producer alive until
 * every queued wait can complete.
 */
int mlx_event_free(mlx_event event);

/**
 * Return true when the event has been signaled.
 */
int mlx_event_is_signaled(bool* result, mlx_event event);

/**
 * Signal the event in stream.
 */
int mlx_event_signal(mlx_event event, mlx_stream stream);

/**
 * Wait for the event to be signaled.
 */
int mlx_event_wait(mlx_event event);

/**
 * Wait in stream for the event to be signaled.
 */
int mlx_event_wait_stream(mlx_event event, mlx_stream stream);

/**@}*/

#ifdef __cplusplus
}
#endif

#endif
