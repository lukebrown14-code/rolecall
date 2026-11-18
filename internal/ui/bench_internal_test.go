package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/internal/store"
	"github.com/lukebrown14-code/rolecall/internal/ui/watch"
)

// Benchmarks for the costs that grow with the Workspace: a frame, a keypress, a watcher
// batch and a bulk move. They run only with -bench, on a synthetic Workspace of benchSizes
// Applications generated in a temp folder, so `go test ./...` is not slowed:
//
//	go test -run '^$' -bench . ./internal/ui
//
// Each size is generated once per benchmark function; the fixture Workspace's own
// Applications ride along, so malformed and off-list rows are in every list.

var benchSizes = []int{100, 1000, 3000}

// benchStatuses is the Status mix the generator cycles: Sweeps leave many Leads and
// Dismissed folders, and a few get further.
var benchStatuses = [][]string{
	{store.StatusLead}, {store.StatusLead}, {store.StatusLead}, {store.StatusLead, store.StatusDismissed},
	{store.StatusLead, store.StatusDraft}, {store.StatusLead, store.StatusDraft, store.StatusApplied},
	{store.StatusLead, store.StatusApplied, store.StatusInterviewing}, {store.StatusLead},
	{store.StatusLead, store.StatusDismissed}, {store.StatusLead, store.StatusApplied, store.StatusRejected},
}

// benchTracks puts one Track off workspace.json's list, so the Track filter has an
// off-list option, as an old or renamed Track gives it.
var benchTracks = []string{"SWE", "Platform", "Data"}

// benchWorkspace is a copy of the fixture Workspace with n generated Applications added.
func benchWorkspace(b *testing.B, n int) string {
	b.Helper()
	root := b.TempDir()
	if err := os.CopyFS(root, os.DirFS("../../fixtures/workspace")); err != nil {
		b.Fatal(err)
	}
	jd := strings.Repeat("We build software. You will write Go and run services.\n", 40)
	fit := "Verdict: good\n\n## Gaps\n- Kubernetes — none\n- Go — some\n" + strings.Repeat("Reasoning line.\n", 40)
	for i := range n {
		dir := filepath.Join(root, "jobs", "SWE", fmt.Sprintf("gen-%05d", i))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			b.Fatal(err)
		}
		day := 1 + i%28
		var meta strings.Builder
		fmt.Fprintf(&meta, "schema: 1\ntitle: Engineer %d\ncompany: Company %d\ntrack: %s\nposted: 2026-09-%02d\nsource: greenhouse\n",
			i, i%97, benchTracks[i%len(benchTracks)], day)
		if i%5 == 0 {
			fmt.Fprintf(&meta, "closes: 2026-10-%02d\n", day)
		}
		meta.WriteString("status_log:\n")
		for j, s := range benchStatuses[i%len(benchStatuses)] {
			fmt.Fprintf(&meta, "  - {status: %s, at: 2026-09-%02dT%02d:00:00Z}\n", s, day, j+1)
		}
		files := map[string]string{"meta.yml": meta.String(), "jd.md": jd}
		if i%2 == 0 {
			files["fit.md"] = fit
		}
		for name, body := range files {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
				b.Fatal(err)
			}
		}
	}
	return root
}

// benchModel opens root at a fixed clock and a roomy terminal.
func benchModel(b *testing.B, root string) model {
	b.Helper()
	s, err := store.Open(root)
	if err != nil {
		b.Fatal(err)
	}
	m, err := New(s, func() time.Time { return fixtureNowInternal })
	if err != nil {
		b.Fatal(err)
	}
	mm := m.(model)
	mm.width, mm.height = 160, 50
	return mm
}

// benchScreens are the screens a keypress is measured on: Leads, Pipeline and Stats. A
// frame alone is measured on Pipeline, the screen with the most views and counts.
var benchScreens = []struct {
	name   string
	screen int
	view   bool
}{{"leads", 0, false}, {"pipeline", 2, true}, {"stats", 3, false}}

