package ui

import (
	"testing"

	"github.com/lukebrown14-code/rolecall/internal/ui/settings"
)

// Every Settings group has one handler, the handlers are in display order, and the group
// indices line up with Groups — so reordering Groups cannot silently move a group's keys
// or help, and a new group cannot be added without a handler.
func TestSettingsHandlersCoverEveryGroup(t *testing.T) {
	if len(settingsHandlers) != len(settings.Groups) {
		t.Fatalf("handlers = %d, groups = %d", len(settingsHandlers), len(settings.Groups))
	}
	for i, h := range settingsHandlers {
		if h.group != i {
			t.Fatalf("handler %d is for group %d (%s)", i, h.group, settings.Groups[h.group])
		}
		if h.help == nil {
			t.Fatalf("group %s has no help", settings.Groups[h.group])
		}
	}
	if settings.Groups[settings.GroupMachine] != settings.GroupMachineName {
		t.Fatalf("GroupMachine = %d, which is %q, not the machine group", settings.GroupMachine, settings.Groups[settings.GroupMachine])
	}
}
