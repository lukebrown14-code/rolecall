// Package ui implements the Owner's terminal Workspace browser.
package ui

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/internal/config"
	"github.com/lukebrown14-code/rolecall/internal/store"
	"github.com/lukebrown14-code/rolecall/internal/ui/watch"
)

// allView is the Applied screen's first view: every Application past Draft, one
// section per table in appliedGroups order.
const allView = "All"

var appliedGroups = []string{tableApplied, tableInterviewing, tableOffer, tableClosed}

// defaultFollowUpDays applies when the Owner configures none.
const defaultFollowUpDays = 14

type model struct {
	store                 *store.Store
	apps                  []store.Application
	screen, table, cursor int
	tracks                []string
	track                 int
	text                  string
	overlay               mode // the pop-up, prompt or search input that owns the keyboard
	selectAfter           string
	undo                  []undoMove
	toast                 string    // feedback line: what just happened (cleared on the next key)
	movedTo               string    // destination Status of the move the toast reports, else ""
	toastKind             toastKind // how the feedback line draws the toast
	toastHint             string    // key hint shown with it, e.g. "u undo"
	flash                 string    // folder of the row just moved, lit until flashID's tick
	flashID               int
	tick                  Ticker
	err                   error
	width                 int
	gen                   int // bumps when Workspace content is reloaded; invalidates the render cache
	followUpDays          int
	statsTop              int              // the first Stats line shown when the report is taller than the body
	stats                 *statsCache      // the Stats report for the current interaction; see statsReport
	configHome            string           // where config.toml lives; empty means machine settings cannot be saved
	configErr             error            // config.toml's last load error (*config.FileError when it does not parse)
	configWarnings        []config.Warning // what the last config.toml load worked around (config.Config.Warnings)
	ascii                 bool             // ascii in config.toml; ROLECALL_ASCII=1 also forces it
	inherit               bool             // inherit_terminal in config.toml: ANSI-16 colours, no painted background
	theme                 theme            // dark or light roles; see DetectTheme
	now                   func() time.Time
	editor                string
	back                  *settingsBack // where Esc returns from Settings opened with a comma
	editorSource          string        // where the effective Editor came from, for Settings
	sources               SettingSources
	external              ExternalRunner
	detail                *detailState
	lastTab               detailTab // the detail tab last chosen; the next detail opens on it where it has one
	height                int
	watchWindow           time.Duration
	watch                 *watch.Watcher
	downloads             string
	md                    *mdCache
	plugin                string
	harness               string
	captured              CapturedRunner
	capturedRun           bool // a build or fetch is in flight; only one captured run at a time
	building              *buildState
	spinner               spinner.Model
	profile               *profileState
	profileStarted        bool // the Profile has more than Contact, so the welcome drops its Profile step
	settings              *settingsState
	// rules are workspace.json's Search rules, re-read with every scan, for the age warning.
	rules       store.SearchRules
	visa        store.Visa         // workspace.json visa, for the sponsorship and contract badges
	payFX       map[string]float64 // workspace.json pay_fx, for the annual pay estimate
	leadsNewest bool               // Leads sorted newest first instead of by deadline; o toggles
	compactRows bool               // one-line list rows at wide sizes; z toggles, compact_rows in config.toml
	writing     map[string]int     // documents written just now (watch.DocumentKey) and their mark's generation
	writingGen  int
	legacy      []string        // dashboard-era files in the Workspace (store.LegacyFiles), warned about in the footer
	clipboard   ClipboardWriter // nil uses the system clipboard program
}

