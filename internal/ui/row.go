package ui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/internal/schedule"
	"github.com/lukebrown14-code/rolecall/internal/store"
)

// row is one Application as two lines: title and right-hand columns, then company ·
// track · dates; or as one line, in aligned columns, when compact rows are on. The
// selected row is a full-width bar; text on it is never dimmed.
func (m model) row(p pal, a store.Application, sel bool, show slotSet, w int) []string {
	line := lipgloss.NewStyle().Width(w).MaxWidth(w)
	titleC, dimC := p.Fg, p.Dim
	if m.expired(a) {
		titleC = p.Dim
	}
	marked := m.isMarked(a.Folder)
	fill := sel
	// A marked row off the cursor takes the subtle mark fill; the cursor row keeps the
	// selection fill, and its ◉ says it is marked too.
	var ground color.Color
	switch {
	case fill:
		ground = p.Sel
	case marked:
		ground = p.MarkFill()
	}
	st := func(c color.Color) lipgloss.Style {
		s := lipgloss.NewStyle().Foreground(c)
		if ground != nil {
			s = s.Background(ground)
		}
		return s
	}
	if ground != nil {
		line = line.Background(ground)
	}
	// The gutter is three cells: the selection bar, the mark column, a space. The mark has
	// its own column, so the Status glyph sits in the same cell on every row, marked or
	// not, under the cursor or not, and the ◉ never touches the ▌ or the glyph.
	cursor := st(p.Fg).Render(" ")
	if sel {
		titleC, dimC = p.Fg, p.Sub
		cursor = xansi.Cut(p.SelGutter(fill), 0, 1)
	}
	space := st(p.Fg).Render(" ")
	bar2 := cursor + space + space // the second line carries the selection bar only: the mark sits on the first
	bar := bar2
	if marked {
		// ◉ reads without colour (ASCII: [), beside the ▌ on the cursor row.
		bar = cursor + st(p.Fg).Bold(true).Render(glyphMarked) + space
	}
	if a.Malformed != nil {
		title := st(p.Bad).Bold(true).Render("⚠ ") + st(p.Bad).Bold(true).Render(m.malformedTitle(a))
		if fill {
			title = st(p.Bad).Bold(true).Render("⚠ ") + st(p.Fg).Bold(true).Render(m.malformedTitle(a)) // the words clear 4.5:1 on the fill
		}
		reason := m.malformedReason(a)
		if m.compactShown() {
			return []string{line.Render(xansi.Truncate(bar+title+st(dimC).Render("  "+reason), w, "…"))}
		}
		return []string{line.Render(bar + title),
			line.Render(bar + st(dimC).Render("  "+xansi.Truncate(reason, w-ellipsis-statusMarkWidth, "…")))} // the bar spans both lines, like any row
	}
	flag, fit := m.slotText(a)
	fitC, heatC := p.FitColor(a.Fit), p.Warn
	if fitC == p.Dim {
		fitC = dimC
	}
	if fill {
		// Coloured text on the selection fill falls below 4.5:1, so the selected row's
		// words take the secondary colour; its Status glyph keeps the colour.
		fitC, heatC = p.Sub, p.Sub
	}
	// The right-hand columns are the view's (slotsFor): a row without a value holds the
	// column blank, so every column starts at the same cell on every row. The flag sits
	// flush right in its column; the Fit word starts at its column's first cell.
	buildRight := func() string {
		heat := st(heatC).Width(show.flagW).Align(lipgloss.Right)
		if past, due := schedule.FollowUpDaysPast(a, m.now(), m.followUpDays); due && followUpLate(past) {
			heat = heat.Bold(true)
		}
		out := heat.Render(flag)
		if show.fitW > 0 {
			out += st(p.Fg).Render(strings.Repeat(" ", show.fitW-fitSlotWidth)) +
				st(fitC).Width(fitSlotWidth).Render(xansi.Truncate(fit, fitSlotWidth, ""))
		}
		return out + st(p.Fg).Render(strings.Repeat(" ", rowPad))
	}
	// The row carries no ready meter: the detail's DOCS line and a Draft's second line say
	// how far its Documents are.
	right := buildRight()
	// The Status glyph leads the title; the second line lines up under the title.
	// On the selection fill the row's coloured words and its muted glyph take the
	// secondary colour (OnSel), so they clear the floors there.
	rp := p
	if fill {
		rp = pal{p.OnSel()}
	}
	mark := st(rp.StatusColor(a.Status)).Render(p.StatusGlyph(a.Status)) + st(p.Fg).Render(" ")
	room := max(1, w-xansi.StringWidth(bar)-xansi.StringWidth(mark)-xansi.StringWidth(right)-1)
	sep := st(dimC).Render(" · ")
	if m.compactShown() {
		dates := m.compactDates(rp, a, st, dimC)
		left := bar + mark + m.compactColumns(a, show, room, st, titleC, dimC, sel, strings.Join(dates, sep))
		gap := st(p.Fg).Render(strings.Repeat(" ", max(1, w-xansi.StringWidth(left)-xansi.StringWidth(right))))
		lines := []string{line.Render(left + gap + right)}
		if m.flash != "" && a.Folder == m.flash {
			lines = p.FlashRow(lines, w)
		}
		return lines
	}
	// Without colour the title and its meta line would draw alike; the title takes bold
	// there, so the two-line rhythm survives.
	left := bar + mark + st(titleC).Bold(sel || noColor()).Render(xansi.Truncate(titleLabel(a.Title), room, "…"))
	gap := st(p.Fg).Render(strings.Repeat(" ", max(1, w-xansi.StringWidth(left)-xansi.StringWidth(right))))
	meta := strings.Join(m.meta(rp, a, st, dimC), sep)
	// The bar spans both lines, so the selected row reads as one block.
	lines := []string{line.Render(left + gap + right), line.Render(xansi.Truncate(bar2+st(p.Fg).Render("  ")+meta, w, "…"))}
	if m.flash != "" && a.Folder == m.flash {
		lines = p.FlashRow(lines, w)
	}
	return lines
}

