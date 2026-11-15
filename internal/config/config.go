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
	// InheritTerminal draws with the terminal's own ANSI-16 colours and paints no background.
	InheritTerminal bool `toml:"inherit_terminal"`

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

	// Warnings are problems Load worked around, such as a follow_up_days outside 1-90
	// that it clamped into range.
	Warnings []Warning `toml:"-"`
}

// Warning is one config key Load could not use as written: Text says what it did
// instead, without the file's path.
type Warning struct {
	Key, Text string
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

// Path is where the Owner's settings file lives: rolecall/config.toml under
// $XDG_CONFIG_HOME when that is an absolute path, as the XDG spec asks, else under
// home/.config.
func Path(home string) string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); filepath.IsAbs(dir) {
		return filepath.Join(dir, "rolecall", "config.toml")
	}
	return filepath.Join(home, ".config", "rolecall", "config.toml")
}

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

// The follow_up_days range Settings accepts; Load clamps a hand-edited value into it.
const (
	MinFollowUpDays = 1
	MaxFollowUpDays = 90
)

// ErrNoWorkspace reports that neither config.toml nor --dir names a Workspace. Load
// still returns every other setting alongside it, so doctor can check the rest.
var ErrNoWorkspace = errors.New("no Workspace configured")

// FileError is a config.toml rolecall cannot read: not valid TOML, or a key holding the
// wrong kind of value. Line and Column are 1-based, 0 when the parser gave no position.
// Reason never quotes the file's values, which may be private paths.
type FileError struct {
	Path         string
	Line, Column int
	Reason       string
}

func (e *FileError) Error() string {
	if where := e.Where(); where != "" {
		return e.Path + " " + where + ": " + e.Reason
	}
	return e.Path + ": " + e.Reason
}

// Where is "line 2, column 15", or "" without a position.
func (e *FileError) Where() string {
	switch {
	case e.Line == 0:
		return ""
	case e.Column == 0:
		return fmt.Sprintf("line %d", e.Line)
	}
	return fmt.Sprintf("line %d, column %d", e.Line, e.Column)
}

// fileError turns the parser's error into a FileError with its position and reason.
func fileError(path string, err error) *FileError {
	fe := &FileError{Path: path, Reason: strings.TrimPrefix(err.Error(), "toml: ")}
	var de *toml.DecodeError
	if errors.As(err, &de) {
		fe.Line, fe.Column = de.Position()
		if key := strings.Join(de.Key(), "."); key != "" && strings.Contains(fe.Reason, "cannot decode") {
			fe.Reason = key + " must be " + keyKind(key)
		}
	}
	return fe
}

// keyKind is what a config key holds, for the wrong-type message.
func keyKind(key string) string {
	switch key {
	case "follow_up_days":
		return "a whole number of days, like 14"
	case "ascii", "inherit_terminal":
		return "true or false"
	}
	return "a quoted string"
}

// Load reads settings under home; dir overrides the configured Workspace. A config.toml
// that cannot be read returns a *FileError together with every setting at its default
// (and dir, when given), so the caller can carry on and show the error.
func Load(home, dir string) (Config, error) {
	var c Config
	path := Path(home)
	b, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return c, err
	}
	var bad *FileError
	if err := toml.Unmarshal(b, &c); err != nil {
		bad, b, c = fileError(path, err), nil, Config{}
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
	} else if days := max(MinFollowUpDays, min(c.FollowUpDays, MaxFollowUpDays)); days != c.FollowUpDays {
		c.Warnings = append(c.Warnings, Warning{Key: "follow_up_days", Text: fmt.Sprintf(
			"follow_up_days = %d is outside %d-%d; using %d", c.FollowUpDays, MinFollowUpDays, MaxFollowUpDays, days)})
		c.FollowUpDays = days
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
	if bad != nil {
		return c, bad
	}
	if c.Workspace == "" {
		return c, fmt.Errorf("%w in %s", ErrNoWorkspace, path)
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
