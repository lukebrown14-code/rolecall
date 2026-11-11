package ui

import (
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/store"
)

// helpGroup places a key in the layout: the key line's left (actions) or right
// (navigation), or only the `?` pop-up (more).
type helpGroup int

// The `?` pop-up's layout: key column, then column width.
const (
	helpKeyWidth = 9
	helpColumn   = 23 // three fit side by side from 80 columns
	helpColumns  = 3
	keyLineGap   = 4 // the key line's indent plus the space kept between its halves
)

// pinnedKeys stay on a narrow key line after every other action has gone: they are
// how a newcomer finds their way in.
var pinnedKeys = []string{"enter", "a", "/"}

const (
	groupAction helpGroup = iota
	groupNav
	groupMore
	// groupCursor appears in the `?` pop-up's Navigate column but not on the key
	// line: moving the cursor is second nature, so it does not crowd the line.
	groupCursor
)

type helpKey struct {
	key.Binding
	group helpGroup
}

func help(group helpGroup, k, desc string) helpKey {
	return helpKey{key.NewBinding(key.WithKeys(k), key.WithHelp(k, desc)), group}
}

func (k helpKey) pair() [2]string {
	h := k.Help()
	return [2]string{h.Key, h.Desc}
}

// overlayOpen reports whether a form, popup, prompt or search box owns the keyboard.
func (m model) overlayOpen() bool {
	return m.form != nil || m.duplicate != nil || len(m.options) > 0 ||
		m.confirm != nil || m.pendingDownload != nil || m.buildFail != nil || m.filtering
}

// boxOpen reports whether a pop-up whose bottom border carries its own keys is open.
// The key line then shows context instead of repeating those keys. The help pop-up is
// excluded: its border says "any key close", so the key line keeps the view's keys.
func (m model) boxOpen() bool {
	return m.form != nil || m.duplicate != nil || len(m.options) > 0 ||
		m.confirm != nil || m.pendingDownload != nil
}

// viewContext names where the Owner is, for the key line while a pop-up holds the
// keyboard: the screen, its view when it has more than one, and the Application under it.
func (m model) viewContext() string {
	label := screens[m.screen]
	if ts := tables[m.screen]; len(ts) > 1 && ts[m.table] != label {
		label += " · " + ts[m.table]
	}
	if a, ok := m.current(); ok {
		label += " · " + a.Title
	}
	return label
}

// helpKeys is the one key map behind the key line and the `?` pop-up: only the keys
// that do something in the current context. An open overlay owns the keyboard, so it
// offers its own keys and nothing from the view beneath it.
func (m model) helpKeys() []helpKey {
	if m.overlayOpen() {
		var keys []helpKey
		for _, kv := range m.overlayHints() {
			group := groupAction
			if strings.HasPrefix(kv[0], "esc") || strings.HasSuffix(kv[0], "esc") {
				group = groupNav
			}
			keys = append(keys, help(group, kv[0], kv[1]))
		}
		return keys
	}
	a, onRow := m.current()
	// A malformed row can only be inspected: Edit, Move, undo, notes and downloads are refused there.
	usable := !onRow || a.Malformed == nil
	var keys []helpKey
	if m.detail != nil {
		keys = append(keys, help(groupAction, "←→", "tab"), help(groupAction, "↑↓", "scroll"))
		if usable {
			keys = append(keys, help(groupAction, "e", "edit file"))
		}
	} else if onRow {
		keys = append(keys, help(groupAction, "enter", "open"))
		if usable {
			keys = append(keys, help(groupAction, "e", "edit"))
		}
	}
	if onRow && usable {
		keys = append(keys, help(groupAction, "m", "move"))
	}
	if usable && len(m.undo) > 0 {
		keys = append(keys, help(groupAction, "u", "undo"))
	}
	if onRow && usable {
		if m.detail == nil && (a.Status == store.StatusLead || a.Status == store.StatusDraft) {
			keys = append(keys, help(groupAction, "x", "dismiss"))
		}
		keys = append(keys, help(groupAction, "n", "notes"))
		if m.plugin != "" && buildable(a) {
			keys = append(keys, help(groupAction, "b", "build"))
		}
		if len(a.PDFs) > 0 {
			keys = append(keys, help(groupAction, "d", "download"))
		}
	}
	// Handoff: f on any Application, p while it is still a Lead or Draft.
	if onRow && m.harness != "" {
		keys = append(keys, help(groupAction, "f", "assess fit"))
		if usable && (a.Status == store.StatusLead || a.Status == store.StatusDraft) {
			keys = append(keys, help(groupAction, "p", "prepare docs"))
		}
	}
	if m.detail != nil {
		return append(keys, help(groupNav, "esc", "back"), help(groupNav, "?", "all keys"), help(groupMore, "q", "quit"))
	}
	keys = append(keys, help(groupAction, "a", "add"), help(groupAction, "/", "search"), help(groupNav, "1-3", "screen"))
	if len(tables[m.screen]) > 1 {
		keys = append(keys, help(groupNav, "tab", "view"))
	}
	keys = append(keys, help(groupNav, "?", "all keys"))
	if len(m.tracks) > 0 {
		keys = append(keys, help(groupMore, "t", "cycle track"))
	}
	return append(keys, help(groupCursor, "j k ↑↓", "cursor"), help(groupCursor, "g G", "top / bottom"),
		help(groupMore, "esc", "clear search"), help(groupMore, "r", "rescan"), help(groupMore, "q", "quit"))
}

