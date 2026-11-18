package settings

import (
	"errors"
	"image/color"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/internal/config"
	"github.com/lukebrown14-code/rolecall/internal/store"
	"github.com/lukebrown14-code/rolecall/internal/ui/theme"
)

// Look is what Settings borrows from package ui to draw as every other screen does: the
// palette, the shared measures, and the text, pane and pop-up helpers. Settings cannot
// import ui, so ui hands them in.
type Look struct {
	theme.Palette
	// Edge is the chrome column, Inset the text column, RightMargin the cells kept clear
	// at the right and Gutter the usual gap.
	Edge, Inset, RightMargin, Gutter int
	// Marker is the cells a row's marker takes; AboutGutter and AboutDivider set the
	// about pane off the rows.
	Marker, AboutGutter, AboutDivider int
	// PopWidth is a Settings pop-up's width and PopInset its border and padding.
	PopWidth, PopInset int
	ErrorMark          string
	Wrap               func(s string, w int) []string
	KeepTogether       func(w int, sep string, phrases ...string) string
	Divider            func(h int) string
	ClipLines          func(lines []string, from, h int, indent string) []string
	ContextRow         func(left, right string, w int) string
	Popup              func(title string, warn bool, body, hints string) string
}

// Settings screen sizing.
const (
	// maxLabelWidth caps the label column, which is otherwise as wide as the group's longest label.
	maxLabelWidth = 24
	// maxFieldWidth is the widest an inline field grows: a value is edited where it sits.
	maxFieldWidth = 30
	// minFieldWidth is the narrowest an inline field is drawn.
	minFieldWidth = 12
	// minValueRoom is the least width worth giving a value.
	minValueRoom = 12
	// bothSides halves a height to keep the cursor's row central.
	bothSides = 2
)

// tabs is the sections as `[Tracking]  Tracks …`: the active one in the single selection
// style, brackets and all, so it reads without colour, as the active tab does. Every
// section is named while they fit in room; otherwise only the active one and its place.
func (m Model) tabs(look Look, room int) string {
	p := look.Palette
	active := func(name string) string { return p.SelText(true).Render("[" + name + "]") }
	var parts []string
	for i, name := range Groups {
		if i == m.Group {
			parts = append(parts, active(name))
		} else {
			parts = append(parts, p.Text(p.Dim).Render(name))
		}
	}
	full := strings.Join(parts, "  ")
	if xansi.StringWidth(full) <= room {
		return full
	}
	return active(Groups[m.Group]) + p.Text(p.Dim).Render(" "+strconv.Itoa(m.Group+1)+" of "+strconv.Itoa(len(Groups)))
}

// Context is the heading and the section tabs, with right (the search chip) at the right.
// The cursor rests here too (k from the first field), where only the section keys act.
func (m Model) Context(look Look, w int, right string) string {
	p := look.Palette
	lead := strings.Repeat(" ", look.Inset)
	if m.Ws.Err != nil {
		return lead + p.Heading("SETTINGS")
	}
	if m.Header {
		lead = strings.Repeat(" ", look.Edge) + p.SelGutter(true)
	}
	head := lead + p.Heading("SETTINGS") + "  "
	room := w - look.RightMargin - xansi.StringWidth(head)
	if right != "" {
		room -= xansi.StringWidth(right) + look.Gutter
	}
	return look.ContextRow(head+m.tabs(look, room), right, w)
}

// Body is the settings of the selected section, or the invalid-file notice. With
// rowsW > 0 the focused row's about text has a pane of its own, rowsW from the left.
func (m Model) Body(look Look, env Env, w, h, rowsW int) string {
	p := look.Palette
	if m.Ws.Err != nil {
		pad := strings.Repeat(" ", look.Inset)
		head, why := "workspace.json is not valid JSON", m.Ws.Err
		var tle *store.TrackListError
		switch {
		case m.Ws.NewerSchema:
			head = "workspace.json is schema " + strconv.Itoa(m.Ws.Schema) + ", from a newer rolecall"
		case errors.As(why, &tle):
			head, why = "workspace.json lists a Track rolecall can't use", tle.Err
		}
		// The path and the reason wrap rather than cut: a long path is what the Owner has to
		// open, and a cut one cannot be copied.
		lines := []string{p.Text(p.Bad).Render(head)}
		for _, part := range []string{m.Ws.Path, why.Error()} {
			for _, l := range look.Wrap(part, max(1, w-look.Inset-look.RightMargin)) {
				lines = append(lines, p.Text(p.Fg).Render(l))
			}
		}
		lines = append(lines, "", p.Text(p.Dim).Render("Press e to fix it in your editor. Nothing is written."))
		text := pad + strings.Join(lines, "\n"+pad)
		return lipgloss.Place(w, h, lipgloss.Left, lipgloss.Top, text)
	}
	if rowsW == 0 {
		return lipgloss.NewStyle().Width(w).Height(h).MaxHeight(h).PaddingLeft(look.Edge).
			Render(m.RowsView(look, env, w-look.Edge-look.RightMargin, h, true))
	}
	rows := lipgloss.NewStyle().Width(rowsW).Height(h).MaxHeight(h).PaddingLeft(look.Edge).
		Render(m.RowsView(look, env, rowsW-look.Edge-look.Gutter, h, false))
	aboutW := w - rowsW - look.AboutDivider
	about := lipgloss.NewStyle().Width(aboutW).Height(h).MaxHeight(h).PaddingLeft(look.AboutGutter).
		Render(strings.Join(m.about(look, env, aboutW-look.AboutGutter-look.RightMargin), "\n"))
	return lipgloss.JoinHorizontal(lipgloss.Top, rows, look.Divider(h), about)
}

