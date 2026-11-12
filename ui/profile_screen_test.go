package ui_test

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/teatest/v2"
)

// noProfile is the one message a missing Profile shows.
const noProfile = "No Profile yet: profile/master-profile.md not found"

// writeMasterProfile replaces the copied Workspace's Profile.
func writeMasterProfile(t *testing.T, root, text string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "profile", "master-profile.md"), []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

// profileFiles is every file under profile/, with its content and modification time, so a
// test can prove the Profile screen never writes there.
func profileFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	dir := filepath.Join(root, "profile")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		files[e.Name()] = info.ModTime().String() + "\n" + string(b)
	}
	return files
}

func TestProfileOpensOnFiveFromAnyScreen(t *testing.T) {
	for _, from := range []string{"1", "2", "3", "4", "5"} {
		view := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, from, "5"))
		if !strings.Contains(view, "5 Profile") {
			t.Fatalf("5 from screen %s did not open Profile:\n%s", from, view)
		}
	}
}

// Switching screens closes an open detail view, so 5 from a detail leaves it behind
// rather than keeping its keys and its frame.
func TestProfileScreenKeyClosesAnOpenDetail(t *testing.T) {
	view := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "3", "enter", "5"))
	for _, want := range []string{"5 Profile", "alex.quinn@example.test"} {
		if !strings.Contains(view, want) {
			t.Fatalf("5 from an open detail lacks %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "‹ esc") {
		t.Fatalf("the detail view survived 5:\n%s", view)
	}
}

// Sections follow the file, a heading added by the Owner is included, and the `#` title is
// not a section.
func TestProfileSectionsFollowTheFileAndSkipTheTitle(t *testing.T) {
	root := fixtureCopy(t, func(root string) {
		path := filepath.Join(root, "profile", "master-profile.md")
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		writeMasterProfile(t, root, string(b)+"\n## Volunteer\n\nCommunity mentoring.\n")
	})
	view := ansi.Strip(runKeys(t, root, 120, "5"))
	// The list column cuts long headings, so each is matched by its start.
	headings := []string{"Contact", "Headlines", "Experience", "Key Skills", "Summary", "Facts that are", "Volunteer"}
	last := -1
	for _, heading := range headings {
		i := strings.Index(view, heading)
		if i < 0 {
			t.Fatalf("section %q missing:\n%s", heading, view)
		}
		if i < last {
			t.Fatalf("section %q out of file order:\n%s", heading, view)
		}
		last = i
	}
	if strings.Contains(view, "Master Profile") {
		t.Fatalf("the # title became a section:\n%s", view)
	}
}

func TestProfileJKMoveSectionAndContentFollows(t *testing.T) {
	contact := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "5"))
	if !strings.Contains(contact, "alex.quinn@example.test") {
		t.Fatalf("Profile did not open on Contact:\n%s", contact)
	}
	headlines := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "5", "j"))
	if strings.Contains(headlines, "alex.quinn@example.test") || !strings.Contains(headlines, "Full Stack Software Engineer") {
		t.Fatalf("j did not move to Headlines:\n%s", headlines)
	}
	back := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "5", "j", "k"))
	if !strings.Contains(back, "alex.quinn@example.test") {
		t.Fatalf("k did not move back to Contact:\n%s", back)
	}
}

// Glamour moves a table's links to a footnote unless told to keep them inline, which
// left the Contact table's Email cell empty and the address below the table.
func TestProfileContactEmailRendersInlineInTheTable(t *testing.T) {
	view := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "5"))
	email := "alex.quinn@example.test"
	cell := ""
	for _, line := range strings.Split(view, "\n") {
		_, right, ok := strings.Cut(line, "│")
		if ok && strings.Contains(right, "Email") {
			cell = right
			break
		}
	}
	if !strings.Contains(cell, email) {
		t.Fatalf("Email cell does not hold the address inline: %q\n%s", cell, view)
	}
	if footnote := regexp.MustCompile(`\[\d+\]:`); footnote.MatchString(view) {
		t.Fatalf("Contact still renders a link footnote:\n%s", view)
	}
}

// profileListRowText is the list column of the row whose heading starts with name, with
// the padding before the divider trimmed. The content pane is ignored, so a warning
// that names a section does not read as a list row. ASCII folds the pane separator, so
// both forms are accepted.
func profileListRowText(view, name string) (string, bool) {
	for _, line := range strings.Split(view, "\n") {
		i := strings.IndexAny(line, "│|")
		if i < 0 || !strings.Contains(line[:i], name) {
			continue
		}
		return strings.TrimRight(line[:i], " "), true
	}
	return "", false
}

