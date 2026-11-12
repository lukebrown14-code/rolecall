package doctor

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckSetupReportsMissingDependenciesAndTemplate(t *testing.T) {
	workspace := t.TempDir()
	plugin := filepath.Join(t.TempDir(), "plugin")
	if err := os.MkdirAll(filepath.Join(plugin, "templates", "fonts"), 0o700); err != nil {
		t.Fatal(err)
	}
	lookup := func(name string) (string, error) { return "", errors.New("not found") }

	report := CheckSetup(Setup{Workspace: workspace, Plugin: plugin, Harness: "claude \"/{skill} {dir}\""}, lookup)
	joined := strings.Join(report.Warnings, "\n")
	for _, want := range []string{
		"typst", "brew install typst", "pandoc", "brew install pandoc", "Python 3", "brew install python",
		"Plugin", "Harness command", "HankenGrotesk-400-normal.ttf",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("warnings missing %q:\n%s", want, joined)
		}
	}
	if report.Template != "Header Band (default)" {
		t.Errorf("Template = %q, want default Header Band", report.Template)
	}
}

func TestCheckSetupFindsWorkspaceTemplateAndDependencies(t *testing.T) {
	workspace := t.TempDir()
	plugin := t.TempDir()
	if err := os.MkdirAll(filepath.Join(plugin, "templates", "fonts"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "brand.typ"), []byte("#let ink = black"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, name := range templateFonts {
		if err := os.WriteFile(filepath.Join(plugin, "templates", "fonts", name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	lookup := func(name string) (string, error) { return "/stub/" + name, nil }

	report := CheckSetup(Setup{Workspace: workspace, Plugin: plugin, Harness: "claude \"/{skill} {dir}\""}, lookup)
	if len(report.Warnings) != 0 {
		t.Errorf("warnings = %v, want none", report.Warnings)
	}
	if report.Template != "Workspace override (brand.typ)" {
		t.Errorf("Template = %q, want Workspace override", report.Template)
	}
}

func TestCheckSetupEmptyHarnessIsWarning(t *testing.T) {
	workspace := t.TempDir()
	plugin := t.TempDir()
	lookup := func(name string) (string, error) { return "/stub/" + name, nil }

	report := CheckSetup(Setup{Workspace: workspace, Plugin: plugin}, lookup)
	if !strings.Contains(strings.Join(report.Warnings, "\n"), "harness") {
		t.Fatalf("warnings = %v, want harness hint", report.Warnings)
	}
}
