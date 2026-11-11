package ui_test

import (
	"github.com/lukebrown14-code/rolecall/ui"
	"strings"
	"testing"
)

func TestMoveOptions(t *testing.T) {
	for _, tc := range []struct{ from, previous, prefix string }{
		{"Lead", "", "Draft Applied Dismissed"}, {"Draft", "", "Applied Lead Dismissed"},
		{"Applied", "", "Interviewing Rejected Withdrawn Offer"}, {"Interviewing", "", "Offer Rejected Withdrawn Applied"},
		{"Offer", "", "Withdrawn Rejected"}, {"Rejected", "", "Interviewing Applied"}, {"Withdrawn", "", "Interviewing Applied"},
		{"Dismissed", "Draft", "Draft"},
	} {
		got := ui.MoveOptions(tc.from, tc.previous)
		if !strings.HasPrefix(strings.Join(got, " "), tc.prefix) {
			t.Errorf("%s: %v", tc.from, got)
		}
		seen := map[string]bool{tc.from: true}
		for _, s := range got {
			if seen[s] {
				t.Errorf("duplicate/current %s", s)
			}
			seen[s] = true
		}
		if len(got) != 7 {
			t.Errorf("options: %v", got)
		}
	}
}
