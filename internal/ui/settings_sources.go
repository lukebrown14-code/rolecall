package ui

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/internal/store"
	"github.com/lukebrown14-code/rolecall/internal/ui/settings"
)

// Source form field labels.
const (
	fieldSourceName     = "Name"
	fieldSourceType     = "Type"
	fieldSourcePlatform = "Platform"
	fieldSourceTargets  = "Targets"
	fieldSourceURL      = "URL"
	fieldSourceAccess   = "Access"
)

// focusedSource is the Source under the cursor, if the cursor is on one.
func (m model) focusedSource() (store.Source, bool) {
	st := m.settings
	if st == nil || st.Ws.Err != nil || !m.settingsRowsFocused() || st.Group != settings.GroupSources || st.Row >= len(st.Ws.SourceList) {
		return store.Source{}, false
	}
	return st.Ws.SourceList[st.Row], true
}

// onAddSource reports whether the cursor is on "+ add a Source".
func (m model) onAddSource() bool {
	st := m.settings
	return st != nil && st.Ws.Err == nil && m.settingsRowsFocused() && st.Group == settings.GroupSources && st.Row == len(st.Ws.SourceList)
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

// sourceToggleVerb is what o does to src: "turn off" or "turn on".
func sourceToggleVerb(src store.Source) string {
	if src.On() {
		return "turn off"
	}
	return "turn on"
}

// sourcesHelpKeys are the Sources keys, shown while those rows have focus.
func (m model) sourcesHelpKeys() []helpKey {
	if src, ok := m.focusedSource(); ok {
		return []helpKey{help(groupAction, "enter", "edit").at(prioCore),
			help(groupAction, keySourceToggle, sourceToggleVerb(src)).at(prioCore), help(groupAction, keySourceCheck, "check").at(prioCore)}
	}
	if m.onAddSource() {
		return []helpKey{help(groupAction, "enter", "add a Source").at(prioCore)}
	}
	return nil
}

// sourcesKey opens the focused Source's form on enter (the add form from "+ add a
// Source") and turns it on or off on o. handled is false for any other key or group.
func (m model) sourcesKey(k tea.KeyPressMsg) (next model, handled bool) {
	switch k.String() {
	case "enter":
		if m.onAddSource() {
			m.overlay = m.addSourcePopup()
			return m, true
		}
		src, ok := m.focusedSource()
		if !ok {
			return m, false
		}
		m.overlay = m.editSourcePopup(src)
		return m, true
	case keySourceToggle:
		src, ok := m.focusedSource()
		if !ok {
			return m, false
		}
		return m.setSourceEnabled(src.Name, !src.On()), true
	}
	return m, false
}

// setSourceEnabled saves a Source's on/off and offers u to put it back.
func (m model) setSourceEnabled(name string, on bool) model {
	if err := m.store.SetSourceEnabled(name, on); err != nil {
		m.err = err
		return m
	}
	m.err = nil
	m = m.loadSettings()
	word := map[bool]string{true: "on", false: "off"}
	return m.saved("Turned "+word[on]+" "+name, settingsFile, &settingsUndo{
		what: name + " back " + word[!on],
		restore: func(m model) model {
			if err := m.store.SetSourceEnabled(name, !on); err != nil {
				m.err = err
				return m
			}
			return m.loadSettings()
		},
	})
}

// sourceForm is the add and edit form of a Source, opening with src's values.
func sourceForm(title string, src store.Source) *form {
	choices := func(values ...string) []choice {
		out := make([]choice, len(values))
		for i, v := range values {
			out[i] = choice{Value: v, Label: v}
		}
		return out
	}
	return newFormOf(title,
		formField{Label: fieldSourceName, Required: true, Value: src.Name, Placeholder: "linkedin"},
		formField{Label: fieldSourceType, Value: src.Type, Choices: choices(store.SourceTypes...)},
		formField{Label: fieldSourcePlatform, Value: src.Platform, Placeholder: "greenhouse", Note: "api only: " + strings.Join(store.SourcePlatforms, ", ")},
		formField{Label: fieldSourceTargets, Value: strings.Join(src.Targets, ", "), Placeholder: "cultureamp, buildkite", Note: "api only · comma-separated"},
		formField{Label: fieldSourceURL, Value: src.URL, Placeholder: "https://…", Note: "the search page, for browser or manual"},
		formField{Label: fieldSourceAccess, Value: src.AccessText(), Choices: choices(store.SourceAccess...)})
}

// formSource is the Source a saved sourceForm describes.
func formSource(f *form) store.Source {
	return store.Source{Name: strings.TrimSpace(f.Value(fieldSourceName)), Type: f.Value(fieldSourceType),
		Platform: strings.TrimSpace(f.Value(fieldSourcePlatform)), Targets: splitPlaces(f.Value(fieldSourceTargets)),
		URL: strings.TrimSpace(f.Value(fieldSourceURL)), Access: f.Value(fieldSourceAccess)}
}

// addSourcePopup is the form that adds a Source. Nothing is written until it is saved.
func (m model) addSourcePopup() mode {
	f := sourceForm("Add a Source", store.Source{Type: store.SourceBrowser})
	return &formPopup{form: f, width: popSmall, save: func(m *model, f *form) (mode, error) {
		src := formSource(f)
		if err := m.store.AddSource(src); err != nil {
			return nil, err
		}
		m.err = nil
		*m = m.loadSettings()
		*m = m.saved("Added Source "+src.Name, settingsFile, &settingsUndo{
			what: "Source " + src.Name + " removed",
			restore: func(m model) model {
				if err := m.store.RemoveSource(src.Name); err != nil {
					m.err = err
					return m
				}
				return m.loadSettings()
			},
		})
		return nil, nil
	}}
}

// editSourcePopup is enter on a Source: its form, opening with its values. Saving
// rewrites those fields in place and offers u to put them back; on or off is o's.
func (m model) editSourcePopup(old store.Source) mode {
	f := sourceForm("Edit Source "+old.Name, old)
	f.ShowDirty = true
	return &formPopup{form: f, width: popSmall, save: func(m *model, f *form) (mode, error) {
		if !f.Dirty() {
			return nil, nil
		}
		src := formSource(f)
		if err := m.store.UpdateSource(old.Name, src); err != nil {
			return nil, err
		}
		m.err = nil
		*m = m.loadSettings()
		*m = m.saved("Saved Source "+src.Name, settingsFile, &settingsUndo{
			what: "Source " + old.Name + " as it was",
			restore: func(m model) model {
				if err := m.store.UpdateSource(src.Name, old); err != nil {
					m.err = err
					return m
				}
				return m.loadSettings()
			},
		})
		return nil, nil
	}}
}

// sourceChecked is a Source check's outcome, delivered to the model.
type sourceChecked struct {
	name   string
	result CapturedResult
}

// sourceCheckResult is the one JSON line sweep.py --test-source prints.
type sourceCheckResult struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Count  int    `json:"count"`
	Access string `json:"access"`
	Note   string `json:"note"`
}

