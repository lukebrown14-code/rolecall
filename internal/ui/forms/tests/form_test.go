package forms_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/internal/ui/forms"
)

// Editing a URL-role field invalidates a fetch's result; editing any other field does
// not. The engine matches the role, never a label.
func TestEditingURLRoleFieldClearsTheFetch(t *testing.T) {
	f := forms.New("Add", forms.Field{Label: "URL", Role: forms.RoleURL}, forms.Field{Label: "Title"})
	f.Fetched, f.Jd, f.Signals = true, "ad text", []string{"signal"}
	f.Edited(&f.Fields[0])
	if f.Fetched || f.Jd != "" || f.Signals != nil {
		t.Fatalf("editing the URL field left fetch state: %+v", f)
	}
	f.Fetched, f.Jd = true, "ad text"
	f.Edited(&f.Fields[1])
	if !f.Fetched || f.Jd != "ad text" {
		t.Fatalf("editing a plain field cleared the fetch: %+v", f)
	}
}

func TestSubmitValidatesAllFieldsAndFocusesFirstError(t *testing.T) {
	f := forms.New("Contact", forms.Field{Label: "Name", Required: true}, forms.Field{Label: "Email", Required: true})
	save := tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}
	if got := f.Update(save); got != forms.Open {
		t.Fatalf("empty form submitted: %v", got)
	}
	if f.Focus != 0 || f.Fields[0].Err == "" || f.Fields[1].Err == "" {
		t.Fatalf("missing errors or wrong focus: %+v", f)
	}
	f.SetValue("Name", "Alex")
	if got := f.Update(save); got != forms.Open || f.Focus != 1 {
		t.Fatalf("expected focus on remaining error: %+v", f)
	}
	f.SetValue("Email", "alex@example.test")
	if got := f.Update(save); got != forms.Submit {
		t.Fatalf("valid form did not submit: %v", got)
	}
}

func TestPasteAndBackspacePreserveGraphemesAndDiscardConfirmation(t *testing.T) {
	f := forms.New("Name", forms.Field{Label: "Name"})
	f.Update(tea.PasteMsg{Content: "  Alex\r\n👩‍💻  "})
	f.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	if got := f.Value("Name"); got != "Alex " {
		t.Fatalf("backspace split grapheme: %q", got)
	}
	if got := f.Update(tea.KeyPressMsg{Code: tea.KeyEscape}); got != forms.Open || !f.Discarding {
		t.Fatal("edited value was discarded without confirmation")
	}
	f.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	if f.Discarding || f.Value("Name") != "Alex " {
		t.Fatal("keep editing lost the value")
	}
	f.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if got := f.Update(tea.KeyPressMsg{Code: 'y', Text: "y"}); got != forms.Cancel {
		t.Fatalf("confirmed discard = %v", got)
	}
}

func press(f *forms.Form, keys ...tea.KeyPressMsg) {
	for _, k := range keys {
		f.Update(k)
	}
}

var (
	keyDown  = tea.KeyPressMsg{Code: tea.KeyDown}
	keyUp    = tea.KeyPressMsg{Code: tea.KeyUp}
	keyLeft  = tea.KeyPressMsg{Code: tea.KeyLeft}
	keyRight = tea.KeyPressMsg{Code: tea.KeyRight}
	keySpace = tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
	keyClear = tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl}
)

// ↑↓ move focus over a Choice as over a text field, never changing it; ←→ and space change it.
func TestArrowsMoveFocusOverAChoiceWithoutChangingIt(t *testing.T) {
	track := forms.Field{Label: "Track", Value: "SWE", Choices: []forms.Choice{{Value: "SWE", Label: "SWE"}, {Value: "Platform", Label: "Platform"}}}
	f := forms.New("Add", forms.Field{Label: "Title"}, track, forms.Field{Label: "Closes"})
	press(f, keyDown, keyDown)
	if f.Focus != 2 || f.Value("Track") != "SWE" {
		t.Fatalf("↓↓ from Title: focus %d, Track %q", f.Focus, f.Value("Track"))
	}
	press(f, keyUp)
	if f.Focus != 1 || f.Value("Track") != "SWE" {
		t.Fatalf("↑ from Closes: focus %d, Track %q", f.Focus, f.Value("Track"))
	}
	press(f, keyRight)
	if f.Value("Track") != "Platform" {
		t.Fatalf("→ did not change the Choice: %q", f.Value("Track"))
	}
	press(f, keyLeft)
	if f.Value("Track") != "SWE" {
		t.Fatalf("← did not change the Choice back: %q", f.Value("Track"))
	}
	press(f, keySpace)
	if f.Value("Track") != "Platform" || f.Focus != 1 {
		t.Fatalf("space did not change the Choice: %q", f.Value("Track"))
	}
}

// ctrl+u empties a text field; a selected opening value is replaced by typing and cleared
// by backspace, and any other key keeps it and drops the selection.
func TestSelectedValueIsReplacedAndCtrlUClears(t *testing.T) {
	f := forms.New("Edit", forms.Field{Label: "Places", Value: "Australia"})
	f.SelectValue()
	press(f, tea.KeyPressMsg{Code: 'n', Text: "n"}, tea.KeyPressMsg{Code: 'z', Text: "z"})
	if got := f.Value("Places"); got != "nz" {
		t.Fatalf("typing over a selected value: %q", got)
	}
	f = forms.New("Edit", forms.Field{Label: "Places", Value: "Australia"})
	f.SelectValue()
	press(f, tea.KeyPressMsg{Code: tea.KeyBackspace})
	if got := f.Value("Places"); got != "" {
		t.Fatalf("backspace on a selected value: %q", got)
	}
	f = forms.New("Edit", forms.Field{Label: "Places", Value: "Australia"})
	f.SelectValue()
	press(f, keyRight, tea.KeyPressMsg{Code: '!', Text: "!"})
	if got := f.Value("Places"); got != "Australia!" {
		t.Fatalf("→ should keep the value and type after it: %q", got)
	}
	press(f, keyClear)
	if got := f.Value("Places"); got != "" {
		t.Fatalf("ctrl+u left %q", got)
	}
}
