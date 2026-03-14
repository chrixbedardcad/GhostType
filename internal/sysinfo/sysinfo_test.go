package sysinfo

import (
	"runtime"
	"strings"
	"testing"
)

func TestCollectOS(t *testing.T) {
	info := Collect()
	if info.OS == "" {
		t.Fatal("OS must not be empty")
	}
	if info.OS != runtime.GOOS {
		t.Fatalf("OS = %q, want %q", info.OS, runtime.GOOS)
	}
}

func TestCollectArch(t *testing.T) {
	info := Collect()
	if info.Arch == "" {
		t.Fatal("Arch must not be empty")
	}
	if info.Arch != runtime.GOARCH {
		t.Fatalf("Arch = %q, want %q", info.Arch, runtime.GOARCH)
	}
}

func TestCollectGoVersion(t *testing.T) {
	info := Collect()
	if info.GoVersion == "" {
		t.Fatal("GoVersion must not be empty")
	}
	if info.GoVersion != runtime.Version() {
		t.Fatalf("GoVersion = %q, want %q", info.GoVersion, runtime.Version())
	}
}

func TestCollectOSVersion(t *testing.T) {
	info := Collect()
	// OSVersion should be populated (may be "unknown" if platform detection fails,
	// but must not be empty).
	if info.OSVersion == "" {
		t.Fatal("OSVersion must not be empty")
	}
}

func TestCollectLocale(t *testing.T) {
	info := Collect()
	// Locale should be populated (may be "unknown").
	if info.Locale == "" {
		t.Fatal("Locale must not be empty")
	}
}

func TestCollectKeyboardLayout(t *testing.T) {
	info := Collect()
	// KeyboardLayout should be populated (may be "unknown").
	if info.KeyboardLayout == "" {
		t.Fatal("KeyboardLayout must not be empty")
	}
}

func TestString(t *testing.T) {
	info := Collect()
	s := info.String()

	if s == "" {
		t.Fatal("String() must not return empty")
	}

	// Verify all expected fields appear in the output.
	for _, field := range []string{"OS:", "Locale:", "Keyboard:", "Go:"} {
		if !strings.Contains(s, field) {
			t.Fatalf("String() missing %q field, got:\n%s", field, s)
		}
	}
}

func TestStringContainsValues(t *testing.T) {
	info := Collect()
	s := info.String()

	// The OS name and Go version should appear in the string output.
	if !strings.Contains(s, info.OS) {
		t.Fatalf("String() does not contain OS %q", info.OS)
	}
	if !strings.Contains(s, info.GoVersion) {
		t.Fatalf("String() does not contain GoVersion %q", info.GoVersion)
	}
}
