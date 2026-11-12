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
	if config.Schema != nil {
		if bytes.Equal(config.Schema, []byte("null")) {
			return nil, fmt.Errorf("%s: invalid Workspace schema", path)
		}
		var version int
		if err := json.Unmarshal(config.Schema, &version); err != nil {
			return nil, fmt.Errorf("%s: invalid Workspace schema", path)
		}
		if version != 1 {
			if version > 1 {
				return nil, fmt.Errorf("%s: Workspace schema %d is newer than this version; upgrade rolecall", path, version)
			}
			return nil, fmt.Errorf("%s: unsupported Workspace schema %d", path, version)
		}
	}
	return config.Tracks, nil
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