// compactDates is a compact row's dates column: a booked next step first, then the
// dates that matter for its table and the ad's badges, as the two-line row's meta has them.
func (m model) compactDates(p pal, a store.Application, st func(color.Color) lipgloss.Style, dimC color.Color) []string {
	dates := m.metaDates(p, a, st, dimC)
	if text, s := m.rowNextStep(a); s != closeNone {
		dates = append([]string{p.CloseStyle(s, st, dimC).Render(text)}, dates...)
	}
	return dates
}

// compactColumns is a compact row between its Status glyph and its right-hand columns, in
// room cells: the title, company and track in columns as wide as the view's widest (each
// capped), then the dates and badges, cut at the end. The first of the dates keeps
// show.datesW cells, whole: short of room the title gives way first, down to compactTitleMin,
// then the track column goes, the company narrows, the title narrows further, and last
// the company goes.
func (m model) compactColumns(a store.Application, show slotSet, room int, st func(color.Color) lipgloss.Style,
	titleC, dimC color.Color, sel bool, dates string) string {
	titleW := min(show.titleW, max(compactTitleMin, room*compactTitleShare/percent), room)
	companyW, trackW, minDates := show.companyW, show.trackW, show.datesW
	need := func() int {
		n := titleW
		if companyW > 0 {
			n += columnGap + companyW
		}
		if trackW > 0 {
			n += columnGap + trackW
		}
		if minDates > 0 {
			n += columnGap + minDates
		}
		return n
	}
	for over := need() - room; over > 0 && minDates > 0; over = need() - room {
		switch {
		case titleW > compactTitleMin:
			titleW = max(compactTitleMin, titleW-over)
		case trackW > 0:
			trackW = 0
		case companyW > compactCompanyMin:
			companyW = max(compactCompanyMin, companyW-over)
		case titleW > compactTitleFloor:
			titleW = max(compactTitleFloor, titleW-over)
		case companyW > 0:
			companyW = 0
		default:
			minDates = 0 // no room left to give: the dates are cut at the end
		}
	}
	if titleW+columnGap+companyW > room {
		companyW = 0
	}
	if titleW+columnGap+companyW+columnGap+trackW > room {
		trackW = 0
	}
	cell := func(s string, n int) string {
		if n == 0 {
			return ""
		}
		return st(dimC).Render(strings.Repeat(" ", columnGap)) + st(dimC).Width(n).Render(xansi.Truncate(s, n, "…"))
	}
	out := st(titleC).Bold(sel).Width(titleW).Render(xansi.Truncate(titleLabel(a.Title), titleW, "…")) +
		cell(a.CompanyLabel(), companyW) + cell(a.Track, trackW)
	if rest := room - xansi.StringWidth(out) - columnGap; rest > 0 && dates != "" {
		out += st(dimC).Render(strings.Repeat(" ", columnGap)) + xansi.Truncate(dates, rest, "…")
	}
	return out
}

