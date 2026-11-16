package ui

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/lukebrown14-code/rolecall/internal/store"
	"github.com/lukebrown14-code/rolecall/internal/ui/settings"
)

// onTrack reports whether the Tracking row under the cursor is a Track.
func (m model) onTrack() bool {
	st := m.settings
	return st != nil && st.Ws.Err == nil && st.Group == settings.GroupTracking && st.Row < len(st.Ws.Tracks)
}

// onAddTrack reports whether the cursor is on the "+ add a Track" row, after the Tracks
// and the Tracks only Applications name.
func (m model) onAddTrack() bool {
	st := m.settings
	return st != nil && st.Ws.Err == nil && st.Group == settings.GroupTracking && st.Row == len(st.Ws.Tracks)+len(st.OffList())
}

// offListTrack is the Track under the cursor that Applications use but workspace.json
// does not list; ok is false on any other row.
func (m model) offListTrack() (string, bool) {
	st := m.settings
	if st == nil || st.Ws.Err != nil || st.Group != settings.GroupTracking || st.Row < len(st.Ws.Tracks) {
		return "", false
	}
	off := st.OffList()
	if i := st.Row - len(st.Ws.Tracks); i < len(off) {
		return off[i], true
	}
	return "", false
}

// addOffList adds a Track Applications already use to workspace.json, so it can be
// filtered by and renamed like any other.
func (m model) addOffList(name string) model {
	was := m.settings.Ws.Tracks
	tracks := append(slices.Clone(was), name)
	// The list is checked as Tracks will read it, so a name it would refuse (a case twin
	// of a listed Track, a slash) is never written.
	if err := validTrackName(name, was); err != nil {
		m.err = fmt.Errorf("Can't list %s: %w", name, err)
		return m
	}
	if err := store.ValidTrackList(tracks); err != nil {
		m.err = fmt.Errorf("Can't list %s: %w", name, err)
		return m
	}
	if m = m.saveTracks(tracks); m.err != nil {
		return m
	}
	s := *m.settings
	s.Row = len(tracks) - 1
	m.settings = &s
	return m.saved("Added Track "+name, settingsFile, m.tracksUndo("Track "+name+" off the list again", was))
}

// trackHelpKeys are the Track keys, shown while the Tracking rows have focus.
func (m model) trackHelpKeys() []helpKey {
	if m.settings == nil || m.settings.Ws.Err != nil || !m.settingsRowsFocused() || m.settings.Group != settings.GroupTracking {
		return nil
	}
	keys := []helpKey{help(groupAction, "a", "add")}
	switch {
	case m.onTrack():
		// enter renames, as it edits every other row: r stays rescan, as it is everywhere.
		keys = append([]helpKey{help(groupAction, "enter", "rename").at(prioCore)}, keys...)
		keys = append(keys, help(groupAction, "x", "remove"), help(groupAction, "J/K", "reorder"))
	case m.onAddTrack():
		keys = []helpKey{help(groupAction, "enter", "add a Track").at(prioCore)}
	default:
		if _, ok := m.offListTrack(); ok {
			keys = append(keys, help(groupAction, "enter", "add to Tracks").at(prioCore))
		}
	}
	return keys
}

// settingsRowsFocused reports whether keys act on the settings rows: the cursor is on a
// field, not the header.
func (m model) settingsRowsFocused() bool {
	return m.settings != nil && !m.settings.Header
}

// tracksKey handles a, J, K, x and enter (add, or rename a Track) on the Tracking rows.
// handled is false for any other key.
func (m model) tracksKey(k tea.KeyPressMsg) (next model, handled bool) {
	st := m.settings
	if !m.settingsRowsFocused() || st.Group != settings.GroupTracking {
		return m, false
	}
	tracks := slices.Clone(st.Ws.Tracks)
	on := m.onTrack()
	switch k.String() {
	case "a":
		m.overlay = m.trackAddPopup()
	case "enter":
		switch {
		case m.onAddTrack():
			m.overlay = m.trackAddPopup()
		case on:
			m.overlay = m.renamePopup(tracks[st.Row])
		default:
			if name, ok := m.offListTrack(); ok {
				return m.addOffList(name), true
			}
			return m, false
		}
	case "J", "K":
		to := st.Row + 1
		if k.String() == "K" {
			to = st.Row - 1
		}
		if !on || to < 0 || to >= len(tracks) {
			return m, true
		}
		moved := tracks[st.Row]
		tracks[st.Row], tracks[to] = tracks[to], tracks[st.Row]
		if m = m.saveTracks(tracks); m.err == nil {
			s := *m.settings
			s.Row = to
			m.settings = &s
			m = m.saved("Moved Track "+moved, settingsFile, m.tracksUndo("Tracks back in their old order", st.Ws.Tracks))
		}
	case "x":
		if !on {
			return m, true
		}
		name := tracks[st.Row]
		n, err := m.store.TrackUse(name)
		if err != nil {
			m.err = err
			return m, true
		}
		if n > 0 {
			m.overlay = m.removeBlocked(name, n)
			return m, true
		}
		tracks = slices.Delete(tracks, st.Row, st.Row+1)
		if m = m.saveTracks(tracks); m.err == nil {
			s := *m.settings
			s.Row = m.selectableRow(settings.GroupTracking, min(s.Row, len(m.trackingRows())-1), -1)
			m.settings = &s
			m = m.saved("Removed Track "+name, settingsFile, m.tracksUndo("Track "+name+" restored", st.Ws.Tracks))
		}
	default:
		return m, false
	}
	return m, true
}

