// Package config resolves the Owner's local settings.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Workspace    string
	Editor       string
	FollowUpDays int
	Downloads    string
}

// Load reads settings under home; dir overrides the configured Workspace.
func Load(home, dir string) (Config, error) {
	c := Config{FollowUpDays: 14}
	path := filepath.Join(home, ".config/rolecall/config.toml")
	b, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return c, err
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			return c, fmt.Errorf("%s: invalid setting", path)
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if k == "follow_up_days" {
			c.FollowUpDays, err = strconv.Atoi(v)
		} else {
			var s string
			s, err = strconv.Unquote(v)
			switch k {
			case "workspace":
				c.Workspace = s
			case "editor":
				c.Editor = s
			case "downloads":
				c.Downloads = s
			}
		}
		if err != nil {
			return c, fmt.Errorf("%s: %w", path, err)
		}
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
	if c.Workspace == "" {
		return c, fmt.Errorf("no Workspace configured in %s", path)
	}
	return c, nil
}
