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

func editorWarnings(t *testing.T, editor string, found map[string]bool) []string {
	t.Helper()
	lookup := func(name string) (string, error) {
		if found[name] {
			return "/stub/" + name, nil
		}
		return "", errors.New("not found")
	}
	all := map[string]bool{"typst": true, "pandoc": true, "python3": true, "claude": true}
	for k, v := range found {
		all[k] = v
	}
	plugin := t.TempDir()
	report := CheckSetup(Setup{Workspace: t.TempDir(), Plugin: plugin, Harness: "claude", Editor: editor}, func(n string) (string, error) {
		if all[n] {
			return "/stub/" + n, nil
		}
		return lookup(n)
	})
	var out []string
	for _, w := range report.Warnings {
		if strings.Contains(w, "Editor") {
			out = append(out, w)
		}
	}
	return out
}

func TestCheckSetupMissingEditorIsWarning(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	got := editorWarnings(t, `"/opt/my tools/nvim" -u NONE`, nil)
	if len(got) != 1 || !strings.Contains(got[0], "/opt/my tools/nvim") {
		t.Fatalf("Editor warnings = %v, want one naming the quoted path", got)
	}
}

func TestCheckSetupFoundUnsetAndEnvEditorsAreClean(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	if got := editorWarnings(t, "code --wait", map[string]bool{"code": true}); len(got) != 0 {
		t.Errorf("found: %v", got)
	}
	if got := editorWarnings(t, "", map[string]bool{"vi": true}); len(got) != 0 {
		t.Errorf("unset: %v", got)
	}
	t.Setenv("EDITOR", "hx")
	if got := editorWarnings(t, "", map[string]bool{"hx": true}); len(got) != 0 {
		t.Errorf("env: %v", got)
	}
	t.Setenv("VISUAL", "zed --wait")
	if got := editorWarnings(t, "", map[string]bool{"zed": true}); len(got) != 0 {
		t.Errorf("VISUAL: %v", got)
	}
	if got := editorWarnings(t, "", map[string]bool{"hx": true}); len(got) != 1 {
		t.Errorf("VISUAL takes precedence over EDITOR, want warning: %v", got)
	}
}
