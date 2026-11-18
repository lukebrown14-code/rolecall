package ui_test

import (
	"strings"
	"testing"

	"github.com/lukebrown14-code/rolecall/internal/ui"
)

const emailHint = "Enter an email like name@example.com"

var partialEmails = []string{"ada@", "@x.com", "ada@x"}

// The Contact form refuses a partial address, in the one email wording, and saves a whole one.
func TestContactFormRefusesPartialEmails(t *testing.T) {
	for _, bad := range partialEmails {
		root := freshWorkspace(t)
		d := newDriver(t, root, setupNow, 120, 40, ui.WithFirstRunSetup())
		d.Type("Ada Example")
		d.Type("enter")
		d.Type(bad)
		d.Type("ctrl+s")
		requireAll(t, d.plain(), "✘ Enter an email like", "name@example.com") // the pop-up wraps it
		if strings.Contains(masterProfile(t, root), "Ada") {
			t.Errorf("%q was written to the Profile", bad)
		}
	}
	root := freshWorkspace(t)
	d := newDriver(t, root, setupNow, 120, 40, ui.WithFirstRunSetup())
	d.Type("Ada Example")
	d.Type("enter")
	d.Type("ada@example.com")
	d.Type("ctrl+s")
	requireAll(t, masterProfile(t, root), "| Email | ada@example.com |")
}

// The Application contact refuses the same partial addresses in the same words, and a
// whole address or a link still saves.
func TestApplicationContactRefusesPartialEmails(t *testing.T) {
	fill := func(reach string) (*driver, string) {
		root := stepWorkspace(t, "contact: Jo\n")
		d := newDriver(t, root, fixtureClock, 120, 40)
		editStepper(d)
		for range 7 { // URL … Resume only
			d.Type("enter")
		}
		d.Type("2026-10-05T09:00")
		d.Type("enter")
		d.Type("Phone screen")
		d.Type("enter")
		d.Type("enter") // keep the contact's name
		d.Type("enter") // no role
		d.Type(reach)
		d.Type("ctrl+s")
		return d, metaText(t, root, "stepper")
	}
	for _, bad := range partialEmails {
		d, meta := fill(bad)
		requireAll(t, d.plain(), "✘ "+emailHint)
		if strings.Contains(meta, bad) || strings.Contains(meta, "Phone screen") {
			t.Errorf("%q was saved:\n%s", bad, meta)
		}
	}
	for _, good := range []string{"ada@example.com", "https://example.com/jo"} {
		if _, meta := fill(good); !strings.Contains(meta, good) {
			t.Errorf("%q was not saved:\n%s", good, meta)
		}
	}
}
