package doctor

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckReportsLooseWorkspaceFilesAndFolders(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "Applications")
	private := filepath.Join(nested, "Example")
	if err := os.MkdirAll(private, 0o700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(private, "profile.md")
	if err := os.WriteFile(file, []byte("fictional"), 0o600); err != nil {
		t.Fatal(err)
	}
	for path, mode := range map[string]os.FileMode{root: 0o755, nested: 0o755, file: 0o644} {
		if err := os.Chmod(path, mode); err != nil {
			t.Fatal(err)
		}
	}

	warnings, err := Check(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(warnings, "\n")
	for _, want := range []string{
		root + " (directory): permissions 0755; fix with chmod 700 " + root,
		nested + " (directory): permissions 0755; fix with chmod 700 " + nested,
		file + " (file): permissions 0644; fix with chmod 600 " + file,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("warnings missing %q:\n%s", want, got)
		}
	}
}

func TestCheckReportsLooseFilesInsideWorkspace(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	private := filepath.Join(root, "private")
	if err := os.Mkdir(private, 0o700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(private, "profile.md")
	if err := os.WriteFile(file, []byte("fictional"), 0o644); err != nil {
		t.Fatal(err)
	}

	warnings, err := Check(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], file+" (file)") || !strings.Contains(warnings[0], "chmod 600 "+file) {
		t.Fatalf("warnings = %v, want file warning with chmod fix for %s", warnings, file)
	}
}

// A Workspace that is a git repository reports no warnings about .git: git manages its
// own file modes, and the walk must not descend into hidden directories.
func TestCheckIgnoresGitDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "init", root).Run(); err != nil {
		t.Skipf("git unavailable: %v", err)
	}
	warnings, err := Check(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Fatalf("fresh git Workspace warnings = %v, want none", warnings)
	}
}

func TestCheckReportsAllClearForPrivateWorkspace(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "private.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	warnings, err := Check(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Fatalf("private Workspace warnings = %v, want none", warnings)
	}
}

// A copied Workspace trips every file: one summary line carries the one command, and the
// other warnings keep their place.
func TestCollapsePermissionsFoldsPerPathWarnings(t *testing.T) {
	in := []string{
		"/w/a (file): permissions 0644; fix with chmod 600 /w/a",
		"/w/b (file): permissions 0644; fix with chmod 600 /w/b",
		"/w/c (directory): permissions 0755; fix with chmod 700 /w/c",
		"git remote origin is public",
	}
	got := CollapsePermissions(in, "/w")
	if len(got) != 2 || !strings.Contains(got[0], "3 files and folders") || !strings.Contains(got[0], "chmod -R go-rwx /w") || got[1] != in[3] {
		t.Fatalf("collapsed = %q", got)
	}
	// One path keeps its own, more precise line.
	if one := CollapsePermissions(in[:1], "/w"); len(one) != 1 || one[0] != in[0] {
		t.Fatalf("single warning changed: %q", one)
	}
}
