package change

import (
	"cmp"
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
	Running    = "running"
	Proposed   = "proposed"
	Applying   = vault.Applying
	Applied    = "applied"
	Rejected   = "rejected"
	Superseded = "superseded"
	Undone     = "undone"
)

// Kinds of a work document: a change that starts before its writes are known.
const (
	KindIngest = "ingest"
	KindRepair = "repair"
	KindDraft  = "draft"
)

// Widget is the block the Obsidian plugin renders as the change's buttons: Approve and
// Cancel while it is proposed, its result after. It holds no data; the plugin reads the
// document's frontmatter.
const Widget = "```atlas-change\n```"

// rewriteNote marks a modify the link or tag rewrite pass made.
const rewriteNote = "Rewrite only."

// Counts are the writes of a change by op, and the files its rewrites reach.
type Counts struct {
	Create       int `json:"create"`
	Modify       int `json:"modify"`
	Rename       int `json:"rename"`
	Remove       int `json:"remove"`
	Confirm      int `json:"confirm"`
	Retag        int `json:"retag"`
	LinkRewrites int `json:"link_rewrites"`
	TagRewrites  int `json:"tag_rewrites"`
}

func (c Counts) pairs() []struct {
	n    int
	name string
} {
	return []struct {
		n    int
		name string
	}{{c.Create, "create"}, {c.Modify, "modify"}, {c.Rename, "rename"}, {c.Remove, "remove"}, {c.Confirm, "confirm"}, {c.Retag, "retag"}, {c.LinkRewrites, "link rewrites"}, {c.TagRewrites, "tag rewrites"}}
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

var countsPattern = regexp.MustCompile(`(\d+) (create|modify|rename|remove|confirm|retag|link rewrites|tag rewrites)`)

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
// leadFor is a change document's lead callout, from its frontmatter.
func leadFor(content string) string {
	d := doc.Parse("", []byte(content))
	status := d.Str("status")
	if status == "" {
		status = Proposed
	}
	title := doc.Capital(status)
	if status == Running {
		title += " · " + workLabel(d)
	} else {
		title += " · " + ParseCounts(d.Str("counts")).short()
		var absorbs []string
		for _, a := range d.List("absorbs") {
			absorbs = append(absorbs, doc.LinkTarget(a))
		}
		if len(absorbs) > 0 {
			title += " · absorbs " + strings.Join(doc.Links(absorbs), ", ")
		}
	}
	var line string
	switch status {
	case Running:
		line = "The agent works. Each step appears under Progress. It proposes the writes here, and you decide then."
	case Proposed:
		line = "Review the documents below. Edit any of them here if you want. Then press Approve, or say yes in the chat."
	case Applying:
		line = "Apply stopped halfway. The next write of any kind puts the documents back and sets this change to proposed."
	case Applied:
		line = "Applied " + d.Str("applied") + ". Undo takes it back while none of its documents changed since."
	case Rejected:
		line = "Rejected: " + d.Str("reason")
	case Superseded:
		line = "A later change replaced this one."
	case Undone:
		line = "Undone. The documents are back as they were before it."
	}
	return doc.Callout("change", title, line)
}

// workLabel names a work document's kind and its files: "ingest · 3 files".
func workLabel(d *doc.Doc) string {
	label := cmp.Or(d.Str("kind"), "work")
	if n := len(d.List("files")); n > 0 {
		label += " · " + strconv.Itoa(n) + " " + doc.Plural(n, "file", "files")
	}
	return label
}

// prose is a document's body without its lead callout: what a reader of a modify counts.
func prose(content string) string {
	_, body, _ := doc.Split(content)
	return doc.StripLead(body)
}

// summary is the Summary section: one line per write, with the reason the plan gave.
// It links only the documents the change leaves in place, so no later remove of one
// leaves the record with a link it cannot keep.
func summary(ops []*op, prior before) string {
	var b strings.Builder
	for _, o := range ops {
		var line string
		switch o.Kind {
		case OpCreate:
			kind := o.Type
			if o.TopicKind != "" {
				kind += " (" + o.TopicKind + ")"
			}
			line = fmt.Sprintf("- **create** %s %s", kind, doc.Link(o.Title))
		case OpModify:
			added, removed := diffLines(prose(prior(o)), prose(o.Content))
			line = fmt.Sprintf("- **modify** %s · +%d −%d", doc.Link(cmp.Or(o.NewTitle, o.Title)), added, removed)
			if added == 0 && removed == 0 {
				line = fmt.Sprintf("- **modify** %s · fields only", doc.Link(cmp.Or(o.NewTitle, o.Title)))
			}
			if o.Rewrite {
				line += " · links or tags follow"
			}
		case OpRename:
			line = fmt.Sprintf("- **rename** %s → %s", o.Title, doc.Link(o.NewTitle))
		case OpRemove:
			line = fmt.Sprintf("- **remove** %s (to trash)", o.Title)
		case OpConfirm:
			line = fmt.Sprintf("- **confirm** %s", doc.Link(o.Title))
		case OpRetag:
			line = fmt.Sprintf("- **retag** #%s → #%s · %s", o.From, o.To, strconv.Itoa(o.Files)+" "+doc.Plural(o.Files, "file", "files"))
		}
		if o.Why != "" {
			line += " · " + o.Why
		}
		b.WriteString(line + "\n")
	}
	return b.String()
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
		return fmt.Sprintf("### retag · %s → %s · %d %s", o.From, o.To, o.Files, doc.Plural(o.Files, "file", "files"))
	}
	return ""
}

