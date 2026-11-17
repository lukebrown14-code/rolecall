package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/internal/config"
	"github.com/lukebrown14-code/rolecall/internal/store"
	"github.com/lukebrown14-code/rolecall/internal/ui/settings"
)

// An empty screen says what is missing in one muted line and the next key to press, as a
// key chip. A hint is offered only when the registry lists its key here, so it is never
// wrong: a key that does nothing in this context is left out.

// emptyHint is one next-key hint: the key and the words beside it.
type emptyHint struct{ key, words string }

// addLeadHint is the hint every empty list shares.
var addLeadHint = emptyHint{"a", "add a Lead"}

// keyAvailable reports whether the registry lists key in the current context.
func (m model) keyAvailable(key string) bool {
	for _, k := range m.screenKeys() {
		if k.Help().Key == key {
			return true
		}
	}
	return false
}

// availableHints drops the hints whose key is not listed here.
func (m model) availableHints(hints ...emptyHint) []emptyHint {
	var out []emptyHint
	for _, h := range hints {
		if m.keyAvailable(h.key) {
			out = append(out, h)
		}
	}
	return out
}

// emptyViewNoun completes "No …" for a view.
func emptyViewNoun(table string) string {
	switch table {
	case tableLeads:
		return "Leads"
	case tableDismissed:
		return "dismissed Leads"
	case tableFiltered:
		return "Leads a Search rule dismissed"
	case tableDraft:
		return "Drafts"
	case tableDeadlines:
		return "Leads or Drafts with a close date"
	case tableApplied:
		return "Applied Applications"
	case tableInterviewing:
		return "Interviewing Applications"
	case tableOffer:
		return "Offers"
	case tableClosed:
		return "closed Applications"
	}
	return "Applications"
}

// emptyViewLine is what an empty view says and the keys that help: change the Track when
// one filters, change the view, add a Lead.
func (m model) emptyViewLine(table string) (string, []emptyHint) {
	msg := "No " + emptyViewNoun(table)
	hints := []emptyHint{{"s", descChangeView}, addLeadHint}
	switch {
	case m.track > 0:
		msg += " in " + m.trackName()
		hints = append([]emptyHint{{"t", "change Track"}}, hints...)
	case table != tableLeads && table != tableDismissed && table != tableFiltered:
		msg += " yet"
	}
	return msg, m.availableHints(hints...)
}

// emptySearchLine is the line for a search with no matches here. When other screens or
// views have matches it says where, "No match for \"orbit\" in Leads · 1 in Applied", with the
// key to the first of them.
func (m model) emptySearchLine() (string, []emptyHint) {
	clear := emptyHint{"esc", "clear search"}
	if where, key, words := m.searchElsewhere(); len(where) > 0 {
		msg := "No match for \"" + m.text + "\" in " + m.contextName() + " · " + strings.Join(where, " · ")
		hints := m.availableHints(clear)
		if key != "" {
			hints = append([]emptyHint{{key, "go to " + words}}, hints...)
		}
		return msg, hints
	}
	return "No Applications match \"" + m.text + "\"", m.availableHints(clear)
}

// The first-run welcome shows on a table screen of a Workspace with no Applications at
// all. It offers the first Lead, then a short setup checklist: Editor, Harness, Profile,
// each with the key that goes there, and only the steps not yet done.

// welcoming reports that the welcome is on screen.
func (m model) welcoming() bool { return m.hasTables() && m.detail == nil && len(m.apps) == 0 }

// setupStep is one item of the welcome's checklist.
type setupStep struct {
	key   string
	words func(model) string
	done  func(model) bool
}

// setupSteps is the checklist in order. Editor and Harness keys open Settings › This
// machine on their own row; 5 is the Profile screen, whose missing-Profile notice gives
// the same instruction (profileSetupWords), so the two never disagree.
var setupSteps = []setupStep{
	{"e", text("pick an Editor"), editorChosen},
	{"h", text("connect a Harness"), func(m model) bool { return m.harness != "" }},
	{"5", func(m model) string { return "set up your Profile: " + m.profileSetupWords() }, func(m model) bool { return m.profileStarted }},
}

// editorChosen reports an Editor the Owner set (config, $VISUAL or $EDITOR), not the vi
// fallback.
func editorChosen(m model) bool {
	return m.editor != "" || (m.editorSource != "" && m.editorSource != config.SourceDefault)
}

// profileBeyondContact reports a Profile with any section besides Contact: init writes
// only Contact, and create-profile adds the rest.
func profileBeyondContact(p store.Profile) bool {
	for _, s := range p.Sections {
		if s.Heading != headingContact && strings.TrimSpace(s.Body) != "" {
			return true
		}
	}
	return false
}

