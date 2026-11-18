package ui_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/internal/ui"
)

// recordEditor is an external runner that records each command and changes nothing.
func recordEditor(got *[]string) ui.ExternalRunner {
	return func(c *exec.Cmd, done tea.ExecCallback) tea.Cmd {
		return func() tea.Msg {
			*got = append(*got, strings.Join(c.Args, " "))
			return done(nil)
		}
	}
}

// A malformed row is titled by what is wrong, and e on it — or on the error it causes —
// opens its meta.yml in the Editor at the failing line, writing nothing itself.
func TestMalformedRowOpensTheFailingLineInTheEditor(t *testing.T) {
	root := fixtureCopy(t, nil)
	path := filepath.Join(root, "jobs", "SWE", "malformed", "meta.yml")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, keys := range [][]string{{"G", "e"}, {"G", "m", "d", "e"}, {"G", "enter", "e"}} {
		var got []string
		m := driveStart(t, root, fixtureClock, 120, 36, ui.WithEditor("vim"), ui.WithExternalRunner(recordEditor(&got)))
		if view := chipped(m.View().Content); !strings.Contains(view, "⚠ Can't read SWE/malformed") {
			t.Fatalf("malformed row is not titled by its problem:\n%s", view)
		}
		m = driveModel(t, m, keys...)
		if len(got) != 1 || !strings.Contains(got[0], "exec vim +4 \"$1\"") || !strings.HasSuffix(got[0], path) {
			t.Fatalf("%v: editor commands %q, want vim +4 on %s", keys, got, path)
		}
		if view := chipped(m.View().Content); strings.Contains(view, "Can't read SWE/malformed ·") {
			t.Fatalf("%v: the error stayed after e opened the file:\n%s", keys, view)
		}
	}
	if after, _ := os.ReadFile(path); string(after) != string(before) {
		t.Fatal("e wrote to the malformed meta.yml")
	}
}

// movedToast is the toast a move to Interviewing shows: verb, company, arrow, Status.
var movedToast = regexp.MustCompile(`✓ Moved to Interviewing · \S`)

// A move says what happened with an undo hint and lights the moved row until its tick
// fires; the tick is driven by hand, so nothing sleeps.
func TestMoveToastsAndFlashesTheRow(t *testing.T) {
	var fire []func(time.Time) tea.Msg
	var waits []time.Duration
	ticker := func(d time.Duration, fn func(time.Time) tea.Msg) tea.Cmd {
		waits, fire = append(waits, d), append(fire, fn)
		return nil
	}
	m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 36, ui.WithTicker(ticker))
	m = driveModel(t, m, "3", "m", "i") // the All view keeps the row in sight
	lit := m.View().Content
	if plain := chipped(lit); !movedToast.MatchString(plain) || !strings.Contains(plain, "u undo") {
		t.Fatalf("move lacks its toast and undo hint:\n%s", plain)
	}
	if len(fire) != 1 || waits[0] < 300*time.Millisecond || waits[0] > time.Second {
		t.Fatalf("want one flash tick of about 600ms, got %v", waits)
	}
	m = runMsg(t, m, fire[0](time.Now()))
	out := m.View().Content
	if out == lit {
		t.Fatal("the flash did not end on its tick")
	}
	if chipped(out) != chipped(lit) {
		t.Fatal("the flash changed text, not only colour")
	}
	m = driveModel(t, m, "u")
	if plain := chipped(m.View().Content); !strings.Contains(plain, "✓ Undone · back to ") {
		t.Fatalf("undo lacks its toast:\n%s", plain)
	}
}

// The error line keeps its keys whole and cuts the message instead.
func TestErrorLineCutsTheMessageNotTheKeys(t *testing.T) {
	for _, w := range []int{120, 80, 60} {
		view := chipped(runKeysSized(t, fixtureCopy(t, nil), w, 24, "G", "m", "d"))
		// The feedback shares the breadcrumb's line, above the key line.
		lines := strings.Split(strings.TrimRight(view, "\n"), "\n")
		last := lines[len(lines)-2]
		if !strings.HasSuffix(strings.TrimRight(last, " "), "e fix in editor  esc hide") {
			t.Fatalf("%d: error keys cut or missing: %q", w, last)
		}
		if !strings.Contains(last, "✘ Can't read SWE/malformed") {
			t.Fatalf("%d: error message missing: %q", w, last)
		}
	}
}