// renderDocument writes a change document.
func renderDocument(p *planned, id string, now time.Time, base *doc.Doc, prior before) string {
	titles := make([]string, len(p.Absorbs))
	for i, d := range p.Absorbs {
		titles[i] = vault.Title(d)
	}
	supersedes := ""
	if p.Supersedes != nil {
		supersedes = doc.Link(vault.Title(p.Supersedes))
	}
	counts := countOps(p.Ops, p.Outside)
	stamp := vault.Stamp(now)
	created, kind, session := stamp, "", ""
	var files []string
	var keep []string
	if base != nil {
		created = cmp.Or(base.Str("created"), stamp)
		kind, session, files = base.Str("kind"), base.Str("session"), base.List("files")
		for _, name := range []string{"Files", "Progress"} {
			if text, ok := doc.Section(base.Body, name); ok {
				keep = append(keep, "## "+name+"\n\n"+strings.TrimSpace(text)+"\n")
			}
		}
	}
	fields := []doc.Field{
		{Key: "id", Value: id},
		{Key: "type", Value: "change"},
		{Key: "created", Value: created},
		{Key: "updated", Value: stamp},
		{Key: "status", Value: Proposed},
	}
	if kind != "" {
		fields = append(fields, doc.Field{Key: "kind", Value: kind}, doc.Field{Key: "files", Value: doc.NonNil(files)})
	}
	fields = append(fields,
		doc.Field{Key: "absorbs", Value: doc.Links(titles)},
		doc.Field{Key: "proposed", Value: stamp},
		doc.Field{Key: "session", Value: session},
		doc.Field{Key: "counts", Value: counts.String()},
		doc.Field{Key: "new_tags", Value: doc.NonNil(p.NewTags)},
		doc.Field{Key: "applied", Value: ""},
		doc.Field{Key: "supersedes", Value: supersedes},
		doc.Field{Key: "reason", Value: ""},
		doc.Field{Key: "cssclasses", Value: []string{CSSClass}},
	)
	var b strings.Builder
	b.WriteString(Widget + "\n\n")
	b.WriteString("## Summary\n\n" + summary(p.Ops, prior) + "\n")
	for _, k := range keep {
		b.WriteString(k + "\n")
	}
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
	content := doc.Render(fields, b.String())
	return doc.ReplaceLead(content, leadFor(content))
}

// CSSClass styles a change document in Obsidian.
const CSSClass = "atlas-change"

// renderWork is a new work document: running, with its files and an empty Progress.
func renderWork(id, kind string, files []string, now time.Time) string {
	stamp := vault.Stamp(now)
	fields := []doc.Field{
		{Key: "id", Value: id},
		{Key: "type", Value: "change"},
		{Key: "created", Value: stamp},
		{Key: "updated", Value: stamp},
		{Key: "status", Value: Running},
		{Key: "kind", Value: kind},
		{Key: "files", Value: doc.NonNil(files)},
		{Key: "absorbs", Value: []string{}},
		{Key: "session", Value: ""},
		{Key: "counts", Value: ""},
		{Key: "reason", Value: ""},
		{Key: "cssclasses", Value: []string{CSSClass}},
	}
	var b strings.Builder
	b.WriteString(Widget + "\n\n")
	if len(files) > 0 {
		b.WriteString("## Files\n\n")
		for _, f := range files {
			b.WriteString("- `" + f + "`\n")
		}
		b.WriteString("\n")
	}
	b.WriteString("## Progress\n\n## Notes\n\n## Absorbed\n\n## Writes\n")
	content := doc.Render(fields, b.String())
	return doc.ReplaceLead(content, leadFor(content))
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
// create or a modify is the text inside its fence, which the user may have
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
	case OpRename:
		titles := strings.Join(rest, " · ")
		i := strings.LastIndex(titles, " → ")
		if i < 0 {
			return nil, fmt.Errorf("the heading %q names no new title", h)
		}
		o.Title, o.NewTitle = titles[:i], titles[i+len(" → "):]
	default:
		return nil, fmt.Errorf("the heading %q is not a create, modify, rename, remove, confirm, or retag", h)
	}
	// A title a create or a rename takes becomes a path, so a heading edited
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
	return doc.ReplaceLead(content, leadFor(content))
}
