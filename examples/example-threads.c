/* Copyright © 2023-2024 Apple Inc. */

/* Streams and events across threads.
 *
 * Each section demonstrates one guarantee the headers make, and prints enough
 * state to show it held. Run it; the expectations are in the output.
 */

#include <pthread.h>
#include <stdio.h>
#include "mlx/c/mlx.h"

#define NUM_WORKERS 4

static int failures = 0;

static void check(int cond, const char* what) {
  printf("  %-58s %s\n", what, cond ? "ok" : "FAILED");
  if (!cond) {
    failures++;
  }
}

/* ---------------------------------------------------------------- streams */

typedef struct {
  mlx_stream stream;
  float in;
  float out;
  int status;
} work;

/* A shared stream may be used from any thread, but only by one thread at a
 * time: "thread unsafe" in mlx_stream_new_thread_unsafe_device names the
 * registration, and serializing work on the stream is the caller's job. Drop
 * this mutex and the Metal backend aborts with "A command encoder is already
 * encoding to this command buffer" -- not an error return, an abort. */
static pthread_mutex_t submit = PTHREAD_MUTEX_INITIALIZER;

/* Doubles in on stream, from whatever thread runs this. */
static void* double_on_stream(void* arg) {
  work* w = (work*)arg;
  mlx_array a = mlx_array_new_float32(w->in);
  mlx_array sum = mlx_array_new();

  pthread_mutex_lock(&submit);
  w->status = mlx_add(&sum, a, a, w->stream);
  if (w->status == 0) {
    w->status = mlx_array_item_float32(&w->out, sum);
  }
  pthread_mutex_unlock(&submit);

  mlx_array_free(sum);
  mlx_array_free(a);
  return NULL;
}

/* One stream, created here, used by every worker. mlx_stream_new_device
 * returns a process-global stream precisely so this is allowed. */
static void shared_stream(mlx_device dev) {
  printf("A shared stream used from %d worker threads\n", NUM_WORKERS);
  printf("  (serialized by the caller -- see the note above submit)\n");

  mlx_stream s = mlx_stream_new_device(dev);
  pthread_t threads[NUM_WORKERS];
  work items[NUM_WORKERS];

  for (int i = 0; i < NUM_WORKERS; i++) {
    items[i].stream = s;
    items[i].in = (float)(i + 1);
    items[i].out = 0.0f;
    items[i].status = -1;
    pthread_create(&threads[i], NULL, double_on_stream, &items[i]);
  }
  for (int i = 0; i < NUM_WORKERS; i++) {
    pthread_join(threads[i], NULL);
  }

  for (int i = 0; i < NUM_WORKERS; i++) {
    char msg[80];
    snprintf(
        msg,
        sizeof(msg),
        "worker %d: %.0f + %.0f = %.0f",
        i,
        items[i].in,
        items[i].in,
        items[i].out);
    check(items[i].status == 0 && items[i].out == items[i].in * 2.0f, msg);
  }

  mlx_stream_free(s);
}

/* ----------------------------------------------------------- descriptors */

typedef struct {
  mlx_thread_local_stream descriptor;
  int resolved_index;
} resolution;

/* Resolving one descriptor on N threads yields N distinct streams: the
 * descriptor names a stream per thread rather than a single shared one. */
static void* resolve_descriptor(void* arg) {
  resolution* r = (resolution*)arg;
  mlx_stream s = mlx_stream_new();

  if (mlx_thread_local_stream_resolve(&s, &r->descriptor) == 0) {
    mlx_stream_get_index(&r->resolved_index, s);
    mlx_thread_local_stream_synchronize(&r->descriptor);
  }

  mlx_stream_free(s);
  return NULL;
}

static void thread_local_streams(mlx_device dev) {
  printf("One thread-local descriptor resolved on %d threads\n", NUM_WORKERS);

  mlx_thread_local_stream descriptor = mlx_thread_local_stream_new(dev);
  check(
      mlx_thread_local_stream_is_valid(descriptor),
      "descriptor is valid after creation");

  pthread_t threads[NUM_WORKERS];
  resolution results[NUM_WORKERS];

  for (int i = 0; i < NUM_WORKERS; i++) {
    results[i].descriptor = descriptor;
    results[i].resolved_index = -1;
    pthread_create(&threads[i], NULL, resolve_descriptor, &results[i]);
  }
  for (int i = 0; i < NUM_WORKERS; i++) {
    pthread_join(threads[i], NULL);
  }

  printf("  resolved stream indices:");
  for (int i = 0; i < NUM_WORKERS; i++) {
    printf(" %d", results[i].resolved_index);
  }
  printf("\n");

  int distinct = 1;
  for (int i = 0; i < NUM_WORKERS; i++) {
    if (results[i].resolved_index < 0) {
      distinct = 0;
    }
    for (int j = i + 1; j < NUM_WORKERS; j++) {
      if (results[i].resolved_index == results[j].resolved_index) {
        distinct = 0;
      }
    }
  }
  check(distinct, "each thread resolved a stream of its own");
  printf("  (so work submitted through one descriptor is NOT ordered\n");
  printf("   between threads -- use one stream for that)\n");
}

/* ---------------------------------------------------------------- events */

typedef struct {
  mlx_event ready;
  mlx_stream stream;
  mlx_array result;
  float seen;
  int status;
} handoff;

/* Produces a value on its stream, then signals. */
static void* producer(void* arg) {
  handoff* h = (handoff*)arg;
  mlx_array a = mlx_array_new_float32(21.0f);

  h->status = mlx_add(&h->result, a, a, h->stream);
  if (h->status == 0) {
    h->status = mlx_array_eval(h->result);
  }
  mlx_event_signal(h->ready, h->stream);

  mlx_array_free(a);
  return NULL;
}

/* Waits for the signal, then reads what the producer computed. */
static void* consumer(void* arg) {
  handoff* h = (handoff*)arg;
  mlx_event_wait(h->ready);
  mlx_array_item_float32(&h->seen, h->result);
  return NULL;
}

static void event_handoff(mlx_device dev) {
  printf("Event handoff between two threads\n");

  handoff h;
  h.stream = mlx_stream_new_device(dev);
  h.ready = mlx_event_new(h.stream);
  h.result = mlx_array_new();
  h.seen = 0.0f;
  h.status = -1;

  bool signaled = true;
  mlx_event_is_signaled(&signaled, h.ready);
  check(!signaled, "a new event starts unsignaled");

  pthread_t p, c;
  pthread_create(&p, NULL, producer, &h);
  pthread_create(&c, NULL, consumer, &h);
  pthread_join(p, NULL);
  pthread_join(c, NULL);

  char msg[80];
  snprintf(msg, sizeof(msg), "consumer read %.0f after the signal", h.seen);
  check(h.status == 0 && h.seen == 42.0f, msg);

  mlx_event_is_signaled(&signaled, h.ready);
  check(signaled, "the event stays signaled -- it is one-shot");

  mlx_array_free(h.result);
  mlx_event_free(h.ready);
  mlx_stream_free(h.stream);
}

/* ----------------------------------------------------------------- main */

int main(void) {
  mlx_device dev = mlx_device_new();
  mlx_get_default_device(&dev);

  shared_stream(dev);
  printf("\n");
  thread_local_streams(dev);
  printf("\n");
  event_handoff(dev);

  mlx_synchronize_default();
  mlx_device_free(dev);

  printf("\n%s\n", failures == 0 ? "all checks passed" : "CHECKS FAILED");
  return failures == 0 ? 0 : 1;
}
