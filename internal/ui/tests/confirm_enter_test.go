package ui_test

import (
	"strings"
	"testing"
)

// A yes-or-no confirmation names its keys plainly, "y yes · n / esc no", and enter, which
// does not answer it, says to press y instead of doing nothing; y still confirms after.
func TestConfirmationsAnswerEnterWithAHint(t *testing.T) {
	for _, tc := range []struct {
		name, title string
		keys        []string
	}{
		{"bulk dismiss", "Dismiss 2 Applications?", []string{"1", "*", "*", "x"}},
		{"docs not Ready", "Docs not Ready", []string{"1", "m", "a"}},
	} {
		root := fixtureCopy(t, nil)
		m := driveModel(t, driveStart(t, root, fixtureClock, 120, 36), tc.keys...)
		view := chipped(m.View().Content)
		if !strings.Contains(view, tc.title) || !strings.Contains(view, "y yes · n / esc no") {
			t.Fatalf("%s: keys not worded y yes · n / esc no:\n%s", tc.name, view)
		}
		if strings.Contains(view, "Press y to confirm") {
			t.Fatalf("%s: the hint shows before enter:\n%s", tc.name, view)
		}
		m = driveModel(t, m, "enter")
		view = chipped(m.View().Content)
		if !strings.Contains(view, tc.title) || !strings.Contains(view, "Press y to confirm, or n to cancel.") {
			t.Fatalf("%s: enter did nothing visible:\n%s", tc.name, view)
		}
		m = driveModel(t, m, "y")
		if view := chipped(m.View().Content); strings.Contains(view, tc.title) {
			t.Fatalf("%s: y did not confirm after enter:\n%s", tc.name, view)
		}
	}
}
