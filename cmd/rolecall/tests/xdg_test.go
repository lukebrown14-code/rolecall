package main_test

import (
	"testing"

	"github.com/lukebrown14-code/rolecall/internal/testenv"
)

// TestMain pins the environment (internal/testenv) in this process and so in the binaries
// it starts with os.Environ(): config.toml resolves under each test's temp HOME, and the
// developer's NO_COLOR, ROLECALL_PLUGIN or $EDITOR never reach the binary under test.
func TestMain(m *testing.M) { testenv.Main(m) }
