package ui

import (
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/internal/store"
)

// nextStep is the one thing to do next with an Application, worked out from its Status
// and documents, and the key that does it here. key is "" when no key does it (a
// document with no Harness set to write it), so the line never offers a key that
// would do nothing.
type nextStep struct {
	label, what, why, key, desc string // label is "NEXT" unless the step says otherwise
}

// Words the next step repeats.
const (
	followUpLabel = "FOLLOW-UP"
	descNudge     = "draft nudge"
	descMove      = "move"
	descOpen      = "open"
	stepAddJD     = "add the JD"
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
		if past, due := followUpDaysPast(a, m.now(), m.followUpDays); due {
			step := nextStep{label: followUpLabel, what: followUpWords(past), key: "n", desc: "note"}
			if harness {
				step.key, step.desc = "r", descNudge
			}
			return step, true
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

// nextLine is the next step as a field row, "NEXT    add the JD   [enter] open" or
// "FOLLOW-UP  due 9 days ago   [r] draft nudge", inside w cells. The reason joins the
// words only when the whole row fits. When even the words and the key do not fit, wrap
// puts the key and its action on a second line at the same right edge (the card is
// narrow); without wrap the row cuts as every field row does.
func (m model) nextLine(p pal, a store.Application, w int, wrap bool) string {
	s, ok := m.nextAction(a)
	if !ok {
		return ""
	}
	name := "NEXT"
	if s.label != "" {
		name = s.label
	}
	labelW := max(fieldLabelW, xansi.StringWidth(name)+2) //nolint:mnd // the gap after a long label
	what := p.Text(p.Fg).Render(s.what)
	fits := func(value string) bool {
		return labelW+xansi.StringWidth(value)+1+xansi.StringWidth("["+s.key+"] "+s.desc) <= w
	}
	if s.why != "" {
		if full := what + p.Text(p.Dim).Render(" · "+s.why); fits(full) {
			what = full
		}
	}
	if wrap && s.key != "" && !fits(what) {
		return p.fieldRowIn(labelW, name, what, "", "", w) + "\n" + p.fieldRow("", "", s.key, s.desc, w)
	}
	return p.fieldRowIn(labelW, name, what, s.key, s.desc, w)
}
