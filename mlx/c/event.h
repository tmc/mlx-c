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
 * The event is one-shot: it is created unsignaled, and a single
 * mlx_event_signal marks it signaled for good. It does not reset.
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
 * Freeing an empty event is allowed. Copies of an event share ownership of the
 * same handle and must not be freed more than once.
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
