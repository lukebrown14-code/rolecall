package store_test

import (
	"reflect"
	"testing"
)

func TestSetSearchAreaKeepsOtherKeysAndOrder(t *testing.T) {
	s := fixtureProfileStore(t)
	writeWorkspaceJSON(t, s.Root(), `{"tracks":["SWE"],"search_area":{"places":["Old"],"remote":"no"},"sources":[1]}`)
	if err := s.SetSearchArea([]string{"Australia", "APAC"}, "anywhere"); err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"tracks\": [\n    \"SWE\"\n  ],\n  \"search_area\": {\n    \"places\": [\n      \"Australia\",\n      \"APAC\"\n    ],\n    \"remote\": \"anywhere\"\n  },\n  \"sources\": [\n    1\n  ]\n}\n"
	if got := readWorkspaceJSON(t, s.Root()); got != want {
		t.Fatalf("file =\n%s\nwant\n%s", got, want)
	}
	ws, _ := s.Settings()
	if !reflect.DeepEqual(ws.Places, []string{"Australia", "APAC"}) || ws.Remote != "anywhere" {
		t.Fatalf("settings = %+v", ws)
	}
}

func TestSetSearchAreaRefusesInvalidJSONAndBadRemote(t *testing.T) {
	s := fixtureProfileStore(t)
	writeWorkspaceJSON(t, s.Root(), `{oops`)
	if err := s.SetSearchArea([]string{"A"}, "in-places"); err == nil {
		t.Fatal("wrote over invalid JSON")
	}
	if got := readWorkspaceJSON(t, s.Root()); got != `{oops` {
		t.Fatalf("file changed: %s", got)
	}
	writeWorkspaceJSON(t, s.Root(), `{}`)
	if err := s.SetSearchArea([]string{"A"}, "maybe"); err == nil {
		t.Fatal("accepted bad remote")
	}
}
