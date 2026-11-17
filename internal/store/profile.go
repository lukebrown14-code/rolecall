package store

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ProfileSection is one `##` section of the Profile: its heading, the Markdown under
// it, and the count of what it holds. Kind is empty when the section is prose.
type ProfileSection struct {
	Heading, Body string
	Count         int
	Kind          string // entries, rows, bullets or items; "" for no count
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
	CVs      []CV              // the CVs in cv/, general first (CVs)
}

// profileFileNames are the other files the Harness reads, in the order the screen lists
// them. Anything else in profile/ is the Owner's business and is not listed.
var profileFileNames = []string{"projects.md", "writing-preferences.md", "phrasing-bank.md"}

// requiredContact is the order the Profile screen reports missing fields in.
var requiredContact = []string{"Name", "Email"}

// minCommaListItems is the shortest comma list: one comma joins two items.
const minCommaListItems = 2

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
	// The Profile is read for the screen, so its text is made safe to draw here, once.
	b = []byte(DocumentText(string(b)))
	contact := parseContact(section(string(b), "Contact"))
	var sections []ProfileSection
	if exists {
		sections = parseSections(string(b))
	}
	files, err := s.profileFiles()
	if err != nil {
		return Profile{}, err
	}
	cvs, err := s.CVs()
	if err != nil {
		return Profile{}, err
	}
	return Profile{Exists: exists, Contact: contact, Missing: missingContact(contact), Sections: sections, Files: files, CVs: cvs}, nil
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
		files = append(files, ProfileFile{Name: name, Path: path, Body: DocumentText(string(b))})
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
			out = append(out, newSection(heading, trimBlank(lines[start:end])))
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
		return []ProfileSection{newSection("Profile", trimBlank(lines))}
	}
	return out
}

// newSection pairs a heading and body with the count of what the body holds.
func newSection(heading, body string) ProfileSection {
	count, kind := countSection(body)
	return ProfileSection{Heading: heading, Body: body, Count: count, Kind: kind}
}

// countSection applies the Profile count rules in order: `###` entries, then table data
// rows, then bullets, then a one-paragraph comma list. Anything else is prose, with no
// count, so the list shows no meaningless number.
func countSection(body string) (int, string) {
	lines := strings.Split(body, "\n")
	entries := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "### ") {
			entries++
		}
	}
	if entries > 0 {
		return entries, "entries"
	}
	rows, header := 0, false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !tableRow(line) || tableDivider(line) {
			continue
		}
		if !header {
			header = true // the first row names the columns, so it is not data
			continue
		}
		rows++
	}
	if header {
		return rows, "rows"
	}
	bullets := 0
	for _, line := range lines {
		if isBullet(strings.TrimSpace(line)) {
			bullets++
		}
	}
	if bullets > 0 {
		return bullets, "bullets"
	}
	if items, ok := commaItems(body); ok {
		return items, "items"
	}
	return 0, ""
}

// tableRow reports a Markdown table row: it starts and ends with a pipe, as the
// tables in SCHEMA.md do.
func tableRow(line string) bool {
	return len(line) >= 2 && line[0] == '|' && line[len(line)-1] == '|'
}

// tableDivider reports a scaffolding row such as `| :-- | :-- |`: only pipes, colons,
// dashes and spaces, so it separates the header from the data and holds nothing.
func tableDivider(line string) bool {
	for _, r := range line {
		if !strings.ContainsRune("|:- \t", r) {
			return false
		}
	}
	return true
}

// isBullet reports a trimmed list line: `- `, `* ` or `+ `.
func isBullet(line string) bool {
	return strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") || strings.HasPrefix(line, "+ ")
}

// commaItems counts the items in a single-paragraph comma list. Prose is not a list:
// sentence punctuation or a colon means the commas join clauses, and a blank-line break
// means more than one paragraph. A list also needs at least two items.
func commaItems(body string) (int, bool) {
	if !strings.Contains(body, ",") {
		return 0, false
	}
	if strings.ContainsAny(body, ".!?;:") {
		return 0, false
	}
	for _, line := range strings.Split(body, "\n") {
		if strings.TrimSpace(line) == "" {
			return 0, false
		}
	}
	items := 0
	for _, item := range strings.Split(body, ",") {
		if strings.TrimSpace(item) != "" {
			items++
		}
	}
	if items < minCommaListItems {
		return 0, false
	}
	return items, true
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
