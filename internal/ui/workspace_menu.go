package ui

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	xansi "github.com/charmbracelet/x/ansi"
)

// menuWidth is the Workspace menu's box width, in cells: room for its bottom border's
// "enter run · esc close".
const menuWidth = 28

// menuLead is the cells before each menu label: the selection gutter "▌ ", as every list
// draws its cursor.
const menuLead = 2

// menuTitle names the Workspace menu, set into its top border like every pop-up's title.
const menuTitle = "Menu"

// The menu's key hints.
const (
	menuHintMove = "move"
	menuHintRun  = "run"
)

// menuItem is one row of the Workspace menu: a label and the direct key that does the
// same thing.
type menuItem struct{ label, key string }

// menuActions are the registry keys the menu offers after the screens, with the menu's
// own wording for each. A key joins the menu only when the keymap registry lists it for
// the screen the menu was opened from, so the menu cannot offer a key that does nothing
// there or drift from the keys the footer and Help show.
var menuActions = []menuItem{
	{"Add Application", "a"},
	{"Search", "/"},
	{"Status filter", "s"},
	{"Track filter", "t"},
	{"Command palette", ":"},
	{"Help", "?"},
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
		items = append(items, menuItem{d.name, strconv.Itoa(i + 1)})
	}
	listed := map[string]bool{}
	for _, k := range m.screenKeys() {
		listed[k.Help().Key] = true
	}
	for _, it := range menuActions {
		// The registry labels the Track filter "t"; the palette key is global.
		if listed[it.key] {
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
	}
	return wm, nil
}

// anchored: the menu sits top-left under the tab bar, not centred.
func (wm *workspaceMenu) anchored() bool { return true }

// View is the pop-up template: the title in the top border, the keys in the bottom one,
// two cells of padding either side, and the one selection gutter as the cursor.
func (wm *workspaceMenu) View(m *model, p pal) string {
	cw := popWidth(menuWidth, m.viewWidth()) - popInset
	lines := make([]string, len(wm.items))
	for i, it := range wm.items {
		// The gutter before the selected label, two spaces before the rest, so every label
		// starts in the same column and every key in the column before the padding.
		room := max(0, cw-menuLead-1-xansi.StringWidth(it.key))
		label := padRight(xansi.Truncate(it.label, room, "…"), room)
		if i == wm.sel {
			lines[i] = p.SelLine(true, cw, p.SelGutter(true)+p.SelText(true).Render(label+" ")+
				p.Key().Background(p.Sel).Render(it.key))
			continue
		}
		lines[i] = strings.Repeat(" ", menuLead) + p.Text(p.Fg).Render(label) + " " + p.Key().Render(it.key)
	}
	return popup(p, menuTitle, false, menuWidth, m.viewWidth(), strings.Join(lines, "\n"),
		p.Keys([][2]string{{"enter", menuHintRun}, {"esc", hintClose}}))
}

// anchoredMode is a mode whose box sits top-left under the tab bar.
type anchoredMode interface{ anchored() bool }
