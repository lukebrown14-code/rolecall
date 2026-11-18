package settings

import "github.com/lukebrown14-code/rolecall/internal/store"

// Intent is what the Settings sub-model asks of the application. The sub-model never
// writes a file or starts a program: package ui applies each intent through store or
// config, reloads, and reports, so only store touches the Workspace.
type Intent interface{ isIntent() }

// Write is one change to workspace.json (through store) or config.toml (through config).
type Write interface {
	Intent
	// File is the file the change goes to, as the save message names it.
	File() string
}

// SetSearchArea writes workspace.json's search_area.
type SetSearchArea struct {
	Places []string
	Remote string
}

// SetRule writes one Rules row: a search_rules key, or a visa key (IsVisa). Empty text
// clears it.
type SetRule struct{ Key, Text string }

// SetGate writes one document Gate.
type SetGate struct{ Key, Text string }

// SetTracks writes the Track list. The application rescans, as the header filter reads it.
type SetTracks struct{ Tracks []string }

// SetSourceEnabled turns a Source on or off.
type SetSourceEnabled struct {
	Name string
	On   bool
}

// AddSource adds a Source.
type AddSource struct{ Source store.Source }

// RemoveSource removes a Source.
type RemoveSource struct{ Name string }

// UpdateSource rewrites the Source called Name.
type UpdateSource struct {
	Name   string
	Source store.Source
}

// SetMachine writes one config.toml key. The application re-reads config.toml, not
// workspace.json.
type SetMachine struct {
	Key   string
	Value any
}

// RenameTrack renames a Track across workspace.json, meta.yml and roles.json. The
// application hands the result back to Popup.Renamed.
type RenameTrack struct{ Old, New string }

// Save is a write and its report: once the write is applied and reloaded, Model.Saved
// places the cursor, words the message and keeps the undo.
type Save struct {
	Write Write
	// Msg is the save message, or "" while it names the value as the reloaded file reads it.
	Msg string
	// Undo is what u puts back; nil for none.
	Undo *Undo
	// OnForm keeps a refused write on the pop-up's form, under its field. Otherwise the
	// refusal goes on the feedback line and the pop-up closes.
	OnForm bool
	place  placement
	row    int   // the row placeAt puts the cursor on
	rule   *Rule // a Rules row, whose message names the value as the file now reads
}

// placement is where the cursor goes once a save is reloaded.
type placement int

const (
	placeKeep   placement = iota // where the reload left it
	placeAt                      // on Save.row
	placeRemove                  // on the row that took a removed Track's place
)

// Restore is u: apply Undo.Restore, then report "Undone · " + Undo.What.
type Restore struct{ Undo Undo }

// Undo is the last Settings save, so u can put the old value back once.
type Undo struct {
	What    string // "Editor back to vi"
	Restore Write
}

// Open opens a Settings pop-up over the screen.
type Open struct{ Popup *Popup }

// Fail puts an error on the feedback line.
type Fail struct{ Err error }

// Note puts a note on the feedback line.
type Note struct{ Text string }

// CheckSource runs the Plugin's Sweep against one Source in the background.
type CheckSource struct{ Name string }

// ShowTrack leaves Settings for the lists, filtered to a Track.
type ShowTrack struct{ Track string }

func (SetSearchArea) isIntent()    {}
func (SetRule) isIntent()          {}
func (SetGate) isIntent()          {}
func (SetTracks) isIntent()        {}
func (SetSourceEnabled) isIntent() {}
func (AddSource) isIntent()        {}
func (RemoveSource) isIntent()     {}
func (UpdateSource) isIntent()     {}
func (SetMachine) isIntent()       {}
func (RenameTrack) isIntent()      {}
func (Save) isIntent()             {}
func (Restore) isIntent()          {}
func (Open) isIntent()             {}
func (Fail) isIntent()             {}
func (Note) isIntent()             {}
func (CheckSource) isIntent()      {}
func (ShowTrack) isIntent()        {}

func (SetSearchArea) File() string    { return settingsFile }
func (SetRule) File() string          { return settingsFile }
func (SetGate) File() string          { return settingsFile }
func (SetTracks) File() string        { return settingsFile }
func (SetSourceEnabled) File() string { return settingsFile }
func (AddSource) File() string        { return settingsFile }
func (RemoveSource) File() string     { return settingsFile }
func (UpdateSource) File() string     { return settingsFile }
func (SetMachine) File() string       { return configFile }

// IsVisa reports a Rules key that writes workspace.json visa rather than search_rules.
func IsVisa(key string) bool {
	return key == store.VisaNeedsSponsorship || key == store.VisaMaxContract
}
