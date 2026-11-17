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

	report := CheckSetup(Setup{Workspace: workspace, Plugin: plugin, Harness: "claude \"/{skill} {dir}\"", GOOS: "darwin"}, lookup)
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

// Linux gets package-manager hints, not Homebrew.
func TestCheckSetupGivesLinuxInstallHints(t *testing.T) {
	lookup := func(name string) (string, error) { return "", errors.New("not found") }
	report := CheckSetup(Setup{Workspace: t.TempDir(), Plugin: t.TempDir(), GOOS: "linux"}, lookup)
	joined := strings.Join(report.Warnings, "\n")
	if strings.Contains(joined, "brew") {
		t.Errorf("Linux hints mention brew:\n%s", joined)
	}
	for _, want := range []string{"sudo apt install pandoc", "sudo dnf install python3", "typst/typst#installation"} {
		if !strings.Contains(joined, want) {
			t.Errorf("warnings missing %q:\n%s", want, joined)
		}
	}
}

// Without a Plugin, the one Plugin warning says it all: no font list repeating it. The
// hints name the real config path and show a harness example.
func TestCheckSetupWithoutPluginWarnsOnceAndNamesConfigPath(t *testing.T) {
	lookup := func(name string) (string, error) { return "/stub/" + name, nil }
	cfg := "/xdg/rolecall/config.toml"
	report := CheckSetup(Setup{Workspace: t.TempDir(), ConfigPath: cfg}, lookup)
	joined := strings.Join(report.Warnings, "\n")
	if strings.Contains(joined, "fonts") {
		t.Errorf("fonts reported without a Plugin:\n%s", joined)
	}
	if len(report.Warnings) != 2 {
		t.Errorf("want the Plugin and Harness warnings only, got:\n%s", joined)
	}
	for _, want := range []string{"Set plugin in " + cfg, "set it in " + cfg, harnessExample, "docs/harnesses/claude-code.md"} {
		if !strings.Contains(joined, want) {
			t.Errorf("warnings missing %q:\n%s", want, joined)
		}
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

// An Editor that is only the vi fallback is a warning, not an OK; a chosen one stays OK.
func TestCheckSetupWarnsWhenEditorIsOnlyTheViFallback(t *testing.T) {
	lookup := func(name string) (string, error) { return "/stub/" + name, nil }
	fallback := CheckSetup(Setup{Workspace: t.TempDir(), EditorIsFallback: true}, lookup)
	chosen := CheckSetup(Setup{Workspace: t.TempDir(), Editor: "nano"}, lookup)
	has := func(r SetupReport) bool {
		for _, w := range r.Warnings {
			if strings.HasPrefix(w, "Editor") && strings.Contains(w, "vi fallback") {
				return true
			}
		}
		return false
	}
	if !has(fallback) {
		t.Errorf("no vi fallback warning: %q", fallback.Warnings)
	}
	if has(chosen) {
		t.Errorf("chosen Editor warned: %q", chosen.Warnings)
	}
}
