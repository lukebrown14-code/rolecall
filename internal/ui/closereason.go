package ui

import (
	"github.com/lukebrown14-code/rolecall/internal/store"
)

// fieldCloseReason is the close-reason pop-up's one field.
const fieldCloseReason = "Reason"

// closeReasonSkip is the pop-up's first choice, which writes nothing.
const closeReasonSkip = "skip"

// closeReasonPopup asks why an Application just moved to Rejected or Withdrawn ended. The
// move has already happened: enter with "skip" or esc leaves it as it is, and the reason
// can be set no other way than another move, so the pop-up is the one chance and optional.
func (m model) closeReasonPopup(a store.Application, status string) mode {
	choices := []choice{{Value: "", Label: closeReasonSkip}}
	for _, r := range store.CloseReasons {
		choices = append(choices, choice{Value: r, Label: r})
	}
	f := newFormOf("Why did it end? · "+status, formField{Label: fieldCloseReason, Choices: choices})
	f.Intro = moveName(a)
	f.Submit = "save"
	return &formPopup{form: f, width: popSmall, save: func(m *model, f *form) (mode, error) {
		reason := f.Value(fieldCloseReason)
		if reason == "" {
			return nil, nil
		}
		if err := m.store.SetCloseReason(a.Folder, reason); err != nil {
			m.err = err
			return nil, nil
		}
		m.reload(m.cursorFolder())
		return nil, nil
	}}
}
