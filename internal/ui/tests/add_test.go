package ui_test

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/internal/store"
	"github.com/lukebrown14-code/rolecall/internal/ui"
)

type addHarness struct {
	t      *testing.T
	root   string
	store  *store.Store
	tm     *driver
	mu     sync.Mutex
	edited []string
}

// newAddHarness runs the program on a temp copy of the fixture Workspace. The editor is a fake
// runner that records the file it was asked to open and pastes a known JD into it.
func newAddHarness(t *testing.T) *addHarness {
	t.Helper()
	h := &addHarness{t: t, root: t.TempDir()}
	if err := os.CopyFS(h.root, os.DirFS("../../../fixtures/workspace")); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(h.root)
	if err != nil {
		t.Fatal(err)
	}
	h.store = s
	runner := func(c *exec.Cmd, done tea.ExecCallback) tea.Cmd {
		return func() tea.Msg {
			path := c.Args[len(c.Args)-1]
			h.mu.Lock()
			h.edited = append(h.edited, path)
			h.mu.Unlock()
			return done(os.WriteFile(path, []byte("Pasted JD"), 0600))
		}
	}
	h.tm = newDriver(t, h.root, fixtureClock, 120, 40, ui.WithExternalRunner(runner))
	h.wait("Leads 7")
	return h
}

// wait fails unless every text is on the current screen.
func (h *addHarness) wait(texts ...string) {
	h.t.Helper()
	view := h.tm.plain()
	for _, text := range texts {
		if !strings.Contains(view, text) {
			h.t.Fatalf("screen lacks %q:\n%s", text, view)
		}
	}
}

// waitForFile fails unless the fake editor wrote the file.
func (h *addHarness) waitForFile(path, content string) {
	h.t.Helper()
	if got := readFile(h.t, path); got != content {
		h.t.Fatalf("editor wrote %q, want %q", got, content)
	}
}

// waitForStatus fails unless the Application's current Status on disk is the given one.
func (h *addHarness) waitForStatus(folder, status string) {
	h.t.Helper()
	a, err := h.store.Rescan(folder)
	if err != nil || a == nil || a.Malformed != nil || a.Status != status {
		h.t.Fatalf("status of %s is %v (%v), want %s", folder, a, err, status)
	}
}

func (h *addHarness) enter() { h.tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter}) }
func (h *addHarness) esc()   { h.tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape}) }

// advance accepts the focused field. The form moves focus as it handles the key, and
// the program handles messages in order, so the next key reaches the next field.
func (h *addHarness) advance() {
	h.t.Helper()
	h.enter()
}

// quit returns the final rendered screen.
func (h *addHarness) quit() string {
	h.t.Helper()
	return h.tm.View()
}

// snapshot lists every path and file content under the Workspace.
func (h *addHarness) snapshot() map[string]string {
	h.t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(h.root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		b := ""
		if !d.IsDir() {
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			b = string(data)
		}
		out[p] = b
		return nil
	})
	if err != nil {
		h.t.Fatal(err)
	}
	return out
}

func sameSnapshot(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if w, ok := b[k]; !ok || w != v {
			return false
		}
	}
	return true
}

// selectedRow returns the plain text of the selected row's first line: the one marked
// with the selection bar ▌.
func selectedRow(t *testing.T, view string) string {
	t.Helper()
	for _, line := range strings.Split(chipped(view), "\n") {
		if _, rest, ok := strings.Cut(line, "▌ "); ok {
			// The title follows its Status glyph.
			for _, g := range statusGlyphs {
				rest = strings.TrimPrefix(rest, g+" ")
			}
			return strings.TrimSpace(rest)
		}
	}
	t.Fatalf("no selected row in view:\n%s", view)
	return ""
}

func TestAddFormOpens(t *testing.T) {
	h := newAddHarness(t)
	h.tm.Type("a")
	h.wait("Add a Lead", "Company", "Track", "URL", "Closes", "Fit")
	h.esc()
	h.quit()
}

func TestAddRejectsMissingRequiredFields(t *testing.T) {
	h := newAddHarness(t)
	before := h.snapshot()
	h.tm.Type("a")
	h.wait("Title")
	h.enter() // empty URL moves on to Title
	h.enter() // Title is empty
	h.wait("Title is required")
	h.tm.Type("Site Reliability Engineer")
	h.advance()
	h.enter()
	h.wait("Company is required")
	h.esc()
	h.quit()
	if !sameSnapshot(before, h.snapshot()) {
		t.Fatal("rejected form wrote to the Workspace")
	}
	if len(h.edited) != 0 {
		t.Fatalf("editor opened: %v", h.edited)
	}
}

