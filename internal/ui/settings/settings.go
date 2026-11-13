// Package settings owns Settings screen data, navigation, and validation.
package settings

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/lukebrown14-code/rolecall/internal/config"
	"github.com/lukebrown14-code/rolecall/internal/store"
)

// Groups are the groups in display order; the Group* indices match this order.
var Groups = []string{"Search", "Documents", "Tracking", GroupMachineName}

// Group indices, in display order. A handler names its group with one of these rather
// than a bare integer, so inserting a group cannot silently move another's behaviour.
const (
	GroupSearch = iota
	GroupDocuments
	GroupTracking
	GroupMachine
)

const GroupMachineName = "This machine"

// Labels of the settings rows the code refers to.
const (
	LabelEditor    = "Editor"
	LabelFollowUp  = "Follow-up due after"
	LabelHarness   = "Harness"
	LabelWorkspace = "Workspace"
	LabelDownloads = "Downloads"
	LabelPlugin    = "Plugin"
	LabelASCII     = "ASCII glyphs"
	LabelAddTrack  = "+ add a Track"
)

// settingRow is one line in a group. Every editable value says where it comes from
// (source), an unset one shows an example instead, and a read-only row says why it is
// fixed, so "not set", "default" and "read-only" never read alike.
type Row struct {
	Label, Value string
	Unit         string // after the value: "days", "words"
	Source       string // the file or variable the value comes from, or "default"
	Example      string // shown while the value is unset
	ReadOnly     bool   // shown, never edited here; the cursor passes over it
	Reason       string // why a read-only row is fixed
	Warning      string // a problem with the value, e.g. an Editor not on PATH
	Section      string // a heading drawn above the row
	Action       bool   // "+ add a Track": no value, enter acts
}

// settingsState is the loaded workspace.json and where the Owner is. It is replaced, never
// mutated in place, so copies of the model stay independent.
type State[Undo any] struct {
	Ws       store.WorkspaceSettings
	TrackUse map[string]int // Applications on each Track, archived ones included
	Group    int
	Row      int
	Focus    int // settingsList or settingsRows
	Undo     *Undo
}

// selectable reports whether the cursor can rest on a row: everything but read-only rows.
func (r Row) Selectable() bool { return !r.ReadOnly }

// settingValueText is a row's value with its unit, as the value column shows it.
func (r Row) ValueText() string {
	if r.Value != "" && r.Unit != "" {
		return r.Value + " " + r.Unit
	}
	return r.Value
}

// provenance is what follows a row's value: why a read-only row is fixed, where a value
// comes from, or an example while it is unset.
func (r Row) Provenance() string {
	switch {
	case r.ReadOnly && r.Source != "":
		return r.Reason + " · " + r.Source
	case r.ReadOnly:
		return r.Reason
	case r.Value == "" && r.Example != "":
		return "example: " + r.Example
	}
	return r.Source
}

// tail is the source and warning after a row's value, as plain text, for sizing.
func (r Row) Tail() string {
	t := r.Provenance()
	if r.Warning != "" {
		t += WarnSep + r.Warning
	}
	return t
}

// SourceText is a source as the value's provenance reads: "from $EDITOR", "config.toml".
func SourceText(src string) string {
	switch src {
	case config.SourceVisual, config.SourceEditor, config.SourcePlugin, config.SourceFlag:
		return "from " + src
	}
	return src
}

// FileSource is a workspace.json value's source: the file, or the default it falls back to.
func FileSource(set bool) string {
	if set {
		return settingsFile
	}
	return config.SourceDefault
}

// RemoteLabel is search_area.remote as the Owner reads it; missing means the default.
func RemoteLabel(remote string) string {
	if l, ok := RemoteLabels[remote]; ok {
		return l
	}
	return RemoteLabels[store.RemoteInPlaces]
}

// SplitPlaces turns the Places answer into its list, dropping blanks.
func SplitPlaces(s string) ([]string, error) {
	var places []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			places = append(places, p)
		}
	}
	if len(places) == 0 {
		return nil, errors.New(fieldPlaces + " is required")
	}
	return places, nil
}

// ValidFollowUpDays is the follow-up rule, checked as the Owner leaves the field.
func ValidFollowUpDays(s string) error {
	days, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || days < minFollowUpDays || days > maxFollowUpDays {
		return errFollowUpDays
	}
	return nil
}

