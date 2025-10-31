/* Copyright © 2023-2024 Apple Inc. */

#include "mlx/c/io_gguf.h"
#include "mlx/c/error.h"
#include "mlx/c/private/array.h"
#include "mlx/c/private/map.h"
#include "mlx/c/private/stream.h"
#include "mlx/io.h"

// Wrapper for GGUFMetaData map
struct mlx_gguf_metadata_map_cpp_ {
  std::unordered_map<std::string, mlx::core::GGUFMetaData> map;
};

// Wrapper for metadata map iterator
struct mlx_gguf_metadata_map_iterator_cpp_ {
  std::unordered_map<std::string, mlx::core::GGUFMetaData>::const_iterator current;
  const std::unordered_map<std::string, mlx::core::GGUFMetaData>* map;
  // Cache last accessed entry since unordered_map iterators aren't bidirectional
  std::string last_key;
  const mlx::core::GGUFMetaData* last_value;
};

extern "C" mlx_gguf_metadata_map mlx_gguf_metadata_map_new(void) {
  try {
    return mlx_gguf_metadata_map{
        new mlx_gguf_metadata_map_cpp_{
            std::unordered_map<std::string, mlx::core::GGUFMetaData>()}};
  } catch (std::exception& e) {
    mlx_error(e.what());
    return mlx_gguf_metadata_map{nullptr};
  }
}

