package ui

import (
	"path/filepath"
	"sort"
	"strings"
	"time"

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
	out := []string{a.Title, a.Company, a.Track}
	fit := a.Fit
	if fit == "" {
		fit = glyphNone
	}
	docs := ""
	switch strings.ToLower(a.Documents) {
	case "ready":
		docs = glyphReady
	case "written":
		docs = glyphMissing
	}
	if table == tableDismissed {
		return append(out, entryDate(a.LastChange))
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
	return append(out, entryDate(a.AppliedDate), fit, docs, flag)
}

func SortApplications(rows []store.Application, table string) {
	date := func(a store.Application) string {
		if beforeApplied(table) {
			return a.Closes
		}
		if table == tableDismissed {
			if a.LastChange != nil {
				return a.LastChange.At
			}
			return ""
		}
		if a.AppliedDate != nil {
			return a.AppliedDate.At
		}
		return ""
	}
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := date(rows[i]), date(rows[j])
		if a == b {
			return rows[i].Folder < rows[j].Folder
		}
		if a == "" {
			return false
		}
		if b == "" {
			return true
		}
		if beforeApplied(table) {
			return a < b
		}
		return a > b
	})
}
