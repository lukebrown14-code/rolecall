package store_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/lukebrown14-code/rolecall/internal/store"
)

// writeLead writes a meta.yml under jobs/SWE/<name> and returns the folder.
func writeLead(t *testing.T, s *store.Store, name, meta string) string {
	t.Helper()
	folder := filepath.Join(s.Root(), "jobs", "SWE", name)
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "meta.yml"), []byte(meta), 0o644); err != nil {
		t.Fatal(err)
	}
	return folder
}

func rescan(t *testing.T, s *store.Store, folder string) store.Application {
	t.Helper()
	a, err := s.Rescan(folder)
	if err != nil || a == nil {
		t.Fatalf("rescan: %v", err)
	}
	return *a
}

const leadHead = "schema: 1\ntitle: Engineer\ncompany: Acme\ntrack: SWE\n"

func TestSweepKeysAreRead(t *testing.T) {
	s := fixtureProfileStore(t)
	folder := writeLead(t, s, "swept", leadHead+"posted: 2026-11-01\nposted_label: \"14d ago\"\napplicants: 47\nsource: linkedin\n"+
		"filter_reason: expired\nfilter_note: \"posted 47 days ago; limit 30\"\nstatus_log:\n  - {status: Dismissed, at: 2026-11-02T00:00:00Z}\n")
	a := rescan(t, s, folder)
	if a.Malformed != nil {
		t.Fatal(a.Malformed)
	}
	if a.Posted != "2026-11-01" || a.PostedLabel != "14d ago" || a.Applicants == nil || *a.Applicants != 47 || a.Source != "linkedin" ||
		a.FilterReason != store.FilterExpired || a.FilterNote != "posted 47 days ago; limit 30" || a.FilterOverruled || !a.Filtered() {
		t.Fatalf("app = %+v", a)
	}
}

func TestBadSweepKeysAreMalformed(t *testing.T) {
	s := fixtureProfileStore(t)
	for _, line := range []string{"posted: 2026-13-01", "applicants: -1", "applicants: many", "filter_reason: vibes", "filter_overruled: yes"} {
		folder := writeLead(t, s, "bad", leadHead+line+"\n")
		if a := rescan(t, s, folder); a.Malformed == nil {
			t.Errorf("%q read as valid", line)
		}
	}
}

