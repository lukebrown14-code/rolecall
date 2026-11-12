package ui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"charm.land/bubbles/v2/key"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/store"
)

// The keymap registry. Every screen, focus and pop-up declares the keys it accepts as
// helpKeys, each with a group (where it sits) and a priority (how long it survives on a
// narrow key line). The key line, the `?` pop-up and the `:` command palette all read
// the same list from helpKeys, so a key is declared once and shows up everywhere.
//
// To add an action: append help(group, key, description) to the list for its context —
// the list view below, a screen's help func in screenDefs, or a mode's Keys — and raise
// or lower it with .at(priority) when the group's default is wrong for it.

// helpGroup places a key: the key line's left (actions) or right (navigation), and the
// `?` pop-up's columns.
type helpGroup int

const (
	// groupAction is something done to what is here: the key line's left, the pop-up's
	// first column.
	groupAction helpGroup = iota
	// groupNav moves around this context: the key line's right, the pop-up's second column.
	groupNav
	// groupCursor is navigation so familiar it only appears in the `?` pop-up.
	groupCursor
	// groupGlobal works on every screen: the pop-up's Everywhere column. Only `?` and the
	// screen digits reach the key line.
	groupGlobal
)

// Key-line priorities: when the line is too narrow the lowest goes first, and among
// equals the rightmost. prioHelpOnly keys never reach the key line.
const (
	prioHelpOnly = iota
	prioScreen   // 1-6 screen, tab focus, : commands: learnt once, dropped first
	prioNav      // ←→ view or tab, ↑↓ scroll
	prioMinor    // the occasional actions: x, u, d, b, f, p, t
	prioMain     // e edit, n notes, a pop-up's own keys
	prioCore     // enter, m move, a add, / search: how a newcomer finds their way in
	prioPinned   // ? help and esc back: how to get unstuck, never dropped
)

// The `?` pop-up's layout: key column, then column width.
const (
	helpKeyWidth = 9
	helpColumn   = 23 // three fit side by side from 80 columns
	helpColumns  = 3
	keyLineGap   = 4 // the key line's indent plus the space kept between its halves
)

type helpKey struct {
	key.Binding
	group helpGroup
	prio  int
	// press is the one key the command palette sends to run this action, or "" when the
	// label names several keys ("j k", "1-6") and the palette leaves it out.
	press string
}

// help declares a key with its group's default priority.
func help(group helpGroup, k, desc string) helpKey {
	prio := prioHelpOnly
	switch group {
	case groupAction:
		prio = prioMain
	case groupNav:
		prio = prioNav
	}
	return helpKey{key.NewBinding(key.WithKeys(k), key.WithHelp(k, desc)), group, prio, pressable(k)}
}

// at sets a key's key-line priority.
func (k helpKey) at(prio int) helpKey {
	k.prio = prio
	return k
}

// pressable is the key the palette can send for a label: a single character or a named
// key. Labels naming several keys or a range are not runnable.
func pressable(k string) string {
	switch k {
	case "enter", "esc", "tab", "shift+tab", keyBackspace:
		return k
	}
	if utf8.RuneCountInString(k) == 1 && k != "←" && k != "→" && k != "↑" && k != "↓" {
		return k
	}
	return ""
}

func (k helpKey) pair() [2]string {
	h := k.Help()
	return [2]string{h.Key, h.Desc}
}

// contextName names where the Owner is: the screen, its view when it has more than one,
// and the detail tab.
func (m model) contextName() string {
	label := screenDefs[m.screen].name
	if m.detail != nil {
		return label + " · " + string(m.detail.tab)
	}
	if ts := m.views(); len(ts) > 1 && ts[m.table] != label {
		label += " · " + ts[m.table]
	}
	return label
}

// helpKeys is the registry's answer for the current context: an open pop-up's keys
// while one owns the keyboard, otherwise the context's keys followed by the global ones.
func (m model) helpKeys() []helpKey {
	if m.overlay != nil {
		return modeKeys(m.overlay.Keys())
	}
	return m.screenKeys()
}

// screenKeys is the context's keys and then the global keys, ignoring any pop-up: what
// the `?` pop-up and the command palette list.
func (m model) screenKeys() []helpKey {
	ctx := m.contextKeys()
	return append(ctx, m.globalKeys(ctx)...)
}

