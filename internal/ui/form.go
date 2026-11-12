package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/internal/ui/forms"
)

type form = forms.Form
type formField = forms.Field
type choice = forms.Choice

var newFormOf = forms.New

const (
	formOpen      = forms.Open
	formSubmit    = forms.Submit
	formCancel    = forms.Cancel
	formMark      = 2
	formReq       = 2
	formUnderline = 24
	keyCtrlS      = "ctrl+s"
	keyNoEsc      = "n esc"
)

// formBox draws the form at width w (before the pop-up grows to fit its hints).
func (m model) formBox(p pal, f *form, w int) string {
	lw := f.LabelWidth()
	indent := strings.Repeat(" ", formMark+lw+formReq)
	avail := max(minTitle, popWidth(w, m.viewWidth())-popInset-formMark-lw-formReq-1)
	required := false
	rows := make([]string, 0, len(f.Fields))
	for i := range f.Fields {
		fld := &f.Fields[i]
		required = required || fld.Required
		focused := i == f.Focus && !f.Discarding
		mark, label := "  ", p.Text(p.Sub).Width(lw).Align(lipgloss.Right).Render(fld.Label)
		if focused {
			mark = p.Text(p.Accent).Render("▌ ")
			label = p.Text(p.Accent).Bold(true).Width(lw).Align(lipgloss.Right).Render(fld.Label)
		}
		req := " "
		if fld.Required {
			// Neutral until the field fails; then the * turns with its error line.
			req = p.Text(p.Dim).Render("*")
			if fld.Err != "" {
				req = p.Text(p.Bad).Bold(true).Render("*")
			}
		}
		row := mark + label + req + " " + m.formValue(p, f, fld, focused, avail)
		if f.ShowDirty && fld.Value != fld.Orig {
			row += " " + p.Text(p.Info).Render("●")
		}
		if fld.Err != "" {
			// The error has its own line under the value, so it is never cut off.
			for _, l := range wrapWords("✕ "+fld.Err, avail) {
				row += "\n" + indent + p.Text(p.Bad).Render(l)
			}
		}
		if fld.Label == fieldURL {
			// Fetch failures and visa/clearance signals sit under the URL; signals are
			// shown for the Owner's eyes only and never written to the Application.
			for _, s := range f.Signals {
				row += "\n" + indent + p.Text(p.Warn).Render("⚠ "+s)
			}
			if f.FetchErr != "" {
				row += "\n" + indent + p.Text(p.Bad).Render("✕ "+f.FetchErr)
			}
		}
		rows = append(rows, row)
	}
	if f.Discarding {
		rows = append(rows, "", p.Text(p.Warn).Bold(true).Render("Discard changes?")+p.Text(p.Sub).Render(" y/n"))
	}
	note := ""
	if required {
		note = "* required"
	}
	return popupNote(p, f.Title, note, false, w, m.viewWidth(), strings.Join(rows, "\n"), p.Keys(f.Keys()))
}

// formValue is one field's value cell: a choice between arrows, the URL spinner, an empty
// field's placeholder or underline, or the typed text with the cursor after it.
func (m model) formValue(p pal, f *form, fld *formField, focused bool, avail int) string {
	cursor := ""
	if focused {
		cursor = p.Text(p.Accent).Render("▏")
	}
	valC := p.Sub
	if focused {
		valC = p.Fg
	}
	aside := ""
	if fld.Note != "" {
		aside = p.Text(p.Dim).Render("  " + fld.Note)
	}
	switch {
	case fld.Choices != nil:
		arrow := p.Dim
		if focused {
			arrow = p.Accent
		}
		label := fld.Value
		for _, c := range fld.Choices {
			if c.Value == fld.Value {
				label = c.Label
			}
		}
		return p.Text(arrow).Render("‹ ") + p.Text(valC).Bold(focused).Render(label) + p.Text(arrow).Render(" ›") + aside
	case fld.Label == fieldURL && f.Fetching:
		return p.Text(p.Accent).Render(m.spinner.View()) + p.Text(p.Sub).Render(" Fetching…")
	case fld.Value == "":
		if fld.Placeholder != "" {
			return cursor + p.Text(p.Dim).Italic(true).Render(fld.Placeholder) + aside
		}
		// A faint underline shows where to type, apart from the labels and examples.
		line := min(formUnderline, max(1, avail-xansi.StringWidth(fld.Note)-gutter-1))
		return cursor + p.Text(p.Faint).Render(strings.Repeat("_", line)) + aside
	}
	// Long text shows its end, where the typing happens.
	shown := fld.Value
	unit := ""
	if fld.Unit != "" {
		unit = p.Text(p.Dim).Render(" " + fld.Unit)
	}
	if over := xansi.StringWidth(shown) + xansi.StringWidth(unit) + xansi.StringWidth(aside) - avail; over > 0 {
		aside = ""
		if over = xansi.StringWidth(shown) + xansi.StringWidth(unit) - avail; over > 0 {
			shown = xansi.TruncateLeft(shown, over+1, "…")
		}
	}
	return p.Text(valC).Render(shown) + cursor + unit + aside
}

// formPopup is a pop-up that is only a form: the Search area, a gate, a machine setting,
// a Track's add and rename. save runs on submit; an error it returns goes under the
// focused field and keeps the pop-up open, and otherwise its mode replaces the pop-up
// (nil closes it).
type formPopup struct {
	form  *form
	width int
	save  func(m *model, f *form) (mode, error)
}

func (fp *formPopup) Keys() [][2]string { return fp.form.Keys() }

func (fp *formPopup) boxed() bool { return true }

func (fp *formPopup) View(m *model, p pal) string { return m.formBox(p, fp.form, fp.width) }

func (fp *formPopup) Update(m *model, msg tea.Msg) (mode, tea.Cmd) {
	switch fp.form.Update(msg) {
	case formOpen:
		return fp, nil
	case formCancel:
		return nil, nil
	}
	next, err := fp.save(m, fp.form)
	if err != nil {
		fp.form.Focused().Err = err.Error()
		return fp, nil
	}
	return next, nil
}

var pasted = forms.Pasted
var dropLastGrapheme = forms.DropLastGrapheme
