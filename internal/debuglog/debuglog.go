// Package debuglog provides a runtime-switchable debug logger.
// It manages the log file handle, auto-disable timer, and slog default swap.
package debuglog

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	autoDisableAfter = 30 * time.Minute
	logFileName      = "ghostspell.log"
	tailLines        = 200
)

// State holds the runtime debug logging state. Safe for concurrent use.
type State struct {
	mu          sync.Mutex
	enabled     bool
	logFile     *os.File
	logPath     string
	configDir   string
	autoTimer   *time.Timer
	onAutoStop  func() // called when auto-disable fires (e.g. to refresh tray menu)
}

// New creates a new debug log state rooted in the given config directory.
func New(configDir string) *State {
	return &State{
		configDir: configDir,
		logPath:   filepath.Join(configDir, logFileName),
	}
}

// Enable activates debug-level logging to the log file.
// Returns the log file path.
func (s *State) Enable() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.enabled {
		return s.logPath, nil
	}

	if err := os.MkdirAll(s.configDir, 0755); err != nil {
		return "", fmt.Errorf("create log dir: %w", err)
	}

	f, err := os.OpenFile(s.logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return "", fmt.Errorf("open log file: %w", err)
	}

	s.logFile = f
	s.enabled = true

	logger := slog.New(slog.NewTextHandler(f, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(logger)

	slog.Info("=== Debug logging enabled ===")

	// Auto-disable timer.
	if s.autoTimer != nil {
		s.autoTimer.Stop()
	}
	s.autoTimer = time.AfterFunc(autoDisableAfter, func() {
		s.Disable()
		fmt.Println("[debuglog] Auto-disabled after 30 minutes")
		if s.onAutoStop != nil {
			s.onAutoStop()
		}
	})

	return s.logPath, nil
}

// Disable deactivates debug logging and closes the log file.
func (s *State) Disable() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.enabled {
		return
	}

	slog.Info("=== Debug logging disabled ===")

	// Switch to no-op logger.
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError + 1})))

	if s.autoTimer != nil {
		s.autoTimer.Stop()
		s.autoTimer = nil
	}

	if s.logFile != nil {
		s.logFile.Close()
		s.logFile = nil
	}

	s.enabled = false
}

// Enabled returns whether debug logging is currently active.
func (s *State) Enabled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.enabled
}

// LogPath returns the path to the log file.
func (s *State) LogPath() string {
	return s.logPath
}

// SetOnAutoStop sets a callback invoked when the auto-disable timer fires.
func (s *State) SetOnAutoStop(fn func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onAutoStop = fn
}

// Tail returns the last N lines of the log file as a string.
func (s *State) Tail() (string, error) {
	data, err := os.ReadFile(s.logPath)
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) > tailLines {
		lines = lines[len(lines)-tailLines:]
	}
	return strings.Join(lines, "\n"), nil
}

// InitFromConfig sets up logging based on the existing config values.
// This is called at startup to honour the config file's log_level setting.
func (s *State) InitFromConfig(logLevel string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	logLevel = strings.ToLower(strings.TrimSpace(logLevel))
	if logLevel == "" {
		// Default to debug logging during development — always have a log file.
		logLevel = "debug"
	}

	level := slog.LevelInfo
	switch logLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	if err := os.MkdirAll(s.configDir, 0755); err != nil {
		return
	}
	f, err := os.OpenFile(s.logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}

	s.logFile = f
	s.enabled = logLevel == "debug"

	logger := slog.New(slog.NewTextHandler(f, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)
}