// modeKeys turns a pop-up's key pairs into registry keys: esc (and keys ending in it)
// on the right and pinned, the rest on the left.
func modeKeys(pairs [][2]string) []helpKey {
	keys := make([]helpKey, 0, len(pairs))
	for _, kv := range pairs {
		k := help(groupAction, kv[0], kv[1])
		if kv[0] == "esc" || strings.HasSuffix(kv[0], " esc") {
			k = help(groupNav, kv[0], kv[1]).at(prioPinned)
		}
		keys = append(keys, k)
	}
	return keys
}

// contextKeys is only the keys that do something in the current screen, detail tab or
// row, without the global ones.
func (m model) contextKeys() []helpKey {
	// A screen without Application tables owns its keys, and so its help.
	if !m.hasTables() {
		return screenDefs[m.screen].help(m)
	}
	a, onRow := m.current()
	// A malformed row can be inspected and fixed in the Editor; Move, undo, notes and
	// downloads are refused there.
	usable := !onRow || a.Malformed == nil
	var keys []helpKey
	if m.detail != nil {
		if m.detail.tab == tabDocs && usable {
			keys = append(keys, help(groupAction, "enter", "open").at(prioCore))
		}
		if usable {
			keys = append(keys, help(groupAction, "e", "edit"))
		} else {
			keys = append(keys, help(groupAction, "e", "fix in editor").at(prioCore))
		}
	} else if onRow {
		keys = append(keys, help(groupAction, "enter", "open").at(prioCore))
		if usable {
			keys = append(keys, help(groupAction, "e", "edit"))
		} else {
			keys = append(keys, help(groupAction, "e", "fix in editor").at(prioCore))
		}
	}
	if onRow && usable {
		keys = append(keys, help(groupAction, "m", "move").at(prioCore))
	}
	if usable && len(m.undo) > 0 {
		keys = append(keys, help(groupAction, "u", "undo").at(prioMinor))
	}
	if onRow && usable {
		if m.detail == nil && (a.Status == store.StatusLead || a.Status == store.StatusDraft) {
			keys = append(keys, help(groupAction, "x", "dismiss").at(prioMinor))
		}
		notes := help(groupAction, "n", "notes")
		if m.detail != nil {
			notes = notes.at(prioMinor) // the reader's navigation outlasts it
		}
		keys = append(keys, notes)
		if m.plugin != "" && buildable(a) {
			keys = append(keys, help(groupAction, "b", "build").at(prioMinor))
		}
		if len(a.PDFs) > 0 {
			keys = append(keys, help(groupAction, "d", "download").at(prioMinor))
		}
	}
	// Handoff: f on any Application, p while it is still a Lead or Draft.
	if onRow && m.harness != "" {
		keys = append(keys, help(groupAction, "f", "assess fit").at(prioMinor))
		if usable && (a.Status == store.StatusLead || a.Status == store.StatusDraft) {
			keys = append(keys, help(groupAction, "p", "prepare docs").at(prioMinor))
		}
	}
	if m.detail != nil {
		vertical := "scroll"
		if m.detail.tab == tabDocs {
			vertical = "select"
		}
		// Reading is what the detail view is for, so its navigation outlasts the actions.
		return append(keys, help(groupNav, "←→", "tab").at(prioCore), help(groupNav, "↑↓", vertical).at(prioMain),
			help(groupCursor, "g G", "top / bottom"), help(groupNav, "esc", "back").at(prioPinned))
	}
	if m.views()[m.table] == tableLeads {
		sort := "sort newest"
		if m.leadsNewest {
			sort = "sort by close date"
		}
		keys = append(keys, help(groupAction, "s", sort).at(prioMinor))
	}
	keys = append(keys, help(groupAction, "a", "add").at(prioCore), help(groupAction, "/", "search").at(prioCore))
	if len(m.views()) > 1 {
		keys = append(keys, help(groupNav, "←→", "view"))
	}
	return append(keys, help(groupCursor, "j k ↑↓", "cursor"), help(groupCursor, "g G", "top / bottom"))
}

// trackFilterApplies reports whether t and T cycle the track filter here: the tables
// and Stats, not the detail view, Profile or Settings.
func (m model) trackFilterApplies() bool {
	return len(m.tracks) > 0 && m.detail == nil && (m.hasTables() || m.screen == screenIndex(screenStats))
}

