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
	requireAll(t, d.plain(), "✘ Enter an email like", "you@example.com") // the pop-up wraps it
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

// The Contact form asks for GitHub and Work rights too, saves them to the Contact table,
// and at 80 columns no "optional" wraps onto a line of its own under LinkedIn.
func TestContactFormAsksForGitHubAndWorkRightsAt80Columns(t *testing.T) {
	root := freshWorkspace(t)
	d := newDriver(t, root, setupNow, 80, 30, ui.WithFirstRunSetup())
	view := d.plain()
	requireAll(t, view, "GitHub", "Work rights", "the rest", "is optional", "Profile, then c.")
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "│  optional") {
			t.Fatalf("optional wrapped onto its own line:\n%s", view)
		}
	}
	d.Type("Alex Quinn")
	d.Type("enter")
	d.Type("alex@example.test")
	for range 4 { // Phone, Location, LinkedIn, GitHub
		d.Type("enter")
	}
	d.Type("github.com/alexquinn")
	d.Type("enter")
	d.Type("Citizen")
	d.Type("ctrl+s")
	requireAll(t, masterProfile(t, root), "| GitHub | github.com/alexquinn |", "| Work rights | Citizen |")
}

// Opened from the Profile screen, the form does not tell the Owner to go to the Profile.
func TestContactFormFromProfileDoesNotSendYouToProfile(t *testing.T) {
	root := freshWorkspace(t)
	d := newDriver(t, root, setupNow, 100, 30)
	d.Type("5")
	d.Type("c")
	view := d.plain()
	requireAll(t, view, "Your Contact", "Name and Email are required")
	if strings.Contains(view, "5 Profile, then c") {
		t.Fatalf("form opened from Profile points back at Profile:\n%s", view)
	}
}

// A fresh Profile screen has no empty Field/Value table: it says to fill the Contact in,
// and lists the sections a Profile needs with how to write each by hand.
func TestFreshProfileScreenListsSectionsAndHowToFillThem(t *testing.T) {
	d := newDriver(t, freshWorkspace(t), setupNow, 100, 40)
	d.Type("5")
	view := d.plain()
	requireAll(t, view, "No Contact details yet", "## Headlines (approved per track)", "## Experience",
		"## Key Skills keyword pool", "## Summary", "## Facts that are NOT available", "One ### heading per job")
	if strings.Contains(view, "Field") && strings.Contains(view, "│ Value") {
		t.Fatalf("empty Field/Value table shown:\n%s", view)
	}
}

// With the starter file `rolecall init` writes, each section's page carries its how-to,
// and the starter's comments do not count as a Profile begun.
func TestStarterProfileSectionsShowTheirHowTo(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ws")
	if err := store.InitializeStarterWorkspace(root); err != nil {
		t.Fatal(err)
	}
	d := newDriver(t, root, setupNow, 100, 40)
	requireAll(t, d.plain(), "Set up", "5  set up your Profile")
	d.Type("5")
	d.Type("j")
	d.Type("j")
	requireAll(t, d.plain(), "SECTION Experience", "One ### heading per job", "Nothing here yet")
}
