package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/internal/store"
	"github.com/lukebrown14-code/rolecall/internal/ui/watch"
)

// rowsFresh fails when any cached view of m (every list screen's views, under every Track
// option, plus the totals, Deadlines, the Track options and Stats' input) differs from
// the same view derived with no cache. Reading every view also fills the cache, so a
// mutation that fails to invalidate it shows at the next check.
func rowsFresh(t *testing.T, m model, step string) {
	t.Helper()
	if m.rowMemo == nil {
		t.Fatalf("%s: model has no row cache", step)
	}
	fresh := m
	fresh.rowMemo = nil
	same := func(what string, got, want any) {
		t.Helper()
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: cached %s differs from a fresh derivation:\n got %v\nwant %v", step, what, got, want)
		}
	}
	same("rows", folders(m.rows()), folders(fresh.rows()))
	same("Track options", m.filterTracks(), fresh.filterTracks())
	for track := 0; track <= len(fresh.filterTracks())+1; track++ { // +1: a Track since gone
		c, f := m, fresh
		c.track, f.track = track, track
		same(fmt.Sprintf("Stats input (track %d)", track), c.statsApps(), f.statsApps())
		same(fmt.Sprintf("Deadlines total (track %d)", track), c.deadlineRows(false), f.deadlineRows(false))
		for screen, d := range screenDefs {
			if !d.hasTables {
				continue
			}
			c.screen, f.screen = screen, screen
			for _, table := range d.tables {
				what := fmt.Sprintf("%s › %s (track %d)", d.name, table, track)
				got, want := c.rowsOf(screen, table), f.rowsOf(screen, table)
				same(what, folders(got), folders(want))
				same(what+" content", got, want)
				same(what+" total", c.totalCount(screen, table), f.totalCount(screen, table))
			}
		}
	}
}

// folders is rows as their folder names, for a readable failure.
func folders(rows []store.Application) []string {
	out := make([]string, len(rows))
	for i, a := range rows {
		out[i] = filepath.Base(a.Folder) + ":" + a.Status
	}
	return out
}

// press sends keys to m one by one.
func press(t *testing.T, m model, keys ...string) model {
	t.Helper()
	for _, k := range keys {
		var msg tea.KeyPressMsg
		switch k {
		case "right":
			msg = tea.KeyPressMsg{Code: tea.KeyRight}
		case "enter":
			msg = tea.KeyPressMsg{Code: tea.KeyEnter}
		case "esc":
			msg = tea.KeyPressMsg{Code: tea.KeyEscape}
		case "backspace":
			msg = tea.KeyPressMsg{Code: tea.KeyBackspace}
		default:
			msg = tea.KeyPressMsg{Code: rune(k[0]), Text: k}
		}
		m = send(m, msg)
	}
	return m
}

func send(m model, msg tea.Msg) model {
	next, _ := m.Update(msg)
	return next.(model)
}

// rewrite replaces old with new in a file of the Workspace.
func rewrite(t *testing.T, path, old, new string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), old) {
		t.Fatalf("%s does not hold %q", path, old)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(string(b), old, new, 1)), 0o600); err != nil {
		t.Fatal(err)
	}
}

