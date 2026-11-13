package ui_test

import (
	"strings"
	"testing"

	"github.com/lukebrown14-code/rolecall/internal/store"
)

func TestNavigation(t *testing.T) {
	m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 40)
	for _, tc := range []struct {
		keys []string
		want string
	}{
		{nil, "Leads 4"},
		{[]string{"2"}, "Drafts 2"},
		{[]string{"3"}, "closed 6"},
		{[]string{"tab", "?"}, "Keys"},
	} {
		m = driveModel(t, m, tc.keys...)
		if v := chipped(m.View().Content); !strings.Contains(v, tc.want) {
			t.Fatalf("keys %q: screen lacks %q:\n%s", tc.keys, tc.want, v)
		}
	}
	driveModel(t, m, "4", "f", "p", "b", "q")
}

func TestDismissRestoreAndUndo(t *testing.T) {
	root := fixtureCopy(t, nil)
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := s.Scan()
	m := driveStart(t, root, fixtureClock, 120, 40)

	m = driveModel(t, m, "x")
	if v := chipped(m.View().Content); !strings.Contains(v, "✓ Dismissed ") {
		t.Fatalf("dismiss did not report:\n%s", v)
	}
	m = driveModel(t, m, "tab", "m", "enter") // Dismissed → restore to Lead
	if v := chipped(m.View().Content); !strings.Contains(v, "✓ Moved ") {
		t.Fatalf("restore did not report:\n%s", v)
	}
	driveModel(t, m, "u", "u", "u", "q")

	after, _ := s.Scan()
	for i, a := range after {
		if len(a.StatusLog) != len(before[i].StatusLog) {
			if a.Status != store.StatusLead || len(a.StatusLog) != len(before[i].StatusLog)+4 {
				t.Fatalf("unexpected history: %+v", a)
			}
			return
		}
	}
	t.Fatal("no status history appended")
}

func TestFilters(t *testing.T) {
	m := driveStart(t, fixtureCopy(t, nil), nil, 120, 40)
	for _, tc := range []struct {
		keys []string
		want string
	}{
		{[]string{"/zzznomatch"}, "No Applications match"},
		{[]string{"enter", "2"}, "Draft"},
		{[]string{"r", "esc"}, "Drafts 2"},
	} {
		m = driveModel(t, m, tc.keys...)
		if v := chipped(m.View().Content); !strings.Contains(v, tc.want) {
			t.Fatalf("keys %q: screen lacks %q:\n%s", tc.keys, tc.want, v)
		}
	}
}
