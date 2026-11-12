package store

import (
	"encoding/json"
	"os"
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
	Err           error // parse error for an invalid workspace.json
	Tracks        []string
	Places        []string
	Remote        string
	LetterWordCap int
	LetterWordMin int
	KeySkillsMin  int
	KeySkillsMax  int
	Sources       int // entries in sources
	Blockers      int // entries in blockers
	// Set names the gate keys workspace.json sets itself; the rest are the defaults.
	Set map[string]bool
}

// Settings reads workspace.json without writing it. Invalid JSON is reported in Err, not
// returned, so the screen can show it; only an I/O failure is an error.
func (s *Store) Settings() (WorkspaceSettings, error) {
	out := WorkspaceSettings{
		Path:          filepath.Join(s.root, "workspace.json"),
		LetterWordCap: DefaultLetterWordCap, LetterWordMin: DefaultLetterWordMin,
		KeySkillsMin: DefaultKeySkillsMin, KeySkillsMax: DefaultKeySkillsMax,
	}
	b, err := os.ReadFile(out.Path)
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	var raw struct {
		Tracks     []string `json:"tracks"`
		SearchArea struct {
			Places []string `json:"places"`
			Remote string   `json:"remote"`
		} `json:"search_area"`
		LetterWordCap *int            `json:"letterWordCap"`
		LetterWordMin *int            `json:"letterWordMin"`
		KeySkillsMin  *int            `json:"keySkillsMin"`
		KeySkillsMax  *int            `json:"keySkillsMax"`
		Sources       json.RawMessage `json:"sources"`
		Blockers      json.RawMessage `json:"blockers"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		out.Err = err
		return out, nil
	}
	out.Tracks, out.Places, out.Remote = raw.Tracks, raw.SearchArea.Places, raw.SearchArea.Remote
	out.Set = map[string]bool{}
	for _, g := range []struct {
		key      string
		dst, src *int
	}{
		{GateLetterCap, &out.LetterWordCap, raw.LetterWordCap}, {GateLetterMin, &out.LetterWordMin, raw.LetterWordMin},
		{GateSkillsMin, &out.KeySkillsMin, raw.KeySkillsMin}, {GateSkillsMax, &out.KeySkillsMax, raw.KeySkillsMax},
	} {
		if g.src != nil {
			*g.dst = *g.src
			out.Set[g.key] = true
		}
	}
	out.Sources, out.Blockers = entryCount(raw.Sources), entryCount(raw.Blockers)
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
