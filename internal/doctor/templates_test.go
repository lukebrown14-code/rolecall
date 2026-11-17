package doctor

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func writeWorkspaceFiles(t *testing.T, workspace string, rels ...string) {
	t.Helper()
	for _, rel := range rels {
		path := filepath.Join(workspace, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestTemplateFilesListsTemplatesFolderOverrides(t *testing.T) {
	workspace := t.TempDir()
	writeWorkspaceFiles(t, workspace, "templates/resume.typ", "templates/cover-letter.typ", "templates/brand.typ")

	overrides, ignored := TemplateFiles(workspace)

	want := []string{"templates/brand.typ", "templates/resume.typ", "templates/cover-letter.typ"}
	if !slices.Equal(overrides, want) {
		t.Errorf("overrides = %v, want %v", overrides, want)
	}
	if len(ignored) != 0 {
		t.Errorf("ignored = %v, want none", ignored)
	}
}

func TestTemplateFilesKeepsTheRootBrandOverride(t *testing.T) {
	workspace := t.TempDir()
	writeWorkspaceFiles(t, workspace, "brand.typ")

	overrides, ignored := TemplateFiles(workspace)

	if !slices.Equal(overrides, []string{"brand.typ"}) || len(ignored) != 0 {
		t.Errorf("overrides = %v, ignored = %v; want [brand.typ] and none ignored", overrides, ignored)
	}
}

func TestTemplateFilesWarnsAboutFilesTheBuildIgnores(t *testing.T) {
	workspace := t.TempDir()
	writeWorkspaceFiles(t, workspace, "brand.typ", "templates/brand.typ", "templates/style.md", "templates/notes.txt", "templates/fonts/a.ttf")

	overrides, ignored := TemplateFiles(workspace)

	if !slices.Equal(overrides, []string{"templates/brand.typ"}) {
		t.Errorf("overrides = %v, want only templates/brand.typ", overrides)
	}
	joined := strings.Join(ignored, "\n")
	for _, want := range []string{
		"brand.typ is not read: templates/brand.typ overrides it",
		"templates/style.md is not read: the skills read the Plugin's styles/au.md; put your own writing rules in profile/writing-preferences.md",
		"templates/notes.txt is not read",
		"templates/fonts is not read",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("ignored missing %q:\n%s", want, joined)
		}
	}
}

func TestCheckSetupNamesEveryOverrideAndWarnsAboutIgnoredFiles(t *testing.T) {
	workspace := t.TempDir()
	writeWorkspaceFiles(t, workspace, "templates/resume.typ", "templates/style.md")
	lookup := func(name string) (string, error) { return "/stub/" + name, nil }

	report := CheckSetup(Setup{Workspace: workspace, Plugin: t.TempDir(), Harness: "claude"}, lookup)

	if report.Template != "Workspace override (templates/resume.typ)" {
		t.Errorf("Template = %q", report.Template)
	}
	if len(report.Ignored) != 1 || !strings.Contains(strings.Join(report.Warnings, "\n"), "templates/style.md is not read") {
		t.Errorf("Ignored = %v, Warnings = %v; want the style.md warning in both", report.Ignored, report.Warnings)
	}
}
