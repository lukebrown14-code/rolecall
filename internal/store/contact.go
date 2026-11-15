package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// contactFields is the order the Contact table lists its fields in. Fields the Owner
// added that are not listed here follow them, alphabetically.
var contactFields = []string{"Name", "Email", "Phone", "Location", "LinkedIn", "GitHub", "Work rights"}

// SetContact writes the Contact table of profile/master-profile.md, the one part of
// profile/ the TUI writes (from its first-run form). Each given field replaces the
// stored value, an empty one removes it, and fields not given are kept. Everything
// outside the `## Contact` section is left as it was; a file with no Contact section
// gets one after its title.
func (s *Store) SetContact(fields map[string]string) error {
	path := filepath.Join(s.root, "profile", "master-profile.md")
	b, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	text := strings.ReplaceAll(string(b), "\r\n", "\n")
	contact := parseContact(section(text, "Contact"))
	for k, v := range fields {
		contact[k] = strings.TrimSpace(v)
	}
	if err := os.MkdirAll(filepath.Dir(path), privateDirMode); err != nil {
		return err
	}
	return atomicWrite(nil, path, []byte(replaceContact(text, contactSection(contact))), privateFileMode)
}

// contactSection is the `## Contact` heading and its table, ending in a newline.
func contactSection(fields map[string]string) string {
	var extra []string
	for k := range fields {
		if !slices.Contains(contactFields, k) {
			extra = append(extra, k)
		}
	}
	slices.Sort(extra)
	var b strings.Builder
	b.WriteString("## Contact\n\n| Field | Value |\n| :-- | :-- |\n")
	for _, field := range append(slices.Clone(contactFields), extra...) {
		if value := strings.TrimSpace(fields[field]); value != "" {
			fmt.Fprintf(&b, "| %s | %s |\n", field, strings.ReplaceAll(value, "|", "\\|"))
		}
	}
	return b.String()
}

// replaceContact swaps the `## Contact` section of text for contact. With no such
// section, contact goes after the `# ` title, or starts a new file.
func replaceContact(text, contact string) string {
	if strings.TrimSpace(text) == "" {
		return "# Master Profile\n\n" + contact
	}
	lines := strings.Split(text, "\n")
	start, end := -1, len(lines)
	for i, line := range lines {
		heading, ok := strings.CutPrefix(line, "## ")
		if !ok {
			continue
		}
		if start >= 0 {
			end = i
			break
		}
		if strings.TrimSpace(heading) == "Contact" {
			start = i
		}
	}
	if start < 0 {
		// No Contact yet: insert it after the title line, or at the top.
		at := 0
		if strings.HasPrefix(lines[0], "# ") {
			at = 1
		}
		before := strings.TrimRight(strings.Join(lines[:at], "\n"), "\n")
		after := strings.TrimLeft(strings.Join(lines[at:], "\n"), "\n")
		return joinBlocks(before, contact, after)
	}
	before := strings.TrimRight(strings.Join(lines[:start], "\n"), "\n")
	after := strings.TrimLeft(strings.Join(lines[end:], "\n"), "\n")
	return joinBlocks(before, contact, after)
}

// joinBlocks joins the non-empty blocks with one blank line and ends with a newline.
func joinBlocks(blocks ...string) string {
	var kept []string
	for _, b := range blocks {
		if b = strings.TrimRight(b, "\n"); b != "" {
			kept = append(kept, b)
		}
	}
	return strings.Join(kept, "\n\n") + "\n"
}
