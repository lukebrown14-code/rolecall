package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/lukebrown14-code/rolecall/internal/config"
	"github.com/lukebrown14-code/rolecall/internal/doctor"
	"github.com/lukebrown14-code/rolecall/internal/ui"
)

// initCommand is `rolecall init [DIR]`: it creates a Workspace and records it in
// config.toml, asking nothing. The questions (Contact first) are asked by the app
// itself the first time it opens, so there is one place to answer them.
func initCommand(args []string, out, errOut io.Writer) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(errOut)
	fs.Usage = func() {
		_, _ = fmt.Fprintln(errOut, "usage: rolecall init [--switch] [DIR]   (default: ./rolecall)\n\nCreates a private Workspace with its own local git repository and remembers it in\n"+
			config.Path("~")+". It asks nothing; rolecall asks for your Contact the first time it opens.\n\n"+
			"When config.toml already opens another Workspace, init leaves it as the default and\n"+
			"says how to open the new one. --switch makes DIR the default instead (DIR may be\n"+
			"an existing Workspace).")
	}
	// --switch may come after DIR too, as people type it; the flag package stops at DIR.
	switchDefault := false
	rest := args[:0:0]
	for _, a := range args {
		if a == "--switch" || a == "-switch" {
			switchDefault = true
			continue
		}
		rest = append(rest, a)
	}
	if err := fs.Parse(rest); err != nil {
		return err
	}
	if fs.NArg() > 1 {
		return errors.New("usage: rolecall init [--switch] [DIR]")
	}
	path := defaultWorkspacePath()
	if fs.NArg() == 1 {
		path = strings.TrimSpace(fs.Arg(0))
	}
	if path == "" {
		return errors.New("Workspace path is required")
	}
	root, err := filepath.Abs(expandHome(path))
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	cfgPath := config.Path(home)
	before, loadErr := config.Load(home, "")
	var broken *config.FileError
	if errors.As(loadErr, &broken) {
		// Patching a file that does not parse could make it worse; say so instead.
		return fmt.Errorf("%v\n  init created nothing", brokenConfigError(broken))
	}
	exists := isWorkspace(root)
	if exists && !switchDefault {
		return fmt.Errorf("%s is already a Workspace; open it with: rolecall --dir %s\n  To open it by default: rolecall init --switch %s",
			root, ui.QuoteShellArgument(root), ui.QuoteShellArgument(root))
	}
	if !exists {
		if err := initWorkspaceFiles(root); err != nil {
			return err
		}
	}
	// config.toml already opens another Workspace that is still there: switching it silently
	// would make the Owner's Applications seem gone, so the default stays unless --switch.
	previous := before.Workspace
	keep := !switchDefault && previous != "" && filepath.Clean(previous) != root && isWorkspace(previous)
	if !keep {
		// One Settings writer: the same config.Patch the Settings screen uses, so the
		// workspace key lands in the right place and no other setting is disturbed.
		if err := config.Patch(cfgPath, "workspace", root); err != nil {
			return err
		}
	}
	switch {
	case exists:
		_, _ = fmt.Fprintf(out, "rolecall now opens %s by default.\n", root)
	default:
		open := "rolecall"
		if keep {
			open = "rolecall --dir " + ui.QuoteShellArgument(root)
		}
		writeInitDone(out, root, before.Harness, open)
	}
	switch {
	case keep:
		_, _ = fmt.Fprintf(out, "\n%s still opens your existing Workspace, %s.\n  Open the new one with: rolecall --dir %s\n  Make it the default:   rolecall init --switch %s\n",
			cfgPath, previous, ui.QuoteShellArgument(root), ui.QuoteShellArgument(root))
	case previous != "" && filepath.Clean(previous) != root && isWorkspace(previous):
		_, _ = fmt.Fprintf(out, "Your previous Workspace is still at %s (rolecall --dir %s).\n", previous, ui.QuoteShellArgument(previous))
	}
	return nil
}

// isWorkspace reports a folder that holds a workspace.json.
func isWorkspace(root string) bool {
	info, err := os.Stat(filepath.Join(root, "workspace.json"))
	return err == nil && info.Mode().IsRegular()
}

// writeInitDone is what init prints once the Workspace exists: the next steps first, as
// a short list, then the privacy note. The Harness half of that note shows only once a
// Harness is set; before then the steps say what a Harness is. open is the command that
// opens the new Workspace.
func writeInitDone(out io.Writer, root, harness, open string) {
	lines := []string{"Created your Workspace at " + root, "", "Next:", "  1. Run `" + open + "` to open it and add your Contact."}
	if harness == "" {
		lines = append(lines,
			"  2. Connect a Harness, the AI coding assistant that runs rolecall's skills: see "+doctor.HarnessDocsURL+" (Claude Code) or "+doctor.OtherHarnessDocsURL+".",
			"  3. In your Harness, run the create-profile skill to build your Profile.")
	} else {
		lines = append(lines, "  2. In your Harness, run the create-profile skill to build your Profile.")
	}
	lines = append(lines, "", "Keep this Workspace private: it holds your personal data.")
	if harness != "" {
		lines = append(lines, "Your Harness sends the Profile, resume and job text you give it to its AI service.")
	}
	for _, l := range lines {
		_, _ = fmt.Fprintln(out, l)
	}
}

// defaultWorkspacePath is a rolecall folder in the directory init runs from.
func defaultWorkspacePath() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return filepath.Join(cwd, "rolecall")
}

// expandHome turns a leading ~ into the home directory, as a shell would; a quoted
// "~/rolecall" would otherwise create a folder named "~".
func expandHome(path string) string {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~"))
}
