package ui

import (
	"image/color"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/store"
)

// profilePath is the one file the Profile screen reads, named in its context row and
// in the missing-Profile message.
const profilePath = "profile/master-profile.md"

// noProfile is how a Workspace without a Profile reads.
const noProfile = "No Profile yet: " + profilePath + " not found"

// The two panes tab moves between.
const (
	profileList = iota
	profileContent
	profilePanes
)

// profileMarker is the selected section's leading mark: bold and accent carry it too,
// so it stays readable without colour.
const profileMarker = "▸ "

// profileMarkerWidth is the cells the marker, or its indent on unselected rows, takes.
const profileMarkerWidth = 2

// Contact's mark stands in for a count: ✓ when Name and Email are both present, ✗ when
// either is missing. The store's Missing decides; the screen never parses Contact again.
const (
	contactOK  = "✓"
	contactBad = "✗"
)

// Profile screen sizing: the section list, the divider, and the gutter before the
// Markdown. The list is a side panel (layout.go) so headings line up; the content takes
// the rest. The list needs less room than an Application preview, so it stays down to
// the medium size class and the Markdown gets everything below that.
const (
	profileSplitWidth = mediumWidth
	profileDivider    = 1
	profileGutter     = 2
)

// profileListWidth is the section list's column, divider excluded.
func (m model) profileListWidth() int { return m.layout().panel(profileShare, profileMin, profileMax) }

// profileState is the loaded Profile and where the Owner is in it. It is replaced, never
// mutated in place, so copies of the model stay independent.
type profileState struct {
	profile  store.Profile
	selected int // index over the sections, then the other files
	focus    int // profileList or profileContent
	body     viewport.Model
}

// sectionCount is the selectable sections, before the divider.
func (st profileState) sectionCount() int { return len(st.profile.Sections) }

// entries is the whole selection: the sections, then the other files below the divider.
func (st profileState) entries() int { return st.sectionCount() + len(st.profile.Files) }

// entryName is the selected entry's name: a section heading or another file's name.
func (st profileState) entryName() string {
	if file, ok := st.file(); ok {
		return file.Name
	}
	if st.selected >= 0 && st.selected < len(st.profile.Sections) {
		return st.profile.Sections[st.selected].Heading
	}
	return ""
}

// position is the 1-based place in the narrow context row and what it counts against. A
// section counts against the sections ("Experience 3/6"); below the divider the other
// files continue the whole list ("projects.md 7/7").
func (st profileState) position() (int, int) {
	if st.selected < st.sectionCount() {
		return st.selected + 1, st.sectionCount()
	}
	return st.selected + 1, max(st.entries(), 1)
}

// file returns the selected other Profile file; false when a section is selected.
func (st profileState) file() (store.ProfileFile, bool) {
	i := st.selected - st.sectionCount()
	if i < 0 || i >= len(st.profile.Files) {
		return store.ProfileFile{}, false
	}
	return st.profile.Files[i], true
}

// path is the file the current selection reads.
func (st profileState) path() string {
	if file, ok := st.file(); ok {
		return file.Path
	}
	return profilePath
}

// row is the list row the selection sits on. The divider takes one row between the
// sections and the files, and is never selected itself.
func (st profileState) row() int {
	if len(st.profile.Files) > 0 && st.selected >= st.sectionCount() {
		return st.selected + 1
	}
	return st.selected
}

// profileLoaded reports whether there is a Profile to show: a loaded master-profile.md.
func (m model) profileLoaded() bool { return m.profile != nil && m.profile.profile.Exists }

// profileSelectedPath is the file the current selection reads: an other Profile file's
// path, or master-profile.md for a section. e opens it. A missing Profile names the file
// the notice names, because nothing can be selected.
func (m model) profileSelectedPath() string {
	if !m.profileLoaded() {
		return profilePath
	}
	return m.profile.path()
}

// loadProfile re-reads the Profile. It runs when the Owner switches to the screen, on
// r, and after the editor closes; profile/ is not watched, so this is how the screen
// sees a change. The selection is kept by name, not index, so a heading added above it
// does not move it; focus stays on the pane the Owner left it on.
func (m model) loadProfile() model {
	p, err := m.store.Profile()
	if err != nil {
		m.err = err
		return m
	}
	st := &profileState{profile: p, focus: profileList, body: viewport.New()}
	if m.profile != nil {
		st.selected, st.focus = m.profile.selectedIndex(p), m.profile.focus
	}
	st.selected = min(max(st.selected, 0), max(st.entries()-1, 0))
	m.profile = st
	return m.syncProfile()
}

