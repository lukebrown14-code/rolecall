package ui_test

import (
	"os"
	"strings"
	"testing"
	"time"
)

// showcaseNow is the clock for tests on examples/workspace: after its last status_log
// date, with Paperbark's follow-up due and Tidewater's interview within the week.
func showcaseNow() time.Time { return time.Date(2026, 11, 19, 9, 0, 0, 0, time.UTC) }

// Every screen and view of the showcase Workspace, the README's and the demo GIF's, has
// the Applications, Profile and settings it promises, and nothing reads as empty.
func TestShowcaseEveryScreenHasContent(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../../../examples/workspace")); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		keys []string
		want []string
	}{
		{[]string{"1"}, []string{"Leads › All tracks · row 1 of 4", "Kestrel Freight", "Lyrebird Media", "Saltbush Labs", "Can't read SWE/hand-edited-example"}},
		{[]string{"1", "tab"}, []string{"Leads › Dismissed", "Numbat Security"}},
		{[]string{"1", "tab", "tab"}, []string{"Leads › Filtered", "Glasswing Insurance"}},
		{[]string{"2"}, []string{"Drafts › All tracks · row 1 of 2", "Corella Pay", "✓ ready", "Bluegum Health"}},
		{[]string{"2", "tab"}, []string{"Drafts › Deadlines", "Corella Pay", "Kestrel Freight"}},
		{[]string{"3"}, []string{"Pipeline › All · All tracks · row 1 of 6", "1 follow-up due", "Tidewater Analytics", "next 24 Nov 10:00", "Ironbark Energy", "rejected", "withdrawn"}},
		{[]string{"3", "tab"}, []string{"Pipeline › Applied", "Paperbark Software", "Mallee Robotics"}},
		{[]string{"3", "tab", "tab"}, []string{"Pipeline › Interviewing", "Tidewater Analytics"}},
		{[]string{"3", "tab", "tab", "tab"}, []string{"Pipeline › Offer", "Ironbark Energy"}},
		{[]string{"3", "tab", "tab", "tab", "tab"}, []string{"Pipeline › Closed", "Wattlebird Games", "Quandong Travel"}},
		{[]string{"4"}, []string{"1 follow-up due", "FUNNEL", "Offer", "RESPONSE RATE", "BY TRACK", "INSIGHTS"}},
		{[]string{"5"}, []string{"Casey Marlow", "WORK RIGHTS", "projects.md", "general.md"}},
		{[]string{"6"}, []string{"Melbourne, Australia"}},
		{[]string{"6", "]"}, []string{"45 days"}},
		{[]string{"6", "]", "]"}, []string{"greenhouse", "linkedin", "seek"}},
		{[]string{"6", "]", "]", "]"}, []string{"350 words"}},
		{[]string{"6", "]", "]", "]", "]"}, []string{"SWE", "Platform", "14 days"}},
	} {
		view := chipped(driveModel(t, driveStart(t, root, showcaseNow, 120, 36), tc.keys...).View().Content)
		for _, want := range tc.want {
			if !strings.Contains(view, want) {
				t.Errorf("keys %q: no %q:\n%s", tc.keys, want, view)
			}
		}
		// The empty-view messages (empty.go, stats.go) and the first-run setup prompt.
		for _, empty := range []string{"No Leads", "No dismissed Leads", "No Drafts", "No Applied", "No Interviewing", "No Offers", "No closed", "No Applications", "add your first Lead", "set up your Profile"} {
			if strings.Contains(view, empty) {
				t.Errorf("keys %q: reads as empty (%q):\n%s", tc.keys, empty, view)
			}
		}
	}
}
