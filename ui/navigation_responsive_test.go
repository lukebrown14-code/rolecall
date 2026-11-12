package ui_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestFilteredNavigationKeepsScreenNames(t *testing.T) {
	root := fixtureCopy(t, nil)
	for _, size := range [][2]int{{120, 36}, {80, 24}, {60, 16}} {
		for _, keys := range [][]string{{"/", "Orbit", "enter"}, {"t"}} {
			t.Run(fmt.Sprint(size, keys), func(t *testing.T) {
				m := driveStart(t, root, nil, size[0], size[1])
				m = driveModel(t, m, append([]string{"3"}, keys...)...)
				view := ansi.Strip(m.View().Content)
				line := strings.Split(view, "\n")[0]
				if keys[0] == "/" && !strings.Contains(view, "search Orbit") {
					t.Errorf("missing search context: %s", view)
				}
				if keys[0] == "t" && !strings.Contains(view, "track SWE") {
					t.Errorf("missing track context: %s", view)
				}
				for _, label := range []string{"Lead", "Draft", "Appl", "Stat"} {
					if !strings.Contains(line, label) {
						t.Errorf("navigation missing %q: %s", label, line)
					}
				}
				if ansi.StringWidth(line) > size[0] {
					t.Errorf("navigation exceeds width: %s", line)
				}
			})
		}
	}
}
