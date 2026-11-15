package ui

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	xansi "github.com/charmbracelet/x/ansi"
)

// menuWidth is the Workspace menu's box width, in cells.
const menuWidth = 26

// menuLead is the cells before each menu label: the cursor "▌ ▸ " on the selected row.
const menuLead = 4

// menuTitle names the Workspace menu; its top border also carries "esc".
const menuTitle = "Menu"

// The menu's key hints. "select", not "move": m moves an Application.
const (
	menuHintMove = "select"
	menuHintRun  = "run"
)

// menuItem is one row of the Workspace menu: a label and the direct key that does the
// same thing. desc, when set, is the registry description the key must have here: r is
// rescan on most screens but renames a Track in Settings.
type menuItem struct{ label, key, desc string }

// menuActions are the registry keys the menu offers after the screens, with the menu's
// own wording for each, sentence case. A key joins the menu only when the keymap registry
// lists it for the screen the menu was opened from, so the menu cannot offer a key that
// does nothing there or drift from the keys the footer, Help and the palette show.
var menuActions = []menuItem{
	{"Add Application", "a", ""},
	{"Search", "/", ""},
	{"Switch view", "s", "view"},
	{"Track filter", "t", ""},
	{"Rescan", "r", "rescan"},
	{"Command palette", ":", ""},
	{"Help", "?", ""},
}

// workspaceMenu is the menu `space` opens: the screens and the commands that cut across
// them, each with its direct key. Running an item sends that key, as the command
// palette does, so the item behaves exactly as the key.
type workspaceMenu struct {
	items []menuItem
	sel   int
}

// openMenu builds the menu for the current screen.
func (m model) openMenu() *workspaceMenu {
	items := make([]menuItem, 0, len(screenDefs)+len(menuActions))
	for i, d := range screenDefs {
		items = append(items, menuItem{d.name, strconv.Itoa(i + 1), ""})
	}
	listed := map[string]string{}
	for _, k := range m.screenKeys() {
		if _, seen := listed[k.Help().Key]; !seen {
			listed[k.Help().Key] = k.Help().Desc
		}
	}
	for _, it := range menuActions {
		if desc, ok := listed[it.key]; ok && (it.desc == "" || it.desc == desc) {
			items = append(items, it)
		}
	}
	return &workspaceMenu{items: items}
}

func (wm *workspaceMenu) Keys() [][2]string {
	return [][2]string{{"↑↓", menuHintMove}, {"enter", menuHintRun}, {"esc", hintClose}}
}

func (wm *workspaceMenu) Update(m *model, msg tea.Msg) (mode, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return wm, nil
	}
	switch k.String() {
	case "esc":
		return nil, nil
	case "down", "j":
		wm.sel = min(wm.sel+1, len(wm.items)-1)
	case "up", "k":
		wm.sel = max(wm.sel-1, 0)
	case "enter":
		press := wm.items[wm.sel].key
		return nil, func() tea.Msg { return keyPress(press) }
	case " ", spaceKey, "q":
		return nil, nil // space toggles the menu closed
	default:
		// Each row shows its key, so the key fires from inside the menu too.
		for _, it := range wm.items {
			if it.key == k.String() {
				press := it.key
				return nil, func() tea.Msg { return keyPress(press) }
			}
		}
	}
	return wm, nil
}

// anchored: the menu sits top-left under the tab bar, not centred.
func (wm *workspaceMenu) anchored() bool { return true }

func (wm *workspaceMenu) View(m *model, p pal) string {
	cw := popWidth(menuWidth, m.viewWidth()) - popInset + gutter
	lines := make([]string, len(wm.items))
	for i, it := range wm.items {
		// "▌ ▸ " before the selected label, four spaces before the rest, so every label
		// starts in the same column.
		room := max(0, cw-menuLead-1-xansi.StringWidth(it.key))
		label := padRight(xansi.Truncate(it.label, room, "…"), room)
		if i == wm.sel {
			mark := p.SelBase(true).Foreground(p.Accent).Bold(true).Render("▌") + p.SelText(true).Render(" ▸ ")
			lines[i] = p.SelLine(true, cw, mark+p.SelText(true).Render(label+" "+it.key))
			continue
		}
		lines[i] = strings.Repeat(" ", menuLead) + p.Text(p.Fg).Render(label) + " " + p.Key().Render(it.key)
	}
	return popupNote(p, menuTitle, "esc", false, menuWidth, m.viewWidth(), strings.Join(lines, "\n"), p.Keys([][2]string{{"↑↓", menuHintMove}, {"enter", menuHintRun}}))
}

// anchoredMode is a mode whose box sits top-left under the tab bar.
type anchoredMode interface{ anchored() bool }
