package ui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/internal/store"
)

// preview is the selected Application beside the list, h lines: the card, then as much
// of fit.md and jd.md as the room under it holds.
func (m model) preview(p pal, a store.Application, w, h int) string {
	return lipgloss.NewStyle().Width(w).Height(h).MaxHeight(h).Padding(0, 1).Render(m.previewText(p, a, w-previewInset, h, cardPreview))
}

// previewText is the card's lines inside inner cells, at most h: the title, Company ·
// Track, then the fields. Beside the list (cardPreview) the room left fills with the
// start of fit.md, then of jd.md; beside the detail tabs (cardDetail), which show those
// whole, the link closes it instead. Opening the detail leaves the card's rows in place.
func (m model) previewText(p pal, a store.Application, inner, h int, mode cardMode) string {
	wrap := func(s string, c color.Color, maxLines int) string {
		return p.Text(c).Render(strings.Join(clampLines(wrapWords(s, inner), maxLines, inner), "\n"))
	}
	if a.Malformed != nil {
		return p.Text(p.Bad).Bold(true).Render("⚠ "+m.malformedTitle(a)) + "\n\n" + wrap(m.malformedReason(a), p.Fg, errorLines) +
			"\n\n" + p.Text(p.Accent).Render("e") + p.Text(p.Dim).Render(" fix in editor")
	}
	card := func(lean bool) []string {
		lines := strings.Split(wrap(titleLabel(a.Title), p.Fg, titleLines), "\n")
		lines = append(lines, m.previewMeta(p, a, inner), "")
		for _, r := range m.cardRows(p, a, inner, mode, lean) {
			lines = append(lines, strings.Split(r, "\n")...)
		}
		if mode == cardDetail && a.URL != "" {
			// One inline row, as every other field: the link and the key that opens it.
			key, action := "", ""
			if _, ok := webLink(a); ok {
				key, action = "w", descOpen
			}
			lines = append(lines, "", p.cardRow("LINK", p.Text(p.Info).Render(a.URL), key, action, inner))
		}
		return lines
	}
	lines := card(false)
	if len(lines) > h {
		// Short of room the Sweep's and the ad's rows fold first; what still does not fit
		// ends in the one overflow cue, never a silent cut.
		lines = p.cutCard(card(true), h, inner, mode)
	}
	if mode == cardPreview {
		// The Fit's reasoning first, as it decides dismiss or draft; the JD takes what is left.
		// Without a Fit in meta.yml, fit.md's text is reasoning with no verdict: the label
		// says so, so its words never read as a second Fit beside "fit not assessed".
		label := "FIT"
		if a.Fit == "" {
			label = fitReasoningOnly
		}
		fit := m.excerpt(p, label, m.docLines(a, fitFile), min(fitExcerptLines, h-len(lines)), inner)
		lines = append(lines, fit...)
		lines = append(lines, m.excerpt(p, docJD, m.docLines(a, jdFile), h-len(lines), inner)...)
	}
	return strings.Join(lines, "\n")
}

// cutCard keeps the card's first lines inside h, ending on "↓ 5 more" and where the rest
// is: enter opens the detail from the list; the detail's card does not scroll, so a
// taller window shows it (the tabs hold the Notes and Fit whole).
func (p pal) cutCard(lines []string, h, w int, mode cardMode) []string {
	if len(lines) <= h || h < 2 { //nolint:mnd // a line of the card and the cue
		return lines
	}
	shown := lines[:h-1]
	for len(shown) > 1 && xansi.Strip(shown[len(shown)-1]) == "" {
		shown = shown[:len(shown)-1] // the cue never follows a blank line
	}
	where := p.Key().Render("enter") + p.Text(p.Dim).Render(" open")
	if mode != cardPreview {
		where = p.Text(p.Dim).Render("taller window shows all")
	}
	cue := p.MoreCue(false, len(lines)-len(shown)) + p.Text(p.Dim).Render(" · ") + where
	return append(append([]string{}, shown...), xansi.Truncate(cue, w, "…"))
}

// fitReasoningOnly heads the preview's FIT excerpt when fit.md has text but no Fit is
// set, as the Fit tab says "fit not assessed · reasoning saved below".
const fitReasoningOnly = "FIT · reasoning only, " + fitUnassessed

// fitExcerptLines is the most of the preview the FIT block takes, its blank line and
// heading included, so the JD's opening still shows under a long fit.md.
const fitExcerptLines = 10

