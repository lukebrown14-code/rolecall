package ui

import (
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
)

// paletteChrome is the pop-up's borders and padding around its list.
const paletteChrome = 4

// paletteMode is the command finder (space, `:` or ctrl+k): every action the keymap registry
// offers where it was opened, filtered as the Owner types. Enter runs the action by
// sending its key, so an action behaves exactly as if its key were pressed.
type paletteMode struct {
	query string
	sel   int
	items []helpKey
}

// opensFinder reports whether a key press is one of the command finder's own keys, which
// the finder does not list.
func opensFinder(press string) bool { return press == ":" || press == "ctrl+k" || press == spaceKey }

// openPalette lists the runnable keys of the current context and the global ones, then
// one entry per other screen: the registry's "1-6 screen" names a range, which the palette
// cannot send, so each screen gets its own "Stats screen  4".
func (m model) openPalette() *paletteMode {
	var items []helpKey
	for _, k := range m.screenKeys() {
		if k.press != "" && !opensFinder(k.press) {
			items = append(items, k)
		}
	}
	items = append(items, m.editFieldItems()...)
	for i, d := range screenDefs {
		if i != m.screen || m.detail != nil {
			items = append(items, help(groupGlobal, strconv.Itoa(i+1), d.name+" screen"))
		}
	}
	return &paletteMode{items: items}
}

// editFieldItems are the palette's ways into one field of the Edit form, which e opens on
// its first field: the next step, the first Contact and, on a Rejected or Withdrawn
// Application, the close reason. They have no key of their own, so the key column is
// blank: e opens the same form, under its own one name, edit.
func (m model) editFieldItems() []helpKey {
	a, ok := m.current()
	if !m.hasTables() || !ok || a.Malformed != nil {
		return nil
	}
	contact := "add contact"
	if len(a.Contacts) > 0 {
		contact = "edit contact"
	}
	fields := [][2]string{{fieldNextStep, "edit next step"}, {fieldContact, contact}}
	if endedStatus(a.Status) {
		fields = append(fields, [2]string{fieldCloseReason, "set close reason"})
	}
	items := make([]helpKey, 0, len(fields))
	for _, f := range fields {
		k := help(groupAction, "", f[1])
		k.field = f[0]
		items = append(items, k)
	}
	return items
}

// editFieldMsg opens the Edit form on the row under the cursor, focused on one field: what
// a palette entry for that field sends.
type editFieldMsg struct{ field string }

// openEditAt is the Edit form on the current row with field focused.
func (m *model) openEditAt(field string) tea.Cmd {
	a, ok := m.current()
	if !ok {
		return nil
	}
	cmd := m.openEdit(a)
	if fm, ok := m.overlay.(*formMode); ok {
		for i := range fm.form.Fields {
			if fm.form.Fields[i].Label == field {
				fm.form.Focus = i
			}
		}
	}
	return cmd
}

func (pm *paletteMode) Keys() [][2]string {
	return [][2]string{{"↑↓", "select"}, {"enter", "run"}, {"esc", hintClose}}
}

// matches is the items the query fits, best first: a name starting with it, then a word
// of the name starting with it, then a name containing it, then the key itself, then the
// letters in order anywhere. The name is the description without "(needs a Harness)", so
// typing "harn" finds connect a Harness before the keys that need one.
func (pm *paletteMode) matches() []helpKey {
	q := strings.ToLower(pm.query)
	if q == "" {
		return pm.items
	}
	type scored struct {
		k     helpKey
		score int
	}
	var out []scored
	for _, k := range pm.items {
		name := strings.ToLower(strings.TrimSpace(strings.TrimSuffix(k.Help().Desc, needsHarnessNote)))
		switch {
		case strings.HasPrefix(name, q):
			out = append(out, scored{k, 0})
		case strings.Contains(" "+name, " "+q):
			out = append(out, scored{k, 1})
		case strings.Contains(name, q):
			out = append(out, scored{k, 2})
		case strings.ToLower(k.Help().Key) == q:
			out = append(out, scored{k, 3})
		case subsequence(name, q):
			out = append(out, scored{k, 4})
		}
	}
	slices.SortStableFunc(out, func(a, b scored) int { return a.score - b.score })
	keys := make([]helpKey, len(out))
	for i, s := range out {
		keys[i] = s.k
	}
	return keys
}

