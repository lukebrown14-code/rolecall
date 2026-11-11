package ui

import "testing"

func TestHarnessCommandExpandsAndQuotesDir(t *testing.T) {
	for _, tc := range []struct {
		name, template, skill, dir, want string
	}{
		{"skill and dir", `claude "/{skill} {dir}"`, "assess-fit", "/tmp/jobs/fern", `claude "/assess-fit '/tmp/jobs/fern'"`},
		{"spaces stay one argument", "run {skill} --dir {dir}", "prepare-documents", "/tmp/a b/fern", "run prepare-documents --dir '/tmp/a b/fern'"},
		{"an embedded quote is spliced", "x {dir}", "assess-fit", "/tmp/o'brien", `x '/tmp/o'\''brien'`},
	} {
		if got := harnessCommand(tc.template, tc.skill, tc.dir); got != tc.want {
			t.Fatalf("%s: got %q want %q", tc.name, got, tc.want)
		}
	}
}