// requireProfileListRow is profileListRowText, failing when the row is missing.
func requireProfileListRow(t *testing.T, view, name string) string {
	t.Helper()
	row, ok := profileListRowText(view, name)
	if !ok {
		t.Fatalf("no list row for %q:\n%s", name, view)
	}
	return row
}

// removeProfileLines deletes whole lines from a temp copy's Profile, matching the line
// with surrounding whitespace trimmed. Each argument is one whole line.
func removeProfileLines(t *testing.T, root string, drop ...string) {
	t.Helper()
	path := filepath.Join(root, "profile", "master-profile.md")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var kept []string
	for _, line := range strings.Split(string(b), "\n") {
		remove := false
		for _, want := range drop {
			if strings.TrimSpace(line) == want {
				remove = true
			}
		}
		if !remove {
			kept = append(kept, line)
		}
	}
	writeMasterProfile(t, root, strings.Join(kept, "\n"))
}

// Every section row carries its count right-aligned; prose shows none, and Contact
// shows ✓ from the store's Contact reader rather than a count.
func TestProfileSectionRowsShowCounts(t *testing.T) {
	view := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "5"))
	for _, tc := range []struct{ section, count string }{
		{"Contact", "✓"},
		{"Headlines", "2"},
		{"Experience", "2"},
		{"Key Skills", "8"},
		{"Facts that are", "2"},
	} {
		row := requireProfileListRow(t, view, tc.section)
		if !strings.HasSuffix(row, tc.count) {
			t.Fatalf("%s row = %q, want count %q:\n%s", tc.section, row, tc.count, view)
		}
	}
	if row := requireProfileListRow(t, view, "Summary"); !strings.HasSuffix(row, "Summary") {
		t.Fatalf("Summary row = %q, want no count:\n%s", row, view)
	}
}

// A missing required field marks Contact ✗ and says at the top of the content pane which
// field is missing and that builds will fail.
func TestProfileContactMarkAndWarningFollowMissingFields(t *testing.T) {
	cases := []struct {
		name    string
		drop    []string
		warning string
	}{
		{
			"email",
			[]string{"| Email | alex.quinn@example.test |"},
			"⚠ Contact is missing Email. Builds will fail until it is added.",
		},
		{
			"name",
			[]string{"| Name | Alex Quinn |"},
			"⚠ Contact is missing Name. Builds will fail until it is added.",
		},
		{
			"both",
			[]string{"| Name | Alex Quinn |", "| Email | alex.quinn@example.test |"},
			"⚠ Contact is missing Name and Email. Builds will fail until they are added.",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := fixtureCopy(t, func(root string) { removeProfileLines(t, root, tc.drop...) })
			view := ansi.Strip(runKeys(t, root, 120, "5"))
			if row := requireProfileListRow(t, view, "Contact"); !strings.HasSuffix(row, "✗") {
				t.Fatalf("Contact row = %q, want ✗:\n%s", row, view)
			}
			if !strings.Contains(view, tc.warning) {
				t.Fatalf("warning %q absent:\n%s", tc.warning, view)
			}
			if strings.Index(view, tc.warning) > strings.Index(view, "+61 400 000 000") {
				t.Fatalf("warning is not above the Contact table:\n%s", view)
			}
		})
	}
}

// Without a Contact section there is no row to mark, but the warning still names both
// required fields and stays visible on whichever section is selected.
func TestProfileWithoutContactSectionStillWarns(t *testing.T) {
	root := fixtureCopy(t, func(root string) {
		path := filepath.Join(root, "profile", "master-profile.md")
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(b)
		start := strings.Index(text, "## Contact")
		if start < 0 {
			t.Fatal("fixture has no Contact section")
		}
		end := start + strings.Index(text[start:], "## Headlines")
		if end <= start {
			t.Fatal("fixture has no Headlines section after Contact")
		}
		writeMasterProfile(t, root, text[:start]+text[end:])
	})
	view := ansi.Strip(runKeys(t, root, 120, "5", "j"))
	warning := "⚠ Contact is missing Name and Email. Builds will fail until they are added."
	if !strings.Contains(view, warning) {
		t.Fatalf("warning %q absent:\n%s", warning, view)
	}
	if _, ok := profileListRowText(view, "Contact"); ok {
		t.Fatalf("a Contact row survived the removed section:\n%s", view)
	}
	for _, want := range []string{"Acme — Brisbane, AU", "Experience"} {
		if !strings.Contains(view, want) {
			t.Fatalf("screen lost %q:\n%s", want, view)
		}
	}
}

