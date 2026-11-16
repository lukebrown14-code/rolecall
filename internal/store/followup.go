package store

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

// keyFollowedUp is the day the Owner last followed up on a sent Application without its
// Status changing ("followed_up: 2026-11-16"). It restarts the Follow-up due count. It is
// not the dashboard era's legacy `followup` key, which stays unread.
const keyFollowedUp = "followed_up"

// readFollowedUp reads followed_up. A value that is not a YYYY-MM-DD date reads as unset
// rather than making the file malformed: the key only times a reminder. The Python reader
// (plugin/scripts/meta.py) reads it the same way, so the two agree on what is malformed.
func readFollowedUp(v string) string {
	if _, err := time.Parse(time.DateOnly, v); err != nil {
		return ""
	}
	return v
}

// SetFollowedUp records the day the Owner followed up (YYYY-MM-DD), or removes the key
// when day is empty, as an undo does. Only that line changes, by the Format rules' patch.
func (s *Store) SetFollowedUp(folder, day string) error {
	if day == "" {
		return s.update(folder, func(lines []string) ([]string, error) {
			return slices.DeleteFunc(lines, func(l string) bool { return strings.HasPrefix(l, keyFollowedUp+":") }), nil
		})
	}
	if _, err := time.Parse(time.DateOnly, day); err != nil {
		return fmt.Errorf("invalid followed_up date %q", day)
	}
	return s.Patch(folder, map[string]string{keyFollowedUp: day})
}
