package ui_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/teatest/v2"
	"github.com/lukebrown14-code/rolecall/store"
	"github.com/lukebrown14-code/rolecall/ui"
)

// fakeFetch stands in for the captured process runner on the URL fetch path: it
// records the commands it was asked to run and reports configured output.
type fakeFetch struct {
	mu     sync.Mutex
	calls  [][]string
	stdout string
	stderr string
	err    error
	block  chan struct{} // when set, the run waits for a close before finishing
}

func (f *fakeFetch) runner(c *exec.Cmd, done func(ui.CapturedResult) tea.Msg) tea.Cmd {
	return func() tea.Msg {
		f.mu.Lock()
		f.calls = append(f.calls, append([]string(nil), c.Args...))
		f.mu.Unlock()
		if f.block != nil {
			<-f.block
		}
		return done(ui.CapturedResult{Stdout: f.stdout, Stderr: f.stderr, Err: f.err})
	}
}

// fetchHarness drives the TUI on a temp copy of the fixture Workspace with a fake
// captured runner and a recording editor.
type fetchHarness struct {
	t      *testing.T
	root   string
	store  *store.Store
	tm     *driver
	fake   *fakeFetch
	mu     sync.Mutex
	edited []string
}

func newFetchHarness(t *testing.T, opts ...ui.Option) *fetchHarness {
	t.Helper()
	h := &fetchHarness{t: t, root: t.TempDir(), fake: &fakeFetch{}}
	if err := os.CopyFS(h.root, os.DirFS("../fixtures/workspace")); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(h.root)
	if err != nil {
		t.Fatal(err)
	}
	h.store = s
	editor := func(c *exec.Cmd, done tea.ExecCallback) tea.Cmd {
		return func() tea.Msg {
			path := c.Args[len(c.Args)-1]
			h.mu.Lock()
			h.edited = append(h.edited, path)
			h.mu.Unlock()
			return done(os.WriteFile(path, []byte("Pasted JD"), 0600))
		}
	}
	base := []ui.Option{ui.WithExternalRunner(editor), ui.WithCapturedRunner(h.fake.runner)}
	h.tm = newDriver(t, h.root, fixtureClock, 120, 40, append(base, opts...)...)
	h.wait("Leads 4")
	return h
}

func (h *fetchHarness) wait(texts ...string) {
	h.t.Helper()
	view := h.tm.plain()
	for _, text := range texts {
		if !strings.Contains(view, text) {
			h.t.Fatalf("screen lacks %q:\n%s", text, view)
		}
	}
}

func (h *fetchHarness) enter() { h.tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter}) }
func (h *fetchHarness) tab()   { h.tm.Send(tea.KeyPressMsg{Code: tea.KeyTab}) }
func (h *fetchHarness) shiftTab() {
	h.tm.Send(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
}
func (h *fetchHarness) editedNow() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.edited...)
}

func (h *fetchHarness) calls() [][]string {
	h.fake.mu.Lock()
	defer h.fake.mu.Unlock()
	return append([][]string(nil), h.fake.calls...)
}

// fetchJSON is new-job.py --fetch-only's output for a fake Fetched page.
func fetchJSON(t *testing.T, jd string, signals [][]string, role, company, closes string) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"jd":      jd,
		"signals": signals,
		"meta":    map[string]string{"role": role, "company": company, "closes": closes, "description": ""},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// addURLLeads types a URL and starts the fetch in the Add form.
func (h *fetchHarness) openAddWithURL(url string) {
	h.tm.Type("a")
	h.wait("Add a Lead")
	h.tm.Type(url)
	h.enter()
}

func TestAddURLIsFirstFieldAndEditKeepsOrder(t *testing.T) {
	h := newFetchHarness(t, ui.WithPlugin("/plugin"))
	h.tm.Type("a")
	h.wait("Add a Lead")
	view := h.tm.plain()
	if u, ti := strings.Index(view, "URL"), strings.Index(view, "Title"); u < 0 || ti < 0 || u > ti {
		t.Fatalf("Add order: URL at %d, Title at %d:\n%s", u, ti, view)
	}
	h.tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape}) // close Add

	h.tm.Type("e") // Edit the first Lead
	h.wait("Resume only")
	view = h.tm.plain()
	if u, ti := strings.Index(view, "URL"), strings.Index(view, "Title"); u < 0 || ti < 0 || ti > u {
		t.Fatalf("Edit order: URL at %d, Title at %d:\n%s", u, ti, view)
	}
}

