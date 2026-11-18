package settings

import (
	"fmt"
	"slices"
	"strconv"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/internal/store"
)

// Key words: enter on a row whose value opens for editing, and adding a Track.
const (
	descEdit = "edit"
	descAdd  = "add"
)

// searchKeys is the Search area key, shown while the Search rows have focus.
func (m Model) searchKeys(env Env) []Key {
	if m.Ws.Err != nil || !m.rowsFocused() || m.Group != GroupSearch || m.Row > 1 {
		return nil
	}
	what := "edit Search area"
	if env.Narrow {
		what = descEdit // leaves room for the pinned ←→ section
	}
	return []Key{{Key: "enter", Desc: what, Prio: PrioCore}}
}

// searchKey opens the Search area form on enter over Places or Remote roles.
func (m Model) searchKey(k tea.KeyPressMsg, _ Env) (Model, Intent, bool) {
	if k.String() != "enter" || !m.rowsFocused() || m.Group != GroupSearch || m.Row > 1 {
		return m, nil, false
	}
	return m, Open{Popup: m.searchAreaPopup()}, true
}

// enterEditKeys is "enter edit" while a group whose every row opens a pop-up has focus.
func (m Model) enterEditKeys(group int) []Key {
	if m.Ws.Err != nil || !m.rowsFocused() || m.Group != group {
		return nil
	}
	return []Key{{Key: "enter", Desc: descEdit, Prio: PrioCore}}
}

// rulesKeys are the Rules keys, shown while those rows have focus.
func (m Model) rulesKeys(Env) []Key { return m.enterEditKeys(GroupRules) }

// rulesKey opens the focused Search rule's pop-up on enter, or flips Needs sponsorship.
// handled is false for any other key or group.
func (m Model) rulesKey(k tea.KeyPressMsg, env Env) (Model, Intent, bool) {
	if !m.rowsFocused() || m.Group != GroupRules || k.String() != "enter" || m.Row >= len(Rules)+len(VisaRules) {
		return m, nil, false
	}
	if m.Row < len(Rules) {
		return m, Open{Popup: m.rulePopup(Rules[m.Row])}, true
	}
	r := VisaRules[m.Row-len(Rules)]
	if r.Key == store.VisaNeedsSponsorship {
		return m, m.saveRule(r, strconv.FormatBool(!m.Ws.Visa.NeedsSponsorship), env), true
	}
	return m, Open{Popup: m.rulePopup(r)}, true
}

// ruleText is a Rules row's value as the Owner types it.
func (m Model) ruleText(key string) string {
	if IsVisa(key) {
		return VisaText(m.Ws.Visa, key)
	}
	return RuleText(m.Ws.Rules, key)
}

// saveRule writes one Rules row, names what it saved and offers its undo.
func (m Model) saveRule(r Rule, text string, env Env) Save {
	was := m.ruleText(r.Key)
	shown := m.rowShown(GroupRules, r.Label, env) // what the undo puts back, as the row reads now
	return Save{Write: SetRule{Key: r.Key, Text: text}, rule: &r,
		Undo: &Undo{What: fmt.Sprintf("%s back to %s", r.Label, shown), Restore: SetRule{Key: r.Key, Text: was}}}
}

// ruleSaved is a Rules save's message, naming the value as the reloaded file reads it.
func (m Model) ruleSaved(r Rule) string {
	msg := "Cleared " + r.Label
	if now := m.ruleText(r.Key); now != "" {
		if r.Key == store.VisaNeedsSponsorship {
			now = YesNo(now == "true")
		}
		msg = "Saved " + r.Label + ": " + now
		if r.Unit != "" {
			msg += " " + r.Unit
		}
	}
	return msg
}

// Sources keys. Each is free of every global key: t filters by Track everywhere, so a
// Source is checked with c, and enter opens the Source's form rather than flip it, so an
// enter pressed out of habit never stops a Source the Sweep relies on.
const (
	// keySourceToggle turns the focused Source off or on.
	keySourceToggle = "o"
	// keySourceCheck runs the Plugin's Sweep against the focused Source.
	keySourceCheck = "c"
)

// focusedSource is the Source under the cursor, if the cursor is on one.
func (m Model) focusedSource() (store.Source, bool) {
	if m.Ws.Err != nil || !m.rowsFocused() || m.Group != GroupSources || m.Row >= len(m.Ws.SourceList) {
		return store.Source{}, false
	}
	return m.Ws.SourceList[m.Row], true
}

// onAddSource reports whether the cursor is on "+ add a Source".
func (m Model) onAddSource() bool {
	return m.Ws.Err == nil && m.rowsFocused() && m.Group == GroupSources && m.Row == len(m.Ws.SourceList)
}

