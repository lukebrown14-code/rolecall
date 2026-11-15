package store_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lukebrown14-code/rolecall/internal/store"
)

func readMasterProfile(t *testing.T, s *store.Store) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(s.Root(), "profile", "master-profile.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestSetContactFillsAnEmptyStarterProfile(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ws")
	if err := store.InitializeWorkspace(root, nil, nil, store.SearchArea{}); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetContact(map[string]string{"Name": "Alex Quinn", "Email": "alex@example.test", "Phone": " "}); err != nil {
		t.Fatal(err)
	}
	want := "# Master Profile\n\n## Contact\n\n| Field | Value |\n| :-- | :-- |\n| Name | Alex Quinn |\n| Email | alex@example.test |\n"
	if got := readMasterProfile(t, s); got != want {
		t.Fatalf("master-profile.md =\n%q\nwant\n%q", got, want)
	}
	p, err := s.Profile()
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Missing) != 0 {
		t.Fatalf("Missing = %v, want none", p.Missing)
	}
}

func TestSetContactKeepsEverythingOutsideTheContactTable(t *testing.T) {
	s := fixtureProfileStore(t)
	writeMasterProfile(t, s, "# Me\n\nIntro.\n\n## Contact\n\n| Field | Value |\n| :-- | :-- |\n| Name | Old |\n| GitHub | github.com/old |\n| Website | old.test |\n\n## Experience\n\n- Job\n")
	if err := s.SetContact(map[string]string{"Name": "New", "Email": "sam@example.test"}); err != nil {
		t.Fatal(err)
	}
	want := "# Me\n\nIntro.\n\n## Contact\n\n| Field | Value |\n| :-- | :-- |\n| Name | New |\n| Email | sam@example.test |\n| GitHub | github.com/old |\n| Website | old.test |\n\n## Experience\n\n- Job\n"
	if got := readMasterProfile(t, s); got != want {
		t.Fatalf("master-profile.md =\n%q\nwant\n%q", got, want)
	}
}

func TestSetContactAddsTheSectionAfterTheTitle(t *testing.T) {
	s := fixtureProfileStore(t)
	writeMasterProfile(t, s, "# Me\n\n## Experience\n\n- Job\n")
	if err := s.SetContact(map[string]string{"Name": "A", "Email": "jobs@example.test"}); err != nil {
		t.Fatal(err)
	}
	want := "# Me\n\n## Contact\n\n| Field | Value |\n| :-- | :-- |\n| Name | A |\n| Email | jobs@example.test |\n\n## Experience\n\n- Job\n"
	if got := readMasterProfile(t, s); got != want {
		t.Fatalf("master-profile.md =\n%q\nwant\n%q", got, want)
	}
}
