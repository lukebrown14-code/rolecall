package store

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

var keyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*$`)
var entryPattern = regexp.MustCompile(`^  - \{status: ([A-Za-z]+), at: ([^,}]+)(, approx: (true|false))?\}([ \t]+#.*)?$`)

// The meta.yml keys a Sweep writes about where a Lead came from and which Search rule,
// if any, dismissed it.
const (
	keyPosted          = "posted"
	keyPostedLabel     = "posted_label"
	keyApplicants      = "applicants"
	keySource          = "source"
	keyFilterReason    = "filter_reason"
	keyFilterNote      = "filter_note"
	keyFilterOverruled = "filter_overruled"
)

// The Search rules a filter_reason can name.
const (
	FilterExpired        = "expired"
	FilterBlocker        = "blocker"
	FilterApplicantCap   = "applicant_cap"
	FilterLocation       = "location"
	FilterContractLength = "contract_length"
	FilterClosed         = "closed"
	FilterPulled         = "pulled"
	FilterDuplicate      = "duplicate"
)

// FilterReasons are the valid filter_reason values.
var FilterReasons = []string{FilterExpired, FilterBlocker, FilterApplicantCap, FilterLocation, FilterContractLength,
	FilterClosed, FilterPulled, FilterDuplicate}

func validStatus(s string) bool { return slices.Contains(Statuses, s) }

// blank is SCHEMA's blank line or value: empty or only spaces and tabs. strings.TrimSpace
// would also take a vertical tab or form feed for one.
func blank(s string) bool { return strings.Trim(s, " \t") == "" }

// wholeNumber reads SCHEMA's whole number (schema, applicants, term_months): ASCII digits
// only, no sign, within a signed 64-bit integer. strconv.Atoi alone accepts "+5".
func wholeNumber(v string) (int, bool) {
	if !wholePattern.MatchString(v) {
		return 0, false
	}
	n, err := strconv.ParseInt(v, 10, 64)
	return int(n), err == nil
}

// validTimestamp is a status_log at: RFC 3339 with an offset of at most 23:59, or a date.
// time.Parse alone takes a +24:00 or +23:60 offset.
func validTimestamp(s string) bool {
	if _, err := time.Parse(time.RFC3339, s); err != nil {
		_, err = time.Parse(time.DateOnly, s)
		return err == nil
	}
	if strings.HasSuffix(s, "Z") {
		return true
	}
	offset := s[len(s)-len("00:00"):]
	return offset[:2] <= "23" && offset[3:] <= "59"
}

// ParseError is a meta.yml the store cannot read: the file, the first line it failed
// on, and why. Its text is "path:line: reason".
type ParseError struct {
	Path   string
	Line   int
	Reason error
}

func (e *ParseError) Error() string {
	return DisplayText(fmt.Sprintf("%s:%d: %v", e.Path, e.Line, e.Reason))
}

func (e *ParseError) Unwrap() error { return e.Reason }
func readApplication(dir string, b []byte) Application {
	a := Application{Folder: dir, Schema: 1, Status: StatusLead}
	fail := func(n int, err error) Application {
		a.Malformed = &ParseError{Path: filepath.Join(dir, "meta.yml"), Line: n, Reason: err}
		return a
	}
	if !utf8.Valid(b) || strings.Contains(string(b), "\r") || !strings.HasSuffix(string(b), "\n") {
		return fail(1, fmt.Errorf("expected UTF-8, LF and trailing newline"))
	}
	seen := map[string]bool{}
	inLog, inContacts := false, false
	legacy := ""
	for n, line := range strings.Split(strings.TrimSuffix(string(b), "\n"), "\n") {
		if blank(line) || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			if inContacts {
				c, err := readContactEntry(line)
				if err != nil {
					return fail(n+1, err)
				}
				a.Contacts = append(a.Contacts, c)
				continue
			}
			m := entryPattern.FindStringSubmatch(line)
			if !inLog || m == nil {
				return fail(n+1, fmt.Errorf("invalid status_log entry or unexpected nesting"))
			}
			if !validStatus(m[1]) {
				return fail(n+1, fmt.Errorf("unknown Status"))
			}
			if !validTimestamp(m[2]) {
				return fail(n+1, fmt.Errorf("invalid status timestamp"))
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
		inContacts = k == keyContacts
		if inContacts {
			if t := strings.Trim(raw, " \t"); t != "" && t != "[]" {
				return fail(n+1, fmt.Errorf("invalid contacts"))
			}
			inContacts = blank(raw)
			continue
		}
		if k == keyNextStep {
			ns, err := readNextStep(raw)
			if err != nil {
				return fail(n+1, err)
			}
			a.NextStep = ns
			continue
		}
		if inLog {
			if t := strings.Trim(raw, " \t"); t != "" && t != "[]" {
				return fail(n+1, fmt.Errorf("invalid status_log"))
			}
			if strings.Trim(raw, " \t") == "[]" {
				inLog = false
			}
			continue
		}
		v, err := scalar(raw)
		if err != nil {
			return fail(n+1, err)
		}
		if handled, err := a.readAdFact(k, v); handled {
			if err != nil {
				return fail(n+1, err)
			}
			continue
		}
		switch k {
		case "schema":
			version, ok := wholeNumber(v)
			if ok && version > 1 {
				return fail(n+1, fmt.Errorf("schema %d is newer than this version; upgrade rolecall", version))
			}
			if !ok || version != 1 {
				return fail(n+1, fmt.Errorf("unsupported schema"))
			}
		// The strings the UI draws are cleaned once here, so no escape a scraped ad
		// carried reaches the terminal (DisplayText).
		case keyTitle:
			a.Title = DisplayText(v)
		case keyCompany:
			a.Company = DisplayText(v)
		case keyTrack:
			// Spaces around a Track are noise from a hand edit: " SWE " is SWE, not a
			// second Track that workspace.json could never list.
			a.Track = strings.TrimSpace(DisplayText(v))
		case "url":
			a.URL = DisplayText(v)
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
		case keyPosted:
			if v != "" {
				if _, err := time.Parse("2006-01-02", v); err != nil {
					return fail(n+1, fmt.Errorf("invalid posted date"))
				}
			}
			a.Posted = v
		case keyPostedLabel:
			a.PostedLabel = DisplayText(v)
		case keyApplicants:
			if v != "" {
				count, ok := wholeNumber(v)
				if !ok {
					return fail(n+1, fmt.Errorf("invalid applicants"))
				}
				a.Applicants = &count
			}
		case keySource:
			a.Source = strings.TrimSpace(DisplayText(v))
		case keyFilterReason:
			if v != "" && !slices.Contains(FilterReasons, v) {
				return fail(n+1, fmt.Errorf("invalid filter_reason"))
			}
			a.FilterReason = v
		case keyFilterNote:
			a.FilterNote = DisplayText(v)
		case keyLegacyContact:
			legacy = v
		case keyCloseReason:
			a.CloseReason = strings.TrimSpace(DisplayText(v))
		case keyFollowedUp:
			a.FollowedUp = readFollowedUp(v)
		case keyFilterOverruled:
			if v != "true" && v != "false" {
				return fail(n+1, fmt.Errorf("invalid filter_overruled"))
			}
			a.FilterOverruled = v == "true"
		case keyClosesUnclear:
			if v != "true" && v != "false" {
				return fail(n+1, fmt.Errorf("invalid closes_unclear"))
			}
			a.ClosesUnclear = v == "true"
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
	// The dashboard era's single `contact` still reads, as the one Contact, until the
	// Owner saves a contacts list.
	if c, ok := legacyContact(legacy); ok && !seen[keyContacts] {
		a.Contacts = []Contact{c}
	}
	return a
}
