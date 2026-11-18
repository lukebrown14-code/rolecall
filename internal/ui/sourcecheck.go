package ui

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// sourceChecked is a Source check's outcome, delivered to the model. The check is c on a
// Settings › Sources row: the Plugin's sweep.py --test-source against that one Source.
type sourceChecked struct {
	name   string
	result CapturedResult
}

// sourceCheckResult is the one JSON line sweep.py --test-source prints.
type sourceCheckResult struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Count  int    `json:"count"`
	Access string `json:"access"`
	Note   string `json:"note"`
}

// finishSourceCheck reports a Source check as a toast: "greenhouse: works · 41 jobs", or
// the access problem and the script's note. Output that is not the expected line is an error.
func (m model) finishSourceCheck(done sourceChecked) model {
	line := strings.TrimSpace(done.result.Stdout)
	if i := strings.LastIndex(line, "\n"); i >= 0 {
		line = line[i+1:]
	}
	var r sourceCheckResult
	if err := json.Unmarshal([]byte(line), &r); err != nil {
		out := combinedOutput(done.result)
		if out == "" && done.result.Err != nil {
			out = done.result.Err.Error()
		}
		m.err = fmt.Errorf("checking %s failed: %s", done.name, firstLine(out))
		return m
	}
	msg := done.name + ": " + r.Access
	if r.OK {
		msg += " · " + strconv.Itoa(r.Count) + " jobs"
	}
	if r.Note != "" {
		msg += " · " + r.Note
	}
	m.toast, m.toastHint, m.toastKind = msg, "", toastPlain
	return m
}

// firstLine is the first non-blank line of s.
func firstLine(s string) string {
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			return l
		}
	}
	return "no output"
}