// Every kind of change the Owner, the agent or the clock makes leaves the cached rows
// equal to rows derived from scratch: moves, undo, bulk moves and their undo, edits,
// follow-ups, search, view and Track changes, the Leads sort, live refresh (changed, new
// and removed folders), r after workspace.json changes, resizes, and the day turning.
func TestRowCacheMatchesFreshRowsAfterEveryChange(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../../fixtures/workspace")); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	clock := fixtureNowInternal
	mm, err := New(s, func() time.Time { return clock }, WithTicker(func(time.Duration, func(time.Time) tea.Msg) tea.Cmd { return nil }))
	if err != nil {
		t.Fatal(err)
	}
	m := mm.(model)
	swe := filepath.Join(root, "jobs", "SWE")
	m.width, m.height = 160, 50
	check := func(step string) {
		t.Helper()
		_ = m.View()
		rowsFresh(t, m, step)
	}
	check("start")
	toastHas := func(want string) {
		t.Helper()
		if m.err != nil || !strings.Contains(m.toast, want) {
			t.Fatalf("toast %q (err %v), want it to say %q", m.toast, m.err, want)
		}
	}

	steps := []struct {
		name string
		do   func()
	}{
		{"resize", func() { m = send(m, tea.WindowSizeMsg{Width: 90, Height: 30}) }},
		{"cursor", func() { m = press(t, m, "j", "j") }},
		{"search typed", func() { m = press(t, m, "/", "e") }},
		{"search narrowed", func() { m = press(t, m, "n") }},
		{"search backspace", func() { m = press(t, m, "backspace") }},
		{"search kept", func() { m = press(t, m, "enter") }},
		{"search cleared", func() { m = press(t, m, "esc") }},
		{"view changed", func() { m = press(t, m, "right") }},
		{"view back", func() { m = press(t, m, "s", "s") }},
		{"track changed", func() { m = press(t, m, "t") }},
		{"track changed again", func() { m = press(t, m, "t") }},
		{"track back to all", func() { m.track = 0 }},
		{"Leads sort", func() { m = press(t, m, "1", "o") }},
		{"Leads sort back", func() { m = press(t, m, "o") }},
		{"move (dismiss)", func() { m = press(t, m, "g", "x"); toastHas("Dismissed · ") }},
		{"undo", func() { m = press(t, m, "u"); toastHas("Undone · ") }},
		{"bulk dismiss", func() { m = press(t, m, "V", "x", "y"); toastHas("Dismissed · ") }},
		{"bulk undo", func() { m = press(t, m, "u"); toastHas("moved back") }},
		{"follow-up", func() { m = press(t, m, "3", "right", "l"); toastHas("Followed up · ") }},
		{"follow-up undo", func() { m = press(t, m, "u"); toastHas("Undone · ") }},
		{"edit", func() {
			fern := filepath.Join(swe, "fern")
			rewrite(t, filepath.Join(fern, "meta.yml"), "title: Platform Engineer", "title: Zebra Engineer")
			m = send(m, externalFinished{folder: fern})
		}},
		{"refresh changed folder", func() {
			folder := filepath.Join(swe, "orbit") // the agent moves it on: Applied to Interviewing
			rewrite(t, filepath.Join(folder, "meta.yml"), "{status: Applied, at: 2026-09-25T01:00:00Z}",
				"{status: Applied, at: 2026-09-25T01:00:00Z}\n  - {status: Interviewing, at: 2026-09-30T01:00:00Z}")
			m = send(m, watch.Changed{Folders: []string{folder}})
		}},
		{"refresh new folder", func() {
			if err := os.CopyFS(filepath.Join(swe, "aaa-copy"), os.DirFS(filepath.Join(swe, "orbit"))); err != nil {
				t.Fatal(err)
			}
			m = send(m, watch.Changed{Folders: []string{filepath.Join(swe, "aaa-copy")}})
		}},
		{"refresh removed folder by Track path", func() {
			if err := os.RemoveAll(filepath.Join(swe, "status-offer")); err != nil {
				t.Fatal(err)
			}
			m = send(m, watch.Changed{Folders: []string{swe}})
		}},
		{"r after workspace.json changes", func() {
			ws := filepath.Join(root, "workspace.json")
			rewrite(t, ws, `"tracks":["SWE","Platform"]`, `"tracks":["SWE"]`)
			rewrite(t, ws, `"max_posting_age_days":30`, `"max_posting_age_days":20`)
			m = press(t, m, "1", "r")
		}},
		{"Search rule saved in Settings", func() {
			// As saveRule does: write the rule, then reload the rules alone, no rescan.
			if err := m.setRule(store.RuleMaxPostingAge, "40"); err != nil {
				t.Fatal(err)
			}
			m.loadRules()
		}},
		{"hours pass", func() { clock = clock.Add(5 * time.Hour) }},
		{"day rollover", func() { clock = clock.Add(24 * time.Hour) }},
		{"ten days on", func() { clock = clock.Add(10 * 24 * time.Hour) }},
		{"Pipeline after the days", func() { m = press(t, m, "3") }},
	}
	for _, step := range steps {
		step.do()
		check(step.name)
	}
}

// A keypress that changes nothing the rows are derived from derives no view again, and
// a change derives each view at most once however many widgets read it.
func TestRowCacheDerivesEachViewOncePerState(t *testing.T) {
	m := fixtureModel(t)
	m.screen, m.width, m.height = 2, 160, 50 // Pipeline: the most views and counts
	_ = m.View()
	c := m.rowMemo
	built := c.builds
	m = press(t, m, "j")
	_ = m.View()
	if c.builds != built {
		t.Fatalf("j and a frame derived %d views again, want none", c.builds-built)
	}
	m = press(t, m, "/", "e")
	_ = m.View()
	built = c.builds
	m = press(t, m, "n")
	_ = m.View()
	if derived, views := c.builds-built, len(c.rows); derived != views {
		t.Fatalf("a search key derived %d views for %d distinct views: some more than once", derived, views)
	}
}