// globalKeys is the keys that work on every screen. A context key with the same key
// wins (Settings' r renames a Track), so a key is never listed twice.
func (m model) globalKeys(ctx []helpKey) []helpKey {
	keys := []helpKey{help(groupGlobal, "1-6", "screen").at(prioScreen)}
	if m.trackFilterApplies() {
		keys = append(keys, help(groupGlobal, "t", "next track"), help(groupGlobal, "T", "previous track"))
	}
	if m.text != "" && m.detail == nil && m.screen != settingsScreen {
		keys = append(keys, help(groupGlobal, "esc", "clear search"))
	}
	if m.screen != settingsScreen {
		keys = append(keys, help(groupGlobal, ",", "settings"))
	}
	keys = append(keys, help(groupGlobal, "r", "rescan"), help(groupGlobal, ":", "commands").at(prioScreen), help(groupGlobal, "ctrl+k", "commands"),
		help(groupGlobal, "?", "help").at(prioPinned), help(groupGlobal, "q", "quit"))
	taken := map[string]bool{}
	for _, k := range ctx {
		taken[k.Help().Key] = true
	}
	out := keys[:0]
	for _, k := range keys {
		if !taken[k.Help().Key] {
			out = append(out, k)
		}
	}
	return out
}

// screenIndex is a screen's position in screenDefs.
func screenIndex(name string) int {
	for i, d := range screenDefs {
		if d.name == name {
			return i
		}
	}
	return -1
}

func pairs(keys []helpKey, groups ...helpGroup) [][2]string {
	var out [][2]string
	for _, k := range keys {
		for _, g := range groups {
			if k.group == g {
				out = append(out, k.pair())
			}
		}
	}
	return out
}

// fitKeys is the key line's two halves: actions on the left, navigation and the
// footer's global keys on the right. While they do not fit in w, the lowest-priority key
// goes, the rightmost first among equals, so navigation hints drop before main actions.
func fitKeys(p pal, keys []helpKey, w int) (left, right string) {
	var shown []helpKey
	for _, k := range keys {
		if k.prio > prioHelpOnly {
			shown = append(shown, k)
		}
	}
	// Left and right in display order: actions, then navigation, then globals.
	ordered := make([]helpKey, 0, len(shown))
	for _, g := range []helpGroup{groupAction, groupNav, groupGlobal} {
		for _, k := range shown {
			if k.group == g {
				ordered = append(ordered, k)
			}
		}
	}
	render := func() (string, string) {
		return p.keys(pairs(ordered, groupAction)), p.keys(pairs(ordered, groupNav, groupGlobal))
	}
	for {
		left, right = render()
		width := xansi.StringWidth(left) + xansi.StringWidth(right) + keyLineGap
		if left == "" || right == "" {
			width -= keyLineGap - gutter // one half is empty: no gap between them
		}
		if width <= w || len(ordered) == 0 {
			return left, right
		}
		drop := -1
		for i, k := range ordered {
			if k.prio < prioPinned && (drop < 0 || k.prio <= ordered[drop].prio) {
				drop = i
			}
		}
		if drop < 0 {
			// Only pinned keys are left: cut the line rather than lose them.
			return xansi.Truncate(left, max(0, w-xansi.StringWidth(right)-keyLineGap), "…"), right
		}
		ordered = append(ordered[:drop], ordered[drop+1:]...)
	}
}

// keyLine is the bottom line: the current context's keys, or an open pop-up's own keys,
// fitted by priority.
func (m model) keyLine(p pal, w int) string {
	left, right := fitKeys(p, m.helpKeys(), w-gutter)
	return " " + spread(left, right, w-gutter)
}

// helpTitle names the `?` pop-up's context.
func (m model) helpTitle() string { return "Keys · " + m.contextName() }

