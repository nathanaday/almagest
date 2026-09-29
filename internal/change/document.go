package change

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

// Statuses of a change document.
const (
	Proposed   = "proposed"
	Applying   = "applying"
	Applied    = "applied"
	Rejected   = "rejected"
	Superseded = "superseded"
	Undone     = "undone"
)

// rewriteNote marks a modify the link rewrite pass made.
const rewriteNote = "Link rewrite only."

// Counts are the writes of a change by kind.
type Counts struct {
	Create       int `json:"create"`
	Modify       int `json:"modify"`
	Rename       int `json:"rename"`
	Remove       int `json:"remove"`
	LinkRewrites int `json:"link_rewrites"`
}

// Writes is the number of writes that change a page of the wiki.
func (c Counts) Writes() int { return c.Create + c.Modify + c.Rename + c.Remove }

// String is the counts as the frontmatter holds them.
func (c Counts) String() string {
	return fmt.Sprintf("%d create, %d modify, %d rename, %d remove, %d link rewrites", c.Create, c.Modify, c.Rename, c.Remove, c.LinkRewrites)
}

// short is the counts that are not zero, for the callout.
func (c Counts) short() string {
	var parts []string
	for _, p := range []struct {
		n    int
		name string
	}{{c.Create, "create"}, {c.Modify, "modify"}, {c.Rename, "rename"}, {c.Remove, "remove"}, {c.LinkRewrites, "link rewrites"}} {
		if p.n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", p.n, p.name))
		}
	}
	if len(parts) == 0 {
		return "no writes"
	}
	return strings.Join(parts, ", ")
}

var countsPattern = regexp.MustCompile(`(\d+) (create|modify|rename|remove|link rewrites)`)

// ParseCounts reads the counts field.
func ParseCounts(s string) Counts {
	var c Counts
	for _, m := range countsPattern.FindAllStringSubmatch(s, -1) {
		n := 0
		fmt.Sscanf(m[1], "%d", &n)
		switch m[2] {
		case "create":
			c.Create = n
		case "modify":
			c.Modify = n
		case "rename":
			c.Rename = n
		case "remove":
			c.Remove = n
		case "link rewrites":
			c.LinkRewrites = n
		}
	}
	return c
}

func countOps(ops []*op, outside int) Counts {
	c := Counts{LinkRewrites: outside}
	for _, o := range ops {
		switch o.Kind {
		case "create":
			c.Create++
		case "modify":
			c.Modify++
		case "rename":
			c.Rename++
		case "remove":
			c.Remove++
		}
	}
	return c
}

// lead is the change document's lead callout for its status.
func lead(status string, counts Counts, absorbs []string, applied, reason string) string {
	title := strings.ToUpper(status[:1]) + status[1:] + " · " + counts.short()
	if len(absorbs) > 0 {
		title += " · absorbs " + strings.Join(doc.Links(absorbs), ", ")
	}
	var line string
	switch status {
	case Proposed:
		line = "Review the pages below. Edit any of them here if you want. Then say yes in the chat, or press Apply."
	case Applying:
		line = "Apply stopped halfway. The next write of any kind puts the pages back and sets this change to proposed."
	case Applied:
		line = "Applied " + applied + ". Undo takes it back while none of its pages changed since."
	case Rejected:
		line = "Rejected: " + reason
	case Superseded:
		line = "A later change replaced this one."
	case Undone:
		line = "Undone. The pages are back as they were before it."
	}
	return doc.Callout("change", title, line)
}

// fence is a run of backticks longer than any run inside content, so any markdown
// survives inside it.
func fence(content string) string {
	longest, run := 0, 0
	for _, r := range content {
		if r == '`' {
			run++
			longest = max(longest, run)
			continue
		}
		run = 0
	}
	return strings.Repeat("`", max(5, longest+1))
}

