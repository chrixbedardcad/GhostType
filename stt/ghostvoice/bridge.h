//go:build ghostvoice

/*
 * Ghost Voice C Bridge
 *
 * Thin wrapper around whisper.cpp's C API (whisper.h).
 * Only exposes the subset needed for GhostSpell:
 * model loading, audio transcription, and cleanup.
 *
 * This file is compiled by CGo when building with -tags ghostvoice.
 */

#ifndef GHOST_VOICE_BRIDGE_H
#define GHOST_VOICE_BRIDGE_H

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

/* Opaque engine handle. */
typedef struct ghost_voice_engine ghost_voice_engine;

/* Transcription result. */
typedef struct {
    char    *text;           /* Transcribed text (heap-allocated, caller frees) */
    char    *language;       /* Detected language code (heap-allocated)          */
    int32_t  segments;       /* Number of text segments                          */
    int64_t  process_time_ms;/* Total processing time in milliseconds            */
} ghost_voice_result;

/* Create a new voice engine. */
ghost_voice_engine *ghost_voice_new(int32_t threads);

/* Load a GGML whisper model from path.
 * Returns 0 on success, nonzero on error (writes to errBuf). */
int ghost_voice_load(ghost_voice_engine *eng, const char *model_path,
                     char *errBuf, int errBufLen);

/* Transcribe PCM audio data.
 * pcm: 16-bit signed, mono, 16kHz samples as float32 (whisper.cpp expects float).
 * n_samples: number of float samples.
 * language: BCP-47 language code or NULL for auto-detect.
 * Returns 0 on success, nonzero on error. */
int ghost_voice_transcribe(ghost_voice_engine *eng,
                           const float *pcm, int32_t n_samples,
                           const char *language,
                           ghost_voice_result *result,
                           char *errBuf, int errBufLen);

/* Check if a model is loaded. */
int ghost_voice_is_loaded(ghost_voice_engine *eng);

/* Request abort of in-progress transcription. Thread-safe. */
void ghost_voice_abort(ghost_voice_engine *eng);

/* Unload the current model (free memory). */
void ghost_voice_unload(ghost_voice_engine *eng);

/* Destroy the engine. */
void ghost_voice_free(ghost_voice_engine *eng);

/* Free a heap-allocated string returned by the engine. */
void ghost_voice_string_free(char *str);

#ifdef __cplusplus
}
#endif

#endif /* GHOST_VOICE_BRIDGE_H */
