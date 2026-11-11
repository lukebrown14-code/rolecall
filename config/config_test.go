package config_test

import (
	"github.com/lukebrown14-code/rolecall/config"
	"os"
	"path/filepath"
	"testing"
)

func TestLoader(t *testing.T) {
	home := t.TempDir()
	c, err := config.Load(home, "")
	if err == nil || err.Error() != "no Workspace configured in "+filepath.Join(home, ".config/rolecall/config.toml") {
		t.Fatalf("missing: %v", err)
	}
	p := filepath.Join(home, ".config/rolecall")
	os.MkdirAll(p, 0755)
	os.WriteFile(filepath.Join(p, "config.toml"), []byte("workspace = \"~/work\"\neditor = \"vim\"\ndownloads = \"~/Downloads\"\n"), 0644)
	c, err = config.Load(home, "")
	if err != nil || c.Workspace != filepath.Join(home, "work") || c.Downloads != filepath.Join(home, "Downloads") || c.Editor != "vim" || c.FollowUpDays != 14 {
		t.Fatalf("%+v %v", c, err)
	}
	c, err = config.Load(home, "~/other")
	if err != nil || c.Workspace != filepath.Join(home, "other") {
		t.Fatalf("%+v %v", c, err)
	}
	os.WriteFile(filepath.Join(p, "config.toml"), []byte("follow_up_days = 21\n"), 0644)
	c, err = config.Load(home, "/override")
	if err != nil || c.FollowUpDays != 21 || c.Workspace != "/override" {
		t.Fatalf("%+v %v", c, err)
	}
}

func TestTOMLLiteralAndComments(t *testing.T) {
	home := t.TempDir()
	p := filepath.Join(home, ".config/rolecall")
	os.MkdirAll(p, 0700)
	os.WriteFile(filepath.Join(p, "config.toml"), []byte("workspace = '~/work' # workspace\neditor = 'vim # literal' # editor\nfollow_up_days = 21 # days\ndownloads = \"~/Downloads\" # downloads\n"), 0600)
	for _, dir := range []string{"", "~/override"} {
		c, err := config.Load(home, dir)
		want := filepath.Join(home, "work")
		if dir != "" {
			want = filepath.Join(home, "override")
		}
		if err != nil || c.Workspace != want || c.FollowUpDays != 21 || c.Editor != "vim # literal" || c.Downloads != filepath.Join(home, "Downloads") {
			t.Fatalf("%+v %v", c, err)
		}
	}
}
