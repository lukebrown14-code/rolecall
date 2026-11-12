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

	// EditorSource names where the effective Editor came from: one of the Source*
	// constants. Editor itself stays the configured value (empty when unset), so the
	// ui keeps resolving the fallbacks the same way it always has.
	EditorSource string `toml:"-"`

	// The same for the other settings: one of the Source* constants (or SourceFlag).
	WorkspaceSource    string `toml:"-"`
	FollowUpDaysSource string `toml:"-"`
	DownloadsSource    string `toml:"-"`
	PluginSource       string `toml:"-"`
	HarnessSource      string `toml:"-"`
}

// Where an effective value came from, as the Settings view names it.
const (
	SourceConfig  = "config.toml"
	SourceVisual  = "$VISUAL"
	SourceEditor  = "$EDITOR"
	SourceDefault = "default"
	SourceFlag    = "--dir"
	SourcePlugin  = "$ROLECALL_PLUGIN"
)

// editorSource follows the Editor resolution order: config, $VISUAL, $EDITOR, vi.
func editorSource(configured string) string {
	switch {
	case configured != "":
		return SourceConfig
	case os.Getenv("VISUAL") != "":
		return SourceVisual
	case os.Getenv("EDITOR") != "":
		return SourceEditor
	}
	return SourceDefault
}

// Path is where the Owner's settings file lives under home.
func Path(home string) string { return filepath.Join(home, ".config/rolecall/config.toml") }

// Ensure creates an empty config file (and its directory) at path if it is missing.
func Ensure(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), dirMode); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, fileMode)
	if err != nil {
		return err
	}
	return f.Close()
}

// defaultFollowUpDays is the Follow-up threshold when config sets none.
const defaultFollowUpDays = 14

// Load reads settings under home; dir overrides the configured Workspace.
func Load(home, dir string) (Config, error) {
	var c Config
	path := Path(home)
	b, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return c, err
	}
	if err := toml.Unmarshal(b, &c); err != nil {
		return c, fmt.Errorf("%s: invalid TOML", path)
	}
	// A pointer tells an explicit follow_up_days = 0 from an absent key.
	var present struct {
		FollowUpDays *int `toml:"follow_up_days"`
	}
	_ = toml.Unmarshal(b, &present)
	set := func(configured bool) string {
		if configured {
			return SourceConfig
		}
		return SourceDefault
	}
	c.FollowUpDaysSource = set(present.FollowUpDays != nil)
	if present.FollowUpDays == nil {
		c.FollowUpDays = defaultFollowUpDays
	}
	c.DownloadsSource = set(c.Downloads != "")
	c.HarnessSource = set(c.Harness != "")
	c.WorkspaceSource = set(c.Workspace != "")
	if dir != "" {
		c.Workspace, c.WorkspaceSource = dir, SourceFlag
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
	c.EditorSource = editorSource(c.Editor)
	c.Workspace = expand(c.Workspace)
	c.Downloads = expand(c.Downloads)
	c.PluginSource = set(c.Plugin != "")
	// plugin is configured, taken from the environment, or found beside the
	// installed executable in a supported release layout.
	if c.Plugin == "" {
		if c.Plugin = os.Getenv("ROLECALL_PLUGIN"); c.Plugin != "" {
			c.PluginSource = SourcePlugin
		}
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
