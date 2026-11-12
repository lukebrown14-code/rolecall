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

func TestWriteWorkspaceConfigKeepsOtherSettingsAndComment(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	before := "workspace = '~/old' # existing workspace\neditor = 'vim' # keep editor\n"
	if err := os.WriteFile(path, []byte(before), 0640); err != nil {
		t.Fatal(err)
	}
	if err := writeWorkspaceConfig(path, "/tmp/new workspace"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "workspace = \"/tmp/new workspace\" # existing workspace\neditor = 'vim' # keep editor\n"
	if string(got) != want {
		t.Fatalf("config = %q, want %q", got, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("config permissions = %04o, want 0600", info.Mode().Perm())
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
