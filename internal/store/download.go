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
	if _, err := s.applicationRecord(folder); err != nil {
		return err
	}
	data, err := os.ReadFile(filepath.Join(folder, "out", name))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, sharedDirMode); err != nil {
		return err
	}
	dest := filepath.Join(dir, name)
	mode := sharedFileMode
	if !overwrite {
		if _, err := os.Stat(dest); err == nil {
			return &os.PathError{Op: "open", Path: dest, Err: os.ErrExist}
		} else if !os.IsNotExist(err) {
			return err
		}
	} else if info, err := os.Stat(dest); err == nil {
		mode = info.Mode().Perm()
	} else if !os.IsNotExist(err) {
		return err
	}
	return s.atomicWrite(dest, data, mode)
}
