// Package store reads Applications from a Workspace.
package store

import (
	"errors"
	"os"
	"path/filepath"
)

type Application struct {
	Folder, Title, Company, Track, Status                                string
	Documents                                                            string
	PDFs                                                                 []string
	JDHasText, ResumeHasText, CoverLetterHasText, GapsHasText            bool
	HasJD, HasNotes, HasFit, HasGaps, HasResume, HasCoverLetter, HasSent bool
	Schema                                                               int
	URL, Closes, Fit                                                     string
	ResumeOnly                                                           bool
	// Posted is the date the ad was posted (YYYY-MM-DD) and PostedLabel the label the
	// site showed for it ("14d ago"); either may be empty.
	Posted, PostedLabel string
	// Applicants is how many had applied when the Lead was found; nil when unknown.
	Applicants *int
	// Source is the Source registry name the Lead came from, such as linkedin.
	Source string
	// FilterReason names the Search rule that dismissed the Application, and FilterNote
	// its detail. FilterOverruled marks one the Owner restored: no rule dismisses it again.
	FilterReason, FilterNote string
	FilterOverruled          bool
	// FollowedUp is the day (YYYY-MM-DD) the Owner last followed up without a Status
	// change, or "". Follow-up due counts from it when it is later than the last change.
	FollowedUp string
	AdFacts
	StatusLog               []StatusEntry
	AppliedDate, LastChange *StatusEntry
	Malformed               error `json:"-"`
}

// Filtered reports that a Search rule dismissed the Application and the Owner has not
// overruled it.
func (a Application) Filtered() bool {
	return a.Status == StatusDismissed && a.FilterReason != "" && !a.FilterOverruled
}

// Unsent reports that the Application is still the Owner's to send: a Lead or a Draft.
// Documents are prepared, built and dismissed only while it is.
func (a Application) Unsent() bool {
	return a.Status == StatusLead || a.Status == StatusDraft
}

type StatusEntry struct {
	Status string
	At     string
	Approx bool
}
type Store struct {
	root  string
	files FileSystem
}

func Open(root string) (*Store, error) { return OpenWithFileSystem(root, nil) }

// OpenWithFileSystem opens a Workspace with an atomic publication filesystem.
func OpenWithFileSystem(root string, files FileSystem) (*Store, error) {
	// Cleaned, so the root is a prefix of every path joined under it: "./ws" joins as "ws/…".
	root = filepath.Clean(root)
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, &os.PathError{Op: "open", Path: root, Err: errors.New("Workspace is not a directory")}
	}
	return &Store{root: root, files: files}, nil
}

// Scan reads every Application under jobs/ (the archive aside). A folder that fails to
// read comes back Malformed with the I/O error, so the rest still show.
func (s *Store) Scan() ([]Application, error) {
	apps := []Application{}
	tracks, err := os.ReadDir(filepath.Join(s.root, "jobs"))
	if errors.Is(err, os.ErrNotExist) {
		return apps, nil
	}
	if err != nil {
		return nil, err
	}
	for _, track := range tracks {
		if !track.IsDir() || track.Name() == "_archive" {
			continue
		}
		folders, err := os.ReadDir(filepath.Join(s.root, "jobs", track.Name()))
		if err != nil {
			return nil, err
		}
		for _, folder := range folders {
			if !folder.IsDir() {
				continue
			}
			dir := filepath.Join(s.root, "jobs", track.Name(), folder.Name())
			a, err := s.Rescan(dir)
			if err != nil {
				// One unreadable folder is one Malformed Application, not an empty Workspace.
				a = &Application{Folder: dir, Schema: 1, Status: StatusLead, Malformed: &ParseError{Path: dir, Reason: err}}
			}
			if a != nil {
				apps = append(apps, *a)
			}
		}
	}
	return apps, nil
}

// application is the one gate for folder-scoped methods: a folder is an Application
// only when it holds a meta.yml. A missing meta.yml returns os.ErrNotExist. Callers that
// need the parsed record use applicationRecord.
func (s *Store) application(folder string) error {
	if _, err := os.Stat(filepath.Join(folder, "meta.yml")); err != nil {
		return err
	}
	return nil
}

// applicationRecord is the gate plus the parsed Application: a malformed meta.yml is
// refused as an error. Every write uses it.
func (s *Store) applicationRecord(folder string) error {
	if err := s.application(folder); err != nil {
		return err
	}
	a, err := s.readApplicationFile(folder)
	if err != nil {
		return err
	}
	if a.Malformed != nil {
		return a.Malformed
	}
	return nil
}

// readApplicationFile reads the Application at folder without refusing a malformed
// meta.yml. A missing meta.yml returns os.ErrNotExist.
func (s *Store) readApplicationFile(folder string) (Application, error) {
	b, err := os.ReadFile(filepath.Join(folder, "meta.yml"))
	if err != nil {
		return Application{}, err
	}
	return readApplication(folder, b), nil
}

func (s *Store) Rescan(dir string) (*Application, error) {
	a, err := s.readApplicationFile(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err = readFolder(&a); err != nil {
		return nil, err
	}
	return &a, nil
}

// Root is the Workspace folder.
func (s *Store) Root() string { return s.root }
