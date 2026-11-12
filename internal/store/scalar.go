package store

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// scalarToken splits a scalar from its exact whitespace/comment suffix.
func scalarToken(raw string) (string, string, error) {
	start := len(raw) - len(strings.TrimLeft(raw, " \t"))
	end := len(raw)
	if start < len(raw) && raw[start] == '"' {
		end = -1
		for i := start + 1; i < len(raw); i++ {
			if raw[i] == '\\' {
				i++
				continue
			}
			if raw[i] == '"' {
				end = i + 1
				break
			}
		}
		if end < 0 {
			return "", "", fmt.Errorf("invalid quoted scalar")
		}
		tail := strings.TrimLeft(raw[end:], " \t")
		if tail != "" && !strings.HasPrefix(tail, "#") {
			return "", "", fmt.Errorf("invalid quoted scalar suffix")
		}
	} else {
		for i := start; i < len(raw); i++ {
			if raw[i] == '#' && (i == start || raw[i-1] == ' ' || raw[i-1] == '\t') {
				end = i
				break
			}
		}
		end = len(strings.TrimRight(raw[:end], " \t"))
	}
	if end < start {
		end = start
	}
	return raw[start:end], raw[end:], nil
}
func scalar(raw string) (string, error) {
	token, _, err := scalarToken(raw)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(token, "\"") {
		return decodeQuoted(token)
	}
	if strings.ContainsAny(token, "\t\r") || strings.HasPrefix(token, "{") || strings.HasPrefix(token, "[") || strings.HasPrefix(token, "'") || token == "|" || token == ">" || strings.Contains(token, ": ") {
		return "", fmt.Errorf("invalid scalar")
	}
	return token, nil
}
func decodeQuoted(token string) (string, error) {
	var out strings.Builder
	escapes := map[byte]rune{'0': '\x00', 'a': '\a', 'b': '\b', 't': '\t', 'n': '\n', 'v': '\v', 'f': '\f', 'r': '\r', 'e': '\x1b', ' ': ' ', '"': '"', '/': '/', '\\': '\\', 'N': '\u0085', '_': '\u00a0', 'L': '\u2028', 'P': '\u2029'}
	for i := 1; i < len(token)-1; i++ {
		if token[i] != '\\' {
			out.WriteByte(token[i])
			continue
		}
		i++
		c := token[i]
		if r, ok := escapes[c]; ok {
			out.WriteRune(r)
			continue
		}
		n := 0
		switch c {
		case 'x':
			n = 2
		case 'u':
			n = 4
		case 'U':
			n = 8
		default:
			return "", fmt.Errorf("invalid YAML escape")
		}
		if i+n >= len(token)-1 {
			return "", fmt.Errorf("short YAML escape")
		}
		v, err := strconv.ParseUint(token[i+1:i+1+n], 16, 32)
		if err != nil || !utf8.ValidRune(rune(v)) {
			return "", fmt.Errorf("invalid Unicode escape")
		}
		out.WriteRune(rune(v))
		i += n
	}
	return out.String(), nil
}
func quoteScalar(value string) string {
	var out strings.Builder
	out.WriteByte('"')
	for _, r := range value {
		switch r {
		case '"', '\\':
			out.WriteByte('\\')
			out.WriteRune(r)
		default:
			if r < 0x20 || (r >= 0x7f && r <= 0x9f) || r == 0x2028 || r == 0x2029 {
				fmt.Fprintf(&out, "\\u%04X", r)
			} else {
				out.WriteRune(r)
			}
		}
	}
	out.WriteByte('"')
	return out.String()
}