// tracksUndo puts the Track list back as it was.
func (m model) tracksUndo(what string, was []string) *settingsUndo {
	was = slices.Clone(was)
	return &settingsUndo{what: what, restore: func(m model) model { return m.saveTracks(was) }}
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

// trackAddPopup asks for a new Track's name.
func (m model) trackAddPopup() mode {
	f := newFormOf("Add a Track", formField{Label: "Name", Required: true, Placeholder: "e.g. Data, Platform"})
	f.Submit = "add"
	return &formPopup{form: f, width: popSmall, save: func(m *model, f *form) (mode, error) {
		name := strings.TrimSpace(f.Value("Name"))
		if err := validTrackName(name, m.settings.Ws.Tracks); err != nil {
			return nil, err
		}
		was := m.settings.Ws.Tracks
		tracks := append(slices.Clone(was), name)
		if *m = m.saveTracks(tracks); m.err != nil {
			return nil, nil
		}
		s := *m.settings
		s.Row = len(tracks) - 1
		m.settings = &s
		*m = m.saved("Added Track "+name, settingsFile, m.tracksUndo("Track "+name+" removed", was))
		return nil, nil
	}}
}

// trackCount is how many of a Track's Applications the lists hold, and how many of those
// are Dismissed or unreadable, the ones easy to miss when counting the lists. The rest
// of TrackUse's count is archived.
func (m model) trackCount(track string) (listed, dismissed, unreadable int) {
	for _, a := range m.apps {
		if a.Track != track {
			continue
		}
		listed++
		switch {
		case a.Malformed != nil:
			unreadable++
		case a.Status == store.StatusDismissed:
			dismissed++
		}
	}
	return listed, dismissed, unreadable
}

// removeBlocked is the refusal for a Track still in use, counting what the lists do not
// show so the number adds up.
func (m model) removeBlocked(track string, n int) *removeBlockedMode {
	listed, dismissed, unreadable := m.trackCount(track)
	var incl []string
	if dismissed > 0 {
		incl = append(incl, strconv.Itoa(dismissed)+" Dismissed")
	}
	if unreadable > 0 {
		incl = append(incl, strconv.Itoa(unreadable)+" unreadable")
	}
	if other := n - listed; other > 0 {
		incl = append(incl, strconv.Itoa(other)+" archived")
	}
	return &removeBlockedMode{track: track, n: n, incl: strings.Join(incl, ", ")}
}

// removeBlockedMode refuses removing a Track that Applications still use. It is a pop-up
// by the rows rather than a footer error, and it names the way out: move the Applications
// still on it to another Track first.
type removeBlockedMode struct {
	track string
	n     int
	incl  string // "1 Dismissed, 2 archived": the part of n the lists do not show
}

func (rb *removeBlockedMode) Keys() [][2]string { return [][2]string{{"esc", hintClose}} }

func (rb *removeBlockedMode) View(m *model, p pal) string {
	what := fmt.Sprintf("%s is used by %s", rb.track, plural(rb.n, "Application", "Applications"))
	if rb.incl != "" {
		what += ",\nincl. " + rb.incl
	}
	next := "Move its Applications to another Track first: e on each, then change its Track."
	body := lipgloss.NewStyle().Foreground(p.Fg).Width(popSmall-popInset).Render(what+".") + "\n\n" +
		p.Text(p.Dim).Width(popSmall-popInset).Render(next)
	return popup(p, "Can't remove Track "+rb.track, true, popSmall, m.viewWidth(), body, p.Keys(rb.Keys()))
}

func (rb *removeBlockedMode) Update(m *model, msg tea.Msg) (mode, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return rb, nil
	}
	switch k.String() {
	case "esc", "enter":
		return nil, nil
	}
	return rb, nil
}

var validTrackName = store.ValidTrackName
