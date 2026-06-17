package main

import (
	"testing"
)

// TestVersion asserts the binary has a version string for --version output.
func TestVersion(t *testing.T) {
	if Version == "" {
		t.Fatal("Version should not be empty")
	}
}
