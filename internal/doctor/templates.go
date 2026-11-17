package doctor

import (
	"os"
	"path/filepath"
	"slices"
)

// templateFiles are the Plugin Template files a Workspace may override by putting its
// own copy in <workspace>/templates/ (build.py's TEMPLATE_FILES).
var templateFiles = []string{"brand.typ", "resume.typ", "cover-letter.typ"}

// ignoredHints says why a known file the build never reads is ignored and what to do
// instead, keyed by its Workspace-relative path.
var ignoredHints = map[string]string{
	"templates/style.md": "the skills read the Plugin's styles/au.md; put your own writing rules in profile/writing-preferences.md",
	"style.md":           "the skills read the Plugin's styles/au.md; put your own writing rules in profile/writing-preferences.md",
	"templates/fonts":    "fonts load from the Plugin's templates/fonts",
}

// TemplateFiles reports the Workspace Template overrides the build uses, as
// Workspace-relative paths, and a warning for each Workspace file that looks like a
// Template setting but is never read. A root brand.typ is the older override spot: it
// still applies unless templates/brand.typ is there too.
func TemplateFiles(workspace string) (overrides, ignored []string) {
	if workspace == "" {
		return nil, nil
	}
	isFile := func(rel string) bool {
		info, err := os.Stat(filepath.Join(workspace, rel))
		return err == nil && info.Mode().IsRegular()
	}
	for _, name := range templateFiles {
		switch rel := "templates/" + name; {
		case isFile(rel):
			overrides = append(overrides, rel)
		case name == "brand.typ" && isFile(name):
			overrides = append(overrides, name)
		}
	}
	if isFile("brand.typ") && isFile("templates/brand.typ") {
		ignored = append(ignored, "brand.typ is not read: templates/brand.typ overrides it; merge your changes into templates/brand.typ and delete brand.typ")
	}
	if isFile("style.md") {
		ignored = append(ignored, "style.md is not read: "+ignoredHints["style.md"])
	}
	entries, err := os.ReadDir(filepath.Join(workspace, "templates"))
	if err != nil {
		return overrides, ignored
	}
	for _, entry := range entries {
		if slices.Contains(templateFiles, entry.Name()) && entry.Type().IsRegular() {
			continue
		}
		rel := "templates/" + entry.Name()
		hint, known := ignoredHints[rel]
		if !known {
			hint = "the build reads only brand.typ, resume.typ and cover-letter.typ from templates/"
		}
		ignored = append(ignored, rel+" is not read: "+hint)
	}
	return overrides, ignored
}
