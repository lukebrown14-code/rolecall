package ui

import (
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/rivo/uniseg"
)

// The one form every form pop-up draws: Add and Edit, the Search area, a gate, a machine
// setting, and a Track's add and rename. Its conventions are shared, so they are set here
// once: a right-aligned label column, ▌ and the accent on the focused field, a muted * on
// required fields with "* required" in the top border, a faint underline where an empty
// field takes typing, errors on their own line under the field, enter to the next field
// (saving on the last), ctrl+s to save from anywhere, and esc asking once before it
// throws typed text away.

// choice is one value of a choice field and how it reads on screen.
type choice struct{ value, label string }

// formField is one row of a form: a text input, or a choice cycled with ←→ (or ↑↓).
type formField struct {
	label       string
	required    bool
	value       string
	orig        string   // the value the form opened with, so an edit shows and esc asks
	choices     []choice // nil for a text field
	placeholder string   // an example shown while the field is empty
	note        string   // a short aside after the value, e.g. "optional" or "min 150"
	unit        string   // after a typed value: "days", "words"
	validate    func(string) error
	err         string
}

// form is a form pop-up's state: its fields, which one has focus, and the Add form's URL
// fetch.
type form struct {
	title  string
	fields []formField
	focus  int
	// submit names what the last enter and ctrl+s do: "save" unless set.
	submit string
	// showDirty marks a field edited but not yet saved with ●; an Add form leaves it off,
	// since every field there is new.
	showDirty bool
	// discarding is the "Discard changes?" question esc asks while typed text would be
	// lost.
	discarding bool

	// URL fetch state (v0.2, Add only).
	fetching bool     // a fetch is in flight; the URL field shows a spinner
	fetched  bool     // a fetch succeeded; jd holds the ad to write on save
	jd       string   // the fetched ad text, written to jd.md on save
	signals  []string // visa/clearance signals under the URL; never stored
	fetchErr string   // one-line fetch failure, under the URL
}

// newFormOf builds a form, remembering each field's opening value.
func newFormOf(title string, fields ...formField) *form {
	for i := range fields {
		fields[i].orig = fields[i].value
	}
	return &form{title: title, fields: fields}
}

const (
	formMark        = 2 // "▌ " or its blank
	formReq         = 2 // the * column and the space after it
	formUnderline   = 24
	keyCtrlS        = "ctrl+s"
	hintDiscard     = "discard"
	hintKeepEditing = "keep editing"
	keyNoEsc        = "n esc"
)

// formResult is what one message did to a form.
type formResult int

const (
	formOpen formResult = iota
	formSubmit
	formCancel
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
		f.edited(fld)
	}
}

// focused is the field the cursor is on.
func (f *form) focused() *formField { return &f.fields[f.focus] }

// setValue writes a field the form filled in itself, without touching its fetch state.
func (f *form) setValue(label, value string) {
	for i := range f.fields {
		if f.fields[i].label == label {
			f.fields[i].value = value
			return
		}
	}
}

// clearFetch forgets a fetch's result.
func (f *form) clearFetch() {
	f.fetched, f.jd, f.fetchErr, f.signals = false, "", "", nil
}

// edited notes a text field changed. Editing the URL invalidates any fetch, so a
// stale ad, signals or failure never linger over a new URL.
func (f *form) edited(fld *formField) {
	if fld.label == fieldURL {
		f.clearFetch()
	}
}

// dirty reports whether any field differs from what the form opened with.
func (f *form) dirty() bool {
	return slices.ContainsFunc(f.fields, func(fld formField) bool { return fld.value != fld.orig })
}