// GateValue is a Gate's current value.
func GateValue(ws store.WorkspaceSettings, key string) int {
	switch key {
	case store.GateLetterCap:
		return ws.LetterWordCap
	case store.GateLetterMin:
		return ws.LetterWordMin
	case store.GateSkillsMin:
		return ws.KeySkillsMin
	}
	return ws.KeySkillsMax
}

// GateRange is the bound the other Gate of the pair sets on this one: "min 150", "max 12".
func GateRange(ws store.WorkspaceSettings, key string) string {
	switch key {
	case store.GateLetterCap:
		return fmt.Sprintf("above the minimum, %d", ws.LetterWordMin)
	case store.GateLetterMin:
		return fmt.Sprintf("below the maximum, %d", ws.LetterWordCap)
	case store.GateSkillsMin:
		return fmt.Sprintf("at most the maximum, %d", ws.KeySkillsMax)
	}
	return fmt.Sprintf("at least the minimum, %d", ws.KeySkillsMin)
}

// Remote rule labels: how search_area.remote reads, here and in the Search area form.
var RemoteLabels = map[string]string{
	store.RemoteInPlaces: "count if in my Places",
	store.RemoteAnywhere: "count from anywhere",
	store.RemoteNo:       "never count",
}

// WarnSep leads a row's warning after its source.
const WarnSep = "  ⚠ "

var errFollowUpDays = errors.New("Follow-up must be a whole number of days, 1 to 90")

// Gate is one Documents row: its workspace.json key, its label and the unit it counts.
type Gate struct{ Key, Label, Unit string }

// Gates are the Documents rows in display order, worded alike: what is limited, then
// minimum or maximum.
var Gates = []Gate{
	{store.GateLetterCap, "Cover letter maximum", "words"},
	{store.GateLetterMin, "Cover letter minimum", "words"},
	{store.GateSkillsMin, "Key Skills minimum", "skills"},
	{store.GateSkillsMax, "Key Skills maximum", "skills"},
}

// settingsRows are the settings of a group, each with its current value and source.
func (st State[Undo]) Rows(group int, machine Machine) []Row {
	ws := st.Ws
	switch Groups[group] {
	case "Search":
		return []Row{
			{Label: fieldPlaces, Value: strings.Join(ws.Places, ", "), Source: settingsFile, Example: "Australia, APAC"},
			{Label: fieldRemote, Value: RemoteLabel(ws.Remote), Source: FileSource(ws.Remote != "")},
			{Label: "Sources", Value: strconv.Itoa(ws.Sources), ReadOnly: true, Reason: "read-only"},
			{Label: "Blockers", Value: strconv.Itoa(ws.Blockers), ReadOnly: true, Reason: "read-only"},
		}
	case "Documents":
		return st.gateRows()
	case "Tracking":
		return st.trackingRows(machine)
	}
	editor := machine.Editor
	editorRow := Row{Label: LabelEditor, Value: editor, Source: SourceText(machine.EditorSource), Example: "nvim, code --wait"}
	if !machine.EditorAvailable {
		editorRow.Warning = "not found on PATH"
	}
	ascii := config.SourceDefault
	if machine.ASCII {
		ascii = configFile
	}
	return []Row{
		{Label: LabelWorkspace, Value: machine.Workspace, ReadOnly: true, Reason: "fixed this session", Source: SourceText(machine.Sources.Workspace)},
		editorRow,
		{Label: LabelDownloads, Value: machine.Downloads, Source: SourceText(machine.Sources.Downloads), Example: "~/Downloads"},
		{Label: LabelHarness, Value: machine.Harness, Source: SourceText(machine.Sources.Harness), Example: `claude "/{skill} {dir}"`},
		{Label: LabelPlugin, Value: machine.Plugin, Source: SourceText(machine.Sources.Plugin), Example: "path to rolecall/plugin"},
		{Label: LabelASCII, Value: yesNo(machine.ASCII), Source: ascii},
	}
}

