package store

import (
	"os"
	"path/filepath"
)

// ReadDocument reads one file of an Application folder. A missing file returns an
// error satisfying os.IsNotExist.
func (s *Store) ReadDocument(folder, name string) ([]byte, error) {
	if err := s.application(folder); err != nil {
		return nil, err
	}
	return os.ReadFile(filepath.Join(folder, name))
}

// WriteDocument replaces one file of an Application folder.
func (s *Store) WriteDocument(folder, name string, data []byte) error {
	if err := s.applicationRecord(folder); err != nil {
		return err
	}
	path := filepath.Join(folder, name)
	return s.writePreservingMode(path, data, privateFileMode)
}
