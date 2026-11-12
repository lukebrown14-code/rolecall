package store_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
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
