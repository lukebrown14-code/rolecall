package store

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

var keyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*$`)
var entryPattern = regexp.MustCompile(`^  - \{status: ([A-Za-z]+), at: ([^,}]+)(, approx: (true|false))?\}([ \t]+#.*)?$`)

func validStatus(s string) bool { return slices.Contains(Statuses, s) }
func readApplication(dir string, b []byte) Application {
	a := Application{Folder: dir, Schema: 1, Status: StatusLead}
	fail := func(n int, err error) Application {
		a.Malformed = fmt.Errorf("%s:%d: %w", filepath.Join(dir, "meta.yml"), n, err)
		return a
	}
	if !utf8.Valid(b) || strings.Contains(string(b), "\r") || !strings.HasSuffix(string(b), "\n") {
		return fail(1, fmt.Errorf("expected UTF-8, LF and trailing newline"))
	}
	seen := map[string]bool{}
	inLog := false
	for n, line := range strings.Split(strings.TrimSuffix(string(b), "\n"), "\n") {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			m := entryPattern.FindStringSubmatch(line)
			if !inLog || m == nil {
				return fail(n+1, fmt.Errorf("invalid status_log entry or unexpected nesting"))
			}
			if !validStatus(m[1]) {
				return fail(n+1, fmt.Errorf("unknown Status %q", m[1]))
			}
			if _, err := time.Parse(time.RFC3339, m[2]); err != nil {
				if _, err = time.Parse("2006-01-02", m[2]); err != nil {
					return fail(n+1, fmt.Errorf("invalid status timestamp"))
				}
			}
			e := StatusEntry{Status: m[1], At: m[2], Approx: m[4] == "true"}
			a.StatusLog = append(a.StatusLog, e)
			a.Status = e.Status
			a.LastChange = &e
			if e.Status == StatusApplied && a.AppliedDate == nil {
				a.AppliedDate = &e
			}
			continue
		}
		k, raw, ok := strings.Cut(line, ":")
		if !ok || !keyPattern.MatchString(k) || seen[k] {
			return fail(n+1, fmt.Errorf("invalid or duplicate key"))
		}
		seen[k] = true
		inLog = k == "status_log"
		if inLog {
			if strings.TrimSpace(raw) != "" && strings.TrimSpace(raw) != "[]" {
				return fail(n+1, fmt.Errorf("invalid status_log"))
			}
			if strings.TrimSpace(raw) == "[]" {
				inLog = false
			}
			continue
		}
		v, err := scalar(raw)
		if err != nil {
			return fail(n+1, err)
		}
		switch k {
		case "schema":
			if v != "1" {
				return fail(n+1, fmt.Errorf("unsupported schema"))
			}
		case keyTitle:
			a.Title = v
		case keyCompany:
			a.Company = v
		case keyTrack:
			a.Track = v
		case "url":
			a.URL = v
		case "closes":
			if v != "" {
				if _, err := time.Parse("2006-01-02", v); err != nil {
					return fail(n+1, fmt.Errorf("invalid closes date"))
				}
			}
			a.Closes = v
		case "fit":
			switch v {
			case "", FitStrong, FitGood, FitMarginal, FitPoor:
				a.Fit = v
			default:
				return fail(n+1, fmt.Errorf("invalid Fit"))
			}
		case "resume_only":
			if v != "true" && v != "false" {
				return fail(n+1, fmt.Errorf("invalid resume_only"))
			}
			a.ResumeOnly = v == "true"
		}
	}
	for _, k := range []string{keyTitle, keyCompany, keyTrack} {
		if !seen[k] {
			return fail(1, fmt.Errorf("missing %s", k))
		}
	}
	return a
}
