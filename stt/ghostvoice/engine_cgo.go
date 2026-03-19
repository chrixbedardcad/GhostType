//go:build ghostvoice

package ghostvoice

/*
#cgo CFLAGS: -I${SRCDIR}/../../build/whisper/include -O2
#cgo LDFLAGS: -L${SRCDIR}/../../build/whisper/lib
#cgo linux LDFLAGS: -Wl,--start-group -lwhisper -lggml -lggml-cpu -lggml-base -Wl,--end-group -lstdc++ -lm -lpthread
#cgo darwin LDFLAGS: -lwhisper -lggml -lggml-cpu -lggml-base -lc++ -lm -lpthread -framework Accelerate
#cgo windows LDFLAGS: -lwhisper -lggml -lggml-cpu -lggml-base -lstdc++ -lm -lpthread -lkernel32

#include "bridge.h"
#include <stdlib.h>
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// cgoEngine is the real whisper.cpp engine.
type cgoEngine struct {
	handle *C.ghost_voice_engine
}

func newEngine(threads int) engine {
	h := C.ghost_voice_new(C.int32_t(threads))
	if h == nil {
		return &cgoEngine{}
	}
	return &cgoEngine{handle: h}
}

func engineAvailable() bool { return true }

func (e *cgoEngine) load(modelPath string) error {
	if e.handle == nil {
		return fmt.Errorf("engine handle is nil")
	}
	cPath := C.CString(modelPath)
	defer C.free(unsafe.Pointer(cPath))

	errBuf := make([]byte, 512)
	ret := C.ghost_voice_load(e.handle, cPath, (*C.char)(unsafe.Pointer(&errBuf[0])), C.int(len(errBuf)))
	if ret != 0 {
		return fmt.Errorf("%s", C.GoString((*C.char)(unsafe.Pointer(&errBuf[0]))))
	}
	return nil
}

func (e *cgoEngine) transcribe(pcmFloat []float32, language string) (string, string, error) {
	if e.handle == nil {
		return "", "", fmt.Errorf("engine handle is nil")
	}

	var cLang *C.char
	if language != "" {
		cLang = C.CString(language)
		defer C.free(unsafe.Pointer(cLang))
	}

	var result C.ghost_voice_result
	errBuf := make([]byte, 512)

	ret := C.ghost_voice_transcribe(
		e.handle,
		(*C.float)(unsafe.Pointer(&pcmFloat[0])),
		C.int32_t(len(pcmFloat)),
		cLang,
		&result,
		(*C.char)(unsafe.Pointer(&errBuf[0])),
		C.int(len(errBuf)),
	)
	if ret != 0 {
		return "", "", fmt.Errorf("%s", C.GoString((*C.char)(unsafe.Pointer(&errBuf[0]))))
	}

	text := C.GoString(result.text)
	lang := C.GoString(result.language)

	C.ghost_voice_string_free(result.text)
	C.ghost_voice_string_free(result.language)

	return text, lang, nil
}

func (e *cgoEngine) isLoaded() bool {
	if e.handle == nil {
		return false
	}
	return C.ghost_voice_is_loaded(e.handle) != 0
}

func (e *cgoEngine) unload() {
	if e.handle != nil {
		C.ghost_voice_unload(e.handle)
	}
}

func (e *cgoEngine) close() {
	if e.handle != nil {
		C.ghost_voice_free(e.handle)
		e.handle = nil
	}
}
