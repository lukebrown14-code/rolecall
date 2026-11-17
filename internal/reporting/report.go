// Package reporting calculates application statistics independently of terminal rendering.
package reporting

import (
	"math"
	"slices"
	"strings"
	"time"

	"github.com/lukebrown14-code/rolecall/internal/store"
)

// StatsWeeks is the number of Monday-start weeks in the weekly chart.
const (
	StatsWeeks          = 12
	daysPerWeek         = 7
	defaultFollowUpDays = 14
)

// UndoWindow is how soon after a move an entry restoring the Status before it reads as
// the TUI's undo of that move. Undo appends the earlier Status again (the log is append
// only), so the log alone cannot tell an undo from a deliberate move back; an undo is
// pressed seconds after the move, and a move back is days later.
const UndoWindow = 15 * time.Minute

// FunnelStatuses are the Statuses the funnel counts, in pipeline order.
var FunnelStatuses = []string{store.StatusLead, store.StatusDraft, store.StatusApplied, store.StatusInterviewing, store.StatusOffer}

// StatsReport is the pure Stats value. It is built from the Applications after the
// track filter and a fixed "now"; the screen only renders it.
type StatsReport struct {
	Funnel    []FunnelStep
	Dismissed int
	Rejected  int
	Withdrawn int
	Response  Response
	Weeks     []Week
	Tracks    []TrackStats
	// DropOff is where closed Applications ended, by close_reason, most first. It is empty
	// until at least one closed Application has a reason; Unrecorded counts the ones
	// without.
	DropOff    []DropOff
	Unrecorded int
}

// DropOff is how many Rejected or Withdrawn Applications carry one close_reason.
type DropOff struct {
	Reason string
	Count  int
}

// FunnelStep is one rung of the ever-reached funnel (see Reached). Lead and Draft are
// optional starts, so a rung can be higher than the one before. Through is how many
// reached the previous rung's Status before this one; Skipped is how many reached this
// one without the previous rung first. Conversion is Through as a percentage of the
// previous rung, OK only when that rung is not empty. All are zero on the first rung.
type FunnelStep struct {
	Status       string
	Count        int
	Through      int
	Skipped      int
	Conversion   int
	ConversionOK bool
}

// Response is the Response rate. Reached and Applied are the numerator and
// denominator; OK is false when the denominator is zero, so the screen shows "—".
// TooNew is the Applications left out because they were applied inside the follow-up
// window.
type Response struct {
	Reached int
	Applied int
	Rate    int
	OK      bool
	TooNew  int
}

// Week is one Monday-start week in the weekly chart, local time.
type Week struct {
	Start time.Time
	Count int
}

// TrackStats is one row of the per-track table. Applied counts Applications that ever
// reached Applied, as the funnel does, so the tracks sum to the funnel's Applied rung.
// Judged is those applied outside the follow-up window (the Response denominator);
// Rate is Responses/Judged.
type TrackStats struct {
	Track     string
	Applied   int
	Judged    int
	Responses int
	Rate      int
	OK        bool
}

// EffectiveLog is a status_log with each undone move taken out. An entry that restores
// the Status before the previous entry, within UndoWindow of it, is an undo: it and the
// move it undid are both dropped. Undoing an Application's first move back to Lead
// (an empty log reads as Lead) empties the log. Date-only entries, which only an agent or
// a hand edit writes, never read as an undo.
func EffectiveLog(log []store.StatusEntry) []store.StatusEntry {
	var out []store.StatusEntry
	for _, e := range log {
		n := len(out)
		switch {
		case n >= 2 && out[n-2].Status == e.Status && undoes(out[n-1], e):
			out = out[:n-1]
		case n == 1 && e.Status == store.StatusLead && out[0].Status != store.StatusLead && undoes(out[0], e):
			out = out[:0]
		default:
			out = append(out, e)
		}
	}
	return out
}

