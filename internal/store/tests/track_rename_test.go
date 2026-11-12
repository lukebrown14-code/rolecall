package store_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func readTrackOf(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, "jobs", rel, "meta.yml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range strings.Split(string(b), "\n") {
		if v, ok := strings.CutPrefix(l, "track:"); ok {
			return strings.Trim(strings.TrimSpace(v), "\"")
		}
	}
	return ""
}

func writeArchived(t *testing.T, root string) {
	t.Helper()
	dir := filepath.Join(root, "jobs", "_archive", "gone")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	meta := "title: Gone\ncompany: Example\ntrack: SWE  # moved\nstatus_log:\n  - {status: Lead, at: 2026-01-01T00:00:00Z}\n"
	if err := os.WriteFile(filepath.Join(dir, "meta.yml"), []byte(meta), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestRenameTrackRewritesApplicationsArchiveTracksAndRoles(t *testing.T) {
	s := fixtureProfileStore(t)
	root := s.Root()
	writeArchived(t, root)
	impact, err := s.TrackRenameImpact("SWE")
	if err != nil {
		t.Fatal(err)
	}
	if impact.Applications != 10 || !impact.Roles {
		t.Fatalf("impact = %+v, want 10 Applications (9 readable + 1 archived) and roles.json", impact)
	}
	res, err := s.RenameTrack("SWE", "Engineering")
	if err != nil {
		t.Fatal(err)
	}
	if res.Applications != 10 || !res.Roles {
		t.Fatalf("result = %+v", res)
	}
	if got := readTrackOf(t, root, "SWE/orbit"); got != "Engineering" {
		t.Fatalf("orbit track = %q", got)
	}
	arch, _ := os.ReadFile(filepath.Join(root, "jobs/_archive/gone/meta.yml"))
	if !strings.Contains(string(arch), `track: "Engineering"  # moved`) {
		t.Fatalf("archived meta = %s", arch)
	}
	if _, err := os.Stat(filepath.Join(root, "jobs/SWE/orbit/meta.yml")); err != nil {
		t.Fatalf("folder moved: %v", err)
	}
	if got := readTrackOf(t, root, "SWE/malformed"); got != "SWE" {
		t.Fatalf("malformed file touched: %q", got)
	}
	tracks, _ := s.Tracks()
	if !reflect.DeepEqual(tracks, []string{"Engineering", "Platform"}) {
		t.Fatalf("tracks = %v", tracks)
	}
	roles, _ := os.ReadFile(filepath.Join(root, "jobs/roles.json"))
	if !strings.Contains(string(roles), `"track": "Engineering"`) || strings.Contains(string(roles), `"track": "SWE"`) ||
		!strings.Contains(string(roles), `"track": "Platform"`) {
		t.Fatalf("roles.json = %s", roles)
	}
}

// A "track" key nested inside another value — the Owner's notes, or a role's own
// sub-object — is not a role's Track and must be left byte for byte, along with the
// file's formatting and key order.
func TestRenameTrackLeavesNestedTrackKeysAlone(t *testing.T) {
	s := fixtureProfileStore(t)
	root := s.Root()
	const before = `{
  "version": 1,
  "notes": {"track": "SWE"},
  "roles": [
    {"id": "backend", "track": "SWE", "meta": {"track": "SWE"}}
  ]
}
`
	const want = `{
  "version": 1,
  "notes": {"track": "SWE"},
  "roles": [
    {"id": "backend", "track": "Engineering", "meta": {"track": "SWE"}}
  ]
}
`
	if err := os.WriteFile(filepath.Join(root, "jobs", "roles.json"), []byte(before), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RenameTrack("SWE", "Engineering"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(root, "jobs", "roles.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("roles.json =\n%s\nwant\n%s", got, want)
	}
}

func TestRenameTrackSkipsAndNamesMalformed(t *testing.T) {
	s := fixtureProfileStore(t)
	res, err := s.RenameTrack("SWE", "Engineering")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join("SWE", "malformed")}
	if !reflect.DeepEqual(res.Skipped, want) {
		t.Fatalf("skipped = %v, want %v", res.Skipped, want)
	}
}

func TestRenameTrackWithoutRolesFile(t *testing.T) {
	s := fixtureProfileStore(t)
	if err := os.Remove(filepath.Join(s.Root(), "jobs/roles.json")); err != nil {
		t.Fatal(err)
	}
	res, err := s.RenameTrack("Platform", "Infra")
	if err != nil {
		t.Fatal(err)
	}
	if res.Roles || res.Applications != 1 {
		t.Fatalf("result = %+v", res)
	}
}

func TestRenameTrackRefusesBadNamesAndInvalidFilesBeforeWriting(t *testing.T) {
	s := fixtureProfileStore(t)
	for _, name := range []string{"", "  ", "a/b", "platform"} {
		if _, err := s.RenameTrack("SWE", name); err == nil {
			t.Fatalf("%q accepted", name)
		}
	}
	if got := readTrackOf(t, s.Root(), "SWE/orbit"); got != "SWE" {
		t.Fatalf("changed on refusal: %q", got)
	}
	writeWorkspaceJSON(t, s.Root(), `{broken`)
	if _, err := s.RenameTrack("SWE", "Engineering"); err == nil {
		t.Fatal("renamed with invalid workspace.json")
	}
	if got := readTrackOf(t, s.Root(), "SWE/orbit"); got != "SWE" {
		t.Fatalf("meta.yml changed before workspace.json was checked: %q", got)
	}
}

func TestRenameTrackAllowsChangingCaseOnly(t *testing.T) {
	s := fixtureProfileStore(t)
	if _, err := s.RenameTrack("Platform", "PLATFORM"); err != nil {
		t.Fatal(err)
	}
	tracks, _ := s.Tracks()
	if !reflect.DeepEqual(tracks, []string{"SWE", "PLATFORM"}) {
		t.Fatalf("tracks = %v", tracks)
	}
}
