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
	formCancel    = forms.Cancel
	formMark      = 2
	formReq       = 2
	formUnderline = 24
	keyNoEsc      = "n / esc"
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
		if fld.Group != "" && (i == 0 || f.Fields[i-1].Group != fld.Group) {
			// A group's heading, as the card labels its sections, with a blank line between
			// groups.
			if len(rows) > 0 {
				rows = append(rows, "")
			}
			rows = append(rows, strings.Repeat(" ", formMark)+p.blockLabel(strings.ToUpper(fld.Group)))
		}
		required = required || fld.Required
		focused := i == f.Focus && !f.Discarding
		mark, label := "  ", p.Text(p.Sub).Width(lw).Align(lipgloss.Right).Render(fld.Label)
		if focused {
			mark = p.SelGutter(false)
			label = p.Text(p.Sub).Bold(true).Width(lw).Align(lipgloss.Right).Render(fld.Label)
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
			row += " " + p.Text(p.Info).Render(glyphChanged)
		}
		if fld.Err != "" {
			// The error has its own line under the value, so it is never cut off.
			for _, l := range wrapWords(errorMark+" "+fld.Err, avail) {
				row += "\n" + indent + p.Text(p.Bad).Render(l)
			}
		}
		if fld.Role == forms.RoleURL {
			// Fetch failures and visa/clearance signals sit under the URL; signals are
			// shown for the Owner's eyes only and never written to the Application.
			for _, s := range f.Signals {
				row += "\n" + indent + p.Text(p.Warn).Render("⚠ "+s)
			}
			if f.FetchErr != "" {
				row += "\n" + indent + p.Text(p.Bad).Render(errorMark+" "+f.FetchErr)
			}
		}
		rows = append(rows, row)
	}
	if f.Intro != "" {
		// What the Form is for, above the fields; the pop-up wraps it.
		rows = append([]string{p.Text(p.Sub).Render(f.Intro), ""}, rows...)
	}
	title := f.Title
	note := ""
	if required {
		note = "* required"
	}
	if f.Discarding {
		// The question goes in the title, not a body line: a form taller than its pop-up
		// scrolls, and the confirmation must never scroll out of sight. Losing typed text
		// is drawn as a warning (⚠ and the Warn border), at least as strong as the Docs
		// not Ready question, with what is lost said first.
		title, note = "Discard changes?", ""
		rows = append([]string{p.Text(p.Sub).Render("Your typed values will be lost."), ""}, rows...)
	}
	return popupNote(p, title, note, f.Discarding, w, m.viewWidth(), strings.Join(rows, "\n"), p.Keys(f.Keys()))
}

// formValue is one field's value cell: a choice between arrows, the URL spinner, an empty
// field's placeholder or underline, or the typed text with the cursor after it.
func (m model) formValue(p pal, f *form, fld *formField, focused bool, avail int) string {
	cursor := ""
	if focused {
		cursor = p.Text(p.Fg).Render("▏")
	}
	valC := p.Sub
	if focused {
		valC = p.Fg
	}
	aside := ""
	if fld.Note != "" {
		aside = p.Text(p.Dim).Render("  " + fld.Note)
	}
	if fld.Echo != nil {
		if echo := fld.Echo(fld.Value); echo != "" {
			aside = p.Text(p.Info).Render("  " + echo)
		}
	}
	switch {
	case fld.Choices != nil:
		arrow := p.Dim
		if focused {
			arrow = p.Fg
		}
		label := fld.Value
		for _, c := range fld.Choices {
			if c.Value == fld.Value {
				label = c.Label
			}
		}
		return p.Text(arrow).Render("‹ ") + p.Text(valC).Bold(focused).Render(label) + p.Text(arrow).Render(" ›") + aside
	case fld.Role == forms.RoleURL && f.Fetching:
		return p.SpinnerStyle().Render(m.spinner.View()) + p.Text(p.Sub).Render(" Fetching…")
	case fld.Value == "":
		// The cursor takes the first cell of an empty field rather than pushing it right,
		// so an empty field starts in the same column focused or not.
		if fld.Placeholder != "" {
			ph := p.Text(p.Dim).Italic(true)
			if !focused {
				return ph.Render(fld.Placeholder) + aside
			}
			first, _ := xansi.FirstGraphemeCluster(fld.Placeholder, xansi.GraphemeWidth)
			return ph.Reverse(true).Render(first) + ph.Render(strings.TrimPrefix(fld.Placeholder, first)) + aside
		}
		// A faint underline shows where to type, apart from the labels and examples.
		line := min(formUnderline, max(1, avail-xansi.StringWidth(fld.Note)-gutter-1))
		if focused {
			return cursor + p.Text(p.Faint).Render(strings.Repeat("_", line-1)) + aside
		}
		return p.Text(p.Faint).Render(strings.Repeat("_", line)) + aside
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
	if focused && f.Selected {
		// A selected opening value: the first key typed replaces it.
		return p.Text(valC).Reverse(true).Render(shown) + cursor + unit + aside
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

// quitHeld is ctrl+c while the Owner has typed into a Settings field or pop-up, or the Add
// or Edit Application form: rather than drop the text, it asks "Discard changes?" as esc
// on a form does. held is false when there is nothing typed to lose, or the question is
// already up, and the quit goes ahead.
func (m model) quitHeld() (next model, held bool) {
	var f *form
	switch o := m.overlay.(type) {
	case *formPopup:
		f = o.form
	case *settingsPopup:
		f = o.p.Form
	case *formMode: // the Add and Edit Application forms
		f = o.form
	}
	if f != nil {
		if f.Discarding || !f.Dirty() {
			return m, false
		}
		f.Discarding = true
		return m, true
	}
	if !m.fieldOpen() {
		return m, false
	}
	st, held := m.settings.HoldQuit()
	m.settings = &st
	return m, held
}

var pasted = forms.Pasted
var dropLastGrapheme = forms.DropLastGrapheme
