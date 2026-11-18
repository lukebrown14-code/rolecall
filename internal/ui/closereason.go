package ui

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/internal/store"
)

// fieldCloseReason is the Edit form's close reason field on a Rejected or Withdrawn
// Application. It sits under the form's Closed heading, as the card's CLOSED row.
const fieldCloseReason = "Reason"

// closeReasonNone is the Edit form's close reason choice that records none.
const closeReasonNone = "none"

// hintSkip is what esc does in the close reason pop-up: the move stands, no reason is written.
const hintSkip = "skip"

// closeReasonLabels are the words the screen shows for the suggested close_reason values;
// meta.yml keeps the values themselves (SCHEMA).
var closeReasonLabels = map[string]string{
	"no-response":     "No response",
	"after-screen":    "After screen",
	"after-interview": "After interview",
	"salary":          "Salary",
	"role-closed":     "Role closed",
	"other":           "Other",
}

// closeReasonLabel is how a close_reason reads on screen: a suggested value's words, or
// the Owner's own words with hyphens and underscores read as spaces and a capital first.
func closeReasonLabel(reason string) string {
	reason = strings.TrimSpace(reason)
	if l, ok := closeReasonLabels[reason]; ok {
		return l
	}
	reason = strings.NewReplacer("-", " ", "_", " ").Replace(reason)
	r, size := utf8.DecodeRuneInString(reason)
	if size == 0 {
		return ""
	}
	return string(unicode.ToUpper(r)) + reason[size:]
}

// closeReasonChoices are the Edit form's close reason choices: none, then the suggested
// values, with the saved one kept when it is the Owner's own words, so saving never
// rewrites it.
func closeReasonChoices(saved string) []choice {
	out := []choice{{Value: "", Label: closeReasonNone}}
	for _, r := range store.CloseReasons {
		out = append(out, choice{Value: r, Label: closeReasonLabel(r)})
	}
	if _, ok := closeReasonLabels[saved]; !ok && saved != "" {
		out = append(out, choice{Value: saved, Label: closeReasonLabel(saved)})
	}
	return out
}

// closeReasonMode asks why Applications that just moved to Rejected or Withdrawn ended.
// The move has already happened, and u still undoes it: esc skips the question and
// writes nothing, and a reason can be set or changed later from Edit. The reasons are a
// list with number keys, the cursor on the first.
//
// For a bulk move (bulk.go) apps is every Application that moved, and the one reason is
// written to each of them.
type closeReasonMode struct {
	apps   []store.Application
	status string
	sel    int
	// The move's toast, put back when the pop-up closes so its u undo stays on offer.
	toast, toastHint, movedTo, follow string
	toastKind                         toastKind
}

func (m model) closeReasonPopup(apps []store.Application, status string) mode {
	return &closeReasonMode{apps: apps, status: status,
		toast: m.toast, toastHint: m.toastHint, movedTo: m.movedTo, follow: m.follow, toastKind: m.toastKind}
}

func (cr *closeReasonMode) Keys() [][2]string {
	return [][2]string{{"1-" + strconv.Itoa(len(store.CloseReasons)), "pick"}, {"↑↓ enter", "save"}, {"esc", hintSkip}}
}

func (cr *closeReasonMode) View(m *model, p pal) string { return m.closeReasonBox(p, cr) }

func (cr *closeReasonMode) Update(m *model, msg tea.Msg) (mode, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return cr, nil
	}
	switch key := k.String(); key {
	case "esc":
		cr.restoreToast(m)
		return nil, nil
	case "down", "j":
		cr.sel = min(cr.sel+1, len(store.CloseReasons)-1)
	case "up", "k":
		cr.sel = max(cr.sel-1, 0)
	case "enter":
		return cr.pick(m, store.CloseReasons[cr.sel])
	default:
		if n, err := strconv.Atoi(key); err == nil && n >= 1 && n <= len(store.CloseReasons) {
			return cr.pick(m, store.CloseReasons[n-1])
		}
	}
	return cr, nil
}

// pick writes reason to every Application that moved and closes the pop-up.
func (cr *closeReasonMode) pick(m *model, reason string) (mode, tea.Cmd) {
	var written []string
	for _, a := range cr.apps {
		if err := m.store.SetCloseReason(a.Folder, reason); err != nil {
			m.err = err
			break
		}
		written = append(written, a.Folder)
	}
	m.reload(m.cursorFolder(), written...) // none written: a full rescan, as before
	if m.err == nil {
		cr.restoreToast(m)
		if m.toast != "" {
			m.toast += " · " + closeReasonLabel(reason)
		}
	}
	return nil, nil
}

func (cr *closeReasonMode) restoreToast(m *model) {
	m.toast, m.toastHint, m.movedTo, m.toastKind, m.follow = cr.toast, cr.toastHint, cr.movedTo, cr.toastKind, cr.follow
}

// title says the move is done before it asks: "Moved to Rejected · why did it end?".
func (cr *closeReasonMode) title() string {
	if len(cr.apps) > 1 {
		return "Moved to " + cr.status + " · why did they end?"
	}
	return "Moved to " + cr.status + " · why did it end?"
}

func (m model) closeReasonBox(p pal, cr *closeReasonMode) string {
	cw := popSmall - popInset
	subject := moveName(cr.apps[0])
	if len(cr.apps) > 1 {
		subject = applications(len(cr.apps)) + " marked"
	}
	var b strings.Builder
	b.WriteString(p.Text(p.Sub).Render(xansi.Truncate(subject, cw, "…")) + "\n\n")
	for i, r := range store.CloseReasons {
		key, label := strconv.Itoa(i+1), closeReasonLabel(r)
		row := "  " + p.Key().Render(key) + "  " + p.Text(p.Fg).Render(label)
		if i == cr.sel {
			bg := p.SelBase(true)
			row = p.SelLine(true, cw+gutter, p.SelGutter(true)+
				p.Key().Background(p.Sel).Render(key)+bg.Render("  ")+p.SelText(true).Render(label))
		}
		b.WriteString(row + "\n")
	}
	b.WriteString("\n" + p.Text(p.Dim).Render(xansi.Truncate("u undoes the move · e edits the reason later", cw, "…")))
	return popup(p, cr.title(), false, popSmall, m.viewWidth(), b.String(), p.Keys(cr.Keys()))
}
