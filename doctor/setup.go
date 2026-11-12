package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Setup describes the local dependencies checked by rolecall doctor.
type Setup struct {
	Workspace string
	Plugin    string
	Harness   string
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
	checkCommand("typst", "typst", "brew install typst")
	checkCommand("pandoc", "pandoc", "brew install pandoc")
	checkCommand("python3", "Python 3", "brew install python")

	if setup.Plugin == "" {
		report.Checks = append(report.Checks, CheckResult{Name: pluginPathLabel, Hint: "Set plugin in ~/.config/rolecall/config.toml or ROLECALL_PLUGIN"})
	} else if info, err := os.Stat(setup.Plugin); err != nil || !info.IsDir() {
		report.Checks = append(report.Checks, CheckResult{Name: pluginPathLabel, Detail: setup.Plugin, Hint: "Set plugin to the installed rolecall Plugin path"})
	} else {
		report.Checks = append(report.Checks, CheckResult{Name: pluginPathLabel, Found: true, Detail: setup.Plugin})
	}

	fontsDir := filepath.Join(setup.Plugin, "templates", "fonts")
	var missingFonts []string
	for _, font := range templateFonts {
		if info, err := os.Stat(filepath.Join(fontsDir, font)); err != nil || !info.Mode().IsRegular() {
			missingFonts = append(missingFonts, font)
		}
	}
	if setup.Plugin == "" || len(missingFonts) > 0 {
		report.Checks = append(report.Checks, CheckResult{Name: "Template fonts", Detail: strings.Join(missingFonts, ", "), Hint: "Reinstall the rolecall Plugin with its templates/fonts files"})
	} else {
		report.Checks = append(report.Checks, CheckResult{Name: "Template fonts", Found: true, Detail: fontsDir})
	}

	harness := strings.Fields(setup.Harness)
	if len(harness) == 0 {
		report.Checks = append(report.Checks, CheckResult{Name: harnessCheckLabel, Hint: "Set harness in ~/.config/rolecall/config.toml"})
	} else {
		command := strings.Trim(harness[0], "\"'")
		path, err := lookup(command)
		if err != nil || path == "" {
			report.Checks = append(report.Checks, CheckResult{Name: harnessCheckLabel, Detail: command, Hint: "Install the Harness or set its command on PATH"})
		} else {
			report.Checks = append(report.Checks, CheckResult{Name: harnessCheckLabel, Found: true, Detail: path})
		}
	}

	brand := filepath.Join(setup.Workspace, "brand.typ")
	if info, err := os.Stat(brand); err == nil && info.Mode().IsRegular() {
		report.Template = "Workspace override (brand.typ)"
	} else {
		report.Template = "Header Band (default)"
	}
	for _, check := range report.Checks {
		if !check.Found {
			detail := check.Name
			if check.Detail != "" {
				detail += " (" + check.Detail + ")"
			}
			report.Warnings = append(report.Warnings, fmt.Sprintf("%s; %s", detail, check.Hint))
		}
	}
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
