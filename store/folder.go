package store

import (
	"os"
	"path/filepath"
	"strings"
)

const resumeFile = "resume.md"

const gapsFile = "gaps.md"
const jdFile = "jd.md"

const coverLetterFile = "cover-letter.md"

func readFolder(a *Application) error {
	flags := []struct {
		name string
		dest *bool
		dir  bool
	}{
		{jdFile, &a.HasJD, false}, {"notes.md", &a.HasNotes, false}, {"fit.md", &a.HasFit, false}, {gapsFile, &a.HasGaps, false}, {resumeFile, &a.HasResume, false}, {coverLetterFile, &a.HasCoverLetter, false}, {"sent", &a.HasSent, true},
	}
	for _, f := range flags {
		info, err := os.Stat(filepath.Join(a.Folder, f.name))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		*f.dest = info.IsDir() == f.dir
	}

	for _, doc := range []struct {
		name    string
		present bool
		text    *bool
	}{
		{jdFile, a.HasJD, &a.JDHasText},
		{resumeFile, a.HasResume, &a.ResumeHasText},
		{coverLetterFile, a.HasCoverLetter, &a.CoverLetterHasText},
		{gapsFile, a.HasGaps, &a.GapsHasText},
	} {
		if !doc.present {
			continue
		}
		b, err := os.ReadFile(filepath.Join(a.Folder, doc.name))
		if err != nil {
			return err
		}
		*doc.text = strings.TrimSpace(string(b)) != ""
	}
	entries, err := os.ReadDir(filepath.Join(a.Folder, "out"))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	resume, letter := false, false
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".pdf" {
			continue
		}
		a.PDFs = append(a.PDFs, e.Name())
		resume = resume || strings.Contains(e.Name(), "Resume")
		letter = letter || strings.Contains(e.Name(), "Cover Letter")
	}
	if resume && (letter || a.ResumeOnly) {
		a.Documents = DocsReady
		return nil
	}
	if a.HasResume {
		b, err := os.ReadFile(filepath.Join(a.Folder, resumeFile))
		if err != nil {
			return err
		}
		if strings.TrimSpace(string(b)) != "" {
			a.Documents = DocsWritten
		}
	}
	return nil
}
