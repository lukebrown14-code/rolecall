package store

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ProfileSection is one `##` section of the Profile: its heading and the Markdown
// under it, with surrounding blank lines trimmed.
type ProfileSection struct {
	Heading, Body string
}

// ProfileFile is one of the other Profile files beside master-profile.md: its whole
// Markdown for the screen to read, and the path e opens.
type ProfileFile struct {
	Name, Path, Body string
}

// Profile is the locked fact set documents are written from: its Contact fields, its
// sections in file order, and the other Profile files that exist.
type Profile struct {
	Exists   bool              // master-profile.md was read
	Contact  map[string]string // field -> cleaned value
	Missing  []string          // required fields with no value, in Name, Email order
	Sections []ProfileSection  // `##` sections in file order
	Files    []ProfileFile     // other Profile files, in profileFileNames order
}

// profileFileNames are the other files the Harness reads, in the order the screen lists
// them. Anything else in profile/ is the Owner's business and is not listed.
var profileFileNames = []string{"projects.md", "writing-preferences.md", "phrasing-bank.md"}

// requiredContact is the order the Profile screen reports missing fields in.
var requiredContact = []string{"Name", "Email"}

var (
	// A trailing italic aside on a cell is a note to the human, not the value.
	contactNote = regexp.MustCompile(`\s*\*\([^)\n]*\)\*\s*$`)
	// Bold, italic and code wrappers are presentation, so the value is inside.
	contactMarkup = regexp.MustCompile("\\*\\*([^*\\n]+)\\*\\*|\\*([^*\\n]+)\\*|`([^`\\n]+)`")
)

// Profile reads profile/master-profile.md: its Contact, its `##` sections, and the
// other Profile files that exist. A missing file or section has no fields, so the
// required ones are reported missing, not an error.
func (s *Store) Profile() (Profile, error) {
	b, err := os.ReadFile(filepath.Join(s.root, "profile", "master-profile.md"))
	exists := true
	if errors.Is(err, os.ErrNotExist) {
		b, exists = nil, false
	} else if err != nil {
		return Profile{}, err
	}
	contact := parseContact(section(string(b), "Contact"))
	var sections []ProfileSection
	if exists {
		sections = parseSections(string(b))
	}
	files, err := s.profileFiles()
	if err != nil {
		return Profile{}, err
	}
	return Profile{Exists: exists, Contact: contact, Missing: missingContact(contact), Sections: sections, Files: files}, nil
}

// profileFiles reads the other Profile files, skipping the ones that do not exist: a
// file the Owner never made is simply absent from the list.
func (s *Store) profileFiles() ([]ProfileFile, error) {
	var files []ProfileFile
	for _, name := range profileFileNames {
		path := filepath.Join("profile", name)
		b, err := os.ReadFile(filepath.Join(s.root, path))
		if errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return nil, err
		}
		files = append(files, ProfileFile{Name: name, Path: path, Body: string(b)})
	}
	return files, nil
}

// parseSections splits the Profile at its `##` headings. Text before the first heading
// is the title, not a section; a file with no headings is one section holding it all.
// `###` is a subheading inside a section, so only `## ` starts one.
func parseSections(text string) []ProfileSection {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	lines := strings.Split(text, "\n")
	var out []ProfileSection
	heading, start := "", 0
	flush := func(end int) {
		if heading != "" {
			out = append(out, ProfileSection{Heading: heading, Body: trimBlank(lines[start:end])})
		}
	}
	for i, line := range lines {
		if rest, ok := strings.CutPrefix(line, "## "); ok {
			flush(i)
			heading, start = strings.TrimSpace(rest), i+1
		}
	}
	flush(len(lines))
	if len(out) == 0 {
		// A free-form Profile is still readable: one section with everything in it,
		// title included, because there is no heading to separate it.
		return []ProfileSection{{Heading: "Profile", Body: trimBlank(lines)}}
	}
	return out
}

// trimBlank drops the blank lines around a section's text.
func trimBlank(lines []string) string {
	start, end := 0, len(lines)
	for start < end && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return strings.Join(lines[start:end], "\n")
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
