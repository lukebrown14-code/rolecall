package store

import (
	"os"
	"path/filepath"
)

// ReadDocument reads one file of an Application folder. A missing file returns an
// error satisfying os.IsNotExist.
func (s *Store) ReadDocument(folder, name string) ([]byte, error) {
	return os.ReadFile(filepath.Join(folder, name))
}
