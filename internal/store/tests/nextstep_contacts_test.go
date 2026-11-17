package store_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lukebrown14-code/rolecall/internal/store"
)

func metaWith(t *testing.T, extra string) (*store.Store, string) {
	t.Helper()
	root := t.TempDir()
	folder := filepath.Join(root, "jobs", "SWE", "x")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "schema: 1\ntitle: T\ncompany: C\ntrack: SWE\n" + extra + "status_log:\n  - {status: Applied, at: 2026-01-01T00:00:00Z}\n"
	if err := os.WriteFile(filepath.Join(folder, "meta.yml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	return s, folder
}

func read(t *testing.T, s *store.Store, folder string) store.Application {
	t.Helper()
	a, err := s.Rescan(folder)
	if err != nil || a == nil {
		t.Fatal(a, err)
	}
	return *a
}

func TestNextStepAndContactsRead(t *testing.T) {
	s, folder := metaWith(t, "next_step: {at: 2026-12-03T10:30, what: \"Phone screen, with Sam\"} # keep\ncontacts:\n  - {name: \"Sam Lee\", role: Recruiter, email: sam@example.test}\n  - {url: \"https://example.test/in/kim\"}\nclose_reason: no-response\n")
	a := read(t, s, folder)
	if a.Malformed != nil {
		t.Fatal(a.Malformed)
	}
	if a.NextStep == nil || a.NextStep.At != "2026-12-03T10:30" || a.NextStep.What != "Phone screen, with Sam" {
		t.Fatalf("next step %+v", a.NextStep)
	}
	if len(a.Contacts) != 2 || a.Contacts[0].Email != "sam@example.test" || a.Contacts[1].Reach() != "https://example.test/in/kim" {
		t.Fatalf("contacts %+v", a.Contacts)
	}
	if a.CloseReason != "no-response" {
		t.Fatal(a.CloseReason)
	}
}

func TestNextStepAndContactsMalformed(t *testing.T) {
	for name, extra := range map[string]string{
		"bad date":      "next_step: {at: 2026-13-03, what: x}\n",
		"no at":         "next_step: {what: x}\n",
		"bad time":      "next_step: {at: 2026-12-03T25:00}\n",
		"not a map":     "next_step: soon\n",
		"unclosed":      "next_step: {at: 2026-12-03\n",
		"empty contact": "contacts:\n  - {role: x}\n",
		"scalar":        "contacts: Sam\n",
		"stray indent":  "contacts: []\n  - {name: Sam}\n",
	} {
		s, folder := metaWith(t, extra)
		if read(t, s, folder).Malformed == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestLegacyContactReadsAsTheFirstContact(t *testing.T) {
	s, folder := metaWith(t, "contact: Jo Park\n")
	a := read(t, s, folder)
	if len(a.Contacts) != 1 || a.Contacts[0].Name != "Jo Park" {
		t.Fatalf("%+v", a.Contacts)
	}
	s, folder = metaWith(t, "contact: jo@example.test\n")
	if c := read(t, s, folder).Contacts; len(c) != 1 || c[0].Email != "jo@example.test" {
		t.Fatalf("%+v", c)
	}
}

func TestSetNextStepPatchesOneLine(t *testing.T) {
	s, folder := metaWith(t, "salary: \"legacy\" # kept\n")
	if err := s.SetNextStep(folder, &store.NextStep{At: "2026-12-03", What: "Take-home due"}); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(folder, "meta.yml"))
	want := "salary: \"legacy\" # kept\nnext_step: {at: 2026-12-03, what: \"Take-home due\"}\nstatus_log:\n"
	if !strings.Contains(string(b), want) {
		t.Fatalf("%s", b)
	}
	if err := s.SetNextStep(folder, &store.NextStep{At: "2026-12-04T09:00", What: "Call"}); err != nil {
		t.Fatal(err)
	}
	if a := read(t, s, folder); a.NextStep.At != "2026-12-04T09:00" || a.NextStep.What != "Call" {
		t.Fatalf("%+v", a.NextStep)
	}
	if err := s.SetNextStep(folder, &store.NextStep{At: "soon"}); err == nil {
		t.Fatal("accepted a bad date")
	}
	if err := s.SetNextStep(folder, nil); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(filepath.Join(folder, "meta.yml"))
	if strings.Contains(string(b), "next_step") || !strings.Contains(string(b), "salary: \"legacy\" # kept\n") {
		t.Fatalf("%s", b)
	}
}

func TestSetContactsReplacesTheBlockAndRetiresLegacy(t *testing.T) {
	s, folder := metaWith(t, "contact: Jo Park\nfit: Good\n")
	list := []store.Contact{{Name: "Jo Park", Role: "Hiring manager", Email: "jo@example.test"}, {Name: "Kim \"K\" Ng", URL: "https://example.test/k"}}
	if err := s.SetContacts(folder, list); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(folder, "meta.yml"))
	text := string(b)
	if strings.Contains(text, "\ncontact:") || !strings.Contains(text, "fit: Good\n") {
		t.Fatalf("%s", text)
	}
	got := read(t, s, folder).Contacts
	if len(got) != 2 || got[0] != list[0] || got[1] != list[1] {
		t.Fatalf("%+v", got)
	}
	if err := s.SetContacts(folder, list[:1]); err != nil {
		t.Fatal(err)
	}
	if got := read(t, s, folder).Contacts; len(got) != 1 {
		t.Fatalf("%+v", got)
	}
	if err := s.SetContacts(folder, nil); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(filepath.Join(folder, "meta.yml"))
	if strings.Contains(string(b), "contacts") {
		t.Fatalf("%s", b)
	}
	if err := s.SetContacts(folder, []store.Contact{{}}); err == nil {
		t.Fatal("accepted an empty contact")
	}
}

func TestCloseReasonGoesWhenTheApplicationReopens(t *testing.T) {
	s, folder := metaWith(t, "")
	at := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	if _, err := s.AppendStatus(folder, store.StatusRejected, at); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCloseReason(folder, " after-screen "); err != nil {
		t.Fatal(err)
	}
	if got := read(t, s, folder).CloseReason; got != "after-screen" {
		t.Fatal(got)
	}
	// Rejected → Withdrawn keeps it; leaving the closed Statuses drops it.
	if _, err := s.AppendStatus(folder, store.StatusWithdrawn, at.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if got := read(t, s, folder).CloseReason; got != "after-screen" {
		t.Fatal(got)
	}
	if _, err := s.AppendStatus(folder, store.StatusInterviewing, at.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if got := read(t, s, folder).CloseReason; got != "" {
		t.Fatal(got)
	}
	if err := s.SetCloseReason(folder, ""); err != nil {
		t.Fatal(err)
	}
}
