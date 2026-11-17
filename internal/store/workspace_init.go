package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	return initializeWorkspace(root, fs, contact, tracks, search, false)
}

// InitializeStarterWorkspace is what `rolecall init` runs: a new Workspace whose
// master-profile.md already has every expected section under its heading, each with a
// one-line how-to, so a Profile can be written by hand without a Harness.
func InitializeStarterWorkspace(root string) error {
	return initializeWorkspace(root, nil, nil, nil, SearchArea{}, true)
}

func initializeWorkspace(root string, files FileSystem, contact map[string]string, tracks []string, search SearchArea, skeleton bool) error {
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
	profile := contactMarkdown(contact)
	if skeleton {
		profile = ProfileSkeleton(contact)
	}
	starter := map[string]string{
		"workspace.json":                 string(b) + "\n",
		"profile/master-profile.md":      profile,
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
	return "# Master Profile\n\n" + contactSection(fields)
}
