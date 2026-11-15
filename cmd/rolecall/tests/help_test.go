package main_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// --help names the subcommands, so init and doctor can be found from the binary, and
// asking for help is not a failure.
func TestCLIHelpListsSubcommandsAndExitsZero(t *testing.T) {
	binary := buildRolecall(t)
	for _, args := range [][]string{{"--help"}, {"init", "--help"}, {"doctor", "--help"}} {
		cmd := exec.Command(binary, args...)
		cmd.Env = append(os.Environ(), "HOME="+t.TempDir())
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Errorf("%v: exit %v\n%s", args, err, out)
		}
		if strings.Contains(string(out), "help requested") {
			t.Errorf("%v prints the flag package's error:\n%s", args, out)
		}
		if args[0] == "--help" {
			for _, want := range []string{"rolecall init", "rolecall doctor", "-dir", "-version"} {
				if !strings.Contains(string(out), want) {
					t.Errorf("--help missing %q:\n%s", want, out)
				}
			}
		}
	}
}
