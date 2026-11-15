package main

import (
	"os"
	"testing"
)

// TestMain clears XDG_CONFIG_HOME so config.toml resolves under each test's temp HOME and
// never under the developer's real config directory.
func TestMain(m *testing.M) {
	os.Unsetenv("XDG_CONFIG_HOME")
	os.Unsetenv("XDG_STATE_HOME")
	os.Exit(m.Run())
}
