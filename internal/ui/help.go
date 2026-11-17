package ui

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"charm.land/bubbles/v2/key"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/internal/store"
	appearance "github.com/lukebrown14-code/rolecall/internal/ui/theme"
)

// The keymap registry. Every screen, focus and pop-up declares the keys it accepts as
// helpKeys, each with a group (where it sits) and a priority (how long it survives on a
// narrow key line). The key line, the `?` pop-up and the `:` command palette all read
// the same list from helpKeys, so a key is declared once and shows up everywhere.
//
// r is always rescan; F drafts a Follow-up on a Follow-up due row. space opens the
// Workspace menu. g and G are go-to-top and bottom; M is not bound. The Leads sort is o
// and s cycles the view.
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
	// groupFilter narrows or orders what is listed (search, status filter, Track filter,
	// sort, the command palette): the key line's left after the actions, and the `?`
	// pop-up's Filter & find section.
	groupFilter
)

// Key-line priorities: when the line is too narrow the lowest goes first, and among
// equals the rightmost. prioHelpOnly keys never reach the key line.
const (
	prioHelpOnly = iota
	prioScreen   // 1-6 screen, tab focus, : commands: learnt once, dropped first
	prioNav      // ←→ view or tab, ↑↓ scroll
	prioMinor    // the occasional actions: x, u, d, b, f, p, t
	prioMain     // e edit, n notes, a pop-up's own keys
	prioLean     // x dismiss on a Lead or Draft: kept on the lean key line, dropped before the core
	prioCore     // enter, m move, a add, / search, u undo while there is one: how a newcomer finds their way in
	prioPinned   // ? help and esc back: how to get unstuck, never dropped
)

// The `?` pop-up's layout: key column, then column width.
const (
	helpKeyWidth = 9
	helpColumn   = 23 // three fit side by side from 80 columns
	keyLineGap   = 4  // the key line's indent plus the space kept between its halves
)

type helpKey struct {
	key.Binding
	group helpGroup
	prio  int
	// press is the one key the command palette sends to run this action, or "" when the
	// label names several keys ("j k", "1-6") and the palette leaves it out.
	press string
	// dim draws the key and its words muted in the help and palette: a Handoff key while
	// no Harness is set.
	dim bool
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
	return helpKey{Binding: key.NewBinding(key.WithKeys(k), key.WithHelp(k, desc)), group: group, prio: prio, press: pressable(k)}
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
	keys := m.actionKeys()
	if m.detail != nil {
		vertical := "scroll"
		if m.detail.tab == tabDocs {
			vertical = "select"
		}
		// Reading is what the detail view is for, so its navigation outlasts the actions.
		keys = append(keys, help(groupNav, "←→", "tab").at(prioCore), help(groupNav, "↑↓", vertical).at(prioMain),
			help(groupCursor, "g G", "top / bottom"), help(groupCursor, "J", "next row"), help(groupCursor, "K", "previous row"))
		if m.detail.tab != tabDocs {
			keys = append(keys, help(groupCursor, "pgup pgdn", "page"))
		}
		return append(keys, help(groupNav, "esc", "back").at(prioPinned))
	}
	if len(m.views()) > 1 {
		keys = append(keys, help(groupNav, "←→", "view").at(prioHelpOnly)) // s says it on the key line
	}
	return append(keys, help(groupCursor, "j k ↑↓", "cursor"), help(groupCursor, "g G", "top / bottom"))
}

// descTrackFilter is t's one name, in the help, the palette and the space menu.
const descTrackFilter = "track filter"

// trackFilterApplies reports whether t and T cycle the track filter here: the tables
// and Stats, not the detail view, Profile or Settings.
func (m model) trackFilterApplies() bool {
	return len(m.tracks) > 0 && m.detail == nil && (m.hasTables() || m.screen == screenIndex(screenStats))
}

