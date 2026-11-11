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
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../../fixtures/workspace")); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary, "--dir", root)
	out, err := cmd.CombinedOutput()
	if err != nil || string(out) != "Lead: 4\nDraft: 2\nApplied: 1\nInterviewing: 2\nOffer: 2\nRejected: 2\nWithdrawn: 2\nDismissed: 2\n" {
		t.Fatalf("%s %v", out, err)
	}
	homeConfig := t.TempDir()
	configDir := filepath.Join(homeConfig, ".config/rolecall")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte("workspace = '/missing' # configured\nfollow_up_days = 21 # days\neditor = 'vim'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command(binary, "--dir", root)
	cmd.Env = append(os.Environ(), "HOME="+homeConfig, "GOCACHE="+filepath.Join(t.TempDir(), "cache"))
	out, err = cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "Applied: 1\n") {
		t.Fatalf("%s %v", out, err)
	}
	home := t.TempDir()
	cmd = exec.Command(binary)
	cmd.Env = append(os.Environ(), "HOME="+home, "GOCACHE="+filepath.Join(t.TempDir(), "cache"))
	out, err = cmd.CombinedOutput()
	if err == nil || !strings.HasPrefix(string(out), "no Workspace configured in "+filepath.Join(home, ".config/rolecall/config.toml")+"\n") {
		t.Fatalf("%s %v", out, err)
	}
}
