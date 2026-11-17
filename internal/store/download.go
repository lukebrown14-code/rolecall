package store

import (
	"os"
	"path/filepath"
)

// CopyPDF copies out/<name> of the Application folder into dir under the same name,
// creating dir if needed. If a file of that name already exists in dir it returns an
// error satisfying errors.Is(err, os.ErrExist) and leaves it untouched, unless
// overwrite is set.
func (s *Store) CopyPDF(folder, name, dir string, overwrite bool) error {
	if err := s.applicationRecord(folder); err != nil {
		return err
	}
	return s.copyInto(filepath.Join(folder, "out", name), dir, overwrite)
}

// copyInto copies the file at src into dir under its own name, creating dir if needed,
// with CopyPDF's rule for a name that is already taken.
func (s *Store) copyInto(src, dir string, overwrite bool) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	name := filepath.Base(src)
	if err := s.fs().MkdirAll(dir, sharedDirMode); err != nil {
		return err
	}
	dest := filepath.Join(dir, name)
	if !overwrite {
		if _, err := os.Stat(dest); err == nil {
			return &os.PathError{Op: "open", Path: dest, Err: os.ErrExist}
		} else if !os.IsNotExist(err) {
			return err
		}
		return s.atomicWrite(dest, data, sharedFileMode)
	}
	return s.writePreservingMode(dest, data, sharedFileMode)
}
