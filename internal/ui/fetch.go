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
	args := []string{filepath.Join(m.plugin, "scripts", "new-job.py"), "--fetch-only", "--url", url, "--data", m.store.Root()}
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

// fetchReason is the first non-blank line the script printed, or a generic reason.
func fetchReason(r CapturedResult) string {
	for _, line := range strings.Split(combinedOutput(r), "\n") {
		if s := strings.TrimSpace(line); s != "" {
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