func TestProfileTabFocusScrollsALongSection(t *testing.T) {
	var lines []string
	for i := 1; i <= 60; i++ {
		lines = append(lines, fmt.Sprintf("- line %02d", i))
	}
	root := fixtureCopy(t, func(root string) {
		writeMasterProfile(t, root, "# Title\n\n## Long\n\n"+strings.Join(lines, "\n")+"\n")
	})
	top := ansi.Strip(runKeys(t, root, 120, "5"))
	if !strings.Contains(top, "line 01") || strings.Contains(top, "line 40") {
		t.Fatalf("long section did not start at the top:\n%s", top)
	}
	keys := []string{"5", "tab"}
	for range 20 {
		keys = append(keys, "j")
	}
	scrolled := ansi.Strip(runKeys(t, root, 120, keys...))
	if !strings.Contains(scrolled, "CONTENT") {
		t.Fatalf("tab did not focus the content:\n%s", scrolled)
	}
	if !strings.Contains(scrolled, "line 21") || strings.Contains(scrolled, "line 01") {
		t.Fatalf("j did not scroll the content:\n%s", scrolled)
	}
}

func TestProfileMissingShowsTheMessageAndNoList(t *testing.T) {
	root := fixtureCopy(t, func(root string) {
		if err := os.Remove(filepath.Join(root, "profile", "master-profile.md")); err != nil {
			t.Fatal(err)
		}
	})
	view := ansi.Strip(runKeys(t, root, 120, "5"))
	if !strings.Contains(view, noProfile) {
		t.Fatalf("missing Profile message absent:\n%s", view)
	}
	for _, unwanted := range []string{"SECTIONS", "CONTACT", "Contact", "Experience"} {
		if strings.Contains(view, unwanted) {
			t.Fatalf("missing Profile still shows %q:\n%s", unwanted, view)
		}
	}
}

// hasProfileDivider reports a run of rule dashes in the list column, before the pane
// separator: the row the other Profile files sit under. ASCII folds both the dashes and
// the separator, so both forms are accepted.
func hasProfileDivider(view string) bool {
	for _, line := range strings.Split(view, "\n") {
		i := strings.IndexAny(line, "│|")
		if i < 0 {
			continue
		}
		left := strings.TrimSpace(line[:i])
		if len(left) >= 2 && strings.Trim(left, "─-") == "" {
			return true
		}
	}
	return false
}

// profileKeysToFile opens the fixture Profile and walks past its six sections to the
// first other file.
var profileKeysToFile = []string{"5", "j", "j", "j", "j", "j", "j"}

// The other Profile files that exist sit below a divider, after the sections.
func TestProfileShowsExistingOtherFilesBelowTheDivider(t *testing.T) {
	view := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "5"))
	if !hasProfileDivider(view) {
		t.Fatalf("no divider above the other Profile files:\n%s", view)
	}
	last := strings.Index(view, "Facts that are")
	projects := strings.Index(view, "projects.md")
	if last < 0 || projects < 0 || projects < last {
		t.Fatalf("projects.md not below the sections:\n%s", view)
	}
	if !strings.Contains(view[last:projects], "─") {
		t.Fatalf("projects.md not below a divider:\n%s", view)
	}
	for _, absent := range []string{"writing-preferences.md", "phrasing-bank.md"} {
		if strings.Contains(view, absent) {
			t.Fatalf("%s listed but not in the Workspace:\n%s", absent, view)
		}
	}
}

// j crosses from the last section to the first file and k crosses straight back.
func TestProfileJKCrossesTheDivider(t *testing.T) {
	root := fixtureCopy(t, nil)
	file := ansi.Strip(runKeys(t, root, 120, profileKeysToFile...))
	if !strings.Contains(file, "▸ projects.md") || !strings.Contains(file, "prints local tide times") {
		t.Fatalf("j did not reach projects.md:\n%s", file)
	}
	keys := append(append([]string{}, profileKeysToFile...), "k")
	back := ansi.Strip(runKeys(t, root, 120, keys...))
	if !strings.Contains(back, "No Kubernetes production experience") || strings.Contains(back, "prints local tide times") {
		t.Fatalf("k did not cross back to the last section:\n%s", back)
	}
}

// A selected file renders whole, and the context row names it rather than the Profile.
func TestProfileSelectingAFileRendersItAndShowsItsPath(t *testing.T) {
	view := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, profileKeysToFile...))
	for _, want := range []string{"▸ projects.md", "Tide Tables", "A command-line tool that prints local tide times. Go, open source.", "profile/projects.md"} {
		if !strings.Contains(view, want) {
			t.Fatalf("selected projects.md lacks %q:\n%s", want, view)
		}
	}
}