func pairs(keys []helpKey, group helpGroup) [][2]string {
	var out [][2]string
	for _, k := range keys {
		if k.group == group {
			out = append(out, k.pair())
		}
	}
	return out
}

// keyLine is the bottom line: actions on the left, navigation on the right and never
// cut. When narrow, actions drop from the right, pinnedKeys last; "? all keys" on the
// right says where the rest went.
func (m model) keyLine(p pal, w int) string {
	if m.boxOpen() {
		// The pop-up's bottom border already lists its keys; the key line gives context.
		return " " + p.text(p.dim).Render(xansi.Truncate(m.viewContext(), max(0, w-gutter), "…"))
	}
	keys := m.helpKeys()
	actions, nav := pairs(keys, groupAction), p.keys(pairs(keys, groupNav))
	fits := func() bool { return xansi.StringWidth(p.keys(actions))+xansi.StringWidth(nav)+keyLineGap <= w }
	for i := len(actions) - 1; i >= 0 && !fits(); i-- {
		if !slices.Contains(pinnedKeys, actions[i][0]) {
			actions = slices.Delete(actions, i, i+1)
		}
	}
	for len(actions) > 0 && !fits() {
		actions = actions[:len(actions)-1]
	}
	return " " + spread(p.keys(actions), nav, w-gutter)
}

// helpBox is the `?` pop-up: every key valid here, in columns.
func (m model) helpBox(p pal) string {
	keys := m.helpKeys()
	col := func(title string, list [][2]string) string {
		var b strings.Builder
		b.WriteString(p.heading(title) + "\n")
		for _, kv := range list {
			b.WriteString(p.text(p.accent).Bold(true).Render(padRight(kv[0], helpKeyWidth)) + p.text(p.fg).Render(kv[1]) + "\n")
		}
		return strings.TrimSuffix(b.String(), "\n")
	}
	nav := append(pairs(keys, groupNav), pairs(keys, groupCursor)...)
	cols := []string{col("Actions", pairs(keys, groupAction)), col("Navigate", nav), col("More", pairs(keys, groupMore))}
	legend := p.heading("Glyphs") + "\n" +
		p.text(p.dim).Render("● ready · ○ written · ✓ present · ⚠ warning") + "\n" +
		p.text(p.dim).Render("✕ rejected or error · ↩ withdrawn · ~ approx · ⚑ follow-up due")
	body := joinColumns(cols, helpColumn, popWidth(popLarge, m.viewWidth())-popInset >= helpColumns*helpColumn) + "\n\n" + legend
	return popup(p, "Keys", false, popLarge, m.viewWidth(), body, p.keys([][2]string{{"any key", "close"}}))
}
