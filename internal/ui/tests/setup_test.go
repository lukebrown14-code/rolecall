package ui_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lukebrown14-code/rolecall/internal/store"
	"github.com/lukebrown14-code/rolecall/internal/ui"
)

// freshWorkspace is what rolecall init leaves: a starter Workspace with no Contact.
func freshWorkspace(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "ws")
	if err := store.InitializeWorkspace(root, nil, nil, store.SearchArea{}); err != nil {
		t.Fatal(err)
	}
	return root
}

func setupNow() time.Time { return time.Date(2026, 11, 15, 9, 0, 0, 0, time.UTC) }

func masterProfile(t *testing.T, root string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, "profile", "master-profile.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestFirstRunOpensOnTheContactFormAndSavesIt(t *testing.T) {
	root := freshWorkspace(t)
	d := newDriver(t, root, setupNow, 120, 40, ui.WithFirstRunSetup())
	requireAll(t, d.plain(), "Welcome", "Name", "Email", "LinkedIn")

	d.Type("Alex Quinn")
	d.Type("enter")
	d.Type("alex@example.test")
	d.Type("ctrl+s")

	got := masterProfile(t, root)
	requireAll(t, got, "| Name | Alex Quinn |", "| Email | alex@example.test |")
	if strings.Contains(d.plain(), "Welcome") {
		t.Fatalf("form still open after save:\n%s", d.plain())
	}
	requireAll(t, d.plain(), "Saved Contact")
}

func TestFirstRunFormRequiresAnEmail(t *testing.T) {
	root := freshWorkspace(t)
	d := newDriver(t, root, setupNow, 120, 40, ui.WithFirstRunSetup())
	d.Type("Alex Quinn")
	d.Type("enter")
	d.Type("not-an-email")
	d.Type("ctrl+s")
	requireAll(t, d.plain(), "enter an email address")
	if strings.Contains(masterProfile(t, root), "Alex") {
		t.Fatal("an invalid form was written")
	}
}

func TestFirstRunFormCanBeSkipped(t *testing.T) {
	root := freshWorkspace(t)
	d := newDriver(t, root, setupNow, 120, 40, ui.WithFirstRunSetup())
	d.Type("esc")
	if strings.Contains(d.plain(), "Welcome") {
		t.Fatalf("esc did not close the form:\n%s", d.plain())
	}
}

func TestFirstRunFormStaysClosedOnceContactIsComplete(t *testing.T) {
	root := freshWorkspace(t)
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetContact(map[string]string{"Name": "Alex", "Email": "alex@example.test"}); err != nil {
		t.Fatal(err)
	}
	d := newDriver(t, root, setupNow, 120, 40, ui.WithFirstRunSetup())
	if strings.Contains(d.plain(), "Welcome") {
		t.Fatalf("form opened for a complete Contact:\n%s", d.plain())
	}
}