func TestAddURLFetchFillsSignalsWritesJDAndSelectsRow(t *testing.T) {
	h := newFetchHarness(t, ui.WithPlugin("/plugin"))
	h.fake.stdout = fetchJSON(t, "Fetched job description text",
		[][]string{{"Security clearance required", "BLOCKER"}, {"No visa sponsorship", "BLOCKER"}},
		"Site Reliability Engineer", "Acme", "2028-02-02")
	h.openAddWithURL("https://example.test/sre")
	h.wait("Site Reliability Engineer", "Acme", "2028-02-02", "Security clearance required", "No visa sponsorship", "⚠")

	for range 5 {
		h.tab() // URL → Title → Company → Track → Closes → Fit
	}
	h.enter() // save on the last field

	folder := filepath.Join(h.root, "jobs", "SWE", "acme-site-reliability-engineer-"+fixtureDay)
	if got := readFile(t, filepath.Join(folder, "jd.md")); got != "Fetched job description text" {
		t.Fatalf("jd.md = %q", got)
	}
	if edited := h.editedNow(); len(edited) != 0 {
		t.Fatalf("editor opened after a successful fetch: %v", edited)
	}
	want := "python3 /plugin/scripts/new-job.py --fetch-only --url https://example.test/sre --data " + h.root
	if calls := h.calls(); len(calls) != 1 || strings.Join(calls[0], " ") != want {
		t.Fatalf("fetch command = %v, want %q", calls, want)
	}
	if row := selectedRow(t, h.tm.plain()); !strings.HasPrefix(row, "Site Reliability Engineer") {
		t.Fatalf("selected row = %q", row)
	}
	// Signals are never written to the Application.
	if b := readFile(t, filepath.Join(folder, "meta.yml")); strings.Contains(b, "clearance") || strings.Contains(b, "sponsorship") {
		t.Fatalf("signals written to meta.yml:\n%s", b)
	}
}

func TestAddURLFetchFillsOnlyEmptyFields(t *testing.T) {
	h := newFetchHarness(t, ui.WithPlugin("/plugin"))
	h.fake.stdout = fetchJSON(t, "Fetched text", nil, "Fetched Title", "Fetched Co", "2028-02-02")
	h.tm.Type("a")
	h.wait("Add a Lead")
	h.tm.Type("https://example.test/x")
	h.tab() // → Title
	h.tm.Type("My Title")
	h.tab() // → Company
	h.tm.Type("My Co")
	h.tab() // → Track
	h.tab() // → Closes
	h.tm.Type("2027-01-01")
	h.shiftTab() // → Track
	h.shiftTab() // → Company
	h.shiftTab() // → Title
	h.shiftTab() // → URL
	h.enter()    // fetch

	h.wait("My Title", "My Co", "2027-01-01")
	view := h.tm.plain()
	if strings.Contains(view, "Fetched Title") || strings.Contains(view, "Fetched Co") || strings.Contains(view, "2028-02-02") {
		t.Fatalf("fetch overwrote typed values:\n%s", ansi.Strip(view))
	}
}

func TestAddURLFetchReFetchesOnEdit(t *testing.T) {
	h := newFetchHarness(t, ui.WithPlugin("/plugin"))
	h.fake.stdout = fetchJSON(t, "First text", [][]string{{"alpha signal", "BLOCKER"}}, "Role A", "Co A", "")
	h.openAddWithURL("https://example.test/a")
	h.wait("alpha signal")

	h.fake.stdout = fetchJSON(t, "Second text", [][]string{{"beta signal", "BLOCKER"}}, "Role A", "Co A", "")
	h.tm.Type("b") // edit the URL (focus is still on it)
	h.enter()      // fetch again
	h.wait("beta signal")
	if calls := h.calls(); len(calls) != 2 {
		t.Fatalf("fetch calls = %d, want 2", len(calls))
	}
	if strings.Contains(h.tm.plain(), "alpha signal") {
		t.Fatal("stale signals survived the re-fetch")
	}
}

