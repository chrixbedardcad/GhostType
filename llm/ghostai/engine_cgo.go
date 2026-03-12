//go:build ghostai

package ghostai

/*
#cgo CFLAGS: -I${SRCDIR}/../../build/llama/include -O2
#cgo LDFLAGS: -L${SRCDIR}/../../build/llama/lib -lm -lpthread
#cgo linux LDFLAGS: -Wl,--start-group -lllama -lggml -lggml-cpu -lggml-base -Wl,--end-group -lstdc++
#cgo darwin LDFLAGS: -lllama -lggml -lggml-cpu -lggml-blas -lggml-base -lc++ -framework Accelerate
#cgo windows LDFLAGS: -lllama -lggml -lggml-cpu -lggml-base -lstdc++ -lkernel32

#include "bridge.h"
#include <stdlib.h>
*/
import "C"

import (
	"fmt"
	"sync"
	"unsafe"
)

var backendOnce sync.Once

// cgoBackend is the real Ghost-AI engine backed by llama.cpp via CGo.
type cgoBackend struct {
	handle *C.ghost_engine
}

func newBackend(config Config) engineBackend {
	backendOnce.Do(func() {
		C.ghost_backend_init()
	})

	cfg := C.ghost_config{
		context_size:   C.int32_t(config.ContextSize),
		threads:        C.int32_t(config.Threads),
		batch_size:     C.int32_t(config.BatchSize),
		temperature:    C.float(config.Temperature),
		top_p:          C.float(config.TopP),
		top_k:          C.int32_t(config.TopK),
		repeat_penalty: C.float(config.RepeatPenalty),
		repeat_last_n:  C.int32_t(config.RepeatLastN),
		seed:           C.uint32_t(config.Seed),
	}

	handle := C.ghost_engine_new(cfg)
	if handle == nil {
		// Allocation failure — return a backend that will error on every call.
		return &cgoBackend{handle: nil}
	}

	return &cgoBackend{handle: handle}
}

func backendAvailable() bool { return true }

func (b *cgoBackend) load(modelPath string) error {
	if b.handle == nil {
		return fmt.Errorf("engine allocation failed")
	}

	cPath := C.CString(modelPath)
	defer C.free(unsafe.Pointer(cPath))

	var errBuf [512]C.char
	rc := C.ghost_engine_load(b.handle, cPath, &errBuf[0], 512)
	if rc != 0 {
		return fmt.Errorf("%s", C.GoString(&errBuf[0]))
	}
	return nil
}

func (b *cgoBackend) complete(prompt string, maxTokens int, abort *int32) (string, Stats, error) {
	if b.handle == nil {
		return "", Stats{}, fmt.Errorf("engine allocation failed")
	}

	cPrompt := C.CString(prompt)
	defer C.free(unsafe.Pointer(cPrompt))

	var cStats C.ghost_stats
	var errBuf [512]C.char

	// abort is an *int32 that maps directly to C volatile int*.
	cAbort := (*C.int)(unsafe.Pointer(abort))

	result := C.ghost_engine_complete(
		b.handle,
		cPrompt,
		C.int(maxTokens),
		cAbort,
		&cStats,
		&errBuf[0], 512,
	)

	stats := Stats{
		PromptTokens:     int(cStats.prompt_tokens),
		CompletionTokens: int(cStats.completion_tokens),
		PromptTimeMs:     int64(cStats.prompt_time_ms),
		CompletionTimeMs: int64(cStats.completion_time_ms),
		TokensPerSecond:  float64(cStats.tokens_per_second),
	}

	if result == nil {
		errMsg := C.GoString(&errBuf[0])
		if errMsg == "aborted" {
			return "", stats, ErrAborted
		}
		return "", stats, fmt.Errorf("ghost-ai: %s", errMsg)
	}

	text := C.GoString(result)
	C.ghost_string_free(result)

	return text, stats, nil
}

func (b *cgoBackend) unload() {
	if b.handle != nil {
		C.ghost_engine_unload(b.handle)
	}
}

func (b *cgoBackend) isLoaded() bool {
	if b.handle == nil {
		return false
	}
	return C.ghost_engine_is_loaded(b.handle) != 0
}

func (b *cgoBackend) modelInfo() (ModelInfo, error) {
	if b.handle == nil {
		return ModelInfo{}, ErrNotLoaded
	}

	var cInfo C.ghost_model_info
	if C.ghost_engine_model_info(b.handle, &cInfo) != 0 {
		return ModelInfo{}, ErrNotLoaded
	}

	return ModelInfo{
		Description:  C.GoString(&cInfo.desc[0]),
		SizeBytes:    uint64(cInfo.size_bytes),
		NumParams:    uint64(cInfo.n_params),
		ContextTrain: int(cInfo.context_train),
		VocabSize:    int(cInfo.vocab_size),
	}, nil
}

func (b *cgoBackend) close() {
	if b.handle != nil {
		C.ghost_engine_free(b.handle)
		b.handle = nil
	}
}