// Scrolling Stats reads fit.md once per Workspace state, not once per keypress.
func TestStatsReadsFitOncePerState(t *testing.T) {
	m := fixtureModel(t)
	m.screen, m.width, m.height = 3, 60, 16
	reads := 0
	real := commonGaps
	commonGaps = func(s *store.Store, apps []store.Application) []store.GapCount {
		reads++
		return real(s, apps)
	}
	t.Cleanup(func() { commonGaps = real })
	for range 3 {
		m = press(t, m, "j")
		_ = m.View()
	}
	if reads != 1 {
		t.Fatalf("scrolling Stats read fit.md %d times, want 1", reads)
	}
	m = send(m, watch.Changed{Folders: []string{filepath.Join(m.store.Root(), "jobs", "SWE", "docs-written")}})
	_ = m.View()
	if reads != 2 {
		t.Fatalf("a rescan left Stats on the old fit.md: %d reads, want 2", reads)
	}
}

// A bulk move and its undo each reload the rows once, not once per Application.
func TestBulkMoveAndUndoReloadOnce(t *testing.T) {
	m := fixtureModel(t)
	var leads []store.Application
	for _, a := range m.apps {
		if a.Status == store.StatusLead && a.Malformed == nil {
			leads = append(leads, a)
		}
	}
	if len(leads) < 3 {
		t.Fatalf("fixture has %d Leads, want at least 3", len(leads))
	}
	gen := m.gen
	_, moved := m.moveMany(leads, store.StatusDismissed)
	if m.err != nil || len(moved) != len(leads) {
		t.Fatalf("bulk dismiss moved %d of %d: %v", len(moved), len(leads), m.err)
	}
	if m.gen != gen+1 {
		t.Fatalf("bulk dismiss of %d reloaded %d times, want 1", len(leads), m.gen-gen)
	}
	for _, a := range leads {
		if got, _ := m.application(a.Folder); got.Status != store.StatusDismissed {
			t.Fatalf("%s is %s after the bulk dismiss", a.Folder, got.Status)
		}
	}
	gen = m.gen
	m.undoLast()
	if m.err != nil || m.gen != gen+1 {
		t.Fatalf("bulk undo: err %v, %d reloads, want 1", m.err, m.gen-gen)
	}
	for _, a := range leads {
		if got, _ := m.application(a.Folder); got.Status != store.StatusLead {
			t.Fatalf("%s is %s after the undo", a.Folder, got.Status)
		}
	}
	if want := fmt.Sprintf("Undone · %s moved back", applications(len(leads))); m.toast != want {
		t.Fatalf("undo toast %q, want %q", m.toast, want)
	}
}

// A watcher batch naming every Application and Track folder, with some changed, one added
// and one removed, leaves the rows exactly as a full scan reads them.
func TestRefreshOfEveryFolderMatchesAFullScan(t *testing.T) {
	m := fixtureModel(t)
	swe := filepath.Join(m.store.Root(), "jobs", "SWE")
	paths := []string{swe}
	for _, a := range m.apps {
		paths = append(paths, a.Folder, a.Folder) // named twice, as a batch can
	}
	rewrite(t, filepath.Join(swe, "fern", "meta.yml"), "title: Platform Engineer", "title: Changed")
	if err := os.CopyFS(filepath.Join(swe, "new-one"), os.DirFS(filepath.Join(swe, "orbit"))); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(swe, "status-draft")); err != nil {
		t.Fatal(err)
	}
	paths = append(paths, filepath.Join(swe, "new-one"))
	next := m.refresh(paths)
	want, err := m.store.Scan()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(next.apps, want) {
		t.Fatalf("refresh:\n got %v\nwant %v", folders(next.apps), folders(want))
	}
}

// A Track folder that goes away takes every Application in it off the rows.
func TestRefreshOfARemovedTrackDropsItsApplications(t *testing.T) {
	m := fixtureModel(t)
	swe := filepath.Join(m.store.Root(), "jobs", "SWE")
	if err := os.RemoveAll(swe); err != nil {
		t.Fatal(err)
	}
	next := m.refresh([]string{swe})
	if len(next.apps) != 0 {
		t.Fatalf("%d Applications left after their Track folder went: %v", len(next.apps), folders(next.apps))
	}
}
