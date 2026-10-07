package doc

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// NonNil is list, or an empty list for nil, so JSON writes [] and not null.
func NonNil(list []string) []string {
	if list == nil {
		return []string{}
	}
	return list
}

// OneLine is s on one line with its spaces folded, cut to n runes with an ellipsis.
func OneLine(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len([]rune(s)) > n {
		s = string([]rune(s)[:n-1]) + "…"
	}
	return s
}

// Plural is one when n is 1, and many otherwise.
func Plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// Capital is s with its first byte in upper case.
func Capital(s string) string {
	if s == "" {
		return s
	}
	r, n := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[n:]
}

// LineCount is the number of lines in s, not counting trailing newlines.
func LineCount(s string) int {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}
