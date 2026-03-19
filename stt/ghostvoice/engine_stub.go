//go:build !ghostvoice

package ghostvoice

import "fmt"

// Stub engine for builds without -tags ghostvoice.

var ErrNotAvailable = fmt.Errorf("ghost-voice: engine not available (build with -tags ghostvoice)")

func newEngine(_ int) engine      { return &stubEngine{} }
func engineAvailable() bool       { return false }

type stubEngine struct{}

func (s *stubEngine) load(_ string) error                              { return ErrNotAvailable }
func (s *stubEngine) transcribe(_ []float32, _ string) (string, string, error) { return "", "", ErrNotAvailable }
func (s *stubEngine) isLoaded() bool                                   { return false }
func (s *stubEngine) unload()                                          {}
func (s *stubEngine) close()                                           {}
