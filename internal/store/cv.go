package store

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// cvDir is the Workspace folder that holds CVs: each cv/<name>.md is one, built by
// `build.py --cv <name>` into cv/out/<name>/. The resume or export create-profile
// imported also sits here; only Markdown files are CVs.
const cvDir = "cv"

// generalCV is the CV listed first: the Owner's everyday untailored resume.
const generalCV = "general"

// CV is a general resume written from the Profile for no Application. Path is
// Workspace-relative; PDF is the built PDF's file name in OutDir, or "" before a build.
// Stale marks a PDF older than the Markdown, so it no longer shows the latest edit.
type CV struct {
	Name, Path, Body string
	OutDir, PDF      string
	Stale            bool
}

// FileName is the CV's Markdown file name, as the Profile screen lists it.
func (c CV) FileName() string { return c.Name + ".md" }

// CVs reads every cv/*.md, general first and the rest by name. A Workspace with no cv/
// folder has none.
func (s *Store) CVs() ([]CV, error) {
	entries, err := os.ReadDir(filepath.Join(s.root, cvDir))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var cvs []CV
	for _, entry := range entries {
		name, ok := strings.CutSuffix(entry.Name(), ".md")
		if !ok || name == "" || !entry.Type().IsRegular() {
			continue
		}
		cv, err := s.readCV(name)
		if err != nil {
			return nil, err
		}
		cvs = append(cvs, cv)
	}
	slices.SortStableFunc(cvs, func(a, b CV) int {
		switch {
		case a.Name == generalCV:
			return -1
		case b.Name == generalCV:
			return 1
		}
		return strings.Compare(a.Name, b.Name)
	})
	return cvs, nil
}

// readCV reads cv/<name>.md and finds its built PDF: the newest PDF in cv/out/<name>/.
func (s *Store) readCV(name string) (CV, error) {
	rel := filepath.Join(cvDir, name+".md")
	path := filepath.Join(s.root, rel)
	b, err := os.ReadFile(path)
	if err != nil {
		return CV{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return CV{}, err
	}
	cv := CV{Name: name, Path: rel, Body: DocumentText(string(b)), OutDir: filepath.Join(s.root, cvDir, "out", name)}
	pdfs, err := os.ReadDir(cv.OutDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return CV{}, err
	}
	var built time.Time
	for _, pdf := range pdfs {
		if !pdf.Type().IsRegular() || !strings.EqualFold(filepath.Ext(pdf.Name()), ".pdf") {
			continue
		}
		pdfInfo, err := pdf.Info()
		if err != nil {
			return CV{}, err
		}
		if cv.PDF == "" || pdfInfo.ModTime().After(built) {
			cv.PDF, built = pdf.Name(), pdfInfo.ModTime()
		}
	}
	cv.Stale = cv.PDF != "" && info.ModTime().After(built)
	return cv, nil
}

// CopyCVPDF copies a CV's built PDF into dir under the same name, as CopyPDF does for an
// Application: it refuses to replace a file of that name unless overwrite is set.
func (s *Store) CopyCVPDF(cv CV, dir string, overwrite bool) error {
	if cv.PDF == "" {
		return os.ErrNotExist
	}
	return s.copyInto(filepath.Join(cv.OutDir, cv.PDF), dir, overwrite)
}
