package ui_test

import (
	"strings"
	"testing"
)

// On a Track the Tracking key line names add and remove beside rename, so they are found
// without opening the help.
func TestSettingsTrackingKeyLineShowsAddAndRemove(t *testing.T) {
	lines := settingsAt(t, 120, 36, "]", "]", "]", "]")
	requireAll(t, footer(lines), "enter rename", "a add", "x remove")
}

// A refused removal names the Applications that block it, the first few and how many
// more, and f shows them: the lists, filtered to the Track.
func TestSettingsRemoveTrackRefusalNamesTheApplications(t *testing.T) {
	d := newDriver(t, fixtureCopy(t, nil), fixtureClock, 120, 36)
	for _, k := range []string{"6", "]", "]", "]", "]", "x"} {
		d.Type(k)
	}
	v := d.plain()
	requireAll(t, v, "Can't remove Track SWE", "· Fictional Labs", "more", "f show them")
	d.Type("f")
	v = d.plain()
	if strings.Contains(v, "Can't remove") || strings.Contains(v, "SETTINGS") {
		t.Fatalf("f left the refusal or Settings open:\n%s", v)
	}
	requireAll(t, v, "[SWE ", "Leads › SWE")
}

// Every Settings edit opens with its value selected, so typing replaces it: a Source's
// form too, as the Search area, Rules, Documents and rename forms already do.
func TestSettingsSourceEditTypingReplacesTheName(t *testing.T) {
	lines := settingsAt(t, 120, 36, "]", "]", "enter", "x")
	v := strings.Join(lines, "\n")
	if strings.Contains(v, "greenhousex") {
		t.Fatalf("typing appended to the Source name instead of replacing it:\n%s", v)
	}
	requireAll(t, v, "Name* x")
}
