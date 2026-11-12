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

// Every user-visible string uses GLOSSARY.md's terms: an unqualified term in an
// _Avoid_ line may not appear in a string literal. Qualified ones, "dismissed (for
// this)", depend on context and are left to review.
func TestStringsAvoidGlossaryAvoidTerms(t *testing.T) {
	g, err := os.ReadFile("../../../GLOSSARY.md")
	if err != nil {
		t.Fatal(err)
	}
	var avoid []*regexp.Regexp
	for _, line := range strings.Split(string(g), "\n") {
		rest, ok := strings.CutPrefix(line, "_Avoid_: ")
		if !ok {
			continue
		}
		// A trailing "(for this)" qualifies the whole line: those terms depend on context
		// and are left to review. Elsewhere a parenthesis qualifies only its own term.
		if strings.HasSuffix(rest, "(for this)") || strings.HasSuffix(rest, "(for these)") {
			continue
		}
		for _, term := range splitOutsideParens(rest) {
			if term = strings.TrimSpace(term); term == "" || strings.Contains(term, "(") {
				continue
			}
			if _, ok := secondMeaning[strings.ToLower(term)]; !ok {
				avoid = append(avoid, regexp.MustCompile(`(?i)\b`+regexp.QuoteMeta(term)+`\b`))
			}
		}
	}
	if len(avoid) == 0 {
		t.Fatal("no _Avoid_ terms found in GLOSSARY.md")
	}
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
			for _, re := range avoid {
				if re.MatchString(s) {
					t.Errorf("%s: %q uses %q, which GLOSSARY.md avoids", fs.Position(lit.Pos()), s, re.String()[6:len(re.String())-2])
				}
			}
			return true
		})
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
