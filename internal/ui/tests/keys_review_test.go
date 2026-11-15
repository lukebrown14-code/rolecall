package ui_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// Tests for the keys review: every key a screen advertises works there, and the keys say
// what they do.

func view(t *testing.T, keys ...string) string {
	t.Helper()
	return chipped(runKeysSized(t, fixtureCopy(t, nil), 120, 36, keys...))
}

// Every screen digit the detail key line offers ("1-6 screen") leaves the detail.
func TestEveryScreenDigitWorksFromTheDetail(t *testing.T) {
	for d, tab := range map[string]string{"1": "[1 Leads", "2": "[2 Drafts", "4": "[4 Stats", "5": "[5 Profile", "6": "[6 Settings"} {
		if v := view(t, "3", "enter", d); !strings.Contains(v, tab) || strings.Contains(v, "‹ esc") {
			t.Errorf("%s from the detail did not switch screen:\n%s", d, v)
		}
	}
}

// tab and shift+tab cycle the detail's tabs as ←→ do: the detail has one pane.
func TestTabCyclesDetailTabs(t *testing.T) {
	m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 36)
	m = driveModel(t, m, "3", "/docs-written", "enter", "enter")
	first := plain(m)
	m = driveModel(t, m, "tab")
	if plain(m) == first || !strings.Contains(plain(m), "[Fit]") {
		t.Fatalf("tab did not move to the next tab:\n%s", plain(m))
	}
	m = runMsg(t, m, tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	if !strings.Contains(plain(m), "[JD]") {
		t.Fatalf("shift+tab did not go back:\n%s", plain(m))
	}
}

// space never paged the detail (it opens the menu); the help names pgup/pgdn instead.
func TestDetailHelpNamesPaging(t *testing.T) {
	if v := view(t, "3", "enter", "?"); !strings.Contains(v, "[pgup pgdn] page") {
		t.Fatalf("help lacks pgup pgdn:\n%s", v)
	}
}

// The menu's shown keys fire from inside it, and space closes it.
func TestMenuAcceleratorsFire(t *testing.T) {
	if v := view(t, " ", "3"); !strings.Contains(v, "[3 Applied") || strings.Contains(v, "Menu") {
		t.Fatalf("3 in the menu did not go to Applied:\n%s", v)
	}
	if v := view(t, " ", " "); strings.Contains(v, "Menu") {
		t.Fatalf("space did not close the menu:\n%s", v)
	}
	if v := view(t, " "); !strings.Contains(v, "Rescan") || !strings.Contains(v, "Switch view") || strings.Contains(v, "status filter") {
		t.Fatalf("menu wording:\n%s", v)
	}
}

// The palette reaches every screen, as the menu does.
func TestPaletteSwitchesScreens(t *testing.T) {
	v := view(t, ":", "drafts")
	if !strings.Contains(v, "Drafts screen") {
		t.Fatalf("palette has no Drafts:\n%s", v)
	}
	if v := view(t, ":", "stats", "enter"); !strings.Contains(v, "[4 Stats") {
		t.Fatalf("palette did not open Stats:\n%s", v)
	}
}

// A closing key on the help also runs underneath: "? m" opens Move.
func TestHelpClosingKeyRunsBeneath(t *testing.T) {
	if v := view(t, "?", "m"); !strings.Contains(v, "Move") {
		t.Fatalf("? m did not open Move:\n%s", v)
	}
}

// m on a malformed row is refused at the key, with the parse error and e fix in editor,
// rather than a Move pop-up for a record the store cannot read; u stays listed there.
func TestMoveRefusedAtTheKeyOnAMalformedRow(t *testing.T) {
	v := view(t, "1", "G", "m")
	if strings.Contains(v, "╭─ Move") {
		t.Fatalf("Move opened on a malformed row:\n%s", v)
	}
	if !strings.Contains(v, "Can't read SWE/malformed") || !strings.Contains(v, "e fix in editor") {
		t.Fatalf("no parse error:\n%s", v)
	}
	// Moving the row above off the view lands the cursor on the malformed row.
	if line := keyLineOf(runKeysSized(t, fixtureCopy(t, nil), 120, 36, "1", "j", "j", "m", "d", "esc")); !hasKey(line, "u") {
		t.Fatalf("u undo gone on the malformed row: %q", line)
	}
}

func TestUndoWithNothingToUndoSaysSo(t *testing.T) {
	if v := view(t, "u"); !strings.Contains(v, "Nothing to undo") {
		t.Fatalf("u was silent:\n%s", v)
	}
}

// esc on the toast leaves the cursor where it was: only a search resets it.
func TestEscOnAToastKeepsTheCursor(t *testing.T) {
	v := view(t, "1", "j", "j", "m", "d", "esc")
	if !strings.Contains(v, "3/3") {
		t.Fatalf("esc moved the cursor:\n%s", v)
	}
}

