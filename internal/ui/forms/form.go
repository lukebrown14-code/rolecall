// Package forms owns field editing, validation, focus, and submission.
package forms

import (
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/rivo/uniseg"
)

// The one Form every Form pop-up draws: Add and Edit, the Search area, a gate, a machine
// setting, and a Track's add and rename. Its conventions are shared, so they are set here
// once: a right-aligned label column, ▌ and the accent on the focused field, a muted * on
// required fields with "* required" in the top border, a faint underline where an empty
// field takes typing, errors on their own line under the field, enter to the next field
// (saving on the last), ctrl+s to save from anywhere, and esc asking once before it
// throws typed text away.

// Choice is one value of a Choice field and how it reads on screen.
type Choice struct{ Value, Label string }

// FieldRole names behaviour a field carries beyond editing, so the engine never matches
// on a label.
type FieldRole int

const (
	RoleNone FieldRole = iota
	// RoleURL is a field whose edits invalidate a fetch's result.
	RoleURL
)

// Field is one row of a Form: a text input, or a Choice cycled with ←→ (or ↑↓).
type Field struct {
	Label       string
	Role        FieldRole
	Required    bool
	Value       string
	Orig        string   // the value the Form opened with, so an edit shows and esc asks
	Choices     []Choice // nil for a text field
	Placeholder string   // an example shown while the field is empty
	Note        string   // a short aside after the value, e.g. "optional" or "min 150"
	Unit        string   // after a typed value: "days", "words"
	Validate    func(string) error
	Err         string
}

// Form is a Form pop-up's state: its fields, which one has focus, and the Add Form's URL
// fetch.
type Form struct {
	Title  string
	Fields []Field
	Focus  int
	// submit names what the last enter and ctrl+s do: "save" unless set.
	Submit string
	// showDirty marks a field edited but not yet saved with ●; an Add Form leaves it off,
	// since every field there is new.
	ShowDirty bool
	// discarding is the "Discard changes?" question esc asks while typed text would be
	// lost.
	Discarding bool

	// URL fetch state (v0.2, Add only).
	Fetching bool     // a fetch is in flight; the URL field shows a spinner
	Fetched  bool     // a fetch succeeded; jd holds the ad to write on save
	Jd       string   // the fetched ad text, written to jd.md on save
	Signals  []string // visa/clearance signals under the URL; never stored
	FetchErr string   // one-line fetch failure, under the URL
}

