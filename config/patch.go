package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	dirMode  = 0o700
	fileMode = 0o600
)

// Patch sets one top-level key in the config file at path, in place. Comments, other lines
// and the key's trailing comment are kept; a missing key is added before the first table,
// and a missing file is created. value is a string, int or bool.
func Patch(path, key string, value any) error {
	var lit string
	switch v := value.(type) {
	case string:
		lit = `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(v) + `"`
	case int:
		lit = strconv.Itoa(v)
	case bool:
		lit = strconv.FormatBool(v)
	default:
		return fmt.Errorf("config: cannot write %T", value)
	}
	b, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	lines := strings.SplitAfter(string(b), "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	keyLine := regexp.MustCompile(`^(\s*` + regexp.QuoteMeta(key) + `\s*=\s*)`)
	insertAt := len(lines)
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "[") {
			insertAt = i
			break
		}
		if m := keyLine.FindStringSubmatch(l); m != nil {
			lines[i] = m[1] + lit + valueTail(l[len(m[1]):])
			return write(path, lines)
		}
	}
	if insertAt > 0 && !strings.HasSuffix(lines[insertAt-1], "\n") {
		lines[insertAt-1] += "\n"
	}
	lines = append(lines[:insertAt], append([]string{key + " = " + lit + "\n"}, lines[insertAt:]...)...)
	return write(path, lines)
}

// valueTail is what follows a value on its line: the spacing, trailing comment and newline.
func valueTail(rest string) string {
	i := 0
	if rest != "" && (rest[0] == '"' || rest[0] == '\'') {
		q := rest[0]
		for i = 1; i < len(rest) && rest[i] != q; i++ {
			if q == '"' && rest[i] == '\\' {
				i++
			}
		}
		i++
	} else {
		for i < len(rest) && rest[i] != '#' && rest[i] != '\n' {
			i++
		}
		for i > 0 && rest[i-1] == ' ' {
			i--
		}
	}
	return rest[min(i, len(rest)):]
}

func write(path string, lines []string) error {
	if err := os.MkdirAll(filepath.Dir(path), dirMode); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "")), fileMode)
}