// heading is the Writes heading of an op. It names the page by id and holds no wikilink,
// so a page renamed or removed later leaves no dead link in the record.
func (o *op) heading() string {
	switch o.Kind {
	case "create":
		return fmt.Sprintf("### create · %s · %s · %s", o.Type, o.Title, o.ID)
	case "modify":
		return fmt.Sprintf("### modify · %s · %s · base %s", o.Title, o.ID, doc.Short(o.Base))
	case "rename":
		return fmt.Sprintf("### rename · %s → %s · %s · base %s", o.Title, o.NewTitle, o.ID, doc.Short(o.Base))
	case "remove":
		h := fmt.Sprintf("### remove · %s · %s · base %s", o.Title, o.ID, doc.Short(o.Base))
		if o.Redirect != "" {
			h += " · redirect " + o.Redirect
		}
		return h
	}
	return ""
}

// renderDocument writes a change document.
func renderDocument(p *planned, id string, now time.Time) string {
	titles := make([]string, len(p.Absorbs))
	for i, d := range p.Absorbs {
		titles[i] = vault.Title(d)
	}
	thread, supersedes := "", ""
	if p.Thread != nil {
		thread = doc.Link(vault.Title(p.Thread))
	}
	if p.Supersedes != nil {
		supersedes = doc.Link(vault.Title(p.Supersedes))
	}
	counts := countOps(p.Ops, len(p.Outside))
	fields := []doc.Field{
		{Key: "id", Value: id},
		{Key: "type", Value: "change"},
		{Key: "created", Value: vault.Date(now)},
		{Key: "updated", Value: vault.Date(now)},
		{Key: "status", Value: Proposed},
		{Key: "absorbs", Value: doc.Links(titles)},
		{Key: "thread", Value: thread},
		{Key: "proposed", Value: vault.Stamp(now)},
		{Key: "session", Value: ""},
		{Key: "counts", Value: counts.String()},
		{Key: "applied", Value: ""},
		{Key: "supersedes", Value: supersedes},
		{Key: "reason", Value: ""},
	}
	var b strings.Builder
	b.WriteString(lead(Proposed, counts, titles, "", "") + "\n\n")
	b.WriteString("## Notes\n\n")
	if p.Notes != "" {
		b.WriteString(p.Notes + "\n\n")
	}
	b.WriteString("## Absorbed\n\n")
	if len(p.Absorbs) > 0 {
		b.WriteString("| Document | Id | Hash |\n|---|---|---|\n")
		for _, d := range p.Absorbs {
			fmt.Fprintf(&b, "| %s | %s | %s |\n", doc.Link(vault.Title(d)), d.ID(), doc.Short(vault.Hash(d)))
		}
		b.WriteString("\n")
	}
	b.WriteString("## Writes\n")
	b.WriteString(renderWrites(p.Ops, p.Outside, p.Folders))
	return doc.Render(fields, b.String())
}

// movesHeading lists the scope folders a change moves. Apply works the moves out again
// from the pages, so the list is a record for the reader.
const movesHeading = "folder moves"

// renderWrites is the Writes section's body: one heading per op, then the link rewrites
// and the folder moves.
func renderWrites(ops []*op, outside []Rewrite, folders []Move) string {
	var b strings.Builder
	for _, o := range ops {
		b.WriteString("\n" + o.heading() + "\n")
		if o.Rewrite {
			b.WriteString("\n" + rewriteNote + "\n")
		}
		if o.writesContent() {
			f := fence(o.Content)
			b.WriteString("\n" + f + "markdown\n" + strings.TrimRight(o.Content, "\n") + "\n" + f + "\n")
		}
	}
	if len(outside) > 0 {
		b.WriteString("\n### link rewrites\n\n")
		for _, rw := range outside {
			var pairs []string
			for _, l := range rw.Links {
				pairs = append(pairs, "`[["+l+"]]`")
			}
			fmt.Fprintf(&b, "- %s: %s\n", rw.Title, strings.Join(pairs, ", "))
		}
	}
	if len(folders) > 0 {
		b.WriteString("\n### " + movesHeading + "\n\n")
		for _, m := range folders {
			fmt.Fprintf(&b, "- `%s` → `%s`: %d files\n", m.From, m.To, m.Files)
		}
	}
	return b.String()
}

