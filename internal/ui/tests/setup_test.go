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

// The first-run form says what it is for and that esc skips it, and its placeholders say
// what goes in a field: none reads as a person's details.
func TestFirstRunFormExplainsItselfAndItsPlaceholdersAreNotData(t *testing.T) {
	d := newDriver(t, freshWorkspace(t), setupNow, 120, 40, ui.WithFirstRunSetup())
	v := strings.Join(strings.Fields(d.plain()), " ")
	requireAll(t, v, "Welcome to rolecall", "rolecall puts these on every Resume", "change them any time", "esc skip",
		"your full name", "your email address")
	for _, gone := range []string{"Alex", "alex.quinn", "esc cancel"} {
		if strings.Contains(v, gone) {
			t.Fatalf("the first-run form shows %q:\n%s", gone, d.plain())
		}
	}
}

// A skipped Contact form opens again from Profile with c, and saving it writes the Contact.
func TestProfileCOpensTheContactForm(t *testing.T) {
	root := freshWorkspace(t)
	d := newDriver(t, root, setupNow, 120, 40, ui.WithFirstRunSetup())
	d.Type("esc")
	d.Type("5")
	requireAll(t, d.plain(), "c fill in Contact")
	d.Type("c")
	requireAll(t, d.plain(), "Your Contact", "Name", "Email", "esc cancel")
	d.Type("Alex Quinn")
	d.Type("enter")
	d.Type("alex@example.test")
	d.Type("ctrl+s")
	requireAll(t, masterProfile(t, root), "| Name | Alex Quinn |", "| Email | alex@example.test |")
	requireAll(t, d.plain(), "Saved Contact", "NAME", "Alex Quinn")
}