// subsequence reports whether q's letters appear in s in order.
func subsequence(s, q string) bool {
	for _, r := range q {
		i := strings.IndexRune(s, r)
		if i < 0 {
			return false
		}
		s = s[i+utf8.RuneLen(r):]
	}
	return true
}

func (pm *paletteMode) Update(m *model, msg tea.Msg) (mode, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.PasteMsg:
		pm.query += pasted(msg.Content)
		pm.sel = 0
	case tea.KeyPressMsg:
		found := pm.matches()
		switch msg.String() {
		case "esc":
			return nil, nil
		case "enter":
			if len(found) == 0 {
				return pm, nil
			}
			k := found[min(pm.sel, len(found)-1)]
			if k.field != "" {
				return nil, func() tea.Msg { return editFieldMsg{field: k.field} }
			}
			return nil, func() tea.Msg { return keyPress(k.press) }
		case "down", "ctrl+n":
			pm.sel = min(pm.sel+1, max(0, len(found)-1))
		case "up", "ctrl+p":
			pm.sel = max(pm.sel-1, 0)
		case keyBackspace:
			pm.query = dropLastGrapheme(pm.query)
			pm.sel = 0
		default:
			if msg.Text != "" {
				pm.query += msg.Text
				pm.sel = 0
			}
		}
	}
	return pm, nil
}

// View is "Commands" and the query in the title and the matching actions, the selected
// one marked. When they do not all fit, a "↑ N more" or "↓ N more" line takes the place of
// a row at the end that has more.
func (pm *paletteMode) View(m *model, p pal) string {
	found := pm.matches()
	limit := max(1, m.layout().bodyHeight(m.contextLines())-paletteChrome)
	rows := limit
	if len(found) > limit && limit > 1 {
		rows = limit - 1 // one line for a cue
	}
	window := func() int { return max(0, min(pm.sel-rows+1, len(found)-rows)) }
	start := window()
	if start > 0 && start+rows < len(found) && rows > 1 {
		rows-- // more above and below: a line for each cue
		start = window()
	}
	end := min(len(found), start+rows)
	var lines []string
	cues := rows < limit // a one-line list has no room for one
	if start > 0 && cues {
		lines = append(lines, "  "+p.MoreCue(true, start))
	}
	for i := start; i < end; i++ {
		kv := found[i].pair()
		if i == pm.sel {
			lines = append(lines, p.SelLine(true, popWidth(popSmall, m.viewWidth())-popInset+gutter,
				p.SelGutter(true)+p.SelText(true).Render(padRight(kv[0], helpKeyWidth-gutter)+kv[1])))
			continue
		}
		// Keys drawn as the help and the key line draw them; a key that needs a Harness is
		// dimmed with its words.
		ks := p.Key()
		if found[i].dim {
			ks = p.Text(p.Dim)
		}
		lines = append(lines, "  "+ks.Render(padRight(kv[0], helpKeyWidth-gutter))+p.Text(p.Dim).Render(kv[1]))
	}
	if below := len(found) - end; below > 0 && cues {
		lines = append(lines, "  "+p.MoreCue(false, below))
	}
	if len(lines) == 0 {
		lines = []string{p.Text(p.Dim).Render("No command matches \"" + pm.query + "\"")}
	}
	return popup(p, "Commands: "+pm.query+"▏", false, popSmall, m.viewWidth(), strings.Join(lines, "\n"), p.Keys(pm.Keys()))
}

// keyPress is the message a key name stands for, as the program would receive it.
func keyPress(name string) tea.KeyPressMsg {
	switch name {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "shift+tab":
		return tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	case keyBackspace:
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	case spaceKey:
		return tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
	}
	r, _ := utf8.DecodeRuneInString(name)
	return tea.KeyPressMsg{Code: r, Text: name}
}
