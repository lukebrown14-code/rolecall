package store

import (
	"errors"
	"os"
	"path/filepath"
)

// EnsureNotes creates an empty notes file without changing existing notes.
func (s *Store) EnsureNotes(folder string) error {
	if err := s.applicationRecord(folder); err != nil {
		return err
	}
	path := filepath.Join(folder, "notes.md")
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return s.atomicWrite(path, nil, privateFileMode)
}
