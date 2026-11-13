package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SearchArea configures which role locations the Plugin's Sweep keeps.
type SearchArea struct {
	Places []string `json:"places,omitempty"`
	Remote string   `json:"remote,omitempty"`
}

// ValidateWorkspacePath refuses to initialize a file or non-empty directory.
func ValidateWorkspacePath(root string) error {
	info, err := os.Stat(root)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if info == nil {
		return nil
	}
	if !info.IsDir() {
		return fmt.Errorf("cannot initialize Workspace: %s is not a directory", root)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return fmt.Errorf("cannot initialize Workspace: %s is not empty", root)
	}
	return nil
}

// InitializeWorkspace writes the private starter files for a new Workspace. The optional
// filesystem is the injection seam for the writes and folder creation; nil uses the OS.
func InitializeWorkspace(root string, contact map[string]string, tracks []string, search SearchArea, files ...FileSystem) error {
	var fs FileSystem
	if len(files) > 0 {
		fs = files[0]
	}
	return initializeWorkspace(root, fs, contact, tracks, search)
}

func initializeWorkspace(root string, files FileSystem, contact map[string]string, tracks []string, search SearchArea) error {
	if err := ValidateWorkspacePath(root); err != nil {
		return err
	}
	fs := fsOrOS(files)
	if err := fs.MkdirAll(root, privateDirMode); err != nil {
		return err
	}
	if err := os.Chmod(root, privateDirMode); err != nil {
		return err
	}
	for _, dir := range []string{"profile", "c" + "v", "jobs"} {
		if err := fs.Mkdir(filepath.Join(root, dir), privateDirMode); err != nil {
			return err
		}
	}
	settings := struct {
		Tracks []string    `json:"tracks"`
		Search *SearchArea `json:"search_area,omitempty"`
	}{Tracks: tracks}
	if settings.Tracks == nil {
		settings.Tracks = []string{}
	}
	if len(search.Places) > 0 || search.Remote != "" {
		settings.Search = &search
	}
	b, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	starter := map[string]string{
		"workspace.json":                 string(b) + "\n",
		"profile/master-profile.md":      contactMarkdown(contact),
		"profile/phrasing-bank.md":       "",
		"profile/projects.md":            "",
		"profile/writing-preferences.md": "",
	}
	for name, contents := range starter {
		if err := atomicWrite(files, filepath.Join(root, name), []byte(contents), privateFileMode); err != nil {
			return err
		}
	}
	return nil
}

func contactMarkdown(fields map[string]string) string {
	var b strings.Builder
	b.WriteString("# Master Profile\n\n## Contact\n\n| Field | Value |\n| :-- | :-- |\n")
	for _, field := range []string{"Name", "Email", "Phone", "Location", "LinkedIn", "GitHub", "Work rights"} {
		value := strings.TrimSpace(fields[field])
		if value != "" {
			fmt.Fprintf(&b, "| %s | %s |\n", field, strings.ReplaceAll(value, "|", "\\|"))
		}
	}
	return b.String()
}
