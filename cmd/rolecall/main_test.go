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
	if err != nil || string(out) != "Lead: 3\nDraft: 1\nApplied: 1\nInterviewing: 1\nOffer: 1\nRejected: 1\nWithdrawn: 1\nDismissed: 1\n" {
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