func TestAddURLFetchFailureShowsReasonAndOpensEditor(t *testing.T) {
	h := newFetchHarness(t, ui.WithPlugin("/plugin"))
	h.fake.stderr = "example.test serves job ads behind a login, so fetching it will not work.\nmore detail"
	h.fake.err = errors.New("exit status 1")
	h.openAddWithURL("https://example.test/walled")
	h.wait("example.test serves job ads behind")

	// The form goes on by hand; save opens $EDITOR as in v0.1.
	h.tab()
	h.tm.Type("Backend Engineer")
	h.tab()
	h.tm.Type("Acme")
	for range 3 {
		h.tab() // Track → Closes → Fit
	}
	h.enter()
	folder := filepath.Join(h.root, "jobs", "SWE", "acme-backend-engineer-"+fixtureDay)
	if got := readFile(t, filepath.Join(folder, "jd.md")); got != "Pasted JD" {
		t.Fatalf("jd.md = %q", got)
	}
	if edited := h.editedNow(); len(edited) != 1 || edited[0] != filepath.Join(folder, "jd.md") {
		t.Fatalf("editor opened %v, want [%s]", edited, filepath.Join(folder, "jd.md"))
	}
}

func TestAddURLFetchUnparsableOutputShowsGenericReason(t *testing.T) {
	h := newFetchHarness(t, ui.WithPlugin("/plugin"))
	h.fake.stdout = "not json"
	h.openAddWithURL("https://example.test/broken")
	h.wait("Could not fetch the job description")
	if edited := h.editedNow(); len(edited) != 0 {
		t.Fatalf("editor opened during fetch: %v", edited)
	}
}

func TestAddURLFetchSkippedWhenDuplicate(t *testing.T) {
	h := newFetchHarness(t, ui.WithPlugin("/plugin"))
	h.fake.stdout = fetchJSON(t, "Fetched text", nil, "Fetched", "Fetched", "")
	h.openAddWithURL("https://example.com/status-dismissed")
	h.wait("Duplicate URL")
	if calls := h.calls(); len(calls) != 0 {
		t.Fatalf("duplicate URL still fetched: %v", calls)
	}
	h.tm.Type("o") // open the existing Application
	h.wait("Dismissed 2")
	if row := selectedRow(t, h.tm.plain()); !strings.HasPrefix(row, "Engineer #1") {
		t.Fatalf("selected row = %q", row)
	}
}

func TestAddURLWithoutPluginAdvances(t *testing.T) {
	h := newFetchHarness(t) // no plugin
	h.tm.Type("a")
	h.wait("Add a Lead")
	h.tm.Type("https://example.test/x")
	h.enter() // no fetch: moves to Title
	h.enter() // Title is next and empty
	h.wait("Title is required")
	if calls := h.calls(); len(calls) != 0 {
		t.Fatalf("fetched without a plugin: %v", calls)
	}
}

// TestAddURLFetchSpinnerKeepsFormResponsive checks the spinner on the URL field and
// that the form still takes keys while the fetch is in flight.
func TestAddURLFetchSpinnerKeepsFormResponsive(t *testing.T) {
	t.Setenv("ROLECALL_ASCII", "1")
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../fixtures/workspace")); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	fake := &fakeFetch{block: make(chan struct{})}
	fake.stdout = fetchJSON(t, "Fetched text", nil, "Role", "Co", "")
	m, err := ui.New(s, fixtureClock, ui.WithCapturedRunner(fake.runner), ui.WithPlugin("/plugin"))
	if err != nil {
		t.Fatal(err)
	}
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 40))
	tm.Type("a")
	tm.Type("https://example.test/x")
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	var seen []byte
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		seen = append(seen, b...)
		return bytes.Contains(seen, []byte("Fetching"))
	}, teatest.WithDuration(3*time.Second))
	tm.Send(tea.KeyPressMsg{Code: tea.KeyTab}) // the form keeps taking keys while fetching
	close(fake.block)
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		seen = append(seen, b...)
		return bytes.Contains(seen, []byte("Role"))
	}, teatest.WithDuration(3*time.Second))
	if err := tm.Quit(); err != nil {
		t.Fatal(err)
	}
}
