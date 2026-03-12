//go:build ghostai

/*
 * Ghost-AI C Bridge
 *
 * Thin wrapper around llama.cpp's C API (llama.h).
 * Only exposes the subset of functionality needed for GhostSpell:
 * model loading, text completion, and cleanup.
 *
 * This file is compiled by CGo when building with -tags ghostai.
 */

#ifndef GHOST_AI_BRIDGE_H
#define GHOST_AI_BRIDGE_H

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

/* Opaque engine handle. */
typedef struct ghost_engine ghost_engine;

/* Engine configuration. */
typedef struct {
    int32_t  context_size;   /* Token context window (default: 2048)        */
    int32_t  threads;        /* CPU threads for inference                    */
    int32_t  batch_size;     /* Batch size for prompt processing             */
    float    temperature;    /* Sampling temperature                         */
    float    top_p;          /* Nucleus sampling top-p                       */
    int32_t  top_k;          /* Top-k sampling                               */
    float    repeat_penalty; /* Repetition penalty                           */
    int32_t  repeat_last_n;  /* Repeat penalty window                        */
    uint32_t seed;           /* RNG seed (0xFFFFFFFF = random)               */
} ghost_config;

/* Completion statistics. */
typedef struct {
    int32_t  prompt_tokens;
    int32_t  completion_tokens;
    int64_t  prompt_time_ms;
    int64_t  completion_time_ms;
    double   tokens_per_second;
} ghost_stats;

/* Model information. */
typedef struct {
    char     desc[256];
    uint64_t size_bytes;
    uint64_t n_params;
    int32_t  context_train;
    int32_t  vocab_size;
} ghost_model_info;

/*
 * Initialize the llama.cpp backend. Call once at process startup.
 */
void ghost_backend_init(void);

/*
 * Shut down the llama.cpp backend. Call once at process exit.
 */
void ghost_backend_free(void);

/*
 * Create a new engine with the given configuration.
 * Returns NULL on allocation failure.
 */
ghost_engine* ghost_engine_new(ghost_config cfg);

/*
 * Load a GGUF model file.
 * Returns 0 on success, -1 on error (check error_buf).
 * If a model is already loaded, it is unloaded first.
 */
int ghost_engine_load(ghost_engine* e, const char* model_path,
                      char* error_buf, int error_buf_size);

/*
 * Apply the model's chat template to format system + user messages.
 *
 * Uses llama_chat_apply_template() which reads the template from model
 * metadata (tokenizer.chat_template). Falls back to ChatML if none found.
 *
 * Returns a heap-allocated string (free with ghost_string_free),
 * or NULL on error.
 */
char* ghost_engine_apply_chat(ghost_engine* e,
                               const char* system_msg,
                               const char* user_msg,
                               char* error_buf, int error_buf_size);

/*
 * Run text completion.
 *
 * prompt:     Full prompt text (already formatted via apply_chat).
 * max_tokens: Maximum tokens to generate.
 * abort_flag: Pointer to an int; set to non-zero to cancel inference.
 *             The C inference loop checks this between tokens.
 * stats:      Filled with timing/token statistics.
 * error_buf:  Filled with error description on failure.
 *
 * Returns a heap-allocated string (free with ghost_string_free),
 * or NULL on error. May return partial text if aborted.
 */
char* ghost_engine_complete(ghost_engine* e,
                            const char* prompt,
                            int max_tokens,
                            volatile int* abort_flag,
                            ghost_stats* stats,
                            char* error_buf, int error_buf_size);

/*
 * Unload the model and free associated memory.
 * The engine remains valid — call ghost_engine_load() again to load a new model.
 */
void ghost_engine_unload(ghost_engine* e);

/*
 * Destroy the engine and free all resources.
 */
void ghost_engine_free(ghost_engine* e);

/*
 * Get information about the loaded model.
 * Returns 0 on success, -1 if no model is loaded.
 */
int ghost_engine_model_info(ghost_engine* e, ghost_model_info* info);

/*
 * Check if a model is currently loaded.
 */
int ghost_engine_is_loaded(ghost_engine* e);

/*
 * Free a string returned by ghost_engine_complete.
 */
void ghost_string_free(char* s);

#ifdef __cplusplus
}
#endif

#endif /* GHOST_AI_BRIDGE_H */