// selectedIndex finds where the selection points in a freshly read Profile: the same
// section heading or other file name. A selection that is gone falls back to the first
// section, which is where a missing Profile starts too.
func (st profileState) selectedIndex(p store.Profile) int {
	if st.selected < len(st.profile.Sections) {
		heading := st.profile.Sections[st.selected].Heading
		for i, section := range p.Sections {
			if section.Heading == heading {
				return i
			}
		}
		return 0
	}
	if file, ok := st.file(); ok {
		for i, other := range p.Files {
			if other.Name == file.Name {
				return len(p.Sections) + i
			}
		}
	}
	return 0
}

// syncProfile keeps the content pane at the current size and on the selected section,
// without moving the scroll position. Other screens leave the loaded Profile alone.
func (m model) syncProfile() model {
	if m.profile == nil || m.screen != profileScreen {
		return m
	}
	st := *m.profile
	st.body.SetWidth(m.profileContentWidth(m.viewWidth()))
	st.body.SetHeight(m.profileBodyHeight())
	if !m.profileListShown(m.viewWidth()) {
		// The list is hidden, so it keeps the focus: there is no second pane to own it.
		st.focus = profileList
	}
	offset := st.body.YOffset()
	st.body.SetContent(m.profileMarkdown(st))
	st.body.SetYOffset(offset)
	m.profile = &st
	return m
}

// profileBodyHeight is the lines between the two rules, as on every screen.
func (m model) profileBodyHeight() int { return m.layout().bodyHeight(1) }

// profileListShown reports whether the section list has room beside the content. Below the
// split width the list is hidden and the context row names the selection instead.
func (m model) profileListShown(w int) bool { return w >= profileSplitWidth }

// profilePaneWidth is the content pane: the whole width when the list is hidden, otherwise
// what is left beside the list and its divider.
func (m model) profilePaneWidth(w int) int {
	if !m.profileListShown(w) {
		return max(minWrapWidth+inset, w)
	}
	return max(minWrapWidth+profileGutter, w-m.profileListWidth()-profileDivider)
}

// profilePaneGutter is the cells before the Markdown: the gutter after the divider, or
// the text column when the pane has the whole width.
func (m model) profilePaneGutter(w int) int {
	if m.profileListShown(w) {
		return profileGutter
	}
	return inset
}

// profileContentWidth is the Markdown wrap width inside that pane.
func (m model) profileContentWidth(w int) int {
	return m.profilePaneWidth(w) - m.profilePaneGutter(w) - rightMargin
}

// profileMarkdown renders the selected section or other Profile file, with the Contact
// warning above it when a required field is missing. Glamour tables can run wider than
// the wrap width, so every line is cut to the pane rather than spilling over the frame.
func (m model) profileMarkdown(st profileState) string {
	var body string
	if file, ok := st.file(); ok {
		body = file.Body
	} else if st.selected >= 0 && st.selected < len(st.profile.Sections) {
		body = st.profile.Sections[st.selected].Body
	}
	warning := contactWarning(st.profile.Missing)
	if body == "" && warning == "" {
		return ""
	}
	width := m.profileContentWidth(m.viewWidth())
	var lines []string
	if warning != "" {
		lines = append(lines, warning)
	}
	if body != "" {
		if warning != "" {
			lines = append(lines, "")
		}
		lines = append(lines, strings.Split(m.renderMarkdown(body, width), "\n")...)
	}
	for i, line := range lines {
		lines[i] = xansi.Truncate(line, width, "…")
	}
	return strings.Join(lines, "\n")
}

// contactWarning names the required Contact fields that are missing, so the Owner can fix
// them before a build fails. It shows above whatever section or file is selected: the
// warning belongs to the Profile, not to the Contact section.
func contactWarning(missing []string) string {
	switch len(missing) {
	case 0:
		return ""
	case 1:
		return "⚠ Contact is missing " + missing[0] + ". Builds will fail until it is added."
	}
	return "⚠ Contact is missing " + strings.Join(missing, " and ") + ". Builds will fail until they are added."
}

