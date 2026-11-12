package main

import (
	tea "charm.land/bubbletea/v2"
	"errors"
	"flag"
	"fmt"
	"github.com/lukebrown14-code/rolecall/config"
	"github.com/lukebrown14-code/rolecall/doctor"
	"github.com/lukebrown14-code/rolecall/process"
	"github.com/lukebrown14-code/rolecall/store"
	"github.com/lukebrown14-code/rolecall/ui"
	"os"
	"time"
)

// watchDebounce groups bursts of file events into one rescan.
const watchDebounce = 200 * time.Millisecond

const usageErrorExitCode = 2

func main() {
	if len(os.Args) > 1 && os.Args[1] == "init" {
		if err := initWorkspace(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
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

func initWorkspace(args []string) error {
	return runInit(args, os.Stdin, os.Stdout, os.Stderr, launchProfile)
}

// runInit is the CLI seam for onboarding; launcher is injected so tests can
// exercise the profile handoff without starting an external Harness.
func runInit(args []string, in *os.File, out, errOut *os.File, launcher profileLauncher) error {
	return initWizard(args, in, out, errOut, launcher)
}

func initWorkspaceFiles(root string, contact contactDetails, tracks []string, search store.SearchArea) error {
	if err := store.InitializeWorkspace(root, contact.fields(), tracks, search); err != nil {
		return err
	}
	if err := process.InitGit(root); err != nil {
		return err
	}
	return nil
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
	if _, err := store.Open(c.Workspace); err != nil {
		return fmt.Errorf("open Workspace: %w", err)
	}
	warnings, err := doctor.Check(c.Workspace, doctor.GitHubVisibilityLookup)
	if err != nil {
		return fmt.Errorf("check Workspace permissions: %w", err)
	}
	if len(warnings) == 0 {
		fmt.Println("Workspace permissions are private.")
	} else {
		for _, warning := range warnings {
			fmt.Printf("WARNING: %s\n", warning)
		}
	}

	report := doctor.CheckSetup(doctor.Setup{Workspace: c.Workspace, Plugin: c.Plugin, Harness: c.Harness, Editor: c.Editor}, nil)
	for _, check := range report.Checks {
		if check.Found {
			fmt.Printf("OK: %s found (%s)\n", check.Name, check.Detail)
			continue
		}
		message := check.Name + " missing"
		if check.Detail != "" {
			message += " (" + check.Detail + ")"
		}
		fmt.Printf("WARNING: %s; %s\n", message, check.Hint)
	}
	fmt.Printf("Template in use: %s. See README → Make your documents yours.\n", report.Template)
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
	m, err := ui.New(s, nil, ui.WithEditor(c.Editor), ui.WithEditorSource(c.EditorSource), ui.WithFollowUpDays(c.FollowUpDays), ui.WithDownloads(c.Downloads), ui.WithPlugin(c.Plugin), ui.WithHarness(c.Harness), ui.WithConfigHome(home), ui.WithSettingSources(ui.SettingSources{Workspace: c.WorkspaceSource, FollowUpDays: c.FollowUpDaysSource, Downloads: c.DownloadsSource, Plugin: c.PluginSource, Harness: c.HarnessSource}), ui.WithASCII(c.ASCII), ui.DetectTheme(os.Stdin, os.Stdout), ui.WithWatch(watchDebounce))
	if err != nil {
		return err
	}
	_, err = tea.NewProgram(m).Run()
	return err
}
