package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// hostileDir makes a real folder whose name would run commands or break quoting if it
// were spliced into the shell script: $(...), backticks, both quotes, spaces and a newline.
func hostileDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "a b $(touch pwned) `touch pwned` \"dq 'sq\nline two")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return dir
}

// runHarness runs the Handoff exactly as runHandoff does, through real /bin/sh, with
// printf standing in for the Harness so each argument it receives comes back NUL-separated.
func runHarness(t *testing.T, template, skill, dir string) []string {
	t.Helper()
	work := t.TempDir()
	c := exec.Command("/bin/sh", harnessArgs(template, skill, dir)...)
	c.Dir = work
	out, err := c.Output()
	if err != nil {
		t.Fatalf("%s: %v", template, err)
	}
	for _, where := range []string{work, dir, filepath.Dir(dir)} {
		if _, err := os.Stat(filepath.Join(where, "pwned")); err == nil {
			t.Fatalf("%s: the folder name executed a command", template)
		}
	}
	return strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
}

func TestHarnessPassesHostileDirAsOneLiteralArgument(t *testing.T) {
	dir := hostileDir(t)
	const skill = "assess-fit"
	for _, tc := range []struct {
		name, template string
		want           []string
	}{
		// The documented forms: docs/harnesses.md, the Settings example and doctor's hint.
		{"inside double quotes", `printf '%s\0' "/rolecall:{skill} {dir}"`, []string{"/rolecall:" + skill + " " + dir}},
		{"bare", `printf '%s\0' --cd {dir} "Use the {skill} skill."`, []string{"--cd", dir, "Use the " + skill + " skill."}},
		{"bare after cd", `cd {dir} && printf '%s\0' "$PWD" {skill}`, []string{dir, skill}},
		// Forms an Owner may write.
		{"inside single quotes", `printf '%s\0' '{dir}' 'run {skill} in {dir} now'`, []string{dir, "run " + skill + " in " + dir + " now"}},
		{"beside escaped double quotes", `printf '%s\0' "say \"{dir}\""`, []string{`say "` + dir + `"`}},
		{"bare, glued to text", `printf '%s\0' --dir={dir}/x`, []string{"--dir=" + dir + "/x"}},
	} {
		if got := runHarness(t, tc.template, skill, dir); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: harness got %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestHarnessScriptNeverContainsTheFolder(t *testing.T) {
	dir := hostileDir(t)
	args := harnessArgs(`claude "/rolecall:{skill} {dir}"`, "assess-fit", dir)
	want := []string{"-c", `claude "/rolecall:${2} ${1}"`, harnessArgZero, dir, "assess-fit"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %q, want %q", args, want)
	}
}
