// Package doctor checks privacy-related Workspace permissions.
package doctor

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Check reports Workspace paths whose permissions allow broader access than
// rolecall's private-file defaults. Symlinks are not followed.
func Check(root string) ([]string, error) {
	var warnings []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		var limit os.FileMode
		var kind, fix string
		switch {
		case info.IsDir():
			limit, kind, fix = 0o700, "directory", "700"
		case info.Mode().IsRegular():
			limit, kind, fix = 0o600, "file", "600"
		default:
			return nil
		}
		mode := info.Mode().Perm()
		if mode&^limit != 0 {
			warnings = append(warnings, fmt.Sprintf("%s (%s): permissions %04o; fix with chmod %s %s", path, kind, mode, fix, shellQuote(path)))
		}
		return nil
	})
	return warnings, err
}

func shellQuote(s string) string {
	if s != "" && strings.IndexFunc(s, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("/._-", r))
	}) == -1 {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
