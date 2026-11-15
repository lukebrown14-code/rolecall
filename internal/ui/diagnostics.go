package ui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/internal/diag"
)

// slowUpdate is how long one Update may take before the diagnostics log names it: past
// this a key press feels laggy.
const slowUpdate = 100 * time.Millisecond

// Update handles one message, recording to the diagnostics log (ADR-0006) when it is on:
// a panic, the first terminal size, a screen or pop-up change, a new feedback-line error
// and a slow update. It records names and error text only, never what a screen shows.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !diag.On() {
		return m.update(msg)
	}
	defer diag.Guard("update")
	start := time.Now()
	next, cmd := m.update(msg)
	if n, ok := next.(model); ok {
		m.observe(n, msg, time.Since(start))
	}
	return next, guardCmd(cmd)
}

// observe records what changed between m and next on msg.
func (m model) observe(next model, msg tea.Msg, took time.Duration) {
	if size, ok := msg.(tea.WindowSizeMsg); ok && m.width == 0 {
		diag.Record("terminal", "width", size.Width, "height", size.Height)
	}
	if next.screen != m.screen && next.screen >= 0 && next.screen < len(screenDefs) {
		diag.Record("screen", "name", screenDefs[next.screen].name)
	}
	if was, now := modeName(m.overlay), modeName(next.overlay); now != was && now != "" {
		diag.Record("overlay", "name", now)
	}
	if next.err != nil && (m.err == nil || next.err.Error() != m.err.Error()) {
		diag.Error("feedback_error", next.err, "shown", next.errorText(next.err), "type", fmt.Sprintf("%T", next.err))
	}
	if took >= slowUpdate {
		diag.Record("slow_update", "msg", fmt.Sprintf("%T", msg), "ms", took.Milliseconds())
	}
}

// modeName is a pop-up's Go type, which names it well enough to read a log by.
func modeName(o mode) string {
	if o == nil {
		return ""
	}
	return fmt.Sprintf("%T", o)
}

// guardCmd records a panic inside cmd, then lets it go on to the program's own recovery.
// The commands inside a tea.Batch or tea.Sequence run apart from it and are not
// covered; Bubble Tea still prints their panics.
func guardCmd(cmd tea.Cmd) tea.Cmd {
	if cmd == nil {
		return nil
	}
	return func() tea.Msg {
		defer diag.Guard("command")
		return cmd()
	}
}
