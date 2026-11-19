package store_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/lukebrown14-code/rolecall/internal/store"
)

// TestShowcaseWorkspace keeps examples/workspace, the README's and the demo GIF's
// Workspace, what it promises: a complete Profile with a built CV, every Status, Ready
// documents on everything that was sent, and exactly one malformed example.
func TestShowcaseWorkspace(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../../../examples/workspace")); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	apps, err := s.Scan()
	if err != nil {
		t.Fatal(err)
	}

	statuses := map[string]int{}
	var malformed, filtered, resumeOnly, written int
	for _, a := range apps {
		if a.Malformed != nil {
			malformed++
			continue
		}
		statuses[a.Status]++
		if a.Filtered() {
			filtered++
		}
		if a.ResumeOnly {
			resumeOnly++
		}
		if a.Documents == store.DocsWritten {
			written++
		}
		if a.AppliedDate != nil && (a.Documents != store.DocsReady || !a.HasSent) {
			t.Errorf("%s was sent but its documents are %q (sent/ %v); build its PDFs and freeze sent/", rel(root, a.Folder), a.Documents, a.HasSent)
		}
	}
	for _, status := range []string{store.StatusLead, store.StatusDraft, store.StatusApplied, store.StatusInterviewing, store.StatusOffer, store.StatusRejected, store.StatusWithdrawn, store.StatusDismissed} {
		if statuses[status] == 0 {
			t.Errorf("no Application is %s", status)
		}
	}
	if malformed != 1 || filtered == 0 || resumeOnly == 0 || written == 0 {
		t.Errorf("want one malformed and at least one Filtered, Resume only and Written; got %d, %d, %d, %d", malformed, filtered, resumeOnly, written)
	}

	profile, err := s.Profile()
	if err != nil {
		t.Fatal(err)
	}
	if len(profile.Missing) > 0 || len(profile.Files) != 3 {
		t.Errorf("Profile: missing Contact fields %v, %d other files", profile.Missing, len(profile.Files))
	}
	if len(profile.CVs) == 0 || profile.CVs[0].PDF == "" {
		t.Errorf("want a built general CV: %+v", profile.CVs)
	}
}

// The Plugin's reader (plugin/scripts/meta.py) reads every showcase Application as the
// store does, and finds the same one malformed: the TUI and the Harness see one Workspace.
func TestShowcasePythonReaderAgrees(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is not available")
	}
	// meta.py resolves symlinks in the Workspace path (macOS's /var is /private/var), so
	// hand both readers the resolved path and their Folder values compare.
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.CopyFS(root, os.DirFS("../../../examples/workspace")); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(python, "../../../plugin/scripts/meta.py", "--data", root)
	cmd.Env = append(os.Environ(), "TZ=UTC")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("meta.py: %v", err)
	}
	var read []map[string]any
	if err := json.Unmarshal(out, &read); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	apps, err := s.Scan()
	if err != nil {
		t.Fatal(err)
	}
	if len(read) != len(apps) {
		t.Fatalf("meta.py read %d Applications, the store %d", len(read), len(apps))
	}
	for i, a := range apps {
		py := read[i]
		if py["Malformed"] != (a.Malformed != nil) {
			t.Errorf("%s: meta.py malformed = %v, store %v", rel(root, a.Folder), py["Malformed"], a.Malformed)
		}
		b, err := json.Marshal(a)
		if err != nil {
			t.Fatal(err)
		}
		var goApp map[string]any
		if err := json.Unmarshal(b, &goApp); err != nil {
			t.Fatal(err)
		}
		// Compare what meta.py writes; the store's extra fields (ResumeBuilt, JDHasText
		// and so on) are Go-only, as fixtures/expected.json's contract test also has it.
		for key, value := range py {
			if key == "Malformed" || key == "Error" {
				continue
			}
			if !reflect.DeepEqual(emptyIsNil(value), emptyIsNil(goApp[key])) {
				t.Errorf("%s: %s is %v in meta.py, %v in the store", rel(root, a.Folder), key, value, goApp[key])
			}
		}
	}
}

// emptyIsNil reads an empty JSON list as absent: Python writes [] where Go writes null.
func emptyIsNil(v any) any {
	if list, ok := v.([]any); ok && len(list) == 0 {
		return nil
	}
	return v
}

func rel(root, path string) string {
	r, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return r
}
