/* Copyright © 2023-2024 Apple Inc. */

#ifndef MLX_IO_GGUF_H
#define MLX_IO_GGUF_H

#include "mlx/c/array.h"
#include "mlx/c/map.h"
#include "mlx/c/stream.h"

#ifdef __cplusplus
extern "C" {
#endif

/**
 * \defgroup mlx_io_gguf GGUF I/O
 * GGUF file format loading and saving
 */
/**@{*/

/**
 * GGUF metadata value types
 */
typedef enum mlx_gguf_value_type_ {
  MLX_GGUF_VALUE_TYPE_NONE = 0,   // monostate (empty)
  MLX_GGUF_VALUE_TYPE_ARRAY = 1,  // mlx_array
  MLX_GGUF_VALUE_TYPE_STRING = 2, // string
  MLX_GGUF_VALUE_TYPE_STRING_ARRAY = 3 // vector<string>
} mlx_gguf_value_type;

/**
 * Opaque GGUF metadata map
 * Wraps std::unordered_map<std::string, GGUFMetaData> where
 * GGUFMetaData = variant<monostate, array, string, vector<string>>
 */
typedef struct mlx_gguf_metadata_map_ {
  void* ctx;
} mlx_gguf_metadata_map;

/**
 * Opaque GGUF metadata map iterator
 */
typedef struct mlx_gguf_metadata_map_iterator_ {
  void* ctx;
} mlx_gguf_metadata_map_iterator;

/**
 * Load GGUF file
 *
 * @param res_arrays Output map of tensor name -> array
 * @param res_metadata Output metadata map (opaque)
 * @param file Path to GGUF file
 * @param s Stream or device
 * @return 0 on success, non-zero on error
 */
int mlx_load_gguf(
    mlx_map_string_to_array* res_arrays,
    mlx_gguf_metadata_map* res_metadata,
    const char* file,
    const mlx_stream s);

/**
 * Save GGUF file
 *
 * @param file Path to output GGUF file
 * @param arrays Map of tensor name -> array
 * @param metadata Metadata map (opaque, can be empty)
 * @return 0 on success, non-zero on error
 */
int mlx_save_gguf(
    const char* file,
    const mlx_map_string_to_array arrays,
    const mlx_gguf_metadata_map metadata);

/**
 * Create new empty metadata map
 */
mlx_gguf_metadata_map mlx_gguf_metadata_map_new(void);

/**
 * Free metadata map
 */
int mlx_gguf_metadata_map_free(mlx_gguf_metadata_map map);

/**
 * Get number of entries in metadata map
 */
size_t mlx_gguf_metadata_map_size(const mlx_gguf_metadata_map map);

/**
 * Create iterator for metadata map
 */
mlx_gguf_metadata_map_iterator mlx_gguf_metadata_map_iterator_new(
    const mlx_gguf_metadata_map map);

/**
 * Free metadata map iterator
 */
int mlx_gguf_metadata_map_iterator_free(mlx_gguf_metadata_map_iterator it);

/**
 * Advance iterator and get next key-value pair
 *
 * @param res_key Output pointer to key string (valid until map is freed)
 * @param res_type Output pointer to value type
 * @param it Iterator
 * @return 0 if item retrieved, non-zero if no more items
 */
int mlx_gguf_metadata_map_iterator_next(
    const char** res_key,
    mlx_gguf_value_type* res_type,
    mlx_gguf_metadata_map_iterator it);

/**
 * Get string value for current iterator position
 * Only valid if type is MLX_GGUF_VALUE_TYPE_STRING
 *
 * @param res_value Output pointer to string (valid until map is freed)
 * @param it Iterator
 * @return 0 on success, non-zero on error
 */
int mlx_gguf_metadata_get_string(
    const char** res_value,
    mlx_gguf_metadata_map_iterator it);

/**
 * Get array value for current iterator position
 * Only valid if type is MLX_GGUF_VALUE_TYPE_ARRAY
 *
 * @param res_value Output array (caller must free)
 * @param it Iterator
 * @return 0 on success, non-zero on error
 */
int mlx_gguf_metadata_get_array(
    mlx_array* res_value,
    mlx_gguf_metadata_map_iterator it);

/**
 * Get string array size for current iterator position
 * Only valid if type is MLX_GGUF_VALUE_TYPE_STRING_ARRAY
 *
 * @param res_size Output size
 * @param it Iterator
 * @return 0 on success, non-zero on error
 */
int mlx_gguf_metadata_get_string_array_size(
    size_t* res_size,
    mlx_gguf_metadata_map_iterator it);

/**
 * Get string array element for current iterator position
 * Only valid if type is MLX_GGUF_VALUE_TYPE_STRING_ARRAY
 *
 * @param res_value Output pointer to string (valid until map is freed)
 * @param index Element index
 * @param it Iterator
 * @return 0 on success, non-zero on error
 */
int mlx_gguf_metadata_get_string_array_at(
    const char** res_value,
    size_t index,
    mlx_gguf_metadata_map_iterator it);

/**@}*/

#ifdef __cplusplus
}
#endif

#endif // MLX_IO_GGUF_H
