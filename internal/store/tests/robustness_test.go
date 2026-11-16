package store_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lukebrown14-code/rolecall/internal/store"
)

// Two employers advertising one title through the same agency are not one ad; with no
// company stated, the recruiter stands in for it.
func TestLikelyDuplicatesNeedsMoreThanASharedRecruiter(t *testing.T) {
	s := fixtureProfileStore(t) // lead-contract: Quokka Energy via Wattle Talent, Data Engineer
	if got, _ := s.LikelyDuplicates("Other Co", "Wattle Talent", "Data Engineer", ""); len(got) != 0 {
		t.Errorf("shared recruiter alone matched %d", len(got))
	}
	if got, _ := s.LikelyDuplicates("", "Wattle Talent", "Data Engineer", ""); len(got) != 1 {
		t.Errorf("recruiter with no company matched %d, want 1", len(got))
	}
}

// One folder that fails to read is one Malformed Application; the rest still scan.
func TestScanMarksAnUnreadableFolderMalformed(t *testing.T) {
	s := fixtureProfileStore(t)
	before, err := s.Scan()
	if err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(s.Root(), "jobs", "SWE", "broken")
	if err := os.MkdirAll(filepath.Join(bad, "meta.yml"), 0o755); err != nil { // reads as EISDIR
		t.Fatal(err)
	}
	after, err := s.Scan()
	if err != nil {
		t.Fatalf("Scan aborted: %v", err)
	}
	if len(after) != len(before)+1 {
		t.Fatalf("Scan = %d Applications, want %d", len(after), len(before)+1)
	}
	for _, a := range after {
		if a.Folder == bad && a.Malformed == nil {
			t.Fatal("unreadable folder not Malformed")
		}
	}
}

// A symlinked doc is not the harness's file: it does not count and is not read.
func TestSymlinkedDocsAreNotRead(t *testing.T) {
	s := fixtureProfileStore(t)
	outside := filepath.Join(t.TempDir(), "secret.md")
	text := strings.Repeat("Run our data platform end to end. ", 10)
	if err := os.WriteFile(outside, []byte(text+"\n## Gaps\n- secret gap\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	folder := writeLead(t, s, "linked", leadHead)
	for _, name := range []string{"jd.md", "fit.md"} {
		if err := os.Symlink(outside, filepath.Join(folder, name)); err != nil {
			t.Fatal(err)
		}
	}
	a := rescan(t, s, folder)
	if a.HasJD || a.JDHasText || a.HasFit {
		t.Fatalf("symlinked docs counted: %+v", a)
	}
	if got, _ := s.LikelyDuplicates("Elsewhere", "", "Something else", text); len(got) != 0 {
		t.Errorf("symlinked jd.md matched")
	}
	a.HasFit = true // even if a caller claims a fit.md, the symlink is not read
	if gaps := s.CommonGaps([]store.Application{a}); len(gaps) != 0 {
		t.Errorf("symlinked fit.md read: %v", gaps)
	}
}

// renameFailFS fails the nth atomic rename, then behaves.
type renameFailFS struct {
	failingFS
	n *int
}

func (f renameFailFS) Rename(a, b string) error {
	*f.n--
	if *f.n == 0 {
		return errors.New("injected rename")
	}
	return os.Rename(a, b)
}

// A rename that fails partway says how far it got, and running it again finishes it.
func TestRenameTrackFailingPartwayReportsAndResumes(t *testing.T) {
	root := fixtureProfileStore(t).Root()
	n := 2
	s, err := store.OpenWithFileSystem(root, renameFailFS{n: &n})
	if err != nil {
		t.Fatal(err)
	}
	res, err := s.RenameTrack("SWE", "Engineering")
	if err == nil || res.Applications != 1 || !strings.Contains(err.Error(), "renamed 1 ") ||
		!strings.Contains(err.Error(), "run the rename again") {
		t.Fatalf("partial rename = %+v, %v", res, err)
	}
	if tracks, _ := s.Tracks(); tracks[0] != "SWE" {
		t.Fatalf("workspace.json changed before the Applications finished: %v", tracks)
	}
	res, err = s.RenameTrack("SWE", "Engineering")
	if err != nil {
		t.Fatal(err)
	}
	if res.Applications != 12 || !res.Roles {
		t.Fatalf("resumed rename = %+v, want the other 12 Applications and roles.json", res)
	}
	if impact, _ := s.TrackRenameImpact("SWE"); impact.Applications != 0 || impact.Roles {
		t.Fatalf("left on SWE: %+v", impact)
	}
	if tracks, _ := s.Tracks(); tracks[0] != "Engineering" {
		t.Fatalf("tracks = %v", tracks)
	}
}