// selectProfile moves the selection and follows it with the content pane.
func (m model) selectProfile(delta int) model {
	st := *m.profile
	st.selected = min(max(st.selected+delta, 0), max(st.entries()-1, 0))
	st.body.GotoTop()
	m.profile = &st
	return m.syncProfile()
}

// scrollProfile scrolls the content pane, one line per j or k.
func (m model) scrollProfile(delta int) model {
	st := *m.profile
	if delta > 0 {
		st.body.ScrollDown(delta)
	} else {
		st.body.ScrollUp(-delta)
	}
	m.profile = &st
	return m
}

// profileHelpKeys is the keys Profile accepts: sections, edit, focus, screens, rescan,
// help and quit. Application actions are absent because they do nothing here, and so are
// sections and focus when there is no Profile to move through.
func (m model) profileHelpKeys() []helpKey {
	var keys []helpKey
	if m.profileLoaded() {
		keys = append(keys, help(groupAction, "j k", "section"), help(groupAction, "e", "edit file"),
			help(groupNav, "tab", "focus"))
	}
	keys = append(keys, help(groupNav, "1-6", "screen"), help(groupNav, "?", "all keys"), help(groupMore, ",", "settings"))
	if m.err != nil {
		keys = append(keys, help(groupNav, "esc", "clear error"))
	}
	return append(keys, help(groupMore, "r", "rescan"), help(groupMore, "q", "quit"))
}

// profileKey handles the keys the Profile screen accepts. Application keys and the track
// filter are inert: one Profile serves every track. Screen digits are handled before the
// screen dispatch, in screenKey.
func (m model) profileKey(k tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "r":
		m.rescanAll()
		return m.loadProfile(), nil
	case "?":
		m.overlay = &helpMode{}
		return m, nil
	case "q":
		return m, m.quit()
	}
	if !m.profileLoaded() {
		return m, nil // nothing to select, scroll or edit
	}
	listShown := m.profileListShown(m.viewWidth())
	switch k.String() {
	case "e":
		// The screen never writes profile/: the Owner's editor does, and the reload
		// follows when it exits, so the selection can stay on the same heading.
		return m, m.editFile(m.store.Root(), m.profileSelectedPath())
	case "tab", "shift+tab":
		// Below the split the list is hidden, so focus has nowhere to go and stays
		// on the list; j and k still change the selection.
		if !listShown {
			return m, nil
		}
		st := *m.profile
		st.focus = (st.focus + 1) % profilePanes
		m.profile = &st
	case "down", "j":
		if m.profile.focus == profileContent && listShown {
			return m.scrollProfile(1), nil
		}
		return m.selectProfile(1), nil
	case "up", "k":
		if m.profile.focus == profileContent && listShown {
			return m.scrollProfile(-1), nil
		}
		return m.selectProfile(-1), nil
	}
	return m, nil
}

// profileView is the Profile screen: the section list beside the selected section's
// Markdown, in the same frame Stats uses.
func (m model) profileView(p pal) string {
	w, h := m.viewWidth(), m.profileBodyHeight()
	split := 0
	if m.profileLoaded() && m.profileListShown(w) {
		split = m.profileListWidth()
	}
	return m.frame(p, []string{m.profileContextRow(p, w)}, m.profileBody(p, w, h), split, m.statusRow(p, w, m.keyLine(p, w)))
}

// profileContextRow names the focused pane on the left and the file it reads on the
// right, cut to what is left so the row never spills. Once the list is hidden the left
// names the selected entry and its position instead, since there is no list to mark it.
func (m model) profileContextRow(p pal, w int) string {
	return contextRow(m.profileContextLabel(p, w), p.text(p.dim).Render(m.profileSelectedPath()), w)
}

// profileContextLabel is the context row's left: the focused pane, or the selected entry's
// name and 1-based position when the section list is hidden.
func (m model) profileContextLabel(p pal, w int) string {
	pad := strings.Repeat(" ", inset)
	if !m.profileLoaded() {
		return pad + p.heading("PROFILE")
	}
	if !m.profileListShown(w) {
		pos, of := m.profile.position()
		return pad + p.text(p.fg).Bold(true).Render(m.profile.entryName()) +
			p.text(p.dim).Render(" "+strconv.Itoa(pos)+"/"+strconv.Itoa(of))
	}
	pane := "SECTIONS"
	if m.profile.focus == profileContent {
		pane = "CONTENT"
	}
	return pad + p.heading(pane)
}

