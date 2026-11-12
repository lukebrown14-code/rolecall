// Package config resolves the Owner's local settings.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Workspace    string `toml:"workspace"`
	Editor       string `toml:"editor"`
	FollowUpDays int    `toml:"follow_up_days"`
	Downloads    string `toml:"downloads"`
	Plugin       string `toml:"plugin"`
	Harness      string `toml:"harness"`
	ASCII        bool   `toml:"ascii"`
}

// Path is where the Owner's settings file lives under home.
func Path(home string) string { return filepath.Join(home, ".config/rolecall/config.toml") }

// defaultFollowUpDays is the Follow-up threshold when config sets none.
const defaultFollowUpDays = 14

// Load reads settings under home; dir overrides the configured Workspace.
func Load(home, dir string) (Config, error) {
	c := Config{FollowUpDays: defaultFollowUpDays}
	path := Path(home)
	b, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return c, err
	}
	if err := toml.Unmarshal(b, &c); err != nil {
		return c, fmt.Errorf("%s: invalid TOML", path)
	}
	if dir != "" {
		c.Workspace = dir
	}
	expand := func(s string) string {
		if s == "~" {
			return home
		}
		if strings.HasPrefix(s, "~/") {
			return filepath.Join(home, s[2:])
		}
		return s
	}
	c.Workspace = expand(c.Workspace)
	c.Downloads = expand(c.Downloads)
	// plugin is configured, taken from the environment, or found beside the
	// installed executable in a supported release layout.
	if c.Plugin == "" {
		c.Plugin = os.Getenv("ROLECALL_PLUGIN")
	}
	c.Plugin = expand(c.Plugin)
	if c.Plugin == "" {
		if executable, err := os.Executable(); err == nil {
			c.Plugin = InstalledPluginPath(executable)
		}
	}
	if c.Downloads == "" {
		c.Downloads = filepath.Join(home, "Downloads")
	}
	if c.Workspace == "" {
		return c, fmt.Errorf("no Workspace configured in %s", path)
	}
	return c, nil
}

// InstalledPluginPath returns the bundled Plugin location for an installed
// binary, or an empty string when neither supported layout is present.
func InstalledPluginPath(executable string) string {
	resolved, err := filepath.EvalSymlinks(executable)
	if err == nil {
		executable = resolved
	}
	binDir := filepath.Dir(executable)
	for _, candidate := range []string{
		filepath.Join(binDir, "plugin"),
		filepath.Join(binDir, "..", "libexec", "plugin"),
	} {
		info, err := os.Stat(candidate)
		if err == nil && info.IsDir() {
			resolved, err := filepath.Abs(candidate)
			if err == nil {
				return resolved
			}
		}
	}
	return ""
}
