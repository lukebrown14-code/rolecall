package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitRequiresNameAndEmail(t *testing.T) {
	for _, tc := range []struct{ field string }{
		{"Name"},
		{"Email"},
	} {
		t.Run(tc.field, func(t *testing.T) {
			workspace := filepath.Join(t.TempDir(), "workspace")
			args := []string{"--workspace", workspace}
			if tc.field == "Name" {
				args = append(args, "--name=", "--email=alex@example.test")
			} else {
				args = append(args, "--name=Alex", "--email=")
			}
			args = append(args, "--profile=later")
			err := initWizard(args, strings.NewReader(""), io.Discard, io.Discard, nil)
			if err == nil || !strings.Contains(err.Error(), "Name and Email are required") {
				t.Fatalf("init error = %v, want missing required Contact field refusal", err)
			}
		})
	}
}

// A huh form on a stdin that is not a terminal never reads a key: it hangs forever.
// init must refuse with a way forward instead.
func TestInitRefusesPromptsWithoutATerminal(t *testing.T) {
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer devNull.Close()
	workspace := filepath.Join(t.TempDir(), "workspace")
	err = initWizard([]string{"--workspace", workspace, "--profile", "later"}, devNull, io.Discard, io.Discard, nil)
	if err == nil || !strings.Contains(err.Error(), "needs a terminal") {
		t.Fatalf("init error = %v, want a clear non-terminal refusal", err)
	}
}

// init writes the workspace setting through the same patcher the Settings screen uses,
// so a config with a table keeps the key above it instead of appending it inside.
func TestInitWritesWorkspaceSettingBeforeTables(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfg := filepath.Join(home, ".config", "rolecall", "config.toml")
	if err := os.MkdirAll(filepath.Dir(cfg), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg, []byte("[harness]\ncommand = \"x\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	workspace := filepath.Join(t.TempDir(), "workspace")
	args := []string{
		"--workspace", workspace, "--name", "Alex", "--email", "alex@example.test",
		"--phone", "", "--location", "", "--linkedin", "",
		"--tracks", "", "--search-places", "", "--search-remote", "",
		"--profile", "later",
	}
	if err := initWizard(args, strings.NewReader(""), io.Discard, io.Discard, nil); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	key, table := strings.Index(string(got), "workspace = "), strings.Index(string(got), "[harness]")
	if key < 0 || table < 0 || key > table {
		t.Fatalf("workspace key not written above the table:\n%s", got)
	}
	if !strings.Contains(string(got), "workspace = \""+workspace+"\"") {
		t.Fatalf("workspace value missing:\n%s", got)
	}
}

func TestProfileSetupRoutesToHarnessOrPrintsCommand(t *testing.T) {
	home, workspace := t.TempDir(), "/tmp/Owner's $workspace"
	if err := os.MkdirAll(filepath.Join(home, ".config/rolecall"), 0700); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(home, ".config/rolecall/config.toml")

	t.Run("later prints command", func(t *testing.T) {
		var out bytes.Buffer
		if err := finishProfile(initOptions{profile: "later"}, workspace, home, &out, nil); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), `create-profile --data '/tmp/Owner'\''s $workspace'`) {
			t.Fatalf("output = %q", out.String())
		}
	})
	t.Run("unconfigured Harness prints command", func(t *testing.T) {
		if err := os.WriteFile(configPath, []byte(""), 0600); err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		if err := finishProfile(initOptions{profile: "questions"}, workspace, home, &out, nil); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), `create-profile --data '/tmp/Owner'\''s $workspace'`) {
			t.Fatalf("output = %q", out.String())
		}
	})
	t.Run("unconfigured resume command quotes both paths", func(t *testing.T) {
		if err := os.WriteFile(configPath, []byte(""), 0600); err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		resume := "/tmp/Owner's $resume.pdf"
		if err := finishProfile(initOptions{profile: profileResume, resume: resume}, workspace, home, &out, nil); err != nil {
			t.Fatal(err)
		}
		want := `create-profile --data '/tmp/Owner'\''s $workspace' --resume '/tmp/Owner'\''s $resume.pdf'`
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output = %q, want command %q", out.String(), want)
		}
	})
	for _, tc := range []struct{ name, profile, resume string }{{"questions", "questions", ""}, {"resume", "resume", "/tmp/current resume.pdf"}} {
		t.Run(tc.name+" launches create-profile", func(t *testing.T) {
			if err := os.WriteFile(configPath, []byte("harness = 'agent \"/{skill} {dir}\"'\n"), 0600); err != nil {
				t.Fatal(err)
			}
			var gotHarness, gotWorkspace, gotResume string
			var out bytes.Buffer
			err := finishProfile(initOptions{profile: tc.profile, resume: tc.resume}, workspace, home, &out, func(harness, root, resume string) error {
				gotHarness, gotWorkspace, gotResume = harness, root, resume
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if gotHarness == "" || gotWorkspace != workspace || gotResume != tc.resume || out.Len() != 0 {
				t.Fatalf("handoff=(%q,%q,%q), output=%q", gotHarness, gotWorkspace, gotResume, out.String())
			}
		})
	}
}

// The interactive form needs the terminal itself so Bubble Tea can enter raw mode;
// only accessible prompts get the one-line-per-read wrapper.
func TestPromptInputKeepsTheTerminalForInteractiveForms(t *testing.T) {
	if got := promptInput(os.Stdin, false); got != io.Reader(os.Stdin) {
		t.Fatalf("interactive prompt input = %T, want the terminal file itself", got)
	}
	if _, ok := promptInput(os.Stdin, true).(*promptLineReader); !ok {
		t.Fatal("accessible prompt input is not limited to one line per read")
	}
}
