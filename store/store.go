// Package store reads Applications from a Workspace.
package store

import (
	"errors"
	"os"
	"path/filepath"
)

type Application struct {
	Folder, Title, Company, Track, Status                                string
	Documents                                                            string
	PDFs                                                                 []string
	HasJD, HasNotes, HasFit, HasGaps, HasResume, HasCoverLetter, HasSent bool
	Schema                                                               int
	URL, Closes, Fit                                                     string
	ResumeOnly                                                           bool
	StatusLog                                                            []StatusEntry
	AppliedDate, LastChange                                              *StatusEntry
	Malformed                                                            error `json:"-"`
}
type StatusEntry struct {
	Status string
	At     string
	Approx bool
}
type Store struct {
	root  string
	files FileSystem
}

func Open(root string) (*Store, error) { return OpenWithFileSystem(root, nil) }

// OpenWithFileSystem opens a Workspace with an atomic publication filesystem.
func OpenWithFileSystem(root string, files FileSystem) (*Store, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, &os.PathError{Op: "open", Path: root, Err: errors.New("Workspace is not a directory")}
	}
	return &Store{root: root, files: files}, nil
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
			a, err := s.Rescan(dir)
			if err != nil {
				return nil, err
			}
			if a != nil {
				apps = append(apps, *a)
			}
		}
	}
	return apps, nil
}

func (s *Store) Rescan(dir string) (*Application, error) {
	b, err := os.ReadFile(filepath.Join(dir, "meta.yml"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	a := readApplication(dir, b)
	if err = readFolder(&a); err != nil {
		return nil, err
	}
	return &a, nil
}

// Root is the Workspace folder.
func (s *Store) Root() string { return s.root }
