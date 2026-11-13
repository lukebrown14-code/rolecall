package ui

import (
	"strings"

	xansi "github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/internal/store"
)

// nextStep is the one thing to do next with an Application, worked out from its Status
// and documents, and the key that does it here. key is "" when no key does it (a
// document with no Harness set to write it), so the line never offers a key that
// would do nothing.
type nextStep struct {
	what, why, key, desc string
}

// Words the next step repeats.
const (
	descMove  = "move"
	descOpen  = "open"
	stepAddJD = "add the JD"
)

// nextAction is the Application's next step, or false when there is none: a closed or
// malformed Application.
func (m model) nextAction(a store.Application) (nextStep, bool) {
	if a.Malformed != nil {
		return nextStep{}, false
	}
	harness := m.harness != ""
	switch a.Status {
	case store.StatusLead, store.StatusDraft:
		if text, s := closeInfo(a.Closes, m.now()); s == closePast {
			return nextStep{what: "dismiss it", why: text, key: "m", desc: descMove}, true
		}
		switch {
		case !a.JDHasText:
			switch {
			case m.detail == nil:
				return nextStep{what: stepAddJD, key: "enter", desc: descOpen}, true
			case m.detail.tab == tabJD:
				return nextStep{what: stepAddJD, key: "e", desc: "open " + jdFile}, true
			}
			return nextStep{what: stepAddJD, why: "on the JD tab"}, true
		case a.Fit == "" && harness:
			return nextStep{what: "assess the fit", key: "f", desc: "assess fit"}, true
		case !a.ResumeHasText:
			return m.writeStep("write the resume"), true
		case !a.ResumeOnly && !a.CoverLetterHasText:
			return m.writeStep("write the cover letter"), true
		case a.Documents != store.DocsReady:
			if m.plugin != "" && buildable(a) {
				return nextStep{what: "build the PDFs", key: "b", desc: "build"}, true
			}
			return nextStep{what: "build the PDFs"}, true
		}
		return nextStep{what: "send it, then mark it Applied", key: "m", desc: descMove}, true
	case store.StatusApplied, store.StatusInterviewing:
		if _, flag, _ := m.slotText(a); flag != "" {
			return nextStep{what: "follow up", why: "quiet " + strings.TrimPrefix(flag, followUpFlag+" "), key: "n", desc: "note"}, true
		}
		if a.Status == store.StatusInterviewing {
			return nextStep{what: "prepare for the interview", key: "n", desc: "note"}, true
		}
		return nextStep{what: "wait to hear back", key: "m", desc: descMove}, true
	case store.StatusOffer:
		return nextStep{what: "decide on the offer", key: "m", desc: descMove}, true
	}
	return nextStep{}, false
}

// writeStep is a document still to write: the Harness writes it with p, when there is one.
func (m model) writeStep(what string) nextStep {
	if m.harness != "" {
		return nextStep{what: what, key: "p", desc: "prepare docs"}
	}
	return nextStep{what: what}
}

// nextLine is "NEXT  follow up · quiet 9mo   n note", cut to w from the right but keeping
// its key: the reason goes first, then the words are cut.
func (m model) nextLine(p pal, a store.Application, w int) string {
	s, ok := m.nextAction(a)
	if !ok {
		return ""
	}
	label := p.Text(p.Dim).Bold(true).Render("NEXT  ")
	keys := ""
	if s.key != "" {
		keys = "   " + p.Key().Render(s.key) + p.Text(p.Dim).Render(" "+s.desc)
	}
	what := p.Text(p.Fg).Render(s.what)
	if s.why != "" {
		if full := what + p.Text(p.Dim).Render(" · "+s.why); xansi.StringWidth(label+full+keys) <= w {
			what = full
		}
	}
	if xansi.StringWidth(label+what+keys) <= w || keys == "" {
		return label + xansi.Truncate(what, max(0, w-xansi.StringWidth(label+keys)), "…") + keys
	}
	// Too narrow for one line: the key goes under the words, lined up with them.
	return label + xansi.Truncate(what, max(0, w-xansi.StringWidth(label)), "…") + "\n" +
		strings.Repeat(" ", xansi.StringWidth(label)) + strings.TrimLeft(keys, " ")
}