// focusedRow is the row under the cursor; ok is false on the header.
func (m Model) focusedRow(env Env) (Row, bool) {
	rows := m.GroupRows(m.Group, env)
	if m.Header || m.Row < 0 || m.Row >= len(rows) {
		return Row{}, false
	}
	return rows[m.Row], true
}

// about is the about pane: what the focused setting is, an example of its value, the file
// it is kept in and any warning, or what the group is while the header has focus.
func (m Model) about(look Look, env Env, w int) []string {
	p := look.Palette
	w = max(1, w)
	para := func(s string, c func(string) string) []string {
		var out []string
		for _, l := range look.Wrap(s, w) {
			out = append(out, c(l))
		}
		return out
	}
	fg := func(s string) string { return p.Text(p.Fg).Render(s) }
	dim := func(s string) string { return p.Text(p.Dim).Render(s) }
	r, ok := m.focusedRow(env)
	if !ok {
		lines := []string{p.Heading(Groups[m.Group]), ""}
		return append(lines, para(GroupAbout[m.Group], fg)...)
	}
	label := strings.TrimPrefix(r.Label, "+ ")
	lines := []string{p.Heading(label), ""}
	lines = append(lines, para(r.About, fg)...)
	if r.Example != "" {
		lines = append(lines, "")
		lines = append(lines, para("Example: "+r.Example, dim)...)
	}
	if !r.Action {
		lines = append(lines, "")
		lines = append(lines, para("Kept in "+m.FileOf(m.Group, m.Row, env)+".", dim)...)
	}
	if r.Warning != "" {
		lines = append(lines, "")
		lines = append(lines, para("⚠ "+r.Warning, func(s string) string { return p.Text(p.Warn).Render(s) })...)
	}
	return lines
}

// configNotice is the lines This machine shows above its rows while config.toml has a
// problem: what and where, and how to fix it.
func configNotice(look Look, env Env, width int) []string {
	p := look.Palette
	var lines []string
	add := func(text string, c color.Color) {
		for _, l := range look.Wrap(text, max(1, width-look.Marker)) {
			lines = append(lines, strings.Repeat(" ", look.Marker)+p.Text(c).Render(l))
		}
	}
	var fe *config.FileError
	switch {
	case errors.As(env.ConfigErr, &fe):
		head := configFile + " has an error"
		if w := fe.Where(); w != "" {
			head += " (" + w + ")"
		}
		add(head, p.Bad)
		add(fe.Reason, p.Fg)
		add(fe.Path, p.Fg)
		add("Press e to fix it in your editor. Until then rolecall uses the values below.", p.Dim)
	case env.ConfigErr != nil:
		add(env.ConfigErr.Error(), p.Bad)
	}
	// Warnings sit on their own setting's row, not here.
	if len(lines) > 0 {
		lines = append(lines, "")
	}
	return lines
}

