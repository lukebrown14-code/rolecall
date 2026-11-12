package store_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/lukebrown14-code/rolecall/internal/store"
)

func writeWorkspaceJSON(t *testing.T, root, text string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "workspace.json"), []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestSettingsReadsFixtureWorkspace(t *testing.T) {
	s := fixtureProfileStore(t)
	got, err := s.Settings()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Tracks, []string{"SWE", "Platform"}) ||
		!reflect.DeepEqual(got.Places, []string{"Australia"}) || got.Remote != "in-places" {
		t.Fatalf("settings = %+v", got)
	}
	if got.Path != filepath.Join(s.Root(), "workspace.json") || got.Err != nil {
		t.Fatalf("path/err = %q, %v", got.Path, got.Err)
	}
}

func TestSettingsGatesDefaultAndCountRows(t *testing.T) {
	s := fixtureProfileStore(t)
	got, _ := s.Settings()
	if got.LetterWordCap != 350 || got.LetterWordMin != 150 || got.KeySkillsMin != 8 || got.KeySkillsMax != 12 {
		t.Fatalf("defaults = %+v", got)
	}
	writeWorkspaceJSON(t, s.Root(), `{"letterWordCap":300,"keySkillsMax":10,"sources":[{"a":1},{"b":2}],"blockers":["x","y","z"]}`)
	got, _ = s.Settings()
	if got.LetterWordCap != 300 || got.KeySkillsMax != 10 || got.Sources != 2 || got.Blockers != 3 {
		t.Fatalf("settings = %+v", got)
	}
}

// A schema above this build's is reported, not silently rewritten: the Owner may be on a
// newer rolecall whose keys this one would drop.
func TestSettingsReportsNewerSchema(t *testing.T) {
	s := fixtureProfileStore(t)
	writeWorkspaceJSON(t, s.Root(), `{"schema":2,"tracks":["SWE"]}`)
	got, err := s.Settings()
	if err != nil {
		t.Fatal(err)
	}
	if !got.NewerSchema || got.Schema != 2 || got.Err == nil {
		t.Fatalf("settings = %+v", got)
	}
}

func TestWritesRefuseNewerSchemaWorkspace(t *testing.T) {
	s := fixtureProfileStore(t)
	const before = `{"schema":2,"tracks":["SWE"],"future":{"key":true}}`
	writeWorkspaceJSON(t, s.Root(), before)
	if err := s.SetTracks([]string{"SWE", "Platform"}); err == nil {
		t.Fatal("SetTracks wrote to a newer-schema Workspace")
	}
	if err := s.SetGate(store.GateLetterCap, "300"); err == nil {
		t.Fatal("SetGate wrote to a newer-schema Workspace")
	}
	if err := s.SetSearchArea([]string{"Somewhere"}, store.RemoteAnywhere); err == nil {
		t.Fatal("SetSearchArea wrote to a newer-schema Workspace")
	}
	got, err := os.ReadFile(filepath.Join(s.Root(), "workspace.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != before {
		t.Fatalf("workspace.json changed:\n%s", got)
	}
}

func TestSettingsInvalidJSONIsReportedNotReturnedAsError(t *testing.T) {
	s := fixtureProfileStore(t)
	writeWorkspaceJSON(t, s.Root(), `{"tracks":`)
	got, err := s.Settings()
	if err != nil {
		t.Fatal(err)
	}
	if got.Err == nil || !strings.Contains(got.Err.Error(), "unexpected end of JSON input") {
		t.Fatalf("Err = %v", got.Err)
	}
}
