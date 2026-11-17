package ui

import (
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/internal/store"
)

// Bulk actions. `*` marks the row under the cursor (and again unmarks it); with rows marked,
// x dismisses and m moves all of them, after a confirmation that names the count, and one u
// undoes the lot. Marks belong to the view they were made in: another screen, view or an esc
// clears them, so a bulk action never reaches a row the Owner cannot see.

// keyMark marks or unmarks the row under the cursor.
const keyMark = "*"

// markSet is the folders marked in one view.
type markSet struct {
	screen, table int
	folders       map[string]bool
}

// markedHere is the marked rows of the view on screen, in display order. Marks made in
// another view are not here.
func (m model) markedHere() []store.Application {
	if m.marks == nil || m.detail != nil || !m.hasTables() || m.marks.screen != m.screen || m.marks.table != m.table {
		return nil
	}
	var out []store.Application
	for _, a := range m.rows() {
		if m.marks.folders[a.Folder] {
			out = append(out, a)
		}
	}
	return out
}

// isMarked reports whether folder's row is marked in the view on screen.
func (m model) isMarked(folder string) bool {
	return m.marks != nil && m.marks.screen == m.screen && m.marks.table == m.table && m.marks.folders[folder]
}

// toggleMark marks or unmarks the row under the cursor.
func (m *model) toggleMark() {
	a, ok := m.selected()
	if !ok {
		return
	}
	if m.marks == nil || m.marks.screen != m.screen || m.marks.table != m.table {
		m.marks = &markSet{screen: m.screen, table: m.table, folders: map[string]bool{}}
	}
	if m.marks.folders[a.Folder] {
		delete(m.marks.folders, a.Folder)
	} else {
		m.marks.folders[a.Folder] = true
	}
	if n := len(m.markedHere()); n > 0 {
		m.toast, m.toastKind = fmt.Sprintf("%d marked · x dismiss · m move · esc clear", n), toastNote
	}
}

// clearMarks drops every mark, and reports whether there were any to drop.
func (m *model) clearMarks() bool {
	had := len(m.markedHere()) > 0
	m.marks = nil
	return had
}

// markCount is the marked rows in view, for the breadcrumb and the keys.
func (m model) markCount() int { return len(m.markedHere()) }

// applications is a count in words: "1 Application", "3 Applications".
func applications(n int) string {
	if n == 1 {
		return "1 Application"
	}
	return strconv.Itoa(n) + " Applications"
}

// bulkMode asks before moving every marked Application to one Status. It names how many
// move, which are skipped and which will be sent without Ready Documents.
type bulkMode struct {
	apps    []store.Application // those that will move
	status  string
	skipped int // marked rows that do not apply (already there, or malformed, or not a Lead or Draft)
	notRdy  int // moving to Applied without Ready Documents
}

// newBulk builds the confirmation for moving the marked rows to status. Rows already at the
// Status, malformed rows and, for Dismissed, anything past Draft are left out.
func (m model) newBulk(marked []store.Application, status string) *bulkMode {
	bm := &bulkMode{status: status}
	for _, a := range marked {
		switch {
		case a.Malformed != nil, a.Status == status, status == store.StatusDismissed && !a.Unsent():
			bm.skipped++
		default:
			bm.apps = append(bm.apps, a)
			if status == store.StatusApplied && a.Documents != store.DocsReady {
				bm.notRdy++
			}
		}
	}
	return bm
}

func (bm *bulkMode) Keys() [][2]string {
	return [][2]string{{"y", bm.verb()}, {keyNoEsc, "cancel"}}
}

// verb is what y does: "dismiss 3", "move 3".
func (bm *bulkMode) verb() string {
	if bm.status == store.StatusDismissed {
		return "dismiss " + strconv.Itoa(len(bm.apps))
	}
	return "move " + strconv.Itoa(len(bm.apps))
}

func (bm *bulkMode) View(m *model, p pal) string { return m.bulkBox(p, bm) }

