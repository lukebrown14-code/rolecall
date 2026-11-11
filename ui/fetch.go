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
func (m model) fetching() bool { return m.form != nil && m.form.fetching }

// canFetch reports whether Enter should fetch: the Add form (never Edit), the URL
// field focused, a non-empty URL, a configured Plugin, and no fetch already running.
func (m model) canFetch() bool {
	f := m.form
	return m.editOf == nil && m.plugin != "" && f != nil && !f.fetching &&
		f.focused().label == fieldURL && strings.TrimSpace(f.value(fieldURL)) != ""
}

// startFetch checks the URL for a duplicate first: a known URL shows the existing
// warning and never fetches. A new URL starts a captured fetch with a spinner.
func (m model) startFetch() (tea.Model, tea.Cmd) {
	url := strings.TrimSpace(m.form.value(fieldURL))
	matches, err := m.store.FindByURL(url)
	if err != nil {
		m.err = err
		return m, nil
	}
	if len(matches) > 0 {
		fields := createFields(m.form.addValues())
		m.form = nil
		m.duplicate, m.duplicateFields = &matches[0], fields
		return m, nil
	}
	m.form.fetching, m.form.fetchErr = true, ""
	return m, tea.Batch(m.fetch(url), m.spinner.Tick)
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
func (m model) finishFetch(done fetchFinished) (tea.Model, tea.Cmd) {
	f := m.form
	if f == nil {
		return m, nil
	}
	f.fetching = false
	if strings.TrimSpace(f.value(fieldURL)) != done.url {
		return m, nil // the URL changed while the fetch was in flight
	}
	if done.failed != "" {
		f.fetchErr, f.signals, f.fetched, f.jd = done.failed, nil, false, ""
		return m, nil
	}
	f.fetchErr, f.signals = "", done.signals
	f.fetched, f.jd = true, done.jd
	if strings.TrimSpace(f.value("Title")) == "" && done.meta.Role != "" {
		f.setValue("Title", done.meta.Role)
	}
	if strings.TrimSpace(f.value("Company")) == "" && done.meta.Company != "" {
		f.setValue("Company", done.meta.Company)
	}
	if strings.TrimSpace(f.value("Closes")) == "" && done.meta.Closes != "" {
		f.setValue("Closes", done.meta.Closes)
	}
	return m, nil
}
