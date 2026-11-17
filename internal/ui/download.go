package ui

import (
	"errors"
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/internal/store"
)

// WithDownloads sets the folder that `d` copies upload-ready PDFs into.
func WithDownloads(dir string) Option { return func(m *model) { m.downloads = dir } }

// downloadState is a download paused on a name clash, awaiting the Owner's y/n. It copies
// one set of built PDFs: an Application's out/*.pdf, or a CV's PDF.
type downloadState struct {
	pdfs   []string
	folder string // where the copy runs from: the Application folder or the Workspace
	// copyPDF copies one of pdfs into the downloads folder, replacing a file of that name
	// only when overwrite is set.
	copyPDF func(m *model, name string, overwrite bool) error
	next    int // index into pdfs of the clashing file
	copied  int
}

func (d downloadState) clash() string { return d.pdfs[d.next] }

// download copies every out/*.pdf of the Application into the downloads folder, asking
// before overwriting a file of the same name, then reveals the folder and reports the count.
func (m *model) download(a store.Application) tea.Cmd {
	return m.startDownload(downloadState{pdfs: a.PDFs, folder: a.Folder,
		copyPDF: func(m *model, name string, overwrite bool) error {
			return m.store.CopyPDF(a.Folder, name, m.downloads, overwrite)
		}})
}

// downloadCV copies a CV's built PDF into the downloads folder, as download does for an
// Application's.
func (m *model) downloadCV(cv store.CV) tea.Cmd {
	return m.startDownload(downloadState{pdfs: []string{cv.PDF}, folder: m.store.Root(),
		copyPDF: func(m *model, _ string, overwrite bool) error {
			return m.store.CopyCVPDF(cv, m.downloads, overwrite)
		}})
}

func (m *model) startDownload(d downloadState) tea.Cmd {
	next, cmd := m.continueDownload(d)
	m.overlay = next
	return cmd
}

// continueDownload copies the rest of the PDFs, returning the mode that pauses on the next
// clash, or nil when the copy is done.
func (m *model) continueDownload(d downloadState) (mode, tea.Cmd) {
	for ; d.next < len(d.pdfs); d.next++ {
		err := d.copyPDF(m, d.pdfs[d.next], false)
		if errors.Is(err, os.ErrExist) {
			return &downloadMode{state: d}, nil
		}
		if err != nil {
			m.err = err
			return nil, nil
		}
		d.copied++
	}
	noun := "PDFs"
	if d.copied == 1 {
		noun = "PDF"
	}
	m.toast = fmt.Sprintf("Saved %d %s → %s", d.copied, noun, displayPath(m.downloads))
	return nil, m.openPath(d.folder, m.downloads, true)
}

// answerDownload applies y/a/n/esc to the pending clash; other keys leave the prompt open.
func (m *model) answerDownload(d downloadState, key string) (mode, tea.Cmd) {
	switch key {
	case "y":
		if err := d.copyPDF(m, d.clash(), true); err != nil {
			m.err = err
			return nil, nil
		}
		d.copied++
		d.next++
	case "a":
		// Overwrite this file and every later clash without asking again.
		for ; d.next < len(d.pdfs); d.next++ {
			if err := d.copyPDF(m, d.pdfs[d.next], true); err != nil {
				m.err = err
				return nil, nil
			}
			d.copied++
		}
	case "n":
		d.next++
	case "esc":
		return nil, nil
	default:
		return &downloadMode{state: d}, nil
	}
	return m.continueDownload(d)
}

// displayPath abbreviates the home folder to ~.
func displayPath(path string) string {
	if home, err := os.UserHomeDir(); err == nil && home != "" && strings.HasPrefix(path, home+string(os.PathSeparator)) {
		return "~" + path[len(home):]
	}
	return path
}
