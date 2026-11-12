package ui

import (
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/lukebrown14-code/rolecall/store"
)

// renameKey opens the rename pop-up on r over a Track row. handled is false anywhere else,
// so r keeps meaning rescan.
func (m model) renameKey(k tea.KeyPressMsg) (next model, handled bool) {
	st := m.settings
	if k.String() != "r" || st == nil || st.ws.Err != nil || st.focus != settingsRows || st.group != groupTracking ||
		st.row >= len(st.ws.Tracks) {
		return m, false
	}
	old := st.ws.Tracks[st.row]
	m.overlay = &renameMode{old: old, name: old}
	return m, true
}

const (
	renameName = iota
	renameConfirm
	renameReport
)

// renameMode renames a Track across the Workspace in three steps: ask for the name, confirm
// the reach, then (only when files were skipped) report what the Owner must fix by hand.
type renameMode struct {
	old, name string
	err       string
	stage     int
	impact    store.RenameImpact
	result    store.RenameResult
}

func (rm *renameMode) boxed() bool { return true }

func (rm *renameMode) Keys() [][2]string {
	switch rm.stage {
	case renameConfirm:
		return [][2]string{{"y enter", "rename"}, {"n esc", "cancel"}}
	case renameReport:
		return [][2]string{{"esc", "close"}}
	}
	return [][2]string{{"enter", hintReview}, {"esc", "cancel"}}
}

func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

func (rm *renameMode) View(m *model, p pal) string {
	var body string
	switch rm.stage {
	case renameName:
		body = p.text(p.dim).Render("Name  ") + p.text(p.fg).Render(rm.name) + p.text(p.accent).Render("▏")
		if rm.err != "" {
			body += "\n" + p.text(p.bad).Render("✗ "+rm.err)
		}
	case renameConfirm:
		roles := "roles.json will not change."
		if rm.impact.Roles {
			roles = "roles.json will change."
		}
		text := fmt.Sprintf("Rename %s to %s.\n%s will change. %s\nFolders stay where they are.",
			rm.old, rm.name, plural(rm.impact.Applications, "Application", "Applications"), roles)
		body = lipgloss.NewStyle().Foreground(p.fg).Width(popSmall-popInset).Render(text) + "\n\n" +
			p.text(p.warn).Render("This can't be undone with u.")
	default:
		lines := []string{fmt.Sprintf("Renamed %s to %s.", rm.old, rm.name),
			plural(len(rm.result.Skipped), "malformed meta.yml", "malformed meta.yml files") + " skipped, fix by hand:"}
		for _, s := range rm.result.Skipped {
			lines = append(lines, "  "+s)
		}
		body = lipgloss.NewStyle().Foreground(p.fg).Width(popSmall - popInset).Render(strings.Join(lines, "\n"))
	}
	return popup(p, "Rename Track", rm.stage != renameName, popSmall, m.viewWidth(), body, p.keys(rm.Keys()))
}

func (rm *renameMode) Update(m *model, msg tea.Msg) (mode, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.PasteMsg:
		if rm.stage == renameName {
			rm.name += pasted(msg.Content)
			rm.err = ""
		}
	case tea.KeyPressMsg:
		switch rm.stage {
		case renameName:
			return rm.nameKey(m, msg)
		case renameConfirm:
			switch msg.String() {
			case "y", "enter":
				return rm.apply(m)
			case "n", "esc":
				return nil, nil
			}
		default:
			if msg.String() == "esc" || msg.String() == "enter" {
				return nil, nil
			}
		}
	}
	return rm, nil
}

func (rm *renameMode) nameKey(m *model, k tea.KeyPressMsg) (mode, tea.Cmd) {
	switch {
	case k.String() == "esc":
		return nil, nil
	case k.Code == tea.KeyBackspace:
		rm.name = dropLastGrapheme(rm.name)
		rm.err = ""
	case k.String() == "enter":
		name := strings.TrimSpace(rm.name)
		if name == rm.old {
			return nil, nil
		}
		others := slices.DeleteFunc(slices.Clone(m.settings.ws.Tracks), func(t string) bool { return t == rm.old })
		if err := validTrackName(name, others); err != nil {
			rm.err = err.Error()
			return rm, nil
		}
		impact, err := m.store.TrackRenameImpact(rm.old)
		if err != nil {
			m.err = err
			return nil, nil
		}
		rm.name, rm.impact, rm.stage = name, impact, renameConfirm
	default:
		rm.name += k.Text
		rm.err = ""
	}
	return rm, nil
}

// apply runs the rename, then re-reads so the header filter, Applications and Settings follow.
func (rm *renameMode) apply(m *model) (mode, tea.Cmd) {
	res, err := m.store.RenameTrack(rm.old, rm.name)
	m.undo = nil // a rename is not a move: u must not reach back across it
	m.rescanAll()
	*m = m.loadSettings()
	if err != nil {
		m.err = err
		return nil, nil
	}
	rm.result = res
	if len(res.Skipped) > 0 {
		rm.stage = renameReport
		return rm, nil
	}
	m.toast = fmt.Sprintf("Renamed %s to %s (%s)", rm.old, rm.name, plural(res.Applications, "Application", "Applications"))
	return nil, nil
}