// glyphLegend says what each glyph means; every glyph has exactly one meaning. The
// Status glyphs are drawn in their own colours, as the list draws them.
func glyphLegend(p pal, followUpDays int) string {
	row := func(label string, items ...string) string {
		var b strings.Builder
		b.WriteString(p.text(p.fg).Render(padRight(label, helpKeyWidth+1)))
		for i, it := range items {
			if i < len(items)-1 {
				it += strings.Repeat(" ", max(1, helpColumn-gutter-xansi.StringWidth(it)))
			}
			b.WriteString(it)
		}
		return b.String()
	}
	status := func(s string) string { return p.statusMark(s) + p.text(p.dim).Render(" "+s) }
	open := []string{store.StatusLead, store.StatusDraft, store.StatusApplied, store.StatusInterviewing, store.StatusOffer}
	closed := []string{store.StatusRejected, store.StatusWithdrawn, store.StatusDismissed}
	line := func(ss []string) string {
		parts := make([]string, len(ss))
		for i, s := range ss {
			parts[i] = status(s)
		}
		return strings.Join(parts, "  ")
	}
	dim := func(s string) string { return p.text(p.dim).Render(s) }
	current := p.statusMark(store.StatusInterviewing) + p.text(p.fg).Bold(true).Render(" current")
	earlier := p.text(p.faint).Render(p.statusGlyph(store.StatusApplied)) + dim(" earlier")
	return p.heading("Glyphs") + "\n" +
		row("Statuses", line(open)) + "\n" +
		row("", line(closed)) + "\n" +
		row("Timeline", current, earlier, dim("~ approximate date")) + "\n" +
		row("Docs", dim("✓ ready"), dim("○ missing"), dim(meterOn+meterOff+" N of M ready")) + "\n" +
		row("Flags", dim("⚠ needs attention"), dim(fmt.Sprintf("⚑ quiet %d+ days", followUpDays))) + "\n" +
		row("Fit", dim(store.FitStrong+" · "+store.FitGood+" · "+store.FitMarginal+" · "+store.FitPoor+" · — not assessed"))
}

// helpBox is the `?` pop-up: the keys of the context it was opened from, then the
// global keys, then the glyph legend. Paging appears only when there is a second page.
func (m model) helpBox(p pal) string {
	under := m
	under.overlay = nil
	keys := under.screenKeys()
	col := func(title string, list [][2]string) string {
		var b strings.Builder
		b.WriteString(p.heading(title) + "\n")
		for _, kv := range list {
			b.WriteString(p.key().Render(padRight(kv[0], helpKeyWidth)) + p.text(p.fg).Render(kv[1]) + "\n")
		}
		return strings.TrimSuffix(b.String(), "\n")
	}
	cols := []string{col(m.contextName(), pairs(keys, groupAction)), col("Move around", pairs(keys, groupNav, groupCursor)),
		col("Everywhere", pairs(keys, groupGlobal))}
	body := joinColumns(cols, helpColumn, popWidth(popLarge, m.viewWidth())-popInset >= helpColumns*helpColumn)
	if c := conventions(keys); c != "" {
		body += "\n\n" + p.text(p.dim).Render(c)
	}
	// A page fills the body between the rules, less the pop-up's borders and padding.
	const popChrome = 4
	size := max(1, m.layout().bodyHeight(m.contextLines())-popChrome)
	lines := append(strings.Split(body, "\n"), "")
	// The legend starts a page of its own rather than leaving its heading behind, when
	// it fits on one.
	legend := strings.Split(glyphLegend(p, m.followUpDays), "\n")
	if at := len(lines) % size; at != 0 && at+len(legend) > size && len(legend) <= size {
		lines = append(lines, make([]string, size-at)...)
	}
	lines = append(lines, legend...)
	pages := (len(lines) + size - 1) / size
	hm, _ := m.overlay.(*helpMode)
	page := 0
	if hm != nil {
		hm.pages = pages
		hm.page = min(hm.page, pages-1)
		page = hm.page
	}
	body = strings.Join(lines[page*size:min(len(lines), (page+1)*size)], "\n")
	title := m.helpTitle()
	if pages > 1 {
		title += fmt.Sprintf(" · %d/%d", page+1, pages)
	}
	return popup(p, title, false, popLarge, m.viewWidth(), body, p.keys(helpModeKeys(pages)))
}

// conventions is the one line that says how the navigation keys divide the work, for the
// ones this context has: ←→ always switches a view or tab, tab always switches a pane.
func conventions(keys []helpKey) string {
	var parts []string
	for _, k := range keys {
		switch k.Help().Key {
		case "←→":
			parts = append(parts, "←→ switches "+k.Help().Desc)
		case "tab":
			parts = append(parts, "tab switches pane")
		}
	}
	return strings.Join(parts, " · ")
}

// helpModeKeys is the `?` pop-up's own keys: paging only when there is more than a page.
func helpModeKeys(pages int) [][2]string {
	if pages > 1 {
		return [][2]string{{"↑↓", "page"}, {"esc", hintClose}}
	}
	return [][2]string{{"esc", hintClose}}
}