func TestRestoringARuleDismissalOverrulesItAndUndoPutsItBack(t *testing.T) {
	s := fixtureProfileStore(t)
	folder := writeLead(t, s, "expired", leadHead+"filter_reason: expired # by the Sweep\nfilter_note: too old\nstatus_log:\n  - {status: Lead, at: 2026-11-01T00:00:00Z}\n  - {status: Dismissed, at: 2026-11-02T00:00:00Z}\n")
	if _, err := s.AppendStatus(folder, store.StatusLead, time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(folder, "meta.yml"))
	want := leadHead + "filter_overruled: true\nstatus_log:\n  - {status: Lead, at: 2026-11-01T00:00:00Z}\n  - {status: Dismissed, at: 2026-11-02T00:00:00Z}\n  - {status: Lead, at: 2026-11-03T00:00:00Z}\n"
	if string(b) != want {
		t.Fatalf("meta.yml =\n%s\nwant\n%s", b, want)
	}
	if a := rescan(t, s, folder); !a.FilterOverruled || a.FilterReason != "" || a.Filtered() {
		t.Fatalf("app = %+v", a)
	}
	if _, err := s.AppendStatus(folder, store.StatusDismissed, time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if err := s.SetFilter(folder, store.FilterExpired, "too old"); err != nil {
		t.Fatal(err)
	}
	if a := rescan(t, s, folder); a.FilterOverruled || a.FilterReason != store.FilterExpired || a.FilterNote != "too old" || !a.Filtered() {
		t.Fatalf("after undo app = %+v", a)
	}
}

func TestSetSearchRuleKeepsOtherRulesAndClears(t *testing.T) {
	s := fixtureProfileStore(t)
	writeWorkspaceJSON(t, s.Root(), `{"tracks":["SWE"],"search_rules":{"max_contract_months":6}}`)
	if err := s.SetSearchRule(store.RuleMaxPostingAge, "30"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetSearchRule(store.RuleMaxApplicants, "linkedin 100, seek 50"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetSearchRule(store.RuleAmbiguousLabels, "1mo ago = discard, 4w  ago = keep"); err != nil {
		t.Fatal(err)
	}
	ws, _ := s.Settings()
	r := ws.Rules
	if r.MaxPostingAgeDays != 30 || r.MaxContractMonths != 6 || r.ExpiryWarningDays != store.DefaultExpiryWarningDays ||
		!reflect.DeepEqual(r.MaxApplicants, map[string]int{"linkedin": 100, "seek": 50}) ||
		!reflect.DeepEqual(r.AmbiguousAgeLabels, map[string]string{"1mo ago": "discard", "4w ago": "keep"}) {
		t.Fatalf("rules = %+v", r)
	}
	if got := readWorkspaceJSON(t, s.Root()); !strings.Contains(got, `"max_contract_months": 6,`) {
		t.Fatalf("order lost:\n%s", got)
	}
	if err := s.SetSearchRule(store.RuleMaxContract, ""); err != nil {
		t.Fatal(err)
	}
	if ws, _ = s.Settings(); ws.Rules.Set[store.RuleMaxContract] {
		t.Fatal("max_contract_months not cleared")
	}
	for key, bad := range map[string]string{store.RuleMaxPostingAge: "0", store.RuleExpiryWarning: "-1", store.RuleMaxApplicants: "linkedin", store.RuleAmbiguousLabels: "1mo = maybe"} {
		if err := s.SetSearchRule(key, bad); err == nil {
			t.Errorf("%s accepted %q", key, bad)
		}
	}
}

func TestSourcesLegacyObjectIsListedAndConvertedOnWrite(t *testing.T) {
	s := fixtureProfileStore(t)
	writeWorkspaceJSON(t, s.Root(), `{"sources":{"greenhouse":["a","b"],"lever":["c"]}}`)
	ws, _ := s.Settings()
	if len(ws.SourceList) != 2 || ws.Sources != 2 || ws.SourceList[0].Name != "greenhouse" || ws.SourceList[0].Type != store.SourceAPI ||
		!ws.SourceList[0].On() || ws.SourceList[0].AccessText() != store.AccessUnknown || len(ws.SourceList[0].Targets) != 2 {
		t.Fatalf("sources = %+v", ws.SourceList)
	}
	if err := s.SetSourceEnabled("lever", false); err != nil {
		t.Fatal(err)
	}
	if err := s.AddSource(store.Source{Name: "linkedin", Type: store.SourceBrowser, URL: "https://example.test", Access: "login wall"}); err != nil {
		t.Fatal(err)
	}
	ws, _ = s.Settings()
	if len(ws.SourceList) != 3 || ws.SourceList[1].On() || !reflect.DeepEqual(ws.SourceList[0].Targets, []string{"a", "b"}) || ws.SourceList[2].Access != "login wall" {
		t.Fatalf("sources = %+v", ws.SourceList)
	}
	for _, bad := range []store.Source{{Name: "linkedin", Type: store.SourceManual}, {Name: "x", Type: "rss"}, {Name: "y", Type: store.SourceAPI, Platform: "workday"}, {Type: store.SourceManual}} {
		if err := s.AddSource(bad); err == nil {
			t.Errorf("accepted %+v", bad)
		}
	}
	if err := s.RemoveSource("linkedin"); err != nil {
		t.Fatal(err)
	}
	if ws, _ = s.Settings(); len(ws.SourceList) != 2 {
		t.Fatalf("sources = %+v", ws.SourceList)
	}
	writeWorkspaceJSON(t, s.Root(), `{oops`)
	if err := s.SetSourceEnabled("lever", true); err == nil || readWorkspaceJSON(t, s.Root()) != `{oops` {
		t.Fatal("wrote over invalid JSON")
	}
}

// UpdateSource rewrites a Source's own fields in place, keeping its other keys and its
// place, and refuses what AddSource refuses.
func TestUpdateSourceKeepsOtherKeysAndPlace(t *testing.T) {
	s := fixtureProfileStore(t)
	writeWorkspaceJSON(t, s.Root(), `{"sources":[{"name":"gh","type":"api","platform":"greenhouse","targets":["a"],"enabled":false,"notes":"keep me"},{"name":"seek","type":"browser"}]}`)
	if err := s.UpdateSource("gh", store.Source{Name: "boards", Type: store.SourceBrowser, Platform: "greenhouse", Targets: []string{"a"}, URL: "https://example.test", Access: "works"}); err != nil {
		t.Fatal(err)
	}
	ws, _ := s.Settings()
	got := ws.SourceList[0]
	if len(ws.SourceList) != 2 || got.Name != "boards" || got.Type != store.SourceBrowser || got.Platform != "" || got.Targets != nil ||
		got.URL != "https://example.test" || got.Access != "works" || got.On() || got.Notes != "keep me" {
		t.Fatalf("sources = %+v", ws.SourceList)
	}
	for _, bad := range []store.Source{{Name: "SEEK", Type: store.SourceManual}, {Name: "boards", Type: "rss"}, {Name: "", Type: store.SourceManual}} {
		if err := s.UpdateSource("boards", bad); err == nil {
			t.Errorf("accepted %+v", bad)
		}
	}
	if err := s.UpdateSource("nope", store.Source{Name: "nope", Type: store.SourceManual}); err == nil {
		t.Error("updated a Source that is not listed")
	}
}