// pendingSetup is the checklist steps not yet done.
func (m model) pendingSetup() []setupStep {
	var out []setupStep
	for _, s := range setupSteps {
		if !s.done(m) {
			out = append(out, s)
		}
	}
	return out
}

// setupPending reports whether the welcome lists the step with key.
func (m model) setupPending(key string) bool {
	for _, s := range m.pendingSetup() {
		if s.key == key {
			return true
		}
	}
	return false
}

// welcomeActions are the welcome's own keys: e and h open Settings on the Editor and
// Harness rows. They are enabled only while the welcome lists them, so e still edits a
// row everywhere else.
var welcomeActions = []listAction{
	welcomeAction("e", "pick an Editor", settings.LabelEditor),
	welcomeAction("h", "connect a Harness", settings.LabelHarness),
}

// The welcome's keys join the one action table, so the key line, the help and the
// dispatcher all see them. They are added in init, as screenDefs is, because their run
// reaches the Settings screen, which reads the table back.
func init() { listActions = append(listActions, welcomeActions...) }

func welcomeAction(key, desc, label string) listAction {
	on := func(m model) bool { return m.welcoming() && m.setupPending(key) }
	return listAction{
		keys: []string{key}, keyLabel: key, desc: text(desc),
		group: groupAction, prio: fixed(prioMain),
		listed: on, enabled: on, early: true,
		run: func(m *model, _ string) tea.Cmd {
			*m = m.openSettingsAt(label)
			return nil
		},
	}
}

// openSettingsAt opens Settings › This machine with the row labelled label selected.
func (m model) openSettingsAt(label string) model {
	m = m.openSettings()
	if m.settings == nil {
		return m
	}
	st := *m.settings
	st.Group, st.Header = settings.GroupMachine, false
	m.settings = &st
	for i, r := range m.settingsRows(settings.GroupMachine) {
		if r.Label == label {
			st.Row = i
		}
	}
	return m
}

// welcome is the first-run screen: the wordmark and what is missing, the first Lead, the
// setup steps still to do, and the help key, in one key column.
func (m model) welcome(p pal) string {
	step := func(k, desc string) string {
		return p.Key().Render(padRight(k, gutter+1)) + p.Text(p.Fg).Render(desc)
	}
	lines := []string{
		p.Text(p.Fg).Bold(true).Render(wordmark+" rolecall") + p.Text(p.Dim).Render(" · no Applications yet"),
		"",
	}
	if m.keyAvailable("a") {
		lines = append(lines, step("a", "add your first Lead"))
	}
	if pending := m.pendingSetup(); len(pending) > 0 {
		lines = append(lines, "", p.Text(p.Dim).Render("Set up"))
		for _, s := range pending {
			lines = append(lines, step(s.key, s.words(m)))
		}
	}
	lines = append(lines, "", step("?", "see every key"))
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// emptyLines lays a message and its hints out as muted text in rows no wider than w,
// breaking only between hints so a key chip stays beside its words.
func (m model) emptyLines(p pal, w int, msg string, hints []emptyHint) []string {
	w = max(1, w)
	sep, sepW := p.Text(p.Dim).Render(" · "), 3
	atoms := []string{p.Text(p.Dim).Render(msg)}
	for _, h := range hints {
		atoms = append(atoms, p.Key().Render(h.key)+p.Text(p.Dim).Render(" "+h.words))
	}
	var lines []string
	cur, curW := "", 0
	for _, a := range atoms {
		aw := xansi.StringWidth(a)
		if cur != "" && curW+sepW+aw > w {
			lines, cur, curW = append(lines, cur), "", 0
		}
		if cur != "" {
			cur, curW = cur+sep, curW+sepW
		}
		cur, curW = cur+a, curW+aw
	}
	lines = append(lines, cur)
	// A message longer than the row wraps on word boundaries.
	for i, l := range lines {
		if xansi.StringWidth(l) > w {
			lines[i] = lipgloss.NewStyle().Width(w).Render(l)
		}
	}
	return strings.Split(strings.Join(lines, "\n"), "\n")
}

// emptyBody places the empty lines in a body: centred at wide sizes, top-aligned and
// indented at narrow ones.
func (m model) emptyBody(p pal, w, h int, msg string, hints []emptyHint) string {
	if m.layout().wide() {
		lines := m.emptyLines(p, w-2*gutter, msg, hints)
		return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, strings.Join(lines, "\n"))
	}
	pad := strings.Repeat(" ", gutter)
	lines := m.emptyLines(p, w-gutter-rightMargin, msg, hints)
	return lipgloss.Place(w, h, lipgloss.Left, lipgloss.Top, pad+strings.Join(lines, "\n"+pad))
}
