package main_test

import (
	"os"
	"testing"
)

// TestMain clears XDG_CONFIG_HOME so config.toml resolves under each test's temp HOME and
// never under the developer's real config directory, in this process and in the binaries
// it starts with os.Environ(). The diagnostics log is off for the same reason; a test
// that wants it turns it on with a temp HOME.
func TestMain(m *testing.M) {
	os.Unsetenv("XDG_CONFIG_HOME")
	os.Unsetenv("XDG_STATE_HOME")
	os.Setenv("ROLECALL_DIAGNOSTICS", "0")
	os.Exit(m.Run())
}
