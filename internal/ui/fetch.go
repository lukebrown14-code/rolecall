package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// fetchGenericReason is shown when a failed fetch left no usable one-line reason.
const fetchGenericReason = "Could not fetch the job description"

// fetchMeta is the subset of the Plugin's JSON meta the Add form fills in.
type fetchMeta struct {
	Role    string `json:"role"`
	Company string `json:"company"`
	Closes  string `json:"closes"`
}

// fetchPayload is `new-job.py --fetch-only`'s JSON output. signals is a list of
// (label, kind) pairs from the Plugin's visa/clearance scan.
type fetchPayload struct {
	JD      string     `json:"jd"`
	Signals [][]string `json:"signals"`
	Meta    fetchMeta  `json:"meta"`
}

// fetchFinished is a URL fetch's outcome, delivered to the open form. failed names the
// one-line reason when the fetch did not produce a usable ad; it is empty on success.
type fetchFinished struct {
	url     string
	jd      string
	signals []string
	meta    fetchMeta
	failed  string
}

// fetching reports whether a URL fetch is in flight in the open form.
func (m model) fetching() bool {
	fm, _ := m.overlay.(*formMode)
	return fm != nil && fm.form.Fetching
}

// wantsFetch reports whether Enter on the URL would fetch, ignoring whether a captured
// run is already in flight.
func (fm *formMode) wantsFetch(m *model) bool {
	f := fm.form
	return fm.editOf == nil && m.plugin != "" &&
		f.Focused().Label == fieldURL && strings.TrimSpace(f.Value(fieldURL)) != ""
}

// startFetch checks the URL for a duplicate first: a known URL shows the existing
// warning and never fetches. A new URL starts a captured fetch with a spinner.
func (fm *formMode) startFetch(m *model) (mode, tea.Cmd) {
	url := strings.TrimSpace(fm.form.Value(fieldURL))
	matches, err := m.store.FindByURL(url)
	if err != nil {
		m.err = err
		return fm, nil
	}
	if len(matches) > 0 {
		return &duplicateMode{existing: matches[0], fields: createFields(readAddValues(fm.form))}, nil
	}
	fm.form.Fetching, fm.form.FetchErr = true, ""
	m.capturedRun = true
	return fm, tea.Batch(m.fetch(url), m.spinner.Tick)
}

// fetch runs the Plugin's new-job.py in captured mode. The form stays responsive; the
// URL field shows the spinner until fetchFinished arrives.
func (m model) fetch(url string) tea.Cmd {
	args := []string{filepath.Join(m.plugin, "scripts", "new-job.py"), "--fetch-only", "--url", url, dataFlag, m.store.Root()}
	return m.runCaptured("python3", args, func(r CapturedResult) tea.Msg {
		return parseFetch(url, r)
	})
}

// parseFetch turns a captured run into a fetch outcome: the parsed JSON on success, a
// one-line reason otherwise. Unparsable output and a missing python3 get a generic one.
func parseFetch(url string, r CapturedResult) fetchFinished {
	if r.Err != nil {
		return fetchFinished{url: url, failed: fetchReason(r)}
	}
	var p fetchPayload
	if err := json.Unmarshal([]byte(r.Stdout), &p); err != nil || strings.TrimSpace(p.JD) == "" {
		return fetchFinished{url: url, failed: fetchGenericReason}
	}
	return fetchFinished{url: url, jd: p.JD, signals: signalLabels(p.Signals), meta: p.Meta}
}

// fetchWarningBanner is the heading new-job.py's warn() prints above its reason.
const fetchWarningBanner = "WARNING"

// fetchFailure is the one JSON line new-job.py --fetch-only ends a failure with:
// {"error": "...", "kind": "login_wall"}. The kind picks the plain-language message.
type fetchFailure struct {
	Error string `json:"error"`
	Kind  string `json:"kind"`
}

// fetchKindReasons is the message shown for each failure kind the script reports.
var fetchKindReasons = map[string]string{
	"login_wall":  "That site needs a login, so rolecall cannot read it. Paste the job description instead",
	"blocked":     "The site refused the request, as many job sites do. Paste the job description instead",
	"not_found":   "That page was not found; the ad may have been taken down. Check the link",
	"timeout":     "The site took too long to answer. Try again, or paste the job description",
	"bad_url":     "That is not a web address; it must start with http:// or https://",
	"non_html":    "That link is a file, not a web page. Open it and paste the job description instead",
	"unreachable": "Could not reach the site. Check your connection and the link",
	"thin_page":   "The page loaded but its ad is built in the browser. Paste the job description instead",
	"too_short":   "The page held too little text to be a job ad. Paste the job description instead",
}

// fetchReason is the message for a failed run: the kind's own wording when the script's
// last JSON line has a known kind, else that line's error text, else the first line the
// script printed after warn()'s banner, else a generic reason.
func fetchReason(r CapturedResult) string {
	lines := strings.Split(combinedOutput(r), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		var f fetchFailure
		if err := json.Unmarshal([]byte(strings.TrimSpace(lines[i])), &f); err != nil || f.Kind == "" {
			continue
		}
		if reason, ok := fetchKindReasons[f.Kind]; ok {
			return reason
		}
		if s := strings.TrimSpace(f.Error); s != "" {
			return s
		}
	}
	for _, line := range lines {
		if s := strings.TrimSpace(line); s != "" && s != fetchWarningBanner {
			return s
		}
	}
	return fetchGenericReason
}

// signalLabels keeps the label of each (label, kind) signal.
func signalLabels(signals [][]string) []string {
	var out []string
	for _, s := range signals {
		if len(s) > 0 && strings.TrimSpace(s[0]) != "" {
			out = append(out, strings.TrimSpace(s[0]))
		}
	}
	return out
}

// finishFetch records a fetch's result on the form. A success fills Title, Company and
// Closes only where they are still empty and shows the signals; the URL is never
// overwritten. A failure shows one line and lets the form go on by hand.
func (fm *formMode) finishFetch(m *model, done fetchFinished) {
	f := fm.form
	f.Fetching = false
	if strings.TrimSpace(f.Value(fieldURL)) != done.url {
		return // the URL changed while the fetch was in flight
	}
	if done.failed != "" {
		f.FetchErr, f.Signals, f.Fetched, f.Jd = done.failed, nil, false, ""
		return
	}
	f.FetchErr, f.Signals = "", done.signals
	f.Fetched, f.Jd = true, done.jd
	if strings.TrimSpace(f.Value("Title")) == "" && done.meta.Role != "" {
		f.SetValue("Title", done.meta.Role)
	}
	if strings.TrimSpace(f.Value("Company")) == "" && done.meta.Company != "" {
		f.SetValue("Company", done.meta.Company)
	}
	if strings.TrimSpace(f.Value("Closes")) == "" && done.meta.Closes != "" {
		f.SetValue("Closes", done.meta.Closes)
	}
}
