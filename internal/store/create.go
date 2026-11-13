package store

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// CreateFields contains the fields supplied by the Add form.
type CreateFields struct{ Title, Company, Track, URL, Closes, Fit string }

// Create creates a Lead using the supplied clock value and returns its folder.
func (s *Store) Create(fields CreateFields, now time.Time) (folder string, err error) {
	if strings.TrimSpace(fields.Title) == "" || strings.TrimSpace(fields.Company) == "" {
		return "", fmt.Errorf("Title and Company are required")
	}
	if err := ValidTrackName(fields.Track, nil); err != nil {
		return "", err
	}
	text := "schema: 1\n"
	for _, pair := range [][2]string{{keyTitle, fields.Title}, {keyCompany, fields.Company}, {keyTrack, fields.Track}, {"url", fields.URL}, {"closes", fields.Closes}, {"fit", fields.Fit}} {
		if pair[1] != "" {
			text += pair[0] + ": " + quoteScalar(pair[1]) + "\n"
		}
	}
	text += "status_log:\n  - {status: Lead, at: " + now.UTC().Format(time.RFC3339) + "}\n"
	if a := readApplication("", []byte(text)); a.Malformed != nil {
		return "", a.Malformed
	}
	parent := filepath.Join(s.root, "jobs", fields.Track)
	if err = s.fs().MkdirAll(parent, privateDirMode); err != nil {
		return "", err
	}
	slug := strings.Trim(slugSeparators.ReplaceAllString(strings.ToLower(fields.Company+"-"+fields.Title), "-"), "-") + "-" + now.Format("2006-01-02")
	for n := 1; ; n++ {
		name := slug
		if n > 1 {
			name += fmt.Sprintf("-%d", n)
		}
		folder = filepath.Join(parent, name)
		err = s.fs().Mkdir(folder, privateDirMode)
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
			_ = s.fs().RemoveAll(created)
		}
	}()
	if err = s.atomicWrite(filepath.Join(folder, "jd.md"), nil, privateFileMode); err != nil {
		return "", err
	}
	if err = s.atomicWrite(filepath.Join(folder, "meta.yml"), []byte(text), privateFileMode); err != nil {
		return "", err
	}
	return folder, nil
}

var slugSeparators = regexp.MustCompile("[^a-z0-9]+")
