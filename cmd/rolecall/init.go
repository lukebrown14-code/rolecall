package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"charm.land/huh/v2"
	"github.com/lukebrown14-code/rolecall/config"
	"github.com/lukebrown14-code/rolecall/store"
	"github.com/lukebrown14-code/rolecall/ui"
)

const (
	profileResume                = "resume"
	profileQuestions             = "questions"
	profileLater                 = "later"
	configDirMode    os.FileMode = 0o700
	configFileMode   os.FileMode = 0o600
)

type contactDetails struct {
	Name, Email, Phone, Location, LinkedIn string
}

func (c contactDetails) fields() map[string]string {
	return map[string]string{"Name": c.Name, "Email": c.Email, "Phone": c.Phone, "Location": c.Location, "LinkedIn": c.LinkedIn}
}

type initOptions struct {
	contact                    contactDetails
	tracks                     []string
	search                     store.SearchArea
	workspace, profile, resume string
	tracksText, placesText     string
	accessible                 bool
}

type profileLauncher func(harness, workspace, resume string) error

func initWizard(args []string, in io.Reader, out, errOut io.Writer, launcher profileLauncher) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(errOut)
	var o initOptions
	fs.StringVar(&o.contact.Name, "name", "", "Contact name")
	fs.StringVar(&o.contact.Email, "email", "", "Contact email")
	fs.StringVar(&o.contact.Phone, "phone", "", "Contact phone")
	fs.StringVar(&o.contact.Location, "location", "", "Contact location")
	fs.StringVar(&o.contact.LinkedIn, "linkedin", "", "Contact LinkedIn URL")
	fs.StringVar(&o.tracksText, "tracks", "", "Comma-separated Tracks")
	fs.StringVar(&o.placesText, "search-places", "", "Comma-separated Search area places")
	fs.StringVar(&o.search.Remote, "search-remote", "", "Search area remote rule: in-places, anywhere, or no")
	fs.StringVar(&o.workspace, "workspace", "", "Workspace path")
	fs.StringVar(&o.profile, "profile", "", "Profile setup: resume, questions, or later")
	fs.StringVar(&o.resume, "resume", "", "Resume or LinkedIn export path")
	fs.BoolVar(&o.accessible, "accessible", false, "Use accessible prompts")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 1 {
		return errors.New("usage: rolecall init [--workspace DIR] [fields]")
	}
	if fs.NArg() == 1 {
		if o.workspace != "" {
			return errors.New("set the Workspace path with either a positional argument or --workspace, not both")
		}
		o.workspace = fs.Arg(0)
	}
	if wasSet(fs, "workspace") && strings.TrimSpace(o.workspace) == "" {
		return errors.New("Workspace path is required")
	}
	if o.workspace != "" {
		if err := store.ValidateWorkspacePath(o.workspace); err != nil {
			return err
		}
	}
	if (wasSet(fs, "name") && strings.TrimSpace(o.contact.Name) == "") || (wasSet(fs, "email") && strings.TrimSpace(o.contact.Email) == "") {
		return errors.New("Name and Email are required")
	}
	// Huh makes a new Scanner for each accessible prompt. Limit each read to
	// one line so a scanner cannot consume answers meant for later prompts.
	if err := promptMissing(&o, fs, &promptLineReader{reader: bufio.NewReader(in)}, out); err != nil {
		return err
	}
	o.tracks, o.search.Places = splitList(o.tracksText), splitList(o.placesText)
	if strings.TrimSpace(o.contact.Name) == "" || strings.TrimSpace(o.contact.Email) == "" {
		return errors.New("Name and Email are required")
	}
	if o.profile != profileResume && o.profile != profileQuestions && o.profile != profileLater {
		return errors.New("Profile choice must be resume, questions, or later")
	}
	if o.profile == profileResume && strings.TrimSpace(o.resume) == "" {
		return errors.New("Resume path is required")
	}
	if err := validateRemote(o.search.Remote); err != nil {
		return err
	}
	if len(o.search.Places) > 0 && o.search.Remote == "" {
		o.search.Remote = "in-places"
	}
	root, err := filepath.Abs(o.workspace)
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	if err := initWorkspaceFiles(root, o.contact, o.tracks, o.search); err != nil {
		return err
	}
	if err := writeWorkspaceConfig(config.Path(home), root); err != nil {
		return err
	}
	fmt.Fprintln(out, "Created a private Workspace.")
	fmt.Fprintln(out, "The Harness can send your Profile, resume and job text to an AI service using your credentials; review its privacy practices.")
	fmt.Fprintln(out, "Keep this Workspace's git repository private. It contains personal data.")
	fmt.Fprintln(out, "Make your documents yours: see README → Make your documents yours.")
	return finishProfile(o, root, home, out, launcher)
}

type promptLineReader struct {
	reader  *bufio.Reader
	pending []byte
}

func (r *promptLineReader) Read(p []byte) (int, error) {
	if len(r.pending) == 0 {
		line, err := r.reader.ReadBytes('\n')
		if len(line) == 0 {
			return 0, err
		}
		r.pending = line
	}
	n := copy(p, r.pending)
	r.pending = r.pending[n:]
	return n, nil
}

