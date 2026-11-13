package ui

import "testing"

// The layout is a pure function of the terminal size: size class, detail mode, pane
// widths, the list's fixed right columns, filter lines and the footer row.
func TestSizeLayoutBreakpoints(t *testing.T) {
	cases := []struct {
		w       int
		class   sizeClass
		mode    detailMode
		filters int
	}{
		{55, 0, detailFull, 1},
		{56, 0, detailFull, 1},
		{59, 0, detailFull, 1},
		{60, 0, detailFull, 1},
		{71, 0, detailFull, 1},
		{72, classMedium, detailSheet, 2},
		{99, classMedium, detailSheet, 2},
		{100, classWide, detailPane, 2},
		{109, classWide, detailPane, 2},
		{110, classWide, detailPane, 2},
		{120, classWide, detailPane, 2},
	}
	for _, c := range cases {
		for _, h := range []int{14, 19, 20, 21, 36} {
			l := sizeLayout(c.w, h)
			if l.class != c.class || l.detail != c.mode || l.filterLines != c.filters {
				t.Errorf("%dx%d: class %v detail %v filters %d; want %v %v %d", c.w, h, l.class, l.detail, l.filterLines, c.class, c.mode, c.filters)
			}
			if l.flagCol != 6 || l.stageCol != 11 || l.fitCol != 6 {
				t.Errorf("%dx%d: columns %d/%d/%d; want 6/11/6", c.w, h, l.flagCol, l.stageCol, l.fitCol)
			}
			if l.subtitleW < 0 {
				t.Errorf("%dx%d: subtitle width %d is negative", c.w, h, l.subtitleW)
			}
			if l.footerRow != h-1 {
				t.Errorf("%dx%d: footer row %d; want %d", c.w, h, l.footerRow, h-1)
			}
			if l.short != (h <= 20) {
				t.Errorf("%dx%d: short = %v", c.w, h, l.short)
			}
			if c.mode == detailPane {
				if l.detailW < 40 || l.listW+1+l.detailW != c.w {
					t.Errorf("%dx%d: list %d detail %d do not fill the width", c.w, h, l.listW, l.detailW)
				}
			} else if l.listW != c.w || l.detailW != 0 {
				t.Errorf("%dx%d: single pane has list %d detail %d", c.w, h, l.listW, l.detailW)
			}
		}
	}
}

// A bottom sheet needs the list's top part and about 8 detail lines; shorter, the detail
// opens full-screen. Wide and narrow never use a sheet.
func TestSheetHeights(t *testing.T) {
	cases := []struct {
		w, h   int
		ok     bool
		listH  int
		detail int
	}{
		{80, 24, true, 6, 10},
		{80, 20, true, 5, 8},
		{80, 19, false, 0, 0},
		{80, 16, false, 0, 0},
		{60, 36, false, 0, 0},
		{120, 36, false, 0, 0},
	}
	for _, c := range cases {
		list, detail, ok := sizeLayout(c.w, c.h).sheetHeights(tableContext)
		if ok != c.ok {
			t.Errorf("%dx%d: sheet = %v; want %v", c.w, c.h, ok, c.ok)
		}
		if ok && (list != c.listH || detail != c.detail) {
			t.Errorf("%dx%d: list %d detail %d; want %d/%d", c.w, c.h, list, detail, c.listH, c.detail)
		}
	}
}
