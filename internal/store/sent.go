package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// sentFiles are the Application files frozen into sent/ when it is first created.
var sentFiles = []string{resumeFile, coverLetterFile, jdFile, "fit.md", gapsFile}

// RecordSent freezes a copy of what was sent into the Application's sent/ folder:
// whichever of resume.md, cover-letter.md, jd.md, fit.md, gaps.md and out/*.pdf exist.
// An existing sent/ is never touched, so repeat calls are no-ops.
func (s *Store) RecordSent(folder string) error {
	if err := s.applicationRecord(folder); err != nil {
		return err
	}
	sent := filepath.Join(folder, "sent")
	if err := s.fs().Mkdir(sent, sharedDirMode); err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil
		}
		return err
	}
	for _, name := range sentFiles {
		if err := s.copyIfPresent(filepath.Join(folder, name), filepath.Join(sent, name)); err != nil {
			return err
		}
	}
	entries, err := os.ReadDir(filepath.Join(folder, "out"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	var pdfs []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".pdf" {
			pdfs = append(pdfs, e.Name())
		}
	}
	if len(pdfs) > 0 {
		if err := s.fs().Mkdir(filepath.Join(sent, "out"), sharedDirMode); err != nil {
			return err
		}
	}
	for _, name := range pdfs {
		if err := s.copyIfPresent(filepath.Join(folder, "out", name), filepath.Join(sent, "out", name)); err != nil {
			return err
		}
	}
	return nil
}

// SetAsideSent renames an Application's sent/ to sent.undone-<date>/ (with -2, -3… when
// that name is taken), for an undone move to Applied whose RecordSent made it: a later
// real Apply then freezes a fresh sent/, instead of the mistaken move's copy standing as
// what was sent. Nothing is deleted. It returns the new folder name.
func (s *Store) SetAsideSent(folder string, at time.Time) (string, error) {
	if err := s.applicationRecord(folder); err != nil {
		return "", err
	}
	base := "sent.undone-" + at.Format("2006-01-02")
	name := base
	for n := 2; ; n++ {
		if _, err := os.Lstat(filepath.Join(folder, name)); errors.Is(err, os.ErrNotExist) {
			break
		} else if err != nil {
			return "", err
		}
		name = fmt.Sprintf("%s-%d", base, n)
	}
	if err := s.fs().Rename(filepath.Join(folder, "sent"), filepath.Join(folder, name)); err != nil {
		return "", err
	}
	return name, nil
}

func (s *Store) copyIfPresent(from, to string) error {
	info, err := os.Stat(from)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return nil
	}
	data, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	return s.writePreservingMode(to, data, info.Mode().Perm())
}
