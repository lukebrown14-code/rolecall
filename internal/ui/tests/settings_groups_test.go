package ui_test

import (
	"strings"
	"testing"

	"github.com/lukebrown14-code/rolecall/internal/ui/settings"
)

// The Settings groups are one ordered list, and the screen drives every one of them: the
// Group* indices line up with the names, and stepping the group list reaches each group
// with its own rows. The screen dispatches a group's keys from the one handler table, so a
// group with no handler would show nothing here.
func TestEverySettingsGroupShowsItsRows(t *testing.T) {
	if settings.Groups[settings.GroupMachine] != settings.GroupMachineName {
		t.Fatalf("GroupMachine = %d, which is %q, not %q", settings.GroupMachine, settings.Groups[settings.GroupMachine], settings.GroupMachineName)
	}
	if settings.GroupSearch != 0 || settings.GroupDocuments != 1 || settings.GroupTracking != 2 || settings.GroupMachine != 3 {
		t.Fatalf("group indices are not in display order: %d %d %d %d", settings.GroupSearch, settings.GroupDocuments, settings.GroupTracking, settings.GroupMachine)
	}

	// A row only this group shows, in display order.
	want := []string{"Places", "Cover letter maximum", "Follow-up due after", "Editor"}
	m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 40)
	m = driveModel(t, m, "6")
	for i, name := range settings.Groups {
		if view := chipped(m.View().Content); !strings.Contains(view, want[i]) {
			t.Fatalf("group %q (%d) does not show %q:\n%s", name, i, want[i], view)
		}
		m = driveModel(t, m, "]") // ] steps to the next section
	}
}
