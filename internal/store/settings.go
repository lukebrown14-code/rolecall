package store

import (
	"encoding/json"
	"fmt"
	"path/filepath"
)

// Default Build gates, as plugin/scripts/build.py applies them when workspace.json sets none.
const (
	DefaultLetterWordCap = 350
	DefaultLetterWordMin = 150
	DefaultKeySkillsMin  = 8
	DefaultKeySkillsMax  = 12
)

// WorkspaceSettings is workspace.json as the Settings screen shows it. A missing file or
// key means the default. An unreadable file sets Err and leaves the rest at defaults.
type WorkspaceSettings struct {
	Path          string
	Err           error // parse or schema error for a workspace.json this build cannot use
	Tracks        []string
	Places        []string
	Remote        string
	LetterWordCap int
	LetterWordMin int
	KeySkillsMin  int
	KeySkillsMax  int
	Sources       int // entries in sources
	// Rules are the Search rules; SourceList is the sources registry, a legacy object
	// read as one api Source per platform.
	Rules      SearchRules
	SourceList []Source
	Blockers   int // entries in blockers
	// Visa is the Owner's visa limits; PayFX overrides DefaultPayFX.
	Visa  Visa
	PayFX map[string]float64
	// Schema is the workspace.json schema, 1 when the key is absent. NewerSchema marks a
	// schema above 1, so the screen can say the file is from a newer rolecall.
	Schema      int
	NewerSchema bool
	// Set names the gate keys workspace.json sets itself; the rest are the defaults.
	Set map[string]bool
}

// Settings reads workspace.json without writing it. Invalid JSON is reported in Err, not
// returned, so the screen can show it; only an I/O failure is an error.
func (s *Store) Settings() (WorkspaceSettings, error) {
	out := WorkspaceSettings{
		Path:          filepath.Join(s.root, "workspace.json"),
		Schema:        1,
		LetterWordCap: DefaultLetterWordCap, LetterWordMin: DefaultLetterWordMin,
		KeySkillsMin: DefaultKeySkillsMin, KeySkillsMax: DefaultKeySkillsMax,
		Rules: SearchRules{ExpiryWarningDays: DefaultExpiryWarningDays, Set: map[string]bool{}},
	}
	w, err := readWorkspace(s.root)
	if err != nil {
		return out, err
	}
	if w.Raw == nil {
		return out, nil
	}
	if w.ParseErr != nil {
		out.Err = w.ParseErr
		return out, nil
	}
	// A schema this build cannot use stops the screen: it must not offer edits that would
	// be refused, and the Owner needs to see why.
	if version, err := w.schema(); err != nil {
		out.Schema, out.Err, out.NewerSchema = version, err, version > 1
		return out, nil
	} else {
		out.Schema = version
	}
	// decode reads one top-level key; a key present but of the wrong shape is a file error.
	decode := func(key string, dst any) bool {
		raw, ok := w.Values[key]
		if !ok {
			return true
		}
		if err := json.Unmarshal(raw, dst); err != nil {
			out.Err = fmt.Errorf("%s: %w", w.Path, err)
			return false
		}
		return true
	}
	if !decode("tracks", &out.Tracks) {
		return out, nil
	}
	var area struct {
		Places []string `json:"places"`
		Remote string   `json:"remote"`
	}
	if !decode("search_area", &area) {
		return out, nil
	}
	out.Places, out.Remote = area.Places, area.Remote
	out.Set = map[string]bool{}
	for _, g := range []struct {
		key string
		dst *int
	}{
		{GateLetterCap, &out.LetterWordCap}, {GateLetterMin, &out.LetterWordMin},
		{GateSkillsMin, &out.KeySkillsMin}, {GateSkillsMax, &out.KeySkillsMax},
	} {
		if !decode(g.key, g.dst) {
			return out, nil
		}
		if _, ok := w.Values[g.key]; ok {
			out.Set[g.key] = true
		}
	}
	rules, err := decodeSearchRules(w.Values["search_rules"])
	if err != nil {
		out.Err = fmt.Errorf("%s: search_rules: %w", w.Path, err)
		return out, nil
	}
	if out.Visa, err = decodeVisa(w.Values[keyVisa]); err != nil {
		out.Err = fmt.Errorf("%s: visa: %w", w.Path, err)
		return out, nil
	}
	if out.PayFX, err = decodePayFX(w.Values[keyPayFX]); err != nil {
		out.Err = fmt.Errorf("%s: pay_fx: %w", w.Path, err)
		return out, nil
	}
	out.Rules, out.SourceList = rules, decodeSources(w.Values["sources"])
	out.Sources, out.Blockers = entryCount(w.Values["sources"]), entryCount(w.Values["blockers"])
	return out, nil
}

// entryCount counts the elements of a JSON array or the keys of an object; anything else is 0.
func entryCount(raw json.RawMessage) int {
	var list []json.RawMessage
	if json.Unmarshal(raw, &list) == nil {
		return len(list)
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) == nil {
		return len(obj)
	}
	return 0
}
