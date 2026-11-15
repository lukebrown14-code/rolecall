package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/internal/store"
	"github.com/lukebrown14-code/rolecall/internal/ui/forms"
)

// mode is one overlay that owns the keyboard: a pop-up, a prompt or the search input.
// model.Update routes every message to the active mode, so adding a pop-up means writing
// one type — its Update, View and Keys — instead of editing overlayOpen, overlayBox,
// overlayHints and a branch of Update.
type mode interface {
	// Update handles one message and returns the next mode (nil closes it) and a command.
	Update(m *model, msg tea.Msg) (mode, tea.Cmd)
	// View draws the pop-up, or "" for a mode with no box (the search input).
	View(m *model, p pal) string
	// Keys are the mode's keys: the pop-up's bottom border, or the search hints.
	Keys() [][2]string
}

func (m model) searching() bool {
	_, ok := m.overlay.(*searchMode)
	return ok
}

// overlayView is the active mode's box, or "" when none is open.
func (m model) overlayView(p pal) string {
	if m.overlay == nil {
		return ""
	}
	return m.overlay.View(&m, p)
}

// hintClose is the "close" hint shared by the pop-ups a single key dismisses.
const hintClose = "close"

// formMode is the Add/Edit pop-up.
type formMode struct {
	form   *form
	editOf *store.Application
}

func (fm *formMode) Keys() [][2]string { return fm.form.Keys() }

func (fm *formMode) View(m *model, p pal) string { return m.formBox(p, fm.form, popForm) }

// Update runs the form. Nothing is written until it completes.
func (fm *formMode) Update(m *model, msg tea.Msg) (mode, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok && k.String() == "enter" && !fm.form.Discarding && fm.wantsFetch(m) {
		if m.capturedRun {
			fm.form.FetchErr = "A fetch is already running"
			if m.building != nil {
				fm.form.FetchErr = "A build is already running"
			}
			return fm, nil
		}
		return fm.startFetch(m)
	}
	switch fm.form.Update(msg) {
	case formOpen:
		return fm, nil
	case formCancel:
		return nil, nil
	}
	f := fm.form
	v := readAddValues(f)
	if fm.editOf != nil {
		return nil, m.saveEdit(*fm.editOf, v)
	}
	fields := createFields(v)
	if fields.URL != "" {
		if matches, err := m.store.FindByURL(fields.URL); err != nil {
			f.Focused().Err = m.errorText(err)
			return fm, nil
		} else if len(matches) > 0 {
			return &duplicateMode{existing: duplicateOf(matches), others: len(matches) - 1, fields: fields, back: fm}, nil
		}
	}
	return fm.submitAdd(m, fields)
}

// moveMode is the Move pop-up: every Status but the current one, likely next steps first.
type moveMode struct {
	app     store.Application
	options []string
	sel     int
}

// Keys name the first letters on offer, so the letter keys read as keys: "d/a/x… move".
func (mv *moveMode) Keys() [][2]string {
	const shown = 3
	var letters []string
	for _, s := range mv.options[:min(shown, len(mv.options))] {
		letters = append(letters, statusLetters[s])
	}
	label := strings.Join(letters, "/")
	if len(mv.options) > shown {
		label += "…"
	}
	return [][2]string{{label, "move"}, {"↑↓ enter", "pick"}, {"esc", "cancel"}}
}

func (mv *moveMode) View(m *model, p pal) string { return m.moveBox(p, mv) }

// pinned keeps the subject, title then "Company · now Status", in view when the pop-up
// scrolls on a short terminal: which Application moves matters more than the options.
func (mv *moveMode) pinned() int { return moveSubjectLines }

// moveSubjectLines is the Move pop-up's subject: the title, then Company and Status.
const moveSubjectLines = 2

// pinnedMode is a pop-up whose first body lines stay put while the rest scrolls.
type pinnedMode interface{ pinned() int }

// Update moves the Application the pop-up shows, not whatever row the cursor is on now: a
// live refresh can move the cursor while the pop-up is open.
func (mv *moveMode) Update(m *model, msg tea.Msg) (mode, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return mv, nil
	}
	switch key := k.String(); key {
	case "esc":
		return nil, nil
	case "down", "j":
		mv.sel = min(mv.sel+1, len(mv.options)-1)
	case "up", "k":
		mv.sel = max(mv.sel-1, 0)
	case "enter":
		return m.requestMove(mv.app, mv.options[mv.sel])
	default:
		for _, status := range mv.options {
			if statusLetters[status] == key {
				return m.requestMove(mv.app, status)
			}
		}
	}
	return mv, nil
}

