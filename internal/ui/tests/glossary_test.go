package ui_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// secondMeaning lists avoided terms the UI uses in another, legitimate sense, and why.
var secondMeaning = map[string]string{
	"search": "the / filter over rows, not Discovery",
	"match":  "search results, not Fit",
	"job":    "skill names such as capture-job and find-jobs",
}

// proseSecondMeaning lists avoided terms the Plugin and skill prose uses in another,
// legitimate sense, and why. Prose is scanned with code spans stripped, so file names
// such as writing-preferences.md never match.
var proseSecondMeaning = map[string]string{
	"project":     "a GitHub project's facts (profile/projects.md), not the Workspace",
	"preferences": "the Owner's writing preferences (writing-preferences.md), not Settings",
	"brand":       "the brand.typ Template file",
	"compile":     "the typst compile subcommand",
	"render":      "rendering text or a board, not a Build",
	"scan":        "a reader scanning a page or text",
	"score":       "the Owner's word for judging Fit",
	"task":        "an agent task naming a skill, not a Handoff",
	"done":        "the ordinary past tense of \"do\"",
	"match":       "matching text or a register, not Fit",
	"search":      "a manual search, not Discovery",
	"job":         "a job ad, posting or folder, not Experience",
}

// avoidRule is one _Avoid_ term and the regex that finds it.
type avoidRule struct {
	term string
	re   *regexp.Regexp
}

// glossaryAvoidRules reads GLOSSARY.md's _Avoid_ lines into rules, skipping terms the
// given map marks as used in another sense. A trailing "(for this)" qualifies the whole
// line: those terms depend on context and are left to review. Elsewhere a parenthesis
// qualifies only its own term.
func glossaryAvoidRules(t *testing.T, second map[string]string) []avoidRule {
	t.Helper()
	g, err := os.ReadFile("../../../GLOSSARY.md")
	if err != nil {
		t.Fatal(err)
	}
	var rules []avoidRule
	for _, line := range strings.Split(string(g), "\n") {
		rest, ok := strings.CutPrefix(line, "_Avoid_: ")
		if !ok {
			continue
		}
		if strings.HasSuffix(rest, "(for this)") || strings.HasSuffix(rest, "(for these)") {
			continue
		}
		for _, term := range splitOutsideParens(rest) {
			if term = strings.TrimSpace(term); term == "" || strings.Contains(term, "(") {
				continue
			}
			if _, ok := second[strings.ToLower(term)]; !ok {
				rules = append(rules, avoidRule{term, regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(term) + `\b`)})
			}
		}
	}
	if len(rules) == 0 {
		t.Fatal("no _Avoid_ terms found in GLOSSARY.md")
	}
	return rules
}

// Every user-visible string uses GLOSSARY.md's terms: an unqualified term in an
// _Avoid_ line may not appear in a string literal. Qualified ones, "dismissed (for
// this)", depend on context and are left to review.
func TestStringsAvoidGlossaryAvoidTerms(t *testing.T) {
	avoid := glossaryAvoidRules(t, secondMeaning)
	var files []string
	for _, dir := range []string{"..", "../../store", "../../../cmd/rolecall"} {
		err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if entry.Name() == "tests" || entry.Name() == "testdata" {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(path, ".go") {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		fs := token.NewFileSet()
		file, err := parser.ParseFile(fs, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			// Import paths are implementation details, never user-visible wording.
			if _, ok := n.(*ast.ImportSpec); ok {
				return false
			}
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			s, _ := strconv.Unquote(lit.Value)
			// SCHEMA.md fixes this filename; it is not the Settings screen's label.
			if s == "writing-preferences.md" || s == "profile/writing-preferences.md" {
				return true
			}
			for _, rule := range avoid {
				if rule.re.MatchString(s) {
					t.Errorf("%s: %q uses %q, which GLOSSARY.md avoids", fs.Position(lit.Pos()), s, rule.term)
				}
			}
			return true
		})
	}
}

// Follow-up due is the only word for an overdue follow-up: no string the UI draws calls it
// stale, quiet or a reminder.
func TestStringsNeverSayStaleQuietOrReminder(t *testing.T) {
	banned := regexp.MustCompile(`(?i)\b(stale|quiet|reminder)\b`)
	err := filepath.WalkDir("..", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && (entry.Name() == "tests" || entry.Name() == "testdata") {
			return filepath.SkipDir
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(n ast.Node) bool {
			if _, ok := n.(*ast.ImportSpec); ok {
				return false
			}
			if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				if s, _ := strconv.Unquote(lit.Value); banned.MatchString(s) {
					t.Errorf("%s: %q uses a word the UI avoids for Follow-up due", fset.Position(lit.Pos()), s)
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// The Plugin and skill prose is part of the product's vocabulary, so it uses
// GLOSSARY.md's terms too: an unqualified _Avoid_ term may not appear in prose.
// Code spans and fenced blocks are stripped first, so file names and shell
// commands are never matched.
func TestProseAvoidsGlossaryAvoidTerms(t *testing.T) {
	var files []string
	for _, pattern := range []string{
		"../../../plugin/README.md",
		"../../../plugin/writing-rules.md",
		"../../../plugin/skills/*/SKILL.md",
	} {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, matches...)
	}
	if len(files) == 0 {
		t.Fatal("no plugin prose found")
	}
	rules := glossaryAvoidRules(t, proseSecondMeaning)
	fence := regexp.MustCompile("(?s)```.*?```")
	code := regexp.MustCompile("`[^`\n]*`")
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		text := code.ReplaceAllString(fence.ReplaceAllString(string(b), ""), "")
		for i, line := range strings.Split(text, "\n") {
			for _, rule := range rules {
				if rule.re.MatchString(line) {
					t.Errorf("%s:%d: %q uses %q, which GLOSSARY.md avoids", f, i+1, strings.TrimSpace(line), rule.term)
				}
			}
		}
	}
}

// splitOutsideParens splits on commas outside parentheses: "Stage (retired: Prepared,
// Assessed)" is one term.
func splitOutsideParens(s string) []string {
	var out []string
	depth, start := 0, 0
	for i, r := range s {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, s[start:i])
				start = i + 1
			}
		}
	}
	return append(out, s[start:])
}