func TestAddRejectsInvalidClosesDate(t *testing.T) {
	h := newAddHarness(t)
	before := h.snapshot()
	h.tm.Type("a")
	h.wait("Title")
	h.advance() // empty URL moves on to Title
	h.tm.Type("Site Reliability Engineer")
	h.advance()
	h.tm.Type("Acme")
	h.advance()
	h.advance()
	h.tm.Type("2026-13-45")
	h.enter()
	// The shape is right, so the error says the date is not real rather than repeating it.
	h.wait("Closes isn't a real date (YYYY-MM-DD)")
	for range len("2026-13-45") {
		h.tm.Type("backspace")
	}
	h.tm.Type("1 March")
	h.enter()
	h.wait("Closes must be YYYY-MM-DD")
	h.esc()
	h.quit()
	if !sameSnapshot(before, h.snapshot()) {
		t.Fatal("rejected form wrote to the Workspace")
	}
}

// fillAdd completes the form, accepting the first Track and a blank Fit.
func (h *addHarness) fillAdd(title, company, url, closes string) {
	h.tm.Type("a")
	h.wait("URL")
	h.tm.Type("up") // the form opens on Title without a Plugin; the URL is above it
	h.tm.Type(url)
	h.advance()
	h.tm.Type(title)
	h.advance()
	h.tm.Type(company)
	h.advance()
	h.advance()
	h.tm.Type(closes)
	h.advance()
	h.enter()
}

func TestAddCreatesLeadOpensJDAndSelectsRow(t *testing.T) {
	h := newAddHarness(t)
	h.tm.Type("G") // park the cursor elsewhere so selecting the new row is observable
	h.fillAdd("Site Reliability Engineer", "Acme", "https://example.test/sre", "2027-03-01")
	h.waitForFile(filepath.Join(h.root, "jobs", "SWE", "acme-site-reliability-engineer-"+fixtureDay, "jd.md"), "Pasted JD")
	view := h.quit()

	apps, err := h.store.Scan()
	if err != nil {
		t.Fatal(err)
	}
	var created *store.Application
	for i, a := range apps {
		if a.Title == "Site Reliability Engineer" {
			created = &apps[i]
		}
	}
	if created == nil {
		t.Fatal("Application not created")
	}
	if created.Status != "Lead" || len(created.StatusLog) != 1 || created.Company != "Acme" || created.Track != "SWE" || created.URL != "https://example.test/sre" || created.Closes != "2027-03-01" || created.Fit != "" {
		t.Fatalf("unexpected Application: %+v", created)
	}
	wantFolder := filepath.Join(h.root, "jobs", "SWE", "acme-site-reliability-engineer-"+fixtureDay)
	if created.Folder != wantFolder {
		t.Fatalf("folder %s, want %s", created.Folder, wantFolder)
	}
	jd := filepath.Join(wantFolder, "jd.md")
	if len(h.edited) != 1 || h.edited[0] != jd {
		t.Fatalf("editor opened %v, want [%s]", h.edited, jd)
	}
	if b, _ := os.ReadFile(jd); string(b) != "Pasted JD" {
		t.Fatalf("jd.md = %q", b)
	}
	// Soonest close date sorts first on the Leads table, and the new row is the selected one.
	if row := selectedRow(t, view); !strings.HasPrefix(row, "Site Reliability Engineer") {
		t.Fatalf("selected row = %q", row)
	}
}

func TestAddDuplicateURLWarnsAndOpensExisting(t *testing.T) {
	h := newAddHarness(t)
	before := h.snapshot()
	// status-dismissed is the second row of the Dismissed table (docs-dismissed is newer), so opening it
	// proves the table switch and that the cursor lands on the match rather than the first row.
	h.fillAdd("Backend Engineer", "Acme", "https://example.com/status-dismissed", "")
	h.wait("Duplicate URL")
	h.tm.Type("o")
	// o opens the existing one's Detail; esc back from it lands on its row.
	h.wait("‹ esc  Engineer #1")
	h.esc()
	view := h.quit()
	if !sameSnapshot(before, h.snapshot()) {
		t.Fatal("opening the existing Application wrote to the Workspace")
	}
	if len(h.edited) != 0 {
		t.Fatalf("editor opened: %v", h.edited)
	}
	// Engineer #1 is only on the Dismissed table, so selecting it proves the table switch.
	if !strings.Contains(chipped(view), "Leads › Dismissed") {
		t.Fatalf("Dismissed table not shown:\n%s", view)
	}
	if row := selectedRow(t, view); !strings.HasPrefix(row, "Engineer #1") {
		t.Fatalf("selected row = %q", row)
	}
}