// With no other Profile files there is nothing under a divider, so the divider goes too.
func TestProfileWithoutOtherFilesHasNoDivider(t *testing.T) {
	root := fixtureCopy(t, func(root string) {
		if err := os.Remove(filepath.Join(root, "profile", "projects.md")); err != nil {
			t.Fatal(err)
		}
	})
	view := ansi.Strip(runKeys(t, root, 120, "5"))
	if hasProfileDivider(view) {
		t.Fatalf("divider shown with no other Profile files:\n%s", view)
	}
}

// A missing master-profile.md is still the missing Profile, other files or not.
func TestProfileMissingMasterProfileKeepsTheMessage(t *testing.T) {
	root := fixtureCopy(t, func(root string) {
		if err := os.Remove(filepath.Join(root, "profile", "master-profile.md")); err != nil {
			t.Fatal(err)
		}
	})
	view := ansi.Strip(runKeys(t, root, 120, "5"))
	if !strings.Contains(view, noProfile) {
		t.Fatalf("missing Profile message absent:\n%s", view)
	}
	for _, unwanted := range []string{"SECTIONS", "projects.md"} {
		if strings.Contains(view, unwanted) {
			t.Fatalf("missing Profile still shows %q:\n%s", unwanted, view)
		}
	}
}

func TestProfileWithoutHeadingsShowsOneSection(t *testing.T) {
	root := fixtureCopy(t, func(root string) {
		writeMasterProfile(t, root, "Just prose.\n\nMore prose.\n")
	})
	view := ansi.Strip(runKeys(t, root, 120, "5"))
	for _, want := range []string{"▸ Profile", "Just prose.", "More prose."} {
		if !strings.Contains(view, want) {
			t.Fatalf("one-section fallback lacks %q:\n%s", want, view)
		}
	}
}

// Application and row keys, and the track filter, leave Profile and the Workspace alone.
func TestProfileApplicationKeysAndTrackAreInert(t *testing.T) {
	root := fixtureCopy(t, nil)
	before := profileFiles(t, root)
	plain := ansi.Strip(runKeys(t, root, 120, "5"))
	withKeys := ansi.Strip(runKeys(t, root, 120, "5", "a", "b", "f", "p", "n", "x", "d", "m", "u", "v", "enter", "/", "g", "G", "t"))
	if plain != withKeys {
		t.Fatalf("an Application key or t changed Profile:\n--- plain ---\n%s\n--- with keys ---\n%s", plain, withKeys)
	}
	if after := profileFiles(t, root); !reflect.DeepEqual(before, after) {
		t.Fatalf("Profile files changed:\nbefore %v\nafter  %v", before, after)
	}
}

func TestProfileHelpListsOnlyValidKeys(t *testing.T) {
	view := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "5", "?"))
	for _, want := range []string{"j k", "section", "e", "edit file", "tab", "focus", "1-6", "screen", "rescan", "help", "quit"} {
		if !strings.Contains(view, want) {
			t.Fatalf("help lacks %q:\n%s", want, view)
		}
	}
	for _, unwanted := range []string{"move", "add", "search", "notes", "dismiss", "download", "build", "handoff", "enter"} {
		if strings.Contains(view, unwanted) {
			t.Fatalf("help shows %q on Profile:\n%s", unwanted, view)
		}
	}
}

// With no Profile, j, tab and e do nothing, so the help lists only what works.
func TestProfileHelpOmitsInertKeysWithNoProfile(t *testing.T) {
	root := fixtureCopy(t, func(root string) {
		if err := os.Remove(filepath.Join(root, "profile", "master-profile.md")); err != nil {
			t.Fatal(err)
		}
	})
	view := ansi.Strip(runKeys(t, root, 120, "5", "?"))
	for _, wanted := range []string{"1-6", "screen", "rescan", "help", "quit"} {
		if !strings.Contains(view, wanted) {
			t.Fatalf("help lacks %q with no Profile:\n%s", wanted, view)
		}
	}
	for _, unwanted := range []string{"edit file", "j k", "section", "tab", "focus"} {
		if strings.Contains(view, unwanted) {
			t.Fatalf("help offers %q with no Profile:\n%s", unwanted, view)
		}
	}
}

