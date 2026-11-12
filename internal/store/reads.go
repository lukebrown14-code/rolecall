package store

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Tracks returns configured tracks in display order; absent tracks mean no filter.
func (s *Store) Tracks() ([]string, error) {
	path := filepath.Join(s.root, "workspace.json")
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	var config struct {
		Schema json.RawMessage `json:"schema"`
		Tracks []string        `json:"tracks"`
	}
	if err = json.Unmarshal(b, &config); err != nil {
		return nil, fmt.Errorf("%s: invalid Workspace settings", path)
	}
	if _, err := workspaceSchemaVersion(path, config.Schema); err != nil {
		return nil, err
	}
	return config.Tracks, nil
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
