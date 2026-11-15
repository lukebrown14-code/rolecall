package store

import (
	"strings"
	"unicode/utf8"
)

// DisplayText is s made safe to draw in a terminal on one line: tab, newline, carriage
// return and the Unicode line and paragraph separators become a space, and every other
// control character (C0, DEL and C1, ESC among them) is dropped, as are the bidi controls
// and zero-width characters that can reorder or hide text. meta.yml strings often come
// from scraped job ads, and a quoted scalar can decode "\e" or "\u009b", so nothing read
// from one reaches the screen raw.
func DisplayText(s string) string {
	return clean(s, false)
}

// DocumentText is the Markdown of a file made safe to draw in a terminal: the rule of
// DisplayText, except that newlines and tabs are kept so the Markdown still renders. A
// carriage return is dropped (CRLF reads as LF) and a Unicode line or paragraph separator
// becomes a newline. jd.md is the scraped job ad, the least trusted text in the
// Workspace, so no escape sequence in a document reaches the terminal.
func DocumentText(s string) string {
	return clean(s, true)
}

// clean applies the display rule; multiline keeps newlines and tabs.
func clean(s string, multiline bool) string {
	if utf8.ValidString(s) && !strings.ContainsFunc(s, unsafeRune) {
		return s
	}
	var out strings.Builder
	out.Grow(len(s))
	// Ranging decodes invalid UTF-8 as U+FFFD, so a raw 8-bit C1 byte such as 0x9b never
	// reaches the terminal either.
	for _, r := range s {
		switch {
		case multiline && (r == '\n' || r == '\t'):
			out.WriteRune(r)
		case multiline && (r == 0x2028 || r == 0x2029):
			out.WriteByte('\n')
		case multiline && r == '\r':
		case r == '\t' || r == '\n' || r == '\r' || r == 0x2028 || r == 0x2029:
			out.WriteByte(' ')
		case unsafeRune(r):
		default:
			out.WriteRune(r)
		}
	}
	return out.String()
}

// unsafeRune is a character that must not reach the terminal as stored: a C0 or C1
// control, DEL, a Unicode line break, a bidi mark, embedding, override or isolate, or a
// zero-width character other than the joiners that shape emoji and scripts (U+200C ZWNJ
// and U+200D ZWJ are kept).
func unsafeRune(r rune) bool {
	switch {
	case r < 0x20 || (r >= 0x7f && r <= 0x9f), r == 0x2028 || r == 0x2029:
		return true
	case r >= 0x202a && r <= 0x202e, r >= 0x2066 && r <= 0x2069, r == 0x200e || r == 0x200f || r == 0x061c:
		return true
	case r == 0x200b || r == 0x2060 || r == 0xfeff:
		return true
	}
	return false
}
