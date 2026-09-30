package work

import (
	"fmt"
	"sort"
	"strings"

	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/schema"
	"github.com/nathanaday/atlas-obsidian/internal/tags"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

// WriteFunc writes a file when its content differs, and reports whether it wrote.
type WriteFunc func(rel string, content []byte) (bool, error)

// Sync makes every stub, spec, and event agree with the events: the derived fields, the
// lead callouts, and the sections code writes. It writes a file only when its content
// differs, and never changes updated. It returns the paths it wrote.
func (b *Board) Sync(write WriteFunc) ([]string, error) {
	var out []string
	for _, d := range append(append([]*doc.Doc{}, b.Stubs...), b.Specs...) {
		content := b.Derived(d)
		if content == d.Content {
			continue
		}
		wrote, err := write(d.Path, []byte(content))
		if err != nil {
			return out, err
		}
		if wrote {
			out = append(out, d.Path)
		}
	}
	for _, e := range b.Idx.Of("event") {
		content := b.DerivedEvent(e)
		if content == e.Content {
			continue
		}
		wrote, err := write(e.Path, []byte(content))
		if err != nil {
			return out, err
		}
		if wrote {
			out = append(out, e.Path)
		}
	}
	return out, nil
}

// Derived is a stub's or a spec's content with its derived fields, its lead callout, and
// the sections code writes made current.
func (b *Board) Derived(d *doc.Doc) string {
	content := d.Content
	status := b.Status(d)
	set := []doc.Field{{Key: "status", Value: status}, {Key: "refreshed", Value: b.Refreshed(d)}}
	if d.Type() == "spec" && d.Str("kind") == Plan {
		set = append(set,
			doc.Field{Key: "blocked", Value: b.Blocked(d)},
			doc.Field{Key: "active", Value: b.Active(d)},
			doc.Field{Key: "parts", Value: b.PartsText(d)},
			doc.Field{Key: "root", Value: doc.Link(b.Root(d).Title())},
		)
	}
	for _, f := range set {
		if !d.Front.Equal(f.Key, f.Value) {
			content = doc.SetField(content, f.Key, f.Value)
		}
	}
	content = doc.ReplaceLead(content, b.Lead(d))
	if d.Type() == "spec" {
		front, body, _ := doc.Split(content)
		order := schema.Get("spec").SectionsOf(d.Str("kind"))
		switch d.Str("kind") {
		case Plan:
			body = doc.PutSection(body, "Parts", b.PartsTable(d), order)
			body = doc.PutSection(body, "History", HistoryBase, order)
		case Design:
			body = doc.PutSection(body, "Implemented by", ImplementedBase, order)
		}
		content = doc.Join(front, body)
	}
	return content
}

// DerivedEvent is an event's content with its lead callout current.
func (b *Board) DerivedEvent(e *doc.Doc) string {
	subject := doc.LinkTarget(e.Str("subject"))
	part := ""
	if s := b.Subject(e); s != nil {
		subject = s.Title()
		if s.Type() == "spec" && s.Str("kind") == Plan {
			part = b.Root(s).Title()
		}
	}
	return doc.ReplaceLead(e.Content, EventLead(e, subject, part))
}

// Refreshed is when a stub or a spec was last checked against what it describes: its
// last event, or its creation.
func (b *Board) Refreshed(d *doc.Doc) string {
	if e := b.LastEvent(d); e != nil {
		return e.Str("at")
	}
	return d.Str("created")
}

// Lead is a stub's or a spec's lead callout.
func (b *Board) Lead(d *doc.Doc) string {
	if d.Type() == "stub" {
		return b.stubLead(d)
	}
	if d.Str("kind") == Design {
		return b.designLead(d)
	}
	return b.planLead(d)
}

func (b *Board) stubLead(d *doc.Doc) string {
	switch b.Status(d) {
	case Resolved:
		when := ""
		if e := b.Result(d); e != nil {
			when = " " + day(e.Str("at"))
		}
		became := d.List("became")
		if len(became) == 0 {
			return doc.Callout("stub-resolved", "Resolved"+when)
		}
		return doc.Callout("stub-resolved", "Resolved"+when+" → "+strings.Join(became, ", "))
	case Dropped:
		e := b.Result(d)
		title, lines := "Dropped", []string{}
		if e != nil {
			why, _ := doc.Section(e.Body, "Why")
			title += " " + day(e.Str("at"))
			if first := doc.FirstLine(why); first != "" {
				lines = append(lines, oneLine(first, 160))
			}
			lines = append(lines, "See "+doc.Link(e.Title()))
		}
		return doc.Callout("stub-dropped", title, lines...)
	}
	title := "Open · " + orDefault(d.Str("priority"), "normal") + " · planted " + day(d.Str("created"))
	if s := b.plantedIn(d); s != "" {
		title += " in " + doc.Link(s)
	}
	var lines []string
	if t := tagLine(d); t != "" {
		lines = append(lines, t)
	}
	return doc.Callout("stub", title, lines...)
}

// plantedIn is the first session whose work lists a document, or "".
func (b *Board) plantedIn(d *doc.Doc) string {
	var first *doc.Doc
	for _, s := range b.Idx.Of("session") {
		for _, l := range s.List("work") {
			if strings.EqualFold(doc.LinkTarget(l), d.Title()) && (first == nil || s.Str("started") < first.Str("started")) {
				first = s
			}
		}
	}
	if first == nil {
		return ""
	}
	return first.Title()
}

func (b *Board) planLead(d *doc.Doc) string {
	status := b.Status(d)
	path := ""
	anc := b.Ancestors(d)
	for i := len(anc) - 1; i >= 0; i-- {
		path += doc.Link(anc[i].Title()) + " › "
	}
	path += "**" + d.Title() + "**"
	if repos := d.List("repositories"); len(repos) > 0 {
		path += " · " + strings.Join(repos, ", ")
	}
	switch status {
	case Done, Dropped:
		kind, word := "spec-done", "Done"
		if status == Dropped {
			kind, word = "spec-dropped", "Dropped"
		}
		title := word
		if e := b.Result(d); e != nil {
			title += " " + day(e.Str("at")) + " → " + doc.Link(e.Title()+"|"+map[string]string{Done: "result", Dropped: "reason"}[status])
		}
		return doc.Callout(kind, title, path)
	}
	title := capital(status) + " · plan · " + orDefault(d.Str("priority"), "normal")
	if hs := b.Holders(d); len(hs) > 0 {
		title += " · active in " + doc.Link(hs[0].Title())
	}
	lines := []string{path}
	if blocked := b.Blocked(d); blocked != "" {
		lines = append([]string{"Blocked: " + oneLine(blocked, 160)}, lines...)
	}
	var tail []string
	if deps := d.List("depends"); len(deps) > 0 {
		var parts []string
		for _, dep := range deps {
			label := dep
			if s := b.Idx.Linked(dep); s != nil {
				label += " (" + b.Status(s) + ")"
			}
			parts = append(parts, label)
		}
		tail = append(tail, "Depends on "+strings.Join(parts, ", "))
	}
	if p := b.PartsText(d); p != "" {
		tail = append(tail, "parts "+p)
	}
	if e := b.LastEvent(d); e != nil {
		tail = append(tail, "last event: "+e.Str("kind")+" "+minute(e.Str("at")))
	}
	if len(tail) > 0 {
		lines = append(lines, strings.Join(tail, " · "))
	}
	return doc.Callout("spec", title, lines...)
}

func (b *Board) designLead(d *doc.Doc) string {
	if b.Status(d) == Superseded {
		for _, s := range b.Specs {
			if old := b.Idx.Linked(s.Str("supersedes")); old != nil && old.ID() == d.ID() {
				return doc.Callout("design-superseded", "Superseded by "+doc.Link(s.Title()))
			}
		}
	}
	title := "Current · design"
	if repos := d.List("repositories"); len(repos) > 0 {
		title += " · " + strings.Join(repos, ", ")
	}
	n := 0
	for _, s := range b.Specs {
		for _, l := range s.List("implements") {
			if strings.EqualFold(doc.LinkTarget(l), d.Title()) {
				n++
			}
		}
	}
	if n > 0 {
		title += fmt.Sprintf(" · implemented by %d %s", n, plural(n, "plan", "plans"))
	}
	return doc.Callout("design", title)
}

// PartsTable is the Parts section of a plan with parts: each part in order, with its
// status, repositories, and dependencies.
func (b *Board) PartsTable(d *doc.Doc) string {
	parts := b.Parts(d)
	if len(parts) == 0 {
		return ""
	}
	rows := []string{"| # | Part | Status | Repositories | Depends on |", "|---|---|---|---|---|"}
	for i, p := range parts {
		order := p.Str("order")
		if order == "" {
			order = fmt.Sprint(i + 1)
		}
		deps := make([]string, 0)
		for _, dep := range p.List("depends") {
			deps = append(deps, doc.LinkTarget(dep))
		}
		rows = append(rows, fmt.Sprintf("| %s | %s | %s | %s | %s |", order, doc.Link(p.Title()), b.Status(p), strings.Join(p.List("repositories"), ", "), strings.Join(deps, ", ")))
	}
	return strings.Join(rows, "\n")
}

// HistoryBase is a plan's History section: an inline Base of the events whose subject it
// is, newest first. Obsidian renders it live, so the file does not change with each event.
const HistoryBase = "```base\n" + `filters:
  and:
    - file.inFolder("wiki/documents")
    - 'type == "event"'
    - 'list(subject).contains(this.file.asLink())'
views:
  - type: table
    name: History
    order:
      - file.name
      - kind
      - at
      - session
    sort:
      - property: at
        direction: DESC
` + "```"

// ImplementedBase is a design's Implemented by section: an inline Base of the plans
// whose implements names it.
const ImplementedBase = "```base\n" + `filters:
  and:
    - file.inFolder("wiki/documents")
    - 'type == "spec"'
    - 'list(implements).contains(this.file.asLink())'
views:
  - type: table
    name: Implemented by
    order:
      - file.name
      - status
      - repositories
      - refreshed
` + "```"

func tagLine(d *doc.Doc) string {
	list := d.List("tags")
	out := make([]string, 0, len(list))
	for _, t := range list {
		if tags.Valid(t) {
			out = append(out, "#"+t)
		}
	}
	sort.Strings(out)
	return strings.Join(out, " · ")
}

func day(stamp string) string {
	if t, ok := vault.ParseTime(stamp); ok {
		return vault.Date(t)
	}
	return stamp
}

func minute(stamp string) string {
	if t, ok := vault.ParseTime(stamp); ok {
		return t.Format("2006-01-02 15:04")
	}
	return stamp
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