// New builds a Form, remembering each field's opening value.
func New(title string, fields ...Field) *Form {
	for i := range fields {
		fields[i].Orig = fields[i].Value
	}
	return &Form{Title: title, Fields: fields}
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

// Result is what one message did to a Form.
type Result int

const (
	Open Result = iota
	Submit
	Cancel
)

// pasted is clipboard text made fit for a one-line field: line breaks become spaces
// and surrounding blanks are dropped.
func Pasted(s string) string {
	return strings.TrimSpace(strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ").Replace(s))
}

// dropLastGrapheme removes the last user-perceived character, so backspace never
// leaves half an emoji or a stray combining mark.
func DropLastGrapheme(s string) string {
	g, last := uniseg.NewGraphemes(s), 0
	for g.Next() {
		last, _ = g.Positions()
	}
	return s[:last]
}

// paste adds clipboard text to the focused text field.
func (f *Form) Paste(s string) {
	if fld := &f.Fields[f.Focus]; fld.Choices == nil {
		fld.Value += Pasted(s)
		fld.Err = ""
		f.Edited(fld)
	}
}

// focused is the field the cursor is on.
func (f *Form) Focused() *Field { return &f.Fields[f.Focus] }

// setValue writes a field the Form filled in itself, without touching its fetch state.
func (f *Form) SetValue(label, value string) {
	for i := range f.Fields {
		if f.Fields[i].Label == label {
			f.Fields[i].Value = value
			return
		}
	}
}

// clearFetch forgets a fetch's result.
func (f *Form) ClearFetch() {
	f.Fetched, f.Jd, f.FetchErr, f.Signals = false, "", "", nil
}

// edited notes a text field changed. Editing a URL field invalidates any fetch, so a
// stale ad, signals or failure never linger over a new URL.
func (f *Form) Edited(fld *Field) {
	if fld.Role == RoleURL {
		f.ClearFetch()
	}
}

// dirty reports whether any field differs from what the Form opened with.
func (f *Form) Dirty() bool {
	return slices.ContainsFunc(f.Fields, func(fld Field) bool { return fld.Value != fld.Orig })
}

// update runs one message through the Form: paste, typing, esc and its discard question,
// enter and ctrl+s. The caller saves on Submit and closes on Cancel.
func (f *Form) Update(msg tea.Msg) Result {
	if paste, ok := msg.(tea.PasteMsg); ok {
		if !f.Discarding {
			f.Paste(paste.Content)
		}
		return Open
	}
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return Open
	}
	if f.Discarding {
		switch k.String() {
		case "y":
			return Cancel
		case "n", "esc":
			f.Discarding = false
		}
		return Open
	}
	switch k.String() {
	case "esc":
		if f.Dirty() {
			f.Discarding = true
			return Open
		}
		return Cancel
	case keyCtrlS:
		if f.CheckAll() {
			return Submit
		}
		return Open
	}
	if f.Key(k) {
		return Submit
	}
	return Open
}

// key applies one key press. It reports whether the Form was submitted.
func (f *Form) Key(k tea.KeyPressMsg) bool {
	fld := &f.Fields[f.Focus]
	switch s := k.String(); s {
	case "tab", "down", "up", "shift+tab", "left", "right":
		if fld.Choices != nil && s != "tab" && s != "shift+tab" {
			i := slices.IndexFunc(fld.Choices, func(c Choice) bool { return c.Value == fld.Value })
			step := 1
			if s == "up" || s == "left" {
				step = len(fld.Choices) - 1
			}
			fld.Value = fld.Choices[(max(i, 0)+step)%len(fld.Choices)].Value
			return false
		}
		next := f.Focus
		if s == "tab" || s == "down" {
			next = min(f.Focus+1, len(f.Fields)-1)
		} else if s == "shift+tab" || s == "up" {
			next = max(f.Focus-1, 0)
		}
		if next != f.Focus {
			// Leaving a field checks its format (a date, a number), not whether it is
			// filled: passing an empty required field on the way is fine.
			fld.CheckFormat()
			f.Focus = next
		}
	case keyBackspace:
		if fld.Choices == nil && fld.Value != "" {
			fld.Value = DropLastGrapheme(fld.Value)
			fld.Err = ""
			f.Edited(fld)
		}
	case "enter":
		if !fld.Check() {
			return false
		}
		if f.Focus < len(f.Fields)-1 {
			f.Focus++
			return false
		}
		return f.CheckAll()
	default:
		if fld.Choices == nil && k.Text != "" {
			fld.Value += k.Text
			fld.Err = ""
			f.Edited(fld)
		}
	}
	return false
}

// checkAll checks every field, marks each one that fails, and moves focus to the first
// of them. It reports whether all passed.
func (f *Form) CheckAll() bool {
	first := -1
	for i := range f.Fields {
		if !f.Fields[i].Check() && first < 0 {
			first = i
		}
	}
	if first >= 0 {
		f.Focus = first
		return false
	}
	return true
}

// check validates one field and records its error.
func (fld *Field) Check() bool {
	fld.Err = ""
	if fld.Required && strings.TrimSpace(fld.Value) == "" {
		fld.Err = fld.Label + " is required"
		return false
	}
	return fld.CheckFormat()
}

// checkFormat runs the field's own rule only, leaving an empty required field alone.
func (fld *Field) CheckFormat() bool {
	if fld.Validate == nil {
		return true
	}
	fld.Err = ""
	if err := fld.Validate(fld.Value); err != nil {
		fld.Err = err.Error()
	}
	return fld.Err == ""
}

func (f *Form) Value(label string) string {
	for _, fld := range f.Fields {
		if fld.Label == label {
			return fld.Value
		}
	}
	return ""
}

// submitHint is the word for what the Form's last enter and ctrl+s do.
func (f *Form) SubmitHint() string {
	if f.Submit != "" {
		return f.Submit
	}
	return hintSave
}

// keys are the Form's own key hints, for its bottom border and the key line. They say
// what enter does where the focus is: next, or save on the last field.
func (f *Form) Keys() [][2]string {
	if f.Discarding {
		return [][2]string{{"y", hintDiscard}, {keyNoEsc, hintKeepEditing}}
	}
	enter := hintNext
	if f.Focus == len(f.Fields)-1 {
		enter = f.SubmitHint()
	}
	keys := [][2]string{{"enter", enter}}
	if len(f.Fields) > 1 {
		keys = append(keys, [2]string{keyCtrlS, f.SubmitHint()})
	}
	if slices.ContainsFunc(f.Fields, func(fld Field) bool { return fld.Choices != nil }) {
		keys = append(keys, [2]string{"←→", "change"})
	}
	return append(keys, [2]string{"esc", "cancel"})
}

// labelWidth is the label column: as wide as the longest label.
func (f *Form) LabelWidth() int {
	w := 0
	for _, fld := range f.Fields {
		w = max(w, xansi.StringWidth(fld.Label))
	}
	return w
}

const (
	keyBackspace = "backspace"
	hintSave     = "save"
	hintNext     = "next"
)