// New loads a Workspace. The clock is injected for subsequent date-sensitive views.
func New(s *store.Store, now func() time.Time, options ...Option) (tea.Model, error) {
	if now == nil {
		now = time.Now
	}
	th, _ := themeFromEnv()
	m := model{store: s, now: now, followUpDays: defaultFollowUpDays, md: &mdCache{}, stats: &statsCache{}, theme: th,
		spinner: spinner.New(spinner.WithSpinner(spinner.MiniDot))}
	for _, option := range options {
		option(&m)
	}
	m.spinner.Style = m.theme.Base().SpinnerStyle()
	if m.watchWindow > 0 {
		// Watching starts before the first scan so no change slips between the two.
		w, e := watch.Start(filepath.Join(s.Root(), "jobs"), m.watchWindow)
		if e != nil {
			m.err = fmt.Errorf("live refresh unavailable: %w", e)
		}
		m.watch = w
	}
	a, e := s.Scan()
	if e == nil {
		m.apps = a
		m.tracks, e = s.Tracks()
		m.legacy = s.LegacyFiles()
		m.loadRules()
		if p, err := s.Profile(); err == nil {
			m.profileStarted = profileBeyondContact(p)
		}
		if e != nil && (invalidWorkspaceFile(s) || badTrackList(e)) {
			// An invalid workspace.json does not stop startup: the app opens with no Track
			// filter, and Settings shows the path and the parse error with e to fix it.
			// Only a schema this build cannot use still stops it.
			m.tracks, m.err, e = nil, fmt.Errorf("%w · 6 Settings shows why, e to fix it", e), nil
		}
	}
	if e != nil {
		m.watch.Close()
		return nil, e
	}
	return m, nil
}

// invalidWorkspaceFile reports a workspace.json that is not valid JSON or has a key of the
// wrong shape, as opposed to one from a schema this build cannot read.
// badTrackList reports a workspace.json that parses but lists a bad Track: like an
// invalid file, it opens with no Track filter rather than stopping startup.
func badTrackList(err error) bool {
	var tle *store.TrackListError
	return errors.As(err, &tle)
}

func invalidWorkspaceFile(s *store.Store) bool {
	ws, err := s.Settings()
	return err == nil && ws.Err != nil && ws.Schema == 1
}

func (m model) Init() tea.Cmd {
	if m.watch == nil {
		return nil
	}
	return m.watch.Next()
}

// quit stops the watch.Watcher, then quits.
func (m model) quit() tea.Cmd {
	return func() tea.Msg {
		m.watch.Close()
		return tea.QuitMsg{}
	}
}

// rows is the active view in display order: the cursor indexes into it. A screen
// without Application tables has no rows.
func (m model) rows() []store.Application {
	if !m.hasTables() {
		return nil
	}
	return m.rowsOf(m.screen, m.views()[m.table])
}

// rowsOf lists one view of a screen. All groups the Applied screen's tables in
// appliedGroups order; Leads and Draft put expired close dates last.
func (m model) rowsOf(screen int, table string) []store.Application {
	if table == allView {
		var out []store.Application
		for _, g := range appliedGroups {
			out = append(out, m.rowsOf(screen, g)...)
		}
		return out
	}
	if table == tableDeadlines {
		return m.deadlineRows(true)
	}
	var out []store.Application
	for _, a := range m.apps {
		s, t := appLocation(a)
		if s == screenDefs[screen].name && t == table && m.matches(a) {
			out = append(out, a)
		}
	}
	if table == tableLeads && m.leadsNewest {
		sortNewest(out)
		return out
	}
	m.order().sort(out, table)
	if beforeApplied(table) {
		var live, old []store.Application
		for _, a := range out {
			if m.expired(a) {
				old = append(old, a)
			} else {
				live = append(live, a)
			}
		}
		out = append(live, old...)
	}
	return out
}

// deadlineRows is the Deadlines view: every Lead and Draft with a deadline still to
// come (a close date, or a Lead's ad within expiry_warning_days of max_posting_age_days),
// soonest first; then those whose close date has passed; then those whose ad left the
// close date unclear. filter applies the Track and text filters.
func (m model) deadlineRows(filter bool) []store.Application {
	groups := map[deadlineGroup][]store.Application{}
	for _, a := range m.apps {
		if a.Malformed != nil || !beforeApplied(tableOf(a)) || (filter && !m.matches(a)) {
			continue
		}
		if g, ok := m.deadlineGroupOf(a); ok {
			groups[g] = append(groups[g], a)
		}
	}
	var out []store.Application
	for _, g := range []deadlineGroup{deadlineLive, deadlinePast, deadlineUnclear} {
		m.order().sort(groups[g], tableDraft)
		out = append(out, groups[g]...)
	}
	return out
}

