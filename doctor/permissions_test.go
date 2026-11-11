package doctor

import (
	"os"
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
