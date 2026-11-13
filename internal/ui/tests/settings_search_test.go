package ui_test

import "testing"

// searchHarness opens Settings on the Search group with the settings pane focused.
func searchHarness(t *testing.T, workspaceJSON string) *settingsHarness {
	t.Helper()
	h := newSettingsHarness(t, 120, workspaceJSON, nil)
	h.tm.Type("6")
	return h
}

func TestSettingsEditSearchAreaSavesImmediatelyKeepingOtherKeys(t *testing.T) {
	h := searchHarness(t, `{"tracks":["SWE"],"sources":[1],"search_area":{"places":["Old"],"remote":"no"}}`)
	h.tm.Type("enter")
	requireAll(t, h.tm.plain(), "Search area", "Places", "Remote")
	for range 3 {
		h.tm.Type("backspace")
	}
	h.tm.Type("Australia, APAC")
	h.tm.Type("enter")
	h.tm.Type("left") // no -> anywhere (wraps)
	h.tm.Type("enter")
	requireAll(t, h.file(), `"Australia"`, `"APAC"`, `"remote": "anywhere"`, `"sources"`, `"tracks"`)
	requireAll(t, h.tm.plain(), "Australia, APAC", "anywhere")
}

func TestSettingsSearchAreaRejectsEmptyPlaces(t *testing.T) {
	h := searchHarness(t, `{"search_area":{"places":["Old"],"remote":"no"}}`)
	h.tm.Type("enter")
	for range 3 {
		h.tm.Type("backspace")
	}
	h.tm.Type("enter")
	requireAll(t, h.tm.plain(), "Places is required")
	requireAll(t, h.file(), `"Old"`)
	h.tm.Type("esc")
	requireAll(t, h.file(), `"Old"`)
}
