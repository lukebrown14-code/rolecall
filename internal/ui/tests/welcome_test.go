package ui_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lukebrown14-code/rolecall/internal/ui"
)

// contactOnly is an empty Workspace whose Profile is the Contact init writes, and nothing
// more: create-profile has not run yet.
func contactOnly(t *testing.T) string {
	root := emptyJobs(t)
	profile := "# Master Profile\n\n## Contact\n\n| Field | Value |\n| --- | --- |\n| Name | Alex Quinn |\n| Email | alex.quinn@example.test |\n"
	if err := os.WriteFile(filepath.Join(root, "profile", "master-profile.md"), []byte(profile), 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

// The first-run welcome is one checklist: the first Lead, then each setup step not yet
// done, each with the key that goes there.
func TestWelcomeListsSetupStepsNotDone(t *testing.T) {
	d := newDriver(t, contactOnly(t), fixtureClock, 120, 36, ui.WithEditorSource("default"))
	v := d.plain()
	for _, want := range []string{"▞ rolecall · no Applications yet", "a  add your first Lead", "e  pick an Editor",
		"h  connect a Harness", "5  set up your Profile: fill in your Contact, then write the rest in your Editor", "?  see every key"} {
		if !strings.Contains(v, want) {
			t.Errorf("welcome lacks %q:\n%s", want, v)
		}
	}
	for _, gone := range []string{"/ search", "o sort"} {
		if strings.Contains(v, gone) {
			t.Errorf("welcome offers %q on an empty list:\n%s", gone, v)
		}
	}

	done := newDriver(t, emptyJobs(t), fixtureClock, 120, 36, ui.WithEditor("nvim"), ui.WithHarness("claude {prompt}"))
	v = done.plain()
	for _, gone := range []string{"Set up", "pick an Editor", "connect a Harness", "set up your Profile"} {
		if strings.Contains(v, gone) {
			t.Errorf("welcome lists a done step %q:\n%s", gone, v)
		}
	}
}

// e and h open Settings › This machine on the Editor and Harness rows: Enter there edits
// that very setting. Esc goes back to the welcome.
func TestWelcomeKeysLandOnTheirSettingsRow(t *testing.T) {
	for key, label := range map[string]string{"e": "Editor", "h": "Harness"} {
		t.Run(key, func(t *testing.T) {
			d := newDriver(t, contactOnly(t), fixtureClock, 120, 36, ui.WithEditorSource("default"), ui.WithConfigHome(t.TempDir()))
			d.Type(key)
			v := d.plain()
			if !strings.Contains(v, "[This machine]") || !strings.Contains(v, "▌ "+label) {
				t.Fatalf("%s did not open Settings › This machine on %s:\n%s", key, label, v)
			}
			d.key(0x1b)
			if v := d.plain(); !strings.Contains(v, "no Applications yet") {
				t.Fatalf("esc did not return to the welcome:\n%s", v)
			}
		})
	}
}

// The welcome and the missing-Profile screen give one instruction, and with a Harness
// connected both name create-profile; without one neither does.
func TestWelcomeAndProfileGiveTheSameSetupInstruction(t *testing.T) {
	for _, tc := range []struct {
		opts []ui.Option
		want string
	}{
		{nil, "fill in your Contact, then write the rest in your Editor"},
		{[]ui.Option{ui.WithHarness("claude {prompt}")}, "fill in your Contact, then run create-profile in your Harness"},
	} {
		root := emptyJobs(t)
		if err := os.RemoveAll(filepath.Join(root, "profile")); err != nil {
			t.Fatal(err)
		}
		d := newDriver(t, root, fixtureClock, 140, 36, tc.opts...)
		welcome := d.plain()
		d.Type("5")
		profile := d.plain()
		for name, v := range map[string]string{"welcome": welcome, "Profile": profile} {
			if !strings.Contains(v, tc.want) {
				t.Errorf("%s lacks %q:\n%s", name, tc.want, v)
			}
			if tc.opts == nil && strings.Contains(v, "create-profile") {
				t.Errorf("%s names create-profile with no Harness:\n%s", name, v)
			}
		}
	}
}
