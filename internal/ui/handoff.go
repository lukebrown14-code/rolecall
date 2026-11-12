package ui

import (
	"errors"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/internal/store"
)

// The Plugin skills a Handoff can run.
const (
	skillAssessFit        = "assess-fit"
	skillPrepareDocuments = "prepare-documents"
)

// handoffAllowed reports whether a skill applies to an Application: assess-fit on
// any, prepare-documents only on a Lead or Draft.
func handoffAllowed(a store.Application, skill string) bool {
	switch skill {
	case skillAssessFit:
		return true
	case skillPrepareDocuments:
		return a.Status == store.StatusLead || a.Status == store.StatusDraft
	}
	return false
}

// startHandoff validates a Handoff request and, when it can, runs it. A missing
// Harness names the setting and runs nothing.
func (m *model) startHandoff(skill string) tea.Cmd {
	a, ok := m.current()
	if !ok {
		return nil
	}
	if m.harness == "" {
		m.err = errors.New("Set harness in ~/.config/rolecall/config.toml")
		return nil
	}
	if !handoffAllowed(a, skill) {
		return nil
	}
	return m.runHandoff(a.Folder, harnessCommand(m.harness, skill, handoffDir(a.Folder)))
}

// harnessCommand fills the Harness template: {skill} becomes the Plugin skill name
// and {dir} the shell-quoted Application folder.
func harnessCommand(template, skill, dir string) string {
	return strings.NewReplacer("{skill}", skill, "{dir}", QuoteShellArgument(dir)).Replace(template)
}

// QuoteShellArgument wraps s in single quotes for /bin/sh so it stays one argument however
// it is spelled: spaces stay inside, and an embedded quote is spliced to '\”.
func QuoteShellArgument(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// handoffDir is the absolute Application folder handed to {dir}.
func handoffDir(folder string) string {
	dir, err := filepath.Abs(folder)
	if err != nil {
		return folder
	}
	return dir
}