// RowsView lists the selected group's settings, one per line, in three columns: label,
// value, and where the value comes from. The row marker shows only while the settings pane
// has focus. With aboutBelow, the focused row's about text follows the rows, as the pane
// beside them is not drawn.
func (m Model) RowsView(look Look, env Env, width, h int, aboutBelow bool) string {
	p := look.Palette
	marker, gutter := look.Marker, look.Gutter
	rows := m.GroupRows(m.Group, env)
	valueW, labelW, tailW := 0, 0, 0
	for _, r := range rows {
		if !r.ReadOnly {
			// A read-only row's reason never narrows the values: without room it wraps below.
			tailW = max(tailW, xansi.StringWidth(r.Tail()))
		}
		if !r.Action {
			labelW = max(labelW, marker+xansi.StringWidth(r.Label)+gutter)
		}
		w := xansi.StringWidth(r.ValueText())
		if r.Value == "" && !r.Action {
			w = xansi.StringWidth("not set")
		}
		valueW = max(valueW, w)
	}
	labelW = min(labelW, maxLabelWidth)
	room := width - labelW
	valueW = min(valueW, max(minValueRoom, room-tailW-gutter))
	marked := !m.Header
	editing := m.Edit
	var lines []string
	if Groups[m.Group] == GroupMachineName {
		lines = configNotice(look, env, width)
	}
	selLine := 0
	for i, r := range rows {
		if r.Section != "" {
			if len(lines) > 0 {
				lines = append(lines, "")
			}
			lines = append(lines, strings.Repeat(" ", marker)+p.Heading(r.Section))
		}
		selected := marked && i == m.Row
		set, isSetting := rowSetting(m.Group, i, len(rows))
		if i == m.Row {
			selLine = len(lines)
		}
		// Names in the body text colour and values a step quieter, so a row reads by its name.
		mark, label := "  ", p.Text(p.Fg).Render(r.Label)
		switch {
		case selected:
			mark, label = p.SelGutter(true), p.SelText(true).Render(r.Label)
		case r.Action:
			label = p.Text(p.Sub).Render(r.Label)
		}
		line := mark + label
		if r.Action {
			lines = append(lines, xansi.Truncate(line, width, "…"))
			continue
		}
		bg := p.SelBase(selected)
		// Muted text on the selection fill is below 4.5:1, so a selected row's quiet parts
		// take the secondary colour, as a selected list row's do.
		quiet := p.Dim
		if selected {
			quiet = p.Sub
		}
		pad := bg.Render(strings.Repeat(" ", max(1, labelW-marker-xansi.StringWidth(r.Label))))
		colW := valueW
		var value string
		var more []string // a long value's further lines, under its first
		if r.Value == "" {
			value = bg.Foreground(quiet).Italic(true).Render("not set")
		} else {
			colW = rowValueWidth(r, valueW, room, gutter)
			shown := fitValue(r.Label, r.ValueText(), colW)
			if !pathLabel(r.Label) && xansi.StringWidth(r.ValueText()) > colW {
				// Any other value wraps under itself rather than lose its end, at its " · "
				// breaks when it has them, so a count stays with its word.
				wrapped := strings.Split(look.KeepTogether(colW, " · ", strings.Split(r.ValueText(), " · ")...), "\n")
				if !strings.Contains(r.ValueText(), " · ") {
					wrapped = look.Wrap(r.ValueText(), colW)
				}
				shown, more = wrapped[0], wrapped[1:]
			}
			value = bg.Foreground(p.Sub).Render(shown)
		}
		ownW := xansi.StringWidth(value) // the value alone, before the column's padding
		value += bg.Render(strings.Repeat(" ", max(0, colW-ownW)))
		note, warn := bg.Foreground(quiet).Render(r.Note()), ""
		if editing != nil && isSetting && editing.Key == set.key {
			// The field takes the free width, up to its cap, so a placeholder shows whole.
			value, note = inlineField(look, bg, quiet, *editing, r, max(valueW, room-gutter))
			colW, ownW, more = xansi.StringWidth(value), xansi.StringWidth(value), nil
		}
		if r.Warning != "" {
			warn = bg.Foreground(p.Warn).Render(WarnSep + r.Warning)
		}
		tail := note + warn
		fillLine := func(s string) string {
			s = xansi.Truncate(s, width, "…")
			if selected {
				return p.SelLine(true, width, s)
			}
			return s
		}
		line += pad + value
		indent := strings.Repeat(" ", labelW)
		under := func(l string) { lines = append(lines, indent+l) }
		plain := xansi.Strip(tail)
		// The source and any warning are never cut. They line up in their column; without
		// room there they follow the value itself, and without room beside it they go under
		// it, after the rest of a wrapped value.
		if plain != "" && len(more) == 0 {
			gap := bg.Render(strings.Repeat(" ", gutter))
			switch tw := xansi.StringWidth(plain); {
			case width-labelW-colW-gutter >= tw:
				lines = append(lines, fillLine(line+gap+tail))
				continue
			case width-labelW-ownW-gutter >= tw:
				lines = append(lines, fillLine(mark+label+pad+xansi.Cut(value, 0, ownW)+gap+tail))
				continue
			}
			// Without room for both, the source stays beside its value and the warning goes
			// under it, so a long warning never moves the source off the value's line.
			if nw := xansi.StringWidth(xansi.Strip(note)); warn != "" && nw > 0 {
				warnLines := strings.Split(xansi.Wrap(bg.Foreground(p.Warn).Render(strings.TrimLeft(WarnSep, " ")+r.Warning), max(1, width-labelW), " "), "\n")
				switch {
				case width-labelW-colW-gutter >= nw:
					lines = append(lines, fillLine(line+gap+note))
				case width-labelW-ownW-gutter >= nw:
					lines = append(lines, fillLine(mark+label+pad+xansi.Cut(value, 0, ownW)+gap+note))
				default:
					warnLines = nil
				}
				if warnLines != nil {
					for _, l := range warnLines {
						under(l)
					}
					continue
				}
			}
		}
		lines = append(lines, fillLine(line))
		for _, l := range more {
			under(p.Text(p.Sub).Render(l))
		}
		if plain == "" {
			continue
		}
		tailW := max(1, width-labelW)
		parts := []string{tail}
		if xansi.StringWidth(plain) > tailW && warn != "" && xansi.Strip(note) != "" {
			// The source and the warning each take a line rather than break mid-phrase.
			parts = []string{note, bg.Foreground(p.Warn).Render(strings.TrimLeft(WarnSep, " ") + r.Warning)}
		}
		for _, part := range parts {
			// On its own line the tail wraps rather than lose the end of an error.
			for _, l := range strings.Split(xansi.Wrap(part, tailW, " "), "\n") {
				under(l)
			}
		}
	}
	if aboutBelow {
		if r, ok := m.focusedRow(env); ok && r.About != "" {
			about := look.Wrap(r.About, max(1, width-marker))
			if len(lines)+1+len(about) <= h {
				// Short terminals lose the about text before they lose a field.
				lines = append(lines, "")
				for _, l := range about {
					lines = append(lines, strings.Repeat(" ", marker)+p.Text(p.Dim).Render(l))
				}
			}
		}
	}
	return strings.Join(look.ClipLines(lines, selLine-h/bothSides, h, strings.Repeat(" ", marker)), "\n")
}

