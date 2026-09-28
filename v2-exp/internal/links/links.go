// Package links finds the wikilinks in a document and rewrites them. Lint, change
// validation, and every rename read links through it, so all three agree on what a link
// is: a [[target]] outside code, with an optional #heading, ^block, and |alias.
package links

import (
	"regexp"
	"strings"
)

// Link is one wikilink in a text.
type Link struct {
	// Target is the file the link names, without heading or alias: "DINOv2", or a path.
	Target string
	// Fragment is "#heading" or "#^block", or "".
	Fragment string
	// Alias is the text after |, or "".
	Alias string
	Embed bool
	// Start and End are the byte offsets of the whole link in the text.
	Start, End int
	// Line is 1-based.
	Line int
}

var (
	wikiLink   = regexp.MustCompile(`(!)?\[\[([^\[\]\r\n]+?)\]\]`)
	fenceOpen  = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})")
	inlineCode = regexp.MustCompile("`+[^`\n]*`+")
	comment    = regexp.MustCompile(`(?s)%%.*?%%|<!--.*?-->`)
)

// Mask blanks the parts of a text that hold no links: code fences, inline code, and
// comments. Offsets and lines stay where they were.
func Mask(text string) string {
	lines := strings.SplitAfter(text, "\n")
	fence := ""
	for i, l := range lines {
		trimmed := strings.TrimRight(l, "\r\n")
		if m := fenceOpen.FindStringSubmatch(trimmed); m != nil {
			switch {
			case fence == "":
				fence = m[1]
				lines[i] = blank(l)
				continue
			case strings.HasPrefix(strings.TrimSpace(trimmed), fence[:1]) && len(strings.TrimSpace(trimmed)) >= len(fence) && strings.Trim(strings.TrimSpace(trimmed), fence[:1]) == "":
				fence = ""
				lines[i] = blank(l)
				continue
			}
		}
		if fence != "" {
			lines[i] = blank(l)
			continue
		}
		lines[i] = inlineCode.ReplaceAllStringFunc(l, blank)
	}
	out := strings.Join(lines, "")
	return comment.ReplaceAllStringFunc(out, blank)
}

// blank replaces every character but newlines with spaces, byte for byte.
func blank(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c != '\n' && c != '\r' {
			b[i] = ' '
		}
	}
	return string(b)
}

// Find lists the wikilinks of text outside code and comments.
func Find(text string) []Link {
	masked := Mask(text)
	var out []Link
	for _, m := range wikiLink.FindAllStringSubmatchIndex(masked, -1) {
		inner := text[m[4]:m[5]]
		l := Parse(inner)
		l.Embed = m[2] >= 0
		l.Start, l.End = m[0], m[1]
		l.Line = strings.Count(text[:m[0]], "\n") + 1
		out = append(out, l)
	}
	return out
}

// Parse reads the inside of a wikilink: target#fragment|alias.
func Parse(inner string) Link {
	var l Link
	inner, l.Alias, _ = strings.Cut(inner, "|")
	target, frag, hasFrag := strings.Cut(inner, "#")
	l.Target = strings.TrimSpace(target)
	if hasFrag {
		l.Fragment = "#" + frag
	}
	return l
}

// String writes the link back.
func (l Link) String() string {
	s := "[[" + l.Target + l.Fragment
	if l.Alias != "" {
		s += "|" + l.Alias
	}
	s += "]]"
	if l.Embed {
		s = "!" + s
	}
	return s
}

// Key is how a title or link target compares: without case, without a .md extension,
// without surrounding space, and with its path.
func Key(target string) string {
	t := strings.TrimSpace(target)
	if strings.HasSuffix(strings.ToLower(t), ".md") {
		t = t[:len(t)-3]
	}
	return strings.ToLower(t)
}

// BaseKey is Key of the last path element, which is how a bare link finds its file.
func BaseKey(target string) string {
	k := Key(target)
	if i := strings.LastIndex(k, "/"); i >= 0 {
		k = k[i+1:]
	}
	return k
}

// Rename maps a title to its new title; an empty value means the link stays.
type Rename map[string]string

// Rewrite replaces every wikilink of text whose target is a key of r with its new
// title, keeping the fragment, the alias, and the embed mark. A link with a path keeps
// its folder. It returns the new text and how many links changed.
func Rewrite(text string, r Rename) (string, int) {
	if len(r) == 0 {
		return text, 0
	}
	keys := make(map[string]string, len(r))
	for old, title := range r {
		keys[Key(old)] = title
	}
	found := Find(text)
	if len(found) == 0 {
		return text, 0
	}
	var b strings.Builder
	last, n := 0, 0
	for _, l := range found {
		title, ok := keys[BaseKey(l.Target)]
		if !ok || title == "" {
			continue
		}
		next := l
		if i := strings.LastIndex(l.Target, "/"); i >= 0 {
			next.Target = l.Target[:i+1] + title
		} else {
			next.Target = title
		}
		b.WriteString(text[last:l.Start])
		b.WriteString(next.String())
		last = l.End
		n++
	}
	b.WriteString(text[last:])
	return b.String(), n
}

// RewriteValue rewrites one frontmatter value that is a wikilink, and reports whether it
// changed.
func RewriteValue(v string, r Rename) (string, bool) {
	out, n := Rewrite(v, r)
	return out, n > 0
}
