package store

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
)

// NextStep is what the Owner does next on an Application and when: "Phone screen" on
// 2026-12-03T10:30. At is YYYY-MM-DD or YYYY-MM-DDTHH:MM (local time, no zone).
type NextStep struct{ At, What string }

const (
	keyNextStep      = "next_step"
	keyContacts      = "contacts"
	keyLegacyContact = "contact"
	keyCloseReason   = "close_reason"
)

var nextStepAt = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}(T\d{2}:\d{2})?$`)

// ValidNextStepAt reports whether s is YYYY-MM-DD or YYYY-MM-DDTHH:MM and a real date.
func ValidNextStepAt(s string) bool {
	if !nextStepAt.MatchString(s) {
		return false
	}
	layout := time.DateOnly
	if len(s) > len(time.DateOnly) {
		layout = "2006-01-02T15:04"
	}
	_, err := time.Parse(layout, s)
	return err == nil
}

// Day is the date part of At.
func (n NextStep) Day() string { return n.At[:min(len(n.At), len(time.DateOnly))] }

// Time is At as a clock time too, or "" for a date-only step.
func (n NextStep) Time() string {
	if len(n.At) > len(time.DateOnly)+1 {
		return n.At[len(time.DateOnly)+1:]
	}
	return ""
}

// readNextStep reads a next_step line's value (the text after "next_step:"). An empty
// value reads as no next step.
func readNextStep(raw string) (*NextStep, error) {
	if blank(raw) {
		return nil, nil
	}
	m, _, err := parseFlow(raw)
	if err != nil {
		return nil, err
	}
	at := m["at"]
	if !ValidNextStepAt(at) {
		return nil, fmt.Errorf("invalid next_step at (YYYY-MM-DD or YYYY-MM-DDTHH:MM)")
	}
	return &NextStep{At: at, What: DisplayText(m["what"])}, nil
}

// SetNextStep writes next_step, or removes it when ns is nil. Only that line changes.
func (s *Store) SetNextStep(folder string, ns *NextStep) error {
	if ns != nil && !ValidNextStepAt(ns.At) {
		return fmt.Errorf("invalid next_step at %q", ns.At)
	}
	return s.update(folder, func(lines []string) ([]string, error) {
		i := slices.IndexFunc(lines, func(l string) bool { return strings.HasPrefix(l, keyNextStep+":") })
		if ns == nil {
			if i >= 0 {
				lines = slices.Delete(lines, i, i+1)
			}
			return lines, nil
		}
		line := keyNextStep + ": " + flowEntry("at", ns.At, "what", strings.TrimSpace(ns.What))
		if i >= 0 {
			// Keep the inline comment after the old value.
			if _, rest, err := parseFlow(lines[i][len(keyNextStep)+1:]); err == nil {
				line += rest
			}
			lines[i] = line
			return lines, nil
		}
		return insertBeforeLog(lines, []string{line}), nil
	})
}

// insertBeforeLog puts new lines just above status_log:, or at the end without one.
func insertBeforeLog(lines, add []string) []string {
	i := slices.IndexFunc(lines, func(l string) bool { return strings.HasPrefix(l, "status_log:") })
	if i < 0 {
		i = len(lines)
	}
	return slices.Insert(lines, i, add...)
}

// closedWithReason is the Statuses a close_reason belongs to.
func closedWithReason(status string) bool {
	return status == StatusRejected || status == StatusWithdrawn
}

// SetCloseReason records why an Application ended (no-response, after-screen,
// after-interview, salary, role-closed, other, or the Owner's own words), or removes the
// key when reason is blank. Only that line changes.
func (s *Store) SetCloseReason(folder, reason string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return s.update(folder, func(lines []string) ([]string, error) {
			return slices.DeleteFunc(lines, func(l string) bool { return strings.HasPrefix(l, keyCloseReason+":") }), nil
		})
	}
	return s.Patch(folder, map[string]string{keyCloseReason: reason})
}

// CloseReasons are the suggested close_reason values, in the order the move pop-up offers.
var CloseReasons = []string{"no-response", "after-screen", "after-interview", "salary", "role-closed", "other"}
