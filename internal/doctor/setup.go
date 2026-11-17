package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/lukebrown14-code/rolecall/internal/config"
)

// Setup describes the local dependencies checked by rolecall doctor.
type Setup struct {
	Workspace string
	Plugin    string
	Harness   string
	Editor    string // configured editor; empty falls back to $VISUAL, $EDITOR, vi
	// ConfigPath is the resolved config.toml the hints name; empty names the default.
	ConfigPath string
	// GOOS picks the install hints; empty means this machine's.
	GOOS string
}

// harnessExample is a working harness value the hint can show.
const harnessExample = `harness = "claude \"/{skill} {dir}\""`

// The tools doctor checks, by the command it looks for on PATH.
const (
	toolTypst  = "typst"
	toolPandoc = "pandoc"
	toolPython = "python3"
)

// installHints says how to install each tool doctor looks for, per OS. macOS uses
// Homebrew; Linux names the common package managers; anything else gets the project page.
func installHints(goos string) map[string]string {
	switch goos {
	case "darwin":
		return map[string]string{
			toolTypst:  "brew install typst",
			toolPandoc: "brew install pandoc",
			toolPython: "brew install python",
		}
	case "linux":
		return map[string]string{
			toolTypst:  "install it with your package manager or from https://github.com/typst/typst#installation",
			toolPandoc: "sudo apt install pandoc (Debian, Ubuntu) or sudo dnf install pandoc (Fedora)",
			toolPython: "sudo apt install python3 (Debian, Ubuntu) or sudo dnf install python3 (Fedora)",
		}
	}
	return map[string]string{
		toolTypst:  "see https://github.com/typst/typst#installation",
		toolPandoc: "see https://pandoc.org/installing.html",
		toolPython: "see https://www.python.org/downloads/",
	}
}

// LookupPath resolves a command on PATH. Tests can replace it without invoking
// any external programs.
type LookupPath func(string) (string, error)

// CheckResult is one found or missing dependency, with a hint for a missing one.
type CheckResult struct {
	Name   string
	Found  bool
	Detail string
	Hint   string
}

// SetupReport contains dependency results and the effective document Template.
type SetupReport struct {
	Checks   []CheckResult
	Warnings []string
	Template string
	// Ignored names each Workspace file that looks like a Template setting but that the
	// build never reads, with what to do instead. Each is also in Warnings.
	Ignored []string
}

var templateFonts = []string{
	"HankenGrotesk-400-normal.ttf",
	"HankenGrotesk-400-italic.ttf",
	"HankenGrotesk-500-normal.ttf",
	"HankenGrotesk-500-italic.ttf",
	"HankenGrotesk-600-normal.ttf",
	"HankenGrotesk-600-italic.ttf",
	"HankenGrotesk-700-normal.ttf",
	"HankenGrotesk-700-italic.ttf",
}

const (
	pluginPathLabel   = "Plugin path"
	harnessCheckLabel = "Harness command"
	editorCheckLabel  = "Editor"
)