// ASCII mode swaps the toast's check mark for +, the undo toast's too (↩ is Withdrawn's).
func TestToastGlyphsFoldToASCII(t *testing.T) {
	m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 36, ui.WithASCII(true))
	m = driveModel(t, m, "3", "m", "i")
	if plain := chipped(m.View().Content); !strings.Contains(plain, "+ Moved ") {
		t.Fatalf("ASCII toast lacks +:\n%s", plain)
	}
	m = driveModel(t, m, "u")
	if plain := chipped(m.View().Content); !strings.Contains(plain, "+ Undone | back to ") {
		t.Fatalf("ASCII undo toast lacks +:\n%s", plain)
	}
}

// A move that takes the row out of a filtered view says it is hidden by the filter, and
// u still undoes it.
func TestMoveOutOfFilteredViewSaysHiddenByFilter(t *testing.T) {
	m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 36)
	m = driveModel(t, m, "1", "t", "m", "d")
	plain := chipped(m.View().Content)
	if !strings.Contains(plain, "Moved to Draft (hidden by filter) · ") || !strings.Contains(plain, "u undo") {
		t.Fatalf("filtered move lacks its note:\n%s", plain)
	}
}

// Dismiss confirms like a move: the company, u undo and i go to it (the row left the
// view), with the key line still last.
func TestDismissToastOffersUndo(t *testing.T) {
	m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 36)
	m = driveModel(t, m, "x")
	plain := chipped(m.View().Content)
	if !strings.Contains(plain, "✓ Dismissed ") || !strings.Contains(plain, "u undo  i go to it") {
		t.Fatalf("dismiss toast lacks undo:\n%s", plain)
	}
	lines := strings.Split(strings.TrimRight(plain, "\n"), "\n")
	if !strings.Contains(lines[len(lines)-1], "? help") {
		t.Fatalf("key line is not the last row:\n%s", plain)
	}
}

// Undoing a dismiss says where the Application went back to and which one it is.
func TestUndoDismissToastSaysDismiss(t *testing.T) {
	m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 36)
	m = driveModel(t, m, "x", "u")
	plain := chipped(m.View().Content)
	if !regexp.MustCompile(`✓ Undone · back to Lead · \S+.* \(\S`).MatchString(plain) {
		t.Fatalf("undo of a dismiss reads wrong:\n%s", plain)
	}
}

// A move to Offer that the filter hides keeps the celebration and the hidden-by-filter note.
func TestOfferMoveHiddenByFilterKeepsTheNote(t *testing.T) {
	m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 36)
	m = driveModel(t, m, "3", "s", "t", "m", "o")
	plain := chipped(m.View().Content)
	if !strings.Contains(plain, "OFFER · ") || !strings.Contains(plain, "(hidden by filter)") || !strings.Contains(plain, "u undo") {
		t.Fatalf("offer toast lost its filter note or undo:\n%s", plain)
	}
}

// A message on the footer never changes the breadcrumb's form: with room both show whole,
// otherwise the message takes the line alone (it does not shorten the breadcrumb to
// "Pipeline › All · All · 1 of 9" and back).
func TestFeedbackNeverShortensTheBreadcrumb(t *testing.T) {
	for _, w := range []int{120, 100, 80, 70, 60} {
		bare := breadcrumbLine(runKeysSized(t, fixtureCopy(t, nil), w, 24, "3"))
		withNote := chipped(runKeysSized(t, fixtureCopy(t, nil), w, 24, "3", "x")) // a note toast
		line := breadcrumbLine(withNote)
		if line == "" {
			continue // the message took the line alone
		}
		if !strings.Contains(line, strings.TrimSpace(bare)) {
			t.Errorf("%d columns: breadcrumb with a message is %q, without it %q", w, strings.TrimSpace(line), strings.TrimSpace(bare))
		}
	}
}
