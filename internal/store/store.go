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
	JDHasText, ResumeHasText, CoverLetterHasText, GapsHasText            bool
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

// application is the one gate for folder-scoped methods: a folder is an Application
// only when it holds a meta.yml. A missing meta.yml returns os.ErrNotExist. Callers that
// need the parsed record use applicationRecord.
func (s *Store) application(folder string) (Application, error) {
	if _, err := os.Stat(filepath.Join(folder, "meta.yml")); err != nil {
		return Application{}, err
	}
	return Application{Folder: folder}, nil
}

// applicationRecord is the gate plus the parsed Application: a malformed meta.yml is
// refused as an error. Every write uses it.
func (s *Store) applicationRecord(folder string) (Application, error) {
	if _, err := s.application(folder); err != nil {
		return Application{}, err
	}
	a, err := s.readApplicationFile(folder)
	if err != nil {
		return Application{}, err
	}
	if a.Malformed != nil {
		return Application{}, a.Malformed
	}
	return a, nil
}

// readApplicationFile reads the Application at folder without refusing a malformed
// meta.yml. A missing meta.yml returns os.ErrNotExist.
func (s *Store) readApplicationFile(folder string) (Application, error) {
	b, err := os.ReadFile(filepath.Join(folder, "meta.yml"))
	if err != nil {
		return Application{}, err
	}
	return readApplication(folder, b), nil
}

func (s *Store) Rescan(dir string) (*Application, error) {
	a, err := s.readApplicationFile(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err = readFolder(&a); err != nil {
		return nil, err
	}
	return &a, nil
}

// Root is the Workspace folder.
func (s *Store) Root() string { return s.root }
