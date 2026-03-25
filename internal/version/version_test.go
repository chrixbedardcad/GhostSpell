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

func TestVersionFormat(t *testing.T) {
	// Expect X.Y.Z.W where X, Y, Z, W are non-negative integers.
	// W is the PR number (0 if no PR).
	format := regexp.MustCompile(`^\d+\.\d+\.\d+\.\d+$`)
	if !format.MatchString(Version) {
		t.Fatalf("Version %q does not match format X.Y.Z.PR", Version)
	}
}

func TestVersionNoVPrefix(t *testing.T) {
	if len(Version) > 0 && Version[0] == 'v' {
		t.Fatalf("Version %q must not have a 'v' prefix", Version)
	}
}
