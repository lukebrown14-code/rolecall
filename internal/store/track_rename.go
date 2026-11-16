package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// RenameImpact is what renaming a Track will touch.
type RenameImpact struct {
	Applications int  // readable Applications on the Track, archived ones included
	Roles        bool // roles.json has a role on the Track
}

// RenameResult is what a rename changed. Skipped names the malformed meta.yml files that
// carry the old Track, as paths under jobs/, so the Owner can fix them by hand.
type RenameResult struct {
	RenameImpact
	Skipped []string
}

// TrackRenameImpact counts what RenameTrack would change, without writing.
func (s *Store) TrackRenameImpact(old string) (RenameImpact, error) {
	var impact RenameImpact
	err := s.eachMeta(func(dir string, b []byte) error {
		if a := readApplication(dir, b); a.Malformed == nil && a.Track == old {
			impact.Applications++
		}
		return nil
	})
	if err != nil {
		return impact, err
	}
	_, impact.Roles, err = s.rolesWithTrack(old)
	return impact, err
}

// TrackUse counts the Applications on a Track, archived and malformed ones included, so
// removing a Track is refused while anything still carries it.
func (s *Store) TrackUse(track string) (int, error) {
	n := 0
	err := s.eachMeta(func(dir string, b []byte) error {
		if a := readApplication(dir, b); a.Track == track || (a.Malformed != nil && rawTrack(b) == track) {
			n++
		}
		return nil
	})
	return n, err
}

// TrackUseAll counts the Applications on every Track in one pass over the Workspace,
// archived and malformed ones included, so a caller showing each Track's use does not
// walk the whole tree once per Track.
func (s *Store) TrackUseAll() (map[string]int, error) {
	counts := map[string]int{}
	err := s.eachMeta(func(dir string, b []byte) error {
		a := readApplication(dir, b)
		if a.Malformed != nil {
			counts[rawTrack(b)]++
			return nil
		}
		counts[a.Track]++
		return nil
	})
	return counts, err
}

// RenameTrack renames a Track across the Workspace: the track of every readable meta.yml
// under jobs/ (archive included, written with the usual meta.yml rules), then the role
// tracks in jobs/roles.json, then tracks in workspace.json last. Folders never move, and
// malformed meta.yml files are skipped and reported. Both JSON files are checked before
// anything is written. A failure partway returns what was changed with the error; since
// workspace.json still lists the old Track, running the same rename again finishes it.
func (s *Store) RenameTrack(old, name string) (RenameResult, error) {
	var res RenameResult
	name = strings.TrimSpace(name)
	tracks, err := s.Tracks()
	if err != nil {
		return res, fmt.Errorf("workspace.json: %w", err)
	}
	i := slices.Index(tracks, old)
	if i < 0 {
		return res, fmt.Errorf("Track %s is not in workspace.json", old)
	}
	if err := ValidTrackName(name, slices.Delete(slices.Clone(tracks), i, i+1)); err != nil {
		return res, err
	}
	roles, hasRoles, err := s.rolesWithTrack(old)
	if err != nil {
		return res, err
	}
	rolesPath := filepath.Join(s.root, "jobs", "roles.json")
	var nextRoles []byte
	var rolesMode os.FileMode
	if hasRoles {
		if nextRoles, err = renameRoleTracks(roles, old, name); err != nil {
			return res, err
		}
		info, err := os.Stat(rolesPath)
		if err != nil {
			return res, err
		}
		rolesMode = info.Mode().Perm()
	}
	partial := func(err error) error {
		return fmt.Errorf("renamed %d of the Applications on %s, then failed (run the rename again to finish): %w",
			res.Applications, old, err)
	}
	err = s.eachMeta(func(dir string, b []byte) error {
		a := readApplication(dir, b)
		switch {
		case a.Malformed != nil:
			if rawTrack(b) == old {
				rel, _ := filepath.Rel(filepath.Join(s.root, "jobs"), dir)
				res.Skipped = append(res.Skipped, rel)
			}
		case a.Track == old:
			if err := s.Patch(dir, map[string]string{keyTrack: name}); err != nil {
				return err
			}
			res.Applications++
		}
		return nil
	})
	if err != nil {
		return res, partial(err)
	}
	if hasRoles {
		if err := s.atomicWrite(rolesPath, nextRoles, rolesMode); err != nil {
			return res, partial(err)
		}
		res.Roles = true
	}
	tracks[i] = name
	if err := s.SetTracks(tracks); err != nil {
		return res, partial(err)
	}
	return res, nil
}

