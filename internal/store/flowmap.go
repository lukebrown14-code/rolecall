package store

import (
	"fmt"
	"strings"
)

// parseFlow reads a one-line flow map, {key: value, key: "value"}, the shape status_log
// entries use. Values are plain (no comma, brace, bracket or "# ") or double-quoted with
// the scalar escapes. Only text after the closing brace may follow, and only as an inline
// comment; the returned rest is that text, whitespace and all, so a writer keeps it.
func parseFlow(s string) (map[string]string, string, error) {
	bad := func(why string) (map[string]string, string, error) {
		return nil, "", fmt.Errorf("invalid flow map: %s", why)
	}
	s = strings.TrimLeft(s, " \t")
	if !strings.HasPrefix(s, "{") {
		return bad("expected {")
	}
	out := map[string]string{}
	i := 1
	skip := func() {
		for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
			i++
		}
	}
	for {
		skip()
		if i >= len(s) {
			return bad("missing }")
		}
		if s[i] == '}' {
			i++
			break
		}
		start := i
		for i < len(s) && s[i] != ':' && s[i] != ',' && s[i] != '}' {
			i++
		}
		key := strings.TrimSpace(s[start:i])
		if i >= len(s) || s[i] != ':' || !keyPattern.MatchString(key) {
			return bad("expected key:")
		}
		if _, dup := out[key]; dup {
			return bad("duplicate key " + key)
		}
		i++
		skip()
		var value string
		if i < len(s) && s[i] == '"' {
			end := -1
			for j := i + 1; j < len(s); j++ {
				if s[j] == '\\' {
					j++
					continue
				}
				if s[j] == '"' {
					end = j + 1
					break
				}
			}
			if end < 0 {
				return bad("unterminated string")
			}
			v, err := decodeQuoted(s[i:end])
			if err != nil {
				return nil, "", err
			}
			value, i = v, end
		} else {
			start = i
			for i < len(s) && s[i] != ',' && s[i] != '}' {
				i++
			}
			value = strings.TrimSpace(s[start:i])
			if strings.ContainsAny(value, "{[\"'\t") || strings.Contains(value, " #") {
				return bad("unquoted value")
			}
		}
		out[key] = value
		skip()
		if i < len(s) && s[i] == ',' {
			i++
			continue
		}
		if i >= len(s) || s[i] != '}' {
			return bad("expected , or }")
		}
	}
	rest := s[i:]
	if tail := strings.TrimLeft(rest, " \t"); tail != "" && !strings.HasPrefix(tail, "#") {
		return bad("text after }")
	}
	return out, rest, nil
}

// flowEntry renders a flow map in the order given: values quoted, except an `at`; empty
// values left out.
func flowEntry(pairs ...string) string {
	var parts []string
	for i := 0; i+1 < len(pairs); i += 2 {
		switch {
		case pairs[i+1] == "":
		case pairs[i] == "at":
			// A date or date-time is safe plain, as the schema's example shows it.
			parts = append(parts, "at: "+pairs[i+1])
		default:
			parts = append(parts, pairs[i]+": "+quoteScalar(pairs[i+1]))
		}
	}
	return "{" + strings.Join(parts, ", ") + "}"
}
