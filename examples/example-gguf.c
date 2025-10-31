/* Copyright © 2023-2024 Apple Inc. */

#include <stdio.h>
#include <unistd.h>
#include "mlx/c/mlx.h"

void print_array(const char* msg, mlx_array arr) {
  mlx_string str = mlx_string_new();
  mlx_array_tostring(&str, arr);
  printf("%s: %s\n", msg, mlx_string_data(str));
  mlx_string_free(str);
}

int main(void) {
  mlx_stream stream = mlx_default_cpu_stream_new();

  printf("=== GGUF Save Example ===\n\n");

  // Create some test arrays
  mlx_map_string_to_array arrays = mlx_map_string_to_array_new();

  float data1[] = {1.0f, 2.0f, 3.0f, 4.0f};
  int shape1[] = {2, 2};
  mlx_array arr1 = mlx_array_new_data(data1, shape1, 2, MLX_FLOAT32);
  mlx_map_string_to_array_insert(arrays, "weights", arr1);

  float data2[] = {0.5f, 0.5f};
  int shape2[] = {2};
  mlx_array arr2 = mlx_array_new_data(data2, shape2, 1, MLX_FLOAT32);
  mlx_map_string_to_array_insert(arrays, "bias", arr2);

  // Create metadata (opaque for now - just use empty)
  mlx_gguf_metadata_map metadata = mlx_gguf_metadata_map_new();

  // Save GGUF file to temp directory
  const char* temp_file = "/tmp/test_model.gguf";
  printf("Saving GGUF file to %s...\n", temp_file);
  int err = mlx_save_gguf(temp_file, arrays, metadata);
  if (err != 0) {
    printf("Error saving GGUF file\n");
    return 1;
  }
  printf("Saved successfully!\n\n");

  // Clean up
  mlx_array_free(arr1);
  mlx_array_free(arr2);
  mlx_map_string_to_array_free(arrays);
  mlx_gguf_metadata_map_free(metadata);

  printf("=== GGUF Load Example (with metadata) ===\n\n");

  // Load sample.gguf which has metadata
  mlx_map_string_to_array loaded_arrays = mlx_map_string_to_array_new();
  mlx_gguf_metadata_map loaded_metadata = mlx_gguf_metadata_map_new();

  printf("Loading sample.gguf...\n");
  err = mlx_load_gguf(&loaded_arrays, &loaded_metadata, "sample.gguf", stream);
  if (err != 0) {
    printf("Error loading GGUF file\n");
    return 1;
  }
  printf("Loaded successfully!\n\n");

  // Print loaded arrays
  printf("Loaded arrays:\n");
  mlx_map_string_to_array_iterator arr_it =
      mlx_map_string_to_array_iterator_new(loaded_arrays);
  const char* arr_key;
  mlx_array arr_value = mlx_array_new();
  int array_count = 0;
  while (!mlx_map_string_to_array_iterator_next(&arr_key, &arr_value, arr_it)) {
    print_array(arr_key, arr_value);
    array_count++;
  }

  printf("\nLoaded %d arrays\n", array_count);

  // Print metadata
  printf("\nMetadata (%zu entries):\n", mlx_gguf_metadata_map_size(loaded_metadata));
  mlx_gguf_metadata_map_iterator meta_it =
      mlx_gguf_metadata_map_iterator_new(loaded_metadata);
  const char* meta_key;
  mlx_gguf_value_type meta_type;

  while (!mlx_gguf_metadata_map_iterator_next(&meta_key, &meta_type, meta_it)) {
    printf("  %s: ", meta_key);

    switch (meta_type) {
      case MLX_GGUF_VALUE_TYPE_NONE:
        printf("(empty)\n");
        break;

      case MLX_GGUF_VALUE_TYPE_STRING: {
        const char* str_value;
        if (!mlx_gguf_metadata_get_string(&str_value, meta_it)) {
          printf("\"%s\"\n", str_value);
        }
        break;
      }

      case MLX_GGUF_VALUE_TYPE_ARRAY: {
        mlx_array arr_value = mlx_array_new();
        if (!mlx_gguf_metadata_get_array(&arr_value, meta_it)) {
          print_array("array", arr_value);
        }
        mlx_array_free(arr_value);
        break;
      }

      case MLX_GGUF_VALUE_TYPE_STRING_ARRAY: {
        size_t arr_size;
        if (!mlx_gguf_metadata_get_string_array_size(&arr_size, meta_it)) {
          printf("[");
          for (size_t i = 0; i < arr_size; i++) {
            const char* elem;
            if (!mlx_gguf_metadata_get_string_array_at(&elem, i, meta_it)) {
              printf("\"%s\"", elem);
              if (i < arr_size - 1) printf(", ");
            }
          }
          printf("]\n");
        }
        break;
      }
    }
  }

  mlx_gguf_metadata_map_iterator_free(meta_it);

  // Clean up
  mlx_array_free(arr_value);
  mlx_map_string_to_array_iterator_free(arr_it);
  mlx_map_string_to_array_free(loaded_arrays);
  mlx_gguf_metadata_map_free(loaded_metadata);
  mlx_stream_free(stream);

  // Remove temporary file
  unlink(temp_file);

  printf("\nExample completed successfully!\n");
  return 0;
}
