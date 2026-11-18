package store

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// sentFiles are the Application files frozen into sent/ when it is first created.
var sentFiles = []string{resumeFile, coverLetterFile, jdFile, "fit.md", gapsFile}

// sentStagingPrefix names the folder sent/ is built in before it is renamed into place.
// One left behind by a crash is never read and may be deleted.
const sentStagingPrefix = "sent.tmp-"

// MoveResult says what Move did: Wrote that a Status entry was appended, MadeSent that
// this move created sent/ (an undo sets aside only a sent/ its move made).
type MoveResult struct {
	Wrote, MadeSent bool
}

// Move is a move to a Status as the app and `rolecall move` make it. Moving to Applied
// first freezes sent/ (when there is none), then appends the entry, so a failed copy
// moves nothing and a retry starts clean; if the append then fails, the sent/ this move
// made is removed again. An Application already at status is left as it is, sent/ and
// all: an Applied one without sent/ (imported, say) is not frozen from today's files.
// The append keeps AppendStatus's guard against a meta.yml changed by another writer;
// if that writer moved it to status first, the sent/ made here is removed too.
func (s *Store) Move(folder, status string, at time.Time) (MoveResult, error) {
	var r MoveResult
	if !validStatus(status) {
		return r, fmt.Errorf("invalid Status %q", status)
	}
	if err := s.applicationRecord(folder); err != nil {
		return r, err
	}
	a, err := s.readApplicationFile(folder)
	if err != nil {
		return r, err
	}
	if a.Status == status {
		return r, nil
	}
	if status == StatusApplied {
		made, err := s.recordSent(folder)
		if err != nil {
			return r, err
		}
		r.MadeSent = made
	}
	wrote, err := s.AppendStatus(folder, status, at)
	if err != nil || !wrote {
		if r.MadeSent {
			_ = s.fs().RemoveAll(filepath.Join(folder, "sent"))
		}
		return MoveResult{}, err
	}
	r.Wrote = true
	return r, nil
}

// RecordSent freezes a copy of what was sent into the Application's sent/ folder:
// whichever of resume.md, cover-letter.md, jd.md, fit.md, gaps.md and out/*.pdf exist as
// regular files (a symlink is not the harness's file and is left out, as readRegular
// refuses it). An existing sent/ is never touched, so repeat calls are no-ops.
func (s *Store) RecordSent(folder string) error {
	_, err := s.recordSent(folder)
	return err
}

// recordSent is RecordSent, also reporting whether it created sent/. The copy is built
// in a sent.tmp-* folder beside it and renamed to sent/ only when whole, so a failure
// leaves no sent/ (and no staging folder) and the next call starts again.
func (s *Store) recordSent(folder string) (bool, error) {
	if err := s.applicationRecord(folder); err != nil {
		return false, err
	}
	sent := filepath.Join(folder, "sent")
	if exists, err := lexists(sent); err != nil || exists {
		return false, err
	}
	staging, err := s.mkdirStaging(folder)
	if err != nil {
		return false, err
	}
	if err := s.fillSent(folder, staging); err != nil {
		_ = s.fs().RemoveAll(staging)
		return false, err
	}
	// A rename onto an existing empty folder would replace it, so look once more.
	if exists, err := lexists(sent); err != nil || exists {
		_ = s.fs().RemoveAll(staging)
		return false, err
	}
	if err := s.fs().Rename(staging, sent); err != nil {
		_ = s.fs().RemoveAll(staging)
		if exists, _ := lexists(sent); exists {
			return false, nil // another writer froze it first
		}
		return false, err
	}
	// Best effort: the copy is whole; this only makes the rename durable.
	_ = s.fs().SyncDir(folder)
	return true, nil
}

// fillSent copies the sent files from folder into dir.
func (s *Store) fillSent(folder, dir string) error {
	for _, name := range sentFiles {
		if err := s.copyIfPresent(filepath.Join(folder, name), filepath.Join(dir, name)); err != nil {
			return err
		}
	}
	entries, err := os.ReadDir(filepath.Join(folder, "out"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	var pdfs []string
	for _, e := range entries {
		// The entry's type is the link's own, so a symlinked PDF is not followed.
		if e.Type().IsRegular() && filepath.Ext(e.Name()) == ".pdf" {
			pdfs = append(pdfs, e.Name())
		}
	}
	if len(pdfs) == 0 {
		return nil
	}
	if err := s.fs().Mkdir(filepath.Join(dir, "out"), privateDirMode); err != nil {
		return err
	}
	for _, name := range pdfs {
		if err := s.copyIfPresent(filepath.Join(folder, "out", name), filepath.Join(dir, "out", name)); err != nil {
			return err
		}
	}
	return nil
}

// mkdirStaging creates an empty, private sent.tmp-<random> folder in folder.
func (s *Store) mkdirStaging(folder string) (string, error) {
	const attempts, randomBytes = 10, 4
	for range attempts {
		b := make([]byte, randomBytes)
		_, _ = rand.Read(b)
		dir := filepath.Join(folder, sentStagingPrefix+hex.EncodeToString(b))
		err := s.fs().Mkdir(dir, privateDirMode)
		if err == nil {
			return dir, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return "", err
		}
	}
	return "", fmt.Errorf("no free %s* name in %s", sentStagingPrefix, folder)
}

// lexists reports whether path exists, without following a symlink.
func lexists(path string) (bool, error) {
	_, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
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

// copyIfPresent copies a regular file byte for byte with its mode; a missing file or
// anything else (a symlink above all) is skipped.
func (s *Store) copyIfPresent(from, to string) error {
	info, err := os.Lstat(from)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return nil
	}
	data, err := readRegular(from)
	if err != nil {
		return err
	}
	return s.writePreservingMode(to, data, info.Mode().Perm())
}
