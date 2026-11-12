package ui_test

import (
	"testing"

	"github.com/lukebrown14-code/rolecall/internal/ui"
)

func TestStatusLocation(t *testing.T) {
	for _, c := range []struct{ s, screen, table string }{{"Lead", "Leads", "Leads"}, {"Dismissed", "Leads", "Dismissed"}, {"Draft", "Drafts", "Drafts"}, {"Applied", "Applied", "Applied"}, {"Interviewing", "Applied", "Interviewing"}, {"Offer", "Applied", "Offer"}, {"Rejected", "Applied", "Closed"}, {"Withdrawn", "Applied", "Closed"}} {
		s, tbl := ui.StatusLocation(c.s)
		if s != c.screen || tbl != c.table {
			t.Errorf("%s: %s/%s", c.s, s, tbl)
		}
	}
}