// excerpt is a document's opening under a block label, inside room lines counting the
// blank line before it: as many lines as fit, then, when some do not, "↓ 5 more · enter
// open". It is empty when the document has no text or room holds no line of it.
func (m model) excerpt(p pal, label string, text []string, room, w int) []string {
	var body []string
	for _, l := range text {
		if l == "" {
			body = append(body, "")
			continue
		}
		for _, wl := range wrapWords(l, w) {
			body = append(body, p.Text(p.Fg).Render(wl))
		}
	}
	avail := room - 2 //nolint:mnd // the blank line and the label
	switch {
	case len(body) == 0 || avail < 1:
		return nil
	case len(body) > avail && avail < 2: //nolint:mnd // one line of text and the cue
		return nil
	case len(body) > avail:
		shown := body[:avail-1]
		for len(shown) > 1 && shown[len(shown)-1] == "" {
			shown = shown[:len(shown)-1] // the cue never follows a blank line
		}
		cue := p.MoreCue(false, len(body)-len(shown)) + p.Text(p.Dim).Render(" · ") + p.Key().Render("enter") + p.Text(p.Dim).Render(" open")
		body = append(shown, cue)
	}
	return append([]string{"", p.blockLabel(xansi.Truncate(label, w, "…"))}, body...)
}

// docLines is a document's text as plain lines for a short excerpt: headings, tables, the
// fit.md verdict line (the card says the Fit) and Markdown's marks dropped, one blank line
// between paragraphs. Missing files read as none.
func (m model) docLines(a store.Application, name string) []string {
	text := m.md.body(m.gen, 0, a.Folder+"\x00"+name+"\x00lines", func() string {
		b, err := m.store.ReadDocument(a.Folder, name)
		if err != nil {
			return ""
		}
		var keep []string
		for _, l := range strings.Split(string(b), "\n") {
			l = strings.TrimSpace(l)
			switch {
			case l == "":
				if len(keep) > 0 && keep[len(keep)-1] != "" {
					keep = append(keep, "")
				}
			case strings.HasPrefix(l, "#"), strings.HasPrefix(l, "---"), strings.HasPrefix(l, "|"),
				strings.HasPrefix(strings.TrimLeft(l, "*_"), "Verdict:"):
			default:
				if l = strings.TrimSpace(markdownMarks.Replace(l)); l != "" {
					keep = append(keep, l)
				}
			}
		}
		return strings.TrimRight(strings.Join(keep, "\n"), "\n")
	})
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

// clampLines keeps at most n of lines, cutting the last one kept with "…" when more
// follow, so a long title fills its lines rather than ending on a lone "…".
func clampLines(lines []string, n, w int) []string {
	if len(lines) <= n || n <= 0 {
		return lines
	}
	out := append([]string{}, lines[:n]...)
	out[n-1] = xansi.Truncate(out[n-1]+" …", w, "…")
	return out
}

// previewMeta is the line under the card's title: "Orbit Labs · SWE". The row's dates,
// close date and badges are left to the card's own labelled rows, so none shows twice.
func (m model) previewMeta(p pal, a store.Application, w int) string {
	var parts []string
	if c := strings.TrimSpace(a.CompanyLabel()); c != "" {
		parts = append(parts, c)
	}
	if a.Track != "" {
		parts = append(parts, a.Track)
	}
	return p.Text(p.Dim).Render(xansi.Truncate(strings.Join(parts, " · "), w, "…"))
}

// fileMarks is the one-line Document set, each Written then Built, "✓ Resume ○ PDF
// ○ Cover letter ○ PDF": ✓ done, ○ still to do. Written means the file has text, not
// merely that it exists.
func (m model) fileMarks(p pal, a store.Application) string {
	return strings.Join(m.docMarks(p, a), "   ")
}

// docText is a document's text with Markdown headings and list markers dropped, for
// short previews. Missing files read as "".
func (m model) docText(a store.Application, name string) string {
	// docText is width-independent, so it shares the render cache at width 0; the body
	// key is the Document, so the three preview Documents do not evict one another.
	return m.md.body(m.gen, 0, a.Folder+"\x00"+name, func() string {
		b, err := m.store.ReadDocument(a.Folder, name)
		if err != nil {
			return ""
		}
		var keep []string
		for _, l := range strings.Split(string(b), "\n") {
			if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "#") && !strings.HasPrefix(l, "---") {
				keep = append(keep, strings.TrimLeft(l, "-* "))
			}
		}
		return strings.Join(keep, " ")
	})
}
