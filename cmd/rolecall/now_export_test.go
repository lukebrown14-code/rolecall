package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var testToday = time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)

func testClock() time.Time { return testToday }

// exportWorkspace has one Interviewing Application with a next step and contacts, one Lead
// closing in 3 days, one Rejected with a reason, and one Applied that is Follow-up due.
func exportWorkspace(t *testing.T) string {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	apps := map[string]string{
		"step": "title: \"=Engineer\"\ncompany: Stepco\ntrack: SWE\nurl: https://example.test/step\nnext_step: {at: 2026-10-03T14:30, what: \"Phone screen, round 1\"}\n" +
			"contacts:\n  - {name: \"Sam Lee\", role: Recruiter, email: sam@example.test}\n  - {url: \"https://example.test/in/kim\"}\n" +
			"status_log:\n  - {status: Applied, at: 2026-09-20T00:00:00Z}\n  - {status: Interviewing, at: 2026-09-28T00:00:00Z}\n",
		"lead": "title: Lead role\ncompany: Leadco\ntrack: SWE\ncloses: 2026-10-04\nstatus_log:\n  - {status: Lead, at: 2026-09-30T00:00:00Z}\n",
		"gone": "title: Gone\ncompany: Goneco\ntrack: SWE\nclose_reason: after-screen\nnext_step: {at: 2026-10-02}\n" +
			"status_log:\n  - {status: Applied, at: 2026-09-01}\n  - {status: Rejected, at: 2026-09-15}\n",
		"stale": "title: Stale\ncompany: Staleco\ntrack: SWE\nstatus_log:\n  - {status: Applied, at: 2026-08-01}\n",
		"bad":   "title: Bad\ncompany: Badco\ntrack: SWE\nnext_step: soon\nstatus_log:\n  - {status: Lead, at: 2026-09-30}\n",
	}
	for name, body := range apps {
		dir := filepath.Join(root, "jobs", "SWE", name)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "meta.yml"), []byte("schema: 1\n"+body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestNowPrintsOnePlainLine(t *testing.T) {
	root := exportWorkspace(t)
	var out, errOut bytes.Buffer
	if err := nowCommand([]string{"--dir", root}, &out, &errOut, testClock); err != nil {
		t.Fatal(err)
	}
	// The Rejected Application's next step is closed business; the malformed one is skipped.
	want := "1 follow-up due | 1 next step within 7 days | 1 closing within 7 days\n"
	if out.String() != want {
		t.Fatalf("got %q, want %q", out.String(), want)
	}
	if strings.ContainsAny(out.String(), "\x1b") {
		t.Fatal("colour in a scriptable line")
	}
}

func TestNowPrintsNothingWhenNothingIsDue(t *testing.T) {
	root := exportWorkspace(t)
	var out bytes.Buffer
	far := func() time.Time { return time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC) }
	if err := nowCommand([]string{"--dir", root, "--days", "1"}, &out, &bytes.Buffer{}, far); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Fatalf("printed %q", out.String())
	}
}

func TestNowListAndDaysWindow(t *testing.T) {
	root := exportWorkspace(t)
	var out bytes.Buffer
	if err := nowCommand([]string{"--dir", root, "--list", "--days", "2"}, &out, &bytes.Buffer{}, testClock); err != nil {
		t.Fatal(err)
	}
	want := "follow-up\tjobs/SWE/stale\t\tStale\tStaleco\n" + "next-step\tjobs/SWE/step\t2026-10-03T14:30\t=Engineer\tStepco\n"
	if out.String() != want {
		t.Fatalf("got %q, want %q", out.String(), want)
	}
}

func TestNowFailsWithoutAWorkspace(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := nowCommand([]string{"--dir", filepath.Join(t.TempDir(), "missing")}, &bytes.Buffer{}, &bytes.Buffer{}, testClock); err == nil {
		t.Fatal("no error for a missing Workspace")
	}
}

func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			b, _ := os.ReadFile(p)
			out[p] = string(b)
		}
		return nil
	})
	return out
}

