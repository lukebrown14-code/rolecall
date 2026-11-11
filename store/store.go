// Package store reads Applications from a Workspace.
package store

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type Application struct{ Folder, Title, Company, Track, Status string }
type Store struct{ root string }

func Open(root string) (*Store, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, &os.PathError{Op: "open", Path: root, Err: errors.New("Workspace is not a directory")}
	}
	return &Store{root: root}, nil
}

var statusEntry = regexp.MustCompile(`^  - \{status: ([A-Za-z]+),`)

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
			b, err := os.ReadFile(filepath.Join(dir, "meta.yml"))
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				return nil, err
			}
			a := Application{Folder: dir, Status: "Lead"}
			for _, line := range strings.Split(string(b), "\n") {
				if m := statusEntry.FindStringSubmatch(line); m != nil {
					a.Status = m[1]
					continue
				}
				k, v, ok := strings.Cut(line, ":")
				if !ok {
					continue
				}
				v = strings.TrimSpace(v)
				if strings.HasPrefix(v, "\"") {
					if end := strings.LastIndex(v, "\""); end > 0 {
						if q, e := strconv.Unquote(v[:end+1]); e == nil {
							v = q
						}
					}
				} else {
					if i := strings.Index(v, " #"); i >= 0 {
						v = strings.TrimSpace(v[:i])
					}
				}
				switch k {
				case "title":
					a.Title = v
				case "company":
					a.Company = v
				case "track":
					a.Track = v
				}
			}
			apps = append(apps, a)
		}
	}
	return apps, nil
}
