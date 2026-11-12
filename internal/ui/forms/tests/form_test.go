package forms_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/internal/ui/forms"
)

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