func TestExportCSVAndJSON(t *testing.T) {
	root := exportWorkspace(t)
	before := snapshotTree(t, root)
	var out, errOut bytes.Buffer
	if err := exportCommand([]string{"--dir", root}, &out, &errOut, testClock); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(errOut.String(), "Skipped 1 Application") {
		t.Errorf("malformed not reported: %q", errOut.String())
	}
	records, err := csv.NewReader(&out).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 5 || strings.Join(records[0], ",") != strings.Join(exportColumns, ",") {
		t.Fatalf("records = %v", records)
	}
	col := func(row []string, name string) string {
		for i, c := range exportColumns {
			if c == name {
				return row[i]
			}
		}
		t.Fatalf("no column %s", name)
		return ""
	}
	var step, gone []string
	for _, r := range records[1:] {
		switch col(r, "folder") {
		case "jobs/SWE/step":
			step = r
		case "jobs/SWE/gone":
			gone = r
		}
	}
	for name, want := range map[string]string{
		"title": "'=Engineer", "status": "Interviewing", "applied": "2026-09-20", "last_change": "2026-09-28",
		"next_step_at": "2026-10-03T14:30", "next_step_what": "Phone screen, round 1",
		"contacts":   "Sam Lee (Recruiter) <sam@example.test>; <https://example.test/in/kim>",
		"status_log": "Applied 2026-09-20; Interviewing 2026-09-28",
	} {
		if got := col(step, name); got != want {
			t.Errorf("step %s = %q, want %q", name, got, want)
		}
	}
	if got := col(gone, "close_reason"); got != "after-screen" {
		t.Errorf("close_reason = %q", got)
	}

	out.Reset()
	if err := exportCommand([]string{"--dir", root, "--format", "json"}, &out, &bytes.Buffer{}, testClock); err != nil {
		t.Fatal(err)
	}
	var rows []exportRow
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil || len(rows) != 4 {
		t.Fatalf("json: %v, %d rows", err, len(rows))
	}
	for _, r := range rows {
		if r.Folder == "jobs/SWE/step" && (len(r.Contacts) != 2 || r.Contacts[1].URL == "" || len(r.StatusLog) != 2) {
			t.Errorf("step row = %+v", r)
		}
	}
	if !sameTree(before, snapshotTree(t, root)) {
		t.Fatal("export changed the Workspace")
	}
}

// The export dates a timestamp on its day in the Owner's zone (the clock's), as the rows
// and Stats do; a date-only entry keeps the day written.
func TestExportDatesAreTheOwnersLocalDay(t *testing.T) {
	root := exportWorkspace(t)
	west := func() time.Time { return testToday.In(time.FixedZone("PDT", -7*60*60)) }
	var out bytes.Buffer
	if err := exportCommand([]string{"--dir", root, "--format", "json"}, &out, &bytes.Buffer{}, west); err != nil {
		t.Fatal(err)
	}
	var rows []exportRow
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, r := range rows {
		got[r.Folder] = r.Applied + " " + r.LastChange
	}
	for folder, want := range map[string]string{
		"jobs/SWE/step": "2026-09-19 2026-09-27", // 00:00Z is the evening before in PDT
		"jobs/SWE/gone": "2026-09-01 2026-09-15",
	} {
		if got[folder] != want {
			t.Errorf("%s applied/last_change = %q, want %q", folder, got[folder], want)
		}
	}
	out.Reset()
	if err := exportCommand([]string{"--dir", root}, &out, &bytes.Buffer{}, west); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Applied 2026-09-19; Interviewing 2026-09-27") {
		t.Errorf("csv status_log not in the Owner's zone:\n%s", out.String())
	}
}

func sameTree(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func TestExportICS(t *testing.T) {
	root := exportWorkspace(t)
	var out bytes.Buffer
	if err := exportCommand([]string{"--dir", root, "--ics"}, &out, &bytes.Buffer{}, testClock); err != nil {
		t.Fatal(err)
	}
	ics := out.String()
	for _, want := range []string{
		"BEGIN:VCALENDAR\r\n", "END:VCALENDAR\r\n",
		"DTSTART:20261003T143000\r\n", "DTEND:20261003T153000\r\n",
		"SUMMARY:Phone screen\\, round 1: =Engineer (Stepco)\r\n",
		"DTSTART;VALUE=DATE:20261004\r\n", "SUMMARY:Closes: Lead role (Leadco)\r\n",
		"DTSTAMP:20261001T090000Z\r\n",
	} {
		if !strings.Contains(ics, want) {
			t.Errorf("ics lacks %q:\n%s", want, ics)
		}
	}
	// A Rejected Application's next step is not a calendar event.
	if strings.Contains(ics, "Goneco") || strings.Count(ics, "BEGIN:VEVENT") != 2 {
		t.Errorf("wrong events:\n%s", ics)
	}
	for _, l := range strings.Split(strings.TrimSuffix(ics, "\r\n"), "\r\n") {
		if len(l) > 75 {
			t.Errorf("line over 75 octets: %q", l)
		}
	}
}

func TestExportWritesOnlyWhereOutNames(t *testing.T) {
	root := exportWorkspace(t)
	before := snapshotTree(t, root)
	dest := filepath.Join(t.TempDir(), "applications.csv")
	var out bytes.Buffer
	if err := exportCommand([]string{"--dir", root, "--out", dest}, &out, &bytes.Buffer{}, testClock); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Errorf("stdout got %q with --out", out.String())
	}
	info, err := os.Stat(dest)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("out file: %v %v", info, err)
	}
	if !sameTree(before, snapshotTree(t, root)) {
		t.Fatal("export changed the Workspace")
	}
	if err := exportCommand([]string{"--dir", root, "--out", t.TempDir()}, &out, &bytes.Buffer{}, testClock); err == nil {
		t.Fatal("wrote over a folder")
	}
}

func TestExportRefusesBadFlags(t *testing.T) {
	root := exportWorkspace(t)
	for _, args := range [][]string{{"--format", "xml"}, {"--ics", "--format", "csv"}, {"extra"}} {
		if err := exportCommand(append([]string{"--dir", root}, args...), &bytes.Buffer{}, &bytes.Buffer{}, testClock); err == nil {
			t.Errorf("accepted %v", args)
		}
	}
}
