package ui

import (
	"errors"
	"net/url"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/internal/process"
	"github.com/lukebrown14-code/rolecall/internal/store"
)

// An Application's link is the url in its meta.yml: the job ad. w opens it in the
// browser and c copies it. Only http and https links are acted on: meta.yml is written
// by the Harness too, and the desktop launcher would open anything else it names, such
// as a local file or an app.

// ClipboardWriter puts text on the system clipboard.
type ClipboardWriter func(string) error

// WithClipboard injects the clipboard writer; tests use it to see what was copied.
func WithClipboard(w ClipboardWriter) Option { return func(m *model) { m.clipboard = w } }

// linkCopied reports the system clipboard write that follows c.
type linkCopied struct{ err error }

// webLink is the Application's ad URL when it is an http or https link.
func webLink(a store.Application) (string, bool) {
	raw := strings.TrimSpace(a.URL)
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", false
	}
	return raw, true
}

// hasLink reports whether the current row has a link w and c can use.
func hasLink(m model) bool {
	a, ok := m.current()
	if !ok || a.Malformed != nil {
		return false
	}
	_, ok = webLink(a)
	return ok
}

// openLink opens the current Application's ad in the browser.
func (m *model) openLink() tea.Cmd {
	a, ok := m.current()
	if !ok {
		return nil
	}
	link, ok := webLink(a)
	if !ok {
		return nil
	}
	return m.openPath(a.Folder, link, false)
}

// copyLink copies the current Application's ad URL. The terminal is asked too (OSC 52),
// so the copy also works over SSH and where no clipboard program is installed.
func (m *model) copyLink() tea.Cmd {
	a, ok := m.current()
	if !ok {
		return nil
	}
	link, ok := webLink(a)
	if !ok {
		return nil
	}
	write := m.clipboard
	if write == nil {
		write = process.CopyToClipboard
	}
	return tea.Batch(tea.SetClipboard(link), func() tea.Msg { return linkCopied{err: write(link)} })
}

// linkCopiedToast is what the feedback line says once the copy has run.
func (m model) linkCopied(msg linkCopied) model {
	switch {
	case msg.err == nil:
		m.toast = "Copied link"
	case errors.Is(msg.err, process.ErrNoClipboard):
		// Only the terminal was asked; most terminals honour it, some (Terminal.app) do not.
		m.toast = "Copied link through the terminal"
	default:
		m.err = msg.err
	}
	return m
}