// sourceToggleVerb is what o does to src: "turn off" or "turn on".
func sourceToggleVerb(src store.Source) string {
	if src.On() {
		return "turn off"
	}
	return "turn on"
}

// sourcesKeys are the Sources keys, shown while those rows have focus.
func (m Model) sourcesKeys(Env) []Key {
	if src, ok := m.focusedSource(); ok {
		return []Key{{Key: "enter", Desc: descEdit, Prio: PrioCore},
			{Key: keySourceToggle, Desc: sourceToggleVerb(src), Prio: PrioCore}, {Key: keySourceCheck, Desc: "check", Prio: PrioCore}}
	}
	if m.onAddSource() {
		return []Key{{Key: "enter", Desc: "add a Source", Prio: PrioCore}}
	}
	return nil
}

// sourcesKey opens the focused Source's form on enter (the add form from "+ add a
// Source") and turns it on or off on o. handled is false for any other key or group.
func (m Model) sourcesKey(k tea.KeyPressMsg, _ Env) (Model, Intent, bool) {
	switch k.String() {
	case "enter":
		if m.onAddSource() {
			return m, Open{Popup: addSourcePopup()}, true
		}
		src, ok := m.focusedSource()
		if !ok {
			return m, nil, false
		}
		return m, Open{Popup: editSourcePopup(src)}, true
	case keySourceToggle:
		src, ok := m.focusedSource()
		if !ok {
			return m, nil, false
		}
		return m, setSourceEnabled(src.Name, !src.On()), true
	}
	return m, nil, false
}

// setSourceEnabled turns a Source on or off and offers u to put it back.
func setSourceEnabled(name string, on bool) Save {
	word := map[bool]string{true: "on", false: "off"}
	return Save{Write: SetSourceEnabled{Name: name, On: on}, Msg: "Turned " + word[on] + " " + name,
		Undo: &Undo{What: name + " back " + word[!on], Restore: SetSourceEnabled{Name: name, On: !on}}}
}

// gateKeys are the Documents keys, shown while those rows have focus.
func (m Model) gateKeys(Env) []Key { return m.enterEditKeys(GroupDocuments) }

// gatesKey handles enter on a Documents row by opening the value pop-up. handled is false
// for any other key or group.
func (m Model) gatesKey(k tea.KeyPressMsg, _ Env) (Model, Intent, bool) {
	if !m.rowsFocused() || m.Group != GroupDocuments || k.String() != "enter" || m.Row >= len(Gates) {
		return m, nil, false
	}
	return m, Open{Popup: m.gatePopup(Gates[m.Row])}, true
}

// onTrack reports whether the Tracking row under the cursor is a Track.
func (m Model) onTrack() bool {
	return m.Ws.Err == nil && m.Group == GroupTracking && m.Row < len(m.Ws.Tracks)
}

// onAddTrack reports whether the cursor is on the "+ add a Track" row, after the Tracks
// and the Tracks only Applications name.
func (m Model) onAddTrack() bool {
	return m.Ws.Err == nil && m.Group == GroupTracking && m.Row == len(m.Ws.Tracks)+len(m.OffList())
}

// offListTrack is the Track under the cursor that Applications use but workspace.json
// does not list; ok is false on any other row.
func (m Model) offListTrack() (string, bool) {
	if m.Ws.Err != nil || m.Group != GroupTracking || m.Row < len(m.Ws.Tracks) {
		return "", false
	}
	off := m.OffList()
	if i := m.Row - len(m.Ws.Tracks); i < len(off) {
		return off[i], true
	}
	return "", false
}

// trackKeys are the Track keys, shown while the Tracking rows have focus.
func (m Model) trackKeys(Env) []Key {
	if m.Ws.Err != nil || !m.rowsFocused() || m.Group != GroupTracking {
		return nil
	}
	keys := []Key{{Key: "a", Desc: descAdd}}
	switch {
	case m.onTrack():
		// enter renames, as it edits every other row: r stays rescan, as it is everywhere.
		// The key line names add and remove too, or the Owner never finds them.
		keys = []Key{{Key: "enter", Desc: "rename", Prio: PrioCore}, {Key: "a", Desc: descAdd, Prio: PrioCore},
			{Key: "x", Desc: "remove", Prio: PrioCore}, {Key: "J/K", Desc: "reorder"}}
	case m.onAddTrack():
		keys = []Key{{Key: "enter", Desc: "add a Track", Prio: PrioCore}}
	default:
		if _, ok := m.offListTrack(); ok {
			keys = append(keys, Key{Key: "enter", Desc: "add to Tracks", Prio: PrioCore})
		}
	}
	return keys
}

