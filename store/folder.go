package store

import (
	"os"
	"path/filepath"
	"strings"
)

func readFolder(a *Application) error {
	flags := []struct {
		name string
		dest *bool
		dir  bool
	}{
		{"jd.md", &a.HasJD, false}, {"notes.md", &a.HasNotes, false}, {"fit.md", &a.HasFit, false}, {"gaps.md", &a.HasGaps, false}, {"resume.md", &a.HasResume, false}, {"cover-letter.md", &a.HasCoverLetter, false}, {"sent", &a.HasSent, true},
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
		a.Documents = "Ready"
		return nil
	}
	if a.HasResume {
		b, err := os.ReadFile(filepath.Join(a.Folder, "resume.md"))
		if err != nil {
			return err
		}
		if strings.TrimSpace(string(b)) != "" {
			a.Documents = "Written"
		}
	}
	return nil
}
