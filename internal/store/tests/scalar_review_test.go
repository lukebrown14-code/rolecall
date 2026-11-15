package store_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lukebrown14-code/rolecall/internal/store"
)

func TestYAMLEscapes(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../../../fixtures/workspace")); err != nil {
		t.Fatal(err)
	}
	s, _ := store.Open(root)
	folder := filepath.Join(root, "jobs/SWE/orbit")
	text := "title: \"slash\\/ space\\  unicode\\u263A\\U0001F600\\N\\_\\L\\P\" # keep\ncompany: Acme\ntrack: SWE\nstatus_log: []\n"
	if err := os.WriteFile(filepath.Join(folder, "meta.yml"), []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	a, err := s.Rescan(folder)
	if err != nil || a.Malformed != nil || a.Title != store.DisplayText("slash/ space  unicode☺😀\u0085\u00a0\u2028\u2029") {
		t.Fatalf("%+v %v", a, err)
	}
	value := "control\x00\x1b\x7f\u0085\u2028 emoji😀"
	if err := s.Patch(folder, map[string]string{"title": value}); err != nil {
		t.Fatal(err)
	}
	// The file keeps every character, escaped; what the read hands the UI is cleaned.
	b, _ := os.ReadFile(filepath.Join(folder, "meta.yml"))
	if !strings.Contains(string(b), `"control\u0000\u001B\u007F\u0085\u2028 emoji😀"`) {
		t.Fatal(string(b))
	}
	a, err = s.Rescan(folder)
	if err != nil || a.Malformed != nil || a.Title != store.DisplayText(value) || a.Title != "control  emoji😀" {
		t.Fatalf("%+v %v", a, err)
	}
}
func TestMigrationCommentPreserved(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../../../fixtures/workspace")); err != nil {
		t.Fatal(err)
	}
	s, _ := store.Open(root)
	folder := filepath.Join(root, "jobs/SWE/orbit")
	path := filepath.Join(folder, "meta.yml")
	text := "title: Engineer\ncompany: Acme\ntrack: SWE\nstatus_log:\n  - {status: Applied, at: 2026-01-01, approx: true}   # migration only\n"
	os.WriteFile(path, []byte(text), 0600)
	a, _ := s.Rescan(folder)
	if a.Malformed != nil || a.AppliedDate == nil || !a.AppliedDate.Approx {
		t.Fatalf("%+v", a)
	}
	if err := s.AppendStatus(folder, "Offer", time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if !strings.HasPrefix(string(got), text) {
		t.Fatalf("%s", got)
	}
}

func TestYAMLRejectsGoOnlyEscapes(t *testing.T) {
	for _, value := range []string{`"\101"`, `"\'" `, `"\uD800"`, `"\U00110000"`, `"\x1"`} {
		t.Run(value, func(t *testing.T) {
			root := t.TempDir()
			if err := os.CopyFS(root, os.DirFS("../../../fixtures/workspace")); err != nil {
				t.Fatal(err)
			}
			folder := filepath.Join(root, "jobs/SWE/orbit")
			path := filepath.Join(folder, "meta.yml")
			text := "title: " + value + "\ncompany: Acme\ntrack: SWE\nstatus_log: []\n"
			if err := os.WriteFile(path, []byte(text), 0600); err != nil {
				t.Fatal(err)
			}
			s, _ := store.Open(root)
			a, err := s.Rescan(folder)
			if err != nil || a.Malformed == nil {
				t.Fatalf("%+v %v", a, err)
			}
			if err := s.Patch(folder, map[string]string{"fit": "Good"}); err == nil {
				t.Fatal("patched malformed YAML")
			}
			after, _ := os.ReadFile(path)
			if string(after) != text {
				t.Fatal("changed malformed YAML")
			}
		})
	}
}
func TestEmptyPlainScalar(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../../../fixtures/workspace")); err != nil {
		t.Fatal(err)
	}
	s, _ := store.Open(root)
	folder := filepath.Join(root, "jobs/SWE/orbit")
	if err := os.WriteFile(filepath.Join(folder, "meta.yml"), []byte("title: Engineer\ncompany: Acme\ntrack: SWE\nfit:    \nstatus_log: []\n"), 0600); err != nil {
		t.Fatal(err)
	}
	a, err := s.Rescan(folder)
	if err != nil || a.Malformed != nil || a.Fit != "" {
		t.Fatalf("%+v %v", a, err)
	}
}
