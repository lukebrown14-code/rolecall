package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLI(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../../fixtures/workspace")); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", ".", "--dir", root)
	out, err := cmd.CombinedOutput()
	if err != nil || string(out) != "Lead: 4\nDraft: 2\nApplied: 1\nInterviewing: 2\nOffer: 2\nRejected: 2\nWithdrawn: 2\nDismissed: 2\n" {
		t.Fatalf("%s %v", out, err)
	}
	home := t.TempDir()
	cmd = exec.Command("go", "run", ".")
	cmd.Env = append(os.Environ(), "HOME="+home, "GOCACHE="+filepath.Join(t.TempDir(), "cache"))
	out, err = cmd.CombinedOutput()
	if err == nil || !strings.HasPrefix(string(out), "no Workspace configured in "+filepath.Join(home, ".config/rolecall/config.toml")+"\n") {
		t.Fatalf("%s %v", out, err)
	}
}
