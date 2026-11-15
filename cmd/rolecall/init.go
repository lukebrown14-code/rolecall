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
	"github.com/lukebrown14-code/rolecall/internal/ui"
)

// initCommand is `rolecall init [DIR]`: it creates a Workspace and records it in
// config.toml, asking nothing. The questions (Contact first) are asked by the app
// itself the first time it opens, so there is one place to answer them.
func initCommand(args []string, out, errOut io.Writer) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(errOut)
	fs.Usage = func() {
		_, _ = fmt.Fprintln(errOut, "usage: rolecall init [DIR]   (default: ./rolecall)\n\nCreates a private Workspace with its own local git repository and remembers it in\n"+
			config.Path("~")+". It asks nothing; rolecall asks for your Contact the first time it opens.")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 1 {
		return errors.New("usage: rolecall init [DIR]")
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
	if _, err := os.Stat(filepath.Join(root, "workspace.json")); err == nil {
		return fmt.Errorf("%s is already a Workspace; open it with: rolecall --dir %s", root, ui.QuoteShellArgument(root))
	}
	if err := initWorkspaceFiles(root); err != nil {
		return err
	}
	// One Settings writer: the same config.Patch the Settings screen uses, so the
	// workspace key lands in the right place and no other setting is disturbed.
	if err := config.Patch(config.Path(home), "workspace", root); err != nil {
		return err
	}
	harness := ""
	if c, err := config.Load(home, ""); err == nil {
		harness = c.Harness
	}
	writeInitDone(out, root, harness)
	return nil
}

// writeInitDone is what init prints once the Workspace exists: the next steps first, as
// a short list, then the privacy note. The Harness half of that note shows only once a
// Harness is set; before then the steps say what a Harness is.
func writeInitDone(out io.Writer, root, harness string) {
	lines := []string{"Created your Workspace at " + root, "", "Next:", "  1. Run `rolecall` to open it and add your Contact."}
	if harness == "" {
		lines = append(lines,
			"  2. Connect a Harness, the AI coding assistant that runs rolecall's skills: see README › Connect your Harness.",
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
