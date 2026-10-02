package change

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/schema"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

// Statuses of a change document.
const (
	Proposed   = "proposed"
	Applying   = vault.Applying
	Applied    = "applied"
	Rejected   = "rejected"
	Superseded = "superseded"
	Undone     = "undone"
)

// rewriteNote marks a modify the link or tag rewrite pass made.
const rewriteNote = "Rewrite only."

// Counts are the writes of a change by op, and the files its rewrites reach.
type Counts struct {
	Create       int `json:"create"`
	Modify       int `json:"modify"`
	Promote      int `json:"promote"`
	Rename       int `json:"rename"`
	Remove       int `json:"remove"`
	Confirm      int `json:"confirm"`
	Retag        int `json:"retag"`
	LinkRewrites int `json:"link_rewrites"`
	TagRewrites  int `json:"tag_rewrites"`
}

// Writes is the number of writes that change a document.
func (c Counts) Writes() int {
	return c.Create + c.Modify + c.Promote + c.Rename + c.Remove + c.Confirm + c.Retag
}

func (c Counts) pairs() []struct {
	n    int
	name string
} {
	return []struct {
		n    int
		name string
	}{{c.Create, "create"}, {c.Modify, "modify"}, {c.Promote, "promote"}, {c.Rename, "rename"}, {c.Remove, "remove"}, {c.Confirm, "confirm"}, {c.Retag, "retag"}, {c.LinkRewrites, "link rewrites"}, {c.TagRewrites, "tag rewrites"}}
}

// String is the counts as the frontmatter holds them.
func (c Counts) String() string {
	var parts []string
	for _, p := range c.pairs() {
		parts = append(parts, fmt.Sprintf("%d %s", p.n, p.name))
	}
	return strings.Join(parts, ", ")
}

// short is the counts that are not zero, for the callout.
func (c Counts) short() string {
	var parts []string
	for _, p := range c.pairs() {
		if p.n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", p.n, p.name))
		}
	}
	if len(parts) == 0 {
		return "no writes"
	}
	return strings.Join(parts, ", ")
}

var countsPattern = regexp.MustCompile(`(\d+) (create|modify|promote|rename|remove|confirm|retag|link rewrites|tag rewrites)`)

// ParseCounts reads the counts field.
func ParseCounts(s string) Counts {
	var c Counts
	for _, m := range countsPattern.FindAllStringSubmatch(s, -1) {
		n, _ := strconv.Atoi(m[1])
		switch m[2] {
		case "create":
			c.Create = n
		case "modify":
			c.Modify = n
		case "promote":
			c.Promote = n
		case "rename":
			c.Rename = n
		case "remove":
			c.Remove = n
		case "confirm":
			c.Confirm = n
		case "retag":
			c.Retag = n
		case "link rewrites":
			c.LinkRewrites = n
		case "tag rewrites":
			c.TagRewrites = n
		}
	}
	return c
}

func countOps(ops []*op, outside []vault.Rewrite) Counts {
	var c Counts
	for _, o := range ops {
		switch o.Kind {
		case OpCreate:
			c.Create++
		case OpModify:
			c.Modify++
		case OpPromote:
			c.Promote++
		case OpRename:
			c.Rename++
		case OpRemove:
			c.Remove++
		case OpConfirm:
			c.Confirm++
		case OpRetag:
			c.Retag++
		}
	}
	for _, rw := range outside {
		if isTagRewrite(rw) {
			c.TagRewrites++
		} else {
			c.LinkRewrites++
		}
	}
	return c
}

