package main

import (
	tea "charm.land/bubbletea/v2"
	"flag"
	"fmt"
	"github.com/lukebrown14-code/rolecall/config"
	"github.com/lukebrown14-code/rolecall/store"
	"github.com/lukebrown14-code/rolecall/ui"
	"os"
	"time"
)

func main() {
	dir := flag.String("dir", "", "Workspace path")
	flag.Parse()
	if err := run(*dir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
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
	if err != nil {
		return err
	}
	m, err := ui.New(s, nil, ui.WithEditor(c.Editor), ui.WithFollowUpDays(c.FollowUpDays), ui.WithDownloads(c.Downloads), ui.WithWatch(200*time.Millisecond))
	if err != nil {
		return err
	}
	_, err = tea.NewProgram(m).Run()
	return err
}
