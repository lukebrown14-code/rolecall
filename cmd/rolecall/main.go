package main

import (
	tea "charm.land/bubbletea/v2"
	"errors"
	"flag"
	"fmt"
	"github.com/lukebrown14-code/rolecall/config"
	"github.com/lukebrown14-code/rolecall/doctor"
	"github.com/lukebrown14-code/rolecall/store"
	"github.com/lukebrown14-code/rolecall/ui"
	"os"
	"time"
)

// watchDebounce groups bursts of file events into one rescan.
const watchDebounce = 200 * time.Millisecond

const usageErrorExitCode = 2

func main() {
	if len(os.Args) > 1 && os.Args[1] == "doctor" {
		flags := flag.NewFlagSet("doctor", flag.ContinueOnError)
		dir := flags.String("dir", "", "Workspace path")
		if err := flags.Parse(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(usageErrorExitCode)
		}
		if err := runDoctor(*dir); err != nil {
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

func runDoctor(dir string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	c, err := config.Load(home, dir)
	if err != nil {
		return err
	}
	warnings, err := doctor.Check(c.Workspace, doctor.GitHubVisibilityLookup)
	if err != nil {
		return fmt.Errorf("check Workspace permissions: %w", err)
	}
	if len(warnings) == 0 {
		fmt.Println("Workspace permissions are private.")
		return nil
	}
	for _, warning := range warnings {
		fmt.Printf("WARNING: %s\n", warning)
	}
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
