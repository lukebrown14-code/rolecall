package store

import (
	"fmt"
	"slices"
	"strings"
)

// Contact is a person at the employer or the recruiter: a Name, a Role, and an Email or
// a URL (a LinkedIn page). Not the Owner's own Contact, which is part of the Profile.
type Contact struct{ Name, Role, Email, URL string }

// Reach is how to get hold of the Contact: the email, else the URL.
func (c Contact) Reach() string {
	if c.Email != "" {
		return c.Email
	}
	return c.URL
}

// Label is the Contact on one line: "Sam Lee, Recruiter · sam@example.test".
func (c Contact) Label() string {
	s := c.Name
	if c.Role != "" {
		if s != "" {
			s += ", "
		}
		s += c.Role
	}
	if r := c.Reach(); r != "" {
		if s != "" {
			s += " · "
		}
		s += r
	}
	return s
}

func (c Contact) empty() bool { return c.Name == "" && c.Email == "" && c.URL == "" }

// readContactEntry reads one "  - {name: …}" line of contacts.
func readContactEntry(line string) (Contact, error) {
	body, ok := strings.CutPrefix(line, "  - ")
	if !ok {
		return Contact{}, fmt.Errorf("invalid contacts entry or unexpected nesting")
	}
	m, _, err := parseFlow(body)
	if err != nil {
		return Contact{}, err
	}
	c := Contact{Name: DisplayText(m["name"]), Role: DisplayText(m["role"]), Email: DisplayText(m["email"]), URL: DisplayText(m["url"])}
	if c.empty() {
		return Contact{}, fmt.Errorf("empty contacts entry")
	}
	return c, nil
}

// legacyContact reads the dashboard era's `contact: Name` as a Contact: an address with
// an @ is an email, a web address a URL, anything else a name.
func legacyContact(v string) (Contact, bool) {
	v = strings.TrimSpace(DisplayText(v))
	switch {
	case v == "":
		return Contact{}, false
	case strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://"):
		return Contact{URL: v}, true
	case strings.Contains(v, "@") && !strings.ContainsAny(v, " \t"):
		return Contact{Email: v}, true
	}
	return Contact{Name: v}, true
}

// SetContacts replaces the contacts list, and drops the legacy `contact` key whose value
// the list now carries (a Contact read from it is in the list the Owner saved). An empty
// list removes both. The block sits above status_log.
func (s *Store) SetContacts(folder string, contacts []Contact) error {
	for _, c := range contacts {
		if c.empty() {
			return fmt.Errorf("a contact needs a name, an email or a URL")
		}
	}
	return s.update(folder, func(lines []string) ([]string, error) {
		lines = slices.DeleteFunc(lines, func(l string) bool { return strings.HasPrefix(l, keyLegacyContact+":") })
		if start := slices.IndexFunc(lines, func(l string) bool { return strings.HasPrefix(l, keyContacts+":") }); start >= 0 {
			end := start + 1
			for end < len(lines) && strings.HasPrefix(lines[end], "  ") {
				end++
			}
			lines = slices.Delete(lines, start, end)
		}
		if len(contacts) == 0 {
			return lines, nil
		}
		block := []string{keyContacts + ":"}
		for _, c := range contacts {
			block = append(block, "  - "+flowEntry("name", c.Name, "role", c.Role, "email", c.Email, "url", c.URL))
		}
		return insertBeforeLog(lines, block), nil
	})
}