extern "C" int mlx_gguf_metadata_map_free(mlx_gguf_metadata_map map) {
  try {
    if (map.ctx) {
      delete static_cast<mlx_gguf_metadata_map_cpp_*>(map.ctx);
    }
    return 0;
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
}

extern "C" int mlx_load_gguf(
    mlx_map_string_to_array* res_arrays,
    mlx_gguf_metadata_map* res_metadata,
    const char* file,
    const mlx_stream s) {
  try {
    auto [arrays, metadata] =
        mlx::core::load_gguf(std::string(file), mlx_stream_get_(s));

    mlx_map_string_to_array_set_(*res_arrays, std::move(arrays));

    if (res_metadata->ctx) {
      static_cast<mlx_gguf_metadata_map_cpp_*>(res_metadata->ctx)->map =
          std::move(metadata);
    } else {
      res_metadata->ctx =
          new mlx_gguf_metadata_map_cpp_{std::move(metadata)};
    }

    return 0;
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
}

extern "C" int mlx_save_gguf(
    const char* file,
    const mlx_map_string_to_array arrays,
    const mlx_gguf_metadata_map metadata) {
  try {
    std::unordered_map<std::string, mlx::core::GGUFMetaData> meta_map;
    if (metadata.ctx) {
      meta_map = static_cast<mlx_gguf_metadata_map_cpp_*>(metadata.ctx)->map;
    }

    mlx::core::save_gguf(
        std::string(file), mlx_map_string_to_array_get_(arrays), meta_map);

    return 0;
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
}

extern "C" size_t mlx_gguf_metadata_map_size(
    const mlx_gguf_metadata_map map) {
  if (!map.ctx) {
    return 0;
  }
  return static_cast<mlx_gguf_metadata_map_cpp_*>(map.ctx)->map.size();
}

extern "C" mlx_gguf_metadata_map_iterator
mlx_gguf_metadata_map_iterator_new(const mlx_gguf_metadata_map map) {
  try {
    if (!map.ctx) {
      return mlx_gguf_metadata_map_iterator{nullptr};
    }

    auto* map_cpp = static_cast<mlx_gguf_metadata_map_cpp_*>(map.ctx);
    auto* it_cpp = new mlx_gguf_metadata_map_iterator_cpp_{
        map_cpp->map.begin(), &map_cpp->map, "", nullptr};

    return mlx_gguf_metadata_map_iterator{it_cpp};
  } catch (std::exception& e) {
    mlx_error(e.what());
    return mlx_gguf_metadata_map_iterator{nullptr};
  }
}

extern "C" int mlx_gguf_metadata_map_iterator_free(
    mlx_gguf_metadata_map_iterator it) {
  try {
    if (it.ctx) {
      delete static_cast<mlx_gguf_metadata_map_iterator_cpp_*>(it.ctx);
    }
    return 0;
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
}

extern "C" int mlx_gguf_metadata_map_iterator_next(
    const char** res_key,
    mlx_gguf_value_type* res_type,
    mlx_gguf_metadata_map_iterator it) {
  try {
    if (!it.ctx) {
      return 1;
    }

    auto* it_cpp = static_cast<mlx_gguf_metadata_map_iterator_cpp_*>(it.ctx);

    if (it_cpp->current == it_cpp->map->end()) {
      return 2; // No more items (match existing pattern)
    }

    // Cache key and value pointer for later retrieval
    it_cpp->last_key = it_cpp->current->first;
    it_cpp->last_value = &it_cpp->current->second;

    // Return key from cached string
    *res_key = it_cpp->last_key.c_str();

    // Determine the variant type
    const auto& value = *it_cpp->last_value;
    if (std::holds_alternative<std::monostate>(value)) {
      *res_type = MLX_GGUF_VALUE_TYPE_NONE;
    } else if (std::holds_alternative<mlx::core::array>(value)) {
      *res_type = MLX_GGUF_VALUE_TYPE_ARRAY;
    } else if (std::holds_alternative<std::string>(value)) {
      *res_type = MLX_GGUF_VALUE_TYPE_STRING;
    } else if (std::holds_alternative<std::vector<std::string>>(value)) {
      *res_type = MLX_GGUF_VALUE_TYPE_STRING_ARRAY;
    }

    ++it_cpp->current;
    return 0;
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
}

extern "C" int mlx_gguf_metadata_get_string(
    const char** res_value,
    mlx_gguf_metadata_map_iterator it) {
  try {
    if (!it.ctx) {
      return 1;
    }

    auto* it_cpp = static_cast<mlx_gguf_metadata_map_iterator_cpp_*>(it.ctx);

    if (!it_cpp->last_value) {
      return 1; // Iterator not yet advanced
    }

    if (!std::holds_alternative<std::string>(*it_cpp->last_value)) {
      return 1; // Wrong type
    }

    *res_value = std::get<std::string>(*it_cpp->last_value).c_str();
    return 0;
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
}

extern "C" int mlx_gguf_metadata_get_array(
    mlx_array* res_value,
    mlx_gguf_metadata_map_iterator it) {
  try {
    if (!it.ctx) {
      return 1;
    }

    auto* it_cpp = static_cast<mlx_gguf_metadata_map_iterator_cpp_*>(it.ctx);

    if (!it_cpp->last_value) {
      return 1; // Iterator not yet advanced
    }

    if (!std::holds_alternative<mlx::core::array>(*it_cpp->last_value)) {
      return 1; // Wrong type
    }

    mlx_array_set_(*res_value, std::get<mlx::core::array>(*it_cpp->last_value));
    return 0;
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
}

extern "C" int mlx_gguf_metadata_get_string_array_size(
    size_t* res_size,
    mlx_gguf_metadata_map_iterator it) {
  try {
    if (!it.ctx) {
      return 1;
    }

    auto* it_cpp = static_cast<mlx_gguf_metadata_map_iterator_cpp_*>(it.ctx);

    if (!it_cpp->last_value) {
      return 1; // Iterator not yet advanced
    }

    if (!std::holds_alternative<std::vector<std::string>>(*it_cpp->last_value)) {
      return 1; // Wrong type
    }

    *res_size = std::get<std::vector<std::string>>(*it_cpp->last_value).size();
    return 0;
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
}

extern "C" int mlx_gguf_metadata_get_string_array_at(
    const char** res_value,
    size_t index,
    mlx_gguf_metadata_map_iterator it) {
  try {
    if (!it.ctx) {
      return 1;
    }

    auto* it_cpp = static_cast<mlx_gguf_metadata_map_iterator_cpp_*>(it.ctx);

    if (!it_cpp->last_value) {
      return 1; // Iterator not yet advanced
    }

    if (!std::holds_alternative<std::vector<std::string>>(*it_cpp->last_value)) {
      return 1; // Wrong type
    }

    const auto& vec = std::get<std::vector<std::string>>(*it_cpp->last_value);
    if (index >= vec.size()) {
      return 1; // Out of bounds
    }

    *res_value = vec[index].c_str();
    return 0;
  } catch (std::exception& e) {
    mlx_error(e.what());
    return 1;
  }
}
