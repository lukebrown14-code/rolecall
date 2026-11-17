package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// moveWorkspace is a scratch copy of the fixture Workspace, so a move writes nothing real.
func moveWorkspace(t *testing.T) string {
	t.Helper()
	ws := filepath.Join(t.TempDir(), "ws")
	if err := os.CopyFS(ws, os.DirFS(filepath.Join("..", "..", "..", "fixtures", "workspace"))); err != nil {
		t.Fatal(err)
	}
	return ws
}

func runMove(t *testing.T, ws string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(buildRolecall(t), append([]string{"move", "--dir", ws}, args...)...)
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestCLIMoveToAppliedAppendsStatusAndSavesSent(t *testing.T) {
	ws := moveWorkspace(t)
	folder := filepath.Join(ws, "jobs", "SWE", "fern")
	out, err := runMove(t, ws, "jobs/SWE/fern", "applied")
	if err != nil || out != "Applied: jobs/SWE/fern (sent/ saved)\n" {
		t.Fatalf("out = %q, err = %v", out, err)
	}
	if !strings.Contains(readFile(t, filepath.Join(folder, "meta.yml")), "{status: Applied, at: ") {
		t.Error("meta.yml has no Applied entry")
	}
	if _, err := os.Stat(filepath.Join(folder, "sent", "jd.md")); err != nil {
		t.Errorf("sent/jd.md not saved: %v", err)
	}
}

func TestCLIMoveAcceptsBareNameAndAbsolutePath(t *testing.T) {
	ws := moveWorkspace(t)
	if out, err := runMove(t, ws, "fern", "Interviewing"); err != nil || out != "Interviewing: jobs/SWE/fern\n" {
		t.Fatalf("bare name: out = %q, err = %v", out, err)
	}
	if out, err := runMove(t, ws, filepath.Join(ws, "jobs", "SWE", "fern"), "OFFER"); err != nil || out != "Offer: jobs/SWE/fern\n" {
		t.Fatalf("absolute: out = %q, err = %v", out, err)
	}
}

func TestCLIMoveToInterviewingCreatesNoSent(t *testing.T) {
	ws := moveWorkspace(t)
	folder := filepath.Join(ws, "jobs", "SWE", "fern")
	out, err := runMove(t, ws, "jobs/SWE/fern", "Interviewing")
	if err != nil || out != "Interviewing: jobs/SWE/fern\n" {
		t.Fatalf("out = %q, err = %v", out, err)
	}
	if _, err := os.Stat(filepath.Join(folder, "sent")); !os.IsNotExist(err) {
		t.Errorf("sent/ exists after a move to Interviewing: %v", err)
	}
}

func TestCLIMoveToCurrentStatusIsNoOp(t *testing.T) {
	ws := moveWorkspace(t)
	meta := filepath.Join(ws, "jobs", "SWE", "orbit", "meta.yml")
	before := readFile(t, meta)
	out, err := runMove(t, ws, "jobs/SWE/orbit", "applied")
	if err != nil || out != "Already Applied: jobs/SWE/orbit\n" {
		t.Fatalf("out = %q, err = %v", out, err)
	}
	if readFile(t, meta) != before {
		t.Error("meta.yml changed on a no-op move")
	}
}

func TestCLIMoveRefusesUnknownStatus(t *testing.T) {
	ws := moveWorkspace(t)
	meta := filepath.Join(ws, "jobs", "SWE", "fern", "meta.yml")
	before := readFile(t, meta)
	out, err := runMove(t, ws, "jobs/SWE/fern", "hired")
	if err == nil || !strings.Contains(out, `unknown Status "hired"`) {
		t.Fatalf("out = %q, err = %v", out, err)
	}
	if readFile(t, meta) != before {
		t.Error("meta.yml changed on a refused move")
	}
}

func TestCLIMoveRefusesBadFolder(t *testing.T) {
	ws := moveWorkspace(t)
	for _, folder := range []string{"jobs/SWE/nowhere", "jobs/SWE/not-an-application"} {
		out, err := runMove(t, ws, folder, "applied")
		if err == nil || strings.TrimSpace(out) == "" {
			t.Errorf("%s: out = %q, err = %v; want a failure with a message", folder, out, err)
		}
	}
}