// undoes reports whether entry is an undo of move: both timestamped, entry no more than
// UndoWindow after move.
func undoes(move, entry store.StatusEntry) bool {
	from, err := time.Parse(time.RFC3339, move.At)
	if err != nil {
		return false
	}
	to, err := time.Parse(time.RFC3339, entry.At)
	if err != nil {
		return false
	}
	gap := to.Sub(from)
	return gap >= 0 && gap <= UndoWindow
}

// reach is one Status an Application reached and when. Implied is a Status filled in
// because a later one means it happened (see Reached); it carries that Status's date.
type reach struct {
	status  string
	at      string
	implied bool
}

// sentStatuses are the Statuses only a sent Application has, so each implies Applied.
var sentStatuses = map[string]bool{
	store.StatusInterviewing: true, store.StatusOffer: true, store.StatusRejected: true, store.StatusWithdrawn: true,
}

// Reached is the Statuses an Application reached, in log order, read from its
// EffectiveLog (an empty one is Lead). A later Status implies the ones it cannot happen
// without, so the counts agree with the screens: Interviewing, Offer, Rejected and
// Withdrawn imply Applied, and Offer implies Interviewing. An implied Status is placed
// just before the one implying it, with its date. Lead and Draft are optional starts and
// never implied.
func Reached(a store.Application) []string {
	path := reachedOf(a)
	out := make([]string, len(path))
	for i, r := range path {
		out[i] = r.status
	}
	return out
}

func reachedOf(a store.Application) []reach {
	log := EffectiveLog(a.StatusLog)
	if len(log) == 0 {
		return []reach{{status: store.StatusLead}}
	}
	seen := map[string]bool{}
	var out []reach
	add := func(r reach) {
		out = append(out, r)
		seen[r.status] = true
	}
	for _, e := range log {
		if sentStatuses[e.Status] && !seen[store.StatusApplied] {
			add(reach{status: store.StatusApplied, at: e.At, implied: true})
		}
		if e.Status == store.StatusOffer && !seen[store.StatusInterviewing] {
			add(reach{status: store.StatusInterviewing, at: e.At, implied: true})
		}
		add(reach{status: e.Status, at: e.At})
	}
	return out
}

// path is one Application as Stats reads it: where each Status was first reached.
type path struct {
	steps []reach
	first map[string]int
}

func pathOf(a store.Application) path {
	p := path{steps: reachedOf(a), first: map[string]int{}}
	for i, r := range p.steps {
		if _, ok := p.first[r.status]; !ok {
			p.first[r.status] = i
		}
	}
	return p
}

func (p path) reached(status string) bool {
	_, ok := p.first[status]
	return ok
}

// before reports whether earlier was reached before later.
func (p path) before(earlier, later string) bool {
	i, ok := p.first[earlier]
	j, ok2 := p.first[later]
	return ok && ok2 && i < j
}

// responded is a Response: the Application reached Interviewing or Offer.
func (p path) responded() bool {
	return p.reached(store.StatusInterviewing) || p.reached(store.StatusOffer)
}

// appliedAt is the date the Application first reached Applied, in loc. An implied
// Applied takes the date of the Status that implied it, the latest it can have been;
// realOnly leaves those out. ok is false when it never reached Applied or the date does
// not read.
func (p path) appliedAt(loc *time.Location, realOnly bool) (time.Time, bool) {
	i, ok := p.first[store.StatusApplied]
	if !ok || (realOnly && p.steps[i].implied) {
		return time.Time{}, false
	}
	return parseLocal(p.steps[i].at, loc)
}

// judged reports whether an Application that reached Applied is old enough to count
// toward the Response rate. One with no readable Applied date counts: it is not known
// to be new.
func (p path) judged(now time.Time, followUpDays int) bool {
	applied, ok := p.appliedAt(now.Location(), false)
	return !ok || !tooNewToJudge(applied, now, followUpDays)
}

