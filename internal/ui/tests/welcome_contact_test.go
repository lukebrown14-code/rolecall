package ui_test

import (
	"strings"
	"testing"

	"github.com/lukebrown14-code/rolecall/internal/ui"
)

// Saving the Contact on the first-run form ticks it off on the welcome at once, and the
// Profile step stops asking for it.
func TestWelcomeTicksOffASavedContact(t *testing.T) {
	d := newDriver(t, freshWorkspace(t), setupNow, 120, 40, ui.WithFirstRunSetup())
	requireAll(t, d.plain(), "Welcome")
	d.Type("Alex Quinn")
	d.Type("enter")
	d.Type("alex@example.test")
	d.Type("ctrl+s")
	v := d.plain()
	requireAll(t, v, "Saved Contact", "✓  Contact saved", "5  write the rest of your Profile in your Editor")
	if strings.Contains(v, "fill in your Contact") {
		t.Errorf("the welcome still asks for the saved Contact:\n%s", v)
	}
}

// Once the Contact is complete, c stays on the Profile key line as "c edit Contact", and
// the guide under it starts with what it is rather than "Then…".
func TestProfileKeyLineOffersEditContact(t *testing.T) {
	d := newDriver(t, contactOnly(t), setupNow, 120, 40)
	d.Type("5")
	v := d.plain()
	if keys := lastLineOf(v); !strings.Contains(keys, "c edit Contact") {
		t.Errorf("the Profile key line lacks c edit Contact: %q", keys)
	}
	requireAll(t, v, "The rest of your Profile: add these sections")
	if strings.Contains(v, "Then ") {
		t.Errorf("the Profile guide starts with Then:\n%s", v)
	}
}
