package ui

import (
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/internal/config"
	"github.com/lukebrown14-code/rolecall/internal/store"
)

// followUpFlag marks an Applied or Interviewing row whose last change is older than follow_up_days.
const followUpFlag = glyphFollowUp

func entryDate(e *store.StatusEntry) string {
	if e == nil {
		return ""
	}
	d := strings.Split(e.At, "T")[0]
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
		return append(out, entryDate(lastChange(a)), filterReasonLabel(a.FilterReason))
	}
	if table == tableDismissed {
		return append(out, entryDate(lastChange(a)))
	}
	if beforeApplied(table) {
		closes := a.Closes
		if t, ok := parseDay(closes); ok && t.Before(day(now)) {
			closes = "past"
		}
		out = append(out, closes, fit)
		if table == tableDraft {
			out = append(out, docs)
		}
		return out
	}
	flag := ""
	if followUpDue(a, now, followUpDays) {
		flag = followUpFlag
	}
	return append(out, entryDate(appliedDate(a)), fit, docs, flag)
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
		sort.SliceStable(rows, func(i, j int) bool { return o.deadlineLess(rows[i], rows[j]) })
		return
	case table == tableDismissed || table == tableFiltered:
		sortByDate(rows, func(a store.Application) string { return entryAt(lastChange(a)) })
		return
	}
	sortByDate(rows, func(a store.Application) string { return entryAt(appliedDate(a)) })
	if (table != tableApplied && table != tableInterviewing) || o.followUpDays <= 0 {
		return
	}
	overdue := func(a store.Application) int {
		past, due := followUpDaysPast(a, o.now, o.followUpDays)
		if !due {
			return -1
		}
		return past
	}
	sort.SliceStable(rows, func(i, j int) bool { return overdue(rows[i]) > overdue(rows[j]) })
}

// sortByDate orders rows by a date, newest first, with no date last and folder breaking ties.
func sortByDate(rows []store.Application, date func(store.Application) string) {
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := date(rows[i]), date(rows[j])
		if a == b {
			return rows[i].Folder < rows[j].Folder
		}
		return b == "" || (a != "" && a > b)
	})
}

// deadlineLess is the Leads and Drafts order: deadline, then Fit, then newest posted.
func (o rowOrder) deadlineLess(a, b store.Application) bool {
	da, okA := deadline(a, o.rules)
	db, okB := deadline(b, o.rules)
	switch {
	case okA != okB:
		return okA
	case okA && !da.Equal(db):
		return da.Before(db)
	}
	if ra, rb := fitRank(a.Fit), fitRank(b.Fit); ra != rb {
		return ra < rb
	}
	if a.Posted != b.Posted {
		return b.Posted == "" || (a.Posted != "" && a.Posted > b.Posted)
	}
	return a.Folder < b.Folder
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
