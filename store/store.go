// Package store reads Applications from a Workspace.
package store

import (
	"errors"
	"os"
	"path/filepath"
)

type Application struct {
	Folder, Title, Company, Track, Status string
	Schema                                int
	URL, Closes, Fit                      string
	ResumeOnly                            bool
	StatusLog                             []StatusEntry
	AppliedDate, LastChange               *StatusEntry
	Malformed                             error `json:"-"`
}
type StatusEntry struct {
	Status string
	At     string
	Approx bool
}
type Store struct{ root string }

func Open(root string) (*Store, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, &os.PathError{Op: "open", Path: root, Err: errors.New("Workspace is not a directory")}
	}
	return &Store{root: root}, nil
}

func (s *Store) Scan() ([]Application, error) {
	apps := []Application{}
	tracks, err := os.ReadDir(filepath.Join(s.root, "jobs"))
	if errors.Is(err, os.ErrNotExist) {
		return apps, nil
	}
	if err != nil {
		return nil, err
	}
	for _, track := range tracks {
		if !track.IsDir() || track.Name() == "_archive" {
			continue
		}
		folders, err := os.ReadDir(filepath.Join(s.root, "jobs", track.Name()))
		if err != nil {
			return nil, err
		}
		for _, folder := range folders {
			if !folder.IsDir() {
				continue
			}
			dir := filepath.Join(s.root, "jobs", track.Name(), folder.Name())
			b, err := os.ReadFile(filepath.Join(dir, "meta.yml"))
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				return nil, err
			}
			a := readApplication(dir, b)
			apps = append(apps, a)
		}
	}
	return apps, nil
}