// Below 80 columns the section list has no room: it is hidden and the context row names
// the selected entry and its 1-based position instead. At 80 columns the list is back.
func TestProfileNarrowTerminalHidesTheListAndNamesTheSelection(t *testing.T) {
	narrow := ansi.Strip(runKeysSized(t, fixtureCopy(t, nil), 79, 24, "5", "j", "j"))
	if !strings.Contains(narrow, "Experience 3/6") {
		t.Fatalf("narrow context row lacks the selection and position:\n%s", narrow)
	}
	for _, hidden := range []string{"SECTIONS", "Key Skills", "projects.md"} {
		if strings.Contains(narrow, hidden) {
			t.Fatalf("narrow layout still shows %q:\n%s", hidden, narrow)
		}
	}
	wide := ansi.Strip(runKeysSized(t, fixtureCopy(t, nil), 80, 24, "5"))
	if !strings.Contains(wide, "SECTIONS") || !strings.Contains(wide, "Key Skills") {
		t.Fatalf("80 columns still hides the section list:\n%s", wide)
	}
}

// j and k still move the selection when the list is hidden, and the content follows.
func TestProfileNarrowTerminalJKMoveTheSelection(t *testing.T) {
	root := fixtureCopy(t, nil)
	headlines := ansi.Strip(runKeysSized(t, root, 79, 24, "5", "j"))
	if !strings.Contains(headlines, " 2/6") || !strings.Contains(headlines, "Full Stack Software Engineer") {
		t.Fatalf("j did not move to Headlines:\n%s", headlines)
	}
	back := ansi.Strip(runKeysSized(t, root, 79, 24, "5", "j", "k"))
	if !strings.Contains(back, " 1/6") || !strings.Contains(back, "alex.quinn@example.test") {
		t.Fatalf("k did not move back to Contact:\n%s", back)
	}
}

// With the list hidden there is no second pane to focus, so tab stays on the list and j
// still changes the section instead of scrolling the content.
func TestProfileNarrowTerminalTabDoesNotFocusTheHiddenList(t *testing.T) {
	view := ansi.Strip(runKeysSized(t, fixtureCopy(t, nil), 79, 24, "5", "tab", "j"))
	for _, want := range []string{"2/6", "Full Stack Software Engineer"} {
		if !strings.Contains(view, want) {
			t.Fatalf("tab then j did not move to Headlines (%q):\n%s", want, view)
		}
	}
	if strings.Contains(view, "alex.quinn@example.test") {
		t.Fatalf("tab then j did not leave Contact:\n%s", view)
	}
}

// The narrow position continues from the sections into the other files under the divider.
func TestProfileNarrowPositionCountsTheOtherFiles(t *testing.T) {
	view := ansi.Strip(runKeysSized(t, fixtureCopy(t, nil), 79, 24, profileKeysToFile...))
	if !strings.Contains(view, "projects.md 7/7") {
		t.Fatalf("narrow context row lacks projects.md 7/7:\n%s", view)
	}
	if !strings.Contains(view, "profile/projects.md") {
		t.Fatalf("narrow context row lost the file path:\n%s", view)
	}
}

// Below the minimum size the too-small notice replaces Profile, as on every screen.
func TestProfileTooSmallShowsTheNotice(t *testing.T) {
	view := ansi.Strip(runKeysSized(t, fixtureCopy(t, nil), 49, 11, "5"))
	if !strings.Contains(view, "rolecall needs") || !strings.Contains(view, "now 49×11 · resize to continue") {
		t.Fatalf("no too-small notice:\n%s", view)
	}
}

// The wide layouts keep their goldens; the narrow, missing, warning and ASCII states get
// one each.
func TestProfileGolden(t *testing.T) {
	teatest.RequireEqualOutput(t, []byte(ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "5"))))
}

func TestProfileNarrowGolden(t *testing.T) {
	teatest.RequireEqualOutput(t, []byte(ansi.Strip(runKeysSized(t, fixtureCopy(t, nil), 79, 24, "5", "j", "j"))))
}

func TestProfileMissingGolden(t *testing.T) {
	root := fixtureCopy(t, func(root string) {
		if err := os.Remove(filepath.Join(root, "profile", "master-profile.md")); err != nil {
			t.Fatal(err)
		}
	})
	teatest.RequireEqualOutput(t, []byte(ansi.Strip(runKeys(t, root, 120, "5"))))
}

func TestProfileWarningGolden(t *testing.T) {
	root := fixtureCopy(t, func(root string) {
		removeProfileLines(t, root, "| Email | alex.quinn@example.test |")
	})
	teatest.RequireEqualOutput(t, []byte(ansi.Strip(runKeys(t, root, 120, "5"))))
}

func TestProfileAsciiGolden(t *testing.T) {
	t.Setenv("ROLECALL_ASCII", "1")
	view := ansi.Strip(runKeysSized(t, fixtureCopy(t, nil), 120, 36, "5"))
	for _, r := range view {
		if r > 127 {
			t.Fatalf("non-ASCII glyph %q survived ROLECALL_ASCII:\n%s", r, view)
		}
	}
	teatest.RequireEqualOutput(t, []byte(view))
}
