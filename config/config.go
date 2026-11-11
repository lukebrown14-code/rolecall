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
		return c, fmt.Errorf("%s: %w", path, err)
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
	if c.Downloads == "" {
		c.Downloads = filepath.Join(home, "Downloads")
	}
	if c.Workspace == "" {
		return c, fmt.Errorf("no Workspace configured in %s", path)
	}
	return c, nil
}
