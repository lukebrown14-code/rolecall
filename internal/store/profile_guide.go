package store

import (
	"regexp"
	"strings"
)

// ProfileGuideEntry is one `##` section a Profile is expected to have, and how to fill it
// by hand.
type ProfileGuideEntry struct{ Heading, How string }

// ProfileGuide is the sections of profile/master-profile.md after Contact, in file order,
// with a line each on how to write them without a Harness. It is the one source of the
// starter file `rolecall init` writes and of the guide a fresh Profile screen shows.
var ProfileGuide = []ProfileGuideEntry{
	{"Headlines (approved per track)", "A table with the columns Track and Headline: the one-line job title shown under your name for each Track. Only titles you have held or are happy to be called."},
	{"Experience", "One ### heading per job (employer, place). Under it: Dates, Canonical title, then a bullet for each thing you did, with real numbers where you have them."},
	{"Key Skills keyword pool", "One comma-separated list of the skills you can honestly claim. Each resume picks from it."},
	{"Summary", "Two or three sentences: who you are and what you do best."},
	{"Facts that are NOT available", "A bullet for anything you must never claim (a skill you do not have, a licence you do not hold). Documents are checked against this list."},
}

// ProfileSkeleton is the starter master-profile.md: the Contact table, then each expected
// section under its heading with its how-to as a Markdown comment, which no reader treats
// as content.
func ProfileSkeleton(contact map[string]string) string {
	var b strings.Builder
	b.WriteString(contactMarkdown(contact))
	for _, g := range ProfileGuide {
		b.WriteString("\n## " + g.Heading + "\n\n<!-- " + g.How + " -->\n")
	}
	return b.String()
}

var htmlComment = regexp.MustCompile(`(?s)<!--(.*?)-->`)

// splitComments returns body without its HTML comments, and the comments' text joined,
// so a how-to left in the starter file is neither content nor lost.
func splitComments(body string) (content, hint string) {
	var hints []string
	content = htmlComment.ReplaceAllStringFunc(body, func(c string) string {
		hints = append(hints, strings.TrimSpace(htmlComment.FindStringSubmatch(c)[1]))
		return ""
	})
	return strings.TrimSpace(content), strings.Join(hints, " ")
}