// deadlineGroup is a Deadlines section.
type deadlineGroup int

const (
	deadlineLive    deadlineGroup = iota // a deadline still to come
	deadlinePast                         // the close date has passed: Expired
	deadlineUnclear                      // the ad left the close date unclear
)

// deadlineGroupOf is the Deadlines section a Lead or Draft belongs in, and whether it
// belongs in Deadlines at all. A Lead nearing max_posting_age_days is in it as soon as
// its row warns "expires in N days"; one already past the age limit with no close date
// is not, as it has no date left to meet.
func (m model) deadlineGroupOf(a store.Application) (deadlineGroup, bool) {
	_, hasCloses := parseDay(a.Closes)
	expiring := m.postingExpiring(a)
	switch {
	case m.expired(a):
		return deadlinePast, hasCloses
	case hasCloses || expiring:
		return deadlineLive, true
	case a.ClosesUnclear:
		return deadlineUnclear, true
	}
	return 0, false
}

// postingExpiring reports a Lead whose ad is within expiry_warning_days of
// max_posting_age_days, today included: the rows that read "expires in N days".
func (m model) postingExpiring(a store.Application) bool {
	t, ok := postingExpiry(a, m.rules)
	if !ok {
		return false
	}
	left := int(t.Sub(day(m.now())).Hours() / hoursPerDay)
	return left >= 0 && left <= m.rules.ExpiryWarningDays
}

// beforeApplied reports whether a table holds Applications not yet sent: Leads and Draft.
func beforeApplied(table string) bool { return table == tableLeads || table == tableDraft }

func (m model) count(screen int, table string) int { return len(m.rowsOf(screen, table)) }

