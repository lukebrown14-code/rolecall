package ui

import (
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/lukebrown14-code/rolecall/internal/store"
)

// renamePopup asks for a Track's new name; enter goes on to review what will change.
func (m model) renamePopup(old string) mode {
	f := newFormOf("Rename Track "+old, formField{Label: "New name", Required: true, Value: old})
	f.Submit = hintReview
	f.SelectValue() // typing replaces the old name
	return &formPopup{form: f, width: popSmall, save: func(m *model, f *form) (mode, error) {
		name := strings.TrimSpace(f.Value("New name"))
		if name == old {
			return nil, nil
		}
		others := slices.DeleteFunc(slices.Clone(m.settings.Ws.Tracks), func(t string) bool { return t == old })
		if err := validTrackName(name, others); err != nil {
			return nil, err
		}
		impact, err := m.store.TrackRenameImpact(old)
		if err != nil {
			m.err = err
			return nil, nil
		}
		return &renameMode{old: old, name: name, impact: impact, stage: renameConfirm}, nil
	}}
}

const (
	renameConfirm = iota + 1
	renameReport
)

// renameMode is a Track rename after the name is chosen: review what will change and
// confirm, then (only when files were skipped) report what the Owner must fix by hand.
type renameMode struct {
	old, name string
	stage     int
	impact    store.RenameImpact
	result    store.RenameResult
}

func (rm *renameMode) Keys() [][2]string {
	if rm.stage == renameReport {
		return [][2]string{{"esc", hintClose}}
	}
	return [][2]string{{"y enter", "rename"}, {keyNoEsc, "cancel"}}
}

func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

// renameImpactLine is how many Applications a rename rewrites, with the verb agreeing:
// "1 Application changes its Track", "3 Applications change their Track".
func renameImpactLine(n int) string {
	if n == 1 {
		return "1 Application changes its Track in meta.yml, archived ones included."
	}
	return fmt.Sprintf("%d Applications change their Track in meta.yml, archived ones included.", n)
}

func (rm *renameMode) View(m *model, p pal) string {
	w := popSmall - popInset
	para := func(s string) string {
		return lipgloss.NewStyle().Foreground(p.Fg).Width(w).Render(s)
	}
	var body string
	if rm.stage == renameConfirm {
		roles := "roles.json has no role on " + rm.old + "."
		if rm.impact.Roles {
			roles = "roles.json: its roles on " + rm.old + " move to " + rm.name + "."
		}
		body = strings.Join([]string{
			p.Text(p.Fg).Bold(true).Render(rm.old + " → " + rm.name),
			"",
			para(renameImpactLine(rm.impact.Applications)),
			para(roles),
			p.Text(p.Dim).Width(w).Render("Folders stay where they are, under jobs/" + rm.old + "/."),
			"",
			p.Text(p.Warn).Render("This can't be undone with u."),
		}, "\n")
	} else {
		lines := []string{fmt.Sprintf("Renamed %s to %s.", rm.old, rm.name),
			plural(len(rm.result.Skipped), "malformed meta.yml", "malformed meta.yml files") + " skipped, fix by hand:"}
		for _, s := range rm.result.Skipped {
			lines = append(lines, "  "+s)
		}
		body = para(strings.Join(lines, "\n"))
	}
	return popup(p, "Rename Track "+rm.old, true, popSmall, m.viewWidth(), body, p.Keys(rm.Keys()))
}

func (rm *renameMode) Update(m *model, msg tea.Msg) (mode, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return rm, nil
	}
	if rm.stage == renameConfirm {
		switch k.String() {
		case "y", "enter":
			return rm.apply(m)
		case "n", "esc":
			return nil, nil
		}
		return rm, nil
	}
	if k.String() == "esc" || k.String() == "enter" {
		return nil, nil
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
	*m = m.saved(fmt.Sprintf("Renamed %s to %s (%s)", rm.old, rm.name, plural(res.Applications, "Application", "Applications")),
		settingsFile, nil)
	return nil, nil
}
