//go:build ghostai

/*
 * Ghost-AI C Bridge Implementation
 *
 * Calls llama.cpp's C API (llama.h) to provide model loading and inference.
 * Compiled by CGo only when building with -tags ghostai.
 *
 * Pinned to llama.cpp API at tag b8281 (2025).
 */

#include "bridge.h"
#include "llama.h"

#include <stdlib.h>
#include <string.h>
#include <stdio.h>

/* --- Platform-portable timing --- */

#ifdef _WIN32
#include <windows.h>
static int64_t now_ms(void) {
    LARGE_INTEGER freq, counter;
    QueryPerformanceFrequency(&freq);
    QueryPerformanceCounter(&counter);
    return (int64_t)(counter.QuadPart * 1000 / freq.QuadPart);
}
#else
#include <time.h>
static int64_t now_ms(void) {
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return (int64_t)ts.tv_sec * 1000 + ts.tv_nsec / 1000000;
}
#endif

/* --- Engine struct --- */

struct ghost_engine {
    struct llama_model*        model;
    const struct llama_vocab*  vocab;
    ghost_config               config;
    int                        loaded;
};

/* --- Abort callback for llama_context --- */

/*
 * Called by ggml during tensor operations. Returns true (non-zero) to abort.
 * This is the deepest kill switch: it interrupts the decode loop immediately.
 */
static bool abort_callback(void* data) {
    volatile int* flag = (volatile int*)data;
    return flag && *flag != 0;
}

/* --- Public API --- */

void ghost_backend_init(void) {
    llama_backend_init();
}

void ghost_backend_free(void) {
    llama_backend_free();
}

ghost_engine* ghost_engine_new(ghost_config cfg) {
    ghost_engine* e = (ghost_engine*)calloc(1, sizeof(ghost_engine));
    if (!e) return NULL;

    /* Apply defaults for any unset fields. */
    if (cfg.context_size <= 0) cfg.context_size = 2048;
    if (cfg.threads       <= 0) cfg.threads       = 4;
    if (cfg.batch_size    <= 0) cfg.batch_size    = 512;
    if (cfg.temperature   <= 0) cfg.temperature   = 0.1f;
    if (cfg.top_p         <= 0) cfg.top_p         = 0.9f;
    if (cfg.top_k         <= 0) cfg.top_k         = 40;
    if (cfg.repeat_penalty<= 0) cfg.repeat_penalty= 1.1f;
    if (cfg.repeat_last_n <= 0) cfg.repeat_last_n = 64;
    if (cfg.seed          == 0) cfg.seed          = 0xFFFFFFFF;

    e->config = cfg;
    e->loaded = 0;
    return e;
}

int ghost_engine_load(ghost_engine* e, const char* model_path,
                      char* error_buf, int error_buf_size) {
    if (!e) {
        snprintf(error_buf, error_buf_size, "engine is NULL");
        return -1;
    }
    if (e->loaded) {
        ghost_engine_unload(e);
    }

    struct llama_model_params params = llama_model_default_params();
    params.n_gpu_layers = 0; /* CPU only — safe on all platforms */

    e->model = llama_model_load_from_file(model_path, params);
    if (!e->model) {
        snprintf(error_buf, error_buf_size, "failed to load model: %s", model_path);
        return -1;
    }

    e->vocab  = llama_model_get_vocab(e->model);
    e->loaded = 1;
    return 0;
}

char* ghost_engine_apply_chat(ghost_engine* e,
                               const char* system_msg,
                               const char* user_msg,
                               char* error_buf, int error_buf_size) {
    if (!e || !e->loaded || !e->model) {
        snprintf(error_buf, error_buf_size, "model not loaded");
        return NULL;
    }

    /* Build a 2-message chat: system + user. */
    struct llama_chat_message msgs[2];
    msgs[0].role    = "system";
    msgs[0].content = system_msg;
    msgs[1].role    = "user";
    msgs[1].content = user_msg;

    /* First call: measure required buffer size.
     * llama_chat_apply_template returns the number of chars written (or needed). */
    int needed = llama_chat_apply_template(
        llama_model_chat_template(e->model, NULL),
        msgs, 2,
        1 /* add_ass: append assistant turn start */,
        NULL, 0);

    if (needed < 0) {
        snprintf(error_buf, error_buf_size, "chat template apply failed (size query)");
        return NULL;
    }

    /* Allocate and render. */
    int buf_size = needed + 1;
    char* buf = (char*)malloc((size_t)buf_size);
    if (!buf) {
        snprintf(error_buf, error_buf_size, "out of memory (chat template)");
        return NULL;
    }

    int written = llama_chat_apply_template(
        llama_model_chat_template(e->model, NULL),
        msgs, 2,
        1,
        buf, buf_size);

    if (written < 0) {
        free(buf);
        snprintf(error_buf, error_buf_size, "chat template apply failed");
        return NULL;
    }

    buf[written] = '\0';
    return buf;
}