// Build builds the Stats value from Applications and "now". Malformed Applications are
// left out of every figure. followUpDays defaults to the configured default; pass it
// explicitly when the Owner has a setting.
func Build(apps []store.Application, now time.Time, followUpDays ...int) StatsReport {
	days := defaultFollowUpDays
	if len(followUpDays) > 0 {
		days = followUpDays[0]
	}
	var paths []path
	var tracked []store.Application
	for _, a := range apps {
		if a.Malformed == nil {
			paths = append(paths, pathOf(a))
			tracked = append(tracked, a)
		}
	}

	var report StatsReport
	report.Funnel = make([]FunnelStep, len(FunnelStatuses))
	for i, status := range FunnelStatuses {
		report.Funnel[i].Status = status
	}
	for _, p := range paths {
		for i, status := range FunnelStatuses {
			if !p.reached(status) {
				continue
			}
			step := &report.Funnel[i]
			step.Count++
			if i == 0 {
				continue
			}
			if p.before(FunnelStatuses[i-1], status) {
				step.Through++
			} else {
				step.Skipped++
			}
		}
		for status, n := range map[string]*int{store.StatusDismissed: &report.Dismissed, store.StatusRejected: &report.Rejected, store.StatusWithdrawn: &report.Withdrawn} {
			if p.reached(status) {
				*n++
			}
		}
		if !p.reached(store.StatusApplied) {
			continue
		}
		if !p.judged(now, days) {
			report.Response.TooNew++
			continue
		}
		report.Response.Applied++
		if p.responded() {
			report.Response.Reached++
		}
	}
	for i := 1; i < len(report.Funnel); i++ {
		step, prev := &report.Funnel[i], report.Funnel[i-1]
		if prev.Count > 0 {
			step.Conversion = percentOf(step.Through, prev.Count)
			step.ConversionOK = true
		}
	}
	report.Response.OK = report.Response.Applied > 0
	if report.Response.OK {
		report.Response.Rate = percentOf(report.Response.Reached, report.Response.Applied)
	}

	report.DropOff, report.Unrecorded = dropOff(tracked)
	report.Weeks = weeksOf(paths, now, StatsWeeks)
	report.Tracks = tracksOf(tracked, paths, now, days)
	return report
}

// parseLocal reads a date-only or RFC3339 timestamp in loc. The zone comes from the
// caller's "now", not the process-global time.Local, so the Stats value is a pure
// function of its inputs.
func parseLocal(s string, loc *time.Location) (time.Time, bool) {
	if t, err := time.ParseInLocation("2006-01-02", s, loc); err == nil {
		return t, true
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.In(loc), true
	}
	return time.Time{}, false
}

// tooNewToJudge reports whether an Applied date is still inside the follow-up window,
// so there has not been time for a Response. Exactly followUpDays old is outside it.
func tooNewToJudge(applied, now time.Time, followUpDays int) bool {
	return applied.After(now.AddDate(0, 0, -followUpDays))
}

// percent rounds n/d to a whole percentage.
func percentOf(n, d int) int {
	if d <= 0 {
		return 0
	}
	return int(math.Round(float64(n) * 100 / float64(d)))
}

// weekStart is the Monday 00:00 in loc at the start of t's week.
func weekStart(t time.Time, loc *time.Location) time.Time {
	t = t.In(loc)
	day := (int(t.Weekday()) - int(time.Monday) + daysPerWeek) % daysPerWeek // Monday = 0
	t = t.AddDate(0, 0, -day)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
}

// AppliedWeeks is the last n Monday-start weeks, current week last, each with how many
// Applications were applied that week (local time, by the logged Applied date; an
// undone Applied does not count). Malformed Applications are left out. Stats draws 12;
// the tab bar draws the same series over fewer weeks.
func AppliedWeeks(apps []store.Application, now time.Time, n int) []Week {
	var paths []path
	for _, a := range apps {
		if a.Malformed == nil {
			paths = append(paths, pathOf(a))
		}
	}
	return weeksOf(paths, now, n)
}

