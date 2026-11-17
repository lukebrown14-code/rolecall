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
	"strings"
	"time"
)

// watchDebounce groups bursts of file events into one rescan.
const watchDebounce = 200 * time.Millisecond

const usageErrorExitCode = 2

// version is the build version, overridden at release time with
// -ldflags "-X main.version=...". Development builds report the placeholder.
var version = "dev"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "init":
			exitOn(initCommand(os.Args[2:], os.Stdout, os.Stderr))
			return
		case "move":
			exitOn(moveCommand(os.Args[2:], os.Stdout, os.Stderr))
			return
		case "now":
			exitOn(nowCommand(os.Args[2:], os.Stdout, os.Stderr, time.Now))
			return
		case "export":
			exitOn(exportCommand(os.Args[2:], os.Stdout, os.Stderr, time.Now))
			return
		}
	}
	if len(os.Args) > 1 && os.Args[1] == "doctor" {
		flags := flag.NewFlagSet("doctor", flag.ContinueOnError)
		dir := flags.String("dir", "", "Workspace path (default: workspace in config.toml)")
		flags.Usage = func() {
			_, _ = fmt.Fprintln(flags.Output(), "usage: rolecall doctor [--dir PATH]\n\nChecks the Workspace, its privacy, and the tools rolecall uses.")
			flags.PrintDefaults()
		}
		if err := flags.Parse(os.Args[2:]); errors.Is(err, flag.ErrHelp) {
			return
		} else if err != nil {
			os.Exit(usageErrorExitCode)
		}
		if err := runDoctor(*dir); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	flag.Usage = usage
	showVersion := flag.Bool("version", false, "Print the version and exit")
	dir := flag.String("dir", "", "Workspace path (default: workspace in config.toml)")
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

// exitOn ends a subcommand: quietly for --help, with the message and status 1 for an error.
func exitOn(err error) {
	if err == nil || errors.Is(err, flag.ErrHelp) {
		return
	}
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

// usage is `rolecall --help`: the subcommands first, so someone who skipped the install
// script can find init, doctor and move from the binary, then the TUI's own flags.
func usage() {
	out := flag.CommandLine.Output()
	_, _ = fmt.Fprint(out, `usage: rolecall [--dir PATH]    open the Workspace in the terminal app
       rolecall init [--switch] [DIR]
                                 create a Workspace (default: ./rolecall); --switch
                                 makes DIR the default when config.toml names another
       rolecall doctor [--dir PATH]
                                 check the Workspace and the tools rolecall uses
       rolecall move [--dir PATH] FOLDER STATUS
                                 move an Application to a Status as the app does;
                                 Applied also saves sent/
       rolecall now [--dir PATH] [--days N] [--list]
                                 one plain line of what wants doing today: follow-ups
                                 due, next steps and close dates within N days (7)
       rolecall export [--dir PATH] [--format csv|json | --ics] [--out FILE]
                                 every Application as CSV or JSON, or next steps and
                                 close dates as an .ics; stdout unless --out names a file

Flags:
`)
	flag.PrintDefaults()
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

// brokenConfigError is a config.toml that does not parse, as `rolecall` and `rolecall
// doctor` report it: the file, where, why, and what to do.
func brokenConfigError(e *config.FileError) error {
	where := ""
	if w := e.Where(); w != "" {
		where = " (" + w + ")"
	}
	return fmt.Errorf("%s has an error%s: %s\n"+
		"  Fix that line in your editor, then run `rolecall doctor` to check it.\n"+
		"  To open a Workspace meanwhile: rolecall --dir PATH", e.Path, where, e.Reason)
}

// printConfigWarnings names each config.toml value Load could not use as written.
func printConfigWarnings(path string, warnings []config.Warning) {
	for _, w := range warnings {
		fmt.Printf("WARNING: %s: %s\n", path, w.Text)
	}
}

func runDoctor(dir string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	// A Workspace problem does not end the diagnosis: the tool, Plugin and Editor
	// checks below still help someone setting up. doctor fails with it at the end.
	var workspaceErr, configProblem error
	c, err := config.Load(home, dir)
	var configErr *config.FileError
	if errors.As(err, &configErr) {
		// A broken config.toml is reported, and the checks go on with the defaults (and
		// --dir, when given) so the rest of the setup is still diagnosed.
		fmt.Printf("PROBLEM: %v\n", brokenConfigError(configErr))
		configProblem = errors.New("config.toml is invalid")
		if c.Workspace != "" {
			err = nil
		}
	}
	printConfigWarnings(config.Path(home), c.Warnings)
	if configErr != nil && err != nil {
		c.Workspace = "" // the Workspace path is in the broken file
	} else if errors.Is(err, config.ErrNoWorkspace) {
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
		if apps, err := s.Scan(); err == nil {
			printRepeatedStatusWarnings(apps, c.Workspace)
		}
		workspaceErr = checkWorkspaceFile(s)
	}

	report := doctor.CheckSetup(doctor.Setup{Workspace: c.Workspace, Plugin: c.Plugin, Harness: c.Harness, Editor: c.Editor, ConfigPath: config.Path(home)}, nil)
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
	for _, ignored := range report.Ignored {
		fmt.Printf("WARNING: %s\n", ignored)
	}
	fmt.Printf("Template in use: %s. See README → Make your documents yours.\n", report.Template)
	if strings.HasPrefix(report.Template, "Workspace override") {
		fmt.Println("  Plugin updates to an overridden file do not reach your copy; compare it with the Plugin's after updating.")
	}
	return errors.Join(configProblem, workspaceErr)
}

// checkWorkspaceFile reports a workspace.json the app cannot use. The app still opens
// with it, but every Workspace setting is ignored until it is fixed, so doctor fails.
func checkWorkspaceFile(s *store.Store) error {
	ws, err := s.Settings()
	if err != nil {
		return fmt.Errorf("read %s: %w", ws.Path, err)
	}
	if ws.Err == nil {
		// The app also refuses a tracks list that breaks the Track name rules (one listed
		// twice, or blank), so doctor applies the same check.
		var tle *store.TrackListError
		if _, err := s.Tracks(); errors.As(err, &tle) {
			ws.Err = fmt.Errorf("tracks: %w", tle.Err)
		}
	}
	if ws.Err == nil {
		return nil
	}
	fmt.Printf("PROBLEM: %s cannot be used: %v\n", ws.Path, ws.Err)
	fmt.Println("  Fix it in your editor, or open rolecall and press 6 then e. Until then no Workspace settings apply.")
	return errors.New("workspace.json is invalid")
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

// printRepeatedStatusWarnings names each meta.yml whose status_log holds the same entry
// (Status and time) more than once. The app reads the last one; the repeat is noise
// from a hand edit or an older migration, safe to delete.
func printRepeatedStatusWarnings(apps []store.Application, workspace string) {
	for _, a := range apps {
		for _, e := range a.RepeatedStatusEntries() {
			rel, err := filepath.Rel(workspace, filepath.Join(a.Folder, "meta.yml"))
			if err != nil {
				rel = filepath.Join(a.Folder, "meta.yml")
			}
			fmt.Printf("WARNING: %s lists {status: %s, at: %s} more than once; delete the earlier copy (the app reads the last)\n", filepath.ToSlash(rel), e.Status, e.At)
		}
	}
}

func run(dir string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	c, err := config.Load(home, dir)
	var configErr *config.FileError
	var problem error // config.toml's error, shown in the app when it can still open
	switch {
	case errors.As(err, &configErr) && c.Workspace == "":
		// The Workspace path is in the file that does not parse, so there is nothing to open.
		return brokenConfigError(configErr)
	case errors.As(err, &configErr):
		// --dir names the Workspace: open it on defaults and show the error on the feedback
		// line and in Settings, as a bad reload inside the app does.
		problem = configErr
	case errors.Is(err, config.ErrNoWorkspace):
		return noWorkspaceError(home)
	case err != nil:
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
	m, err := ui.New(s, nil, ui.WithEditor(c.Editor), ui.WithEditorSource(c.EditorSource), ui.WithFollowUpDays(c.FollowUpDays), ui.WithDownloads(c.Downloads), ui.WithPlugin(c.Plugin), ui.WithHarness(c.Harness), ui.WithConfigHome(home), ui.WithSettingSources(ui.SettingSources{Workspace: c.WorkspaceSource, FollowUpDays: c.FollowUpDaysSource, Downloads: c.DownloadsSource, Plugin: c.PluginSource, Harness: c.HarnessSource}), ui.WithASCII(c.ASCII), ui.WithInheritTerminal(c.InheritTerminal), ui.WithCompactRows(c.CompactRows), ui.DetectTheme(os.Stdin, os.Stdout), ui.WithWatch(watchDebounce), ui.WithFirstRunSetup(), ui.WithConfigProblem(problem, c.Warnings))
	if err != nil {
		return err
	}
	_, err = tea.NewProgram(m).Run()
	return err
}
