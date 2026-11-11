package main

import (
	"flag"
	"fmt"
	"github.com/lukebrown14-code/rolecall/config"
	"github.com/lukebrown14-code/rolecall/store"
	"os"
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
	apps, err := s.Scan()
	if err != nil {
		return err
	}
	counts := map[string]int{}
	for _, a := range apps {
		counts[a.Status]++
	}
	for _, status := range []string{"Lead", "Draft", "Applied", "Interviewing", "Offer", "Rejected", "Withdrawn", "Dismissed"} {
		fmt.Printf("%s: %d\n", status, counts[status])
	}
	return nil
}