// isTagRewrite reports whether a rewrite came from a retag: its links name a tag.
func isTagRewrite(rw vault.Rewrite) bool {
	for _, l := range rw.Links {
		if !strings.HasPrefix(l, "#") {
			return false
		}
	}
	return len(rw.Links) > 0
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
		line = "Review the documents below. Edit any of them here if you want. Then say yes in the chat, or press Apply."
	case Applying:
		line = "Apply stopped halfway. The next write of any kind puts the documents back and sets this change to proposed."
	case Applied:
		line = "Applied " + applied + ". Undo takes it back while none of its documents changed since."
	case Rejected:
		line = "Rejected: " + reason
	case Superseded:
		line = "A later change replaced this one."
	case Undone:
		line = "Undone. The documents are back as they were before it."
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

// heading is the Writes heading of an op. It names the document by id and holds no
// wikilink, so a document renamed or removed later leaves no dead link in the record.
func (o *op) heading() string {
	switch o.Kind {
	case OpCreate:
		typ := o.Type
		if o.TopicKind != "" {
			typ += " " + o.TopicKind
		}
		return fmt.Sprintf("### create · %s · %s · %s", typ, o.Title, o.ID)
	case OpModify:
		return fmt.Sprintf("### modify · %s · %s · base %s", o.Title, o.ID, doc.Short(o.Base))
	case OpPromote:
		to := "topic " + o.TopicKind
		if o.NewTitle != "" {
			return fmt.Sprintf("### promote · %s → %s · %s · %s · base %s", o.Title, o.NewTitle, to, o.ID, doc.Short(o.Base))
		}
		return fmt.Sprintf("### promote · %s · %s · %s · base %s", o.Title, to, o.ID, doc.Short(o.Base))
	case OpRename:
		return fmt.Sprintf("### rename · %s → %s · %s · base %s", o.Title, o.NewTitle, o.ID, doc.Short(o.Base))
	case OpRemove:
		h := fmt.Sprintf("### remove · %s · %s · base %s", o.Title, o.ID, doc.Short(o.Base))
		if o.Redirect != "" {
			h += " · redirect " + o.Redirect
		}
		return h
	case OpConfirm:
		return fmt.Sprintf("### confirm · %s · %s · base %s", o.Title, o.ID, doc.Short(o.Base))
	case OpRetag:
		return fmt.Sprintf("### retag · %s → %s · %d %s", o.From, o.To, o.Files, plural(o.Files, "file", "files"))
	}
	return ""
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// renderDocument writes a change document.
func renderDocument(p *planned, id string, now time.Time) string {
	titles := make([]string, len(p.Absorbs))
	for i, d := range p.Absorbs {
		titles[i] = vault.Title(d)
	}
	work, supersedes := "", ""
	if p.Work != nil {
		work = doc.Link(vault.Title(p.Work))
	}
	if p.Supersedes != nil {
		supersedes = doc.Link(vault.Title(p.Supersedes))
	}
	counts := countOps(p.Ops, p.Outside)
	stamp := vault.Stamp(now)
	fields := []doc.Field{
		{Key: "id", Value: id},
		{Key: "type", Value: "change"},
		{Key: "created", Value: stamp},
		{Key: "updated", Value: stamp},
		{Key: "status", Value: Proposed},
		{Key: "absorbs", Value: doc.Links(titles)},
		{Key: "work", Value: work},
		{Key: "proposed", Value: stamp},
		{Key: "session", Value: ""},
		{Key: "counts", Value: counts.String()},
		{Key: "new_tags", Value: nonNil(p.NewTags)},
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
	b.WriteString(renderWrites(p.Ops, p.Outside))
	return doc.Render(fields, b.String())
}

// renderWrites is the Writes section's body: one heading per op, then the files the link
// and tag rewrites reach.
func renderWrites(ops []*op, outside []vault.Rewrite) string {
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
	var linkRW, tagRW []vault.Rewrite
	for _, rw := range outside {
		if isTagRewrite(rw) {
			tagRW = append(tagRW, rw)
		} else {
			linkRW = append(linkRW, rw)
		}
	}
	if len(linkRW) > 0 {
		b.WriteString("\n### link rewrites\n\n")
		for _, rw := range linkRW {
			var pairs []string
			for _, l := range rw.Links {
				pairs = append(pairs, "`[["+l+"]]`")
			}
			fmt.Fprintf(&b, "- %s: %s\n", rw.Title, strings.Join(pairs, ", "))
		}
	}
	if len(tagRW) > 0 {
		b.WriteString("\n### tag rewrites\n\n")
		for _, rw := range tagRW {
			fmt.Fprintf(&b, "- %s\n", rw.Title)
		}
	}
	return b.String()
}

// parseWrites reads the ops of a change document's Writes section. The content of a
// create, a modify, or a promote is the text inside its fence, which the user may have
// edited.
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
			if t := strings.TrimSpace(h); t == "link rewrites" || t == "tag rewrites" {
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
	if len(parts) < 2 {
		return nil, fmt.Errorf("the heading %q is not a write", h)
	}
	o := &op{Kind: parts[0]}
	if o.Kind == OpRetag {
		from, to, ok := strings.Cut(parts[1], " → ")
		if !ok {
			return nil, fmt.Errorf("the heading %q names no tags", h)
		}
		o.From, o.To = strings.TrimSpace(from), strings.TrimSpace(to)
		if len(parts) > 2 {
			fmt.Sscanf(parts[2], "%d", &o.Files)
		}
		return o, nil
	}
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
	if len(rest) < 2 || !schema.IDPattern.MatchString(rest[len(rest)-1]) {
		return nil, fmt.Errorf("the heading %q names no document id", h)
	}
	o.ID = rest[len(rest)-1]
	rest = rest[:len(rest)-1]
	switch o.Kind {
	case OpCreate:
		if len(rest) < 2 {
			return nil, fmt.Errorf("the heading %q names no type and title", h)
		}
		typ := strings.Fields(rest[0])
		o.Type = typ[0]
		if len(typ) > 1 {
			o.TopicKind = typ[1]
		}
		o.Title = strings.Join(rest[1:], " · ")
	case OpModify, OpRemove, OpConfirm:
		o.Title = strings.Join(rest, " · ")
	case OpPromote:
		if len(rest) < 2 {
			return nil, fmt.Errorf("the heading %q names no kind", h)
		}
		kind := strings.Fields(rest[len(rest)-1])
		if len(kind) != 2 || kind[0] != "topic" {
			return nil, fmt.Errorf("the heading %q names no topic kind", h)
		}
		o.Type, o.TopicKind = "topic", kind[1]
		titles := strings.Join(rest[:len(rest)-1], " · ")
		if i := strings.LastIndex(titles, " → "); i >= 0 {
			o.Title, o.NewTitle = titles[:i], titles[i+len(" → "):]
		} else {
			o.Title = titles
		}
	case OpRename:
		titles := strings.Join(rest, " · ")
		i := strings.LastIndex(titles, " → ")
		if i < 0 {
			return nil, fmt.Errorf("the heading %q names no new title", h)
		}
		o.Title, o.NewTitle = titles[:i], titles[i+len(" → "):]
	default:
		return nil, fmt.Errorf("the heading %q is not a create, modify, promote, rename, remove, confirm, or retag", h)
	}
	// A title a create, a rename, or a promote takes becomes a path, so a heading edited
	// by hand cannot name one the title check refuses.
	taken := []string{o.NewTitle}
	if o.Kind == OpCreate {
		taken = append(taken, o.Title)
	}
	for _, t := range taken {
		if t == "" {
			continue
		}
		if clean := doc.CleanTitle(t); clean != t {
			return nil, fmt.Errorf("the heading %q takes the title %q, which holds characters a title cannot; propose the change again", h, t)
		}
		if err := doc.CheckTitle(t); err != nil {
			return nil, fmt.Errorf("the heading %q: %w; propose the change again", h, err)
		}
	}
	return o, nil
}

// setStatus rewrites a change document's status, its lead callout, and extra fields.
func setStatus(content, status string, extra ...doc.Field) string {
	content = doc.SetField(content, "status", status)
	content = doc.SetFields(content, extra)
	d := doc.Parse("", []byte(content))
	var absorbs []string
	for _, a := range d.List("absorbs") {
		absorbs = append(absorbs, doc.LinkTarget(a))
	}
	return doc.ReplaceLead(content, lead(status, ParseCounts(d.Str("counts")), absorbs, d.Str("applied"), d.Str("reason")))
}
