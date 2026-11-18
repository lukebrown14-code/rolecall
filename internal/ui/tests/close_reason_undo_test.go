package ui_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Reopening a Rejected Application drops its close_reason (SCHEMA: it belongs to a closed
// Application), and u puts the Application back exactly as it was, reason included.
func TestUndoOfAReopenRestoresTheCloseReason(t *testing.T) {
	root := rejectedStepper(t, "close_reason: \"salary\"\n")
	d := newDriver(t, root, fixtureClock, 120, 40)
	d.Type("3")
	d.Type("/stepper")
	d.Type("enter")
	d.Type("mi")
	if got := metaText(t, root, "stepper"); strings.Contains(got, "close_reason") || !strings.Contains(got, "status: Interviewing") {
		t.Fatalf("the reopen kept the close_reason or did not move:\n%s", got)
	}
	d.Type("u")
	got := metaText(t, root, "stepper")
	if strings.Count(got, "close_reason") != 1 || !strings.Contains(got, "close_reason: \"salary\"\n") {
		t.Fatalf("undo did not restore the close_reason exactly once:\n%s", got)
	}
	if !strings.HasSuffix(strings.TrimSpace(got), "{status: Rejected, at: 2026-10-01T00:00:00Z}") {
		t.Fatalf("undo did not put the Application back to Rejected:\n%s", got)
	}
}

// A bulk reopen undone with one u gives each Application back its own reason.
func TestBulkUndoOfAReopenRestoresEachCloseReason(t *testing.T) {
	reasons := map[string]string{"stepper": "salary", "stepper2": "role-closed"}
	root := fixtureCopy(t, func(root string) {
		for name, reason := range reasons {
			dir := filepath.Join(root, "jobs", "SWE", name)
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			meta := "schema: 1\ntitle: Stepper\ncompany: " + name + "\ntrack: SWE\nclose_reason: " + reason + "\n" +
				"status_log:\n  - {status: Applied, at: 2026-09-28}\n  - {status: Withdrawn, at: 2026-09-29}\n"
			if err := os.WriteFile(filepath.Join(dir, "meta.yml"), []byte(meta), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	})
	d := newDriver(t, root, fixtureClock, 120, 40)
	d.Type("3")
	d.Type("/stepper")
	d.Type("enter")
	d.Type("*")
	d.Type("j")
	d.Type("*")
	d.Type("mi")
	for name := range reasons {
		if got := metaText(t, root, name); strings.Contains(got, "close_reason") || !strings.Contains(got, "status: Interviewing") {
			t.Fatalf("%s: the bulk reopen kept the reason or did not move:\n%s", name, got)
		}
	}
	d.Type("u")
	for name, reason := range reasons {
		if got := metaText(t, root, name); !strings.Contains(got, "close_reason: \""+reason+"\"\n") || !strings.Contains(got, "{status: Withdrawn, at: 2026-10-01T00:00:00Z}") {
			t.Fatalf("%s: bulk undo did not restore its own reason %q:\n%s", name, reason, got)
		}
	}
}