func promptMissing(o *initOptions, fs *flag.FlagSet, in io.Reader, out io.Writer) error {
	fields := make([]huh.Field, 0)
	input := func(name, title string, target *string, required bool) {
		if wasSet(fs, name) {
			return
		}
		f := huh.NewInput().Title(title).Value(target)
		if required {
			f.Validate(func(v string) error {
				if strings.TrimSpace(v) == "" {
					return errors.New("required")
				}
				return nil
			})
		}
		fields = append(fields, f)
	}
	input("name", "Name", &o.contact.Name, true)
	input("email", "Email", &o.contact.Email, true)
	input("phone", "Phone (optional)", &o.contact.Phone, false)
	input("location", "Location (optional)", &o.contact.Location, false)
	input("linkedin", "LinkedIn (optional)", &o.contact.LinkedIn, false)
	if !wasSet(fs, "tracks") {
		fields = append(fields, huh.NewInput().Title("Tracks (comma-separated, optional)").Value(&o.tracksText))
	}
	if !wasSet(fs, "search-places") {
		fields = append(fields, huh.NewInput().Title("Search area places (comma-separated, optional)").Value(&o.placesText))
	}
	if !wasSet(fs, "search-remote") {
		fields = append(fields, huh.NewSelect[string]().Title("Remote roles").Options(huh.NewOption("No Search area", ""), huh.NewOption("In my places", "in-places"), huh.NewOption("Anywhere", "anywhere"), huh.NewOption("No remote roles", "no")).Value(&o.search.Remote))
	}
	if !wasSet(fs, "workspace") && fs.NArg() == 0 {
		fields = append(fields, huh.NewInput().Title("Workspace path").Value(&o.workspace).Validate(func(v string) error {
			if strings.TrimSpace(v) == "" {
				return errors.New("required")
			}
			return nil
		}))
	}
	if len(fields) > 0 {
		form := huh.NewForm(huh.NewGroup(fields...)).WithInput(in).WithOutput(out).WithAccessible(o.accessible)
		if err := form.Run(); err != nil {
			return err
		}
	}
	if o.profile == "" {
		choices := []huh.Option[string]{huh.NewOption("From my current resume", profileResume), huh.NewOption("Answer questions", profileQuestions), huh.NewOption("Later", profileLater)}
		if err := huh.NewForm(huh.NewGroup(huh.NewSelect[string]().Title("Create your Profile now?").Options(choices...).Value(&o.profile))).WithInput(in).WithOutput(out).WithAccessible(o.accessible).Run(); err != nil {
			return err
		}
	}
	if o.profile == profileResume && o.resume == "" {
		if err := huh.NewForm(huh.NewGroup(huh.NewInput().Title("Path to your resume or LinkedIn export").Value(&o.resume).Validate(func(v string) error {
			if strings.TrimSpace(v) == "" {
				return errors.New("required")
			}
			return nil
		}))).WithInput(in).WithOutput(out).WithAccessible(o.accessible).Run(); err != nil {
			return err
		}
	}
	return nil
}

func wasSet(fs *flag.FlagSet, name string) bool {
	found := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}
func splitList(s string) []string {
	var out []string
	for _, item := range strings.Split(s, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}
func validateRemote(s string) error {
	if s == "" || s == "in-places" || s == "anywhere" || s == "no" {
		return nil
	}
	return fmt.Errorf("invalid Search area remote rule %q: use in-places, anywhere, or no", s)
}

func writeWorkspaceConfig(path, workspace string) error {
	if err := os.MkdirAll(filepath.Dir(path), configDirMode); err != nil {
		return err
	}
	b, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	lines := strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
	replaced := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		key, _, hasValue := strings.Cut(trimmed, "=")
		if hasValue && strings.TrimSpace(key) == "workspace" {
			lines[i] = "workspace = " + strconvQuote(workspace) + trailingTOMLComment(line)
			replaced = true
			break
		}
	}
	if !replaced {
		if len(lines) == 1 && lines[0] == "" {
			lines = nil
		}
		lines = append(lines, "workspace = "+strconvQuote(workspace))
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), configFileMode); err != nil {
		return err
	}
	return os.Chmod(path, configFileMode)
}

func trailingTOMLComment(line string) string {
	quoted, escaped := false, false
	for i, r := range line {
		if escaped {
			escaped = false
			continue
		}
		if quoted && r == '\\' {
			escaped = true
			continue
		}
		if r == '"' {
			quoted = !quoted
			continue
		}
		if r == '#' && !quoted {
			return " " + strings.TrimSpace(line[i:])
		}
	}
	return ""
}

func strconvQuote(s string) string { b, _ := json.Marshal(s); return string(b) }

func finishProfile(o initOptions, root, home string, out io.Writer, launcher profileLauncher) error {
	if o.profile == profileLater {
		fmt.Fprintf(out, "Run this when you are ready: create-profile --data %s\n", ui.QuoteShellArgument(root))
		return nil
	}
	resume := ""
	if o.profile == profileResume {
		resume = o.resume
	}
	c, err := config.Load(home, root)
	if err != nil {
		return err
	}
	if c.Harness == "" {
		if resume != "" {
			fmt.Fprintf(out, "Run this in your Harness when ready: create-profile --data %s --resume %s\n", ui.QuoteShellArgument(root), ui.QuoteShellArgument(resume))
		} else {
			fmt.Fprintf(out, "Run this in your Harness when ready: create-profile --data %s\n", ui.QuoteShellArgument(root))
		}
		return nil
	}
	if launcher == nil {
		launcher = launchProfile
	}
	return launcher(c.Harness, root, resume)
}

func launchProfile(harness, workspace, resume string) error {
	return ui.LaunchHarness(harness, workspace, "create-profile", resume)
}
