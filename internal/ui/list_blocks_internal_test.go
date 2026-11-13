package ui

import (
	"testing"
	"time"

	"github.com/lukebrown14-code/rolecall/internal/store"
)

// The list lays out every block before it knows which ones are visible, but only the
// visible rows are rendered. listBlocks therefore reports heights, not lines.
func TestListBlocksComputeHeightsWithoutRendering(t *testing.T) {
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	m := model{now: func() time.Time { return now }, followUpDays: 14}
	rows := make([]store.Application, 40)
	for i := range rows {
		rows[i] = store.Application{Title: "Application", Company: "Acme", Track: "SWE", Status: store.StatusApplied}
	}

	blocks, cur := m.listBlocks(rows, 20)
	if len(blocks) == 0 {
		t.Fatal("no blocks")
	}
	for _, b := range blocks {
		if b.height < 1 {
			t.Fatalf("block %d has height %d", b.idx, b.height)
		}
	}
	if cur != 0 {
		t.Fatalf("cursor block %d, want 0", cur)
	}
}

// The list lays out every block before it knows which are visible, but only the blocks in
// the scroll window are rendered: a seam counts renderBlock calls and bounds them by the
// window height, and by the row count so not every row can be drawn.
func TestListRendersOnlyVisibleBlocks(t *testing.T) {
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	m := model{now: func() time.Time { return now }, followUpDays: 14}
	rows := make([]store.Application, 40)
	for i := range rows {
		rows[i] = store.Application{Title: "Application", Company: "Acme", Track: "SWE", Status: store.StatusApplied}
	}

	calls := 0
	real := renderBlock
	renderBlock = func(m model, p pal, b block, rows []store.Application, show slotSet, w int) []string {
		calls++
		return real(m, p, b, rows, show, w)
	}
	t.Cleanup(func() { renderBlock = real })

	const h = 12
	_ = m.listBody(m.pal(), rows, 80, h)
	if calls == 0 {
		t.Fatal("no blocks were rendered")
	}
	if calls > h {
		t.Fatalf("rendered %d blocks into a %d-line window", calls, h)
	}
	if calls >= len(rows) {
		t.Fatalf("rendered every row: %d blocks for %d rows", calls, len(rows))
	}
}