// CheckSetup inspects configured paths and PATH without launching processes.
func CheckSetup(setup Setup, lookup LookupPath) SetupReport {
	if lookup == nil {
		lookup = lookPathOnPath
	}
	report := SetupReport{}
	checkCommand := func(name, label, hint string) {
		path, err := lookup(name)
		if err != nil || path == "" {
			report.Checks = append(report.Checks, CheckResult{Name: label, Hint: hint})
			return
		}
		report.Checks = append(report.Checks, CheckResult{Name: label, Found: true, Detail: path})
	}
	goos, configPath := setup.GOOS, setup.ConfigPath
	if goos == "" {
		goos = runtime.GOOS
	}
	if configPath == "" {
		configPath = "~/.config/rolecall/config.toml"
	}
	hints := installHints(goos)
	checkCommand("typst", "typst", hints["typst"])
	checkCommand("pandoc", "pandoc", hints["pandoc"])
	checkCommand("python3", "Python 3", hints["python3"])

	pluginFound := false
	if setup.Plugin == "" {
		report.Checks = append(report.Checks, CheckResult{Name: pluginPathLabel, Hint: "Set plugin in " + configPath + " or ROLECALL_PLUGIN"})
	} else if info, err := os.Stat(setup.Plugin); err != nil || !info.IsDir() {
		report.Checks = append(report.Checks, CheckResult{Name: pluginPathLabel, Detail: setup.Plugin, Hint: "Set plugin in " + configPath + " to the installed rolecall Plugin path"})
	} else {
		pluginFound = true
		report.Checks = append(report.Checks, CheckResult{Name: pluginPathLabel, Found: true, Detail: setup.Plugin})
	}

	// The fonts live in the Plugin, so without one the Plugin warning above is the whole
	// story; listing every font as missing too would repeat it.
	if pluginFound {
		fontsDir := filepath.Join(setup.Plugin, "templates", "fonts")
		var missingFonts []string
		for _, font := range templateFonts {
			if info, err := os.Stat(filepath.Join(fontsDir, font)); err != nil || !info.Mode().IsRegular() {
				missingFonts = append(missingFonts, font)
			}
		}
		if len(missingFonts) > 0 {
			report.Checks = append(report.Checks, CheckResult{Name: "Template fonts", Detail: strings.Join(missingFonts, ", "), Hint: "Reinstall the rolecall Plugin with its templates/fonts files"})
		} else {
			report.Checks = append(report.Checks, CheckResult{Name: "Template fonts", Found: true, Detail: fontsDir})
		}
	}

	harness := strings.Fields(setup.Harness)
	if len(harness) == 0 {
		report.Checks = append(report.Checks, CheckResult{Name: harnessCheckLabel,
			Hint: "the Harness is the AI coding agent that runs the Plugin's skills; set it in " + configPath + ", e.g. " + harnessExample + " (README › Connect your Harness)"})
	} else {
		command := strings.Trim(harness[0], "\"'")
		path, err := lookup(command)
		if err != nil || path == "" {
			report.Checks = append(report.Checks, CheckResult{Name: harnessCheckLabel, Detail: command, Hint: "Install the Harness or put its command on PATH (README › Connect your Harness)"})
		} else {
			report.Checks = append(report.Checks, CheckResult{Name: harnessCheckLabel, Found: true, Detail: path})
		}
	}

	editor := config.CommandName(config.ResolveEditor(setup.Editor))
	if path, err := lookup(editor); err != nil || path == "" {
		report.Checks = append(report.Checks, CheckResult{Name: editorCheckLabel, Detail: editor, Hint: "Install the Editor or set editor in " + configPath})
	} else {
		report.Checks = append(report.Checks, CheckResult{Name: editorCheckLabel, Found: true, Detail: path})
	}

	overrides, ignored := TemplateFiles(setup.Workspace)
	if len(overrides) > 0 {
		report.Template = "Workspace override (" + strings.Join(overrides, ", ") + ")"
	} else {
		report.Template = "Header Band (default)"
	}
	report.Ignored = ignored
	for _, check := range report.Checks {
		if !check.Found {
			detail := check.Name
			if check.Detail != "" {
				detail += " (" + check.Detail + ")"
			}
			report.Warnings = append(report.Warnings, fmt.Sprintf("%s; %s", detail, check.Hint))
		}
	}
	report.Warnings = append(report.Warnings, ignored...)
	return report
}

// lookPathOnPath resolves an executable without starting it, preserving
// ADR-0003's restriction on process launches.
func lookPathOnPath(name string) (string, error) {
	if filepath.IsAbs(name) || strings.ContainsRune(name, filepath.Separator) {
		return executable(name)
	}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		candidate := filepath.Join(dir, name)
		if runtime.GOOS == "windows" && filepath.Ext(candidate) == "" {
			for _, ext := range filepath.SplitList(os.Getenv("PATHEXT")) {
				if path, err := executable(candidate + ext); err == nil {
					return path, nil
				}
			}
		}
		if path, err := executable(candidate); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("%s not found on PATH", name)
}

func executable(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || (runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0) {
		return "", fmt.Errorf("%s is not executable", path)
	}
	return path, nil
}
