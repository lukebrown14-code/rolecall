package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/lukebrown14-code/rolecall/internal/config"
	"github.com/lukebrown14-code/rolecall/internal/store"
)

// moveArgs is the folder and the Status move takes.
const moveArgs = 2

// moveCommand is `rolecall move [--dir PATH] [--force] <folder> <status>`: the TUI's move
// without the TUI, so an agent or script can mark an Application Applied (or any other
// Status) and get the same sent/ copy. Where the TUI asks before a not-Ready Application
// goes to Applied (ADR-0004), this refuses unless --force says yes. It prints one line
// saying what moved.
func moveCommand(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("move", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dir := flags.String("dir", "", "Workspace path (default: workspace in config.toml)")
	force := flags.Bool("force", false, "move to Applied even when the documents are not Ready (PDFs not built)")
	flags.Usage = func() {
		_, _ = fmt.Fprintln(flags.Output(), "usage: rolecall move [--dir PATH] [--force] <folder> <status>\n\nMoves an Application to a Status as the app does; moving to Applied also saves sent/,\nand is refused while its PDFs are not built unless --force is given.\nfolder is an absolute path or relative to the Workspace; status is one of: "+strings.Join(store.Statuses, ", ")+".")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != moveArgs {
		flags.Usage()
		return errors.New("move wants a folder and a Status")
	}
	status, ok := canonicalStatus(flags.Arg(1))
	if !ok {
		return fmt.Errorf("unknown Status %q; use one of: %s", flags.Arg(1), strings.Join(store.Statuses, ", "))
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	s, err := openWorkspace(home, *dir)
	if err != nil {
		return err
	}
	folder, err := resolveFolder(s.Root(), flags.Arg(0))
	if err != nil {
		return err
	}
	return moveApplication(s, folder, status, time.Now(), *force, stdout)
}

// canonicalStatus maps a Status typed in any case to the name the file stores.
func canonicalStatus(name string) (string, bool) {
	i := slices.IndexFunc(store.Statuses, func(s string) bool { return strings.EqualFold(s, name) })
	if i < 0 {
		return "", false
	}
	return store.Statuses[i], true
}

// openWorkspace opens the Workspace --dir or config.toml names, with the same messages as
// the app.
func openWorkspace(home, dir string) (*store.Store, error) {
	s, _, err := openWorkspaceConfig(home, dir)
	return s, err
}

// openWorkspaceConfig is openWorkspace, also returning the settings it read.
func openWorkspaceConfig(home, dir string) (*store.Store, config.Config, error) {
	c, err := config.Load(home, dir)
	var configErr *config.FileError
	switch {
	case errors.As(err, &configErr) && c.Workspace == "":
		return nil, c, brokenConfigError(configErr)
	case errors.As(err, &configErr):
		// --dir names the Workspace, so a broken config.toml does not stop a move.
	case errors.Is(err, config.ErrNoWorkspace):
		return nil, c, noWorkspaceError(home)
	case err != nil:
		return nil, c, err
	}
	s, err := store.Open(c.Workspace)
	if errors.Is(err, os.ErrNotExist) {
		return nil, c, fmt.Errorf("Workspace %s does not exist", c.Workspace)
	}
	return s, c, err
}

// resolveFolder follows meta.py's resolve_folder: an absolute path as given; a relative one
// against the Workspace, then jobs/; a bare name also against each track under jobs/. An
// ambiguous name is refused rather than guessed.
func resolveFolder(workspace, value string) (string, error) {
	if filepath.IsAbs(value) {
		return filepath.Clean(value), nil
	}
	isDir := func(p string) bool { i, err := os.Stat(p); return err == nil && i.IsDir() }
	jobs := filepath.Join(workspace, "jobs")
	var found []string
	add := func(p string) {
		if isDir(p) && !slices.Contains(found, p) {
			found = append(found, p)
		}
	}
	add(filepath.Join(workspace, value))
	add(filepath.Join(jobs, value))
	if filepath.Base(value) == value {
		tracks, _ := os.ReadDir(jobs)
		for _, t := range tracks {
			if t.IsDir() {
				add(filepath.Join(jobs, t.Name(), value))
			}
		}
	}
	switch len(found) {
	case 0:
		return "", fmt.Errorf("no Application folder %q under %s", value, jobs)
	case 1:
		return found[0], nil
	}
	return "", fmt.Errorf("%q matches more than one folder (%s); give a path", value, strings.Join(found, ", "))
}

// moveApplication is the TUI's move through store.Move: append the Status, and on Applied
// freeze sent/. A not-Ready Application is refused Applied unless force, as the TUI asks.
func moveApplication(s *store.Store, folder, status string, at time.Time, force bool, out io.Writer) error {
	a, err := s.Rescan(folder)
	if err != nil {
		return err
	}
	if a == nil {
		return fmt.Errorf("%s is not an Application folder (no meta.yml)", folder)
	}
	if a.Malformed != nil {
		return fmt.Errorf("%s: %w", folder, a.Malformed)
	}
	name := folder
	if rel, err := filepath.Rel(s.Root(), folder); err == nil && !strings.HasPrefix(rel, "..") {
		name = filepath.ToSlash(rel)
	}
	if status == store.StatusApplied && a.Status != status && a.Documents != store.DocsReady && !force {
		return fmt.Errorf("%s is not Ready (%s); build its PDFs first, or pass --force to mark it Applied anyway", name, notReadyReason(*a))
	}
	r, err := s.Move(folder, status, at)
	if err != nil {
		// store.Move undoes its own half: nothing was appended, no sent/ was left.
		return fmt.Errorf("%s not moved: %w", name, err)
	}
	line := status + ": " + name
	if !r.Wrote {
		line = "Already " + line
	}
	if r.MadeSent {
		line += " (sent/ saved)"
	}
	_, err = fmt.Fprintln(out, line)
	return err
}

// notReadyReason names the PDFs a not-Ready Application still lacks.
func notReadyReason(a store.Application) string {
	var missing []string
	if !a.ResumeBuilt {
		missing = append(missing, "Resume PDF")
	}
	if !a.CoverLetterBuilt && !a.ResumeOnly {
		missing = append(missing, "Cover Letter PDF")
	}
	return strings.Join(missing, " and ") + " not built"
}
