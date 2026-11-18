package ui

import (
	"strings"
	"time"

	"github.com/lukebrown14-code/rolecall/internal/store"
)

// rowCache keeps the list rows derived from one model state, so a frame and the Update
// before it filter and sort each view once rather than once per widget that asks (the
// list, the tab and filter counts, the breadcrumb, the cursor clamp). Like mdCache it is
// shared by pointer across model copies, and every entry is keyed on what it was derived
// from, so a copy with other inputs never reads another's rows: see rowInputs and
// viewInputs. A model built without one (a test's literal) derives every time.
type rowCache struct {
	data  rowInputs
	view  viewInputs
	built bool

	tracks   []string // filterTracks
	tracksOK bool
	lower    []string                 // each Application's searchText, lower-cased, by index in apps
	gaps     map[int][]store.GapCount // Stats' gaps from fit.md, by Track filter (statsCommonGaps)
	rows     map[rowKey][]store.Application
	totals   map[rowKey]int

	builds int // views derived, for tests
}

// rowInputs is what the Workspace side of the rows is derived from. gen bumps on every
// rescan (dropBodies) and rulesGen on every Search rules load; the slice's first element
// and length catch a model whose apps were swapped without either.
type rowInputs struct {
	gen, rulesGen int
	first         *store.Application
	n             int
	tracks        string // workspace.json's Tracks, joined
}

// viewInputs is what the filtering and sorting read besides the Workspace: the text
// filter, the Leads sort, the follow-up period and today (every date rule is whole-day,
// in now's zone).
type viewInputs struct {
	text         string
	leadsNewest  bool
	followUpDays int
	today        time.Time
	zone         *time.Location
}

// rowKey is one view's rows: a screen's table under a Track filter. Deadlines and the
// unfiltered totals use their own forms of it.
type rowKey struct {
	screen int
	table  string
	track  int
	total  bool // the count ignoring the Track and text filters (totalCount)
	all    bool // Deadlines without the filters
}

// maxCachedViews bounds the views kept for one set of inputs: each typed search
// character is new inputs, so this only guards against a pathological caller.
const maxCachedViews = 512

func (m model) rowInputs() rowInputs {
	in := rowInputs{gen: m.gen, rulesGen: m.rulesGen, n: len(m.apps), tracks: strings.Join(m.tracks, "\x00")}
	if len(m.apps) > 0 {
		in.first = &m.apps[0]
	}
	return in
}

func (m model) viewInputs() viewInputs {
	now := m.now()
	return viewInputs{text: m.text, leadsNewest: m.leadsNewest, followUpDays: m.followUpDays, today: store.Today(now), zone: now.Location()}
}

// cache is the row cache brought up to date with m's inputs, or nil when m has none.
// A change of Workspace drops everything; a change of filter or day drops the views.
func (m model) cache() *rowCache {
	c := m.rowMemo
	if c == nil {
		return nil
	}
	data, view := m.rowInputs(), m.viewInputs()
	if !c.built || c.data != data {
		*c = rowCache{data: data, view: view, built: true, builds: c.builds}
	}
	if c.view != view || len(c.rows)+len(c.totals) > maxCachedViews {
		c.view, c.rows, c.totals = view, nil, nil
	}
	return c
}

// memoRows is the rows for k, derived by derive once per state.
func (m model) memoRows(k rowKey, derive func() []store.Application) []store.Application {
	c := m.cache()
	if c == nil {
		return derive()
	}
	if rows, ok := c.rows[k]; ok {
		return rows
	}
	rows := derive()
	rows = rows[:len(rows):len(rows)] // a caller's append copies rather than writing into the cache
	if c.rows == nil {
		c.rows = map[rowKey][]store.Application{}
	}
	c.rows[k] = rows
	c.builds++
	return rows
}

// memoTotal is the count for k, counted by count once per state.
func (m model) memoTotal(k rowKey, count func() int) int {
	c := m.cache()
	if c == nil {
		return count()
	}
	if n, ok := c.totals[k]; ok {
		return n
	}
	n := count()
	if c.totals == nil {
		c.totals = map[rowKey]int{}
	}
	c.totals[k] = n
	return n
}

// matcher is the Track and text filters as one test of the i-th Application, with the
// Track resolved and the needle lower-cased once rather than per row.
func (m model) matcher() func(i int, a store.Application) bool {
	track, on := m.selectedTrack()
	needle := strings.ToLower(m.text)
	var lower []string
	if c := m.cache(); c != nil {
		if c.lower == nil {
			c.lower = make([]string, len(m.apps))
			for i, a := range m.apps {
				c.lower[i] = strings.ToLower(m.searchText(a))
			}
		}
		lower = c.lower
	}
	return func(i int, a store.Application) bool {
		if on && a.Track != track {
			return false
		}
		if needle == "" {
			return true
		}
		if lower != nil {
			return strings.Contains(lower[i], needle)
		}
		return strings.Contains(strings.ToLower(m.searchText(a)), needle)
	}
}