// An active search always offers its way out, and esc on Profile says what it cleared.
func TestSearchAlwaysShowsHowToClearIt(t *testing.T) {
	for _, w := range []int{120, 80, 60} {
		line := keyLineOf(runKeysSized(t, fixtureCopy(t, nil), w, 24, "/fe", "enter", "3"))
		if !strings.Contains(line, "esc clear search") {
			t.Errorf("%d: key line lacks esc clear search: %q", w, line)
		}
	}
	if v := view(t, "/fe", "enter", "5", "esc"); !strings.Contains(v, `Cleared search "/fe"`) {
		t.Fatalf("esc on Profile cleared the search silently:\n%s", v)
	}
}

// A search with no match here says where the matches are.
func TestEmptySearchPointsElsewhere(t *testing.T) {
	v := view(t, "/orbit", "enter")
	if !strings.Contains(v, `No match for "orbit" in Leads · 1 in Applied`) || !strings.Contains(v, "3 go to Applied") {
		t.Fatalf("empty search does not say where:\n%s", v)
	}
}

// r is always rescan; the Follow-up draft is F, offered only with a Harness.
func TestRIsAlwaysRescan(t *testing.T) {
	root := heatWorkspace(t)
	m := driveStart(t, root, fixtureClock, 120, 40)
	m = driveModel(t, m, "3", "/Highco", "enter")
	if v := plain(m); strings.Contains(v, "F draft") || strings.Contains(v, "No Harness set") {
		t.Fatalf("follow-up draft offered without a Harness:\n%s", v)
	}
	if err := os.WriteFile(filepath.Join(root, "jobs", "SWE", "heat-high", "notes.md"), []byte("new note"), 0o644); err != nil {
		t.Fatal(err)
	}
	m = driveModel(t, m, "r")
	if v := plain(m); strings.Contains(v, "No Harness") {
		t.Fatalf("r did not rescan on a Follow-up due row:\n%s", v)
	}
	m = driveModel(t, m, "F")
	if v := plain(m); !strings.Contains(v, "No Harness set · 6 Settings › This machine › Harness") {
		t.Fatalf("F without a Harness does not point at Settings:\n%s", v)
	}
}

// Without a Harness, NEXT on a Lead with a JD but no resume offers a key that works.
func TestWriteStepWithoutAHarnessOffersAKey(t *testing.T) {
	root := fixtureCopy(t, func(root string) {
		if err := os.WriteFile(filepath.Join(root, "jobs", "SWE", "fern", "jd.md"), []byte("JD"), 0o644); err != nil {
			t.Fatal(err)
		}
	})
	v := chipped(runKeysSized(t, root, 120, 40, "/fern", "enter"))
	if !strings.Contains(lineWith(v, "NEXT"), "write the resume") || !strings.Contains(lineWith(v, "NEXT"), "enter open") {
		t.Fatalf("NEXT offers no key:\n%s", v)
	}
}

// Moving to Offer from Applied › All keeps the All view and follows the row.
func TestOfferFromAllKeepsTheView(t *testing.T) {
	v := view(t, "3", "m", "i", "m", "o")
	if !strings.Contains(v, "Applied › All") {
		t.Fatalf("the view changed:\n%s", v)
	}
}

// The Docs-not-Ready gate names what is missing.
func TestDocsGateNamesWhatIsMissing(t *testing.T) {
	if v := view(t, "2", "m", "a"); !strings.Contains(strings.Join(strings.Fields(v), " "), "Missing: JD, Resume, Cover letter, PDF") {
		t.Fatalf("the gate does not name the documents:\n%s", v)
	}
}

// An undone move leaves no stop on the timeline and does not reset the age in Status.
func TestTimelineIgnoresAnUndoneMove(t *testing.T) {
	after := view(t, "2", "m", "a", "y", "u")
	if !strings.Contains(after, "Undid move") {
		t.Fatalf("no undo:\n%s", after)
	}
	if strings.Contains(lineWith(after, "● Applied"), "1 Oct") || !strings.Contains(lineWith(after, "● Applied"), "next") {
		t.Fatalf("phantom Applied on the timeline:\n%s", after)
	}
	if !strings.Contains(lineWith(after, "✎ docs-none"), "Draft ~7d") {
		t.Fatalf("the age in Status reset after an undo:\n%s", after)
	}
}

// ctrl+c in the Add form with typed text asks before dropping it.
func TestCtrlCInTheAddFormAsks(t *testing.T) {
	m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 36)
	m = driveModel(t, m, "a", "Typed")
	next, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd != nil {
		if _, quit := cmd().(tea.QuitMsg); quit {
			t.Fatal("ctrl+c dropped the typed form")
		}
	}
	if v := plain(next); !strings.Contains(v, "Discard") {
		t.Fatalf("no Discard? question:\n%s", v)
	}
}