// totalCount is the Applications in a view ignoring the track and text filters, so the
// header and tab counts stay totals; the scope line shows the filtered number beside them.
func (m model) totalCount(screen int, table string) int {
	if table == allView {
		n := 0
		for _, g := range appliedGroups {
			n += m.totalCount(screen, g)
		}
		return n
	}
	if table == tableDeadlines {
		return len(m.deadlineRows(false))
	}
	n := 0
	for _, a := range m.apps {
		s, t := appLocation(a)
		if s == screenDefs[screen].name && t == table {
			n++
		}
	}
	return n
}
func (m model) matches(a store.Application) bool {
	track, on := m.selectedTrack()
	return (!on || a.Track == track) && strings.Contains(strings.ToLower(m.searchText(a)), strings.ToLower(m.text))
}
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// The Stats report is built at most once per interaction; this drops the previous
	// interaction's copy so the next access to Stats rebuilds it.
	m.resetStats()
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		m.width, m.height = size.Width, size.Height
	}
	if next, cmd, ok := m.mouse(msg); ok {
		return next, cmd
	}
	if batch, ok := msg.(watch.Changed); ok {
		m = m.refresh(batch.Folders)
		return m, tea.Batch(m.watch.Next(), m.markWriting(batch.Written))
	}
	if done, ok := msg.(writingDone); ok {
		m.endWriting(done)
		return m, nil
	}
	if copied, ok := msg.(linkCopied); ok {
		return m.linkCopied(copied), nil
	}
	if done, ok := msg.(externalFinished); ok {
		a, err := m.store.Rescan(done.folder)
		m.err = done.err
		if done.err == nil && done.toast != "" {
			m.toast = done.toast
		}
		if err != nil {
			m.err = err
		} else if a != nil {
			for i := range m.apps {
				if m.apps[i].Folder == done.folder {
					m.apps[i] = *a
					break
				}
			}
		}
		m.dropBodies()
		if done.folder == m.selectAfter {
			m.selectFolder(done.folder)
			m.selectAfter = ""
		}
		// A Profile edit suspends from screen 5 and finishes with the Workspace root as
		// its folder; profile/ is not watched, so reread it here. An Application edit
		// still rescans its own folder above.
		if m.screen == profileScreen || done.folder == m.store.Root() {
			m = m.loadProfile()
		}
		if m.screen == settingsScreen {
			m.rescanAll() // workspace.json holds the Tracks the header filters by
			m = m.loadSettings()
		}
	}
	if tick, ok := msg.(spinner.TickMsg); ok {
		if m.building == nil && !m.fetching() && len(m.writing) == 0 {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(tick)
		return m, cmd
	}
	if done, ok := msg.(fetchFinished); ok {
		// The form may have been closed while the fetch was in flight: the run is still
		// over, so clear the guard and drop the result rather than leaking it.
		m.capturedRun = false
		fm, ok := m.overlay.(*formMode)
		if !ok {
			return m, nil
		}
		fm.finishFetch(&m, done)
		return m, nil
	}
	if done, ok := msg.(sourceChecked); ok {
		return m.finishSourceCheck(done), nil
	}
	if done, ok := msg.(buildFinished); ok {
		m.capturedRun = false
		return m.finishBuild(done)
	}
	if k, ok := msg.(tea.KeyPressMsg); ok {
		// The feedback line: a success clears on the next key; an error stays until esc.
		m.toast, m.toastHint, m.movedTo, m.toastKind = "", "", "", toastPlain
		if k.String() == "ctrl+c" {
			if next, held := m.quitHeld(); held {
				return next, nil // typed Settings text is never dropped without asking
			}
			return m, m.quit()
		}
	}
	// An open overlay owns the keyboard: it handles the message and everything else waits.
	if m.overlay != nil {
		next, cmd := m.overlay.Update(&m, msg)
		m.overlay = next
		m.clampCursor()
		return m, cmd
	}
	if m.fieldOpen() {
		// An open Settings field owns the keyboard, so typing digits or "," is text.
		switch msg := msg.(type) {
		case tea.KeyPressMsg:
			return m.editKey(msg), nil
		case tea.PasteMsg:
			return m.editPaste(msg.Content), nil
		}
	}
	if done, ok := msg.(flashDone); ok {
		if done.id == m.flashID {
			m.flash = ""
		}
		return m, nil
	}
	var flash tea.Cmd // a move lights its row until this fires
	if k, ok := msg.(tea.KeyPressMsg); ok {
		if k.String() == "esc" && m.err != nil {
			m.err = nil
			return m, nil
		}
		if pe, ok := parseError(m.err); ok && k.String() == "e" {
			// The error on show names a meta.yml the store cannot read: e opens it there.
			m.err = nil
			return m, m.editAt(pe)
		}
		if k.String() == " " || k.String() == spaceKey {
			m.overlay = m.openMenu()
			return m, nil
		}
		if k.String() == ":" || k.String() == "ctrl+k" {
			// The command palette lists what the keys underneath it would do.
			m.overlay = m.openPalette()
			return m, nil
		}
		if k.String() == "," && m.screen != settingsScreen {
			// Settings opens from anywhere, over the detail view and Stats too.
			return m.openSettings(), nil
		}
		if m.connectsHarness(k.String()) {
			// h is how the no-Harness error is fixed, from any screen: Settings › This
			// machine on the Harness row.
			m.err = nil
			return m.openSettingsAt(labelHarness), nil
		}
		if m.detail != nil {
			if next, cmd, handled := m.detailKey(k); handled {
				return next.syncDetail(), cmd
			}
		}
		if next, ok := m.screenKey(k); ok {
			return next, nil
		}
		if !m.hasTables() {
			return screenDefs[m.screen].key(m, k)
		}
		if cmd, handled, early := m.runListAction(k.String()); handled {
			if early {
				return m, cmd
			}
			flash = cmd
		} else {
			switch k.String() {
			case "t", "T":
				m.cycleTrack(k.String() == "T")
			case "esc":
				// esc clears a search; otherwise it only hides the toast, so the cursor
				// stays where the Owner was working.
				if m.text != "" {
					m.text = ""
					m.cursor = 0
				}
			case "q", "ctrl+c":
				return m, m.quit()
			case "right", "tab":
				// ←→ switch views; tab does too here, as a table screen has only one pane.
				m.table = (m.table + 1) % len(m.views())
				m.cursor = 0
			case "left", "shift+tab":
				m.table = (m.table + len(m.views()) - 1) % len(m.views())
				m.cursor = 0
			case "down", "j":
				m.cursor++
			case "up", "k":
				m.cursor--
			case "g":
				m.cursor = 0
			case "G":
				m.cursor = len(m.rows()) - 1
			case "?":
				m.overlay = &helpMode{}
			case "r":
				m.rescanAll()
			}
		}
	}
	m = m.syncDetail()
	m = m.syncProfile()
	m.clampCursor()
	return m, flash
}

