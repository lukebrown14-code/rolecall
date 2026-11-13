package ui_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

var menuSizes = [][2]int{{120, 36}, {60, 16}}

// breadcrumbLine is the line of the screen that carries the "›" breadcrumb.
func breadcrumbLine(view string) string {
	for _, l := range strings.Split(ansi.Strip(view), "\n") {
		if strings.Contains(l, "›") {
			return l
		}
	}
	return ""
}

// menuBoxWidth is the widest rounded box on screen, 0 when there is none.
func menuBoxWidth(view string) int {
	w := 0
	for _, l := range strings.Split(ansi.Strip(view), "\n") {
		r := []rune(l)
		for i, c := range r {
			if c != '╭' {
				continue
			}
			for j := i + 1; j < len(r); j++ {
				if r[j] == '╮' {
					w = max(w, j-i+1)
					break
				}
			}
		}
	}
	return w
}

func TestSpaceOpensTheWorkspaceMenuWithoutCoveringTheFilters(t *testing.T) {
	for _, size := range menuSizes {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			m := driveStart(t, fixtureCopy(t, nil), heldNow, size[0], size[1])
			before := m.View().Content
			open := driveModel(t, m, " ").View().Content
			if w := menuBoxWidth(open); w == 0 || w > 28 {
				t.Fatalf("menu box width %d, want 1..28:\n%s", w, ansi.Strip(open))
			}
			if breadcrumbLine(before) == "" || breadcrumbLine(open) != breadcrumbLine(before) {
				t.Fatalf("menu covers the breadcrumb:\nbefore %q\nafter  %q", breadcrumbLine(before), breadcrumbLine(open))
			}
			top := ""
			for _, l := range strings.Split(ansi.Strip(open), "\n") {
				if strings.Contains(l, "╭") {
					top = l
				}
			}
			if !strings.Contains(top, "esc") {
				t.Fatalf("top border lacks esc: %q", top)
			}
			if strings.Contains(ansi.Strip(open), "Close") {
				t.Fatalf("menu has a Close row:\n%s", ansi.Strip(open))
			}
			if got := markedLines(strings.ReplaceAll(ansi.Strip(open), breadcrumbLine(open), ""), "▸"); len(got) != 1 {
				t.Fatalf("want one ▸ cursor, got %d:\n%s", len(got), ansi.Strip(open))
			}
			if driveModel(t, m, " ", "esc").View().Content != before {
				t.Fatal("esc did not restore the screen")
			}
		})
	}
}

func TestMenuEnterRunsTheItemUnderTheCursor(t *testing.T) {
	m := driveStart(t, fixtureCopy(t, nil), heldNow, 120, 36)
	got := ansi.Strip(driveModel(t, m, " ", "down", "down", "enter").View().Content)
	if !strings.Contains(got, "Applied ›") || strings.Contains(got, "╭") {
		t.Fatalf("third item should open the Applied screen and close the menu:\n%s", got)
	}
}

// menuItems is the menu's item lines (label and right-hand key) at 120x36.
func menuItems(t *testing.T) []string {
	t.Helper()
	m := driveStart(t, fixtureCopy(t, nil), heldNow, 120, 36)
	var items []string
	for _, l := range strings.Split(ansi.Strip(driveModel(t, m, " ").View().Content), "\n") {
		parts := strings.Split(l, "│")
		if len(parts) >= 3 && strings.TrimSpace(parts[1]) != "" {
			items = append(items, strings.TrimSpace(parts[1]))
		}
	}
	return items
}

func TestMenuItemsShowTheirKeyAndThePressedKeyDoesTheSame(t *testing.T) {
	items := menuItems(t)
	m := driveStart(t, fixtureCopy(t, nil), heldNow, 120, 36)
	checked := 0
	for i, it := range items {
		f := strings.Fields(strings.TrimLeft(it, "▌▸ "))
		key := f[len(f)-1]
		if len(f) < 2 {
			t.Fatalf("item %q shows no key", it)
		}
		switch key {
		case "a", "/", "s", ":", "?":
			keys := []string{" "}
			for range i {
				keys = append(keys, "down")
			}
			via := ansi.Strip(driveModel(t, m, append(keys, "enter")...).View().Content)
			direct := ansi.Strip(driveModel(t, m, key).View().Content)
			if via != direct {
				t.Fatalf("item %q != key %q:\nvia menu:\n%s\ndirect:\n%s", it, key, via, direct)
			}
			checked++
		}
	}
	if checked < 3 {
		t.Fatalf("checked only %d items from %q", checked, items)
	}
}

func TestPaletteMenuItemOpensThePalette(t *testing.T) {
	items := menuItems(t)
	idx := -1
	for i, it := range items {
		if strings.HasSuffix(it, " :") {
			idx = i
		}
	}
	if idx < 0 {
		t.Fatalf("no palette item in %q", items)
	}
	keys := []string{" "}
	for range idx {
		keys = append(keys, "down")
	}
	m := driveStart(t, fixtureCopy(t, nil), heldNow, 120, 36)
	got := ansi.Strip(driveModel(t, m, append(keys, "enter")...).View().Content)
	if !strings.Contains(got, "╭─ : ") {
		t.Fatalf("palette not open:\n%s", got)
	}
}

func TestCapitalMDoesNothing(t *testing.T) {
	m := driveStart(t, fixtureCopy(t, nil), heldNow, 120, 36)
	if driveModel(t, m, "M").View().Content != m.View().Content {
		t.Fatal("M changed the screen")
	}
}
