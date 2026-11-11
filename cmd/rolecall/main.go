package main

import (
	tea "charm.land/bubbletea/v2"
	"errors"
	"flag"
	"fmt"
	"github.com/lukebrown14-code/rolecall/config"
	"github.com/lukebrown14-code/rolecall/store"
	"github.com/lukebrown14-code/rolecall/ui"
	"os"
	"path/filepath"
	"time"
)

// watchDebounce groups bursts of file events into one rescan.
const watchDebounce = 200 * time.Millisecond

const (
	workspaceDirMode  os.FileMode = 0o700
	workspaceFileMode os.FileMode = 0o600
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "init" {
		if err := initWorkspace(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	dir := flag.String("dir", "", "Workspace path")
	flag.Parse()
	if err := run(*dir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func initWorkspace(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: rolecall init <dir>")
	}
	root := args[0]
	info, err := os.Stat(root)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if info != nil {
		if !info.IsDir() {
			return fmt.Errorf("cannot initialize Workspace: %s is not a directory", root)
		}
		entries, err := os.ReadDir(root)
		if err != nil {
			return err
		}
		if len(entries) != 0 {
			return fmt.Errorf("cannot initialize Workspace: %s is not empty", root)
		}
	}
	if err := os.MkdirAll(root, workspaceDirMode); err != nil {
		return err
	}
	if err := os.Chmod(root, workspaceDirMode); err != nil {
		return err
	}
	for _, dir := range []string{"profile", "c" + "v", "jobs"} {
		if err := os.Mkdir(filepath.Join(root, dir), workspaceDirMode); err != nil {
			return err
		}
	}
	files := map[string]string{
		"workspace.json":                 "{\"tracks\":[]}\n",
		"profile/master-profile.md":      "# Master Profile\n",
		"profile/phrasing-bank.md":       "",
		"profile/projects.md":            "",
		"profile/writing-preferences.md": "",
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(contents), workspaceFileMode); err != nil {
			return err
		}
	}
	if err := ui.InitGit(root); err != nil {
		return err
	}
	fmt.Println("Created a private Workspace.")
	fmt.Println("The Harness can send your Profile, resume and job text to an AI service using your credentials; review its privacy practices.")
	fmt.Println("Keep this Workspace's git repository private. It contains personal data.")
	return nil
}
func run(dir string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	c, err := config.Load(home, dir)
	if err != nil {
		return err
	}
	s, err := store.Open(c.Workspace)
	if errors.Is(err, os.ErrNotExist) {
		source := "workspace in " + config.Path(home)
		if dir != "" {
			source = "--dir"
		}
		return fmt.Errorf("Workspace %s does not exist; check %s", c.Workspace, source)
	}
	if err != nil {
		return err
	}
	m, err := ui.New(s, nil, ui.WithEditor(c.Editor), ui.WithFollowUpDays(c.FollowUpDays), ui.WithDownloads(c.Downloads), ui.WithPlugin(c.Plugin), ui.WithHarness(c.Harness), ui.WithWatch(watchDebounce))
	if err != nil {
		return err
	}
	_, err = tea.NewProgram(m).Run()
	return err
}