func (bm *bulkMode) Update(m *model, msg tea.Msg) (mode, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return bm, nil
	}
	switch k.String() {
	case "y":
		cmd, moved := m.moveMany(bm.apps, bm.status)
		if (bm.status == store.StatusRejected || bm.status == store.StatusWithdrawn) && len(moved) > 0 && m.err == nil {
			// One reason for every Application that ended; skip or esc leaves them as moved.
			return m.closeReasonPopup(moved, bm.status), cmd
		}
		return nil, cmd
	case "n", "esc":
		return nil, nil
	}
	return bm, nil
}

// bulkAction opens the confirmation for moving the marked rows to status, or says why there
// is nothing to do.
func (m *model) bulkAction(status string) mode {
	bm := m.newBulk(m.markedHere(), status)
	if len(bm.apps) == 0 {
		m.toast, m.toastKind = fmt.Sprintf("Nothing to move to %s · %d marked already there or can't be", status, bm.skipped), toastNote
		if status == store.StatusDismissed {
			m.toast = "x: none of the marked can be dismissed (only Leads and Drafts) · m to move"
		}
		return nil
	}
	return bm
}

// moveMany moves each Application to status, as one action: the toast counts them and one u
// puts every one back. It returns those that moved. It stops at the first failure, which stays on the feedback line, and
// what moved before it stays one undoable group.
func (m *model) moveMany(apps []store.Application, status string) (tea.Cmd, []store.Application) {
	start := len(m.undo)
	var cmds []tea.Cmd
	done := 0
	var moved []store.Application
	for _, a := range apps {
		// A move to Applied was asked for once, for all of them: no per-row prompt.
		cmd := m.move(a, status, false)
		if m.err != nil {
			break
		}
		if len(m.undo) > start+done { // moved (not "already there")
			done++
			moved = append(moved, a)
		}
		cmds = append(cmds, cmd)
	}
	if done > 1 {
		group := append([]undoMove(nil), m.undo[start:]...)
		m.undo = append(m.undo[:start], undoMove{batch: group})
	}
	m.marks = nil
	if done > 0 {
		verb := "Moved to " + status
		if status == store.StatusDismissed {
			verb = "Dismissed"
		}
		m.toast, m.toastHint, m.movedTo, m.toastKind = fmt.Sprintf("%s · %s", verb, applications(done)), undoHint, status, toastPlain
	}
	return tea.Batch(cmds...), moved
}

// undoBatch puts every move of a bulk action back, newest first, and says how many.
func (m *model) undoBatch(group []undoMove) tea.Cmd {
	m.undo = m.undo[:len(m.undo)-1]
	m.undo = append(m.undo, group...)
	var cmds []tea.Cmd
	back := 0
	for range group {
		cmds = append(cmds, m.undoLast())
		if m.err != nil {
			break
		}
		if m.toastKind == toastUndo {
			back++
		}
	}
	if m.err == nil {
		m.toast, m.toastKind, m.toastHint = fmt.Sprintf("Undone · %s moved back", applications(back)), toastUndo, ""
		if back < len(group) {
			m.toast += fmt.Sprintf(" · %d changed since", len(group)-back)
		}
	}
	return tea.Batch(cmds...)
}

// bulkSubject is the Move pop-up's subject for several rows: how many, and from which
// Statuses.
func bulkSubject(apps []store.Application) (title, now string) {
	count := map[string]int{}
	var order []string
	for _, a := range apps {
		if count[a.Status] == 0 {
			order = append(order, a.Status)
		}
		count[a.Status]++
	}
	parts := make([]string, len(order))
	for i, s := range order {
		parts[i] = fmt.Sprintf("%s %d", s, count[s])
	}
	return applications(len(apps)) + " marked", strings.Join(parts, " · ")
}

// bulkOptions are the Statuses the Move pop-up offers for several rows: those some marked
// row is not already at, in the Status order. With one Status among them it is that Status's
// usual order.
func bulkOptions(apps []store.Application) []string {
	same := true
	for _, a := range apps {
		same = same && a.Status == apps[0].Status
	}
	if same {
		return MoveOptions(apps[0].Status, statusBeforeDismissed(apps[0].StatusLog))
	}
	var out []string
	for _, s := range store.Statuses {
		for _, a := range apps {
			if a.Status != s {
				out = append(out, s)
				break
			}
		}
	}
	return out
}
