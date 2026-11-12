package config_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lukebrown14-code/rolecall/config"
)

func TestLoader(t *testing.T) {
	home := t.TempDir()
	c, err := config.Load(home, "")
	if err == nil || err.Error() != "no Workspace configured in "+filepath.Join(home, ".config/rolecall/config.toml") {
		t.Fatalf("missing: %v", err)
	}
	p := filepath.Join(home, ".config/rolecall")
	os.MkdirAll(p, 0755)
	os.WriteFile(filepath.Join(p, "config.toml"), []byte("workspace = \"~/work\"\neditor = \"vim\"\ndownloads = \"~/Downloads\"\n"), 0644)
	c, err = config.Load(home, "")
	if err != nil || c.Workspace != filepath.Join(home, "work") || c.Downloads != filepath.Join(home, "Downloads") || c.Editor != "vim" || c.FollowUpDays != 14 {
		t.Fatalf("%+v %v", c, err)
	}
	c, err = config.Load(home, "~/other")
	if err != nil || c.Workspace != filepath.Join(home, "other") {
		t.Fatalf("%+v %v", c, err)
	}
	os.WriteFile(filepath.Join(p, "config.toml"), []byte("follow_up_days = 21\n"), 0644)
	c, err = config.Load(home, "/override")
	if err != nil || c.FollowUpDays != 21 || c.Workspace != "/override" || c.Downloads != filepath.Join(home, "Downloads") {
		t.Fatalf("%+v %v", c, err)
	}
}

func TestMalformedConfigErrorDoesNotQuoteValues(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".config/rolecall/config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("workspace = \"private-sentinel\"\neditor = [\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := config.Load(home, "")
	if err == nil || !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), "invalid TOML") || strings.Contains(err.Error(), "private-sentinel") {
		t.Fatalf("want path and safe parse reason without config values, got %v", err)
	}
}

func TestPlugin(t *testing.T) {
	home := t.TempDir()
	p := filepath.Join(home, ".config/rolecall")
	os.MkdirAll(p, 0700)
	write := func(body string) {
		if err := os.WriteFile(filepath.Join(p, "config.toml"), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	// Configured, with ~ expansion.
	write("workspace = \"~/work\"\nplugin = \"~/rolecall/plugin\"\n")
	t.Setenv("ROLECALL_PLUGIN", "/env/plugin")
	c, err := config.Load(home, "")
	if err != nil || c.Plugin != filepath.Join(home, "rolecall/plugin") {
		t.Fatalf("configured: %+v %v", c, err)
	}
	// Unset falls back to $ROLECALL_PLUGIN.
	write("workspace = \"~/work\"\n")
	c, err = config.Load(home, "")
	if err != nil || c.Plugin != "/env/plugin" {
		t.Fatalf("env fallback: %+v %v", c, err)
	}
	// Neither set: empty, and nothing is auto-detected.
	t.Setenv("ROLECALL_PLUGIN", "")
	c, err = config.Load(home, "")
	if err != nil || c.Plugin != "" {
		t.Fatalf("unset: %+v %v", c, err)
	}
}

func TestInstalledPluginPath(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0700); err != nil {
		t.Fatal(err)
	}
	plugin := filepath.Join(root, "libexec", "plugin")
	if err := os.MkdirAll(plugin, 0700); err != nil {
		t.Fatal(err)
	}
	got := config.InstalledPluginPath(filepath.Join(bin, "rolecall"))
	if got != plugin {
		t.Fatalf("installed plugin path = %q, want %q", got, plugin)
	}
	if err := os.RemoveAll(plugin); err != nil {
		t.Fatal(err)
	}
	plugin = filepath.Join(bin, "plugin")
	if err := os.MkdirAll(plugin, 0700); err != nil {
		t.Fatal(err)
	}
	if got := config.InstalledPluginPath(filepath.Join(bin, "rolecall")); got != plugin {
		t.Fatalf("archive plugin path = %q, want %q", got, plugin)
	}
}

func TestLoadFindsInstalledPluginWhenUnset(t *testing.T) {
	if os.Getenv("ROLECALL_PLUGIN_TEST_HELPER") == "1" {
		c, err := config.Load(os.Getenv("HOME"), "")
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println(c.Plugin)
		return
	}

	root := t.TempDir()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	installedExecutable := filepath.Join(root, "bin", "rolecall.test")
	if err := os.MkdirAll(filepath.Dir(installedExecutable), 0700); err != nil {
		t.Fatal(err)
	}
	binary, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(installedExecutable, binary, 0700); err != nil {
		t.Fatal(err)
	}
	plugin := filepath.Join(root, "libexec", "plugin")
	if err := os.MkdirAll(plugin, 0700); err != nil {
		t.Fatal(err)
	}
	resolvedPlugin, err := filepath.EvalSymlinks(plugin)
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	configDir := filepath.Join(home, ".config", "rolecall")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte("workspace = \"~/work\"\n"), 0600); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(installedExecutable, "-test.run=^TestLoadFindsInstalledPluginWhenUnset$")
	cmd.Env = withoutEnvironment(os.Environ(), "ROLECALL_PLUGIN", "ROLECALL_PLUGIN_TEST_HELPER", "HOME")
	cmd.Env = append(cmd.Env, "HOME="+home, "ROLECALL_PLUGIN_TEST_HELPER=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("installed config loader failed: %s (%v)", output, err)
	}
	got := strings.TrimSuffix(string(output), "\nPASS\n")
	if got != resolvedPlugin {
		t.Fatalf("installed plugin = %q, want %q", got, resolvedPlugin)
	}
}

func withoutEnvironment(env []string, names ...string) []string {
	filtered := make([]string, 0, len(env))
	for _, value := range env {
		name, _, _ := strings.Cut(value, "=")
		remove := false
		for _, candidate := range names {
			if name == candidate {
				remove = true
				break
			}
		}
		if !remove {
			filtered = append(filtered, value)
		}
	}
	return filtered
}

func TestHarness(t *testing.T) {
	home := t.TempDir()
	p := filepath.Join(home, ".config/rolecall")
	os.MkdirAll(p, 0700)
	write := func(body string) {
		if err := os.WriteFile(filepath.Join(p, "config.toml"), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	// The command template is read verbatim, braces and quotes intact.
	write("workspace = \"~/work\"\nharness = \"claude \\\"/{skill} {dir}\\\"\"\n")
	c, err := config.Load(home, "")
	if err != nil || c.Harness != `claude "/{skill} {dir}"` {
		t.Fatalf("configured: %+v %v", c, err)
	}
	// Missing: empty, and config still loads as before.
	write("workspace = \"~/work\"\n")
	if c, err = config.Load(home, ""); err != nil || c.Harness != "" {
		t.Fatalf("unset: %+v %v", c, err)
	}
}

func TestTOMLLiteralAndComments(t *testing.T) {
	home := t.TempDir()
	p := filepath.Join(home, ".config/rolecall")
	os.MkdirAll(p, 0700)
	os.WriteFile(filepath.Join(p, "config.toml"), []byte("workspace = '~/work' # workspace\neditor = 'vim # literal' # editor\nfollow_up_days = 21 # days\ndownloads = \"~/Downloads\" # downloads\n"), 0600)
	for _, dir := range []string{"", "~/override"} {
		c, err := config.Load(home, dir)
		want := filepath.Join(home, "work")
		if dir != "" {
			want = filepath.Join(home, "override")
		}
		if err != nil || c.Workspace != want || c.FollowUpDays != 21 || c.Editor != "vim # literal" || c.Downloads != filepath.Join(home, "Downloads") {
			t.Fatalf("%+v %v", c, err)
		}
	}
}
