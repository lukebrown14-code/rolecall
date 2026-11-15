package ui_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

var menuSizes = [][2]int{{120, 36}, {60, 16}}

// breadcrumbLine is the line of the screen that carries the "›" breadcrumb.
func breadcrumbLine(view string) string {
	for _, l := range strings.Split(chipped(view), "\n") {
		if strings.Contains(l, "›") {
			return l
		}
	}
	return ""
}

// menuBoxWidth is the widest rounded box on screen, 0 when there is none.
func menuBoxWidth(view string) int {
	w := 0
	for _, l := range strings.Split(chipped(view), "\n") {
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
				t.Fatalf("menu box width %d, want 1..28:\n%s", w, chipped(open))
			}
			if breadcrumbLine(before) == "" || breadcrumbLine(open) != breadcrumbLine(before) {
				t.Fatalf("menu covers the breadcrumb:\nbefore %q\nafter  %q", breadcrumbLine(before), breadcrumbLine(open))
			}
			// The pop-up template: the title in the top border, esc with the other keys in
			// the bottom one.
			top, bottom := "", ""
			for _, l := range strings.Split(chipped(open), "\n") {
				if strings.Contains(l, "╭") {
					top = l
				}
				if strings.Contains(l, "╰") {
					bottom = l
				}
			}
			if !strings.Contains(top, "Menu") || strings.Contains(top, "esc") || !strings.Contains(bottom, "esc close") {
				t.Fatalf("menu borders break the template: top %q bottom %q", top, bottom)
			}
			if strings.Contains(chipped(open), "Close") {
				t.Fatalf("menu has a Close row:\n%s", chipped(open))
			}
			// One cursor: the selection gutter on one row, and no second ▸ beside it.
			body := strings.SplitN(chipped(open), "\n", 3)[2]
			if got := markedLines(strings.ReplaceAll(body, breadcrumbLine(open), ""), "▌"); len(got) != 1 {
				t.Fatalf("want one ▌ cursor, got %d:\n%s", len(got), chipped(open))
			}
			if strings.Contains(body, "▌ ▸") {
				t.Fatalf("the menu draws two cursor marks:\n%s", chipped(open))
			}
			if driveModel(t, m, " ", "esc").View().Content != before {
				t.Fatal("esc did not restore the screen")
			}
		})
	}
}

func TestMenuEnterRunsTheItemUnderTheCursor(t *testing.T) {
	m := driveStart(t, fixtureCopy(t, nil), heldNow, 120, 36)
	got := chipped(driveModel(t, m, " ", "down", "down", "enter").View().Content)
	if !strings.Contains(got, "Applied ›") || strings.Contains(got, "╭") {
		t.Fatalf("third item should open the Applied screen and close the menu:\n%s", got)
	}
}

// menuItems is the menu's item lines (label and right-hand key) at 120x36.
func menuItems(t *testing.T) []string {
	t.Helper()
	m := driveStart(t, fixtureCopy(t, nil), heldNow, 120, 36)
	var items []string
	for _, l := range strings.Split(chipped(driveModel(t, m, " ").View().Content), "\n") {
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
			via := chipped(driveModel(t, m, append(keys, "enter")...).View().Content)
			direct := chipped(driveModel(t, m, key).View().Content)
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
	got := chipped(driveModel(t, m, append(keys, "enter")...).View().Content)
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

// menuLabels is the item labels and keys of the menu open in view.
func menuLabels(view string) [][2]string {
	var out [][2]string
	for _, l := range strings.Split(chipped(view), "\n") {
		parts := strings.Split(l, "│")
		if len(parts) < 3 {
			continue
		}
		f := strings.Fields(strings.TrimLeft(strings.TrimSpace(parts[1]), "▌▸ "))
		if len(f) < 2 {
			continue
		}
		out = append(out, [2]string{strings.Join(f[:len(f)-1], " "), f[len(f)-1]})
	}
	return out
}

// TestEveryMenuItemRunsWhatItsLabelSaysOnEveryScreen opens the menu on every screen,
// the detail view and each Settings group, and runs each item: its label must name what
// it did there, whatever its letter means on that screen (a adds a Track in Settings ›
// Tracking, so Add Application must not be offered there).
func TestEveryMenuItemRunsWhatItsLabelSaysOnEveryScreen(t *testing.T) {
	places := [][]string{{"1"}, {"1", "enter"}, {"2"}, {"3"}, {"4"}, {"5"}, {"6"}, {"6", "right"},
		{"6", "right", "right"}, {"6", "right", "right", "right"}}
	screens := []string{"Leads", "Drafts", "Applied", "Stats", "Profile", "Settings"}
	root := fixtureCopy(t, nil)
	for _, place := range places {
		t.Run(strings.Join(place, " "), func(t *testing.T) {
			m := driveModel(t, driveStart(t, root, heldNow, 120, 36), place...)
			before := chipped(m.View().Content)
			items := menuLabels(driveModel(t, m, " ").View().Content)
			if len(items) < len(screens) {
				t.Fatalf("menu lists %q", items)
			}
			for i, it := range items {
				label, key := it[0], it[1]
				keys := []string{" "}
				for range i {
					keys = append(keys, "down")
				}
				got := chipped(driveModel(t, m, append(keys, "enter")...).View().Content)
				if direct := chipped(driveModel(t, m, key).View().Content); got != direct {
					t.Fatalf("%q ran something other than its key %q", label, key)
				}
				var ok bool
				switch label {
				case "Add Application":
					ok = strings.Contains(got, "Add a Lead")
				case "Command palette":
					ok = strings.Contains(got, "╭─ : ")
				case "Help":
					ok = strings.Contains(got, "Keys ·")
				case "Search", "Switch view", "Track filter":
					// They change the list in place: no pop-up opens, and the screen changes.
					ok = !strings.Contains(got, "╭") && got != before
				case "Rescan":
					ok = !strings.Contains(got, "╭")
				default:
					if !slices.Contains(screens, label) {
						t.Fatalf("unexpected menu item %q", label)
					}
					ok = strings.Contains(breadcrumbLine(got), label)
				}
				if !ok {
					t.Fatalf("%q (%s) did not do what it says:\n%s", label, key, got)
				}
			}
		})
	}
}