// tracksKey handles a, J, K, x and enter (add, or rename a Track) on the Tracking rows.
// handled is false for any other key.
func (m Model) tracksKey(k tea.KeyPressMsg, env Env) (Model, Intent, bool) {
	if !m.rowsFocused() || m.Group != GroupTracking {
		return m, nil, false
	}
	tracks := slices.Clone(m.Ws.Tracks)
	on := m.onTrack()
	switch k.String() {
	case "a":
		return m, Open{Popup: trackAddPopup()}, true
	case "enter":
		switch {
		case m.onAddTrack():
			return m, Open{Popup: trackAddPopup()}, true
		case on:
			return m, Open{Popup: renamePopup(tracks[m.Row])}, true
		}
		if name, ok := m.offListTrack(); ok {
			return m, m.addOffList(name), true
		}
		return m, nil, false
	case "J", "K":
		to := m.Row + 1
		if k.String() == "K" {
			to = m.Row - 1
		}
		if !on || to < 0 || to >= len(tracks) {
			return m, nil, true
		}
		moved := tracks[m.Row]
		tracks[m.Row], tracks[to] = tracks[to], tracks[m.Row]
		return m, Save{Write: SetTracks{Tracks: tracks}, Msg: "Moved Track " + moved, place: placeAt, row: to,
			Undo: tracksUndo("Tracks back in their old order", m.Ws.Tracks)}, true
	case "x":
		if !on {
			return m, nil, true
		}
		name := tracks[m.Row]
		n, err := env.Store.TrackUse(name)
		if err != nil {
			return m, Fail{Err: err}, true
		}
		if n > 0 {
			return m, Open{Popup: removeBlocked(name, n, env.Apps)}, true
		}
		return m, Save{Write: SetTracks{Tracks: slices.Delete(tracks, m.Row, m.Row+1)}, Msg: "Removed Track " + name,
			place: placeRemove, Undo: tracksUndo("Track "+name+" restored", m.Ws.Tracks)}, true
	}
	return m, nil, false
}

// addOffList adds a Track Applications already use to workspace.json, so it can be
// filtered by and renamed like any other.
func (m Model) addOffList(name string) Intent {
	was := m.Ws.Tracks
	tracks := append(slices.Clone(was), name)
	// The list is checked as Tracks will read it, so a name it would refuse (a case twin
	// of a listed Track, a slash) is never written.
	if err := store.ValidTrackName(name, was); err != nil {
		return Fail{Err: fmt.Errorf("Can't list %s: %w", name, err)}
	}
	if err := store.ValidTrackList(tracks); err != nil {
		return Fail{Err: fmt.Errorf("Can't list %s: %w", name, err)}
	}
	return Save{Write: SetTracks{Tracks: tracks}, Msg: "Added Track " + name, place: placeAt, row: len(tracks) - 1,
		Undo: tracksUndo("Track "+name+" off the list again", was)}
}

// tracksUndo puts the Track list back as it was.
func tracksUndo(what string, was []string) *Undo {
	return &Undo{What: what, Restore: SetTracks{Tracks: slices.Clone(was)}}
}

// Breakdown explains each Track's count so the numbers add up: how many of its
// Applications are active, how many closed (Rejected or Withdrawn, as the Pipeline screen's
// Closed view holds), how many dismissed and how many unreadable (malformed, which Stats
// leaves out, so the Stats Track count is the rest).
func Breakdown[U any](apps []store.Application, st State[U]) map[string]string {
	closed, dismissed, unreadable := map[string]int{}, map[string]int{}, map[string]int{}
	for _, a := range apps {
		switch {
		case a.Malformed != nil:
			unreadable[a.Track]++
		case a.Status == store.StatusRejected, a.Status == store.StatusWithdrawn:
			closed[a.Track]++
		case a.Status == store.StatusDismissed:
			dismissed[a.Track]++
		}
	}
	out := map[string]string{}
	for _, t := range st.Ws.Tracks {
		c := min(closed[t], st.TrackUse[t])
		d := min(dismissed[t], st.TrackUse[t]-c)
		u := min(unreadable[t], st.TrackUse[t]-c-d)
		out[t] = " · " + strconv.Itoa(st.TrackUse[t]-c-d-u) + " active"
		if c > 0 {
			out[t] += " · " + strconv.Itoa(c) + " closed"
		}
		if d > 0 {
			out[t] += " · " + strconv.Itoa(d) + " dismissed"
		}
		if u > 0 {
			out[t] += " · " + strconv.Itoa(u) + " unreadable"
		}
	}
	return out
}