// trackingRows lists one row per Track with its Application count, a row to add one, then
// the follow-up threshold.
func (st State[Undo]) trackingRows(machine Machine) []Row {
	var rows []Row
	for _, t := range st.Ws.Tracks {
		rows = append(rows, Row{Label: t, Value: st.trackCount(t, machine)})
	}
	if len(rows) > 0 {
		rows[0].Section = "Tracks"
	}
	rows = append(rows, Row{Label: LabelAddTrack, Action: true})
	if len(rows) == 1 {
		rows[0].Section = "Tracks"
	}
	return append(rows, Row{Label: LabelFollowUp, Value: strconv.Itoa(machine.FollowUpDays), Unit: "days",
		Source: machine.Sources.FollowUpDays, Section: "Follow-up"})
}

// trackCount is a Track's Application count. Where some are closed and some are live it
// says how the count splits, "10 applications · 9 active · 1 closed", so it agrees with
// the status counts beside it.
func (st State[Undo]) trackCount(track string, machine Machine) string {
	n := st.TrackUse[track]
	active := machine.TrackActive[track]
	if closed := n - active; active > 0 && closed > 0 {
		return fmt.Sprintf("%d applications · %d active · %d closed", n, active, closed)
	}
	return plural(n, "Application", "Applications")
}

// gateRows lists the document Gates with their current values. The template choice row is
// not listed: there is one template, and the row appears once there is more than one.
func (st State[Undo]) gateRows() []Row {
	ws := st.Ws
	rows := make([]Row, len(Gates))
	for i, g := range Gates {
		rows[i] = Row{Label: g.Label, Value: strconv.Itoa(GateValue(ws, g.Key)), Unit: g.Unit, Source: FileSource(ws.Set[g.Key])}
	}
	return rows
}

// selectableRow is row if the cursor can rest there, else the nearest row in direction
// dir (then the other way) that it can.
func SelectableRow(rows []Row, row, dir int) int {
	if len(rows) == 0 {
		return 0
	}
	row = min(max(row, 0), len(rows)-1)
	for _, d := range []int{dir, -dir} {
		for i := row; i >= 0 && i < len(rows); i += d {
			if rows[i].Selectable() {
				return i
			}
		}
	}
	return row
}

// moveSetting moves the group while the list has focus, the row otherwise. The row skips
// read-only rows, so enter always has something to do.
func (st State[Undo]) move(delta int, machine Machine) State[Undo] {
	if st.Focus == settingsList {
		st.Group = min(max(st.Group+delta, 0), len(Groups)-1)
		st.Row = SelectableRow(st.Rows(st.Group, machine), 0, 1)
		return st
	}
	rows := st.Rows(st.Group, machine)
	for i := st.Row + delta; i >= 0 && i < len(rows); i += delta {
		if rows[i].Selectable() {
			st.Row = i
			break
		}
	}
	return st
}

// Sources records where each effective machine setting came from.
type Sources struct{ Workspace, FollowUpDays, Downloads, Plugin, Harness string }

// Machine supplies effective values from the running application.
type Machine struct {
	Workspace, Editor, EditorSource, Downloads, Harness, Plugin string
	EditorAvailable, ASCII                                      bool
	FollowUpDays                                                int
	TrackActive                                                 map[string]int // live Applications on each Track, to explain its count
	Sources                                                     Sources
}

// Navigate moves focus or selection, preserving the previous value for model copies.
func (st State[Undo]) Navigate(key string, machine Machine) State[Undo] {
	switch key {
	case "tab", "shift+tab":
		st.Focus = (st.Focus + 1) % settingsPanes
		st.Row = SelectableRow(st.Rows(st.Group, machine), st.Row, 1)
	case "down", "j":
		st = st.move(1, machine)
	case "up", "k":
		st = st.move(-1, machine)
	}
	return st
}

// Load refreshes workspace settings and track counts, preserving navigation and undo.
func (st State[Undo]) Load(s *store.Store) (State[Undo], error) {
	ws, err := s.Settings()
	if err != nil {
		return st, err
	}
	st.Ws = ws
	st.TrackUse = map[string]int{}
	if counts, err := s.TrackUseAll(); err == nil {
		st.TrackUse = counts
	}
	return st, nil
}
func yesNo(on bool) string {
	if on {
		return "Yes"
	}
	return "No"
}
func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

const (
	settingsFile    = "workspace.json"
	configFile      = "config.toml"
	settingsList    = 0
	settingsRows    = 1
	settingsPanes   = 2
	fieldPlaces     = "Places"
	fieldRemote     = "Remote roles"
	minFollowUpDays = 1
	maxFollowUpDays = 90
)
