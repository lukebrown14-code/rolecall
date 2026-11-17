package ui

import (
	"reflect"
	"testing"
)

func TestDesktopCommand(t *testing.T) {
	for _, tc := range []struct {
		platform string
		reveal   bool
		command  string
		args     []string
	}{
		{"darwin", false, "open", []string{"/tmp/Owner Resume.pdf"}},
		{"darwin", true, "open", []string{"-R", "/tmp/Owner Resume.pdf"}},
		{"linux", false, "xdg-open", []string{"/tmp/Owner Resume.pdf"}},
		{"linux", true, "xdg-open", []string{"/tmp/Owner Resume.pdf"}},
		{"windows", false, "rundll32.exe", []string{"url.dll,FileProtocolHandler", "/tmp/Owner Resume.pdf"}},
		{"windows", true, "rundll32.exe", []string{"url.dll,FileProtocolHandler", "/tmp/Owner Resume.pdf"}},
	} {
		t.Run(tc.platform+map[bool]string{true: "/reveal", false: "/open"}[tc.reveal], func(t *testing.T) {
			command, args := desktopCommand(tc.platform, "/tmp/Owner Resume.pdf", tc.reveal)
			if command != tc.command || !reflect.DeepEqual(args, tc.args) {
				t.Fatalf("got %s %q; want %s %q", command, args, tc.command, tc.args)
			}
		})
	}
}

func TestWSLDesktopCommand(t *testing.T) {
	for _, tc := range []struct {
		name         string
		path         string
		wslview, xdg bool
		command      string
		args         []string
	}{
		{"wslview wins", "/tmp/a.pdf", true, true, "wslview", []string{"/tmp/a.pdf"}},
		{"xdg-open when no wslview", "/tmp/a.pdf", false, true, "xdg-open", []string{"/tmp/a.pdf"}},
		{"explorer with wslpath for a file", "/tmp/a b.pdf", false, false, "/bin/sh",
			[]string{"-c", `explorer.exe "$(wslpath -w "$1")"; [ $? -le 1 ]`, "rolecall-open", "/tmp/a b.pdf"}},
		{"explorer takes a URL as is", "https://example.com/x", false, false, "/bin/sh",
			[]string{"-c", `explorer.exe "$1"; [ $? -le 1 ]`, "rolecall-open", "https://example.com/x"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			command, args := wslDesktopCommand(tc.path, tc.wslview, tc.xdg)
			if command != tc.command || !reflect.DeepEqual(args, tc.args) {
				t.Fatalf("got %s %q; want %s %q", command, args, tc.command, tc.args)
			}
		})
	}
}
