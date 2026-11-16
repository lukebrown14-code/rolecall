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

// FileSystem supplies the atomic publication boundary and the folder operations
// writes need. Nil uses the OS.
type FileSystem interface {
	CreateTemp(string, string) (AtomicFile, error)
	Rename(string, string) error
	Remove(string) error
	MkdirAll(string, os.FileMode) error
	Mkdir(string, os.FileMode) error
	RemoveAll(string) error
	// SyncDir flushes a directory's entries, so a rename into it survives power loss.
	SyncDir(string) error
}
type osFiles struct{}

func (osFiles) CreateTemp(dir, pattern string) (AtomicFile, error) {
	return os.CreateTemp(dir, pattern)
}
func (osFiles) Rename(a, b string) error                     { return os.Rename(a, b) }
func (osFiles) Remove(path string) error                     { return os.Remove(path) }
func (osFiles) MkdirAll(path string, mode os.FileMode) error { return os.MkdirAll(path, mode) }
func (osFiles) Mkdir(path string, mode os.FileMode) error    { return os.Mkdir(path, mode) }
func (osFiles) RemoveAll(path string) error                  { return os.RemoveAll(path) }
func (osFiles) SyncDir(path string) error {
	d, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = d.Close() }()
	return d.Sync()
}

// fsOrOS returns the injected filesystem, or the OS when none is injected.
func fsOrOS(files FileSystem) FileSystem {
	if files == nil {
		return osFiles{}
	}
	return files
}

func (s *Store) fs() FileSystem { return fsOrOS(s.files) }

func (s *Store) atomicWrite(path string, data []byte, mode os.FileMode) error {
	return atomicWrite(s.files, path, data, mode)
}

// atomicWrite publishes data through the injected filesystem. Nil uses the OS.
func atomicWrite(files FileSystem, path string, data []byte, mode os.FileMode) error {
	return atomicWriteIf(files, path, data, mode, nil)
}

// atomicWriteIf is atomicWrite with a precondition checked after the temporary file is
// durable and just before the rename: a non-nil error from check abandons the write.
// The parent directory is synced after the rename, best effort.
func atomicWriteIf(files FileSystem, path string, data []byte, mode os.FileMode, check func() error) error {
	fs := fsOrOS(files)
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
	if check != nil {
		if err = check(); err != nil {
			return err
		}
	}
	if err = fs.Rename(f.Name(), path); err != nil {
		return err
	}
	// Best effort: the file is already whole; this only makes the rename itself durable.
	_ = fs.SyncDir(filepath.Dir(path))
	return nil
}

// preserveMode is the mode to write path with: its current permission when it exists, or
// def when it does not. Any stat error other than not-exist is returned.
func preserveMode(path string, def os.FileMode) (os.FileMode, error) {
	info, err := os.Stat(path)
	if err == nil {
		return info.Mode().Perm(), nil
	}
	if os.IsNotExist(err) {
		return def, nil
	}
	return 0, err
}

// writePreservingMode writes data to path, keeping path's permission when it already
// exists and using def otherwise. The stat/keep-or-default shape lives in one place.
func (s *Store) writePreservingMode(path string, data []byte, def os.FileMode) error {
	mode, err := preserveMode(path, def)
	if err != nil {
		return err
	}
	return s.atomicWrite(path, data, mode)
}