// cycleTrack moves the track filter to the next track, or the previous one: All, then
// each workspace.json track in order, then the off-list Tracks rows are on (filterTracks).
// A Track with no rows in this view is skipped, so t never lands on an empty list; All
// always has its turn.
func (m *model) cycleTrack(back bool) {
	if len(m.tracks) == 0 {
		return
	}
	n := len(m.filterTracks()) + 1
	step := 1
	if back {
		step = n - 1
	}
	next := min(m.track, n-1)
	for range n {
		next = (next + step) % n
		if next == 0 || m.trackTotal(next) > 0 {
			break
		}
	}
	m.track = next
	m.cursor = 0
}

// cursorFolder is the folder of the row under the cursor, or "" when the table is empty.
func (m model) cursorFolder() string {
	if rows := m.rows(); m.cursor >= 0 && m.cursor < len(rows) {
		return rows[m.cursor].Folder
	}
	return ""
}

// selected is the row under the cursor with the cursor clamped into range, so a change
// that removed or reordered rows can never make a read panic. ok is false when the
// active view has no rows.
func (m model) selected() (store.Application, bool) {
	rows := m.rows()
	if len(rows) == 0 {
		return store.Application{}, false
	}
	i := min(max(m.cursor, 0), len(rows)-1)
	return rows[i], true
}

// clampCursor keeps the cursor inside the active view after a change that may have
// removed or reordered rows, so the cursor is never left out of range.
func (m *model) clampCursor() {
	n := len(m.rows())
	switch {
	case n == 0:
		m.cursor = 0
	case m.cursor >= n:
		m.cursor = n - 1
	case m.cursor < 0:
		m.cursor = 0
	}
}

// keepCursor puts the cursor on folder's row when the active table still shows it.
func (m *model) keepCursor(folder string) {
	for i, r := range m.rows() {
		if r.Folder == folder {
			m.cursor = i
			return
		}
	}
}

// dropBodies invalidates the rendered-body cache: the Workspace content changed, so every
// cached Document render is stale. Every rescan path calls this instead of bumping gen, so
// a new path cannot forget it.
func (m *model) dropBodies() { m.gen++ }

// reload rescans the whole Workspace and keeps the cursor on folder when it is still shown.
// A failed scan is reported through m.err and leaves the rows as they were.
func (m *model) reload(folder string) bool {
	apps, err := m.store.Scan()
	if err != nil {
		m.err = err
		return false
	}
	m.apps, m.err = apps, nil
	m.loadRules()
	m.dropBodies()
	m.keepCursor(folder)
	return true
}

// loadRules re-reads the Search rules. A workspace.json that does not read leaves the
// rules unset: Settings reports the file, and no age warning is better than a wrong one.
func (m *model) loadRules() {
	m.rules, m.visa, m.payFX = store.SearchRules{}, store.Visa{}, nil
	if ws, err := m.store.Settings(); err == nil && ws.Err == nil {
		m.rules, m.visa, m.payFX = ws.Rules, ws.Visa, ws.PayFX
	}
}

// rescanAll is `r`: it rereads workspace.json as well as every Application.
func (m *model) rescanAll() {
	folder := m.cursorFolder()
	tracks, err := m.store.Tracks()
	if err != nil {
		m.err = err
		return
	}
	chosen, on := m.selectedTrack()
	m.tracks = tracks
	m.track = 0 // unless the filtered Track is still an option
	if i := slices.Index(m.filterTracks(), chosen); on && i >= 0 {
		m.track = i + 1
	}
	m.reload(folder)
}
