package store_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lukebrown14-code/rolecall/internal/store"
)

// TestBuildProducesReadyApplication drives the real build.py end to end against
// a temporary Workspace. It needs pandoc and typst (and python3); the regular CI
// job installs only Go and Python, so it skips there. Install the tools locally
// to run it: brew install pandoc typst poppler.
func TestBuildProducesReadyApplication(t *testing.T) {
	tools := []string{"python3", "pandoc", "typst"}
	for _, tool := range tools {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("skipping build integration test: %s not installed (needs python3, pandoc and typst)", tool)
		}
	}

	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../../../fixtures/workspace")); err != nil {
		t.Fatal(err)
	}
	profile := filepath.Join(root, "profile")
	if err := os.MkdirAll(profile, 0o700); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(profile, "master-profile.md"), contactProfile)
	folder := filepath.Join(root, "jobs", "SWE", "acme-developer")
	if err := os.MkdirAll(folder, 0o700); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(folder, "meta.yml"), buildMeta)
	mustWrite(t, filepath.Join(folder, "jd.md"), "Acme needs a developer to own Python services.\n")
	mustWrite(t, filepath.Join(folder, "resume.md"), buildResume)
	mustWrite(t, filepath.Join(folder, "cover-letter.md"), buildLetter)

	script, err := filepath.Abs("../../../plugin/scripts/build.py")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("python3", script, folder, "--data", root)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build.py failed: %v\n%s", err, output)
	}

	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	apps, err := s.Scan()
	if err != nil {
		t.Fatal(err)
	}
	var built *store.Application
	for i := range apps {
		if apps[i].Folder == folder {
			built = &apps[i]
			break
		}
	}
	if built == nil {
		t.Fatalf("built Application not found under %s", root)
	}
	if built.Malformed != nil {
		t.Fatalf("meta.yml became malformed: %v", built.Malformed)
	}
	if built.Documents != "Ready" {
		t.Fatalf("Documents = %q, want Ready (PDFs: %v)", built.Documents, built.PDFs)
	}
	if built.Status != "Draft" {
		t.Fatalf("Status = %q, want Draft", built.Status)
	}
	drafts := 0
	for _, entry := range built.StatusLog {
		if entry.Status == "Draft" {
			drafts++
		}
	}
	if drafts != 1 {
		t.Fatalf("Draft entries = %d, want 1 (log: %+v)", drafts, built.StatusLog)
	}
	if !strings.Contains(strings.Join(built.PDFs, "|"), "Alex Quinn Resume Acme Developer.pdf") {
		t.Fatalf("resume PDF missing: %v", built.PDFs)
	}
}

func mustWrite(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

const buildMeta = `schema: 1
title: Developer
company: Acme
track: SWE
status_log:
  - {status: Lead, at: 2026-09-24T01:00:00Z}
`

const contactProfile = `# Master Profile — Alex Quinn

## Contact

| Field | Value |
| :-- | :-- |
| Name | Alex Quinn |
| Email | alex.quinn@example.test |
| Phone | +61 400 000 000 |
| Location | Brisbane, QLD |
| Work rights | Full working rights in Australia |

## Headlines (approved per track)

| Track | Headline |
| :-- | :-- |
| SWE | Full Stack Software Engineer |

## Experience

### Acme — Brisbane, AU

**Dates:** Ongoing
**Canonical title:** Developer

- Built services in Python with Docker, SQL, Git, REST, CI/CD, Testing and Linux.

## Key Skills keyword pool

Python, Docker, SQL, Git, REST, CI/CD, Testing, Linux

## Facts that are NOT available

- No Kubernetes production experience
`

const buildResume = `# Alex Quinn

## Professional Summary

Backend developer who ships Python services.

## Key Skills

Python, Docker, SQL, Git, REST, CI/CD, Testing, Linux

## Professional Experience

### Developer — Acme | Ongoing

- Built services in Python with Docker, SQL, Git and REST, owning CI/CD and Testing on Linux.
`

const buildLetter = `# Alex Quinn

Acme — Developer

Dear Acme team,

Acme needs a developer to own Python services, and that is the shape of my recent work.

One practical note: I hold full working rights in Australia.

Kind regards,
Alex Quinn
`
