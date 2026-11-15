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
)

// initCommand is `rolecall init [DIR]`: it creates a Workspace and records it in
// config.toml, asking nothing. The questions (Contact first) are asked by the app
// itself the first time it opens, so there is one place to answer them.
func initCommand(args []string, out, errOut io.Writer) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(errOut)
	fs.Usage = func() { _, _ = fmt.Fprintln(errOut, "usage: rolecall init [DIR]   (default: ./rolecall)") }
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
	if err := initWorkspaceFiles(root); err != nil {
		return err
	}
	// One Settings writer: the same config.Patch the Settings screen uses, so the
	// workspace key lands in the right place and no other setting is disturbed.
	if err := config.Patch(config.Path(home), "workspace", root); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(out, "Created your Workspace at %s\n", root)
	_, _ = fmt.Fprintln(out, "Keep it private: it holds personal data, and the Harness can send your Profile, resume and job text to an AI service.")
	_, _ = fmt.Fprintln(out, "Next: run `rolecall` to finish setting up.")
	return nil
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