// renameRoleTracks rewrites the track value of each role in jobs/roles.json, and nothing
// else: key order, formatting and every unrelated value stay byte for byte. It walks the
// JSON tokens, so a "track" key nested inside another value is never matched.
func renameRoleTracks(raw []byte, old, name string) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	type frame struct {
		object bool   // this container is an object
		role   bool   // this object is an element of the roles array
		roles  bool   // this array is the roles array
		key    string // the key awaiting its value
		hasKey bool
	}
	var stack []*frame
	top := func() *frame {
		if len(stack) == 0 {
			return nil
		}
		return stack[len(stack)-1]
	}
	var edits [][2]int
	prev := int64(0)
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		cur := dec.InputOffset()
		switch t := tok.(type) {
		case json.Delim:
			switch t {
			case '{':
				stack = append(stack, &frame{object: true, role: top() != nil && top().roles})
			case '[':
				f := top()
				isRoles := f != nil && f.object && f.hasKey && f.key == rolesKey
				stack = append(stack, &frame{roles: isRoles})
			case '}', ']':
				stack = stack[:len(stack)-1]
				if f := top(); f != nil && f.object {
					f.hasKey = false
				}
			}
		case string:
			if f := top(); f != nil && f.object {
				if !f.hasKey {
					f.key, f.hasKey = t, true
				} else {
					if f.role && f.key == keyTrack && t == old {
						// Skip the whitespace and the ":" between the key and the value to
						// replace only the value's bytes.
						span := raw[prev:cur]
						i := 0
						for i < len(span) && (span[i] == ' ' || span[i] == '\t' || span[i] == '\r' || span[i] == '\n' || span[i] == ':') {
							i++
						}
						edits = append(edits, [2]int{int(prev) + i, int(cur)})
					}
					f.hasKey = false
				}
			}
		default:
			if f := top(); f != nil && f.object && f.hasKey {
				f.hasKey = false
			}
		}
		prev = cur
	}
	enc, err := json.Marshal(name)
	if err != nil {
		return nil, err
	}
	out := raw
	for i := len(edits) - 1; i >= 0; i-- {
		e := edits[i]
		next := make([]byte, 0, len(out)-(e[1]-e[0])+len(enc))
		next = append(next, out[:e[0]]...)
		next = append(next, enc...)
		next = append(next, out[e[1]:]...)
		out = next
	}
	return out, nil
}

// ValidTrackName applies the Track name rules to an already trimmed name: non-empty,
// unique ignoring case among existing, and a safe folder name (no slash or backslash,
// and not one of the reserved names `.`, `..`, `_archive`). The Add form, RenameTrack and
// Create all use it, so they cannot disagree about what a Track may be called.
func ValidTrackName(name string, existing []string) error {
	switch {
	case name == "":
		return errors.New("Track name is empty")
	case strings.ContainsAny(name, `/\`):
		return errors.New("Track name cannot contain /")
	case name == "." || name == ".." || name == "_archive":
		return fmt.Errorf("Track name cannot be %s", name)
	}
	for _, t := range existing {
		if strings.EqualFold(t, name) {
			return fmt.Errorf("Track %s already exists", t)
		}
	}
	return nil
}

// eachMeta calls fn for every readable meta.yml one level below jobs/, archive included.
func (s *Store) eachMeta(fn func(dir string, b []byte) error) error {
	jobs := filepath.Join(s.root, "jobs")
	groups, err := os.ReadDir(jobs)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, g := range groups {
		if !g.IsDir() {
			continue
		}
		folders, err := os.ReadDir(filepath.Join(jobs, g.Name()))
		if err != nil {
			return err
		}
		for _, f := range folders {
			dir := filepath.Join(jobs, g.Name(), f.Name())
			b, err := os.ReadFile(filepath.Join(dir, "meta.yml"))
			if errors.Is(err, os.ErrNotExist) || !f.IsDir() {
				continue
			}
			if err != nil {
				return err
			}
			if err := fn(dir, b); err != nil {
				return err
			}
		}
	}
	return nil
}

// rolesKey is the top-level key holding the roles array in jobs/roles.json.
const rolesKey = "roles"

// rawTrack reads the track line of a meta.yml that may not parse as a whole.
func rawTrack(b []byte) string {
	for _, l := range strings.Split(string(b), "\n") {
		if v, ok := strings.CutPrefix(l, keyTrack+":"); ok {
			tok, _, err := scalarToken(v)
			if err != nil {
				return ""
			}
			return strings.Trim(strings.TrimSpace(tok), `"'`)
		}
	}
	return ""
}

// rolesWithTrack reads jobs/roles.json (absent is fine, invalid is an error) and reports
// whether any role is on the Track.
func (s *Store) rolesWithTrack(track string) (raw []byte, found bool, err error) {
	path := filepath.Join(s.root, "jobs", "roles.json")
	raw, err = os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var doc struct {
		Roles []struct {
			Track string `json:"track"`
		} `json:"roles"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, false, fmt.Errorf("%s is not valid JSON, not writing: %w", path, err)
	}
	for _, r := range doc.Roles {
		if r.Track == track {
			return raw, true, nil
		}
	}
	return raw, false, nil
}
