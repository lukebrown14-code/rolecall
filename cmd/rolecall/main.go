package main

import (
	tea "charm.land/bubbletea/v2"
	"errors"
	"flag"
	"fmt"
	"github.com/lukebrown14-code/rolecall/internal/config"
	"github.com/lukebrown14-code/rolecall/internal/doctor"
	"github.com/lukebrown14-code/rolecall/internal/process"
	"github.com/lukebrown14-code/rolecall/internal/store"
	"github.com/lukebrown14-code/rolecall/internal/ui"
	"os"
	"path/filepath"
	"time"
)

// watchDebounce groups bursts of file events into one rescan.
const watchDebounce = 200 * time.Millisecond

const usageErrorExitCode = 2

// version is the build version, overridden at release time with
// -ldflags "-X main.version=...". Development builds report the placeholder.
var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "init" {
		if err := initCommand(os.Args[2:], os.Stdout, os.Stderr); err != nil {
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
	showVersion := flag.Bool("version", false, "Print the version and exit")
	dir := flag.String("dir", "", "Workspace path")
	flag.Parse()
	if *showVersion {
		fmt.Printf("rolecall %s\n", version)
		return
	}
	if err := run(*dir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// initWorkspaceFiles writes the starter Workspace and its private local git repository.
func initWorkspaceFiles(root string) error {
	if err := store.InitializeWorkspace(root, nil, nil, store.SearchArea{}); err != nil {
		return err
	}
	return process.InitGit(root)
}

// noWorkspaceError tells someone who has installed rolecall but not run init
// what to do next, instead of only naming the empty config file.
func noWorkspaceError(home string) error {
	return fmt.Errorf("No Workspace yet. Run `rolecall init` to create one, or set workspace in %s", config.Path(home))
}

func runDoctor(dir string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	// A Workspace problem does not end the diagnosis: the tool, Plugin and Editor
	// checks below still help someone setting up. doctor fails with it at the end.
	var workspaceErr error
	c, err := config.Load(home, dir)
	if errors.Is(err, config.ErrNoWorkspace) {
		workspaceErr = noWorkspaceError(home)
		c.Workspace = ""
	} else if err != nil {
		return err
	} else if s, err := store.Open(c.Workspace); err != nil {
		workspaceErr = fmt.Errorf("open Workspace: %w", err)
		c.Workspace = ""
	} else {
		warnings, err := doctor.Check(c.Workspace, doctor.GitHubVisibilityLookup)
		if err != nil {
			return fmt.Errorf("check Workspace permissions: %w", err)
		}
		fmt.Printf("OK: Workspace found (%s)\n", c.Workspace)
		if len(warnings) == 0 {
			fmt.Println("Workspace permissions are private.")
		} else {
			for _, warning := range warnings {
				fmt.Printf("WARNING: %s\n", warning)
			}
		}
		printLegacyWarnings(s.LegacyFiles(), c.Workspace, c.Plugin)
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
	return workspaceErr
}

// printLegacyWarnings names each dashboard-era file in the Workspace. Nothing reads them:
// roles written there never become Leads. The fix is the one-time migration, then
// pointing the Harness at the current Plugin.
func printLegacyWarnings(files []string, workspace, plugin string) {
	if len(files) == 0 {
		return
	}
	for _, f := range files {
		fmt.Printf("WARNING: %s is from an older rolecall and is not read; its roles are not Leads\n", filepath.ToSlash(f))
	}
	script := "<plugin>/scripts/migrate.py"
	if plugin != "" {
		script = ui.QuoteShellArgument(filepath.Join(plugin, "scripts", "migrate.py"))
	}
	fmt.Printf("  Convert them to Lead folders (close rolecall first; --dry-run writes nothing):\n")
	fmt.Printf("    python3 %s --data %s --dry-run\n", script, ui.QuoteShellArgument(workspace))
	fmt.Println("  Then update your Harness to the current Plugin, which saves each Lead as a folder.")
}

func run(dir string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	c, err := config.Load(home, dir)
	if errors.Is(err, config.ErrNoWorkspace) {
		return noWorkspaceError(home)
	}
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
	m, err := ui.New(s, nil, ui.WithEditor(c.Editor), ui.WithEditorSource(c.EditorSource), ui.WithFollowUpDays(c.FollowUpDays), ui.WithDownloads(c.Downloads), ui.WithPlugin(c.Plugin), ui.WithHarness(c.Harness), ui.WithConfigHome(home), ui.WithSettingSources(ui.SettingSources{Workspace: c.WorkspaceSource, FollowUpDays: c.FollowUpDaysSource, Downloads: c.DownloadsSource, Plugin: c.PluginSource, Harness: c.HarnessSource}), ui.WithASCII(c.ASCII), ui.WithInheritTerminal(c.InheritTerminal), ui.DetectTheme(os.Stdin, os.Stdout), ui.WithWatch(watchDebounce), ui.WithFirstRunSetup())
	if err != nil {
		return err
	}
	_, err = tea.NewProgram(m).Run()
	return err
}
