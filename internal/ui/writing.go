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
// alive: the spinner's tick and, per document, the tick that puts its mark out. With no
// Harness set no agent can be writing, so a write (the Owner's own Editor, a Lead's new
// empty jd.md) marks nothing: the mark never claims work that is not happening.
func (m *model) markWriting(paths []string) tea.Cmd {
	if len(paths) == 0 || m.harness == "" {
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

// chipText is one step of a Document: its Written step by the Document's name, its Built
// step as its PDF: "✓ Resume", "○ PDF", or, while its file is being written,
// "⠹ Resume writing…" ("⠹ PDF building…").
func (m model) chipText(p pal, a store.Application, c docChip) string {
	name := c.doc
	busy := " writing…"
	if c.step == stepBuilt {
		name, busy = docPDF, " building…"
	}
	switch {
	case m.isWriting(a, c.file):
		return m.spinner.View() + p.Text(p.Fg).Render(" "+name) + p.Text(p.Dim).Render(busy)
	case c.ready:
		return p.Text(p.Ok).Render(glyphReady) + p.Text(p.Fg).Render(" "+name)
	}
	return p.Text(p.Dim).Render(glyphMissing + " " + name)
}

// docMarks is each Document with its two steps, "✓ Resume ○ PDF", in set order.
func (m model) docMarks(p pal, a store.Application) []string {
	chips := docChips(a)
	out := make([]string, 0, len(chips)/2) //nolint:mnd // two steps per Document
	for i := 0; i+1 < len(chips); i += 2 {
		out = append(out, m.chipText(p, a, chips[i])+" "+m.chipText(p, a, chips[i+1]))
	}
	return out
}
