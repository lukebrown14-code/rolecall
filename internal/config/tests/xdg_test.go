package config_test

import (
	"path/filepath"
	"testing"

	"github.com/lukebrown14-code/rolecall/internal/config"
	"github.com/lukebrown14-code/rolecall/internal/testenv"
)

// TestMain pins the environment (internal/testenv): config.toml resolves under each test's
// temp home, never the developer's real config directory, and $VISUAL/$EDITOR are unset.
func TestMain(m *testing.M) { testenv.Main(m) }

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
