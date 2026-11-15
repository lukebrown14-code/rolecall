package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lukebrown14-code/rolecall/internal/config"
)

// TestMain clears XDG_CONFIG_HOME so config.toml resolves under each test's temp home and
// never under the developer's real config directory.
func TestMain(m *testing.M) {
	os.Unsetenv("XDG_CONFIG_HOME")
	os.Exit(m.Run())
}

func TestPathHonoursXDGConfigHome(t *testing.T) {
	home, xdg := t.TempDir(), t.TempDir()
	if got, want := config.Path(home), filepath.Join(home, ".config", "rolecall", "config.toml"); got != want {
		t.Fatalf("Path without XDG_CONFIG_HOME = %q, want %q", got, want)
	}
	t.Setenv("XDG_CONFIG_HOME", xdg)
	if got, want := config.Path(home), filepath.Join(xdg, "rolecall", "config.toml"); got != want {
		t.Fatalf("Path with XDG_CONFIG_HOME = %q, want %q", got, want)
	}
	// The XDG spec ignores a relative value.
	t.Setenv("XDG_CONFIG_HOME", "relative/dir")
	if got, want := config.Path(home), filepath.Join(home, ".config", "rolecall", "config.toml"); got != want {
		t.Fatalf("Path with relative XDG_CONFIG_HOME = %q, want %q", got, want)
	}
}
