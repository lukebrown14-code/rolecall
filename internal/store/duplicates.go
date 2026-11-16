package store

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

var (
	trailingParen  = regexp.MustCompile(`\s*\([^)]*\)\s*$`)
	nonAlnum       = regexp.MustCompile(`[^a-z0-9]+`)
	companySuffix  = regexp.MustCompile(`\b(pty|ltd|limited|inc|group|australia)\b`)
	titleNoise     = []string{"level", "the", "a", "an"}
	minJDHashChars = 200
)

// TitleKey is a title as a Likely duplicate compares it: a trailing "(…)" dropped,
// lower-cased, every run of non-alphanumerics a space, the words level, the, a and an
// dropped, single-spaced.
func TitleKey(title string) string {
	s := nonAlnum.ReplaceAllString(strings.ToLower(trailingParen.ReplaceAllString(title, "")), " ")
	words := slices.DeleteFunc(strings.Fields(s), func(w string) bool { return slices.Contains(titleNoise, w) })
	return strings.Join(words, " ")
}

// CompanyKey is a company as a Likely duplicate compares it: lower-cased, the words pty,
// ltd, limited, inc, group and australia dropped, non-alphanumerics a space, single-spaced.
func CompanyKey(company string) string {
	s := companySuffix.ReplaceAllString(strings.ToLower(company), " ")
	return strings.Join(strings.Fields(nonAlnum.ReplaceAllString(s, " ")), " ")
}

// JDHash is the sha256 of an ad's text lower-cased with whitespace collapsed, or "" when
// that text is under 200 characters: too short to tell two ads apart.
func JDHash(text string) string {
	s := strings.Join(strings.Fields(strings.ToLower(text)), " ")
	if utf8.RuneCountInString(s) < minJDHashChars {
		return ""
	}
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// companyKeys is the non-empty keys of a company and its recruiter.
func companyKeys(company, recruiter string) []string {
	var out []string
	for _, k := range []string{CompanyKey(company), CompanyKey(recruiter)} {
		if k != "" && !slices.Contains(out, k) {
			out = append(out, k)
		}
	}
	return out
}

// LikelyDuplicates lists the Applications that look like the same ad: an equal TitleKey
// with a company or recruiter in common (either side's), or the same JDHash. It reads
// each Application's jd.md only when jdText is long enough to hash.
func (s *Store) LikelyDuplicates(company, recruiter, title, jdText string) ([]Application, error) {
	apps, err := s.Scan()
	if err != nil {
		return nil, err
	}
	key, names, hash := TitleKey(title), companyKeys(company, recruiter), JDHash(jdText)
	var out []Application
	for _, a := range apps {
		if a.Malformed != nil {
			continue
		}
		same := key != "" && TitleKey(a.Title) == key && slices.ContainsFunc(companyKeys(a.Company, a.Recruiter), func(k string) bool {
			return slices.Contains(names, k)
		})
		if !same && hash != "" {
			if b, err := os.ReadFile(filepath.Join(a.Folder, "jd.md")); err == nil {
				same = JDHash(string(b)) == hash
			}
		}
		if same {
			out = append(out, a)
		}
	}
	return out, nil
}
