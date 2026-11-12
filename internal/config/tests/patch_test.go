package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lukebrown14-code/rolecall/internal/config"
)

func patchFile(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), ".config/rolecall/config.toml")
	if body != "" {
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return p
}

func read(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestPatchKeepsCommentsAndOtherLines(t *testing.T) {
	p := patchFile(t, "# my config\nworkspace = \"~/work\" # where\n\n# editor of choice\neditor = 'vim' # old\nfollow_up_days = 14\n")
	if err := config.Patch(p, "editor", "nano"); err != nil {
		t.Fatal(err)
	}
	if err := config.Patch(p, "follow_up_days", 30); err != nil {
		t.Fatal(err)
	}
	want := "# my config\nworkspace = \"~/work\" # where\n\n# editor of choice\neditor = \"nano\" # old\nfollow_up_days = 30\n"
	if got := read(t, p); got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestPatchAddsMissingKeyAndCreatesFile(t *testing.T) {
	p := patchFile(t, "# c\nworkspace = \"/w\"\n[other]\nx = 1\n")
	if err := config.Patch(p, "harness", `claude "/{skill} {dir}"`); err != nil {
		t.Fatal(err)
	}
	want := "# c\nworkspace = \"/w\"\nharness = \"claude \\\"/{skill} {dir}\\\"\"\n[other]\nx = 1\n"
	if got := read(t, p); got != want {
		t.Fatalf("got:\n%s", got)
	}
	n := patchFile(t, "")
	if err := config.Patch(n, "ascii", true); err != nil {
		t.Fatal(err)
	}
	if got := read(t, n); got != "ascii = true\n" {
		t.Fatalf("new file: %q", got)
	}
}

func TestAsciiParsedAndRoundTrips(t *testing.T) {
	home := t.TempDir()
	p := config.Path(home)
	if err := config.Patch(p, "workspace", "/w"); err != nil {
		t.Fatal(err)
	}
	if c, err := config.Load(home, ""); err != nil || c.ASCII {
		t.Fatalf("default: %+v %v", c, err)
	}
	if err := config.Patch(p, "harness", `a "b" \ c`); err != nil {
		t.Fatal(err)
	}
	if err := config.Patch(p, "ascii", true); err != nil {
		t.Fatal(err)
	}
	if c, err := config.Load(home, ""); err != nil || !c.ASCII || c.Harness != `a "b" \ c` {
		t.Fatalf("%+v %v", c, err)
	}
}

func TestPatchFixtureConfigKeepsEveryComment(t *testing.T) {
	orig, err := os.ReadFile("../../../fixtures/config.toml")
	if err != nil {
		t.Fatal(err)
	}
	p := patchFile(t, string(orig))
	if err := config.Patch(p, "ascii", true); err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(string(orig), "ascii = false #", "ascii = true #", 1)
	if got := read(t, p); got != want {
		t.Fatalf("got:\n%s", got)
	}
}