// sourceCheckKey is c on a Source row: it runs the Plugin's Sweep against that one
// Source in the background (sweep.py --test-source) and reports what it found. With no
// Plugin set it says where to set one.
func (m model) sourceCheckKey(k tea.KeyPressMsg) (model, tea.Cmd, bool) {
	if k.String() != keySourceCheck {
		return m, nil, false
	}
	src, ok := m.focusedSource()
	if !ok {
		return m, nil, false
	}
	if m.plugin == "" {
		m.err = errors.New(unsetMachineSetting(labelPlugin))
		return m, nil, true
	}
	m.toast, m.toastHint, m.toastKind = "Checking "+src.Name+"…", "", toastNote
	args := []string{filepath.Join(m.plugin, "scripts", "sweep.py"), dataFlag, m.store.Root(), "--test-source", src.Name}
	return m, m.runCaptured("python3", args, func(r CapturedResult) tea.Msg {
		return sourceChecked{name: src.Name, result: r}
	}), true
}

// finishSourceCheck reports a Source check as a toast: "greenhouse: works · 41 jobs", or
// the access problem and the script's note. Output that is not the expected line is an error.
func (m model) finishSourceCheck(done sourceChecked) model {
	line := strings.TrimSpace(done.result.Stdout)
	if i := strings.LastIndex(line, "\n"); i >= 0 {
		line = line[i+1:]
	}
	var r sourceCheckResult
	if err := json.Unmarshal([]byte(line), &r); err != nil {
		out := combinedOutput(done.result)
		if out == "" && done.result.Err != nil {
			out = done.result.Err.Error()
		}
		m.err = fmt.Errorf("checking %s failed: %s", done.name, firstLine(out))
		return m
	}
	msg := done.name + ": " + r.Access
	if r.OK {
		msg += " · " + strconv.Itoa(r.Count) + " jobs"
	}
	if r.Note != "" {
		msg += " · " + r.Note
	}
	m.toast, m.toastHint, m.toastKind = msg, "", toastPlain
	return m
}

// firstLine is the first non-blank line of s.
func firstLine(s string) string {
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			return l
		}
	}
	return "no output"
}
