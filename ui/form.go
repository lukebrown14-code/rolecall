package ui

import (
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/rivo/uniseg"
)

// choice is one value of a choice field and how it reads on screen.
type choice struct{ value, label string }

// formField is one row of the Add/Edit form: a text input, or a choice cycled with
// ←→ (or ↑↓).
type formField struct {
	label       string
	required    bool
	value       string
	choices     []choice // nil for a text field
	placeholder string
	validate    func(string) error
	err         string
}

// form is the Add/Edit pop-up: enter checks the focused field and moves on; enter on
// the last field checks them all and submits.
type form struct {
	title  string
	fields []formField
	focus  int
}

const (
	formLabelWidth = 12
	formChrome     = 5 // focus mark (2), required star, space, text cursor
)

// pasted is clipboard text made fit for a one-line field: line breaks become spaces
// and surrounding blanks are dropped.
func pasted(s string) string {
	return strings.TrimSpace(strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ").Replace(s))
}

// dropLastGrapheme removes the last user-perceived character, so backspace never
// leaves half an emoji or a stray combining mark.
func dropLastGrapheme(s string) string {
	g, last := uniseg.NewGraphemes(s), 0
	for g.Next() {
		last, _ = g.Positions()
	}
	return s[:last]
}

// paste adds clipboard text to the focused text field.
func (f *form) paste(s string) {
	if fld := &f.fields[f.focus]; fld.choices == nil {
		fld.value += pasted(s)
		fld.err = ""
	}
}

// key applies one key press. It reports whether the form was submitted.
func (f *form) key(k tea.KeyPressMsg) bool {
	fld := &f.fields[f.focus]
	switch s := k.String(); s {
	case "tab", "down", "up", "shift+tab", "left", "right":
		if fld.choices != nil && s != "tab" && s != "shift+tab" {
			i := slices.IndexFunc(fld.choices, func(c choice) bool { return c.value == fld.value })
			step := 1
			if s == "up" || s == "left" {
				step = len(fld.choices) - 1
			}
			fld.value = fld.choices[(max(i, 0)+step)%len(fld.choices)].value
			return false
		}
		if s == "tab" || s == "down" {
			f.focus = min(f.focus+1, len(f.fields)-1)
		} else if s == "shift+tab" || s == "up" {
			f.focus = max(f.focus-1, 0)
		}
	case "backspace":
		if fld.choices == nil && fld.value != "" {
			fld.value = dropLastGrapheme(fld.value)
			fld.err = ""
		}
	case "enter":
		if !fld.check() {
			return false
		}
		if f.focus < len(f.fields)-1 {
			f.focus++
			return false
		}
		for i := range f.fields {
			if !f.fields[i].check() {
				f.focus = i
				return false
			}
		}
		return true
	default:
		if fld.choices == nil && k.Text != "" {
			fld.value += k.Text
			fld.err = ""
		}
	}
	return false
}

// check validates one field and records its error.
func (fld *formField) check() bool {
	fld.err = ""
	if fld.required && strings.TrimSpace(fld.value) == "" {
		fld.err = fld.label + " is required"
	} else if fld.validate != nil {
		if err := fld.validate(fld.value); err != nil {
			fld.err = err.Error()
		}
	}
	return fld.err == ""
}

func (f *form) value(label string) string {
	for _, fld := range f.fields {
		if fld.label == label {
			return fld.value
		}
	}
	return ""
}

// formBox draws the form: right-aligned labels in a column, * on required fields,
// ‹ › around choices, dim italic placeholders, errors on the field's own row.
func (m model) formBox(p pal, fm *formMode) string {
	f := fm.form
	avail := max(minTitle, popWidth(popForm, m.viewWidth())-popInset-formLabelWidth-formChrome)
	rows := make([]string, len(f.fields))
	for i, fld := range f.fields {
		focused := i == f.focus
		mark, labelC := "  ", p.dim
		if focused {
			mark, labelC = p.text(p.accent).Render("▌ "), p.accent
		}
		req := " "
		if fld.required {
			req = p.text(p.bad).Render("*")
		}
		valC := p.sub
		if focused {
			valC = p.fg
		}
		var val string
		switch {
		case fld.choices != nil:
			arrow := p.faint
			if focused {
				arrow = p.accent
			}
			label := fld.value
			for _, c := range fld.choices {
				if c.value == fld.value {
					label = c.label
				}
			}
			val = p.text(arrow).Render("‹ ") + p.text(valC).Bold(focused).Render(label) + p.text(arrow).Render(" ›")
		case fld.value == "":
			if focused {
				val = p.text(p.accent).Render("▏")
			}
			val += p.text(p.dim).Italic(true).Render(fld.placeholder)
		default:
			// Long text shows its end, where the typing happens.
			shown := fld.value
			if over := xansi.StringWidth(shown) - avail; over > 0 {
				shown = xansi.TruncateLeft(shown, over+1, "…")
			}
			val = p.text(valC).Render(shown)
			if focused {
				val += p.text(p.accent).Render("▏")
			}
		}
		rows[i] = mark + lipgloss.NewStyle().Foreground(labelC).Bold(focused).Width(formLabelWidth).Align(lipgloss.Right).Render(fld.label) + req + " " + val
		if fld.err != "" {
			// The error gets its own line under the value, so it is never cut off.
			rows[i] += "\n" + strings.Repeat(" ", formChrome-1+formLabelWidth) + p.text(p.bad).Render("✕ "+fld.err)
		}
	}
	return popup(p, f.title, false, popForm, m.viewWidth(), strings.Join(rows, "\n"),
		p.keys(fm.Keys()))
}
