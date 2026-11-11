package ui

import (
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

// Profile screen sizing: the section list, the divider, and the gutter before the
// Markdown. The list gets a fixed column so headings line up; the content takes the rest.
const (
	profileListWidth = 24
	profileDivider   = 1
	profileGutter    = 2
)

// profileState is the loaded Profile and where the Owner is in it. It is replaced, never
// mutated in place, so copies of the model stay independent.
type profileState struct {
	profile  store.Profile
	selected int // section index
	focus    int // profileList or profileContent
	body     viewport.Model
}

// loadProfile re-reads the Profile. It runs when the Owner switches to the screen and on
// r; profile/ is not watched, so this is how the screen sees a change.
func (m model) loadProfile() model {
	p, err := m.store.Profile()
	if err != nil {
		m.err = err
		return m
	}
	st := &profileState{profile: p, focus: profileList, body: viewport.New()}
	if m.profile != nil {
		st.selected, st.focus = m.profile.selected, m.profile.focus
	}
	st.selected = min(max(st.selected, 0), max(len(p.Sections)-1, 0))
	m.profile = st
	return m.syncProfile()
}

// syncProfile keeps the content pane at the current size and on the selected section,
// without moving the scroll position. Other screens leave the loaded Profile alone.
func (m model) syncProfile() model {
	if m.profile == nil || m.screen != profileScreen {
		return m
	}
	st := *m.profile
	st.body.SetWidth(m.profileContentWidth())
	st.body.SetHeight(m.profileBodyHeight())
	offset := st.body.YOffset()
	st.body.SetContent(m.profileMarkdown(st))
	st.body.SetYOffset(offset)
	m.profile = &st
	return m
}

// profileBodyHeight is the lines between the two rules, as on every screen.
func (m model) profileBodyHeight() int { return max(minBody, m.viewHeight()-chromeLines) }

// profilePaneWidth is the content pane beside the list and its divider.
func (m model) profilePaneWidth() int {
	return max(minWrapWidth+profileGutter, m.viewWidth()-profileListWidth-profileDivider)
}

// profileContentWidth is the Markdown wrap width inside that pane.
func (m model) profileContentWidth() int { return m.profilePaneWidth() - profileGutter }

// profileMarkdown renders the selected section. Glamour tables can run wider than the
// wrap width, so every line is cut to the pane rather than spilling over the frame.
func (m model) profileMarkdown(st profileState) string {
	if st.selected < 0 || st.selected >= len(st.profile.Sections) {
		return ""
	}
	body := st.profile.Sections[st.selected].Body
	if body == "" {
		return ""
	}
	width := m.profileContentWidth()
	lines := strings.Split(m.renderMarkdown(body, width), "\n")
	for i, line := range lines {
		lines[i] = xansi.Truncate(line, width, "…")
	}
	return strings.Join(lines, "\n")
}

// selectProfile moves the selection and follows it with the content pane.
func (m model) selectProfile(delta int) model {
	st := *m.profile
	st.selected = min(max(st.selected+delta, 0), len(st.profile.Sections)-1)
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

// profileHelpKeys is the keys Profile accepts: sections, focus, screens, rescan, help
// and quit. Application actions are absent because they do nothing here.
func (m model) profileHelpKeys() []helpKey {
	keys := []helpKey{help(groupAction, "j k", "section"), help(groupNav, "tab", "focus"),
		help(groupNav, "1-5", "screen"), help(groupNav, "?", "all keys")}
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
	if m.profile == nil || !m.profile.profile.Exists {
		return m, nil // nothing to select or scroll, and e does not exist yet
	}
	switch k.String() {
	case "tab", "shift+tab":
		st := *m.profile
		st.focus = (st.focus + 1) % profilePanes
		m.profile = &st
	case "down", "j":
		if m.profile.focus == profileContent {
			return m.scrollProfile(1), nil
		}
		return m.selectProfile(1), nil
	case "up", "k":
		if m.profile.focus == profileContent {
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
	rule := p.text(p.rule).Render(strings.Repeat("─", w))
	return m.topBar(p) + "\n\n" + m.profileContextRow(p, w) + "\n" + rule + "\n" +
		m.profileBody(p, w, h) + "\n" + rule + "\n" + m.statusRow(p, w, m.keyLine(p, w))
}

// profileContextRow names the focused pane on the left and the file it reads on the
// right, cut to what is left so the row never spills.
func (m model) profileContextRow(p pal, w int) string {
	pane := "SECTIONS"
	if m.profile != nil {
		switch {
		case !m.profile.profile.Exists:
			pane = "PROFILE"
		case m.profile.focus == profileContent:
			pane = "CONTENT"
		}
	}
	left := "  " + p.heading(pane)
	avail := max(0, w-rightMargin-xansi.StringWidth(left)-gutter)
	return spread(left, p.text(p.dim).Render(xansi.Truncate(profilePath, avail, "…")), w-rightMargin)
}

// profileBody is the list and content panes, or the missing-Profile notice. Both panes
// are exactly the body height so the rules never move.
func (m model) profileBody(p pal, w, h int) string {
	if m.profile == nil || !m.profile.profile.Exists {
		return lipgloss.Place(w, h, lipgloss.Left, lipgloss.Top, "  "+p.text(p.fg).Render(noProfile))
	}
	list := lipgloss.NewStyle().Width(profileListWidth).Height(h).MaxHeight(h).Render(m.profileSectionList(p, h))
	sep := p.text(p.rule).Render(strings.TrimSuffix(strings.Repeat("│\n", h), "\n"))
	pane := lipgloss.NewStyle().Width(m.profilePaneWidth()).Height(h).MaxHeight(h).
		PaddingLeft(profileGutter).Render(m.profile.body.View())
	return lipgloss.JoinHorizontal(lipgloss.Top, list, sep, pane)
}

// profileSectionList is one row per section, the selected one marked and bold. The
// window follows the selection when there are more sections than lines.
func (m model) profileSectionList(p pal, h int) string {
	st := m.profile
	lines := make([]string, len(st.profile.Sections))
	width := profileListWidth - profileGutter
	for i, section := range st.profile.Sections {
		heading := xansi.Truncate(section.Heading, width-profileGutter, "…")
		if i == st.selected {
			lines[i] = p.text(p.accent).Bold(true).Render(profileMarker + heading)
		} else {
			lines[i] = "  " + p.text(p.dim).Render(heading)
		}
	}
	if len(lines) > h {
		start := min(max(0, st.selected-h/2), len(lines)-h)
		lines = lines[start:]
	}
	return strings.Join(lines, "\n")
}
