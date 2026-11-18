package store_test

import (
	"os"
	"path/filepath"
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

func rel(root, path string) string {
	r, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return r
}
