package store

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// CreateFields contains the fields supplied by the Add form.
type CreateFields struct{ Title, Company, Track, URL, Closes, Fit string }

// Create creates a Lead using the supplied clock value and returns its folder.
func (s *Store) Create(fields CreateFields, now time.Time) (folder string, err error) {
	if strings.TrimSpace(fields.Title) == "" || strings.TrimSpace(fields.Company) == "" || fields.Track == "" || fields.Track == "." || fields.Track == ".." || fields.Track == "_archive" || strings.ContainsAny(fields.Track, "/\\") {
		return "", fmt.Errorf("Title, Company and a single Track folder are required")
	}
	text := "schema: 1\n"
	for _, pair := range [][2]string{{"title", fields.Title}, {"company", fields.Company}, {"track", fields.Track}, {"url", fields.URL}, {"closes", fields.Closes}, {"fit", fields.Fit}} {
		if pair[1] != "" {
			text += pair[0] + ": " + strconv.Quote(pair[1]) + "\n"
		}
	}
	text += "status_log:\n  - {status: Lead, at: " + now.UTC().Format(time.RFC3339) + "}\n"
	if a := readApplication("", []byte(text)); a.Malformed != nil {
		return "", a.Malformed
	}
	parent := filepath.Join(s.root, "jobs", fields.Track)
	if err = os.MkdirAll(parent, 0700); err != nil {
		return "", err
	}
	slug := strings.Trim(slugSeparators.ReplaceAllString(strings.ToLower(fields.Company+"-"+fields.Title), "-"), "-") + "-" + now.Format("2006-01-02")
	for n := 1; ; n++ {
		name := slug
		if n > 1 {
			name += fmt.Sprintf("-%d", n)
		}
		folder = filepath.Join(parent, name)
		err = os.Mkdir(folder, 0700)
		if os.IsExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		break
	}
	created := folder
	defer func() {
		if err != nil {
			os.RemoveAll(created)
		}
	}()
	if err = os.WriteFile(filepath.Join(folder, "jd.md"), nil, 0600); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(folder, ".meta-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if _, err = f.WriteString(text); err != nil {
		return "", err
	}
	if err = f.Sync(); err != nil {
		return "", err
	}
	if err = f.Close(); err != nil {
		return "", err
	}
	if err = os.Rename(f.Name(), filepath.Join(folder, "meta.yml")); err != nil {
		return "", err
	}
	return folder, nil
}

var slugSeparators = regexp.MustCompile("[^a-z0-9]+")
