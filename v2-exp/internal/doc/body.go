package doc

import (
	"regexp"
	"strings"
)

// OwnedLeads are the callout types whose leading callout belongs to code. Any other
// callout is the user's, and code never replaces it.
var OwnedLeads = map[string]bool{
	"stub": true, "spec": true, "task": true, "receipt": true, "killed": true,
	"session": true, "change": true,
}

var calloutOpen = regexp.MustCompile(`^>\s*\[!([A-Za-z0-9_-]+)\][+-]?`)

// LeadType is the type of the callout the body opens with, or "".
func LeadType(body string) string {
	rest := strings.TrimLeft(body, "\r\n")
	line, _, _ := strings.Cut(rest, "\n")
	if m := calloutOpen.FindStringSubmatch(line); m != nil {
		return strings.ToLower(m[1])
	}
	return ""
}

// splitLead cuts a body into its opening owned callout and the rest. lead is "" when the
// body does not open with a callout code owns.
func splitLead(body string) (lead, rest string) {
	trimmed := strings.TrimLeft(body, "\r\n")
	if !OwnedLeads[LeadType(trimmed)] {
		return "", body
	}
	lines := strings.SplitAfter(trimmed, "\n")
	i := 0
	for i < len(lines) && strings.HasPrefix(lines[i], ">") {
		i++
	}
	return strings.Join(lines[:i], ""), strings.Join(lines[i:], "")
}

// StripLead is the body without its code-owned lead callout.
func StripLead(body string) string {
	_, rest := splitLead(body)
	return strings.TrimLeft(rest, "\r\n")
}

// Lead is the code-owned callout the body opens with, or "".
func Lead(body string) string {
	lead, _ := splitLead(body)
	return strings.TrimRight(lead, "\n")
}

// ReplaceLead puts lead at the top of content's body, in place of a code-owned callout
// already there. A callout of any other type stays, below the new one.
func ReplaceLead(content, lead string) string {
	front, body, ok := Split(content)
	rest := StripLead(body)
	out := "\n" + strings.TrimRight(lead, "\n") + "\n"
	if rest != "" {
		out += "\n" + rest
	}
	if !ok {
		return strings.TrimLeft(out, "\n")
	}
	return Join(front, out)
}

// Callout builds a callout from its type, title, and lines.
func Callout(kind, title string, lines ...string) string {
	var b strings.Builder
	b.WriteString("> [!" + kind + "] " + title)
	for _, l := range lines {
		b.WriteString("\n> " + l)
	}
	return b.String()
}

var (
	headingLine = regexp.MustCompile(`^(#{1,6})[ \t]+(.+?)[ \t]*#*[ \t]*$`)
	fenceLine   = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})")
)

// Heading is one heading of a body, outside code fences.
type Heading struct {
	Level int
	Title string
	// Line is the heading's index among the body's lines.
	Line int
}

// Headings lists the body's headings outside code fences.
func Headings(body string) []Heading {
	var out []Heading
	fence := ""
	for i, l := range strings.Split(body, "\n") {
		l = strings.TrimRight(l, "\r")
		if m := fenceLine.FindStringSubmatch(l); m != nil {
			switch {
			case fence == "":
				fence = m[1]
			case strings.HasPrefix(strings.TrimSpace(l), fence) && strings.Trim(strings.TrimSpace(l), fence[:1]) == "":
				fence = ""
			}
			continue
		}
		if fence != "" {
			continue
		}
		if m := headingLine.FindStringSubmatch(l); m != nil {
			out = append(out, Heading{Level: len(m[1]), Title: m[2], Line: i})
		}
	}
	return out
}

// sectionSpan finds the lines of the section under heading at level: from the line after
// the heading to the next heading of the same or a higher level. It returns -1 when the
// body has no such heading.
func sectionSpan(lines []string, hs []Heading, level int, title string) (int, int, int) {
	for i, h := range hs {
		if h.Level != level || !strings.EqualFold(h.Title, title) {
			continue
		}
		end := len(lines)
		for _, next := range hs[i+1:] {
			if next.Level <= level {
				end = next.Line
				break
			}
		}
		return h.Line, h.Line + 1, end
	}
	return -1, -1, -1
}

// Section is the text under the level-two heading title, trimmed, and whether the body
// has the heading.
func Section(body, title string) (string, bool) {
	lines := strings.Split(body, "\n")
	_, start, end := sectionSpan(lines, Headings(body), 2, title)
	if start < 0 {
		return "", false
	}
	return strings.TrimSpace(strings.Join(lines[start:end], "\n")), true
}

// SetSection replaces the text under the level-two heading title, or appends the heading
// and text at the end when the body has none.
func SetSection(body, title, text string) string {
	lines := strings.Split(body, "\n")
	_, start, end := sectionSpan(lines, Headings(body), 2, title)
	text = strings.TrimSpace(text)
	if start < 0 {
		out := strings.TrimRight(body, "\n")
		if out != "" {
			out += "\n\n"
		}
		out += "## " + title + "\n"
		if text != "" {
			out += "\n" + text + "\n"
		}
		return out
	}
	block := []string{""}
	if text != "" {
		block = append(block, strings.Split(text, "\n")...)
		block = append(block, "")
	}
	if end == len(lines) {
		// The last section ends the file with one newline.
		block = block[:len(block)-1]
		if text == "" {
			block = nil
		}
		out := append(append([]string{}, lines[:start]...), block...)
		return strings.TrimRight(strings.Join(out, "\n"), "\n") + "\n"
	}
	out := append(append([]string{}, lines[:start]...), block...)
	out = append(out, lines[end:]...)
	return strings.Join(out, "\n")
}

// AppendSection adds line at the end of the level-two section title, creating it when
// the body has none.
func AppendSection(body, title, line string) string {
	have, _ := Section(body, title)
	if have == "" {
		return SetSection(body, title, line)
	}
	return SetSection(body, title, have+"\n"+line)
}

// FirstLine is the first line of text that is not blank, trimmed.
func FirstLine(text string) string {
	for _, l := range strings.Split(text, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			return l
		}
	}
	return ""
}

// LastLine is the last line of text that is not blank, trimmed.
func LastLine(text string) string {
	lines := strings.Split(text, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if l := strings.TrimSpace(lines[i]); l != "" {
			return l
		}
	}
	return ""
}