// weeksOf returns the last n Monday-start weeks, current week last, with Applied
// counts. An Applied outside the window, or only implied by a later Status (it has no
// date of its own), is ignored.
func weeksOf(paths []path, now time.Time, n int) []Week {
	loc := now.Location()
	current := weekStart(now, loc)
	out := make([]Week, n)
	index := make(map[time.Time]int, n)
	for i := range out {
		start := current.AddDate(0, 0, -(n-1-i)*daysPerWeek)
		out[i] = Week{Start: start}
		index[start] = i
	}
	for _, p := range paths {
		applied, ok := p.appliedAt(loc, true)
		if !ok {
			continue
		}
		if i, ok := index[weekStart(applied, loc)]; ok {
			out[i].Count++
		}
	}
	return out
}

// tracksOf returns one row per track in first-appearance order, with eligible
// Applied and Response counts. apps and paths are the same well-formed Applications.
func tracksOf(apps []store.Application, paths []path, now time.Time, followUpDays int) []TrackStats {
	var out []TrackStats
	index := map[string]int{}
	for i, a := range apps {
		at, ok := index[a.Track]
		if !ok {
			at = len(out)
			index[a.Track] = at
			out = append(out, TrackStats{Track: a.Track})
		}
		p, row := paths[i], &out[at]
		if !p.reached(store.StatusApplied) {
			continue
		}
		row.Applied++
		if !p.judged(now, followUpDays) {
			continue
		}
		row.Judged++
		if p.responded() {
			row.Responses++
		}
	}
	for i := range out {
		out[i].OK = out[i].Judged > 0
		if out[i].OK {
			out[i].Rate = percentOf(out[i].Responses, out[i].Judged)
		}
	}
	return out
}

// OrderTracks splits the per-track rows for the table: sent is the Tracks with any
// Applied, busiest first, and idle the rest, which have nothing to show. Within each,
// Tracks keep order (the workspace.json Tracks), then any other Track in the order the
// report found it.
func OrderTracks(rows []TrackStats, order []string) (sent, idle []TrackStats) {
	rank := make(map[string]int, len(order))
	for i, t := range order {
		if _, ok := rank[t]; !ok {
			rank[t] = i
		}
	}
	pos := func(i int) int {
		if r, ok := rank[rows[i].Track]; ok {
			return r
		}
		return len(order) + i
	}
	idx := make([]int, len(rows))
	for i := range idx {
		idx[i] = i
	}
	slices.SortStableFunc(idx, func(a, b int) int {
		if rows[a].Applied != rows[b].Applied {
			return rows[b].Applied - rows[a].Applied
		}
		return pos(a) - pos(b)
	})
	for _, i := range idx {
		if rows[i].Applied > 0 {
			sent = append(sent, rows[i])
		} else {
			idle = append(idle, rows[i])
		}
	}
	return sent, idle
}

// dropOff tallies the close_reason of every Application now Rejected or Withdrawn, most
// first and by name among equals. With no reason recorded anywhere it reports nothing, so
// the screen shows no empty section.
func dropOff(apps []store.Application) ([]DropOff, int) {
	counts := map[string]int{}
	unrecorded := 0
	for _, a := range apps {
		if a.Status != store.StatusRejected && a.Status != store.StatusWithdrawn {
			continue
		}
		if a.CloseReason == "" {
			unrecorded++
			continue
		}
		counts[a.CloseReason]++
	}
	if len(counts) == 0 {
		return nil, 0
	}
	out := make([]DropOff, 0, len(counts))
	for reason, n := range counts {
		out = append(out, DropOff{Reason: reason, Count: n})
	}
	slices.SortFunc(out, func(a, b DropOff) int {
		if a.Count != b.Count {
			return b.Count - a.Count
		}
		return strings.Compare(a.Reason, b.Reason)
	})
	return out, unrecorded
}
