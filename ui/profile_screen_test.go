package ui_test

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
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
	headings := []string{"Contact", "Headlines", "Experience", "Key Skills", "Summary", "Facts that are NOT", "Volunteer"}
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
// separator: the row the other Profile files sit under.
func hasProfileDivider(view string) bool {
	for _, line := range strings.Split(view, "\n") {
		left, _, ok := strings.Cut(line, "│")
		if !ok {
			continue
		}
		left = strings.TrimSpace(left)
		if len(left) >= 2 && strings.Trim(left, "─") == "" {
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
	last := strings.Index(view, "Facts that are NOT")
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
	for _, want := range []string{"j k", "section", "e", "edit file", "tab", "focus", "1-5", "screen", "rescan", "all keys", "quit"} {
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

func TestProfileHelpOmitsEditFileWithNoProfile(t *testing.T) {
	root := fixtureCopy(t, func(root string) {
		if err := os.Remove(filepath.Join(root, "profile", "master-profile.md")); err != nil {
			t.Fatal(err)
		}
	})
	view := ansi.Strip(runKeys(t, root, 120, "5", "?"))
	if strings.Contains(view, "edit file") {
		t.Fatalf("help offers e with no Profile:\n%s", view)
	}
}

func TestProfileGolden(t *testing.T) {
	teatest.RequireEqualOutput(t, []byte(ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "5"))))
}
