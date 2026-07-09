/* Copyright © 2023-2024 Apple Inc. */

#ifndef MLX_EVENT_H
#define MLX_EVENT_H

#include <stdbool.h>
#include <stdint.h>

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
 * Return a new invalid event.
 */
mlx_event mlx_event_new(void);

/**
 * Return a new event associated with stream.
 */
mlx_event mlx_event_new_stream(mlx_stream stream);

/**
 * Free an event.
 */
int mlx_event_free(mlx_event event);

/**
 * Return true when the event has been signaled at its current value.
 */
int mlx_event_is_signaled(bool* res, mlx_event event);

/**
 * Return the event stream.
 */
int mlx_event_stream(mlx_stream* res, mlx_event event);

/**
 * Return the event value.
 */
int mlx_event_value(uint64_t* res, mlx_event event);

/**
 * Set the event value.
 */
int mlx_event_set_value(mlx_event event, uint64_t value);

/**
 * Return true when the event is valid.
 */
int mlx_event_valid(bool* res, mlx_event event);

/**
 * Signal the event in stream.
 */
int mlx_event_signal(mlx_event event, mlx_stream stream);

/**
 * Wait for the event to be signaled at its current value.
 */
int mlx_event_wait(mlx_event event);

/**
 * Wait in stream for the event to be signaled at its current value.
 */
int mlx_event_wait_stream(mlx_event event, mlx_stream stream);

/**@}*/

#ifdef __cplusplus
}
#endif

#endif