// update runs one message through the form: paste, typing, esc and its discard question,
// enter and ctrl+s. The caller saves on formSubmit and closes on formCancel.
func (f *form) update(msg tea.Msg) formResult {
	if paste, ok := msg.(tea.PasteMsg); ok {
		if !f.discarding {
			f.paste(paste.Content)
		}
		return formOpen
	}
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return formOpen
	}
	if f.discarding {
		switch k.String() {
		case "y":
			return formCancel
		case "n", "esc":
			f.discarding = false
		}
		return formOpen
	}
	switch k.String() {
	case "esc":
		if f.dirty() {
			f.discarding = true
			return formOpen
		}
		return formCancel
	case keyCtrlS:
		if f.checkAll() {
			return formSubmit
		}
		return formOpen
	}
	if f.key(k) {
		return formSubmit
	}
	return formOpen
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
		next := f.focus
		if s == "tab" || s == "down" {
			next = min(f.focus+1, len(f.fields)-1)
		} else if s == "shift+tab" || s == "up" {
			next = max(f.focus-1, 0)
		}
		if next != f.focus {
			// Leaving a field checks its format (a date, a number), not whether it is
			// filled: passing an empty required field on the way is fine.
			fld.checkFormat()
			f.focus = next
		}
	case keyBackspace:
		if fld.choices == nil && fld.value != "" {
			fld.value = dropLastGrapheme(fld.value)
			fld.err = ""
			f.edited(fld)
		}
	case "enter":
		if !fld.check() {
			return false
		}
		if f.focus < len(f.fields)-1 {
			f.focus++
			return false
		}
		return f.checkAll()
	default:
		if fld.choices == nil && k.Text != "" {
			fld.value += k.Text
			fld.err = ""
			f.edited(fld)
		}
	}
	return false
}

// checkAll checks every field, marks each one that fails, and moves focus to the first
// of them. It reports whether all passed.
func (f *form) checkAll() bool {
	first := -1
	for i := range f.fields {
		if !f.fields[i].check() && first < 0 {
			first = i
		}
	}
	if first >= 0 {
		f.focus = first
		return false
	}
	return true
}

// check validates one field and records its error.
func (fld *formField) check() bool {
	fld.err = ""
	if fld.required && strings.TrimSpace(fld.value) == "" {
		fld.err = fld.label + " is required"
		return false
	}
	return fld.checkFormat()
}

