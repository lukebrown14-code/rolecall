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
