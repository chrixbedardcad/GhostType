package version

import (
	"regexp"
	"testing"
)

func TestVersionNonEmpty(t *testing.T) {
	if Version == "" {
		t.Fatal("Version must not be empty")
	}
}

func TestVersionSemverFormat(t *testing.T) {
	// Expect X.Y.Z where X, Y, Z are non-negative integers.
	semver := regexp.MustCompile(`^\d+\.\d+\.\d+$`)
	if !semver.MatchString(Version) {
		t.Fatalf("Version %q does not match semver format X.Y.Z", Version)
	}
}

func TestVersionNoVPrefix(t *testing.T) {
	if len(Version) > 0 && Version[0] == 'v' {
		t.Fatalf("Version %q must not have a 'v' prefix", Version)
	}
}
