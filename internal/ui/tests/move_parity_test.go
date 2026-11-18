package ui_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
)

// sentTreeWithModes lists every entry under folder/sent with its mode and contents.
func sentTreeWithModes(t *testing.T, folder string) map[string]string {
	t.Helper()
	got := map[string]string{}
	sent := filepath.Join(folder, "sent")
	err := filepath.WalkDir(sent, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(sent, path)
		text := ""
		if !d.IsDir() {
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			text = string(b)
		}
		got[rel] = fmt.Sprintf("%v %q", info.Mode(), text)
		return nil
	})
	if err != nil {
		t.Fatalf("sent/: %v", err)
	}
	return got
}

// The TUI and `rolecall move` make one move (store.Move): the same meta.yml, bar the
// clock, and the same sent/, file for file and mode for mode.
func TestTUIAndCLIMoveToAppliedAgree(t *testing.T) {
	files := func() map[string]string {
		return map[string]string{"resume.md": "resume", "cover-letter.md": "letter", "jd.md": "jd", "fit.md": "fit",
			"out/Owner Resume Role.pdf": "%PDF resume", "out/Owner Role Cover Letter.pdf": "%PDF letter"}
	}
	tuiRoot, tuiFolder := appliedWorkspace(t, files())
	cliRoot, cliFolder := appliedWorkspace(t, files())

	tm, wait := appliedSession(t, tuiRoot)
	tm.Type("ma") // Ready: no prompt
	wait("Moved to Applied")
	quit(t, tm)

	binary := filepath.Join(t.TempDir(), "rolecall")
	if out, err := exec.Command("go", "build", "-o", binary, "../../../cmd/rolecall").CombinedOutput(); err != nil {
		t.Fatalf("build: %s %v", out, err)
	}
	cmd := exec.Command(binary, "move", "--dir", cliRoot, "jobs/SWE/role", "applied")
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	if out, err := cmd.CombinedOutput(); err != nil || string(out) != "Applied: jobs/SWE/role (sent/ saved)\n" {
		t.Fatalf("rolecall move: %q %v", out, err)
	}

	stamp := regexp.MustCompile(`at: [0-9T:Z-]+\}\n$`)
	meta := func(folder string) string {
		return stamp.ReplaceAllString(readText(t, filepath.Join(folder, "meta.yml")), "at: NOW}\n")
	}
	if a, b := meta(tuiFolder), meta(cliFolder); a != b {
		t.Fatalf("meta.yml differs:\nTUI: %q\nCLI: %q", a, b)
	}
	a, b := sentTreeWithModes(t, tuiFolder), sentTreeWithModes(t, cliFolder)
	if len(a) == 0 || fmt.Sprint(a) != fmt.Sprint(b) {
		t.Fatalf("sent/ differs:\nTUI: %v\nCLI: %v", a, b)
	}
}

func readText(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
