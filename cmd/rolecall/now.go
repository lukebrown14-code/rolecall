package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lukebrown14-code/rolecall/internal/store"
	"github.com/lukebrown14-code/rolecall/internal/ui"
)

// nowCommand is `rolecall now [--dir PATH] [--days N] [--list]`: the Stats "Now" panel as
// text a script can read. It prints one line, plain ASCII with no colour, of what wants
// doing today ("2 follow-ups due | 1 next step within 7 days | 3 closing within 7 days"),
// or nothing when nothing does, and exits 0 either way. It never writes to the Workspace.
func nowCommand(args []string, stdout, stderr io.Writer, clock func() time.Time) error {
	flags := flag.NewFlagSet("now", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dir := flags.String("dir", "", "Workspace path (default: workspace in config.toml)")
	days := flags.Int("days", ui.SoonDays, "how many days ahead a next step or close date counts")
	list := flags.Bool("list", false, "one tab-separated line per item: kind, folder, when, title, company")
	flags.Usage = func() {
		_, _ = fmt.Fprintln(flags.Output(), "usage: rolecall now [--dir PATH] [--days N] [--list]\n\n"+
			"Prints one plain line of what wants doing today: Applications Follow-up due, next\n"+
			"steps and Lead or Draft close dates within N days. Prints nothing when nothing is\n"+
			"due, and exits 0. For a tmux status line, a shell prompt or cron.")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *days < 0 {
		flags.Usage()
		return fmt.Errorf("now takes no arguments and a --days of 0 or more")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	s, cfg, err := openWorkspaceConfig(home, *dir)
	if err != nil {
		return err
	}
	apps, err := s.Scan()
	if err != nil {
		return err
	}
	n := ui.NowFor(apps, clock(), cfg.FollowUpDays, *days)
	if *list {
		return printNowList(stdout, s.Root(), n)
	}
	var parts []string
	count := func(n int, one, many string) string {
		if n == 1 {
			return fmt.Sprintf("1 %s", one)
		}
		return fmt.Sprintf("%d %s", n, many)
	}
	if k := len(n.FollowUps); k > 0 {
		parts = append(parts, count(k, "follow-up due", "follow-ups due"))
	}
	if k := len(n.NextSteps); k > 0 {
		parts = append(parts, fmt.Sprintf("%s within %d days", count(k, "next step", "next steps"), *days))
	}
	if k := len(n.Closing); k > 0 {
		parts = append(parts, fmt.Sprintf("%d closing within %d days", k, *days))
	}
	if len(parts) == 0 {
		return nil
	}
	_, err = fmt.Fprintln(stdout, strings.Join(parts, " | "))
	return err
}

// printNowList is --list: each item on a line of tab-separated fields, so a script can
// act on one Application.
func printNowList(out io.Writer, root string, n ui.Now) error {
	line := func(kind string, a store.Application, when string) error {
		rel, err := filepath.Rel(root, a.Folder)
		if err != nil {
			rel = a.Folder
		}
		clean := func(s string) string { return strings.NewReplacer("\t", " ", "\n", " ", "\r", " ").Replace(s) }
		_, err = fmt.Fprintf(out, "%s\t%s\t%s\t%s\t%s\n", kind, filepath.ToSlash(rel), when, clean(a.Title), clean(a.Company))
		return err
	}
	for _, a := range n.FollowUps {
		if err := line("follow-up", a, ""); err != nil {
			return err
		}
	}
	for _, a := range n.NextSteps {
		if err := line("next-step", a, a.NextStep.At); err != nil {
			return err
		}
	}
	for _, a := range n.Closing {
		if err := line("closes", a, a.Closes); err != nil {
			return err
		}
	}
	return nil
}