char* ghost_engine_complete(ghost_engine* e,
                            const char* prompt,
                            int max_tokens,
                            volatile int* abort_flag,
                            ghost_stats* stats,
                            char* error_buf, int error_buf_size) {
    if (!e || !e->loaded) {
        snprintf(error_buf, error_buf_size, "model not loaded");
        return NULL;
    }

    memset(stats, 0, sizeof(ghost_stats));

    /* --- Create context for this request --- */

    struct llama_context_params ctx_params = llama_context_default_params();
    ctx_params.n_ctx            = (uint32_t)e->config.context_size;
    ctx_params.n_batch          = (uint32_t)e->config.batch_size;
    ctx_params.n_threads        = e->config.threads;
    ctx_params.n_threads_batch  = e->config.threads;
    ctx_params.no_perf          = 1;

    /* Kill switch: abort callback checked during decode. */
    ctx_params.abort_callback      = abort_callback;
    ctx_params.abort_callback_data = (void*)abort_flag;

    struct llama_context* ctx = llama_init_from_model(e->model, ctx_params);
    if (!ctx) {
        snprintf(error_buf, error_buf_size, "failed to create inference context");
        return NULL;
    }

    /* --- Tokenize prompt --- */

    int prompt_len = (int)strlen(prompt);
    int n_max_tok  = prompt_len + 256; /* generous initial estimate */
    llama_token* tokens = (llama_token*)malloc((size_t)n_max_tok * sizeof(llama_token));
    if (!tokens) {
        llama_free(ctx);
        snprintf(error_buf, error_buf_size, "out of memory (tokenize alloc)");
        return NULL;
    }

    int n_prompt = llama_tokenize(e->vocab, prompt, prompt_len,
                                  tokens, n_max_tok, 1 /*add_special*/, 1 /*parse_special*/);
    if (n_prompt < 0) {
        /* Need more space: llama_tokenize returns -needed_size on overflow. */
        n_max_tok = -n_prompt + 1;
        llama_token* bigger = (llama_token*)realloc(tokens, (size_t)n_max_tok * sizeof(llama_token));
        if (!bigger) {
            free(tokens);
            llama_free(ctx);
            snprintf(error_buf, error_buf_size, "out of memory (tokenize realloc)");
            return NULL;
        }
        tokens = bigger;
        n_prompt = llama_tokenize(e->vocab, prompt, prompt_len,
                                  tokens, n_max_tok, 1, 1);
    }
    if (n_prompt < 0) {
        free(tokens);
        llama_free(ctx);
        snprintf(error_buf, error_buf_size, "tokenization failed");
        return NULL;
    }

    stats->prompt_tokens = n_prompt;

    /* --- Check context window --- */

    if (n_prompt + max_tokens > e->config.context_size) {
        max_tokens = e->config.context_size - n_prompt;
        if (max_tokens <= 0) {
            free(tokens);
            llama_free(ctx);
            snprintf(error_buf, error_buf_size,
                     "prompt too long: %d tokens (context is %d)",
                     n_prompt, e->config.context_size);
            return NULL;
        }
    }

    /* --- Process prompt (prefill) --- */

    int64_t t_prompt = now_ms();

    struct llama_batch batch = llama_batch_get_one(tokens, n_prompt);
    if (llama_decode(ctx, batch) != 0) {
        free(tokens);
        llama_free(ctx);
        if (abort_flag && *abort_flag) {
            snprintf(error_buf, error_buf_size, "aborted");
        } else {
            snprintf(error_buf, error_buf_size, "prompt decode failed");
        }
        return NULL;
    }

    stats->prompt_time_ms = now_ms() - t_prompt;

    /* --- Set up sampler chain --- */

    struct llama_sampler_chain_params sparams = llama_sampler_chain_default_params();
    sparams.no_perf = 1;
    struct llama_sampler* sampler = llama_sampler_chain_init(sparams);

    llama_sampler_chain_add(sampler, llama_sampler_init_top_k(e->config.top_k));
    llama_sampler_chain_add(sampler, llama_sampler_init_top_p(e->config.top_p, 1));
    llama_sampler_chain_add(sampler, llama_sampler_init_temp(e->config.temperature));
    llama_sampler_chain_add(sampler, llama_sampler_init_penalties(
        e->config.repeat_last_n,
        e->config.repeat_penalty,
        0.0f,  /* frequency penalty  */
        0.0f   /* presence penalty   */
    ));
    llama_sampler_chain_add(sampler, llama_sampler_init_dist(e->config.seed));

    /* --- Generate tokens --- */

    int64_t t_gen = now_ms();

    int out_cap = 4096;
    int out_len = 0;
    char* output = (char*)malloc((size_t)out_cap);
    if (!output) {
        free(tokens);
        llama_sampler_free(sampler);
        llama_free(ctx);
        snprintf(error_buf, error_buf_size, "out of memory (output alloc)");
        return NULL;
    }
    output[0] = '\0';

    char piece_buf[256];
    int n_gen = 0;

    /*
     * Early stop after </think>: once the model closes its thinking block,
     * the actual answer follows. Allow a generous budget (256 tokens) for
     * the answer, then stop — prevents runaway generation if thinking
     * wasn't fully suppressed by the template.
     */
    int post_think_budget = -1; /* -1 = not tracking (no </think> seen yet) */
    const char* think_close = "</think>";
    const int think_close_len = 8;

    for (int i = 0; i < max_tokens; i++) {
        /* Check abort between tokens. */
        if (abort_flag && *abort_flag) {
            break;
        }

        llama_token new_token = llama_sampler_sample(sampler, ctx, -1);

        /* End of generation? */
        if (llama_vocab_is_eog(e->vocab, new_token)) {
            break;
        }

        /* Detokenize. */
        int piece_len = llama_token_to_piece(e->vocab, new_token,
                                             piece_buf, (int)sizeof(piece_buf),
                                             0 /*lstrip*/, 0 /*special*/);
        if (piece_len > 0) {
            /* Grow output buffer if needed. */
            while (out_len + piece_len + 1 > out_cap) {
                out_cap *= 2;
                char* bigger = (char*)realloc(output, (size_t)out_cap);
                if (!bigger) {
                    free(output);
                    free(tokens);
                    llama_sampler_free(sampler);
                    llama_free(ctx);
                    snprintf(error_buf, error_buf_size, "out of memory (output grow)");
                    return NULL;
                }
                output = bigger;
            }
            memcpy(output + out_len, piece_buf, (size_t)piece_len);
            out_len += piece_len;
            output[out_len] = '\0';
        }

        n_gen++;

        /* Early stop: detect </think> in the output buffer. */
        if (post_think_budget < 0 && out_len >= think_close_len) {
            /* Search only the recent portion of the buffer. */
            int search_start = out_len - think_close_len - piece_len;
            if (search_start < 0) search_start = 0;
            if (strstr(output + search_start, think_close) != NULL) {
                post_think_budget = 256;
            }
        }
        if (post_think_budget >= 0) {
            if (--post_think_budget <= 0) {
                break;
            }
        }

        /* Feed new token back for next iteration. */
        struct llama_batch next = llama_batch_get_one(&new_token, 1);
        if (llama_decode(ctx, next) != 0) {
            /* Decode failed — abort callback may have fired, or real error. */
            break;
        }
    }

    stats->completion_tokens = n_gen;
    stats->completion_time_ms = now_ms() - t_gen;

    if (n_gen > 0 && stats->completion_time_ms > 0) {
        stats->tokens_per_second =
            (double)n_gen / ((double)stats->completion_time_ms / 1000.0);
    }

    /* --- Cleanup --- */

    free(tokens);
    llama_sampler_free(sampler);
    llama_free(ctx);

    /* If aborted with no output, signal error. */
    if (abort_flag && *abort_flag && out_len == 0) {
        free(output);
        snprintf(error_buf, error_buf_size, "aborted");
        return NULL;
    }

    return output;
}

void ghost_engine_unload(ghost_engine* e) {
    if (!e) return;
    if (e->model) {
        llama_model_free(e->model);
        e->model = NULL;
    }
    e->vocab  = NULL;
    e->loaded = 0;
}

void ghost_engine_free(ghost_engine* e) {
    if (!e) return;
    ghost_engine_unload(e);
    free(e);
}

int ghost_engine_model_info(ghost_engine* e, ghost_model_info* info) {
    if (!e || !e->loaded || !e->model) return -1;
    memset(info, 0, sizeof(ghost_model_info));

    llama_model_desc(e->model, info->desc, sizeof(info->desc));
    info->size_bytes    = llama_model_size(e->model);
    info->n_params      = llama_model_n_params(e->model);
    info->context_train = llama_model_n_ctx_train(e->model);
    info->vocab_size    = llama_vocab_n_tokens(e->vocab);

    return 0;
}

int ghost_engine_is_loaded(ghost_engine* e) {
    return (e && e->loaded) ? 1 : 0;
}

void ghost_string_free(char* s) {
    free(s);
}
