package ui_test

import (
	"path/filepath"
	"testing"
)

// An invalid workspace.json does not lock the Owner out: the app starts with no Track
// filter, says what is wrong, and Settings shows the path and parse error with e.
func TestInvalidWorkspaceFileStillStarts(t *testing.T) {
	for name, bad := range map[string]string{"syntax": `{"tracks": [`, "type": `{"tracks": "SWE"}`} {
		t.Run(name, func(t *testing.T) {
			h := newSettingsHarness(t, 120, bad, func(string) {})
			requireAll(t, squash(h.tm.plain()), "invalid Workspace settings", "6 Settings shows why")
			h.tm.Type("6")
			requireAll(t, h.tm.plain(), filepath.Join(h.root, "workspace.json"), "Press e to fix it")
		})
	}
}
