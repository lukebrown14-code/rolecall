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

// sourcesHelpKeys are the Sources keys, shown while those rows have focus.
func (m model) sourcesHelpKeys() []helpKey {
	if src, ok := m.focusedSource(); ok {
		verb := "turn off"
		if !src.On() {
			verb = "turn on"
		}
		return []helpKey{help(groupAction, "enter", verb).at(prioCore), help(groupAction, "t", "test").at(prioCore)}
	}
	if m.onAddSource() {
		return []helpKey{help(groupAction, "enter", "add a Source").at(prioCore)}
	}
	return nil
}

// sourcesKey turns the focused Source on or off on enter, or opens the add form from
// "+ add a Source". handled is false for any other key or group.
func (m model) sourcesKey(k tea.KeyPressMsg) (next model, handled bool) {
	if k.String() != "enter" {
		return m, false
	}
	if m.onAddSource() {
		m.overlay = m.addSourcePopup()
		return m, true
	}
	src, ok := m.focusedSource()
	if !ok {
		return m, false
	}
	return m.setSourceEnabled(src.Name, !src.On()), true
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

// addSourcePopup is the form that adds a Source. Nothing is written until it is saved.
func (m model) addSourcePopup() mode {
	choices := func(values ...string) []choice {
		out := make([]choice, len(values))
		for i, v := range values {
			out[i] = choice{Value: v, Label: v}
		}
		return out
	}
	f := newFormOf("Add a Source",
		formField{Label: fieldSourceName, Required: true, Placeholder: "linkedin"},
		formField{Label: fieldSourceType, Value: store.SourceBrowser, Choices: choices(store.SourceTypes...)},
		formField{Label: fieldSourcePlatform, Placeholder: "greenhouse", Note: "api only: " + strings.Join(store.SourcePlatforms, ", ")},
		formField{Label: fieldSourceTargets, Placeholder: "cultureamp, buildkite", Note: "api only · comma-separated"},
		formField{Label: fieldSourceURL, Placeholder: "https://…", Note: "the search page, for browser or manual"},
		formField{Label: fieldSourceAccess, Value: store.AccessUnknown, Choices: choices(store.SourceAccess...)})
	return &formPopup{form: f, width: popSmall, save: func(m *model, f *form) (mode, error) {
		src := store.Source{Name: strings.TrimSpace(f.Value(fieldSourceName)), Type: f.Value(fieldSourceType),
			Platform: strings.TrimSpace(f.Value(fieldSourcePlatform)), Targets: splitPlaces(f.Value(fieldSourceTargets)),
			URL: strings.TrimSpace(f.Value(fieldSourceURL)), Access: f.Value(fieldSourceAccess)}
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

// sourceTested is a Source test's outcome, delivered to the model.
type sourceTested struct {
	name   string
	result CapturedResult
}

// sourceTestResult is the one JSON line sweep.py --test-source prints.
type sourceTestResult struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Count  int    `json:"count"`
	Access string `json:"access"`
	Note   string `json:"note"`
}

// sourceTestKey is t on a Source row: it runs the Plugin's Sweep against that one Source
// in the background and reports what it found. With no Plugin set it says where to set one.
func (m model) sourceTestKey(k tea.KeyPressMsg) (model, tea.Cmd, bool) {
	if k.String() != "t" {
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
	m.toast, m.toastHint, m.toastKind = "Testing "+src.Name+"…", "", toastNote
	args := []string{filepath.Join(m.plugin, "scripts", "sweep.py"), dataFlag, m.store.Root(), "--test-source", src.Name}
	return m, m.runCaptured("python3", args, func(r CapturedResult) tea.Msg {
		return sourceTested{name: src.Name, result: r}
	}), true
}

// finishSourceTest reports a Source test as a toast: "greenhouse: works · 41 jobs", or
// the access problem and the script's note. Output that is not the expected line is an error.
func (m model) finishSourceTest(done sourceTested) model {
	line := strings.TrimSpace(done.result.Stdout)
	if i := strings.LastIndex(line, "\n"); i >= 0 {
		line = line[i+1:]
	}
	var r sourceTestResult
	if err := json.Unmarshal([]byte(line), &r); err != nil {
		out := combinedOutput(done.result)
		if out == "" && done.result.Err != nil {
			out = done.result.Err.Error()
		}
		m.err = fmt.Errorf("testing %s failed: %s", done.name, firstLine(out))
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
