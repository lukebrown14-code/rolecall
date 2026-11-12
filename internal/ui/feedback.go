package ui

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/internal/store"
)

// parseFixes says how to fix each reason the store refuses a meta.yml, in place of the
// parser's own words.
var parseFixes = map[string]string{
	"invalid status_log entry or unexpected nesting": "indented lines must be status_log entries: - {status: Lead, at: 2026-01-31}",
	"unknown Status":           "status: must be one of " + strings.Join(store.Statuses, ", "),
	"invalid status timestamp": "at: needs a date like 2026-01-31",
	"invalid or duplicate key": "each line needs one key: value, and no key twice",
	"invalid status_log":       "status_log: starts a list; put entries on the indented lines below, or write []",
	"invalid closes date":      "closes: needs a date like 2026-01-31",
	"invalid Fit": "fit: must be " + store.FitStrong + ", " + store.FitGood + ", " + store.FitMarginal + " or " +
		store.FitPoor + ", or empty",
	"invalid resume_only":                     "resume_only: must be true or false",
	"unsupported schema":                      "schema: must be 1",
	"expected UTF-8, LF and trailing newline": "save it as UTF-8 with LF line endings and a final newline",
}

// parseError is the meta.yml failure inside err, if there is one.
func parseError(err error) (*store.ParseError, bool) {
	var pe *store.ParseError
	ok := errors.As(err, &pe)
	return pe, ok
}

// parseFix is how to fix a parse failure, falling back to the parser's reason.
func parseFix(pe *store.ParseError) string {
	reason := pe.Reason.Error()
	if fix, ok := parseFixes[reason]; ok {
		return fix
	}
	if key, ok := strings.CutPrefix(reason, "missing "); ok {
		return "add a " + key + ": line"
	}
	return reason
}

// malformedName is the Application folder a parse failure belongs to, as the Owner
// knows it: "SWE/malformed".
func (m model) malformedName(pe *store.ParseError) string {
	return strings.TrimPrefix(m.rel(filepath.Dir(pe.Path)), "jobs"+string(filepath.Separator))
}

// malformedTitle is the list row's title for an Application whose meta.yml cannot be read.
func (m model) malformedTitle(a store.Application) string {
	if pe, ok := parseError(a.Malformed); ok {
		return "Can't read " + m.malformedName(pe)
	}
	return "Can't read " + filepath.Base(a.Folder)
}

// malformedReason is where and how to fix it: "meta.yml line 4: …".
func (m model) malformedReason(a store.Application) string {
	if pe, ok := parseError(a.Malformed); ok {
		return fmt.Sprintf("%s line %d: %s", filepath.Base(pe.Path), pe.Line, parseFix(pe))
	}
	return m.rel(a.Malformed.Error())
}

// errorText is an error as the feedback line words it: a meta.yml failure names the
// Application and says how to fix it; anything else reads with Workspace-relative paths.
func (m model) errorText(err error) string {
	if pe, ok := parseError(err); ok {
		return fmt.Sprintf("Can't read %s · %s line %d: %s", m.malformedName(pe), filepath.Base(pe.Path), pe.Line, parseFix(pe))
	}
	return m.rel(err.Error())
}

// editAt opens a file that failed to parse in the Editor at the failing line.
func (m model) editAt(pe *store.ParseError) tea.Cmd {
	return m.editFileAt(filepath.Dir(pe.Path), filepath.Base(pe.Path), pe.Line)
}

// flashRow lights a row that just moved: the accent fill the active screen tab uses,
// with the toast saying in words what happened.
func (p pal) FlashRow(lines []string, w int) []string {
	lit := lipgloss.NewStyle().Background(p.Accent).Foreground(p.OnFill).Width(w).MaxWidth(w)
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = lit.Render(xansi.Strip(l))
	}
	return out
}

// hintPairs splits a toast hint, "u undo · esc hide", into key and description pairs.
func hintPairs(hint string) [][2]string {
	var out [][2]string
	for _, part := range strings.Split(hint, " · ") {
		if k, d, _ := strings.Cut(part, " "); k != "" {
			out = append(out, [2]string{k, d})
		}
	}
	return out
}

// feedbackLine is the status row when something just happened: a toast (successes clear
// on the next key), an error (stays until esc), or a build in flight (a spinner until it
// finishes). The message is on the left and is cut first; the keys are on the right and
// are never cut. It never replaces the bottom rule.
func (m model) feedbackLine(p pal, w int) string {
	icon, c, msg, hint := "✓", p.Ok, m.toast, m.toastHint
	raw := false
	switch {
	case m.err != nil:
		msg = m.errorText(m.err)
		icon, c, hint = "⚠", p.Bad, "esc hide"
		if _, ok := parseError(m.err); ok {
			hint = "e fix in editor · esc hide"
		}
	case m.building != nil && m.toast == "":
		msg, icon, c, raw = "Building "+m.building.title+"…", m.spinner.View(), p.Accent, true
	case strings.HasPrefix(m.toast, offerPrefix):
		icon, c = p.StatusGlyph(store.StatusOffer), p.StatusColor(store.StatusOffer)
	}
	if msg == "" {
		return ""
	}
	bg := lipgloss.NewStyle().Background(p.Panel)
	var left string
	if raw {
		left = bg.Render(" ") + icon + bg.Foreground(p.Fg).Render(msg)
	} else {
		left = bg.Foreground(c).Bold(true).Render(" "+icon+" ") + bg.Foreground(p.Fg).Render(msg)
	}
	var keys []string
	for _, kv := range hintPairs(hint) {
		keys = append(keys, bg.Foreground(p.Fg).Bold(true).Render(kv[0])+bg.Foreground(p.Dim).Render(" "+kv[1]))
	}
	right := ""
	if len(keys) > 0 {
		right = strings.Join(keys, bg.Foreground(p.Dim).Render("  ")) + bg.Render(" ")
	}
	left = xansi.Truncate(left, max(0, w-xansi.StringWidth(right)-gutter), "…")
	n := max(1, w-xansi.StringWidth(left)-xansi.StringWidth(right))
	gap := bg.Render(strings.Repeat(" ", n))
	return left + gap + right
}
