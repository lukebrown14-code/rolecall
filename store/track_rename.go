package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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

// RenameTrack renames a Track across the Workspace: the track of every readable meta.yml
// under jobs/ (archive included, written with the usual meta.yml rules), then tracks in
// workspace.json, then the role tracks in jobs/roles.json. Folders never move, and
// malformed meta.yml files are skipped and reported. Both JSON files are checked before
// anything is written.
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
		return res, err
	}
	tracks[i] = name
	if err := s.SetTracks(tracks); err != nil {
		return res, err
	}
	if hasRoles {
		rolesPath := filepath.Join(s.root, "jobs", "roles.json")
		out, err := json.Marshal(name)
		if err != nil {
			return res, err
		}
		oldJSON, _ := json.Marshal(old)
		re := regexp.MustCompile(`("track"\s*:\s*)` + regexp.QuoteMeta(string(oldJSON)))
		info, err := os.Stat(rolesPath)
		if err != nil {
			return res, err
		}
		next := re.ReplaceAllString(string(roles), "${1}"+strings.ReplaceAll(string(out), "$", "$$"))
		if err := s.atomicWrite(rolesPath, []byte(next), info.Mode().Perm()); err != nil {
			return res, err
		}
		res.Roles = true
	}
	return res, nil
}

// ValidTrackName applies the Track name rules to an already trimmed name: non-empty,
// unique ignoring case among existing, no slash (it becomes a folder name).
func ValidTrackName(name string, existing []string) error {
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