// globalKeys is the keys that work on every screen. A context key with the same key
// wins over the global one, so a key is never listed twice.
func (m model) globalKeys(ctx []helpKey) []helpKey {
	if m.fieldOpen() {
		// An open field takes every printable key as text: 1-6, : and ? type into it, so
		// the key line offers only the field's own keys.
		return nil
	}
	keys := []helpKey{help(groupGlobal, "1-6", "screen").at(prioScreen)}
	if m.trackFilterApplies() {
		keys = append(keys, help(groupFilter, "t", descTrackFilter).at(prioMinor), help(groupFilter, "T", "previous track"))
	}
	if m.text != "" && m.detail == nil && m.screen != settingsScreen {
		// A search follows the Owner to every screen, so how to end it is never dropped.
		keys = append(keys, help(groupFilter, "esc", "clear search").at(prioPinned))
	}
	if m.screen != settingsScreen {
		keys = append(keys, help(groupGlobal, ",", "settings"))
	}
	if m.connectsHarness("h") {
		// Listed whatever the Application count, so the Harness keys can be found after
		// the welcome has gone.
		keys = append(keys, help(groupAction, "h", descConnectHarness).at(prioHelpOnly))
	}
	keys = append(keys, help(groupGlobal, "r", "rescan"), help(groupFilter, ":", "commands").at(prioMinor), help(groupFilter, "ctrl+k", "commands"),
		help(groupGlobal, "?", "help").at(prioPinned))
	// The tab bar shows the menu key where it has room, so the key line leaves it to the help.
	menu := help(groupGlobal, "space", "menu").at(prioPinned)
	if m.barKeys(m.pal()) != "" {
		menu = menu.at(prioHelpOnly)
	}
	keys = append(keys, menu)
	keys = append(keys, help(groupGlobal, "q", "quit"))
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

// fieldOpen reports whether a Settings field is being edited, so it owns the keyboard.
func (m model) fieldOpen() bool {
	return m.screen == settingsScreen && m.settings != nil && m.settings.Edit != nil
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
			// The palette key sits with the footer's global keys, beside help, rather than
			// among the filters.
			if k.group == groupFilter && k.Help().Key == ":" {
				k.group = groupGlobal
			}
			shown = append(shown, k)
		}
	}
	// Left and right in display order: actions, then navigation, then globals.
	ordered := make([]helpKey, 0, len(shown))
	for _, g := range []helpGroup{groupAction, groupFilter, groupNav, groupGlobal} {
		for _, k := range shown {
			if k.group == g {
				ordered = append(ordered, k)
			}
		}
	}
	render := func() (string, string) {
		return p.KeyRow(pairs(ordered, groupAction, groupFilter)), p.KeyRow(pairs(ordered, groupNav, groupGlobal))
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
//
// A screen's line is lean: its core and pinned keys (enter, m, a, /, esc, ?, …), plus
// x dismiss on a Lead or Draft and u undo while a move can be undone; the rest live in the
// `?` pop-up. A pop-up keeps all its keys: it has no other help.
func (m model) keyLine(p pal, w int) string {
	keys := m.helpKeys()
	if m.overlay == nil {
		core := keys[:0:0]
		for _, k := range keys {
			if k.prio >= prioLean {
				core = append(core, k)
			}
		}
		keys = core
	}
	left, right := fitKeys(p, keys, w-gutter)
	return " " + spread(left, right, w-gutter)
}

// helpTitle names the `?` pop-up's context.
func (m model) helpTitle() string { return "Keys · " + m.contextName() }

// legendPerLine is the most legend items that share a line before a row wraps under
// itself; fewer fit on a narrow pop-up.
const legendPerLine = 3

// The `?` pop-up's paging: a heading keeps helpHeadRoom lines (itself and two rows) on its
// page, and pages shorter than helpRoomy lines drop their blank spacers.
const (
	helpHeadRoom = 3
	helpRoomy    = 10
	// helpWordsMin is the least room for a row's words beside its label before they go on
	// the lines under it.
	helpWordsMin = 24
	// helpRunOnGain is the fewest rows running the key sections on across three columns
	// must save before a section is split between two.
	helpRunOnGain = 2
	// helpRunOnColumns is the columns the key sections run on across.
	helpRunOnColumns = 3
)

// helpLine is one line of the `?` help's body. head marks a section heading, which
// paging never leaves at the foot of a page; note marks a line that belongs to the one
// above it ("(needs a Harness)" under the keys that need one), which never starts a
// column.
type helpLine struct {
	text       string
	head, note bool
}

// helpBlock is a part of the `?` help paged as one where it fits: the keys, the AI
// skills, the glyph legend, the row words.
type helpBlock []helpLine

// heading is a section heading line.
func heading(p pal, title string) helpLine { return helpLine{text: p.Heading(title), head: true} }

// keySection is a heading and its keys, bare and accent-coloured in one key column as
// the key line and palette draw them: dimmed when the key needs a Harness. The key column
// is at least keyW wide, and wider for a merged row (": ctrl+k"), so the column's words
// line up. A section with no keys here is left out, so no heading stands alone.
func keySection(p pal, title string, keys []helpKey, keyW int) helpBlock {
	rows := mergeAliases(keys)
	if len(rows) == 0 {
		return nil
	}
	for _, r := range rows {
		keyW = max(keyW, xansi.StringWidth(r.key)+1)
	}
	out := helpBlock{heading(p, title)}
	for i, r := range rows {
		ks, ds := p.Key(), p.Text(p.Fg)
		if r.dim {
			ks, ds = p.Text(p.Dim), p.Text(p.Dim)
		}
		// "(needs a Harness)" goes on a line of its own under a run of keys that need
		// one, so the column stays narrow enough for three side by side.
		desc, note, split := strings.Cut(r.desc, " "+needsHarnessNote)
		out = append(out, helpLine{text: ks.Render(padRight(r.key, keyW)) + ds.Render(desc+note)})
		if split && (i == len(rows)-1 || !strings.HasSuffix(rows[i+1].desc, needsHarnessNote)) {
			out = append(out, helpLine{text: strings.Repeat(" ", gutter) + ds.Render(needsHarnessNote), note: true})
		}
	}
	return out
}

// stack sets blocks one under another, a blank line between them.
func stack(blocks ...helpBlock) helpBlock {
	var out helpBlock
	for _, b := range blocks {
		if len(b) == 0 {
			continue
		}
		if len(out) > 0 {
			out = append(out, helpLine{})
		}
		out = append(out, b...)
	}
	return out
}

// sideBySide sets blocks as columns, each at least w wide and a gutter wider than its
// widest line, the last as wide as it needs; ok is false when they do not fit in room. A
// line is a heading when any column's line is.
func sideBySide(cols []helpBlock, w, room int) (helpBlock, bool) {
	var kept []helpBlock
	for _, c := range cols {
		if len(c) > 0 {
			kept = append(kept, c)
		}
	}
	widths := make([]int, len(kept))
	total, rows := 0, 0
	for i, c := range kept {
		for _, l := range c {
			widths[i] = max(widths[i], xansi.StringWidth(l.text))
		}
		if i < len(kept)-1 {
			widths[i] = max(w, widths[i]+gutter)
		}
		total += widths[i]
		rows = max(rows, len(c))
	}
	if total > room {
		return nil, false
	}
	out := make(helpBlock, rows)
	for r := range out {
		var b strings.Builder
		for i, c := range kept {
			if r >= len(c) {
				b.WriteString(strings.Repeat(" ", widths[i]))
				continue
			}
			out[r].head = out[r].head || c[r].head
			b.WriteString(c[r].text)
			if i < len(kept)-1 {
				b.WriteString(strings.Repeat(" ", max(0, widths[i]-xansi.StringWidth(c[r].text))))
			}
		}
		out[r].text = strings.TrimRight(b.String(), " ")
	}
	return out, true
}

// keyBlock is the key sections laid out as the help's width allows: three columns
// (Navigate | Act | Filter & find); failing that two, pairing the sections whichever way
// is shortest; failing that one under another. Each column count tries the usual key
// column first, then one only as wide as each section's keys. With flow three columns
// may also run the sections on like a newspaper's where that saves helpRunOnGain rows,
// so a tall Act column does not set the height of all three.
func keyBlock(p pal, keys []helpKey, inner int, flow bool) helpBlock {
	sections := func(keyW int) (nav, act, find helpBlock) {
		return keySection(p, "Navigate", section(keys, groupNav, groupCursor, groupGlobal), keyW),
			keySection(p, "Act", section(keys, groupAction), keyW),
			keySection(p, "Filter & find", section(keys, groupFilter), keyW)
	}
	// shortest is the shortest of the layouts that fit inner, the first among equals.
	shortest := func(layouts func(nav, act, find helpBlock) [][]helpBlock) helpBlock {
		var best helpBlock
		for _, keyW := range []int{helpKeyWidth, 0} {
			least := 0
			if keyW > 0 {
				least = helpColumn
			}
			for _, cols := range layouts(sections(keyW)) {
				if out, ok := sideBySide(cols, least, inner); ok && (best == nil || len(out) < len(best)) {
					best = out
				}
			}
		}
		return best
	}
	best := shortest(func(nav, act, find helpBlock) [][]helpBlock { return [][]helpBlock{{nav, act, find}} })
	if best == nil {
		best = shortest(func(nav, act, find helpBlock) [][]helpBlock {
			return [][]helpBlock{{stack(nav, find), act}, {nav, stack(act, find)}}
		})
	}
	if flow && best != nil {
		run := shortest(func(nav, act, find helpBlock) [][]helpBlock {
			return [][]helpBlock{runOn(stack(nav, act, find), helpRunOnColumns)}
		})
		if run != nil && len(run) <= len(best)-helpRunOnGain {
			best = run
		}
	}
	if best == nil {
		return stack(sections(helpKeyWidth))
	}
	return best
}

// runOn flows lines down n columns of the least height that holds them, as a newspaper
// runs a story on: a column never ends on a heading or a heading with one row under it,
// and never starts blank or with a note.
func runOn(lines helpBlock, n int) []helpBlock {
	for h := (len(lines) + n - 1) / n; h < len(lines); h++ {
		var cols []helpBlock
		rest := lines
		for len(cols) < n {
			for len(rest) > 0 && rest[0].text == "" {
				rest = rest[1:]
			}
			if len(rest) == 0 {
				break
			}
			cut := min(h, len(rest))
			for cut > 1 && cut < len(rest) && (rest[cut-1].head || rest[cut-2].head || rest[cut].note) {
				cut--
			}
			col := rest[:cut]
			for len(col) > 0 && col[len(col)-1].text == "" {
				col = col[:len(col)-1]
			}
			cols, rest = append(cols, col), rest[cut:]
		}
		if len(rest) == 0 {
			return cols
		}
	}
	return []helpBlock{lines}
}

// labelled lays out label and words pairs as a table: the words beside the label column,
// wrapping under themselves, where they have helpWordsMin cells; otherwise on the lines
// under the label. The label column is as wide as one of the labels up to labelCap,
// whichever takes the fewest lines (the widest among equals), so on a narrow help the
// long labels give their column to the words; a longer label has its words on the line
// below, in the words column. Labels arrive styled; the words are drawn dim.
func labelled(p pal, rows [][2]string, inner, labelCap int) helpBlock {
	var best helpBlock
	widths := map[int]bool{}
	for _, r := range rows {
		widths[xansi.StringWidth(r[0])] = true
	}
	for w := labelCap; w >= 0; w-- {
		if !widths[w] {
			continue
		}
		if out := labelledAt(p, rows, inner, w+gutter); best == nil || len(out) < len(best) {
			best = out
		}
	}
	if best == nil {
		best = labelledAt(p, rows, inner, gutter)
	}
	return best
}

// labelledAt is labelled with a label column labelW wide, its gutter included.
func labelledAt(p pal, rows [][2]string, inner, labelW int) helpBlock {
	dim := p.Text(p.Dim)
	var out helpBlock
	for _, r := range rows {
		if inner-labelW >= helpWordsMin {
			first := padRight(r[0], labelW)
			if xansi.StringWidth(r[0]) > labelW-gutter {
				out = append(out, helpLine{text: r[0]})
				first = strings.Repeat(" ", labelW)
			}
			for i, w := range strings.Split(xansi.Wrap(r[1], inner-labelW, ""), "\n") {
				lead := strings.Repeat(" ", labelW)
				if i == 0 {
					lead = first
				}
				out = append(out, helpLine{text: lead + dim.Render(w)})
			}
			continue
		}
		indent := strings.Repeat(" ", gutter)
		out = append(out, helpLine{text: r[0]})
		for _, w := range strings.Split(xansi.Wrap(r[1], max(1, inner-gutter), ""), "\n") {
			out = append(out, helpLine{text: indent + dim.Render(w)})
		}
	}
	return out
}

// aiSkill is one Plugin skill as the help names it: what it does, and the key that hands
// the Application under the cursor to it, "" for one the Owner runs in their Harness.
type aiSkill struct{ key, name, does string }

// aiSkills is every skill in plugin/skills, the ones with a key first. The words follow
// each SKILL.md's description.
var aiSkills = []aiSkill{
	{"f", skillAssessFit, "sets the Fit from ad and Profile"},
	{"p", skillPrepareDocuments, "resume, letter, gaps; b: PDFs"},
	{"F", skillDraftFollowUp, "drafts follow-up.md; never sends"},
	{"", "capture-job", "saves a job ad as a Lead"},
	{"", "find-jobs", "sweeps your Sources for Leads"},
	{"", "create-profile", "Profile from resume or interview"},
	{"", "import-applications", "brings in a CSV of past applications"},
	{"", "interview-prep", "prep.md from the ad and Profile"},
	{"", "answer-questions", "answers.md for form questions"},
	{"", "help", "answers questions about rolecall"},
}

// skillKeyWidth is the AI skills' key column: one key each, so narrower than the key
// sections', leaving the words room on one line.
const skillKeyWidth = 3

// skillsBlock is the AI skills section: what each Plugin skill does and which key hands
// an Application to it. Without a Harness it says how to connect one first, and the keys
// are dimmed as the Act column dims them.
func (m model) skillsBlock(p pal, inner int) helpBlock {
	dim := p.Text(p.Dim)
	ks := p.Key()
	var note string
	switch {
	case m.harness == "" && m.connectsHarness("h"):
		ks = dim
		note = dim.Render("connect a Harness first: ") + p.Key().Render("h") + dim.Render(" or Settings › "+groupMachineName)
	case m.harness == "":
		ks = dim
		note = dim.Render("connect a Harness first: Settings › " + groupMachineName)
	default:
		note = dim.Render("f p F hand the Application to your Harness; run the others in it")
	}
	rows := make([][2]string, len(aiSkills))
	for i, s := range aiSkills {
		rows[i] = [2]string{ks.Render(padRight(s.key, skillKeyWidth)) + p.Text(p.Fg).Render(s.name), s.does}
	}
	out := helpBlock{heading(p, "AI skills"), {text: note}}
	return append(out, labelled(p, rows, inner, inner)...)
}

// rowWordCap is the widest row word that sets the label column: the Fit values,
// "Strong Good Marginal Poor". A longer one has its meaning on the line below.
const rowWordCap = 25

// rowWords says what the words a list row can show mean, each drawn as the row draws it.
func rowWords(p pal, followUpDays, inner int) helpBlock {
	fitValues := []string{store.FitStrong, store.FitGood, store.FitMarginal, store.FitPoor}
	fits := make([]string, 0, len(fitValues))
	for _, f := range fitValues {
		fits = append(fits, p.Text(p.FitColor(f)).Render(f))
	}
	// Each word drawn as the row draws it (view.go meta): time in amber, a blocker ⚠ and
	// bold in the text colour, a visa fit green, a note muted.
	ok, warn, fg := p.Text(p.Ok), p.Text(p.Warn), p.Text(p.Fg)
	block := fg.Bold(true)
	rows := [][2]string{
		{fg.Render("posted 12d"), "how old the ad is"},
		{warn.Render(glyphWarn + " past the age limit"), "older than Rules › Max posting age"},
		{p.Text(p.Dim).Render("ad summary only"), "the saved ad is only a summary"},
		{ok.Render("contract fits visa cap"), "contract within your visa's limit"},
		{block.Render(glyphWarn + " 12-mo contract > 6-mo visa cap"), "contract runs past your visa's limit"},
		{ok.Render("sponsors visa"), "the ad offers visa sponsorship"},
		{block.Render(glyphWarn + " no sponsorship"), "the ad rules out visa sponsorship"},
		{warn.Render(glyphFollowUp + " 6w"), fmt.Sprintf("follow-up due: unchanged over %dd", followUpDays)},
		{strings.Join(fits, " "), "Fit from assess-fit; blank until run"},
		{fg.Render(tableDismissed), "view: dismissed by you; m moves back"},
		{fg.Render(tableFiltered), "view: dismissed by rules; restorable"},
	}
	return append(helpBlock{heading(p, "Row words")}, labelled(p, rows, inner, rowWordCap)...)
}

// glyphLegend says what each glyph means. It is generated from the glyph table the frame
// draws with (theme.Legend), so a glyph cannot be drawn with one meaning and explained
// with another; every glyph has exactly one. The Status glyphs are drawn in their own
// colours, as the list draws them. Items sit in cells of one width, legendPerLine to a
// line; where inner has no room for that many, each row packs its items instead.
func glyphLegend(p pal, followUpDays, inner int) helpBlock {
	dim := func(s string) string { return p.Text(p.Dim).Render(s) }
	labelW := helpKeyWidth + 1
	indent := strings.Repeat(" ", labelW)
	groups := map[string][]appearance.Glyph{}
	for _, g := range appearance.Legend() {
		groups[g.Group] = append(groups[g.Group], g)
	}
	items := func(group string) []string {
		var out []string
		for _, g := range groups[group] {
			switch g.Mark {
			case meterOn:
				// The ready meter's two cells read as one sample.
				out = append(out, dim(meterOn+meterOn+meterOff+" 2/3 ready"))
			case meterOff:
			case glyphFollowUp:
				out = append(out, dim(fmt.Sprintf("%s %s (%dd)", g.Mark, g.Meaning, followUpDays)))
			default:
				out = append(out, dim(g.Mark+" "+g.Meaning))
			}
		}
		return out
	}
	// The Timeline track keeps each Status's own glyph, so the legend cannot borrow one
	// for "current": it says in words what separates the stops instead.
	timeline := append([]string{p.Text(p.Fg).Bold(true).Render("bold") + dim(" current stop"), dim("dim: other stops")},
		items(appearance.GroupTimeline)...)
	tabled := [][2]any{
		{appearance.GroupTimeline, timeline},
		{appearance.GroupDocs, items(appearance.GroupDocs)},
		{appearance.GroupAgent, items(appearance.GroupAgent)},
		{appearance.GroupFlags, items(appearance.GroupFlags)},
		{appearance.GroupForms, items(appearance.GroupForms)},
	}
	cellW := helpColumn - gutter
	for _, r := range tabled {
		for _, it := range r[1].([]string) {
			cellW = max(cellW, xansi.StringWidth(it)+gutter)
		}
	}
	perLine := max(1, min(legendPerLine, (inner-labelW+gutter)/cellW))
	lead := func(label string, first bool) string {
		if first {
			return p.Text(p.Fg).Render(padRight(label, labelW))
		}
		return indent
	}
	var out helpBlock
	// row sets items in cells, perLine to a line, wrapping under itself.
	row := func(label string, list []string) {
		for start := 0; start < len(list); start += perLine {
			var b strings.Builder
			b.WriteString(lead(label, start == 0))
			chunk := list[start:min(len(list), start+perLine)]
			for i, it := range chunk {
				b.WriteString(it)
				if i < len(chunk)-1 {
					b.WriteString(strings.Repeat(" ", max(1, cellW-xansi.StringWidth(it))))
				}
			}
			out = append(out, helpLine{text: b.String()})
		}
	}
	// flow packs items two cells apart, as many to a line as fit: the Statuses, whose
	// words are short and many.
	flow := func(label string, list []string, first bool) {
		line := lead(label, first)
		used := 0
		for _, it := range list {
			w := xansi.StringWidth(it)
			if used > 0 && labelW+used+gutter+w > inner {
				out = append(out, helpLine{text: line})
				line, used = indent, 0
			}
			if used > 0 {
				line += "  "
				used += gutter
			}
			line += it
			used += w
		}
		out = append(out, helpLine{text: line})
	}
	var open, closed []string
	for _, g := range groups[appearance.GroupStatus] {
		it := p.StatusMark(g.Meaning) + dim(" "+g.Meaning)
		if closedStatus(g.Meaning) {
			closed = append(closed, it)
		} else {
			open = append(open, it)
		}
	}
	out = append(out, heading(p, "Glyphs"))
	flow(appearance.GroupStatus, open, true)
	flow(appearance.GroupStatus, closed, false)
	for _, r := range tabled {
		// Too narrow for legendPerLine cells, the rows pack like the Statuses: a row of
		// cells two to a line takes more lines than packing it.
		if perLine < legendPerLine {
			flow(r[0].(string), r[1].([]string), true)
			continue
		}
		row(r[0].(string), r[1].([]string))
	}
	return out
}

// helpPages cuts the blocks into pages of size lines. A block that would split but fits
// on a page starts a page of its own, unless dropping the spacer above it keeps it whole
// on this one or keeping blocks whole would take more pages than running them on; a
// heading never sits among a page's last lines without rows under it; a page never
// starts blank; and without spacers (a short page cannot spare them) blank lines are
// dropped.
func helpPages(blocks []helpBlock, size int, spacers bool) [][]string {
	whole := cutPages(blocks, size, spacers, true)
	if run := cutPages(blocks, size, spacers, false); len(run) < len(whole) {
		return run
	}
	return whole
}

// cutPages is helpPages with blocks kept whole where they fit on a page, or run on.
func cutPages(blocks []helpBlock, size int, spacers, whole bool) [][]string {
	var pages [][]string
	var page []string
	flush := func() {
		if len(page) > 0 {
			pages = append(pages, page)
			page = nil
		}
	}
	for i, b := range blocks {
		var lines helpBlock
		if i > 0 && spacers {
			lines = append(lines, helpLine{})
		}
		for _, l := range b {
			if spacers || l.text != "" {
				lines = append(lines, l)
			}
		}
		content := len(lines)
		if spacers && i > 0 {
			content-- // the spacer goes at a page's top
		}
		if len(page) > 0 && len(page)+len(lines) > size {
			switch {
			case len(lines) > content && len(page)+content <= size:
				lines = lines[1:] // the spacer goes rather than the block moving on
			case whole && content <= size:
				flush()
			}
		}
		for j, l := range lines {
			if len(page) == 0 && l.text == "" {
				continue
			}
			if l.head && len(page) > 0 && size-len(page) < min(helpHeadRoom, len(lines)-j) {
				flush()
			}
			page = append(page, l.text)
			if len(page) == size {
				flush()
			}
		}
	}
	flush()
	if len(pages) == 0 {
		pages = [][]string{{""}}
	}
	return pages
}

// wrapBlock wraps a block's lines to inner cells, as the pop-up would wrap them, so a
// page is never taller than the room for it. A wrapped heading stays a heading.
func wrapBlock(b helpBlock, inner int) helpBlock {
	var out helpBlock
	for _, l := range b {
		for i, part := range strings.Split(xansi.Wrap(l.text, inner, ""), "\n") {
			out = append(out, helpLine{text: part, head: l.head && i == 0})
		}
	}
	return out
}

// helpContent is the help's blocks at inner cells wide: the keys of the context it was
// opened from and the global keys, the AI skills, the glyph legend, then on a list the
// row words. Each is wrapped to inner, so a page is never taller than the room for it.
func (m model) helpContent(p pal, inner int, flow bool) []helpBlock {
	under := m
	under.overlay = nil
	keys := append(under.screenKeys(), under.dormantKeys()...)
	blocks := []helpBlock{keyBlock(p, keys, inner, flow), m.skillsBlock(p, inner), glyphLegend(p, m.followUpDays, inner)}
	if m.hasTables() {
		blocks = append(blocks, rowWords(p, m.followUpDays, inner))
	}
	for i := range blocks {
		blocks[i] = wrapBlock(blocks[i], inner)
	}
	return blocks
}

// helpPage cuts the help into pages of size lines and returns the open page's lines, its
// title ("Keys · Leads · page 1 of 2", the page only when there is a second) and the
// number of pages, which the open helpMode keeps for its paging keys. Every page is as
// tall as the first, so the help keeps its place as it pages.
func (m model) helpPage(blocks []helpBlock, size int, spacers bool) (lines []string, title string, pages int) {
	all := helpPages(blocks, max(1, size), spacers)
	page := 0
	if hm, _ := m.overlay.(*helpMode); hm != nil {
		hm.pages = len(all)
		hm.page = min(hm.page, len(all)-1)
		page = hm.page
	}
	lines, title = all[page], m.helpTitle()
	if len(all) > 1 {
		lines = append(lines, make([]string, max(0, len(all[0])-len(lines)))...)
		title += fmt.Sprintf(" · page %d of %d", page+1, len(all))
	}
	return lines, title, len(all)
}

// helpFullScreen reports whether the help takes the whole terminal rather than a pop-up:
// below the wide size class a pop-up between the frame's rules is too short to hold the
// help in a page or two.
func (m model) helpFullScreen() bool { return !m.layout().wide() }

// helpScreenOpen reports whether the full-screen help is open: View draws it as the whole
// frame instead of setting it over the screen.
func (m model) helpScreenOpen() bool {
	_, ok := m.overlay.(*helpMode)
	return ok && m.helpFullScreen()
}

// helpView is the help as its size class shows it: full-screen below wide, else the
// pop-up.
func (m model) helpView(p pal) string {
	if m.helpFullScreen() {
		return m.helpScreen(p)
	}
	return m.helpBox(p)
}

// helpScreen is the full-screen help: the page two cells in, then one footer line with
// the title on its left, where a list's breadcrumb sits, and the help's own keys on its
// right. Nothing of the screen beneath shows, so every other line is a page's.
func (m model) helpScreen(p pal) string {
	l := m.layout()
	size := max(1, l.h-1)
	inner, roomy := l.w-gutter-1, size >= helpRoomy // the last column stays clear, as on every screen
	// The key sections run on across columns only where that saves a page.
	blocks := m.helpContent(p, inner, false)
	if run := m.helpContent(p, inner, true); len(helpPages(run, size, roomy)) < len(helpPages(blocks, size, roomy)) {
		blocks = run
	}
	lines, title, pages := m.helpPage(blocks, size, roomy)
	indent := strings.Repeat(" ", gutter)
	for i, line := range lines {
		if line != "" {
			lines[i] = indent + line
		}
	}
	keys := p.KeyRow(helpModeKeys(pages))
	room := max(0, l.w-gutter-gutter-xansi.StringWidth(keys))
	footer := " " + spread(p.Text(p.Sub).Bold(true).Render(xansi.Truncate(title, room, "…")), keys, l.w-gutter)
	return fitLines(strings.Join(lines, "\n"), size) + "\n" + footer
}

// helpBox is the `?` pop-up on a wide terminal: the help paged into the body between
// the frame's rules. Paging appears only when there is a second page.
func (m model) helpBox(p pal) string {
	inner := popWidth(popLarge, m.viewWidth()) - popInset
	// A page fills the body between the rules, less the pop-up's borders and padding; a
	// page short of helpRoomy lines drops the padding rows as well as its spacers.
	const popChrome, borders = 4, 2
	size := m.layout().bodyHeight(m.contextLines()) - popChrome
	roomy := size >= helpRoomy
	if !roomy {
		size += popChrome - borders
	}
	lines, title, pages := m.helpPage(m.helpContent(p, inner, false), size, roomy)
	box := popup(p, title, false, popLarge, m.viewWidth(), strings.Join(lines, "\n"), p.Keys(helpModeKeys(pages)))
	if !roomy {
		// Short of room, the blank rows inside the borders go too: two more lines a page.
		rows := strings.Split(box, "\n")
		box = strings.Join(append(append(rows[:1:1], rows[2:len(rows)-2]...), rows[len(rows)-1]), "\n")
	}
	return box
}

// section is the keys of the given groups, highest priority first and registry order among
// equals.
func section(keys []helpKey, groups ...helpGroup) []helpKey {
	var out []helpKey
	for _, k := range keys {
		if slices.Contains(groups, k.group) {
			out = append(out, k)
		}
	}
	// Pinned keys (? help, esc back) close the list rather than lead it.
	rank := func(k helpKey) int {
		if k.prio == prioPinned {
			return -1
		}
		return k.prio
	}
	slices.SortStableFunc(out, func(a, b helpKey) int { return rank(b) - rank(a) })
	return out
}

// helpRow is one row of a `?` pop-up section: its keys, what they do, and whether it is
// drawn dimmed.
type helpRow struct {
	key, desc string
	dim       bool
}

// mergeAliases is the pop-up's rows: keys that do the same thing (: and ctrl+k open the
// command palette) share one row, ": ctrl+k  commands", at the first one's place.
func mergeAliases(keys []helpKey) []helpRow {
	var rows []helpRow
	at := map[string]int{}
	for _, k := range keys {
		h := k.Help()
		if i, ok := at[h.Desc]; ok {
			// The shorter key leads, ": ctrl+k", whichever the section ranked first.
			if len(h.Key) < len(rows[i].key) {
				rows[i].key = h.Key + " " + rows[i].key
			} else {
				rows[i].key += " " + h.Key
			}
			continue
		}
		at[h.Desc] = len(rows)
		rows = append(rows, helpRow{key: h.Key, desc: h.Desc, dim: k.dim})
	}
	return rows
}

// helpModeKeys is the `?` pop-up's own keys: paging only when there is more than a page.
func helpModeKeys(pages int) [][2]string {
	if pages > 1 {
		return [][2]string{{"↑↓", "page"}, {"esc", hintClose}}
	}
	return [][2]string{{"esc", hintClose}}
}