// BenchmarkFrame is a j keypress and the frame after it on each screen, a frame alone on
// Pipeline, and a search keystroke and its frame.
func BenchmarkFrame(b *testing.B) {
	for _, n := range benchSizes {
		root := benchWorkspace(b, n)
		for _, sc := range benchScreens {
			m := benchModel(b, root)
			m.screen = sc.screen
			if sc.view {
				b.Run(fmt.Sprintf("%s/%d/view", sc.name, n), func(b *testing.B) {
					for b.Loop() {
						_ = m.View()
					}
				})
			}
			b.Run(fmt.Sprintf("%s/%d/key_j+view", sc.name, n), func(b *testing.B) {
				mm := m
				for b.Loop() {
					next, _ := mm.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
					mm = next.(model)
					_ = mm.View()
				}
			})
		}
		m := benchModel(b, root)
		b.Run(fmt.Sprintf("search/%d/type+view", n), func(b *testing.B) {
			next, _ := m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
			mm := next.(model)
			for b.Loop() {
				next, _ := mm.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
				mm = next.(model)
				_ = mm.View()
				next, _ = mm.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
				mm = next.(model)
			}
		})
	}
}

// BenchmarkRefresh is a watcher batch naming one Application, one naming every
// Application and Track (an overflow's rescan, a git checkout), and r, a full scan.
func BenchmarkRefresh(b *testing.B) {
	for _, n := range benchSizes {
		root := benchWorkspace(b, n)
		m := benchModel(b, root)
		all := []string{filepath.Join(root, "jobs", "SWE")}
		for _, a := range m.apps {
			all = append(all, a.Folder)
		}
		b.Run(fmt.Sprintf("one/%d", n), func(b *testing.B) {
			for b.Loop() {
				next, _ := m.Update(watch.Changed{Folders: []string{m.apps[0].Folder}})
				_ = next
			}
		})
		b.Run(fmt.Sprintf("all/%d", n), func(b *testing.B) {
			for b.Loop() {
				next, _ := m.Update(watch.Changed{Folders: all})
				_ = next
			}
		})
		b.Run(fmt.Sprintf("r_full_scan/%d", n), func(b *testing.B) { // the yardstick for all
			for b.Loop() {
				next, _ := m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
				_ = next
			}
		})
	}
}

// benchBulk is how many Leads the bulk benchmark dismisses: a Sweep's worth.
const benchBulk = 100

// BenchmarkBulkDismiss is y over benchBulk marked Leads in the bulk dismiss pop-up, and the
// u that puts them back, each timed alone; the other half and putting the meta.yml files
// back run untimed between rounds.
func BenchmarkBulkDismiss(b *testing.B) {
	for _, n := range benchSizes {
		root := benchWorkspace(b, n)
		m := benchModel(b, root)
		var leads []store.Application
		saved := map[string][]byte{}
		for _, a := range m.apps {
			if a.Status == store.StatusLead && a.Malformed == nil && strings.Contains(a.Folder, "gen-") && len(leads) < benchBulk {
				leads = append(leads, a)
				meta, err := os.ReadFile(filepath.Join(a.Folder, "meta.yml"))
				if err != nil {
					b.Fatal(err)
				}
				saved[a.Folder] = meta
			}
		}
		dismiss := func(mm *model) {
			if mm.moveMany(leads, store.StatusDismissed); mm.err != nil || len(mm.undo) != 1 {
				b.Fatalf("bulk dismiss: err %v, %d undo entries", mm.err, len(mm.undo))
			}
		}
		undo := func(mm *model) {
			if mm.undoLast(); mm.err != nil || len(mm.undo) != 0 {
				b.Fatalf("bulk undo: err %v, %d undo entries", mm.err, len(mm.undo))
			}
		}
		restore := func() {
			for folder, meta := range saved { // the log back to its one Lead entry
				if err := os.WriteFile(filepath.Join(folder, "meta.yml"), meta, 0o600); err != nil {
					b.Fatal(err)
				}
			}
		}
		b.Run(fmt.Sprintf("dismiss/%d", n), func(b *testing.B) {
			mm := m
			for b.Loop() {
				dismiss(&mm)
				b.StopTimer()
				undo(&mm)
				restore()
				b.StartTimer()
			}
		})
		b.Run(fmt.Sprintf("undo/%d", n), func(b *testing.B) {
			mm := m
			for b.Loop() {
				b.StopTimer()
				dismiss(&mm)
				b.StartTimer()
				undo(&mm)
				b.StopTimer()
				restore()
				b.StartTimer()
			}
		})
	}
}
