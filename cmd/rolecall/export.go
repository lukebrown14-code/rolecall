package main

import (
	"bytes"
	"crypto/sha1" //nolint:gosec // a stable calendar UID, not security
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lukebrown14-code/rolecall/internal/reporting"
	"github.com/lukebrown14-code/rolecall/internal/store"
)

// exportCommand is `rolecall export [--dir PATH] [--format csv|json | --ics] [--out FILE]`.
// It is read-only: nothing in the Workspace changes, and the only file it writes is the
// one --out names (ADR-0005: Owner data leaves the Workspace only where the Owner sends
// it). Without --out the export goes to stdout.
func exportCommand(args []string, stdout, stderr io.Writer, clock func() time.Time) error {
	flags := flag.NewFlagSet("export", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dir := flags.String("dir", "", "Workspace path (default: workspace in config.toml)")
	format := flags.String("format", "csv", "csv or json: every Application")
	ics := flags.Bool("ics", false, "an .ics calendar of next steps and close dates instead")
	out := flags.String("out", "", "write to this file instead of stdout")
	flags.Usage = func() {
		_, _ = fmt.Fprintln(flags.Output(), "usage: rolecall export [--dir PATH] [--format csv|json | --ics] [--out FILE]\n\n"+
			"Exports every Application (title, company, Track, Status, dates, next step, contacts,\n"+
			"close reason, status log), or with --ics a calendar of next steps and the close dates\n"+
			"of Leads and Drafts. Read-only. Prints to stdout; --out FILE writes there (the file\n"+
			"holds private data, so it is created readable by you only).")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return err
	}
	formatGiven := false
	flags.Visit(func(f *flag.Flag) { formatGiven = formatGiven || f.Name == "format" })
	switch {
	case flags.NArg() != 0:
		flags.Usage()
		return errors.New("export takes no arguments")
	case *ics && formatGiven:
		return errors.New("--ics and --format cannot be used together")
	case !*ics && *format != "csv" && *format != "json":
		return fmt.Errorf("unknown --format %q; use csv or json", *format)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	s, err := openWorkspace(home, *dir)
	if err != nil {
		return err
	}
	apps, err := s.Scan()
	if err != nil {
		return err
	}
	var readable []store.Application
	skipped := 0
	for _, a := range apps {
		if a.Malformed != nil {
			skipped++
			continue
		}
		readable = append(readable, a)
	}
	var data []byte
	switch {
	case *ics:
		data = icsCalendar(readable, clock())
	case *format == "json":
		data, err = exportJSON(s.Root(), readable)
	default:
		data, err = exportCSV(s.Root(), readable)
	}
	if err != nil {
		return err
	}
	if skipped > 0 {
		_, _ = fmt.Fprintf(stderr, "Skipped %d Application(s) whose meta.yml is malformed (run rolecall doctor)\n", skipped)
	}
	if *out == "" {
		_, err = stdout.Write(data)
		return err
	}
	return writePrivateFile(*out, data)
}

// writePrivateFile writes data to path through a temporary file beside it, so a failed
// export never leaves half a file, readable by the Owner only.
func writePrivateFile(path string, data []byte) error {
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return fmt.Errorf("--out %s is a folder; name a file", path)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".rolecall-export-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if err = tmp.Chmod(privateMode); err == nil {
		_, err = tmp.Write(data)
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// exportLog is one status_log entry as exported.
type exportLog struct {
	Status string `json:"status"`
	At     string `json:"at"`
}

// exportContact is one Contact as exported.
type exportContact struct {
	Name  string `json:"name,omitempty"`
	Role  string `json:"role,omitempty"`
	Email string `json:"email,omitempty"`
	URL   string `json:"url,omitempty"`
}

// exportRow is one Application as exported: the core fields, the dates the Status log
// gives, and what this release added. Undone moves are left out of the dates and the
// log, as the app leaves them out.
type exportRow struct {
	Folder       string          `json:"folder"`
	Title        string          `json:"title"`
	Company      string          `json:"company"`
	Track        string          `json:"track"`
	Status       string          `json:"status"`
	Fit          string          `json:"fit"`
	URL          string          `json:"url"`
	Closes       string          `json:"closes"`
	Posted       string          `json:"posted"`
	Source       string          `json:"source"`
	Recruiter    string          `json:"recruiter"`
	Applied      string          `json:"applied"`
	LastChange   string          `json:"last_change"`
	NextStepAt   string          `json:"next_step_at"`
	NextStepWhat string          `json:"next_step_what"`
	Contacts     []exportContact `json:"contacts"`
	CloseReason  string          `json:"close_reason"`
	StatusLog    []exportLog     `json:"status_log"`
}

var exportColumns = []string{"folder", "title", "company", "track", "status", "fit", "url", "closes", "posted", "source",
	"recruiter", "applied", "last_change", "next_step_at", "next_step_what", "contacts", "close_reason", "status_log"}

// privateMode is the export file's permissions: the Owner's alone.
const privateMode = 0o600

func day(at string) string {
	d, _, _ := strings.Cut(at, "T")
	return d
}

func exportRows(root string, apps []store.Application) []exportRow {
	rows := make([]exportRow, 0, len(apps))
	for _, a := range apps {
		rel, err := filepath.Rel(root, a.Folder)
		if err != nil {
			rel = a.Folder
		}
		log := reporting.EffectiveLog(a.StatusLog)
		if len(log) == 0 || log[len(log)-1].Status != a.Status {
			log = a.StatusLog
		}
		r := exportRow{Folder: filepath.ToSlash(rel), Title: a.Title, Company: a.Company, Track: a.Track, Status: a.Status, Fit: a.Fit,
			URL: a.URL, Closes: a.Closes, Posted: a.Posted, Source: a.Source, Recruiter: a.Recruiter, Contacts: []exportContact{},
			StatusLog: []exportLog{}}
		for _, e := range log {
			r.StatusLog = append(r.StatusLog, exportLog{Status: e.Status, At: e.At})
			if e.Status == store.StatusApplied && r.Applied == "" {
				r.Applied = day(e.At)
			}
			r.LastChange = day(e.At)
		}
		if a.NextStep != nil {
			r.NextStepAt, r.NextStepWhat = a.NextStep.At, a.NextStep.What
		}
		for _, c := range a.Contacts {
			r.Contacts = append(r.Contacts, exportContact{Name: c.Name, Role: c.Role, Email: c.Email, URL: c.URL})
		}
		if a.Status == store.StatusRejected || a.Status == store.StatusWithdrawn {
			r.CloseReason = a.CloseReason
		}
		rows = append(rows, r)
	}
	return rows
}

func exportJSON(root string, apps []store.Application) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(exportRows(root, apps)); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// csvSafe stops a spreadsheet reading a cell as a formula: an ad's title is text from the
// web, and one that begins "=" or "@" must stay text.
func csvSafe(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

func exportCSV(root string, apps []store.Application) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.Write(exportColumns); err != nil {
		return nil, err
	}
	for _, r := range exportRows(root, apps) {
		var contacts, log []string
		for _, c := range r.Contacts {
			label := c.Name
			if c.Role != "" {
				label += " (" + c.Role + ")"
			}
			if reach := c.Email + c.URL; reach != "" {
				label = strings.TrimSpace(label + " <" + reach + ">")
			}
			contacts = append(contacts, label)
		}
		for _, e := range r.StatusLog {
			log = append(log, e.Status+" "+day(e.At))
		}
		cells := []string{r.Folder, r.Title, r.Company, r.Track, r.Status, r.Fit, r.URL, r.Closes, r.Posted, r.Source, r.Recruiter,
			r.Applied, r.LastChange, r.NextStepAt, r.NextStepWhat, strings.Join(contacts, "; "), r.CloseReason, strings.Join(log, "; ")}
		for i := range cells {
			cells[i] = csvSafe(cells[i])
		}
		if err := w.Write(cells); err != nil {
			return nil, err
		}
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}

// icsText escapes text for an iCalendar value (RFC 5545 §3.3.11).
func icsText(s string) string {
	return strings.NewReplacer(`\`, `\\`, ";", `\;`, ",", `\,`, "\r\n", `\n`, "\n", `\n`, "\r", `\n`).Replace(s)
}

// icsFold wraps a content line at 75 octets, continuing on lines that start with a space,
// without splitting a UTF-8 character.
func icsFold(line string) string {
	const limit = 75
	var out strings.Builder
	n := 0
	for _, r := range line {
		size := len(string(r))
		if n+size > limit {
			out.WriteString("\r\n ")
			n = 1
		}
		out.WriteRune(r)
		n += size
	}
	return out.String()
}

func icsUID(folder, kind string) string {
	sum := sha1.Sum([]byte(folder + "|" + kind)) //nolint:gosec // see import
	return hex.EncodeToString(sum[:8]) + "@rolecall"
}

// icsCalendar lists each open Application's next step (a timed event of an hour when it
// has a time, otherwise all day) and each Lead's or Draft's close date (all day). It uses
// floating local times, so a calendar shows them at the hour the Owner typed.
func icsCalendar(apps []store.Application, now time.Time) []byte {
	var lines []string
	add := func(l ...string) { lines = append(lines, l...) }
	add("BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//rolecall//export//EN", "CALSCALE:GREGORIAN")
	stamp := now.UTC().Format("20060102T150405Z")
	name := func(a store.Application) string {
		if a.Company == "" {
			return a.Title
		}
		return a.Title + " (" + a.Company + ")"
	}
	for _, a := range apps {
		if ns := a.NextStep; ns != nil && a.Status != store.StatusRejected && a.Status != store.StatusWithdrawn && a.Status != store.StatusDismissed {
			summary := "Next step: " + name(a)
			if ns.What != "" {
				summary = ns.What + ": " + name(a)
			}
			add("BEGIN:VEVENT", "UID:"+icsUID(a.Folder, "next"), "DTSTAMP:"+stamp)
			if clock := ns.Time(); clock != "" {
				start, _ := time.Parse("2006-01-02T15:04", ns.At)
				add("DTSTART:"+start.Format("20060102T150405"), "DTEND:"+start.Add(time.Hour).Format("20060102T150405"))
			} else {
				start, _ := time.Parse(time.DateOnly, ns.At)
				add("DTSTART;VALUE=DATE:"+start.Format("20060102"), "DTEND;VALUE=DATE:"+start.AddDate(0, 0, 1).Format("20060102"))
			}
			add("SUMMARY:" + icsText(summary))
			if a.URL != "" {
				add("DESCRIPTION:" + icsText(a.URL))
			}
			add("END:VEVENT")
		}
		if a.Unsent() && a.Closes != "" {
			if start, err := time.Parse(time.DateOnly, a.Closes); err == nil {
				add("BEGIN:VEVENT", "UID:"+icsUID(a.Folder, "closes"), "DTSTAMP:"+stamp,
					"DTSTART;VALUE=DATE:"+start.Format("20060102"), "DTEND;VALUE=DATE:"+start.AddDate(0, 0, 1).Format("20060102"),
					"SUMMARY:"+icsText("Closes: "+name(a)))
				if a.URL != "" {
					add("DESCRIPTION:" + icsText(a.URL))
				}
				add("END:VEVENT")
			}
		}
	}
	add("END:VCALENDAR")
	var b strings.Builder
	for _, l := range lines {
		b.WriteString(icsFold(l) + "\r\n")
	}
	return []byte(b.String())
}
