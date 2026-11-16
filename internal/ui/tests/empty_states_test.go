package ui_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// emptyJobs is a Workspace copy with every Application removed.
func emptyJobs(t *testing.T) string {
	return fixtureCopy(t, func(root string) {
		jobs := filepath.Join(root, "jobs")
		if err := os.RemoveAll(jobs); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(jobs, 0o755); err != nil {
			t.Fatal(err)
		}
	})
}

// A search with no matches names it and offers esc, which clears it.
func TestEmptySearchHintsEscClear(t *testing.T) {
	d := newDriver(t, fixtureCopy(t, nil), fixtureClock, 120, 36)
	d.Type("/zzznomatch")
	d.key('\r')
	if v := d.plain(); !strings.Contains(v, `No Applications match "zzznomatch" · esc clear search`) {
		t.Fatalf("no search hint:\n%s", v)
	}
	d.key(0x1b)
	if v := d.plain(); strings.Contains(v, "zzznomatch") {
		t.Fatalf("esc did not clear the search:\n%s", v)
	}
}

// A Workspace with no Applications says what to press on Leads, and the key works.
func TestEmptyWorkspaceHintsAddALead(t *testing.T) {
	d := newDriver(t, emptyJobs(t), fixtureClock, 120, 36)
	d.Type("1")
	if v := d.plain(); !strings.Contains(v, "a  add your first Lead") {
		t.Fatalf("no add hint:\n%s", v)
	}
	d.Type("a")
	if v := d.plain(); strings.Contains(v, "a  add your first Lead") {
		t.Fatalf("a did not open the add form:\n%s", v)
	}
}

// An empty Status view hints the status filter and add, and s moves on.
func TestEmptyStatusHintsFilterAndAdd(t *testing.T) {
	root := fixtureCopy(t, func(root string) {
		matches, _ := filepath.Glob(filepath.Join(root, "jobs", "*", "*", "meta.yml"))
		for _, m := range matches {
			if b, _ := os.ReadFile(m); strings.Contains(string(b), "Offer") {
				_ = os.RemoveAll(filepath.Dir(m))
			}
		}
	})
	d := newDriver(t, root, fixtureClock, 120, 36)
	d.Type("3")
	for i := 0; i < 3; i++ {
		d.Type("s")
	}
	if v := d.plain(); !strings.Contains(v, "No Offers yet · s change view · a add a Lead") {
		t.Fatalf("no status hint:\n%s", v)
	}
	d.Type("s")
	if v := d.plain(); strings.Contains(v, "No Offers yet") {
		t.Fatalf("s did not change the filter:\n%s", v)
	}
}

// A Track with nothing in this view hints t. t never lands on an empty Track, so the
// view is reached by carrying a Track chosen elsewhere.
func TestEmptyTrackHintsT(t *testing.T) {
	d := newDriver(t, fixtureCopy(t, nil), fixtureClock, 120, 36)
	d.Type("1")
	for i := 0; i < 6 && !strings.Contains(d.plain(), "[Platform"); i++ {
		d.Type("t")
	}
	d.Type("2")
	v := d.plain()
	if !strings.Contains(v, "No Drafts in Platform · t change Track") {
		t.Fatalf("no track hint:\n%s", v)
	}
	d.Type("t")
	if v := d.plain(); strings.Contains(v, "No Drafts in Platform") {
		t.Fatalf("t did not change the Track:\n%s", v)
	}
}

// At 60x16 the hint stays inside the width, wrapped on word boundaries.
func TestEmptyHintFitsNarrow(t *testing.T) {
	d := newDriver(t, fixtureCopy(t, nil), fixtureClock, 60, 16)
	d.Type("/zzznomatchatallreally")
	d.key('\r')
	v := d.plain()
	for _, l := range strings.Split(v, "\n") {
		if ansi.StringWidth(l) > 60 {
			t.Fatalf("line too wide: %q", l)
		}
	}
	for _, w := range []string{"No Applications match", "esc clear search"} {
		if !strings.Contains(v, w) {
			t.Fatalf("missing %q:\n%s", w, v)
		}
	}
}
