package main

import (
	"testing"

	"github.com/lukebrown14-code/rolecall/internal/testenv"
)

// TestMain pins the environment (internal/testenv): config.toml resolves under each test's
// temp HOME and never under the developer's real config directory.
func TestMain(m *testing.M) { testenv.Main(m) }
