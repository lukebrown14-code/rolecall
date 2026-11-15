package ui

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/internal/store"
	"github.com/lukebrown14-code/rolecall/internal/ui/watch"
)

// Live agent progress. All the TUI can honestly see of a Harness writing documents in
// another terminal is the file watch.Watcher's events: a document's file was just written. So
// a chip reads "⠹ Resume writing…" while its file changed within writingFor, and the
// meter beside it counts what is ready. Nothing is guessed about a run it cannot see.

// writingFor is how long a document reads as being written after its file last changed.
const writingFor = 4 * time.Second

// writingDone puts out one document's writing mark, unless a later write renewed it.
type writingDone struct {
	key string
	gen int
}

// markWriting notes documents the watch.Watcher saw written and returns what keeps the marks
// alive: the spinner's tick and, per document, the tick that puts its mark out.
func (m *model) markWriting(paths []string) tea.Cmd {
	if len(paths) == 0 {
		return nil
	}
	next := make(map[string]int, len(m.writing)+len(paths))
	for k, v := range m.writing {
		next[k] = v
	}
	tick := m.tick
	if tick == nil {
		tick = tea.Tick
	}
	cmds := []tea.Cmd{m.spinner.Tick}
	for _, p := range paths {
		m.writingGen++
		next[p] = m.writingGen
		done := writingDone{p, m.writingGen}
		cmds = append(cmds, tick(writingFor, func(time.Time) tea.Msg { return done }))
	}
	m.writing = next
	return tea.Batch(cmds...)
}

// endWriting puts out a writing mark when no later write renewed it.
func (m *model) endWriting(done writingDone) {
	if m.writing[done.key] != done.gen {
		return
	}
	next := make(map[string]int, len(m.writing))
	for k, v := range m.writing {
		if k != done.key {
			next[k] = v
		}
	}
	m.writing = next
}

// isWriting reports whether one of an Application's documents was written just now.
func (m model) isWriting(a store.Application, file string) bool {
	_, ok := m.writing[watch.DocumentKey(a.Folder, file)]
	return ok
}

// chipText is one document chip: "✓ Resume", "○ Cover letter", or, while its file is
// being written, "⠹ Resume writing…".
func (m model) chipText(p pal, a store.Application, c docChip) string {
	switch {
	case m.isWriting(a, c.file):
		return m.spinner.View() + p.Text(p.Fg).Render(" "+c.name) + p.Text(p.Dim).Render(" writing…")
	case c.ready:
		return p.Text(p.Ok).Render(glyphReady) + p.Text(p.Fg).Render(" "+c.name)
	}
	return p.Text(p.Dim).Render(glyphMissing + " " + c.name)
}
