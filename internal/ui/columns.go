package ui

import (
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/internal/store"
)

// followUpFlag marks an Applied or Interviewing row whose last change is older than follow_up_days.
const followUpFlag = "⚑"

// separatorWidth is the cells between columns (" | ").
const separatorWidth = 3

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
		fit = "—"
	}
	docs := ""
	switch strings.ToLower(a.Documents) {
	case "ready":
		docs = "✓"
	case "written":
		docs = "○"
	}
	if table == tableDismissed {
		return append(out, entryDate(a.LastChange))
	}
	if beforeApplied(table) {
		closes := a.Closes
		if closes != "" && closes < now.Format("2006-01-02") {
			closes = "past"
		}
		out = append(out, closes, fit)
		if table == tableDraft {
			out = append(out, docs)
		}
		return out
	}
	flag := ""
	if (a.Status == store.StatusApplied || a.Status == store.StatusInterviewing) && a.LastChange != nil {
		at, err := time.Parse(time.RFC3339, a.LastChange.At)
		if err != nil {
			at, err = time.Parse("2006-01-02", a.LastChange.At)
		}
		if err == nil && now.After(at.AddDate(0, 0, followUpDays)) {
			flag = followUpFlag
		}
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

func RenderColumns(cells [][]string, width int) []string {
	widths := make([]int, len(cells[0]))
	for _, row := range cells {
		for i, c := range row {
			if w := ansi.StringWidth(c); w > widths[i] {
				widths[i] = w
			}
		}
	}
	total := separatorWidth * (len(widths) - 1)
	for _, w := range widths {
		total += w
	}
	if width > 0 {
		for _, i := range []int{0, 1, 2, 3, 4, 5, 6} {
			if i >= len(widths) {
				break
			}
			for total > width && widths[i] > 1 {
				widths[i]--
				total--
			}
		}
	}
	out := make([]string, len(cells))
	for n, row := range cells {
		parts := make([]string, len(widths))
		for i := range widths {
			c := ""
			if i < len(row) {
				c = row[i]
			}
			c = ansi.Truncate(c, widths[i], "…")
			parts[i] = c + strings.Repeat(" ", max(0, widths[i]-ansi.StringWidth(c)))
		}
		out[n] = strings.TrimRight(strings.Join(parts, " | "), " ")
	}
	return out
}
