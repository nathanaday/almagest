// Package tags holds the rules of a tag: its form, its place in the tree that / makes, and
// the rewrite of a rename. A document's categories are its tags property; a tag nests
// under the tags its / splits it into, and a document that holds a tag belongs to every
// tag above it.
package tags

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/nathanaday/atlas-obsidian/internal/links"
)

var (
	form      = regexp.MustCompile(`^[a-z0-9-]+(/[a-z0-9-]+)*$`)
	letter    = regexp.MustCompile(`[a-z]`)
	spaces    = regexp.MustCompile(`[\s_]+`)
	dashes    = regexp.MustCompile(`-{2,}`)
	inlineTag = regexp.MustCompile(`(^|[\s(\[,;])#([A-Za-z0-9_/-]*[A-Za-z_][A-Za-z0-9_/-]*)`)
)

// Valid reports whether t is a tag in the form code writes: lower case letters, digits,
// -, and /, with at least one letter, and no empty part.
func Valid(t string) bool {
	return form.MatchString(t) && letter.MatchString(t)
}

// Normalize makes a tag the model or the user gave into the form code writes: lower case,
// spaces and _ made -, a leading # removed. It refuses a tag that is still not valid.
func Normalize(s string) (string, error) {
	t := strings.TrimSpace(s)
	t = strings.TrimPrefix(t, "#")
	t = strings.ToLower(t)
	t = spaces.ReplaceAllString(t, "-")
	t = dashes.ReplaceAllString(t, "-")
	parts := strings.Split(t, "/")
	for i, p := range parts {
		parts[i] = strings.Trim(p, "-")
	}
	t = strings.Join(parts, "/")
	if !Valid(t) {
		return "", fmt.Errorf("%q is no valid tag: a tag is lower case letters, digits, - and /, holds a letter, and has no empty part (work/p3, self-driving)", s)
	}
	return t, nil
}

// NormalizeAll normalizes a list and drops repeats, keeping the order.
func NormalizeAll(list []string) ([]string, error) {
	out := make([]string, 0, len(list))
	var bad []string
	for _, s := range list {
		t, err := Normalize(s)
		if err != nil {
			bad = append(bad, err.Error())
			continue
		}
		if !slices.Contains(out, t) {
			out = append(out, t)
		}
	}
	if len(bad) > 0 {
		return out, fmt.Errorf("%s", strings.Join(bad, "; "))
	}
	return out, nil
}

// Ancestors are the tags above t, the top first: a/b/c gives a and a/b.
func Ancestors(t string) []string {
	parts := strings.Split(t, "/")
	out := make([]string, 0, len(parts)-1)
	for i := 1; i < len(parts); i++ {
		out = append(out, strings.Join(parts[:i], "/"))
	}
	return out
}

// Parent is the tag directly above t, or "".
func Parent(t string) string {
	if i := strings.LastIndex(t, "/"); i >= 0 {
		return t[:i]
	}
	return ""
}

// Leaf is the last part of t.
func Leaf(t string) string { return t[strings.LastIndex(t, "/")+1:] }

// Depth counts the parts of t.
func Depth(t string) int { return strings.Count(t, "/") + 1 }

// Under reports whether t is top or a tag below it.
func Under(t, top string) bool {
	t, top = strings.ToLower(t), strings.ToLower(top)
	return t == top || strings.HasPrefix(t, top+"/")
}

// Holds reports whether a list of tags holds t: it lists t or a tag below t.
func Holds(list []string, t string) bool {
	for _, have := range list {
		if Under(have, t) {
			return true
		}
	}
	return false
}

// HoldsAll reports whether a list holds every tag in want.
func HoldsAll(list, want []string) bool {
	for _, t := range want {
		if !Holds(list, t) {
			return false
		}
	}
	return true
}

// Expand is a list with every tag above each of its tags, without repeats, sorted.
func Expand(list []string) []string {
	seen := map[string]bool{}
	for _, t := range list {
		seen[t] = true
		for _, a := range Ancestors(t) {
			seen[a] = true
		}
	}
	out := make([]string, 0, len(seen))
	for t := range seen {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

// Rename moves t when it is from or lies below it: from a/b to x, a/b/c becomes x/c.
func Rename(t, from, to string) (string, bool) {
	if !Under(t, from) {
		return t, false
	}
	return to + t[len(from):], true
}

// RenameList renames every tag of a list, and keeps one of a tag the rename makes twice.
func RenameList(list []string, from, to string) ([]string, bool) {
	out := make([]string, 0, len(list))
	changed := false
	for _, t := range list {
		n, ok := Rename(t, from, to)
		changed = changed || ok
		if !slices.Contains(out, n) {
			out = append(out, n)
		} else {
			changed = true
		}
	}
	return out, changed
}

// Inline lists the inline tags of a text, outside code and comments, without the #.
func Inline(text string) []string {
	masked := links.Mask(text)
	var out []string
	for _, m := range inlineTag.FindAllStringSubmatch(masked, -1) {
		out = append(out, m[2])
	}
	return out
}

// RenameInline rewrites the inline tags #from and #from/… of a text, outside code and
// comments, and counts them.
func RenameInline(text, from, to string) (string, int) {
	masked := links.Mask(text)
	idx := inlineTag.FindAllStringSubmatchIndex(masked, -1)
	if len(idx) == 0 {
		return text, 0
	}
	var b strings.Builder
	last, n := 0, 0
	for _, m := range idx {
		start, end := m[4], m[5]
		name := text[start:end]
		renamed, ok := Rename(strings.ToLower(name), from, to)
		if !ok {
			continue
		}
		b.WriteString(text[last:start])
		b.WriteString(renamed)
		last = end
		n++
	}
	b.WriteString(text[last:])
	return b.String(), n
}