// checkFormat runs the field's own rule only, leaving an empty required field alone.
func (fld *formField) checkFormat() bool {
	if fld.validate == nil {
		return true
	}
	fld.err = ""
	if err := fld.validate(fld.value); err != nil {
		fld.err = err.Error()
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

// submitHint is the word for what the form's last enter and ctrl+s do.
func (f *form) submitHint() string {
	if f.submit != "" {
		return f.submit
	}
	return hintSave
}

// keys are the form's own key hints, for its bottom border and the key line. They say
// what enter does where the focus is: next, or save on the last field.
func (f *form) keys() [][2]string {
	if f.discarding {
		return [][2]string{{"y", hintDiscard}, {keyNoEsc, hintKeepEditing}}
	}
	enter := hintNext
	if f.focus == len(f.fields)-1 {
		enter = f.submitHint()
	}
	keys := [][2]string{{"enter", enter}}
	if len(f.fields) > 1 {
		keys = append(keys, [2]string{keyCtrlS, f.submitHint()})
	}
	if slices.ContainsFunc(f.fields, func(fld formField) bool { return fld.choices != nil }) {
		keys = append(keys, [2]string{"←→", "change"})
	}
	return append(keys, [2]string{"esc", "cancel"})
}

// labelWidth is the label column: as wide as the longest label.
func (f *form) labelWidth() int {
	w := 0
	for _, fld := range f.fields {
		w = max(w, xansi.StringWidth(fld.label))
	}
	return w
}

// formBox draws the form at width w (before the pop-up grows to fit its hints).
func (m model) formBox(p pal, f *form, w int) string {
	lw := f.labelWidth()
	indent := strings.Repeat(" ", formMark+lw+formReq)
	avail := max(minTitle, popWidth(w, m.viewWidth())-popInset-formMark-lw-formReq-1)
	required := false
	rows := make([]string, 0, len(f.fields))
	for i := range f.fields {
		fld := &f.fields[i]
		required = required || fld.required
		focused := i == f.focus && !f.discarding
		mark, label := "  ", p.text(p.sub).Width(lw).Align(lipgloss.Right).Render(fld.label)
		if focused {
			mark = p.text(p.accent).Render("▌ ")
			label = p.text(p.accent).Bold(true).Width(lw).Align(lipgloss.Right).Render(fld.label)
		}
		req := " "
		if fld.required {
			// Neutral until the field fails; then the * turns with its error line.
			req = p.text(p.dim).Render("*")
			if fld.err != "" {
				req = p.text(p.bad).Bold(true).Render("*")
			}
		}
		row := mark + label + req + " " + m.formValue(p, f, fld, focused, avail)
		if f.showDirty && fld.value != fld.orig {
			row += " " + p.text(p.info).Render("●")
		}
		if fld.err != "" {
			// The error has its own line under the value, so it is never cut off.
			for _, l := range wrapWords("✕ "+fld.err, avail) {
				row += "\n" + indent + p.text(p.bad).Render(l)
			}
		}
		if fld.label == fieldURL {
			// Fetch failures and visa/clearance signals sit under the URL; signals are
			// shown for the Owner's eyes only and never written to the Application.
			for _, s := range f.signals {
				row += "\n" + indent + p.text(p.warn).Render("⚠ "+s)
			}
			if f.fetchErr != "" {
				row += "\n" + indent + p.text(p.bad).Render("✕ "+f.fetchErr)
			}
		}
		rows = append(rows, row)
	}
	if f.discarding {
		rows = append(rows, "", p.text(p.warn).Bold(true).Render("Discard changes?")+p.text(p.sub).Render(" y/n"))
	}
	note := ""
	if required {
		note = "* required"
	}
	return popupNote(p, f.title, note, false, w, m.viewWidth(), strings.Join(rows, "\n"), p.keys(f.keys()))
}

// formValue is one field's value cell: a choice between arrows, the URL spinner, an empty
// field's placeholder or underline, or the typed text with the cursor after it.
func (m model) formValue(p pal, f *form, fld *formField, focused bool, avail int) string {
	cursor := ""
	if focused {
		cursor = p.text(p.accent).Render("▏")
	}
	valC := p.sub
	if focused {
		valC = p.fg
	}
	aside := ""
	if fld.note != "" {
		aside = p.text(p.dim).Render("  " + fld.note)
	}
	switch {
	case fld.choices != nil:
		arrow := p.dim
		if focused {
			arrow = p.accent
		}
		label := fld.value
		for _, c := range fld.choices {
			if c.value == fld.value {
				label = c.label
			}
		}
		return p.text(arrow).Render("‹ ") + p.text(valC).Bold(focused).Render(label) + p.text(arrow).Render(" ›") + aside
	case fld.label == fieldURL && f.fetching:
		return p.text(p.accent).Render(m.spinner.View()) + p.text(p.sub).Render(" Fetching…")
	case fld.value == "":
		if fld.placeholder != "" {
			return cursor + p.text(p.dim).Italic(true).Render(fld.placeholder) + aside
		}
		// A faint underline shows where to type, apart from the labels and examples.
		line := min(formUnderline, max(1, avail-xansi.StringWidth(fld.note)-gutter-1))
		return cursor + p.text(p.faint).Render(strings.Repeat("_", line)) + aside
	}
	// Long text shows its end, where the typing happens.
	shown := fld.value
	unit := ""
	if fld.unit != "" {
		unit = p.text(p.dim).Render(" " + fld.unit)
	}
	if over := xansi.StringWidth(shown) + xansi.StringWidth(unit) + xansi.StringWidth(aside) - avail; over > 0 {
		aside = ""
		if over = xansi.StringWidth(shown) + xansi.StringWidth(unit) - avail; over > 0 {
			shown = xansi.TruncateLeft(shown, over+1, "…")
		}
	}
	return p.text(valC).Render(shown) + cursor + unit + aside
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

func (fp *formPopup) Keys() [][2]string { return fp.form.keys() }

func (fp *formPopup) boxed() bool { return true }

func (fp *formPopup) View(m *model, p pal) string { return m.formBox(p, fp.form, fp.width) }

func (fp *formPopup) Update(m *model, msg tea.Msg) (mode, tea.Cmd) {
	switch fp.form.update(msg) {
	case formOpen:
		return fp, nil
	case formCancel:
		return nil, nil
	}
	next, err := fp.save(m, fp.form)
	if err != nil {
		fp.form.focused().err = err.Error()
		return fp, nil
	}
	return next, nil
}
