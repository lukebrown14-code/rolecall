package store_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/lukebrown14-code/rolecall/store"
)

// fixtureProfileStore copies the shared fixture Workspace so a test can rewrite
// or remove the Profile without touching the checked-in files.
func fixtureProfileStore(t *testing.T) *store.Store {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../fixtures/workspace")); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func writeMasterProfile(t *testing.T, s *store.Store, text string) {
	t.Helper()
	path := filepath.Join(s.Root(), "profile", "master-profile.md")
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
}

// profileWithContactRows writes a Profile whose Contact holds just these rows.
func profileWithContactRows(t *testing.T, rows ...string) store.Profile {
	t.Helper()
	s := fixtureProfileStore(t)
	text := "# Master Profile\n\n## Contact\n\n| Field | Value |\n| :-- | :-- |\n"
	for _, row := range rows {
		text += row + "\n"
	}
	writeMasterProfile(t, s, text)
	p, err := s.Profile()
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// TestProfileContactMatchesSharedExpectations keeps the Go reader and
// identity.py parsing the fixture Profile the same way.
func TestProfileContactMatchesSharedExpectations(t *testing.T) {
	s := fixtureProfileStore(t)
	got, err := s.Profile()
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile("../fixtures/profile-contact.json")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{}
	if err = json.Unmarshal(b, &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Contact, want) {
		t.Fatalf("contact = %v, want %v", got.Contact, want)
	}
	if len(got.Missing) != 0 {
		t.Fatalf("missing = %v, want none", got.Missing)
	}
}

func TestProfileStripsContactMarkupAndNotes(t *testing.T) {
	p := profileWithContactRows(t,
		"| Name | **Sam Rivers** |",
		"| Email | `sam.rivers@example.test` |",
		"| Location | *Remote, AU* |",
		"| Work rights | Australian citizen *(no sponsorship needed)* |",
	)
	want := map[string]string{
		"Name":        "Sam Rivers",
		"Email":       "sam.rivers@example.test",
		"Location":    "Remote, AU",
		"Work rights": "Australian citizen",
	}
	if !reflect.DeepEqual(p.Contact, want) {
		t.Fatalf("contact = %v, want %v", p.Contact, want)
	}
}

func TestProfileReportsMissingRequiredFieldsInOrder(t *testing.T) {
	cases := []struct {
		name string
		rows []string
		want []string
	}{
		{"name", []string{"| Email | sam@example.test |"}, []string{"Name"}},
		{"email", []string{"| Name | Sam Rivers |"}, []string{"Email"}},
		{"both", []string{"| Phone | +61 400 000 000 |"}, []string{"Name", "Email"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := profileWithContactRows(t, tc.rows...)
			if !reflect.DeepEqual(p.Missing, tc.want) {
				t.Fatalf("missing = %v, want %v", p.Missing, tc.want)
			}
		})
	}
}

func TestProfileWithoutContactSectionHasNoFields(t *testing.T) {
	s := fixtureProfileStore(t)
	writeMasterProfile(t, s, "# Master Profile\n\n## Experience\n\n### Acme\n")
	p, err := s.Profile()
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Contact) != 0 || !reflect.DeepEqual(p.Missing, []string{"Name", "Email"}) {
		t.Fatalf("%+v", p)
	}
}

func TestProfileWithoutFileHasNoFields(t *testing.T) {
	s := fixtureProfileStore(t)
	if err := os.Remove(filepath.Join(s.Root(), "profile", "master-profile.md")); err != nil {
		t.Fatal(err)
	}
	p, err := s.Profile()
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Contact) != 0 || !reflect.DeepEqual(p.Missing, []string{"Name", "Email"}) {
		t.Fatalf("%+v", p)
	}
}

// Sections are the `##` headings in file order; the `#` title is not one of them.
func TestProfileSectionsFollowHeadingsInFileOrder(t *testing.T) {
	s := fixtureProfileStore(t)
	p, err := s.Profile()
	if err != nil {
		t.Fatal(err)
	}
	if !p.Exists {
		t.Fatal("fixture Profile reads as missing")
	}
	var got []string
	for _, section := range p.Sections {
		got = append(got, section.Heading)
	}
	want := []string{"Contact", "Headlines (approved per track)", "Experience", "Key Skills keyword pool", "Summary", "Facts that are NOT available"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("headings = %v, want %v", got, want)
	}
	experience := p.Sections[2].Body
	if !strings.HasPrefix(experience, "### Acme — Brisbane, AU") {
		t.Fatalf("Experience body = %q", experience)
	}
	if strings.HasSuffix(experience, "\n") || strings.HasPrefix(experience, "\n") {
		t.Fatalf("Experience body keeps surrounding blank lines: %q", experience)
	}
}

