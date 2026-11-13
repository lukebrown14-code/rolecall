package ui_test

import (
	"strings"
	"testing"
)

// gatesHarness opens Settings on the Documents group with the settings pane focused.
func gatesHarness(t *testing.T, workspaceJSON string) *settingsHarness {
	t.Helper()
	h := newSettingsHarness(t, 120, workspaceJSON, nil)
	h.tm.Type("6")
	h.tm.Type("]")
	return h
}

func TestSettingsEditWordCapSavesAndShows(t *testing.T) {
	h := gatesHarness(t, `{"sources":[1],"letterWordCap":350}`)
	h.tm.Type("enter")
	requireAll(t, h.tm.plain(), "Cover letter maximum")
	for range 3 {
		h.tm.Type("backspace")
	}
	h.tm.Type("300")
	h.tm.Type("enter")
	requireAll(t, h.tm.plain(), "300")
	if f := h.file(); !strings.Contains(f, `"letterWordCap": 300`) || !strings.Contains(f, `"sources"`) {
		t.Fatalf("file = %s", f)
	}
}

func TestSettingsBadGateIsRefusedWithReasonAndNothingSaved(t *testing.T) {
	h := gatesHarness(t, `{"letterWordCap":350}`)
	before := h.file()
	for val, want := range map[string]string{"abc": "whole number", "100": "above the minimum"} {
		h.tm.Type("enter")
		for range 3 {
			h.tm.Type("backspace")
		}
		h.tm.Type(val)
		h.tm.Type("enter")
		requireAll(t, h.tm.plain(), want)
		h.tm.Type("esc")
		h.tm.Type("y") // discard the typed name
	}
	if h.file() != before {
		t.Fatalf("file changed: %s", h.file())
	}
}

func TestSettingsEditKeySkillsRow(t *testing.T) {
	h := gatesHarness(t, `{}`)
	h.tm.Type("j")
	h.tm.Type("j")
	h.tm.Type("enter")
	requireAll(t, h.tm.plain(), "Key Skills minimum")
	h.tm.Type("backspace")
	h.tm.Type("6")
	h.tm.Type("enter")
	if f := h.file(); !strings.Contains(f, `"keySkillsMin": 6`) {
		t.Fatalf("file = %s", f)
	}
}

func TestSettingsHidesTemplateRowWithOneTemplate(t *testing.T) {
	h := gatesHarness(t, `{}`)
	if strings.Contains(h.tm.plain(), "Template") {
		t.Fatalf("template row shown:\n%s", h.tm.plain())
	}
}
