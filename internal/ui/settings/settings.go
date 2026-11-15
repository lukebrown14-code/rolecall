// Package settings owns Settings screen data, navigation, and validation.
package settings

import (
	"errors"
	"fmt"
	"slices"
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

// GroupAbout is one plain sentence per group, for the about text while the cursor rests
// on the section header.
var GroupAbout = []string{
	"Where the job Sweep looks for roles, and what rules one out. Sources and Blockers are set in " + settingsFile + ": e opens it.",
	"The limits the Build checks a cover letter and Key Skills against.",
	"The kinds of role you apply for, and when an Application is due a follow-up.",
	"Settings for this computer only, kept in " + configFile + " outside the Workspace.",
}

// Labels of the settings rows the code refers to.
const (
	LabelEditor    = "Editor"
	LabelFollowUp  = "Follow-up due after"
	LabelHarness   = "Harness"
	LabelWorkspace = "Workspace"
	LabelDownloads = "Downloads"
	LabelPlugin    = "Plugin"
	LabelASCII     = "ASCII glyphs"
	LabelInherit   = "Inherit colours"
	LabelAddTrack  = "+ add a Track"
)

// Examples shown beside an unset value and while it is edited.
const (
	ExampleEditor  = "nvim, code --wait"
	ExampleHarness = `claude "/{skill} {dir}"`
	ExamplePlugin  = "~/rolecall/plugin"
)

// Row is one line in a group. Every editable value says where it comes from (source), an
// unset one shows an example instead, and a read-only row says why it is fixed, so "not
// set", "default" and "read-only" never read alike.
type Row struct {
	Label, Value string
	Unit         string // after the value: "days", "words"
	Source       string // the file or variable the value comes from, or "default"
	Example      string // shown while the value is unset, and while it is edited
	ReadOnly     bool   // shown, never edited here; the cursor passes over it
	Reason       string // why a read-only row is fixed
	Warning      string // a problem with the value, e.g. an Editor not on PATH
	Section      string // a heading drawn above the row
	Action       bool   // "+ add a Track": no value, enter acts
	OffList      bool   // a Track Applications use that workspace.json does not list
	About        string // one plain sentence on what the setting is, for the focused row
}

// State is the loaded workspace.json and where the Owner is. It is replaced, never
// mutated in place, so copies of the model stay independent.
type State[Undo any] struct {
	Ws       store.WorkspaceSettings
	TrackUse map[string]int // Applications on each Track, archived ones included
	// Breakdown follows each Track's count so the numbers add up: " · 9 active · 1 closed".
	Breakdown map[string]string
	Group     int
	Row       int
	Header    bool // the cursor is on the header, where only the section keys act
	Undo      *Undo
	Edit      *Edit // the field being typed into, if any
}

// Edit is the inline field open on a row. Enter saves it at once; esc drops it.
type Edit struct {
	Key, Buf, Err string
	// Orig is what the field opened with, so a quit can tell typed text from none.
	Orig string
	// Placeholder is the effective default an unset value shows while the field is empty.
	Placeholder string
	// Selected is true while the opening text is selected: the first key typed replaces
	// it, and backspace clears it.
	Selected bool
	// Discarding is the "Discard changes?" question a quit raises over typed text.
	Discarding bool
}

// Changed reports whether the Owner has typed into the field.
func (e Edit) Changed() bool { return e.Buf != e.Orig }

// Selectable reports whether the cursor can rest on a row: everything but read-only rows.
func (r Row) Selectable() bool { return !r.ReadOnly }

// ValueText is a row's value with its unit, as the value column shows it.
func (r Row) ValueText() string {
	if r.Value != "" && r.Unit != "" {
		return r.Value + " " + r.Unit
	}
	return r.Value
}

// Provenance is what follows a row's value: why a read-only row is fixed, where a value
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

// Note is what follows a row's value.
func (r Row) Note() string { return r.Provenance() }

// Tail is the note and warning after a row's value, as plain text, for sizing.
func (r Row) Tail() string {
	t := r.Note()
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

// SplitPlaces turns the Places answer into its list, dropping blanks. An empty answer is
// no Search area: every location is kept.
func SplitPlaces(s string) []string {
	var places []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			places = append(places, p)
		}
	}
	return places
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

var errFollowUpDays = errors.New("Enter whole days, 1 to 90")

// Gate is one Documents row: its workspace.json key, its label and the unit it counts.
type Gate struct{ Key, Label, Unit, About string }

// Gates are the Documents rows in display order, worded alike: what is limited, then
// minimum or maximum.
var Gates = []Gate{
	{store.GateLetterCap, "Cover letter maximum", "words", "The Build refuses a cover letter longer than this."},
	{store.GateLetterMin, "Cover letter minimum", "words", "The Build refuses a cover letter shorter than this."},
	{store.GateSkillsMin, "Key Skills minimum", "skills", "The fewest Key Skills a resume may list."},
	{store.GateSkillsMax, "Key Skills maximum", "skills", "The most Key Skills a resume may list."},
}

// editInWorkspace ends the about text of a value only workspace.json can change.
const editInWorkspace = " Set in " + settingsFile + ": e opens it."

// Rows are the settings of a group, each with its current value and source.
func (st State[Undo]) Rows(group int, machine Machine) []Row {
	ws := st.Ws
	switch Groups[group] {
	case "Search":
		return []Row{
			{Label: fieldPlaces, Value: strings.Join(ws.Places, ", "), Source: settingsFile, Example: "Australia, APAC",
				About: "The places a role must be in for the Sweep to keep it. Leave it empty to keep every location."},
			{Label: fieldRemote, Value: RemoteLabel(ws.Remote), Source: FileSource(ws.Remote != ""),
				About: "Whether the Sweep keeps a remote role, and from where."},
			{Label: "Sources", Value: strconv.Itoa(ws.Sources), ReadOnly: true, Reason: "read-only · job boards the Sweep reads",
				About: "The job boards the Sweep reads, and which companies on each." + editInWorkspace},
			{Label: "Blockers", Value: strconv.Itoa(ws.Blockers), ReadOnly: true, Reason: "read-only · signals that rule a role out",
				About: "Signals that rule a role out for you, such as a security clearance." + editInWorkspace},
		}
	case "Documents":
		return st.gateRows()
	case "Tracking":
		return st.trackingRows(machine)
	}
	editorRow := Row{Label: LabelEditor, Value: machine.Editor, Source: SourceText(machine.EditorSource), Example: ExampleEditor,
		About: "The program e opens files in, such as nvim, hx, code --wait, zed --wait or subl --wait. GUI Editors need their wait flag, or rolecall rescans before you finish."}
	if !machine.EditorAvailable {
		editorRow.Warning = "not found on PATH"
	}
	harnessRow := Row{Label: LabelHarness, Value: machine.Harness, Source: SourceText(machine.Sources.Harness), Example: ExampleHarness,
		About:   "The AI coding tool rolecall hands an Application to, such as Claude Code. rolecall fills in {skill} and {dir}.",
		Warning: machine.HarnessWarning}
	pluginRow := Row{Label: LabelPlugin, Value: machine.Plugin, Source: SourceText(machine.Sources.Plugin), Example: ExamplePlugin,
		About: "The folder of rolecall skills and scripts your Harness runs. It holds a skills/ folder.", Warning: machine.PluginWarning}
	ascii := config.SourceDefault
	if machine.ASCII {
		ascii = configFile
	}
	inherit := config.SourceDefault
	if machine.Inherit {
		inherit = configFile
	}
	return []Row{
		{Label: LabelWorkspace, Value: machine.Workspace, ReadOnly: true, Reason: "fixed this session", Source: SourceText(machine.Sources.Workspace)},
		editorRow,
		{Label: LabelDownloads, Value: machine.Downloads, Source: SourceText(machine.Sources.Downloads), Example: "~/Downloads",
			About: "The folder an Application's PDFs are copied to when you download them."},
		harnessRow,
		pluginRow,
		{Label: LabelASCII, Value: yesNo(machine.ASCII), Source: ascii,
			About: "Draw with plain ASCII instead of symbols, for a font or terminal that lacks them."},
		{Label: LabelInherit, Value: yesNo(machine.Inherit), Source: inherit,
			About: "Use your terminal's own 16 colours and background instead of rolecall's colours."},
	}
}

// OffList is the Tracks Applications use that workspace.json does not list, such as
// "Unknown" on old Applications, in name order. An Application with no Track at all is
// not one: there is no name to add.
func (st State[Undo]) OffList() []string {
	var out []string
	for t, n := range st.TrackUse {
		if t != "" && n > 0 && !slices.Contains(st.Ws.Tracks, t) {
			out = append(out, t)
		}
	}
	slices.Sort(out)
	return out
}

// trackingRows lists one row per Track with its Application count, then the Tracks only
// Applications name, a row to add one, then the follow-up threshold.
func (st State[Undo]) trackingRows(machine Machine) []Row {
	var rows []Row
	for _, t := range st.Ws.Tracks {
		rows = append(rows, Row{Label: t, Value: plural(st.TrackUse[t], "application", "applications") + st.Breakdown[t],
			About: "A Track is a kind of role you apply for. Every Application belongs to one, and the Sweep searches each."})
	}
	for _, t := range st.OffList() {
		rows = append(rows, Row{Label: t, Value: plural(st.TrackUse[t], "application", "applications") + " · not listed", OffList: true,
			About: "Applications use this Track, but " + settingsFile + " does not list it. Enter adds it, so you can filter by it and rename it."})
	}
	if len(rows) > 0 {
		rows[0].Section = "Tracks"
	}
	rows = append(rows, Row{Label: LabelAddTrack, Action: true, About: "Add a kind of role you apply for, such as Data or Platform."})
	if len(rows) == 1 {
		rows[0].Section = "Tracks"
	}
	unit := "days"
	if machine.FollowUpDays == 1 {
		unit = "day"
	}
	return append(rows, Row{Label: LabelFollowUp, Value: strconv.Itoa(machine.FollowUpDays), Unit: unit,
		Source: machine.Sources.FollowUpDays, Section: "Follow-up", Warning: machine.FollowUpWarning,
		About: "An Applied Application with no news for this long is flagged as due a follow-up."})
}

// gateRows lists the document Gates with their current values. The template choice row is
// not listed: there is one template, and the row appears once there is more than one.
func (st State[Undo]) gateRows() []Row {
	ws := st.Ws
	rows := make([]Row, len(Gates))
	for i, g := range Gates {
		rows[i] = Row{Label: g.Label, Value: strconv.Itoa(GateValue(ws, g.Key)), Unit: g.Unit, Source: FileSource(ws.Set[g.Key]), About: g.About}
	}
	return rows
}

// SelectableRow is row if the cursor can rest there, else the nearest row in direction
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

// move moves the cursor down (delta 1) or up. The row skips read-only rows, so enter
// always has something to do; above the first row the cursor rests on the header.
func (st State[Undo]) move(delta int, machine Machine) State[Undo] {
	if st.Header {
		if delta > 0 {
			st.Header = false
			st.Row = SelectableRow(st.Rows(st.Group, machine), 0, 1)
		}
		return st
	}
	rows := st.Rows(st.Group, machine)
	for i := st.Row + delta; i >= 0 && i < len(rows); i += delta {
		if rows[i].Selectable() {
			st.Row = i
			return st
		}
	}
	st.Header = delta < 0
	return st
}

// section switches to the group delta away, wrapping, with the cursor on its first row.
func (st State[Undo]) section(delta int, machine Machine) State[Undo] {
	st.Group = (st.Group + delta + len(Groups)) % len(Groups)
	st.Row = SelectableRow(st.Rows(st.Group, machine), 0, 1)
	return st
}

// Sources records where each effective machine setting came from.
type Sources struct{ Workspace, FollowUpDays, Downloads, Plugin, Harness string }

// Machine supplies effective values from the running application.
type Machine struct {
	Workspace, Editor, EditorSource, Downloads, Harness, Plugin string
	EditorAvailable, ASCII, Inherit                             bool
	// HarnessWarning and PluginWarning name a problem with the value, or are empty. They
	// never block a save.
	HarnessWarning, PluginWarning string
	// FollowUpWarning names a follow_up_days config.toml had out of range.
	FollowUpWarning string
	FollowUpDays    int
	Sources         Sources
}

// Navigate moves the section or the field cursor, preserving the previous value for model
// copies. ←→, tab and [ ] switch sections, as ←→ switch views on every other screen; g and
// G go to the first and last row.
func (st State[Undo]) Navigate(key string, machine Machine) State[Undo] {
	switch key {
	case "tab", "]", "right":
		st = st.section(1, machine)
	case "shift+tab", "[", "left":
		st = st.section(-1, machine)
	case "down", "j":
		st = st.move(1, machine)
	case "up", "k":
		st = st.move(-1, machine)
	case "g", "home":
		st.Header = false
		st.Row = SelectableRow(st.Rows(st.Group, machine), 0, 1)
	case "G", "end":
		rows := st.Rows(st.Group, machine)
		st.Header = false
		st.Row = SelectableRow(rows, len(rows)-1, -1)
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

// YesNo is a Yes/No setting as it reads.
func YesNo(on bool) string { return yesNo(on) }

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
	fieldPlaces     = "Places"
	fieldRemote     = "Remote roles"
	minFollowUpDays = config.MinFollowUpDays
	maxFollowUpDays = config.MaxFollowUpDays
)
