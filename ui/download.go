package ui

import (
	"errors"
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/store"
)

// WithDownloads sets the folder that `d` copies upload-ready PDFs into.
func WithDownloads(dir string) Option { return func(m *model) { m.downloads = dir } }

// downloadState is a download paused on a name clash, awaiting the Owner's y/n.
type downloadState struct {
	app    store.Application
	next   int // index into app.PDFs of the clashing file
	copied int
}

func (d downloadState) clash() string { return d.app.PDFs[d.next] }

// download copies every out/*.pdf of the Application into the downloads folder, asking
// before overwriting a file of the same name, then reveals the folder and reports the count.
func (m *model) download(a store.Application) tea.Cmd {
	next, cmd := m.continueDownload(downloadState{app: a})
	m.overlay = next
	return cmd
}

// continueDownload copies the rest of the PDFs, returning the mode that pauses on the next
// clash, or nil when the copy is done.
func (m *model) continueDownload(d downloadState) (mode, tea.Cmd) {
	for ; d.next < len(d.app.PDFs); d.next++ {
		err := m.store.CopyPDF(d.app.Folder, d.app.PDFs[d.next], m.downloads, false)
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
	return nil, m.openPath(d.app.Folder, m.downloads, true)
}

// answerDownload applies y/a/n/esc to the pending clash; other keys leave the prompt open.
func (m *model) answerDownload(d downloadState, key string) (mode, tea.Cmd) {
	switch key {
	case "y":
		if err := m.store.CopyPDF(d.app.Folder, d.clash(), m.downloads, true); err != nil {
			m.err = err
			return nil, nil
		}
		d.copied++
		d.next++
	case "a":
		// Overwrite this file and every later clash without asking again.
		for ; d.next < len(d.app.PDFs); d.next++ {
			if err := m.store.CopyPDF(d.app.Folder, d.app.PDFs[d.next], m.downloads, true); err != nil {
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
