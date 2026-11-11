package store

import (
	"fmt"
	"os"
	"path/filepath"
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
			if !keyPattern.MatchString(k) || k == "status_log" {
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

func (s *Store) update(folder string, change func([]string) ([]string, error)) error {
	path := filepath.Join(folder, "meta.yml")
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if a := readApplication(folder, b); a.Malformed != nil {
		return a.Malformed
	}
	lines, err := change(strings.Split(strings.TrimSuffix(string(b), "\n"), "\n"))
	if err != nil {
		return err
	}
	out := []byte(strings.Join(lines, "\n") + "\n")
	if a := readApplication(folder, out); a.Malformed != nil {
		return a.Malformed
	}
	if string(out) == string(b) {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	return s.atomicWrite(path, out, info.Mode().Perm())
}

// AppendStatus appends a change at the supplied time, normalized to UTC.
func (s *Store) AppendStatus(folder, status string, at time.Time) error {
	if !validStatus(status) {
		return fmt.Errorf("invalid Status %q", status)
	}
	return s.update(folder, func(lines []string) ([]string, error) {
		a := readApplication(folder, []byte(strings.Join(lines, "\n")+"\n"))
		if a.Status == status {
			return lines, nil
		}
		index := -1
		for i, l := range lines {
			if strings.HasPrefix(l, "status_log:") {
				index = i
				break
			}
		}
		if index < 0 {
			lines = append(lines, "status_log:")
			index = len(lines) - 1
		} else {
			lines[index] = "status_log:"
		}
		end := index + 1
		for end < len(lines) && (strings.HasPrefix(lines[end], "  ") || strings.TrimSpace(lines[end]) == "" || strings.HasPrefix(lines[end], "#")) {
			end++
		}
		entry := "  - {status: " + status + ", at: " + at.UTC().Format(time.RFC3339) + "}"
		lines = append(lines, "")
		copy(lines[end+1:], lines[end:])
		lines[end] = entry
		return lines, nil
	})
}
