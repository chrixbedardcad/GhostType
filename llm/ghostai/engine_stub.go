//go:build !ghostai

package ghostai

// stubBackend is used when Ghost-AI is not compiled in (build without -tags ghostai).
// All methods return ErrNotAvailable.
type stubBackend struct{}

func newBackend(_ Config) engineBackend {
	return &stubBackend{}
}

func backendAvailable() bool { return false }

func (s *stubBackend) load(_ string) error                                  { return ErrNotAvailable }
func (s *stubBackend) applyChat(_, _ string) (string, error)                { return "", ErrNotAvailable }
func (s *stubBackend) complete(_ string, _ int, _ *int32) (string, Stats, error) {
	return "", Stats{}, ErrNotAvailable
}
func (s *stubBackend) unload()                    {}
func (s *stubBackend) isLoaded() bool             { return false }
func (s *stubBackend) modelInfo() (ModelInfo, error) { return ModelInfo{}, ErrNotAvailable }
func (s *stubBackend) close()                     {}
