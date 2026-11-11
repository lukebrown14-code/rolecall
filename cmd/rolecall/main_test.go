package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLI(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "rolecall")
	if out, err := exec.Command("go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("%s %v", out, err)
	}
	home := t.TempDir()
	cmd := exec.Command(binary)
	cmd.Env = append(os.Environ(), "HOME="+home, "GOCACHE="+filepath.Join(t.TempDir(), "cache"))
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.HasPrefix(string(out), "no Workspace configured in "+filepath.Join(home, ".config/rolecall/config.toml")+"\n") {
		t.Fatalf("%s %v", out, err)
	}
}

// A configured Workspace that is not on disk is reported on one line that names the config file.
func TestCLIMissingWorkspaceNamesConfigPath(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "rolecall")
	if out, err := exec.Command("go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("%s %v", out, err)
	}
	home := t.TempDir()
	config := filepath.Join(home, ".config/rolecall/config.toml")
	if err := os.MkdirAll(filepath.Dir(config), 0o755); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(home, "nowhere")
	if err := os.WriteFile(config, []byte("workspace = \""+missing+"\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary)
	cmd.Env = append(os.Environ(), "HOME="+home)
	out, err := cmd.CombinedOutput()
	if err == nil || strings.Count(string(out), "\n") != 1 || !strings.Contains(string(out), config) || !strings.Contains(string(out), missing) {
		t.Fatalf("want one line naming %s and %s, got %q %v", config, missing, out, err)
	}
}

func TestCLIMalformedWorkspaceNamesPathWithoutValues(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "rolecall")
	if out, err := exec.Command("go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build failed: %s %v", out, err)
	}
	home := t.TempDir()
	config := filepath.Join(home, ".config/rolecall/config.toml")
	if err := os.MkdirAll(filepath.Dir(config), 0o755); err != nil {
		t.Fatal(err)
	}
	workspace := filepath.Join(home, "workspace")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, []byte("workspace = \""+workspace+"\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	workspaceConfig := filepath.Join(workspace, "workspace.json")
	if err := os.WriteFile(workspaceConfig, []byte(`{"tracks":"private-sentinel"`), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary)
	cmd.Env = append(os.Environ(), "HOME="+home)
	out, err := cmd.CombinedOutput()
	message := string(out)
	if err == nil || !strings.Contains(message, workspaceConfig) || !strings.Contains(message, "invalid Workspace settings") || strings.Contains(message, "private-sentinel") {
		t.Fatalf("want safe error naming %s, got %q %v", workspaceConfig, out, err)
	}
}