// profileBody is the list and content panes, or the missing-Profile notice. Both panes
// are exactly the body height so the rules never move.
func (m model) profileBody(p pal, w, h int) string {
	if !m.profileLoaded() {
		return lipgloss.Place(w, h, lipgloss.Left, lipgloss.Top, strings.Repeat(" ", inset)+p.text(p.fg).Render(noProfile))
	}
	pane := lipgloss.NewStyle().Width(m.profilePaneWidth(w)).Height(h).MaxHeight(h).
		PaddingLeft(m.profilePaneGutter(w)).Render(m.profileContentView(p))
	if !m.profileListShown(w) {
		return pane
	}
	list := lipgloss.NewStyle().Width(m.profileListWidth()).Height(h).MaxHeight(h).Render(m.profileSectionList(p, h))
	return lipgloss.JoinHorizontal(lipgloss.Top, list, p.divider(h), pane)
}

// profileContentView is the content viewport, with the shared overflow cue on its first
// or last line while text is scrolled out of view that way.
func (m model) profileContentView(p pal) string {
	b := m.profile.body
	lines := strings.Split(b.View(), "\n")
	if len(lines) == 0 || b.Height() <= 1 {
		return b.View()
	}
	if above := b.YOffset(); above > 0 {
		lines[0] = p.moreCue(true, above+1)
	}
	if below := b.TotalLineCount() - b.YOffset() - b.Height(); below > 0 && len(lines) >= b.Height() {
		lines[b.Height()-1] = p.moreCue(false, below+1)
	}
	return strings.Join(lines, "\n")
}

// profileSectionList is the sections, a divider, then the other Profile files that exist:
// one row each, with its count right-aligned and the selected one marked and bold. The
// window follows the selection when the entries outrun the lines.
func (m model) profileSectionList(p pal, h int) string {
	st := m.profile
	listW := m.profileListWidth()
	rows := make([]string, 0, st.entries()+1)
	for i, section := range st.profile.Sections {
		suffix, colour := m.profileSectionSuffix(p, section)
		rows = append(rows, profileListRow(p, listW, section.Heading, suffix, colour, i == st.selected))
	}
	if len(st.profile.Files) > 0 {
		rows = append(rows, strings.Repeat(" ", inset)+p.text(p.rule).Render(strings.Repeat("─", listW-inset-gutter)))
	}
	for i, file := range st.profile.Files {
		rows = append(rows, profileListRow(p, listW, file.Name, "", p.dim, st.sectionCount()+i == st.selected))
	}
	return strings.Join(p.clipLines(rows, st.row()-h/bothSides, h, strings.Repeat(" ", inset)), "\n")
}

// profileSectionSuffix is the right-aligned mark on a section row: Contact's ✓ or ✗ in
// place of a count, the count itself, or nothing for prose.
func (m model) profileSectionSuffix(p pal, section store.ProfileSection) (string, color.Color) {
	if section.Heading == "Contact" {
		if len(m.profile.profile.Missing) == 0 {
			return contactOK, p.ok
		}
		return contactBad, p.bad
	}
	if section.Kind != "" {
		return strconv.Itoa(section.Count), p.dim
	}
	return "", p.dim
}

// profileListRow is one entry in the list column: the marker and name on the left, the
// count right-aligned in the column. The name is cut to leave room for the suffix, and a
// section with no count leaves the column blank.
func profileListRow(p pal, listW int, name, suffix string, colour color.Color, selected bool) string {
	width := listW - gutter // two blank cells before the divider
	room := width - inset
	if suffix != "" {
		room -= xansi.StringWidth(suffix) + 1 // one cell between the name and the count
	}
	heading := xansi.Truncate(name, max(1, room), "…")
	left := strings.Repeat(" ", inset) + p.text(p.dim).Render(heading)
	if selected {
		left = strings.Repeat(" ", edge) + p.text(p.accent).Bold(true).Render(profileMarker+heading)
	}
	if suffix == "" {
		return left
	}
	gap := width - inset - xansi.StringWidth(heading) - xansi.StringWidth(suffix)
	return left + strings.Repeat(" ", max(1, gap)) + p.text(colour).Render(suffix)
}
