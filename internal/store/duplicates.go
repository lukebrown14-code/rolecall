package store

import (
	"crypto/sha256"
	"encoding/hex"
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

// employer is who an ad is for, as a Likely duplicate compares it: the company's key,
// or the recruiter's when no company is given, plus the recruiter's key.
type employer struct{ company, recruiter string }

func employerOf(company, recruiter string) employer {
	e := employer{CompanyKey(company), CompanyKey(recruiter)}
	if e.company == "" {
		e.company = e.recruiter
	}
	return e
}

// sameEmployer is two ads for one employer: the same company, or one side's company is
// the other's recruiter (an agency posting under its own name). Two employers that share
// only a recruiter are not the same.
func sameEmployer(a, b employer) bool {
	if a.company == "" || b.company == "" {
		return false
	}
	return a.company == b.company || a.company == b.recruiter || a.recruiter == b.company
}

// LikelyDuplicates lists the Applications that look like the same ad: an equal TitleKey
// for the same employer (sameEmployer), or the same JDHash. It reads
// each Application's jd.md only when jdText is long enough to hash.
func (s *Store) LikelyDuplicates(company, recruiter, title, jdText string) ([]Application, error) {
	apps, err := s.Scan()
	if err != nil {
		return nil, err
	}
	key, who, hash := TitleKey(title), employerOf(company, recruiter), JDHash(jdText)
	var out []Application
	for _, a := range apps {
		if a.Malformed != nil {
			continue
		}
		same := key != "" && TitleKey(a.Title) == key && sameEmployer(who, employerOf(a.Company, a.Recruiter))
		if !same && hash != "" {
			if b, err := readRegular(filepath.Join(a.Folder, "jd.md")); err == nil {
				same = JDHash(string(b)) == hash
			}
		}
		if same {
			out = append(out, a)
		}
	}
	return out, nil
}
