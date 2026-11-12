package ui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/store"
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
	// boxed reports whether the mode is a pop-up whose border carries its own keys, so
	// the key line shows context instead of repeating them.
	boxed() bool
}

// overlayOpen reports whether a mode that hides the view's keys owns the keyboard. The
// help pop-up is excluded: it lists paging controls, so it lists the view's keys.
func (m model) overlayOpen() bool { return m.overlay != nil && !m.helpOpen() }

func (m model) helpOpen() bool {
	_, ok := m.overlay.(*helpMode)
	return ok
}

func (m model) searching() bool {
	_, ok := m.overlay.(*searchMode)
	return ok
}

// boxOpen reports whether a pop-up whose bottom border carries its own keys is open.
func (m model) boxOpen() bool { return m.overlay != nil && m.overlay.boxed() }

// overlayHints are the active mode's keys, or nil when none is open.
func (m model) overlayHints() [][2]string {
	if m.overlay == nil {
		return nil
	}
	return m.overlay.Keys()
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

func (fm *formMode) Keys() [][2]string {
	return [][2]string{{"tab", hintNext}, {"←→", "change"}, {"enter", hintSave}, {"esc", "cancel"}}
}

func (fm *formMode) boxed() bool { return true }

func (fm *formMode) View(m *model, p pal) string { return m.formBox(p, fm, popForm) }

// Update runs the form. Nothing is written until it completes.
func (fm *formMode) Update(m *model, msg tea.Msg) (mode, tea.Cmd) {
	if paste, ok := msg.(tea.PasteMsg); ok {
		fm.form.paste(paste.Content)
		return fm, nil
	}
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return fm, nil
	}
	if k.String() == "esc" {
		return nil, nil
	}
	if k.String() == "enter" && fm.wantsFetch(m) {
		if m.capturedRun {
			fm.form.fetchErr = "A fetch is already running"
			if m.building != nil {
				fm.form.fetchErr = "A build is already running"
			}
			return fm, nil
		}
		return fm.startFetch(m)
	}
	if !fm.form.key(k) {
		return fm, nil
	}
	f := fm.form
	v := f.addValues()
	if fm.editOf != nil {
		return nil, m.saveEdit(*fm.editOf, v)
	}
	fields := createFields(v)
	if fields.URL != "" {
		if matches, err := m.store.FindByURL(fields.URL); err != nil {
			m.err = err
			return nil, nil
		} else if len(matches) > 0 {
			return &duplicateMode{existing: matches[0], fields: fields}, nil
		}
	}
	jd := ""
	if f.fetched {
		jd = f.jd
	}
	return nil, m.createLead(fields, jd)
}

// moveMode is the Move pop-up: every Status but the current one, likely next steps first.
type moveMode struct {
	app     store.Application
	options []string
	sel     int
}

func (mv *moveMode) Keys() [][2]string {
	return [][2]string{{"letter", "move"}, {"↑↓ enter", "pick"}, {"esc", "cancel"}}
}

func (mv *moveMode) boxed() bool { return true }

func (mv *moveMode) View(m *model, p pal) string { return m.moveBox(p, mv) }

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
	return [][2]string{{"y", "mark Applied"}, {"n esc", "cancel"}}
}

func (cm *confirmMode) boxed() bool { return true }

func (cm *confirmMode) View(m *model, p pal) string { return m.confirmBox(p, cm) }

func (cm *confirmMode) Update(m *model, msg tea.Msg) (mode, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return cm, nil
	}
	switch k.String() {
	case "y":
		m.move(cm.app, cm.status, false)
		return nil, nil
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

func (dm *downloadMode) boxed() bool { return true }

func (dm *downloadMode) View(m *model, p pal) string { return m.overwriteBox(p, dm) }

func (dm *downloadMode) Update(m *model, msg tea.Msg) (mode, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return dm, nil
	}
	return m.answerDownload(dm.state, k.String())
}

// duplicateMode warns that the URL is already saved.
type duplicateMode struct {
	existing store.Application
	fields   store.CreateFields
}

func (dm *duplicateMode) Keys() [][2]string {
	return [][2]string{{"o", "open that one"}, {"s", "save anyway"}, {"esc", "cancel"}}
}

func (dm *duplicateMode) boxed() bool { return true }

func (dm *duplicateMode) View(m *model, p pal) string { return m.duplicateBox(p, dm) }

func (dm *duplicateMode) Update(m *model, msg tea.Msg) (mode, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return dm, nil
	}
	switch k.String() {
	case "o", "enter":
		m.selectFolder(dm.existing.Folder)
		return nil, nil
	case "s":
		return nil, m.createLead(dm.fields, "")
	case "esc":
		return nil, nil
	}
	return dm, nil
}

// buildFailMode shows a failed build's own output, until esc.
type buildFailMode struct{ failure *buildFailure }

func (bf *buildFailMode) Keys() [][2]string { return [][2]string{{"esc", hintClose}} }

func (bf *buildFailMode) boxed() bool { return true }

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

// helpMode pages the help; other keys close it.
type helpMode struct{ page int }

func (hm *helpMode) Keys() [][2]string { return [][2]string{{"↑↓", "page"}, {"esc", hintClose}} }

func (hm *helpMode) boxed() bool { return false }

func (hm *helpMode) View(m *model, p pal) string { return m.helpBox(p) }

func (hm *helpMode) Update(m *model, msg tea.Msg) (mode, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch k.String() {
		case "down", "j", "pgdown":
			hm.page++
		case "up", "k", "pgup":
			hm.page = max(0, hm.page-1)
		default:
			return nil, nil
		}
	}
	return hm, nil
}

// searchMode is the `/` filter input. It has no box: the input shows on the status row.
type searchMode struct{}

func (sm *searchMode) Keys() [][2]string {
	return [][2]string{{"enter", "apply"}, {"esc", "clear"}}
}

func (sm *searchMode) boxed() bool { return false }

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
	hintNext   = "next"
	hintSave   = "save"
	hintReview = "review"
)
