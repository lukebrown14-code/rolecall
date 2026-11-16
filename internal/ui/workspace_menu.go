package ui

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

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

// menuKeyWidth is the menu's key column, as wide as the palette's, so the labels line up
// after it.
const menuKeyWidth = helpKeyWidth - gutter

// menuItem is one row of the Workspace menu: the direct key that does the same thing,
// and the label, the registry's name for that key here.
//
// runs reports whether the key runs this item's command on the current screen. A letter
// means different commands on different screens (a adds a Track in Settings › Tracking,
// r renames one), so an item is matched to its command, never to its letter alone. A
// screen digit has no runs: every screen accepts it.
type menuItem struct {
	key, label string
	runs       func(model) bool
	// row is true for the This row group, drawn under its own heading.
	row bool
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

// menuActions are the commands the menu offers after the screens. An item joins the menu
// only where its key runs that command, so the menu cannot offer a key that does nothing
// there, or something else; its label is the registry's name for the key.
var menuActions = []menuItem{
	{key: "a", runs: addsApplication},
	{key: "/", runs: searchesApplications},
	{key: "s", runs: switchesView},
	{key: "t", runs: model.trackFilterApplies},
	{key: "r", runs: registered("r", "rescan")},
	{key: ":", runs: registered(":", "commands")},
	{key: "?", runs: registered("?", "help")},
}

// menuRowGroup heads the menu's row actions.
const menuRowGroup = "This row"

// rowMenuItems is the This row group: the row's main actions (open, move, dismiss, undo,
// edit, notes) from the one action table, in the order the help's Act column lists them.
func (m model) rowMenuItems(taken map[string]bool) []menuItem {
	if !m.hasTables() || !onRow(m) {
		return nil
	}
	var out []menuItem
	for _, k := range section(m.actionKeys(), groupAction) {
		key := k.Help().Key
		if k.prio < prioMain || taken[key] || k.press == "" {
			continue
		}
		taken[key] = true
		out = append(out, menuItem{key: key, label: sentence(k.Help().Desc), row: true})
	}
	return out
}

// sentence is a key's description as a menu item: its first letter upper case.
func sentence(desc string) string {
	if desc == "" {
		return ""
	}
	r, n := utf8.DecodeRuneInString(desc)
	return string(unicode.ToUpper(r)) + desc[n:]
}

// workspaceMenu is the menu `space` opens: the screens, the commands that cut across
// them and the row's own actions, each with its direct key. Running an item sends that
// key, as the command palette does, so the item behaves exactly as the key.
type workspaceMenu struct {
	items []menuItem
	sel   int
}

// openMenu builds the menu for the current screen.
func (m model) openMenu() *workspaceMenu {
	items := make([]menuItem, 0, len(screenDefs)+len(menuActions))
	taken := map[string]bool{}
	for i, d := range screenDefs {
		items = append(items, menuItem{label: d.name, key: strconv.Itoa(i + 1)})
	}
	names := map[string]string{}
	for _, k := range m.screenKeys() {
		if _, ok := names[k.Help().Key]; !ok {
			names[k.Help().Key] = k.Help().Desc
		}
	}
	for _, it := range menuActions {
		if it.runs(m) && names[it.key] != "" {
			it.label = sentence(names[it.key])
			taken[it.key] = true
			items = append(items, it)
		}
	}
	return &workspaceMenu{items: append(items, m.rowMenuItems(taken)...)}
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

// View is the pop-up template, centred like every pop-up: the title in the top border,
// the keys in the bottom one, two cells of padding either side, and the one selection
// gutter as the cursor. Each item is key first, in a fixed key column, as the help and
// the palette draw them; the row actions sit under their own heading.
func (wm *workspaceMenu) View(m *model, p pal) string {
	cw := popWidth(menuWidth, m.viewWidth()) - popInset
	room := max(0, cw-menuLead-menuKeyWidth)
	var lines []string
	for i, it := range wm.items {
		if it.row && (i == 0 || !wm.items[i-1].row) {
			lines = append(lines, "", strings.Repeat(" ", menuLead)+p.Heading(menuRowGroup))
		}
		key := padRight(it.key, menuKeyWidth)
		label := xansi.Truncate(it.label, room, "…")
		if i == wm.sel {
			lines = append(lines, p.SelLine(true, cw, p.SelGutter(true)+p.Key().Background(p.Sel).Render(key)+
				p.SelText(true).Render(label)))
			continue
		}
		lines = append(lines, strings.Repeat(" ", menuLead)+p.Key().Render(key)+p.Text(p.Fg).Render(label))
	}
	return popup(p, menuTitle, false, menuWidth, m.viewWidth(), strings.Join(lines, "\n"),
		p.Keys([][2]string{{"enter", menuHintRun}, {"esc", hintClose}}))
}
