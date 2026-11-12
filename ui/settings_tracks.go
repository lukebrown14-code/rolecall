package ui

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// groupTracking is the Settings group that holds the Tracks.
const groupTracking = 2

// trackingRows lists one row per Track (the first labelled Tracks), then follow-up days.
func (m model) trackingRows() []settingRow {
	var rows []settingRow
	for i, t := range m.settings.ws.Tracks {
		label := ""
		if i == 0 {
			label = "Tracks"
		}
		rows = append(rows, settingRow{label: label, value: t})
	}
	if len(rows) == 0 {
		rows = append(rows, settingRow{label: "Tracks", value: "none (a to add)"})
	}
	return append(rows, settingRow{label: "Follow-up days", value: fmt.Sprint(m.followUpDays)})
}

// trackHelpKeys are the Track keys, shown while the Tracking rows have focus.
func (m model) trackHelpKeys() []helpKey {
	if m.settings == nil || m.settings.ws.Err != nil || m.settings.focus != settingsRows || m.settings.group != groupTracking {
		return nil
	}
	keys := []helpKey{help(groupAction, "a", "add")}
	if m.settings.row < len(m.settings.ws.Tracks) {
		keys = append(keys, help(groupAction, "r", "rename"), help(groupAction, "x", "remove"), help(groupAction, "J/K", "reorder"))
	}
	return keys
}

// validTrackName applies the Track name rules: non-empty after trimming, unique ignoring
// case, no slash (it becomes a folder name).
func validTrackName(name string, existing []string) error {
	switch {
	case name == "":
		return errors.New("Track name is empty")
	case strings.Contains(name, "/"):
		return errors.New("Track name cannot contain /")
	}
	for _, t := range existing {
		if strings.EqualFold(t, name) {
			return fmt.Errorf("Track %s already exists", t)
		}
	}
	return nil
}

// tracksKey handles a, J, K and x on the Tracking rows. handled is false for any other key.
func (m model) tracksKey(k tea.KeyPressMsg) (next model, handled bool) {
	st := m.settings
	if st.focus != settingsRows || st.group != groupTracking {
		return m, false
	}
	tracks := slices.Clone(st.ws.Tracks)
	on := st.row < len(tracks)
	switch k.String() {
	case "a":
		m.overlay = &trackAddMode{}
	case "J", "K":
		to := st.row + 1
		if k.String() == "K" {
			to = st.row - 1
		}
		if !on || to < 0 || to >= len(tracks) {
			return m, true
		}
		moved := tracks[st.row]
		tracks[st.row], tracks[to] = tracks[to], tracks[st.row]
		if m = m.saveTracks(tracks); m.err == nil {
			s := *m.settings
			s.row = to
			m.settings = &s
			m.toast = "Moved Track " + moved
		}
	case "x":
		if !on {
			return m, true
		}
		name := tracks[st.row]
		n, err := m.trackUse(name)
		if err != nil {
			m.err = err
			return m, true
		}
		if n > 0 {
			m.overlay = &removeBlockedMode{track: name, n: n}
			return m, true
		}
		tracks = slices.Delete(tracks, st.row, st.row+1)
		if m = m.saveTracks(tracks); m.err == nil {
			s := *m.settings
			s.row = min(s.row, len(m.trackingRows())-1)
			m.settings = &s
			m.toast = "Removed Track " + name
		}
	default:
		return m, false
	}
	return m, true
}

// trackUse counts the Applications on a Track, archived ones included.
func (m model) trackUse(track string) (int, error) {
	return m.store.TrackUse(track)
}

// saveTracks writes the Tracks, then re-reads so the header filter and the screen follow.
func (m model) saveTracks(tracks []string) model {
	if err := m.store.SetTracks(tracks); err != nil {
		m.err = err
		return m
	}
	m.err = nil
	m.rescanAll()
	return m.loadSettings()
}

// trackAddMode is the pop-up that asks for a new Track's name.
type trackAddMode struct {
	name string
	err  string
}

func (tm *trackAddMode) Keys() [][2]string { return [][2]string{{"enter", "add"}, {"esc", "cancel"}} }

func (tm *trackAddMode) boxed() bool { return true }

func (tm *trackAddMode) View(m *model, p pal) string {
	body := p.text(p.dim).Render("Name  ") + p.text(p.fg).Render(tm.name) + p.text(p.accent).Render("▏")
	if tm.err != "" {
		body += "\n" + p.text(p.bad).Render("✗ "+tm.err)
	}
	return popup(p, "Add Track", false, popSmall, m.viewWidth(), body, p.keys(tm.Keys()))
}

func (tm *trackAddMode) Update(m *model, msg tea.Msg) (mode, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.PasteMsg:
		tm.name += pasted(msg.Content)
		tm.err = ""
	case tea.KeyPressMsg:
		switch {
		case msg.String() == "esc":
			return nil, nil
		case msg.Code == tea.KeyBackspace:
			tm.name = dropLastGrapheme(tm.name)
			tm.err = ""
		case msg.String() == "enter":
			name := strings.TrimSpace(tm.name)
			if err := validTrackName(name, m.settings.ws.Tracks); err != nil {
				tm.err = err.Error()
				return tm, nil
			}
			tracks := append(slices.Clone(m.settings.ws.Tracks), name)
			if *m = m.saveTracks(tracks); m.err != nil {
				return nil, nil
			}
			s := *m.settings
			s.row = len(tracks) - 1
			m.settings = &s
			m.toast = "Added Track " + name
			return nil, nil
		default:
			tm.name += msg.Text
			tm.err = ""
		}
	}
	return tm, nil
}

// removeBlockedMode refuses removing a Track that Applications still use. It is a pop-up
// by the rows rather than a footer error, and it names the way out: rename the Track here,
// or move the Applications still on it to another Track first.
type removeBlockedMode struct {
	track string
	n     int
}

func (rb *removeBlockedMode) Keys() [][2]string {
	return [][2]string{{"r", "rename"}, {"esc", hintClose}}
}

func (rb *removeBlockedMode) boxed() bool { return true }

func (rb *removeBlockedMode) View(m *model, p pal) string {
	what := fmt.Sprintf("%s is used by %s.", rb.track, plural(rb.n, "Application", "Applications"))
	next := "Rename it with r, or move its Applications to another Track first."
	body := lipgloss.NewStyle().Foreground(p.fg).Width(popSmall-popInset).Render(what) + "\n\n" +
		p.text(p.dim).Width(popSmall-popInset).Render(next)
	return popup(p, "Remove Track", true, popSmall, m.viewWidth(), body, p.keys(rb.Keys()))
}

func (rb *removeBlockedMode) Update(m *model, msg tea.Msg) (mode, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return rb, nil
	}
	switch k.String() {
	case "r":
		// The Track cannot be removed, but it can be renamed from here.
		return &renameMode{old: rb.track, name: rb.track}, nil
	case "esc", "enter":
		return nil, nil
	}
	return rb, nil
}
