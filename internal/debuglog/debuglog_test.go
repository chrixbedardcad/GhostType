package debuglog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewState(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	if s == nil {
		t.Fatal("New returned nil")
	}
	if s.Enabled() {
		t.Fatal("new State should not be enabled by default")
	}
}

func TestLogPath(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	want := filepath.Join(dir, "ghostspell.log")
	if got := s.LogPath(); got != want {
		t.Fatalf("LogPath() = %q, want %q", got, want)
	}
}

func TestEnableDisable(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)

	// Enable logging.
	logPath, err := s.Enable()
	if err != nil {
		t.Fatalf("Enable() error: %v", err)
	}
	if logPath == "" {
		t.Fatal("Enable() returned empty log path")
	}
	if !s.Enabled() {
		t.Fatal("expected Enabled() == true after Enable()")
	}

	// Log file should exist.
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Fatalf("log file %q was not created", logPath)
	}

	// Disable logging.
	s.Disable()
	if s.Enabled() {
		t.Fatal("expected Enabled() == false after Disable()")
	}
}

func TestEnableIdempotent(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)

	path1, err := s.Enable()
	if err != nil {
		t.Fatalf("first Enable() error: %v", err)
	}
	path2, err := s.Enable()
	if err != nil {
		t.Fatalf("second Enable() error: %v", err)
	}
	if path1 != path2 {
		t.Fatalf("Enable() returned different paths: %q vs %q", path1, path2)
	}

	s.Disable()
}

func TestDisableWhenNotEnabled(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	// Should not panic.
	s.Disable()
}

func TestTail(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)

	_, err := s.Enable()
	if err != nil {
		t.Fatalf("Enable() error: %v", err)
	}
	s.Disable()

	tail, err := s.Tail()
	if err != nil {
		t.Fatalf("Tail() error: %v", err)
	}
	if !strings.Contains(tail, "Debug logging enabled") {
		t.Fatalf("Tail() output missing expected content, got: %q", tail)
	}
}

func TestTailFileNotExist(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	// Log file doesn't exist yet — Tail should return an error.
	_, err := s.Tail()
	if err == nil {
		t.Fatal("expected error from Tail() when log file does not exist")
	}
}

func TestSetOnAutoStop(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	called := false
	s.SetOnAutoStop(func() { called = true })
	// Just verify it doesn't panic; the callback is tested via the auto timer.
	_ = called
}

func TestInitFromConfig(t *testing.T) {
	tests := []struct {
		level       string
		wantEnabled bool
	}{
		{"debug", true},
		{"info", false},
		{"warn", false},
		{"error", false},
		{"", true}, // default is debug
	}

	for _, tt := range tests {
		t.Run("level="+tt.level, func(t *testing.T) {
			dir := t.TempDir()
			s := New(dir)
			s.InitFromConfig(tt.level)

			if got := s.Enabled(); got != tt.wantEnabled {
				t.Fatalf("InitFromConfig(%q): Enabled() = %v, want %v", tt.level, got, tt.wantEnabled)
			}

			// Log file should have been created.
			if _, err := os.Stat(s.LogPath()); os.IsNotExist(err) {
				t.Fatalf("log file not created for level %q", tt.level)
			}

			// Clean up the file handle.
			s.Disable()
		})
	}
}
