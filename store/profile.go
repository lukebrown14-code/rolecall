package store

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Profile is the locked fact set documents are written from. Contact is the
// only section the store reads so far; later readers add sections beside it.
type Profile struct {
	Contact map[string]string // field -> cleaned value
	Missing []string          // required fields with no value, in Name, Email order
}

// requiredContact is the order the Profile screen reports missing fields in.
var requiredContact = []string{"Name", "Email"}

var (
	// A trailing italic aside on a cell is a note to the human, not the value.
	contactNote = regexp.MustCompile(`\s*\*\([^)\n]*\)\*\s*$`)
	// Bold, italic and code wrappers are presentation, so the value is inside.
	contactMarkup = regexp.MustCompile("\\*\\*([^*\\n]+)\\*\\*|\\*([^*\\n]+)\\*|`([^`\\n]+)`")
)

// Profile reads the Contact from profile/master-profile.md. A missing file or
// section has no fields, so the required ones are reported missing, not an error.
func (s *Store) Profile() (Profile, error) {
	b, err := os.ReadFile(filepath.Join(s.root, "profile", "master-profile.md"))
	if errors.Is(err, os.ErrNotExist) {
		b = nil
	} else if err != nil {
		return Profile{}, err
	}
	contact := parseContact(section(string(b), "Contact"))
	return Profile{Contact: contact, Missing: missingContact(contact)}, nil
}

// section returns the lines under a `## heading`, up to the next `## ` heading
// or the end of the file. In a CRLF file each line keeps its trailing \r here;
// parseContact trims it when reading a row.
func section(text, heading string) []string {
	lines := strings.Split(text, "\n")
	start := -1
	for i, line := range lines {
		rest, ok := strings.CutPrefix(line, "## "+heading)
		if ok && strings.TrimSpace(rest) == "" {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return nil
	}
	for i := start; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "## ") {
			return lines[start:i]
		}
	}
	return lines[start:]
}

// parseContact reads `| Field | Value |` rows, skipping the header and divider.
func parseContact(lines []string) map[string]string {
	fields := map[string]string{}
	for _, line := range lines {
		line = strings.TrimRight(line, " \t\r")
		if len(line) < 2 || line[0] != '|' || line[len(line)-1] != '|' {
			continue
		}
		cells := strings.Split(line[1:len(line)-1], "|")
		key := strings.TrimSpace(cells[0])
		if key == "" || key == "Field" || divider(key) {
			continue
		}
		fields[key] = cleanCell(strings.Join(cells[1:], "|"))
	}
	return fields
}

// divider reports a table scaffolding row such as `:-- | :--`.
func divider(key string) bool {
	for _, r := range key {
		if !strings.ContainsRune(":- ", r) {
			return false
		}
	}
	return true
}

// cleanCell drops a trailing note and unwraps emphasis and code markers.
func cleanCell(value string) string {
	v := contactNote.ReplaceAllString(strings.TrimSpace(value), "")
	return strings.TrimSpace(contactMarkup.ReplaceAllString(v, "${1}${2}${3}"))
}

// missingContact names the required fields with no value, in required order.
func missingContact(contact map[string]string) []string {
	missing := []string{}
	for _, field := range requiredContact {
		if contact[field] == "" {
			missing = append(missing, field)
		}
	}
	return missing
}
