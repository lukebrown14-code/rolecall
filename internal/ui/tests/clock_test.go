package ui_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"
)

// fixtureNow is the clock for tests on fixtures/workspace. It sits after every
// status_log date there, so relative dates read "7d ago", never "in 8mo".
var fixtureNow = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

func fixtureClock() time.Time { return fixtureNow }

// fixtureDay is fixtureNow as a meta.yml date, e.g. a new folder's suffix.
var fixtureDay = fixtureNow.Format("2006-01-02")

// Every status_log date in the fixture Workspace is on or before fixtureNow.
func TestFixtureDatesPrecedeTheTestClock(t *testing.T) {
	at := regexp.MustCompile(`at: (\d{4}-\d{2}-\d{2})`)
	err := filepath.WalkDir("../../../fixtures/workspace", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.Name() != "meta.yml" {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range at.FindAllStringSubmatch(string(b), -1) {
			if m[1] > fixtureDay {
				t.Errorf("%s: status date %s is after the test clock %s; move fixtureNow later", path, m[1], fixtureDay)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
