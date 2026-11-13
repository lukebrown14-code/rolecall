package store

import (
	"os"
	"path/filepath"
)

// AtomicFile is a same-directory temporary file used to publish metadata.
type AtomicFile interface {
	Name() string
	Chmod(os.FileMode) error
	Write([]byte) (int, error)
	Sync() error
	Close() error
}

// FileSystem supplies the atomic publication boundary. Nil uses the OS.
type FileSystem interface {
	CreateTemp(string, string) (AtomicFile, error)
	Rename(string, string) error
	Remove(string) error
}
type osFiles struct{}

func (osFiles) CreateTemp(dir, pattern string) (AtomicFile, error) {
	return os.CreateTemp(dir, pattern)
}
func (osFiles) Rename(a, b string) error { return os.Rename(a, b) }
func (osFiles) Remove(path string) error { return os.Remove(path) }
func (s *Store) atomicWrite(path string, data []byte, mode os.FileMode) error {
	return atomicWrite(s.files, path, data, mode)
}

// atomicWrite publishes data through the injected filesystem. Nil uses the OS.
func atomicWrite(files FileSystem, path string, data []byte, mode os.FileMode) error {
	fs := files
	if fs == nil {
		fs = osFiles{}
	}
	f, err := fs.CreateTemp(filepath.Dir(path), ".meta-*")
	if err != nil {
		return err
	}
	defer func() { _ = fs.Remove(f.Name()) }()
	closed := false
	defer func() {
		if !closed {
			_ = f.Close()
		}
	}()
	if err = f.Chmod(mode); err != nil {
		return err
	}
	if _, err = f.Write(data); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	err = f.Close()
	closed = true
	if err != nil {
		return err
	}
	return fs.Rename(f.Name(), path)
}
