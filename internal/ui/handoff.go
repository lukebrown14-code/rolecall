package ui

import (
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/internal/store"
)

// The Plugin skills a Handoff can run.
const (
	skillAssessFit        = "assess-fit"
	skillPrepareDocuments = "prepare-documents"
	skillDraftFollowUp    = "draft-follow-up"
)

// followUpDraftFile is the message the draft-follow-up skill writes in the Application's
// folder. The TUI only names it; it never sends it (ADR 0005).
const followUpDraftFile = "follow-up.md"

// handoffAllowed reports whether a skill applies to an Application: assess-fit on
// any, prepare-documents only on a Lead or Draft, draft-follow-up only when Follow-up due.
func (m model) handoffAllowed(a store.Application, skill string) bool {
	switch skill {
	case skillAssessFit:
		return true
	case skillPrepareDocuments:
		return a.Unsent()
	case skillDraftFollowUp:
		return followUpDue(a, m.now(), m.followUpDays)
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
		m.err = errNoHarness
		return nil
	}
	if !m.handoffAllowed(a, skill) {
		return nil
	}
	toast := ""
	if skill == skillDraftFollowUp {
		toast = "Draft written to " + m.rel(filepath.Join(a.Folder, followUpDraftFile))
	}
	return m.runHandoff(a.Folder, harnessCommand(m.harness, skill, handoffDir(a.Folder)), toast)
}

// unsetMachineSetting is the error for a key whose machine setting is empty: it names the
// Settings row that sets it in-app, wherever config.toml lives on this machine.
func unsetMachineSetting(label string) string {
	return "No " + label + " set · 6 Settings › " + groupMachineName + " › " + label
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
