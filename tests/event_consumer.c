#include <stdbool.h>

#include "mlx/c/device.h"
#include "mlx/c/event.h"
#include "mlx/c/stream.h"

int main(void) {
  mlx_device device = mlx_device_new_type(MLX_CPU, 0);
  mlx_stream stream = mlx_stream_new_device(device);
  mlx_event event = {0};
  bool signaled = false;
  int status = mlx_event_new(&event, stream);
  status |= mlx_event_signal(event, stream);
  status |= mlx_event_wait(event);
  status |= mlx_event_is_signaled(&signaled, event);
  status |= mlx_event_free(event);
  status |= mlx_stream_free(stream);
  status |= mlx_device_free(device);
  return status != 0 || !signaled;
}
