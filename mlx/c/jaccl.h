/* Copyright © 2026 Apple Inc. */
/* This file is auto-generated. Do not edit manually. */

#ifndef MLX_JACCL_H
#define MLX_JACCL_H

#include <stdbool.h>
#include <stddef.h>

#ifdef __cplusplus
extern "C" {
#endif

/**
 * \defgroup mlx_jaccl JACCL
 * Standalone C API for libjaccl.
 */
/**@{*/

/**
 * A JACCL communication group.
 *
 * A zero value is empty and must be initialized with mlx_jaccl_init or
 * mlx_jaccl_init_config before use.
 */
typedef struct mlx_jaccl_group_ {
  void* ctx;
} mlx_jaccl_group;

/**
 * A JACCL configuration object.
 *
 * Create with mlx_jaccl_config_new and release with mlx_jaccl_config_free.
 */
typedef struct mlx_jaccl_config_ {
  void* ctx;
} mlx_jaccl_config;

/**
 * Element type for JACCL reductions.
 */
typedef enum mlx_jaccl_dtype_ {
  MLX_JACCL_BOOL = 0,
  MLX_JACCL_INT8,
  MLX_JACCL_INT16,
  MLX_JACCL_INT32,
  MLX_JACCL_INT64,
  MLX_JACCL_UINT8,
  MLX_JACCL_UINT16,
  MLX_JACCL_UINT32,
  MLX_JACCL_UINT64,
  MLX_JACCL_FLOAT16,
  MLX_JACCL_BFLOAT16,
  MLX_JACCL_FLOAT32,
  MLX_JACCL_FLOAT64,
  MLX_JACCL_COMPLEX64,
} mlx_jaccl_dtype;

/**
 * Return the size of a JACCL element type in bytes.
 *
 * Returns 0 and sets mlx_jaccl_last_error for an invalid type.
 */
size_t mlx_jaccl_dtype_size(mlx_jaccl_dtype dtype);

/**
 * Return the last error for the calling thread.
 */
const char* mlx_jaccl_last_error(void);

/**
 * Clear the last error for the calling thread.
 */
void mlx_jaccl_clear_error(void);

/**
 * Create an empty group.
 */
mlx_jaccl_group mlx_jaccl_group_new(void);

/**
 * Free a group.
 */
int mlx_jaccl_group_free(mlx_jaccl_group group);

/**
 * Create a configuration.
 */
mlx_jaccl_config mlx_jaccl_config_new(void);

/**
 * Create a configuration and store it in res.
 */
int mlx_jaccl_config_new_out(mlx_jaccl_config* res);

/**
 * Create a configuration from JACCL environment variables.
 *
 * Returns an empty configuration and sets mlx_jaccl_last_error when the
 * environment does not describe a valid JACCL configuration.
 */
mlx_jaccl_config mlx_jaccl_config_from_env(void);

/**
 * Free a configuration.
 */
int mlx_jaccl_config_free(mlx_jaccl_config config);

/**
 * Set the rank for a configuration.
 */
int mlx_jaccl_config_set_rank(mlx_jaccl_config config, int rank);

/**
 * Get the configured rank.
 */
int mlx_jaccl_config_rank(mlx_jaccl_config config);

/**
 * Set the coordinator address for a configuration.
 */
int mlx_jaccl_config_set_coordinator(
    mlx_jaccl_config config,
    const char* coordinator);

/**
 * Get the configured coordinator address.
 *
 * The returned pointer is valid until the next
 * mlx_jaccl_config_coordinator call on the same thread.
 */
const char* mlx_jaccl_config_coordinator(mlx_jaccl_config config);

/**
 * Set devices from a JACCL device JSON file.
 */
int mlx_jaccl_config_set_devices_file(
    mlx_jaccl_config config,
    const char* path);

/**
 * Set devices from a JACCL device JSON string.
 */
int mlx_jaccl_config_set_devices_json(
    mlx_jaccl_config config,
    const char* json);

/**
 * Prefer ring topology for a configuration.
 */
int mlx_jaccl_config_prefer_ring(mlx_jaccl_config config, bool prefer);

/**
 * Check whether a configuration prefers ring topology.
 */
bool mlx_jaccl_config_prefers_ring(mlx_jaccl_config config);

/**
 * Get the configured group size.
 */
int mlx_jaccl_config_size(mlx_jaccl_config config);

/**
 * Check whether a configuration describes a valid mesh.
 */
bool mlx_jaccl_config_is_valid_mesh(mlx_jaccl_config config);

/**
 * Check whether a configuration describes a valid ring.
 */
bool mlx_jaccl_config_is_valid_ring(mlx_jaccl_config config);

/**
 * Check if JACCL is available on this system.
 */
bool mlx_jaccl_is_available(void);

/**
 * Initialize JACCL from environment variables.
 *
 * Reads JACCL_RANK or MLX_RANK, JACCL_IBV_DEVICES or MLX_IBV_DEVICES,
 * JACCL_COORDINATOR or MLX_JACCL_COORDINATOR, and JACCL_RING or
 * MLX_JACCL_RING.
 */
int mlx_jaccl_init(mlx_jaccl_group* res, bool strict);

/**
 * Initialize JACCL from a configuration.
 */
int mlx_jaccl_init_config(
    mlx_jaccl_group* res,
    mlx_jaccl_config config,
    bool strict);

/**
 * Get the rank.
 */
int mlx_jaccl_group_rank(mlx_jaccl_group group);

/**
 * Get the group size.
 */
int mlx_jaccl_group_size(mlx_jaccl_group group);

/**
 * Wait until all ranks enter the barrier.
 */
int mlx_jaccl_barrier(mlx_jaccl_group group);

/**
 * Sum-reduce n_bytes from input into output.
 *
 * n_bytes must be a multiple of the selected dtype size.
 */
int mlx_jaccl_all_sum(
    mlx_jaccl_group group,
    const void* input,
    void* output,
    size_t n_bytes,
    mlx_jaccl_dtype dtype);

/**
 * Max-reduce n_bytes from input into output.
 *
 * n_bytes must be a multiple of the selected dtype size.
 */
int mlx_jaccl_all_max(
    mlx_jaccl_group group,
    const void* input,
    void* output,
    size_t n_bytes,
    mlx_jaccl_dtype dtype);

/**
 * Min-reduce n_bytes from input into output.
 *
 * n_bytes must be a multiple of the selected dtype size.
 */
int mlx_jaccl_all_min(
    mlx_jaccl_group group,
    const void* input,
    void* output,
    size_t n_bytes,
    mlx_jaccl_dtype dtype);

/**
 * Gather n_bytes from input on every rank into output.
 *
 * The output buffer must hold group size times n_bytes.
 */
int mlx_jaccl_all_gather(
    mlx_jaccl_group group,
    const void* input,
    void* output,
    size_t n_bytes);

/**
 * Send input to another rank.
 */
int mlx_jaccl_send(
    mlx_jaccl_group group,
    const void* input,
    size_t n_bytes,
    int dst);

/**
 * Receive output from another rank.
 */
int mlx_jaccl_recv(
    mlx_jaccl_group group,
    void* output,
    size_t n_bytes,
    int src);

/**@}*/

#ifdef __cplusplus
}
#endif

#endif
