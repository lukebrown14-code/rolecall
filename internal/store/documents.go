package store

import (
	"os"
	"path/filepath"
)

// ReadDocument reads one file of an Application folder for the screen: its text passes
// through DocumentText, so no control or escape sequence in it reaches the terminal. It
// is never the source of a write. A missing file returns an error satisfying
// os.IsNotExist.
func (s *Store) ReadDocument(folder, name string) ([]byte, error) {
	if err := s.application(folder); err != nil {
		return nil, err
	}
	b, err := os.ReadFile(filepath.Join(folder, name))
	if err != nil {
		return nil, err
	}
	return []byte(DocumentText(string(b))), nil
}

// WriteDocument replaces one file of an Application folder.
func (s *Store) WriteDocument(folder, name string, data []byte) error {
	if err := s.applicationRecord(folder); err != nil {
		return err
	}
	path := filepath.Join(folder, name)
	return s.writePreservingMode(path, data, privateFileMode)
}
