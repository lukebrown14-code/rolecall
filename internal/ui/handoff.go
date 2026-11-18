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
	return m.runHandoff(a.Folder, harnessArgs(m.harness, skill, handoffDir(a.Folder)), toast)
}

// unsetMachineSetting is the error for a key whose machine setting is empty: it names the
// Settings row that sets it in-app, wherever config.toml lives on this machine.
func unsetMachineSetting(label string) string {
	return "No " + label + " set · 6 Settings › " + groupMachineName + " › " + label
}

// harnessArgZero is $0 of the Harness script: /bin/sh -c SCRIPT rolecall-harness DIR SKILL.
const harnessArgZero = "rolecall-harness"

// harnessRefs maps each template placeholder to the positional parameter that carries it.
var harnessRefs = []struct{ placeholder, ref string }{
	{"{dir}", "${1}"},
	{"{skill}", "${2}"},
}

// harnessArgs is the /bin/sh argument list for a Handoff. The folder and the skill never
// enter the script text: the placeholders become positional-parameter references, so a
// folder path holding $(...), backticks or quotes reaches the Harness as one literal argument.
func harnessArgs(template, skill, dir string) []string {
	return []string{"-c", harnessScript(template), harnessArgZero, dir, skill}
}

// harnessScript replaces {dir} with a reference to $1 and {skill} with one to $2, quoted
// for where the placeholder sits: bare it becomes "${1}"; inside double quotes, ${1};
// inside single quotes, which expand nothing, it closes them, adds "${1}" and reopens them.
// Quoting inside a $(...) or backtick substitution is not tracked.
func harnessScript(template string) string {
	const (
		bare = iota
		single
		double
	)
	var b strings.Builder
	state := bare
	for i := 0; i < len(template); {
		if placeholder, ref, ok := harnessPlaceholderAt(template[i:]); ok {
			switch state {
			case bare:
				b.WriteString(`"` + ref + `"`)
			case double:
				b.WriteString(ref)
			case single:
				b.WriteString(`'"` + ref + `"'`)
			}
			i += len(placeholder)
			continue
		}
		c := template[i]
		switch {
		case c == '\\' && state != single && i+1 < len(template):
			b.WriteString(template[i : i+2]) // an escaped character never opens or closes a quote
			i += 2
			continue
		case c == '\'' && state == bare:
			state = single
		case c == '\'' && state == single:
			state = bare
		case c == '"' && state == bare:
			state = double
		case c == '"' && state == double:
			state = bare
		}
		b.WriteByte(c)
		i++
	}
	return b.String()
}

// harnessPlaceholderAt reports the placeholder s starts with, if any, and its reference.
func harnessPlaceholderAt(s string) (placeholder, ref string, ok bool) {
	for _, r := range harnessRefs {
		if strings.HasPrefix(s, r.placeholder) {
			return r.placeholder, r.ref, true
		}
	}
	return "", "", false
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
