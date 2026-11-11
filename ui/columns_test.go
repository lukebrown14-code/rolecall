package ui_test

import (
	"github.com/lukebrown14-code/rolecall/store"
	"github.com/lukebrown14-code/rolecall/ui"
	"strings"
	"testing"
	"time"
)

func TestApplicationColumns(t *testing.T) {
	a := store.Application{Title: "Engineer", Company: "Acme", Track: "tech", Status: "Applied", Documents: "Ready", AppliedDate: &store.StatusEntry{At: "2026-01-01", Approx: true}, LastChange: &store.StatusEntry{At: "2026-01-01"}}
	got := ui.ApplicationColumns(a, "Applied", time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), 14)
	for _, want := range []string{"~2026-01-01", "●", "⚑", "—"} {
		if !strings.Contains(strings.Join(got, " "), want) {
			t.Fatalf("missing %s: %v", want, got)
		}
	}
}

func TestTableSorting(t *testing.T) {
	for _, table := range []string{"Leads", "Draft", "Dismissed", "Applied", "Interviewing", "Offer", "Closed"} {
		t.Run(table, func(t *testing.T) {
			rows := []store.Application{{Folder: "missing"}, {Folder: "old", Closes: "2026-01-01", AppliedDate: &store.StatusEntry{At: "2026-01-01"}, LastChange: &store.StatusEntry{At: "2026-01-01"}}, {Folder: "new", Closes: "2026-02-01", AppliedDate: &store.StatusEntry{At: "2026-02-01"}, LastChange: &store.StatusEntry{At: "2026-02-01"}}}
			ui.SortApplications(rows, table)
			want := "new"
			if table == "Leads" || table == "Draft" {
				want = "old"
			}
			if rows[0].Folder != want || rows[2].Folder != "missing" {
				t.Fatalf("order: %v", rows)
			}
		})
	}
}

func TestTitleShrinksBeforeCompany(t *testing.T) {
	cells := [][]string{{"Long role title", "Company", "Track"}}
	for _, tc := range []struct {
		width int
		want  string
	}{{25, "Long r… | Company | Track"}, {18, "… | Compa… | Track"}} {
		got := ui.RenderColumns(cells, tc.width)
		if got[0] != tc.want {
			t.Errorf("width %d: %q want %q", tc.width, got[0], tc.want)
		}
	}
}

func TestColumnsAcrossTables(t *testing.T) {
	now := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	for _, table := range []string{"Leads", "Draft", "Dismissed", "Applied", "Interviewing", "Offer", "Closed"} {
		t.Run(table, func(t *testing.T) {
			a := store.Application{Title: "Role", Company: "Firm", Track: "tech", Status: table, Closes: "2026-01-01", Documents: "Written", AppliedDate: &store.StatusEntry{At: "2026-01-01", Approx: true}, LastChange: &store.StatusEntry{At: "2026-01-01", Approx: true}}
			got := strings.Join(ui.ApplicationColumns(a, table, now, 14), "|")
			want := "Role|Firm|tech|~2026-01-01|—|○|"
			switch table {
			case "Leads":
				want = "Role|Firm|tech|past|—"
			case "Draft":
				want = "Role|Firm|tech|past|—|○"
			case "Dismissed":
				want = "Role|Firm|tech|~2026-01-01"
			case "Applied", "Interviewing":
				want += "⚑"
			}
			if got != want {
				t.Fatalf("got %q want %q", got, want)
			}
			if table == "Applied" || table == "Interviewing" {
				if got := ui.ApplicationColumns(a, table, now, 40); got[6] != "" {
					t.Fatalf("configured period ignored: %v", got)
				}
			}
		})
	}
}
