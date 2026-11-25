package store_test

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDocumentSettingsPreserveContactOmit(t *testing.T) {
	s := fixtureProfileStore(t)
	writeWorkspaceJSON(t, s.Root(), `{"contactOmit":{"Analyst":["GitHub"]}}`)
	if err := s.SetGate("letterWordCap", "300"); err != nil {
		t.Fatal(err)
	}
	var saved struct {
		ContactOmit map[string][]string
	}
	if err := json.Unmarshal([]byte(readWorkspaceJSON(t, s.Root())), &saved); err != nil {
		t.Fatal(err)
	}
	got := saved.ContactOmit["Analyst"]
	if len(got) != 1 || got[0] != "GitHub" {
		t.Fatalf("contactOmit = %v", saved.ContactOmit)
	}
}

func TestSetGateWritesOnlyThatKeyAndKeepsTheRest(t *testing.T) {
	s := fixtureProfileStore(t)
	writeWorkspaceJSON(t, s.Root(), `{"sources":[1],"letterWordCap":350}`)
	if err := s.SetGate("letterWordCap", "300"); err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"sources\": [\n    1\n  ],\n  \"letterWordCap\": 300\n}\n"
	if got := readWorkspaceJSON(t, s.Root()); got != want {
		t.Fatalf("file = %q", got)
	}
	ws, _ := s.Settings()
	if ws.LetterWordCap != 300 {
		t.Fatalf("cap = %d", ws.LetterWordCap)
	}
}

func TestSetGateRefusesBadValuesWithTheReasonAndWritesNothing(t *testing.T) {
	s := fixtureProfileStore(t)
	writeWorkspaceJSON(t, s.Root(), `{"letterWordCap":350}`)
	for _, c := range []struct{ key, val, reason string }{
		{"letterWordCap", "abc", "number"},
		{"letterWordCap", "-5", "number"},
		{"letterWordCap", "0", "above 0"},
		{"letterWordCap", "100", "above the minimum"},
		{"letterWordMin", "350", "below the maximum"},
		{"keySkillsMin", "13", "at most the maximum"},
		{"keySkillsMax", "7", "at least the minimum"},
		{"bogus", "1", "unknown"},
	} {
		err := s.SetGate(c.key, c.val)
		if err == nil || !strings.Contains(err.Error(), c.reason) {
			t.Fatalf("%s=%s: err = %v, want %q", c.key, c.val, err, c.reason)
		}
	}
	if got := readWorkspaceJSON(t, s.Root()); got != `{"letterWordCap":350}` {
		t.Fatalf("file changed: %s", got)
	}
}
