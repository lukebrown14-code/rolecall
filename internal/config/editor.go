package config

import "os"

// ResolveEditor returns the Editor command in effect: the configured one, then
// $VISUAL, $EDITOR, then vi.
func ResolveEditor(configured string) string {
	for _, s := range []string{configured, os.Getenv("VISUAL"), os.Getenv("EDITOR")} {
		if s != "" {
			return s
		}
	}
	return "vi"
}

// CommandName returns the executable of a command line: its first word, with a
// leading single- or double-quoted path unquoted. Flags are ignored.
func CommandName(command string) string {
	i := 0
	for i < len(command) && (command[i] == ' ' || command[i] == '\t') {
		i++
	}
	command = command[i:]
	if command != "" && (command[0] == '"' || command[0] == '\'') {
		for j := 1; j < len(command); j++ {
			if command[j] == command[0] {
				return command[1:j]
			}
		}
		return command[1:]
	}
	for j := 0; j < len(command); j++ {
		if command[j] == ' ' || command[j] == '\t' {
			return command[:j]
		}
	}
	return command
}