// meta is a row's second line: company, track, and the dates that matter for its table.
func (m model) meta(p pal, a store.Application, st func(color.Color) lipgloss.Style, dimC color.Color) []string {
	dim := st(dimC)
	var out []string
	if strings.TrimSpace(a.CompanyLabel()) != "" {
		out = append(out, dim.Render(a.CompanyLabel()))
	}
	// A booked next step comes straight after the Company, so a narrow row cuts the
	// softer facts first.
	if text, s := m.rowNextStep(a); s != closeNone {
		out = append(out, p.CloseStyle(s, st, dimC).Render(text))
	}
	out = append(out, dim.Render(a.Track))
	return append(out, m.metaDates(p, a, st, dimC)...)
}

// metaDates is the part of a row's meta after company and track: the dates that matter
// for its table, and on a Lead or Draft the ad's badges. Meaning never rests on colour: a
// close date within a week is bold (and ▲ within two days), and a blocker badge leads
// with ⚠.
func (m model) metaDates(p pal, a store.Application, st func(color.Color) lipgloss.Style, dimC color.Color) []string {
	dim := st(dimC)
	// when renders a Status date, flagging one in the future as a data error.
	when := func(e *store.StatusEntry, prefix string) string {
		if m.futureDate(e) {
			return st(p.Warn).Render(prefix + futureFlag)
		}
		return dim.Render(prefix + m.agoEntry(e))
	}
	var out []string
	switch t := tableOf(a); t {
	case tableLeads, tableDraft:
		// No close date shows nothing: the slot is optional, not a dash on every row.
		if text, s := closeInfo(a.Closes, m.now()); s == closeNone && a.ClosesUnclear {
			out = append(out, st(p.Warn).Render("closes: "+closesUnclearText))
		} else if s != closeNone {
			out = append(out, p.CloseStyle(s, st, dimC).Render(text))
		}
		// After the close date, which matters more: how old the ad is, and how near the
		// max posting age it has come.
		if age := postedAge(a, m.now()); age != "" && t == tableLeads {
			posted := dim.Render("posted " + age)
			if warn := ageWarning(a, m.rules, m.now()); warn != "" {
				posted += st(p.Warn).Render(" " + glyphWarn + " " + warn)
			}
			out = append(out, posted)
		}
		if t == tableDraft {
			label, c := p.DocsLabel(a)
			if c == p.Dim {
				c = dimC
			}
			out = append(out, st(c).Render(label))
		}
		// Badges come last, so a narrow row drops them before anything else, and blockers
		// lead them (adBadges). Amber and red stay for time: a blocker is ⚠ and bold in the
		// text colour, a visa fit green, a note muted.
		for _, b := range m.adBadges(a) {
			switch b.tone {
			case toneBlock:
				out = append(out, st(p.Fg).Bold(true).Render(glyphWarn+" "+b.text))
			case toneOK:
				out = append(out, st(p.Ok).Render(b.text))
			default:
				out = append(out, dim.Render(b.text))
			}
		}
	case tableDismissed:
		if schedule.LastChange(a) != nil {
			out = append(out, when(schedule.LastChange(a), "dismissed "))
		}
	case tableFiltered:
		out = append(out, dim.Render(filterReasonLabel(a.FilterReason)))
		if schedule.LastChange(a) != nil {
			out = append(out, when(schedule.LastChange(a), "dismissed "))
		}
	case tableClosed:
		if schedule.LastChange(a) != nil {
			// The outcome by name, "rejected ~7d ago": "closed" alone could be the ad.
			out = append(out, when(schedule.LastChange(a), strings.ToLower(schedule.LastChange(a).Status)+" "))
		}
	default:
		if schedule.AppliedDate(a) != nil {
			out = append(out, when(schedule.AppliedDate(a), "applied "))
		}
		if schedule.LastChange(a) != nil && schedule.LastChange(a).At != entryAt(schedule.AppliedDate(a)) {
			out = append(out, when(schedule.LastChange(a), strings.ToLower(schedule.LastChange(a).Status)+" "))
		}
	}
	return out
}

// agoEntry is a Status entry's date relative to today, with SCHEMA's ~ for an
// approximate date.
func (m model) agoEntry(e *store.StatusEntry) string {
	r, _ := ago(e.At, m.now())
	if e.Approx {
		r = "~" + r
	}
	return r
}

func entryAt(e *store.StatusEntry) string {
	if e == nil {
		return ""
	}
	return e.At
}