// confirmMode asks before moving a not-Ready Application to Applied.
type confirmMode struct {
	app    store.Application
	status string
}

func (cm *confirmMode) Keys() [][2]string {
	return [][2]string{{"y", "mark Applied"}, {keyNoEsc, "cancel"}}
}

func (cm *confirmMode) View(m *model, p pal) string { return m.confirmBox(p, cm) }

func (cm *confirmMode) Update(m *model, msg tea.Msg) (mode, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return cm, nil
	}
	switch k.String() {
	case "y":
		return nil, m.move(cm.app, cm.status, false)
	case "n", "esc":
		return nil, nil
	}
	return cm, nil
}

// downloadMode is a download paused on a name clash.
type downloadMode struct{ state downloadState }

func (dm *downloadMode) Keys() [][2]string {
	return [][2]string{{"y", "overwrite"}, {"a", "all"}, {"n", "skip"}, {"esc", "stop"}}
}

func (dm *downloadMode) View(m *model, p pal) string { return m.overwriteBox(p, dm) }

func (dm *downloadMode) Update(m *model, msg tea.Msg) (mode, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return dm, nil
	}
	return m.answerDownload(dm.state, k.String())
}

// duplicateMode warns that the URL is already saved. back is the Add form it came from,
// kept whole so esc returns to it.
type duplicateMode struct {
	existing store.Application
	others   int // more saved Applications with the same URL
	fields   store.CreateFields
	back     *formMode
}

func (dm *duplicateMode) Keys() [][2]string {
	return [][2]string{{"o", "discard this, open that one"}, {"s", "save anyway"}, {"esc", "back to form"}}
}

func (dm *duplicateMode) View(m *model, p pal) string { return m.duplicateBox(p, dm) }

func (dm *duplicateMode) Update(m *model, msg tea.Msg) (mode, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return dm, nil
	}
	switch k.String() {
	case "o", "enter":
		m.selectFolder(dm.existing.Folder)
		m.openDetail(dm.existing.Folder)
		return nil, nil
	case "s":
		return dm.back.submitAdd(m, dm.fields)
	case "esc":
		// Back to the form with every value kept, on the URL, the field to change.
		f := dm.back.form
		for i := range f.Fields {
			if f.Fields[i].Role == forms.RoleURL {
				f.Focus = i
			}
		}
		return dm.back, nil
	}
	return dm, nil
}

// buildFailMode shows a failed build's own output, until esc.
type buildFailMode struct{ failure *buildFailure }

func (bf *buildFailMode) Keys() [][2]string { return [][2]string{{"esc", hintClose}} }

func (bf *buildFailMode) View(m *model, p pal) string { return m.buildFailBox(p, bf) }

func (bf *buildFailMode) Update(m *model, msg tea.Msg) (mode, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return bf, nil
	}
	if k.String() == "esc" {
		return nil, nil
	}
	return bf, nil
}

// helpMode pages the help; other keys close it. pages is set as it is drawn.
type helpMode struct{ page, pages int }

func (hm *helpMode) Keys() [][2]string { return helpModeKeys(hm.pages) }

func (hm *helpMode) View(m *model, p pal) string { return m.helpBox(p) }

// Update pages with ↑↓; any other key closes the help. esc, ? and q only close it; every
// other key also runs on the screen beneath, so "? m" opens Move as the help just said.
func (hm *helpMode) Update(m *model, msg tea.Msg) (mode, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch k.String() {
		case "down", "j", "pgdown":
			hm.page = min(hm.page+1, max(0, hm.pages-1))
		case "up", "k", "pgup":
			hm.page = max(0, hm.page-1)
		case "esc", "?", "q":
			return nil, nil
		default:
			return nil, func() tea.Msg { return k }
		}
	}
	return hm, nil
}

// searchMode is the `/` filter input. It has no box: the input shows on the status row.
type searchMode struct{}

func (sm *searchMode) Keys() [][2]string {
	return [][2]string{{"enter", "apply"}, {"esc", "clear"}}
}

func (sm *searchMode) View(m *model, p pal) string { return "" }

func (sm *searchMode) Update(m *model, msg tea.Msg) (mode, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.PasteMsg:
		m.text += pasted(msg.Content)
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			m.text = ""
			m.cursor = 0
			return nil, nil
		case "enter":
			m.cursor = 0
			return nil, nil
		case "backspace":
			m.text = dropLastGrapheme(m.text)
		default:
			if msg.Text != "" {
				m.text += msg.Text
			}
		}
	default:
		return sm, nil
	}
	m.cursor = 0
	return sm, nil
}

// Key-hint labels shared by the pop-up modes.
const (
	hintReview = "review"
)
