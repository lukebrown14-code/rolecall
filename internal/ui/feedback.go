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

// errorMark is a failure's own glyph, distinct from warning's ⚠ and Rejected's ✕, so a
// colour-blind Owner still tells a failure from a warning and from a closed Status.
const errorMark = "✘"

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
	// A folder name is whatever is on disk, so it is cleaned like a meta.yml string.
	return store.DisplayText(strings.TrimPrefix(m.rel(filepath.Dir(pe.Path)), "jobs"+string(filepath.Separator)))
}

// malformedTitle is the list row's title for an Application whose meta.yml cannot be read.
func (m model) malformedTitle(a store.Application) string {
	if pe, ok := parseError(a.Malformed); ok {
		return "Can't read " + m.malformedName(pe)
	}
	return "Can't read " + store.DisplayText(filepath.Base(a.Folder))
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

// feedbackParts is what just happened as the footer shows it, beside the breadcrumb or in
// its place: the message as a chip, then its keys. A toast (successes clear on the next
// key), an error (stays until esc), or a build in flight (a spinner until it finishes).
// short is the keys without "esc hide" for a success, which clears on the next key anyway;
// an error keeps it, as esc is the only way to clear it.
func (m model) feedbackParts(p pal) (msg, keys, short string) {
	msg, hint := m.feedbackMsg(p)
	if msg == "" {
		return "", "", ""
	}
	var ks, shortKs []string
	for _, kv := range hintPairs(hint) {
		k := p.Key().Render(kv[0]) + p.Text(p.Dim).Render(" "+kv[1])
		ks = append(ks, k)
		if kv[0] != "esc" || m.err != nil {
			shortKs = append(shortKs, k)
		}
	}
	return msg + lipgloss.NewStyle().Background(p.Panel).Render(" "), strings.Join(ks, keyGap), strings.Join(shortKs, keyGap)
}

// feedbackRight is the feedback fitted to room cells: the message whole with every key,
// then whole without esc hide; with cut, failing those, the message cut (never the keys,
// which recover from it). ok is false when it does not fit whole and cut is not set.
func feedbackRight(msg, keys, short string, room int, cut bool) (string, bool) {
	for _, k := range []string{keys, short} {
		need := xansi.StringWidth(msg)
		if k != "" {
			need += len(keyGap) + xansi.StringWidth(k)
		}
		if need <= room {
			return joinNonEmpty(keyGap, msg, k), true
		}
	}
	if !cut {
		return "", false
	}
	avail := room
	if short != "" {
		avail -= len(keyGap) + xansi.StringWidth(short)
	}
	return joinNonEmpty(keyGap, xansi.Truncate(msg, max(1, avail), "…"), short), true
}

// feedbackMsg is the feedback's message on the panel colour, with its glyph, and the keys
// that go with it; empty when nothing just happened.
func (m model) feedbackMsg(p pal) (string, string) {
	icon, c, msg, hint := "✓", p.Ok, m.toast, m.toastHint
	raw := false
	switch {
	case m.err != nil:
		msg = m.errorText(m.err)
		icon, c, hint = errorMark, p.Bad, "esc hide"
		if _, ok := parseError(m.err); ok {
			hint = "e fix in editor · esc hide"
		}
	case m.building != nil && m.toast == "":
		msg, icon, c, raw = "Building "+m.building.title+"…", m.spinner.View(), p.Info, true
	case m.toastKind == toastOffer:
		icon, c = p.StatusGlyph(store.StatusOffer), p.StatusColor(store.StatusOffer)
	case m.toastKind == toastNote:
		icon, c = " ", p.Sub // no success mark: nothing changed
	}
	if msg == "" {
		return "", ""
	}
	if icon == "✓" && p.Ascii {
		icon = "+" // the fold's stand-in for ✓, set here because the spinner path is raw
	}
	bg := lipgloss.NewStyle().Background(p.Panel)
	var left string
	if raw {
		left = bg.Render(" ") + icon + bg.Foreground(p.Fg).Render(msg)
	} else {
		left = bg.Foreground(c).Bold(true).Render(" "+icon+" ") + bg.Foreground(p.Fg).Render(msg)
	}
	return left, hint
}
