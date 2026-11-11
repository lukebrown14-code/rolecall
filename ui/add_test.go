package ui_test

import (
	"bytes"
	"github.com/charmbracelet/x/ansi"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"
	"github.com/lukebrown14-code/rolecall/store"
	"github.com/lukebrown14-code/rolecall/ui"
)

type addHarness struct {
	t      *testing.T
	root   string
	store  *store.Store
	tm     *teatest.TestModel
	mu     sync.Mutex
	edited []string
}

// newAddHarness runs the whole program on a temp copy of the fixture Workspace. The editor is a fake
// runner that records the file it was asked to open and pastes a known JD into it.
func newAddHarness(t *testing.T) *addHarness {
	t.Helper()
	h := &addHarness{t: t, root: t.TempDir()}
	if err := os.CopyFS(h.root, os.DirFS("../fixtures/workspace")); err != nil {
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
	m, err := ui.New(s, func() time.Time { return time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC) }, ui.WithExternalRunner(runner))
	if err != nil {
		t.Fatal(err)
	}
	h.tm = teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 40))
	h.wait("Leads 4")
	return h
}

// wait blocks until every text has appeared in the output. WaitFor consumes the output it reads, so
// texts that must be on screen together are awaited in one call.
func (h *addHarness) wait(texts ...string) {
	h.t.Helper()
	var seen []byte
	teatest.WaitFor(h.t, h.tm.Output(), func(b []byte) bool {
		seen = append(seen, b...)
		for _, text := range texts {
			if !bytes.Contains(seen, []byte(text)) {
				return false
			}
		}
		return true
	}, teatest.WithDuration(3*time.Second))
}

// waitForFile blocks until the fake editor has written the file, then lets the program handle the
// editor's return before the caller goes on.
func (h *addHarness) waitForFile(path, content string) {
	h.t.Helper()
	teatest.WaitFor(h.t, h.tm.Output(), func([]byte) bool {
		b, _ := os.ReadFile(path)
		return string(b) == content
	}, teatest.WithDuration(3*time.Second))
	time.Sleep(100 * time.Millisecond)
}

// waitForStatus blocks until the Application's current Status on disk is the given one. The renderer
// emits only changed cells, so on-screen toasts are not reliable to wait on.
func (h *addHarness) waitForStatus(folder, status string) {
	h.t.Helper()
	teatest.WaitFor(h.t, h.tm.Output(), func([]byte) bool {
		a, err := h.store.Rescan(folder)
		return err == nil && a != nil && a.Malformed == nil && a.Status == status
	}, teatest.WithDuration(3*time.Second))
}

func (h *addHarness) enter() { h.tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter}) }
func (h *addHarness) esc()   { h.tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape}) }

// advance accepts the focused field. The form moves focus as it handles the key, and
// the program handles messages in order, so the next key reaches the next field.
func (h *addHarness) advance() {
	h.t.Helper()
	h.enter()
}

// quit ends the program and returns the final rendered screen.
func (h *addHarness) quit() string {
	h.t.Helper()
	h.tm.Type("q")
	return h.tm.FinalModel(h.t, teatest.WithFinalTimeout(3*time.Second)).View().Content
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
	for _, line := range strings.Split(ansi.Strip(view), "\n") {
		if _, rest, ok := strings.Cut(line, "▌ "); ok {
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
	h.enter()
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
	h.tm.Type("Site Reliability Engineer")
	h.advance()
	h.tm.Type("Acme")
	h.advance()
	h.advance()
	h.advance()
	h.tm.Type("2026-13-45")
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
	h.wait("Title")
	h.tm.Type(title)
	h.advance()
	h.tm.Type(company)
	h.advance()
	h.advance()
	h.tm.Type(url)
	h.advance()
	h.tm.Type(closes)
	h.advance()
	h.enter()
}

func TestAddCreatesLeadOpensJDAndSelectsRow(t *testing.T) {
	h := newAddHarness(t)
	h.tm.Type("G") // park the cursor elsewhere so selecting the new row is observable
	h.fillAdd("Site Reliability Engineer", "Acme", "https://example.test/sre", "2027-03-01")
	h.waitForFile(filepath.Join(h.root, "jobs", "SWE", "acme-site-reliability-engineer-2026-10-01", "jd.md"), "Pasted JD")
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
	wantFolder := filepath.Join(h.root, "jobs", "SWE", "acme-site-reliability-engineer-2026-10-01")
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
	view := h.quit()
	if !sameSnapshot(before, h.snapshot()) {
		t.Fatal("opening the existing Application wrote to the Workspace")
	}
	if len(h.edited) != 0 {
		t.Fatalf("editor opened: %v", h.edited)
	}
	// Engineer #1 is only on the Dismissed table, so selecting it proves the table switch.
	if !strings.Contains(ansi.Strip(view), "Dismissed 2") {
		t.Fatalf("Dismissed table not shown:\n%s", view)
	}
	if row := selectedRow(t, view); !strings.HasPrefix(row, "Engineer #1") {
		t.Fatalf("selected row = %q", row)
	}
}

func TestAddThenMoveThroughStatusesAndUndo(t *testing.T) {
	h := newAddHarness(t)
	h.fillAdd("Quokka Wrangler", "Acme", "https://example.test/quokka", "2027-03-01")
	folder := filepath.Join(h.root, "jobs", "SWE", "acme-quokka-wrangler-2026-10-01")
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
