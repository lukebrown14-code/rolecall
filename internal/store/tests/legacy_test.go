package store_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLegacyFilesFindsDashboardEraFilesInJobsAndRoot(t *testing.T) {
	s := fixtureProfileStore(t)
	if got := s.LegacyFiles(); len(got) != 0 {
		t.Fatalf("fixture Workspace LegacyFiles = %v, want none", got)
	}
	for _, rel := range []string{"jobs/SHORTLIST.md", "SHORTLIST.md", "jobs/TRACKER.md", "jobs/sweep-candidates.md"} {
		if err := os.WriteFile(filepath.Join(s.Root(), rel), []byte("# x\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{filepath.Join("jobs", "TRACKER.md"), filepath.Join("jobs", "SHORTLIST.md"), "SHORTLIST.md"}
	if got := s.LegacyFiles(); !reflect.DeepEqual(got, want) {
		t.Fatalf("LegacyFiles = %v, want %v (sweep-candidates.md is current, not legacy)", got, want)
	}
}
