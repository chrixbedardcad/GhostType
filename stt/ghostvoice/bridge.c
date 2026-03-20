//go:build ghostvoice

/*
 * Ghost Voice C Bridge — whisper.cpp wrapper.
 *
 * Provides a simplified C API around whisper.cpp for GhostSpell's
 * speech-to-text pipeline. Compiled via CGo with -tags ghostvoice.
 */

#include "bridge.h"
#include "whisper.h"
#include <stdlib.h>
#include <string.h>
#include <stdio.h>

struct ghost_voice_engine {
    struct whisper_context *ctx;
    int32_t threads;
    volatile int abort_flag;
};

/* Abort callback — whisper.cpp checks this periodically during inference. */
static bool ghost_voice_abort_cb(void *user_data) {
    ghost_voice_engine *eng = (ghost_voice_engine *)user_data;
    return eng && eng->abort_flag;
}

void ghost_voice_abort(ghost_voice_engine *eng) {
    if (eng) eng->abort_flag = 1;
}

ghost_voice_engine *ghost_voice_new(int32_t threads) {
    ghost_voice_engine *eng = (ghost_voice_engine *)calloc(1, sizeof(ghost_voice_engine));
    if (!eng) return NULL;
    eng->threads = threads > 0 ? threads : 4;
    return eng;
}

int ghost_voice_load(ghost_voice_engine *eng, const char *model_path,
                     char *errBuf, int errBufLen) {
    if (!eng) {
        snprintf(errBuf, errBufLen, "engine is NULL");
        return -1;
    }
    if (eng->ctx) {
        whisper_free(eng->ctx);
        eng->ctx = NULL;
    }

    struct whisper_context_params cparams = whisper_context_default_params();
    eng->ctx = whisper_init_from_file_with_params(model_path, cparams);
    if (!eng->ctx) {
        snprintf(errBuf, errBufLen, "failed to load whisper model: %s", model_path);
        return -1;
    }
    return 0;
}

int ghost_voice_transcribe(ghost_voice_engine *eng,
                           const float *pcm, int32_t n_samples,
                           const char *language,
                           ghost_voice_result *result,
                           char *errBuf, int errBufLen) {
    if (!eng || !eng->ctx) {
        snprintf(errBuf, errBufLen, "model not loaded");
        return -1;
    }

    struct whisper_full_params params = whisper_full_default_params(WHISPER_SAMPLING_GREEDY);
    params.n_threads = eng->threads;
    params.print_progress = 0;
    params.print_realtime = 0;
    params.print_timestamps = 0;
    params.single_segment = 0;
    params.no_timestamps = 1;

    if (language && strlen(language) > 0) {
        params.language = language;
        params.detect_language = 0;
    } else {
        params.language = "auto";
        params.detect_language = 1;
    }

    /* Wire abort callback so Go can cancel mid-inference via context. */
    eng->abort_flag = 0;
    params.abort_callback = ghost_voice_abort_cb;
    params.abort_callback_user_data = eng;

    int ret = whisper_full(eng->ctx, params, pcm, n_samples);
    if (ret != 0) {
        snprintf(errBuf, errBufLen, "whisper_full failed with code %d", ret);
        return -1;
    }

    /* Collect all segment text. */
    int n_segments = whisper_full_n_segments(eng->ctx);
    size_t total_len = 0;
    for (int i = 0; i < n_segments; i++) {
        const char *seg = whisper_full_get_segment_text(eng->ctx, i);
        if (seg) total_len += strlen(seg);
    }

    char *text = (char *)malloc(total_len + 1);
    if (!text) {
        snprintf(errBuf, errBufLen, "out of memory");
        return -1;
    }
    text[0] = '\0';

    for (int i = 0; i < n_segments; i++) {
        const char *seg = whisper_full_get_segment_text(eng->ctx, i);
        if (seg) strcat(text, seg);
    }

    /* Detect language. */
    const char *detected_lang = whisper_lang_str(whisper_full_lang_id(eng->ctx));

    result->text = text;
    result->language = detected_lang ? strdup(detected_lang) : strdup("unknown");
    result->segments = n_segments;
    result->process_time_ms = 0; /* TODO: measure actual time */

    return 0;
}

int ghost_voice_is_loaded(ghost_voice_engine *eng) {
    return (eng && eng->ctx) ? 1 : 0;
}

void ghost_voice_unload(ghost_voice_engine *eng) {
    if (eng && eng->ctx) {
        whisper_free(eng->ctx);
        eng->ctx = NULL;
    }
}

void ghost_voice_free(ghost_voice_engine *eng) {
    if (eng) {
        if (eng->ctx) whisper_free(eng->ctx);
        free(eng);
    }
}

void ghost_voice_string_free(char *str) {
    free(str);
}