// parseWrites reads the ops of a change document's Writes section. The content of a
// create or a modify is the text inside its fence, which the user may have edited.
func parseWrites(body string) ([]*op, error) {
	var section string
	for _, h := range doc.Headings(body) {
		if h.Level == 2 && strings.EqualFold(h.Title, "Writes") {
			lines := strings.Split(body, "\n")
			section = strings.Join(lines[h.Line+1:], "\n")
			break
		}
	}
	lines := strings.Split(section, "\n")
	var ops []*op
	var cur *op
	for i := 0; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], "\r")
		if strings.HasPrefix(line, "## ") {
			break
		}
		if h, ok := strings.CutPrefix(line, "### "); ok {
			cur = nil
			if t := strings.TrimSpace(h); t == "link rewrites" || t == movesHeading {
				continue
			}
			o, err := parseHeading(h)
			if err != nil {
				return nil, err
			}
			ops = append(ops, o)
			cur = o
			continue
		}
		if cur == nil {
			continue
		}
		if strings.TrimSpace(line) == rewriteNote {
			cur.Rewrite = true
			continue
		}
		if m := fenceOpen.FindStringSubmatch(line); m != nil && cur.writesContent() && cur.Content == "" {
			marker := m[1]
			var content []string
			closed := false
			for i++; i < len(lines); i++ {
				l := strings.TrimRight(lines[i], "\r")
				if strings.TrimSpace(l) == marker {
					closed = true
					break
				}
				content = append(content, lines[i])
			}
			if !closed {
				return nil, fmt.Errorf("the fence of %q does not close", cur.heading())
			}
			cur.Content = strings.Join(content, "\n") + "\n"
		}
	}
	for _, o := range ops {
		if o.writesContent() && strings.TrimSpace(o.Content) == "" {
			return nil, fmt.Errorf("%s has no content", strings.TrimPrefix(o.heading(), "### "))
		}
	}
	return ops, nil
}

var fenceOpen = regexp.MustCompile("^(`{3,})markdown\\s*$")

// parseHeading reads one Writes heading.
func parseHeading(h string) (*op, error) {
	parts := strings.Split(strings.TrimSpace(h), " · ")
	if len(parts) < 3 {
		return nil, fmt.Errorf("the heading %q is not a write", h)
	}
	o := &op{Kind: parts[0]}
	rest := parts[1:]
	// Tokens at the end: redirect, base, then the id.
	for len(rest) > 0 {
		last := rest[len(rest)-1]
		if v, ok := strings.CutPrefix(last, "redirect "); ok {
			o.Redirect = strings.TrimSpace(v)
		} else if v, ok := strings.CutPrefix(last, "base "); ok {
			o.Base = strings.TrimSpace(v)
		} else {
			break
		}
		rest = rest[:len(rest)-1]
	}
	if len(rest) < 2 || !doc.IDPattern.MatchString(rest[len(rest)-1]) {
		return nil, fmt.Errorf("the heading %q names no page id", h)
	}
	o.ID = rest[len(rest)-1]
	rest = rest[:len(rest)-1]
	switch o.Kind {
	case "create":
		if len(rest) < 2 {
			return nil, fmt.Errorf("the heading %q names no type and title", h)
		}
		o.Type = rest[0]
		o.Title = strings.Join(rest[1:], " · ")
	case "modify", "remove":
		o.Title = strings.Join(rest, " · ")
	case "rename":
		titles := strings.Join(rest, " · ")
		i := strings.LastIndex(titles, " → ")
		if i < 0 {
			return nil, fmt.Errorf("the heading %q names no new title", h)
		}
		o.Title, o.NewTitle = titles[:i], titles[i+len(" → "):]
	default:
		return nil, fmt.Errorf("the heading %q is not a create, modify, rename, or remove", h)
	}
	return o, nil
}

// setStatus rewrites a change document's status, its lead callout, and extra fields.
func setStatus(content, status string, extra ...doc.Field) string {
	d := doc.Parse("", []byte(content))
	content = doc.SetField(content, "status", status)
	content = doc.SetFields(content, extra)
	d = doc.Parse("", []byte(content))
	var absorbs []string
	for _, a := range d.List("absorbs") {
		absorbs = append(absorbs, doc.LinkTarget(a))
	}
	return doc.ReplaceLead(content, lead(status, ParseCounts(d.Str("counts")), absorbs, d.Str("applied"), d.Str("reason")))
}
