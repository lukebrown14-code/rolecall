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

// The menu's key hints. "select", not "move": m moves an Application.
const (
	menuHintMove = "select"
	menuHintRun  = "run"
)

// menuItem is one row of the Workspace menu: a label and the direct key that does the
// same thing.
//
// runs reports whether the key runs this item's command on the current screen. A letter
// means different commands on different screens (a adds a Track in Settings › Tracking,
// r renames one), so an item is matched to its command, never to its letter alone. A
// screen digit has no runs: every screen accepts it.
type menuItem struct {
	label, key string
	runs       func(model) bool
}

// addsApplication reports whether a adds an Application here: an Application list, or
// Stats.
func addsApplication(m model) bool {
	return m.detail == nil && (m.hasTables() || m.screen == screenIndex(screenStats))
}

// searchesApplications reports whether / opens the search here: a list with rows in it.
func searchesApplications(m model) bool {
	return m.hasTables() && m.detail == nil && !m.welcoming()
}

// switchesView reports whether s cycles the view here: a list whose screen has several.
func switchesView(m model) bool {
	return m.hasTables() && m.detail == nil && len(m.views()) > 1
}

// registered reports whether the key the registry lists first for key here, the one the
// key runs, has this description: r rescan, : commands and ? help are told apart from a
// screen's own use of the letter by their wording.
func registered(key, desc string) func(model) bool {
	return func(m model) bool {
		for _, k := range m.screenKeys() {
			if k.Help().Key == key {
				return k.Help().Desc == desc
			}
		}
		return false
	}
}

// menuActions are the commands the menu offers after the screens, with the menu's own
// wording for each, sentence case. An item joins the menu only where its key runs that
// command, so the menu cannot offer a key that does nothing there, or something else.
var menuActions = []menuItem{
	{"Add Application", "a", addsApplication},
	{"Search", "/", searchesApplications},
	{"Switch view", "s", switchesView},
	{"Track filter", "t", model.trackFilterApplies},
	{"Rescan", "r", registered("r", "rescan")},
	{"Command palette", ":", registered(":", "commands")},
	{"Help", "?", registered("?", "help")},
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
		items = append(items, menuItem{label: d.name, key: strconv.Itoa(i + 1)})
	}
	for _, it := range menuActions {
		if it.runs(m) {
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
