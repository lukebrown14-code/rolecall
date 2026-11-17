package store

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"
)

// Patch re-reads and validates metadata before changing scalar keys.
func (s *Store) Patch(folder string, keys map[string]string) error {
	return s.update(folder, func(lines []string) ([]string, error) {
		names := make([]string, 0, len(keys))
		for k := range keys {
			names = append(names, k)
		}
		sort.Strings(names)
		for _, k := range names {
			if !keyPattern.MatchString(k) || k == "status_log" || k == keyContacts || k == keyNextStep {
				return nil, fmt.Errorf("invalid scalar key %q", k)
			}
			value := quoteScalar(keys[k])
			found := false
			for i, line := range lines {
				if strings.HasPrefix(line, k+":") {
					// Keep the exact whitespace and inline comment after the old scalar.
					raw := line[len(k)+1:]
					_, suffix, _ := scalarToken(raw)
					lines[i] = k + ": " + value + suffix
					found = true
					break
				}
			}
			if !found {
				i := len(lines)
				for j, l := range lines {
					if strings.HasPrefix(l, "status_log:") {
						i = j
						break
					}
				}
				lines = append(lines, "")
				copy(lines[i+1:], lines[i:])
				lines[i] = k + ": " + value
			}
		}
		return lines, nil
	})
}

// ErrConflict reports that meta.yml kept changing under a write, so the write gave up
// rather than overwrite another writer's change.
var ErrConflict = errors.New("meta.yml changed during the write; try again")

// updateAttempts bounds how often update re-applies its change to fresh bytes.
const updateAttempts = 3

// update applies change to meta.yml and publishes it only if the file still holds the
// bytes change was applied to. When another writer got in first, change is re-applied
// to the fresh bytes; after updateAttempts it returns ErrConflict.
func (s *Store) update(folder string, change func([]string) ([]string, error)) error {
	_, err := s.updateReport(folder, change)
	return err
}

// updateReport is update, also reporting whether it wrote.
func (s *Store) updateReport(folder string, change func([]string) ([]string, error)) (bool, error) {
	if err := s.applicationRecord(folder); err != nil {
		return false, err
	}
	path := filepath.Join(folder, "meta.yml")
	for range updateAttempts {
		b, err := os.ReadFile(path)
		if err != nil {
			return false, err
		}
		if a := readApplication(folder, b); a.Malformed != nil {
			return false, a.Malformed
		}
		lines, err := change(strings.Split(strings.TrimSuffix(string(b), "\n"), "\n"))
		if err != nil {
			return false, err
		}
		out := []byte(strings.Join(lines, "\n") + "\n")
		if a := readApplication(folder, out); a.Malformed != nil {
			return false, a.Malformed
		}
		if bytes.Equal(out, b) {
			return false, nil
		}
		info, err := os.Stat(path)
		if err != nil {
			return false, err
		}
		err = atomicWriteIf(s.files, path, out, info.Mode().Perm(), func() error {
			now, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if !bytes.Equal(now, b) {
				return errChanged
			}
			return nil
		})
		if errors.Is(err, errChanged) {
			continue
		}
		return err == nil, err
	}
	return false, ErrConflict
}

// errChanged is update's signal that meta.yml moved on before the rename.
var errChanged = errors.New("meta.yml changed")

// statusLogSpan is where status_log: sits in lines and the index just past its entries
// (indented, blank or comment lines), or -1, -1 when there is none.
func statusLogSpan(lines []string) (start, end int) {
	start = slices.IndexFunc(lines, func(l string) bool { return strings.HasPrefix(l, "status_log:") })
	if start < 0 {
		return -1, -1
	}
	end = start + 1
	for end < len(lines) && (strings.HasPrefix(lines[end], "  ") || strings.TrimSpace(lines[end]) == "" || strings.HasPrefix(lines[end], "#")) {
		end++
	}
	return start, end
}

// AppendStatus appends a change at the supplied time, normalized to UTC, and reports
// whether it wrote: an Application already at status is left alone. Moving an
// Application a Search rule dismissed out of Dismissed overrules the rule in the same
// write: filter_overruled is set and filter_reason and filter_note are removed.
func (s *Store) AppendStatus(folder, status string, at time.Time) (bool, error) {
	if !validStatus(status) {
		return false, fmt.Errorf("invalid Status %q", status)
	}
	return s.updateReport(folder, func(lines []string) ([]string, error) {
		a := readApplication(folder, []byte(strings.Join(lines, "\n")+"\n"))
		if a.Status == status {
			return lines, nil
		}
		if a.Status == StatusDismissed && a.FilterReason != "" {
			// Restoring a Lead a Search rule dismissed overrules the rule: it never
			// dismisses this Application again.
			lines = overruleFilter(lines)
		}
		if !closedWithReason(status) {
			// A close_reason belongs to a closed Application: it goes when it reopens
			// (and so when the move to it is undone).
			lines = slices.DeleteFunc(lines, func(l string) bool { return strings.HasPrefix(l, keyCloseReason+":") })
		}
		index, end := statusLogSpan(lines)
		if index < 0 {
			lines = append(lines, "status_log:")
			index, end = len(lines)-1, len(lines)
		}
		lines[index] = "status_log:"
		entry := "  - {status: " + status + ", at: " + at.UTC().Format(time.RFC3339) + "}"
		return slices.Insert(lines, end, entry), nil
	})
}

// overruleFilter drops filter_reason and filter_note and sets filter_overruled: true,
// where filter_reason was.
func overruleFilter(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		switch {
		case strings.HasPrefix(l, keyFilterReason+":"):
			out = append(out, keyFilterOverruled+": true")
		case strings.HasPrefix(l, keyFilterNote+":"), strings.HasPrefix(l, keyFilterOverruled+":"):
		default:
			out = append(out, l)
		}
	}
	return out
}

// SetFilter puts back the Search rule that dismissed an Application, as an undo of its
// restore does: filter_reason and filter_note are written and filter_overruled removed.
// An empty reason does nothing.
func (s *Store) SetFilter(folder, reason, note string) error {
	if reason == "" {
		return nil
	}
	if !slices.Contains(FilterReasons, reason) {
		return fmt.Errorf("invalid filter_reason %q", reason)
	}
	err := s.update(folder, func(lines []string) ([]string, error) {
		return slices.DeleteFunc(lines, func(l string) bool { return strings.HasPrefix(l, keyFilterOverruled+":") }), nil
	})
	if err != nil {
		return err
	}
	keys := map[string]string{keyFilterReason: reason}
	if note != "" {
		keys[keyFilterNote] = note
	}
	return s.Patch(folder, keys)
}
