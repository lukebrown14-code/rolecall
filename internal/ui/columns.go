package ui

import (
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/internal/config"
	"github.com/lukebrown14-code/rolecall/internal/schedule"
	"github.com/lukebrown14-code/rolecall/internal/store"
)

// followUpFlag marks an Applied or Interviewing row whose last change is follow_up_days old or older.
const followUpFlag = glyphFollowUp

// entryDate is a Status entry's day as the rows show it, "2026-11-04", in loc (the
// Owner's zone), with SCHEMA's ~ when approximate.
func entryDate(e *store.StatusEntry, loc *time.Location) string {
	if e == nil {
		return ""
	}
	d, _, _ := strings.Cut(e.At, "T")
	if t, ok := store.LocalDay(e.At, loc); ok {
		d = t.Format(time.DateOnly)
	}
	if e.Approx {
		d = "~" + d
	}
	return d
}

// ApplicationColumns presents the dates and state markers for an Application.
func ApplicationColumns(a store.Application, table string, now time.Time, followUpDays int) []string {
	if a.Malformed != nil {
		return []string{"⚠ " + filepath.Base(a.Folder)}
	}
	out := []string{a.Title, a.CompanyLabel(), a.Track}
	fit := a.Fit // blank when not assessed: no placeholder mark
	docs := ""
	switch strings.ToLower(a.Documents) {
	case "ready":
		docs = glyphReady
	case "written":
		docs = glyphMissing
	}
	if table == tableFiltered {
		return append(out, entryDate(schedule.LastChange(a), now.Location()), filterReasonLabel(a.FilterReason))
	}
	if table == tableDismissed {
		return append(out, entryDate(schedule.LastChange(a), now.Location()))
	}
	if beforeApplied(table) {
		closes := a.Closes
		if schedule.ClosesPassed(a, now) {
			closes = "past"
		}
		out = append(out, closes, fit)
		if table == tableDraft {
			out = append(out, docs)
		}
		return out
	}
	flag := ""
	if schedule.FollowUpDue(a, now, followUpDays) {
		flag = followUpFlag
	}
	return append(out, entryDate(schedule.AppliedDate(a), now.Location()), fit, docs, flag)
}

// SortApplications orders a view's rows as the list does, with no Search rules and no
// Follow-up period: Leads and Drafts by close date, sent Applications by Applied date.
func SortApplications(rows []store.Application, table string) {
	rowOrder{}.sort(rows, table)
}

// rowOrder is what ordering a view needs beyond its rows: the Search rules, for a Lead's
// posting-age expiry; today; and the follow-up period, for putting Follow-ups due first.
type rowOrder struct {
	rules        store.SearchRules
	now          time.Time
	followUpDays int
}

// order is the model's rowOrder.
func (m model) order() rowOrder {
	return rowOrder{rules: m.rules, now: m.now(), followUpDays: m.followUpDays}
}

// The Leads sort, as the Leads section header names it: by deadline (the default) or
// newest added first (o).
const (
	sortByDeadline = "by deadline"
	sortNewestText = "newest first"
)

// fitOrder is the Fits best first; fitRank puts a Fit not yet assessed after them all.
var fitOrder = []string{store.FitStrong, store.FitGood, store.FitMarginal, store.FitPoor}

func fitRank(fit string) int {
	if i := slices.Index(fitOrder, fit); i >= 0 {
		return i
	}
	return len(fitOrder)
}

// sort orders rows for table. Leads and Drafts go by deadline (the close date, or the day
// a Lead's ad passes max_posting_age_days, whichever is sooner), soonest first and no
// deadline last; then by Fit, best first; then by posting date, newest first. Applied and
// Interviewing put Follow-ups due first, most overdue first, then the rest by Applied
// date, newest first. Dismissed, Filtered and Closed go by their last change, newest first.
func (o rowOrder) sort(rows []store.Application, table string) {
	switch {
	case beforeApplied(table):
		sortStableBy(rows, o.deadlineKey, deadlineKeyLess)
		return
	case table == tableDismissed || table == tableFiltered:
		sortByDate(rows, func(a store.Application) string { return entryAt(schedule.LastChange(a)) })
		return
	}
	sortByDate(rows, func(a store.Application) string { return entryAt(schedule.AppliedDate(a)) })
	if (table != tableApplied && table != tableInterviewing) || o.followUpDays <= 0 {
		return
	}
	overdue := func(a store.Application) int {
		past, due := schedule.FollowUpDaysPast(a, o.now, o.followUpDays)
		if !due {
			return -1
		}
		return past
	}
	sortStableBy(rows, overdue, func(a, b int) bool { return a > b })
}

// sortStableBy sorts rows stably by less over a key worked out once per row rather than
// twice per comparison: the keys parse dates, and a list sorts often.
func sortStableBy[K any](rows []store.Application, key func(store.Application) K, less func(a, b K) bool) {
	keys := make([]K, len(rows))
	order := make([]int, len(rows))
	for i, a := range rows {
		keys[i], order[i] = key(a), i
	}
	sort.SliceStable(order, func(i, j int) bool { return less(keys[order[i]], keys[order[j]]) })
	sorted := make([]store.Application, len(rows))
	for i, at := range order {
		sorted[i] = rows[at]
	}
	copy(rows, sorted)
}

// datedRow is a row's sort date and folder, for sortByDate.
type datedRow struct{ date, folder string }

// sortByDate orders rows by a date, newest first, with no date last and folder breaking ties.
func sortByDate(rows []store.Application, date func(store.Application) string) {
	sortStableBy(rows, func(a store.Application) datedRow { return datedRow{date(a), a.Folder} }, func(a, b datedRow) bool {
		if a.date == b.date {
			return a.folder < b.folder
		}
		return b.date == "" || (a.date != "" && a.date > b.date)
	})
}

// deadlineKey is what the Leads and Drafts order compares, worked out once per row.
type deadlineKey struct {
	deadline       time.Time
	ok             bool
	fit            int
	posted, folder string
}

func (o rowOrder) deadlineKey(a store.Application) deadlineKey {
	d, ok := schedule.Deadline(a, o.rules)
	return deadlineKey{deadline: d, ok: ok, fit: fitRank(a.Fit), posted: a.Posted, folder: a.Folder}
}

// deadlineKeyLess is the Leads and Drafts order: deadline, then Fit, then newest posted.
func deadlineKeyLess(a, b deadlineKey) bool {
	switch {
	case a.ok != b.ok:
		return a.ok
	case a.ok && !a.deadline.Equal(b.deadline):
		return a.deadline.Before(b.deadline)
	}
	if a.fit != b.fit {
		return a.fit < b.fit
	}
	if a.posted != b.posted {
		return b.posted == "" || (a.posted != "" && a.posted > b.posted)
	}
	return a.folder < b.folder
}

// WithCompactRows starts with one-line list rows, as compact_rows in config.toml does.
func WithCompactRows(on bool) Option { return func(m *model) { m.compactRows = on } }

// configCompactRows is the config.toml key z saves.
const configCompactRows = "compact_rows"

// toggleCompactRows switches one-line rows, keeping the cursor on its row, and saves the
// choice to config.toml so the next start opens the same way.
func (m *model) toggleCompactRows() tea.Cmd {
	folder := m.cursorFolder()
	m.compactRows = !m.compactRows
	m.keepCursor(folder)
	m.toast = "Rows on two lines"
	if m.compactRows {
		m.toast = "Compact rows, one line each"
	}
	if m.configHome != "" {
		if err := config.Patch(config.Path(m.configHome), configCompactRows, m.compactRows); err != nil {
			m.err = err
		}
	}
	return nil
}
