package store

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// workspace is workspace.json read once. Keys and Values are the ordered top-level keys
// and their raw values, so a writer can preserve every key it does not know; ParseErr is
// set when the file is present but is not a JSON object; Raw is nil when the file is
// absent. Readers project their fields from Values, so the file is read and parsed in one
// place.
type workspace struct {
	Path     string
	Raw      []byte
	Keys     []string
	Values   map[string]json.RawMessage
	ParseErr error
}

// readWorkspace reads workspace.json. A missing file returns a zero Raw with no error; an
// unreadable file returns the error. Every reader and writer goes through it.
func readWorkspace(root string) (workspace, error) {
	w := workspace{Path: filepath.Join(root, "workspace.json")}
	b, err := os.ReadFile(w.Path)
	if os.IsNotExist(err) {
		return w, nil
	}
	if err != nil {
		return w, err
	}
	w.Raw = b
	keys, values, err := orderedObject(b)
	if err != nil {
		// Report the message json.Unmarshal gives: it is the one the Settings screen has
		// always shown for a broken file.
		var probe map[string]json.RawMessage
		if perr := json.Unmarshal(b, &probe); perr != nil {
			err = perr
		}
		w.ParseErr = err
		return w, nil
	}
	w.Keys, w.Values = keys, values
	return w, nil
}

// schema is the Workspace schema, 1 when the key is absent, or an error naming the path.
func (w workspace) schema() (int, error) {
	return workspaceSchemaVersion(w.Path, w.Values["schema"])
}

// Tracks returns configured tracks in display order; absent tracks mean no filter.
func (s *Store) Tracks() ([]string, error) {
	w, err := readWorkspace(s.root)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", w.Path, err)
	}
	if w.Raw == nil {
		return nil, nil
	}
	if w.ParseErr != nil {
		return nil, fmt.Errorf("%s: invalid Workspace settings", w.Path)
	}
	if _, err := w.schema(); err != nil {
		return nil, err
	}
	var tracks []string
	if raw, ok := w.Values["tracks"]; ok {
		if err := json.Unmarshal(raw, &tracks); err != nil {
			return nil, fmt.Errorf("%s: invalid Workspace settings", w.Path)
		}
	}
	if err := checkTracks(tracks); err != nil {
		return nil, &TrackListError{Path: w.Path, Err: err}
	}
	return tracks, nil
}

// TrackListError is a workspace.json whose tracks list breaks the Track name rules.
// The file still parses, so the app can start without a Track filter and say why.
type TrackListError struct {
	Path string
	Err  error
}

func (e *TrackListError) Error() string { return fmt.Sprintf("%s: tracks: %v", e.Path, e.Err) }
func (e *TrackListError) Unwrap() error { return e.Err }

// checkTracks applies the Track name rules Settings applies to a new Track to a
// hand-edited list: each one non-empty, without surrounding spaces, and listed once.
func checkTracks(tracks []string) error {
	for i, t := range tracks {
		if t != strings.TrimSpace(t) {
			return fmt.Errorf("Track %q has spaces around it", DisplayText(t))
		}
		if t != DisplayText(t) {
			return fmt.Errorf("Track %q contains a control character", DisplayText(t))
		}
		if err := ValidTrackName(t, nil); err != nil {
			return err
		}
		for _, earlier := range tracks[:i] {
			if strings.EqualFold(earlier, t) {
				return fmt.Errorf("Track %s is listed twice", DisplayText(t))
			}
		}
	}
	return nil
}

// workspaceSchemaVersion reads the schema key of workspace.json. Absent or 1 is accepted;
// null, a non-number, or any other version is an error naming the path, so a reader or
// writer can refuse a Workspace this build does not understand.
func workspaceSchemaVersion(path string, raw json.RawMessage) (int, error) {
	if raw == nil {
		return 1, nil
	}
	if bytes.Equal(raw, []byte("null")) {
		return 0, fmt.Errorf("%s: invalid Workspace schema", path)
	}
	var version int
	if err := json.Unmarshal(raw, &version); err != nil {
		return 0, fmt.Errorf("%s: invalid Workspace schema", path)
	}
	switch {
	case version == 1:
		return 1, nil
	case version > 1:
		return version, fmt.Errorf("%s: Workspace schema %d is newer than this version; upgrade rolecall", path, version)
	default:
		return version, fmt.Errorf("%s: unsupported Workspace schema %d", path, version)
	}
}

// FindByURL returns every exact URL match, regardless of Status.
func (s *Store) FindByURL(url string) ([]Application, error) {
	apps, err := s.Scan()
	if err != nil {
		return nil, err
	}
	matches := []Application{}
	for _, a := range apps {
		if a.URL == url {
			matches = append(matches, a)
		}
	}
	return matches, nil
}
