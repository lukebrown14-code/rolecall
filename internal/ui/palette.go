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

// paletteMode is the command palette (`:` or ctrl+k): every action the keymap registry
// offers where it was opened, filtered as the Owner types. Enter runs the action by
// sending its key, so an action behaves exactly as if its key were pressed.
type paletteMode struct {
	query string
	sel   int
	items []helpKey
}

// openPalette lists the runnable keys of the current context and the global ones, then
// one entry per other screen: the registry's "1-6 screen" names a range, which the palette
// cannot send, so each screen gets its own "Stats screen  4".
func (m model) openPalette() *paletteMode {
	var items []helpKey
	for _, k := range m.screenKeys() {
		if k.press != "" && k.press != ":" {
			items = append(items, k)
		}
	}
	for i, d := range screenDefs {
		if i != m.screen || m.detail != nil {
			items = append(items, help(groupGlobal, strconv.Itoa(i+1), d.name+" screen"))
		}
	}
	return &paletteMode{items: items}
}

func (pm *paletteMode) Keys() [][2]string {
	return [][2]string{{"↑↓", "select"}, {"enter", "run"}, {"esc", hintClose}}
}

// matches is the items the query fits, best first: a description starting with it, then
// one containing it, then the key itself, then the letters in order anywhere.
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
		desc := strings.ToLower(k.Help().Desc)
		switch {
		case strings.HasPrefix(desc, q):
			out = append(out, scored{k, 0})
		case strings.Contains(desc, q):
			out = append(out, scored{k, 1})
		case strings.ToLower(k.Help().Key) == q:
			out = append(out, scored{k, 2})
		case subsequence(desc, q):
			out = append(out, scored{k, 3})
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
			press := found[min(pm.sel, len(found)-1)].press
			return nil, func() tea.Msg { return keyPress(press) }
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

// View is the query in the title and the matching actions, the selected one marked.
func (pm *paletteMode) View(m *model, p pal) string {
	found := pm.matches()
	limit := max(1, m.layout().bodyHeight(m.contextLines())-paletteChrome)
	start := max(0, min(pm.sel-limit+1, len(found)-limit))
	var lines []string
	for i := start; i < min(len(found), start+limit); i++ {
		kv := found[i].pair()
		if i == pm.sel {
			lines = append(lines, p.SelLine(true, popWidth(popSmall, m.viewWidth())-popInset+gutter,
				p.SelGutter(true)+p.SelText(true).Render(padRight(kv[0], helpKeyWidth-gutter)+kv[1])))
			continue
		}
		lines = append(lines, "  "+p.Text(p.Fg).Bold(true).Render(padRight(kv[0], helpKeyWidth-gutter))+p.Text(p.Dim).Render(kv[1]))
	}
	if len(lines) == 0 {
		lines = []string{p.Text(p.Dim).Render("No command matches \"" + pm.query + "\"")}
	}
	return popup(p, ": "+pm.query+"▏", false, popSmall, m.viewWidth(), strings.Join(lines, "\n"), p.Keys(pm.Keys()))
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