// inlineField is the value cell and the tail of a row being edited: the field where the
// value sits, narrow, with the end of what is typed. An empty field shows the default in
// effect as a placeholder; the opening text shows selected, as typing replaces it. The
// example stays beside the field, and a quit's question takes the tail.
func inlineField(look Look, bg lipgloss.Style, quiet color.Color, e Edit, r Row, valueW int) (value, tail string) {
	p := look.Palette
	fw := min(maxFieldWidth, max(minFieldWidth, valueW))
	cursor := bg.Foreground(p.Fg).Render("▏")
	switch {
	case e.Buf == "" && e.Placeholder != "":
		shown := xansi.Truncate(e.Placeholder, fw-1, "…")
		value = cursor + bg.Foreground(quiet).Italic(true).Render(shown)
		value += bg.Render(strings.Repeat(" ", max(0, fw-xansi.StringWidth(shown)-1)))
	default:
		shown := xansi.TruncateLeft(e.Buf, max(0, xansi.StringWidth(e.Buf)-(fw-1)), "…")
		style := bg.Foreground(p.Fg).Underline(true)
		if e.Selected {
			style = bg.Foreground(p.Fg).Reverse(true)
		}
		value = style.Render(shown) + cursor
		value += bg.Render(strings.Repeat(" ", max(0, fw-xansi.StringWidth(shown)-1)))
	}
	var parts []string
	if r.Unit != "" {
		parts = append(parts, r.Unit)
	}
	if r.Example != "" && (e.Buf != "" || e.Placeholder != r.Example) {
		parts = append(parts, "example: "+r.Example) // not twice while the placeholder shows it
	}
	tail = bg.Foreground(quiet).Render(strings.Join(parts, " · "))
	switch {
	case e.Discarding:
		tail = bg.Foreground(p.Warn).Bold(true).Render("⚠ Discard changes? y discard · n keep editing")
	case e.Err != "":
		tail = bg.Foreground(p.Bad).Render(look.ErrorMark + " " + e.Err)
	}
	return value, tail
}

// rowValueWidth is the cells a row's value gets. Values share one column of colW so the
// sources line up, but a longer value takes the free width after it before it is cut:
// beside its own source when both fit, otherwise all of room, its source below.
func rowValueWidth(r Row, colW, room, gutter int) int {
	vw := xansi.StringWidth(r.ValueText())
	if vw <= colW {
		return colW
	}
	if tw := xansi.StringWidth(r.Tail()); tw > 0 && vw <= room-gutter-tw {
		return vw
	}
	return min(vw, room)
}

// fitValue cuts a value that does not fit. A path keeps its end, where the folder names
// that tell Workspaces apart are; other values keep their start.
func fitValue(label, value string, room int) string {
	if xansi.StringWidth(value) <= room || room < 2 {
		return value
	}
	if pathLabel(label) {
		return "…" + xansi.TruncateLeft(value, xansi.StringWidth(value)-room+1, "")
	}
	return xansi.Truncate(value, room, "…")
}

// pathLabel reports whether a row's value is a path, cut at its start rather than wrapped.
func pathLabel(label string) bool {
	switch label {
	case LabelWorkspace, LabelDownloads, LabelPlugin:
		return true
	}
	return false
}