// A file with no `##` headings is one section holding the whole file.
func TestProfileWithoutHeadingsIsOneSection(t *testing.T) {
	s := fixtureProfileStore(t)
	writeMasterProfile(t, s, "Just prose.\n\nMore prose.\n")
	p, err := s.Profile()
	if err != nil {
		t.Fatal(err)
	}
	want := []store.ProfileSection{{Heading: "Profile", Body: "Just prose.\n\nMore prose."}}
	if !reflect.DeepEqual(p.Sections, want) {
		t.Fatalf("sections = %+v, want %+v", p.Sections, want)
	}
}

// A missing Profile has no sections, and the Contact fields are still reported missing.
func TestProfileMissingFileHasNoSections(t *testing.T) {
	s := fixtureProfileStore(t)
	if err := os.Remove(filepath.Join(s.Root(), "profile", "master-profile.md")); err != nil {
		t.Fatal(err)
	}
	p, err := s.Profile()
	if err != nil {
		t.Fatal(err)
	}
	if p.Exists || len(p.Sections) != 0 {
		t.Fatalf("%+v", p)
	}
}

// The other Profile files are the three the Harness reads, in a fixed order, and only
// the ones that exist. Anything else in profile/ is ignored.
func TestProfileListsOtherFilesInOrder(t *testing.T) {
	s := fixtureProfileStore(t)
	dir := filepath.Join(s.Root(), "profile")
	for _, name := range []string{"phrasing-bank.md", "writing-preferences.md", "notes.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("body of "+name+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	p, err := s.Profile()
	if err != nil {
		t.Fatal(err)
	}
	var got, paths []string
	for _, f := range p.Files {
		got = append(got, f.Name)
		paths = append(paths, f.Path)
	}
	want := []string{"projects.md", "writing-preferences.md", "phrasing-bank.md"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("files = %v, want %v", got, want)
	}
	wantPaths := []string{"profile/projects.md", "profile/writing-preferences.md", "profile/phrasing-bank.md"}
	if !reflect.DeepEqual(paths, wantPaths) {
		t.Fatalf("paths = %v, want %v", paths, wantPaths)
	}
	if !strings.Contains(p.Files[0].Body, "Tide Tables") {
		t.Fatalf("projects.md body = %q", p.Files[0].Body)
	}
}

// A missing master-profile.md is still the missing Profile, but the other files it
// can read are listed.
func TestProfileMissingMasterProfileStillListsOtherFiles(t *testing.T) {
	s := fixtureProfileStore(t)
	if err := os.Remove(filepath.Join(s.Root(), "profile", "master-profile.md")); err != nil {
		t.Fatal(err)
	}
	p, err := s.Profile()
	if err != nil {
		t.Fatal(err)
	}
	if p.Exists || len(p.Sections) != 0 {
		t.Fatalf("%+v", p)
	}
	if len(p.Files) != 1 || p.Files[0].Name != "projects.md" {
		t.Fatalf("files = %+v, want projects.md", p.Files)
	}
}

// `###` is a subheading inside a section, not a section of its own.
func TestProfileSubheadingsAreNotSections(t *testing.T) {
	s := fixtureProfileStore(t)
	writeMasterProfile(t, s, "# Title\n\n## One\n\n### Nested\n\nbody\n\n## Two\n\nlast\n")
	p, err := s.Profile()
	if err != nil {
		t.Fatal(err)
	}
	want := []store.ProfileSection{{Heading: "One", Body: "### Nested\n\nbody"}, {Heading: "Two", Body: "last"}}
	if !reflect.DeepEqual(p.Sections, want) {
		t.Fatalf("sections = %+v, want %+v", p.Sections, want)
	}
}

func TestProfileToleratesCRLFAndPipesInValues(t *testing.T) {
	s := fixtureProfileStore(t)
	writeMasterProfile(t, s, "## Contact\r\n\r\n| Field | Value |\r\n| :-- | :-- |\r\n"+
		"| Name | Sam Rivers |\r\n| Email | sam@example.test |\r\n| Note | one | two |\r\n")
	p, err := s.Profile()
	if err != nil {
		t.Fatal(err)
	}
	if p.Contact["Name"] != "Sam Rivers" || p.Contact["Note"] != "one | two" {
		t.Fatalf("contact = %v", p.Contact)
	}
}
