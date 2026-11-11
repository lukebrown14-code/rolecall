package store

import (
	"errors"
	"io"
	"os"
	"path/filepath"
)

// sentFiles are the Application files frozen into sent/ when it is first created.
var sentFiles = []string{"resume.md", "cover-letter.md", "jd.md", "fit.md", "gaps.md"}

// RecordSent freezes a copy of what was sent into the Application's sent/ folder:
// whichever of resume.md, cover-letter.md, jd.md, fit.md, gaps.md and out/*.pdf exist.
// An existing sent/ is never touched, so repeat calls are no-ops.
func (s *Store) RecordSent(folder string) error {
	sent := filepath.Join(folder, "sent")
	if err := os.Mkdir(sent, 0o755); err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil
		}
		return err
	}
	for _, name := range sentFiles {
		if err := copyIfPresent(filepath.Join(folder, name), filepath.Join(sent, name)); err != nil {
			return err
		}
	}
	pdfs, err := filepath.Glob(filepath.Join(folder, "out", "*.pdf"))
	if err != nil {
		return err
	}
	if len(pdfs) > 0 {
		if err := os.Mkdir(filepath.Join(sent, "out"), 0o755); err != nil {
			return err
		}
	}
	for _, pdf := range pdfs {
		if err := copyIfPresent(pdf, filepath.Join(sent, "out", filepath.Base(pdf))); err != nil {
			return err
		}
	}
	return nil
}

func copyIfPresent(from, to string) error {
	in, err := os.Open(from)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return err
	}
	out, err := os.OpenFile(to, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err = io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
