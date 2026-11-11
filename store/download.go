package store

import (
	"io"
	"os"
	"path/filepath"
)

// CopyPDF copies out/<name> of the Application folder into dir under the same name,
// creating dir if needed. If a file of that name already exists in dir it returns an
// error satisfying errors.Is(err, os.ErrExist) and leaves it untouched, unless
// overwrite is set.
func (s *Store) CopyPDF(folder, name, dir string, overwrite bool) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	in, err := os.Open(filepath.Join(folder, "out", name))
	if err != nil {
		return err
	}
	defer in.Close()
	flags := os.O_CREATE | os.O_WRONLY | os.O_EXCL
	if overwrite {
		flags = os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	}
	out, err := os.OpenFile(filepath.Join(dir, name), flags, 0o644)
	if err != nil {
		return err
	}
	if _, err = io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
