package store_test

import (
	"github.com/lukebrown14-code/rolecall/store"
	"os"
	"path/filepath"
	"testing"
)

func TestPatchPerKeyGoldens(t *testing.T) {
	values := map[string]string{"schema": "1", "title": "Senior Engineer", "company": "Other Co", "track": "Other", "url": "https://example.org/new", "closes": "2027-01-01", "fit": "Poor", "resume_only": "true", "legacy": "updated"}
	for key, value := range values {
		t.Run(key, func(t *testing.T) {
			root := t.TempDir()
			if err := os.CopyFS(root, os.DirFS("../fixtures/workspace")); err != nil {
				t.Fatal(err)
			}
			folder := filepath.Join(root, "jobs/SWE/orbit")
			input, err := os.ReadFile("testdata/patch/input.yml")
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(folder, "meta.yml")
			if err = os.WriteFile(path, input, 0640); err != nil {
				t.Fatal(err)
			}
			if err = os.Chmod(path, 0640); err != nil {
				t.Fatal(err)
			}
			s, _ := store.Open(root)
			if err = s.Patch(folder, map[string]string{key: value}); err != nil {
				t.Fatal(err)
			}
			want, err := os.ReadFile("testdata/patch/" + key + ".golden.yml")
			if err != nil {
				t.Fatal(err)
			}
			got, _ := os.ReadFile(path)
			if string(got) != string(want) {
				t.Fatalf("got:\n%s\nwant:\n%s", got, want)
			}
			info, _ := os.Stat(path)
			if info.Mode().Perm() != 0640 {
				t.Fatal("permissions changed")
			}
		})
	}
}