func TestAddThenMoveThroughStatusesAndUndo(t *testing.T) {
	h := newAddHarness(t)
	h.fillAdd("Quokka Wrangler", "Acme", "https://example.test/quokka", "2027-03-01")
	folder := filepath.Join(h.root, "jobs", "SWE", "acme-quokka-wrangler-"+fixtureDay)
	h.waitForFile(filepath.Join(folder, "jd.md"), "Pasted JD")

	// Filter to the new Application so each screen shows exactly one row.
	h.tm.Type("/Quokka")
	h.enter()

	h.tm.Type("md")
	h.waitForStatus(folder, "Draft")
	h.tm.Type("2ma")
	h.tm.Type("y") // Docs not Ready. Mark Applied anyway?
	h.waitForStatus(folder, "Applied")
	h.tm.Type("3mi") // the All view keeps the row as it moves between sections
	h.waitForStatus(folder, "Interviewing")
	h.tm.Type("mr")
	h.waitForStatus(folder, "Rejected")
	h.esc() // skip the close reason
	h.tm.Type("u")
	h.waitForStatus(folder, "Interviewing")
	h.quit()

	meta, err := os.ReadFile(filepath.Join(folder, "meta.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	inLog := false
	for _, line := range strings.Split(string(meta), "\n") {
		switch {
		case line == "status_log:":
			inLog = true
		case inLog && strings.HasPrefix(line, "  - {status: "):
			got = append(got, strings.TrimPrefix(strings.SplitN(line, ",", 2)[0], "  - {status: "))
		}
	}
	want := []string{"Lead", "Draft", "Applied", "Interviewing", "Rejected", "Interviewing"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("status_log = %v, want %v\n%s", got, want, meta)
	}
	a, err := h.store.Rescan(folder)
	if err != nil || a.Status != "Interviewing" || a.Malformed != nil {
		t.Fatalf("after undo: %+v %v", a, err)
	}
	if !strings.Contains(string(meta), "title: \"Quokka Wrangler\"\n") {
		t.Fatalf("meta.yml fields changed by moves:\n%s", meta)
	}
}

// esc in the Duplicate URL pop-up goes back to the Add form with everything typed, on the
// URL, so the Owner can fix it; nothing is written.
func TestAddDuplicateURLEscReturnsToFormWithValues(t *testing.T) {
	h := newAddHarness(t)
	before := h.snapshot()
	h.fillAdd("Backend Engineer", "Acme", "https://example.com/status-dismissed", "2027-03-01")
	h.wait("Duplicate URL", "esc back to form")
	h.esc()
	h.wait("Add a Lead", "Backend Engineer", "Acme", "2027-03-01", "▌     URL  https://example.com/status-dismissed")
	if !sameSnapshot(before, h.snapshot()) {
		t.Fatal("going back to the form wrote to the Workspace")
	}
	h.tm.Type("-2")
	h.tm.Type("ctrl+s")
	h.waitForFile(filepath.Join(h.root, "jobs", "SWE", "acme-backend-engineer-"+fixtureDay, "jd.md"), "Pasted JD")
}

// Several saved Applications share a URL: the pop-up names one still open, not a
// Dismissed one, and says how many more there are.
func TestAddDuplicateURLPrefersOpenMatchAndCountsOthers(t *testing.T) {
	h := newAddHarness(t)
	h.fillAdd("Backend Engineer", "Acme", "https://example.test/shared", "")
	h.wait("Duplicate URL", "+5 more with the same URL")
	if strings.Contains(h.tm.plain(), "Dismissed).") {
		t.Fatalf("named a closed match:\n%s", h.tm.plain())
	}
}

// A Title too long for a folder name still saves (the slug is capped). A save the store
// refuses keeps the form open with every value and the reason inside it.
func TestAddLongTitleSavesAndCreateErrorKeepsForm(t *testing.T) {
	h := newAddHarness(t)
	long := strings.TrimSpace(strings.Repeat("Engineer ", 30))
	h.fillAdd(long, "Acme", "", "")
	h.waitForFile(filepath.Join(h.root, "jobs", "SWE", "acme-engineer-engineer-engineer-engineer-engineer-engineer-"+fixtureDay, "jd.md"), "Pasted JD")

	// jobs/SWE cannot take a new folder: Create fails, and the form stays.
	jobs := filepath.Join(h.root, "jobs", "SWE")
	if err := os.Chmod(jobs, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(jobs, 0o700) })
	if f, err := os.Create(filepath.Join(jobs, "probe")); err == nil {
		f.Close()
		t.Skip("directory permissions are not enforced (running as root)")
	}
	h.fillAdd("Site Reliability Engineer", "Acme", "", "")
	h.wait("Add a Lead", "Site Reliability Engineer", "permission denied")
}

// Without a Plugin the Add form opens on Title, so a title typed at once lands there; a
// URL field that is not a web address says so when it is left.
func TestAddOpensOnTitleAndChecksTheURL(t *testing.T) {
	d := newDriver(t, fixtureCopy(t, nil), fixtureClock, 120, 40)
	d.Type("a")
	d.Type("My first")
	requireAll(t, d.plain(), "▌   Title* My first")
	d.Type("up")
	d.Type("abc")
	d.Type("down")
	requireAll(t, d.plain(), "✘ URL must start with https://")
}
