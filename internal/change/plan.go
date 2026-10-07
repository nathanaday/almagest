// Package change is the only way knowledge changes: a source, a repository, or a topic.
// The model proposes a plan; propose validates it and writes a change document; the user
// reads it in the chat or in Obsidian, and may edit it; apply reads the document again,
// validates it again, and makes one git commit. Undo restores the paths of one applied
// change.
package change

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/derive"
	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/gitx"
	"github.com/nathanaday/atlas-obsidian/internal/links"
	"github.com/nathanaday/atlas-obsidian/internal/schema"
	"github.com/nathanaday/atlas-obsidian/internal/tags"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

// MaxWrites bounds the writes the model gives in one change. Link and tag rewrites do
// not count.
const MaxWrites = 100

// Plan is the model's proposal: the Wiki Change Plan.
type Plan struct {
	Title string `json:"title" jsonschema:"a short name: the file name of the change document and the commit subject"`
	Notes string `json:"notes,omitempty" jsonschema:"what the change does and why, and every skipped subject with its reason; becomes the Notes section"`
	// ID names a running change, which the plan fills in place of a new document.
	ID         string   `json:"id,omitempty" jsonschema:"the running change (a work document from change start) this plan fills; empty: a new change document"`
	Absorbs    []string `json:"absorbs,omitempty" jsonschema:"ids of the sources this change absorbs into the wiki"`
	Supersedes string   `json:"supersedes,omitempty" jsonschema:"a proposed change this one replaces"`
	NewTags    bool     `json:"new_tags,omitempty" jsonschema:"allow tags no document holds, in tagging: known; set it only after the user agreed"`
	Writes     []Write  `json:"writes" jsonschema:"the writes, in order"`
}

// Write is one write of a plan.
type Write struct {
	Op       string         `json:"op" jsonschema:"create, modify, rename, remove, confirm, or retag"`
	Type     string         `json:"type,omitempty" jsonschema:"create: topic or repository"`
	Kind     string         `json:"kind,omitempty" jsonschema:"create of a topic: concept, entity, policy, or overview"`
	Title    string         `json:"title,omitempty" jsonschema:"create: the new document's title; rename: the new title"`
	ID       string         `json:"id,omitempty" jsonschema:"modify, rename, remove, confirm: the document's id or title"`
	Base     string         `json:"base,omitempty" jsonschema:"the hash of the document as read; code records it when omitted"`
	Redirect string         `json:"redirect,omitempty" jsonschema:"remove: the document that links to the removed one now name"`
	Fields   map[string]any `json:"fields,omitempty" jsonschema:"create, modify: the type's fields; links as ids or titles; modify merges them over the document's fields"`
	Body     *string        `json:"body,omitempty" jsonschema:"create, modify: the body below the lead callout; modify replaces the body only when given"`
	From     string         `json:"from,omitempty" jsonschema:"retag: the tag to rename, with every tag below it"`
	To       string         `json:"to,omitempty" jsonschema:"retag: the new tag; an existing tag merges"`
	Why      string         `json:"why,omitempty" jsonschema:"one line: why this write, for the change's summary"`
}

// Ops a plan takes.
const (
	OpCreate  = "create"
	OpModify  = "modify"
	OpRename  = "rename"
	OpRemove  = "remove"
	OpConfirm = "confirm"
	OpRetag   = "retag"
)

// op is one validated write, as a change document records it.
type op struct {
	Kind      string // create, modify, rename, remove, confirm, retag
	ID        string
	Type      string // the type as the change leaves it
	TopicKind string // create: the topic's kind
	Title     string // the document's title; for a rename, the old one
	NewTitle  string // rename
	Path      string // the document's path now; for a create, where it goes
	Base      string // the hash of the document as read
	Content   string // create, modify: the whole new file
	Redirect  string // remove: the id of the document links move to
	From, To  string // retag
	Files     int    // retag: the files it rewrites
	Rewrite   bool   // a modify the link or tag rewrite pass made
	Why       string // the plan's reason for the write, for the summary
	Trash     string // remove: where apply moves the document
}

// writesContent reports whether the op writes a whole file.
func (o *op) writesContent() bool {
	return o.Kind == OpCreate || o.Kind == OpModify
}

// finalPath is where the op's document lands.
func (o *op) finalPath() string {
	if o.NewTitle != "" {
		return vault.DocPath(o.NewTitle)
	}
	return o.Path
}

// planned is a validated change, ready to write as a document or to apply.
type planned struct {
	Title      string
	Notes      string
	Absorbs    []*doc.Doc
	Supersedes *doc.Doc
	Ops        []*op
	// Outside are the files outside the change's writes whose links or tags the change
	// rewrites.
	Outside  []vault.Rewrite
	NewTags  []string
	Warnings []string
}

// Refusal is a plan that breaks a rule. It names the write and the rule.
type Refusal struct {
	Problems []string
}

func (r *Refusal) Error() string {
	return "the change is refused:\n- " + strings.Join(r.Problems, "\n- ")
}

// check gathers the problems of a plan.
type check struct {
	idx      *vault.Index
	now      time.Time
	allowNew bool
	problems []string
	warnings []string
	newTags  []string
	// claimed holds each title key a create or a rename takes, with its op.
	claimed map[string]*op
	// gone holds each title key a rename or a remove frees.
	gone map[string]bool
	// byID holds the ops on each document.
	byID map[string][]*op
	// defines holds each tag a write's document defines, with the op.
	defines map[string]*op
}

func newCheck(idx *vault.Index, now time.Time, allowNew bool) *check {
	return &check{idx: idx, now: now, allowNew: allowNew, claimed: map[string]*op{}, gone: map[string]bool{}, byID: map[string][]*op{}, defines: map[string]*op{}}
}

func (c *check) refuse(format string, args ...any) {
	c.problems = append(c.problems, fmt.Sprintf(format, args...))
}

func (c *check) warn(format string, args ...any) {
	c.warnings = append(c.warnings, fmt.Sprintf(format, args...))
}

// TypeOfLink resolves a link against the vault as the change leaves it: titles the
// change creates or renames to count, titles it removes or renames away do not.
func (c *check) TypeOfLink(target string) (string, error) {
	t := doc.LinkTarget(target)
	key := links.Key(t)
	if o := c.claimed[key]; o != nil {
		return o.Type, nil
	}
	if d := c.idx.ByID(t); d != nil {
		if c.removed(d.ID()) {
			return "", nil
		}
		return d.Type(), nil
	}
	if c.gone[key] {
		return "", nil
	}
	typ, err := c.idx.TypeOfLink(t)
	if err == nil && typ != "" && typ != "file" {
		if d := c.idx.Linked(t); d != nil {
			return d.Type(), nil
		}
	}
	return typ, err
}

func (c *check) removed(id string) bool {
	for _, o := range c.byID[id] {
		if o.Kind == OpRemove {
			return true
		}
	}
	return false
}

// titleOf is the title a link, an id, or a title names once the change applies, or "".
func (c *check) titleOf(value string) (string, string) {
	t := doc.LinkTarget(value)
	if t == "" {
		return "", ""
	}
	if o := c.claimed[links.Key(t)]; o != nil {
		title := o.Title
		if o.NewTitle != "" {
			title = o.NewTitle
		}
		return title, o.Type
	}
	if d := c.idx.ByID(t); d != nil && !c.removed(d.ID()) {
		return c.finalTitle(d), d.Type()
	}
	if c.gone[links.Key(t)] {
		return "", ""
	}
	if d, err := c.idx.Resolve(t); err == nil && !c.removed(d.ID()) {
		return c.finalTitle(d), d.Type()
	}
	if typ, err := c.idx.TypeOfLink(t); err == nil && typ != "" {
		return t, typ
	}
	return "", ""
}

// finalTitle is a document's title after the change's renames.
func (c *check) finalTitle(d *doc.Doc) string {
	for _, o := range c.byID[d.ID()] {
		if o.NewTitle != "" {
			return o.NewTitle
		}
	}
	return vault.Title(d)
}

// takeTitle claims a title for a create or a rename, and refuses one that
// another document or another write holds.
func (c *check) takeTitle(o *op, title string, self string) bool {
	key := links.Key(title)
	if clean := doc.CleanTitle(title); clean != title {
		c.refuse("%s: the title %q holds characters a title cannot (/ \\ : * ? \" < > | [ ] # ^ →, control characters, a leading dot); write it as %q", o.label(), title, clean)
		return false
	}
	if err := doc.CheckTitle(title); err != nil {
		c.refuse("%s: %v", o.label(), err)
		return false
	}
	if vault.ReservedTitle(title) {
		c.refuse("%s: %q begins as a view's title does; choose another", o.label(), title)
		return false
	}
	if other := c.claimed[key]; other != nil && other != o {
		c.refuse("%s: another write of this change also takes the title %q", o.label(), title)
		return false
	}
	allowed := []string{self}
	for _, p := range c.idx.TitleHolders(title) {
		if p == self {
			continue
		}
		if d := c.idx.ByPath(p); d != nil && c.gone[links.Key(vault.Title(d))] && links.Key(vault.Title(d)) == key {
			allowed = append(allowed, p)
			continue
		}
		c.refuse("%s: the title %q is held by %s; titles are unique in the vault", o.label(), title, p)
		return false
	}
	if rel := vault.DocPath(title); c.idx.V != nil && c.idx.V.Occupied(rel, allowed...) {
		c.refuse("%s: the title %q names a file that already exists on disk under another case or Unicode form (%s); choose another title", o.label(), title, c.idx.V.OnDisk(rel))
		return false
	}
	c.claimed[key] = o
	return true
}

func (o *op) label() string {
	switch o.Kind {
	case OpCreate:
		return fmt.Sprintf("create %s %q", o.Type, o.Title)
	case OpRename:
		return fmt.Sprintf("rename %q", o.Title)
	case OpRetag:
		return fmt.Sprintf("retag %s → %s", o.From, o.To)
	}
	return fmt.Sprintf("%s %q (%s)", o.Kind, o.Title, o.ID)
}

// createTypes are the types a change creates. A source comes from capture.
var createTypes = []string{"topic", "repository"}

// knowledge reports whether a type is one a change writes.
func knowledge(typ string) bool {
	t := schema.Get(typ)
	return t != nil && t.Family == schema.Knowledge
}

// validate turns a plan into a planned change, or refuses it.
func validate(idx *vault.Index, p Plan, now time.Time) (*planned, error) {
	c := newCheck(idx, now, p.NewTags)
	out := &planned{Title: doc.CleanTitle(p.Title), Notes: strings.TrimSpace(p.Notes)}
	if out.Title == "" {
		c.refuse("the plan needs a title: a short name for the change")
	} else if err := doc.CheckTitle(out.Title); err != nil {
		c.refuse("the plan's title: %v", err)
	}
	if len(p.Writes) > MaxWrites {
		c.refuse("the plan has %d writes; one change takes at most %d. Split it into several changes", len(p.Writes), MaxWrites)
	}
	for _, a := range p.Absorbs {
		d, err := idx.Resolve(a)
		if err != nil {
			c.refuse("absorbs: %v", err)
			continue
		}
		if d.Type() != "source" {
			c.refuse("absorbs: %s is a %s; a change absorbs sources", vault.Title(d), d.Type())
			continue
		}
		out.Absorbs = append(out.Absorbs, d)
	}
	if strings.TrimSpace(p.Supersedes) != "" {
		d, err := idx.ResolveType(p.Supersedes, "change")
		switch {
		case err != nil:
			c.refuse("supersedes: %v", err)
		case d.Str("status") != Proposed:
			c.refuse("supersedes: %s is %s; only a proposed change can be superseded", vault.Title(d), d.Str("status"))
		}
		out.Supersedes = d
	}
	// First pass: every op on a document that exists, so the titles they free and take
	// are known before a create claims one.
	ops := make([]*op, len(p.Writes))
	for i, w := range p.Writes {
		w.Op = strings.ToLower(strings.TrimSpace(w.Op))
		switch w.Op {
		case OpModify, OpRename, OpRemove, OpConfirm:
			ops[i] = c.existing(w)
		case OpRetag:
			ops[i] = c.retag(w)
		case OpCreate:
		default:
			c.refuse("write %d: op %q; it must be create, modify, rename, remove, confirm, or retag", i+1, w.Op)
		}
	}
	for i, w := range p.Writes {
		if strings.ToLower(strings.TrimSpace(w.Op)) == OpCreate {
			ops[i] = c.create(w)
		}
	}
	for i, w := range p.Writes {
		if ops[i] != nil {
			ops[i].Why = doc.OneLine(strings.TrimSpace(w.Why), 160)
		}
	}
	for i, w := range p.Writes {
		o := ops[i]
		if o == nil {
			continue
		}
		switch o.Kind {
		case OpCreate, OpModify:
			c.content(o, w, idx.ByID(o.ID))
		case OpRemove:
			if w.Redirect != "" {
				title, _ := c.titleOf(w.Redirect)
				r, err := idx.Resolve(w.Redirect)
				switch {
				case err != nil || title == "":
					c.refuse("%s: redirect %q names no document that stays", o.label(), w.Redirect)
				case r.ID() == o.ID:
					c.refuse("%s: a document cannot redirect to itself", o.label())
				default:
					o.Redirect = r.ID()
				}
			}
		}
	}
	for _, o := range ops {
		if o != nil {
			out.Ops = append(out.Ops, o)
		}
	}
	if len(c.problems) > 0 {
		return nil, &Refusal{Problems: c.problems}
	}
	c.rewrites(out)
	c.deadLinks(out)
	out.Warnings = c.warnings
	out.NewTags = c.newTags
	if len(c.problems) > 0 {
		return nil, &Refusal{Problems: c.problems}
	}
	return out, nil
}

// existing validates an op on a document that exists.
func (c *check) existing(w Write) *op {
	d, err := c.idx.Resolve(w.ID)
	if err != nil {
		c.refuse("%s: %v", w.Op, err)
		return nil
	}
	o := &op{Kind: w.Op, ID: d.ID(), Type: d.Type(), Title: vault.Title(d), Path: d.Path}
	if !knowledge(d.Type()) {
		c.refuse("%s: %s is a %s; a change writes sources, repositories, and topics", o.label(), o.Title, d.Type())
		return nil
	}
	for _, other := range c.byID[o.ID] {
		if other.Kind == o.Kind || other.Kind == OpRemove || o.Kind == OpRemove {
			c.refuse("%s: the plan already has a %s of this document; one document takes one write of each kind, and a remove alone", o.label(), other.Kind)
			return nil
		}
	}
	current := BaseHash(d)
	o.Base = current
	if w.Base != "" {
		if !sameBase(w.Base, d) {
			c.refuse("%s: conflict: %s changed since you read it (base %s, now %s). Read it again and propose again", o.label(), d.Path, doc.Short(w.Base), doc.Short(current))
			return nil
		}
		o.Base = w.Base
	}
	newTitle := func(raw string) {
		title := doc.CleanTitle(raw)
		if title == "" || title == o.Title {
			return
		}
		o.NewTitle = title
		c.gone[links.Key(o.Title)] = true
		if links.Key(title) != links.Key(o.Title) {
			c.takeTitle(o, title, d.Path)
		} else {
			c.claimed[links.Key(title)] = o
		}
	}
	switch o.Kind {
	case OpRename:
		if doc.CleanTitle(w.Title) == "" {
			c.refuse("%s: a rename needs the new title", o.label())
			return nil
		}
		if doc.CleanTitle(w.Title) == o.Title {
			c.refuse("%s: the new title is the old one", o.label())
			return nil
		}
		newTitle(w.Title)
	case OpRemove:
		c.gone[links.Key(o.Title)] = true
		if d.Type() == "source" {
			c.warn("%s: the captured file %s stays in %s", o.label(), d.Str("file"), vault.Originals)
		}
	}
	c.byID[o.ID] = append(c.byID[o.ID], o)
	return o
}

// retag validates a tag rename.
func (c *check) retag(w Write) *op {
	from, err1 := tags.Normalize(w.From)
	to, err2 := tags.Normalize(w.To)
	o := &op{Kind: OpRetag, From: from, To: to}
	switch {
	case err1 != nil:
		c.refuse("retag: from: %v", err1)
		return nil
	case err2 != nil:
		c.refuse("retag: to: %v", err2)
		return nil
	case from == to:
		c.refuse("%s: the tags are one", o.label())
		return nil
	case !c.idx.TagExists(from):
		c.refuse("%s: no document holds %s", o.label(), from)
		return nil
	case tags.Under(to, from):
		c.refuse("%s: %s lies below %s; a tag cannot move under itself", o.label(), to, from)
		return nil
	}
	if c.idx.TagPage(from) != nil && c.idx.TagPage(to) != nil {
		c.refuse("%s: both tags have a page (%s, %s); remove or merge one page first", o.label(), c.idx.TagPage(from).Title(), c.idx.TagPage(to).Title())
		return nil
	}
	if !c.idx.TagExists(to) && !slices.Contains(c.newTags, to) {
		c.newTags = append(c.newTags, to)
	}
	return o
}

// create validates the identity of a new document: its type, kind, and title.
func (c *check) create(w Write) *op {
	typ := strings.ToLower(strings.TrimSpace(w.Type))
	title := doc.CleanTitle(w.Title)
	o := &op{Kind: OpCreate, Type: typ, Title: title, TopicKind: strings.ToLower(strings.TrimSpace(w.Kind))}
	switch {
	case typ == "source":
		c.refuse("%s: a source comes from the source tool's capture; a change modifies it", o.label())
		return nil
	case !slices.Contains(createTypes, typ):
		c.refuse("create: type %q; a change creates a topic or a repository", w.Type)
		return nil
	case title == "":
		c.refuse("%s: a create needs a title", o.label())
		return nil
	case typ == "topic" && !slices.Contains(schema.TopicKinds, o.TopicKind):
		c.refuse("%s: kind %q; a topic is a concept, an entity, a policy, or an overview", o.label(), w.Kind)
		return nil
	}
	o.Path = vault.DocPath(title)
	o.ID = doc.NewID(schema.DocPrefix, func(id string) bool { return c.idx.ByID(id) != nil })
	c.takeTitle(o, title, "")
	return o
}

// content builds and checks the whole new file of a create or a modify.
func (c *check) content(o *op, w Write, current *doc.Doc) {
	t := schema.Get(o.Type)
	fields, removed := c.fields(o, t, w.Fields)
	stamp := vault.Stamp(c.now)
	var content string
	switch {
	case o.Kind == OpCreate:
		list := []doc.Field{{Key: "id", Value: o.ID}, {Key: "type", Value: o.Type}}
		if o.Type == "topic" {
			list = append(list, doc.Field{Key: "kind", Value: o.TopicKind})
		}
		content = doc.Render(append(list, c.ordered(t, fields, stamp, "")...), normalizeBody(bodyOr(w.Body, skeleton(t, o.TopicKind))))
		if o.Type == "repository" {
			content, _ = derive.Facts(doc.Parse(o.Path, []byte(content)), c.now)
			content = doc.SetField(content, "refreshed", stamp)
		}
	default:
		content = current.Content
		keys := make([]string, 0, len(fields))
		for k := range fields {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool { return fieldOrder(t, keys[i]) < fieldOrder(t, keys[j]) })
		for _, k := range keys {
			content = doc.SetField(content, k, fields[k])
		}
		for _, k := range removed {
			content = doc.RemoveField(content, k)
		}
		if w.Body != nil {
			front, body, _ := doc.Split(content)
			lead := doc.Lead(body)
			out := "\n"
			if lead != "" {
				out += lead + "\n\n"
			}
			content = doc.Join(front, out+normalizeBody(*w.Body))
		}
		if current.Type() == "repository" && fields["unlinked"] == true {
			content = doc.SetField(content, "path", "")
		}
		content = doc.SetFields(content, []doc.Field{{Key: "updated", Value: stamp}, {Key: "refreshed", Value: stamp}})
	}
	o.Content = content
	c.checkPage(o, content)
}

// ordered lists the fields of a new document in the type's order: the common ones, the
// type's own, then any the type does not name.
func (c *check) ordered(t *schema.Type, fields map[string]any, stamp, created string) []doc.Field {
	if created == "" {
		created = stamp
	}
	var list []doc.Field
	for _, f := range t.Fields {
		switch f.Name {
		case "id", "type", "kind":
			continue
		case "created":
			list = append(list, doc.Field{Key: "created", Value: created})
			continue
		case "updated", "refreshed":
			list = append(list, doc.Field{Key: f.Name, Value: stamp})
			continue
		case "tags", "aliases":
			if _, ok := fields[f.Name]; !ok {
				list = append(list, doc.Field{Key: f.Name, Value: []string{}})
				continue
			}
		}
		if v, ok := fields[f.Name]; ok {
			list = append(list, doc.Field{Key: f.Name, Value: v})
		}
	}
	var extra []string
	for k := range fields {
		if t.Field(k) == nil {
			extra = append(extra, k)
		}
	}
	sort.Strings(extra)
	for _, k := range extra {
		list = append(list, doc.Field{Key: k, Value: fields[k]})
	}
	return list
}

func bodyOr(b *string, def string) string {
	if b != nil {
		return *b
	}
	return def
}

// checkPage checks a document's whole content against its type.
func (c *check) checkPage(o *op, content string) {
	d := doc.Parse(o.finalPath(), []byte(content))
	if d.FrontErr != nil {
		c.refuse("%s: the frontmatter does not parse: %v", o.label(), d.FrontErr)
		return
	}
	t := schema.Get(o.Type)
	if d.ID() != o.ID || d.Type() != o.Type {
		c.refuse("%s: id and type are code's; they must stay %s and %s", o.label(), o.ID, o.Type)
		return
	}
	for _, p := range t.Check(schema.Values(d.Front.Map()), c) {
		c.refuse("%s: %s", o.label(), p)
	}
	if o.Type == "repository" && !d.Front.Bool("unlinked") {
		if err := c.repoPath(d.Str("path"), o.ID); err != nil {
			c.refuse("%s: path: %v", o.label(), err)
		}
	}
	if def := strings.ToLower(d.Str("defines")); def != "" {
		if d.Type() == "topic" && d.Str("kind") != "overview" {
			c.refuse("%s: only an overview or a repository defines a tag", o.label())
		}
		if other := c.defines[def]; other != nil && other != o {
			c.refuse("%s: %s also defines %s in this change; one document defines a tag", o.label(), other.label(), def)
		}
		for _, page := range c.idx.TagPages()[def] {
			if page.ID() != o.ID && !c.removed(page.ID()) {
				c.refuse("%s: %s defines %s already; one document defines a tag", o.label(), page.Title(), def)
			}
		}
		c.defines[def] = o
	}
	for _, a := range d.List("aliases") {
		for _, holder := range c.idx.TitleHolders(a) {
			if holder != o.Path {
				c.refuse("%s: the alias %q is held by %s; titles and aliases are unique in the vault", o.label(), a, holder)
			}
		}
		if other := c.claimed[links.Key(a)]; other != nil && other != o {
			c.refuse("%s: the alias %q is a title this change gives to %s", o.label(), a, other.label())
		}
	}
}

// repoPath checks a repository's path: the root of a git work tree, outside the vault,
// that no other repository document holds.
func (c *check) repoPath(p, self string) error {
	if strings.TrimSpace(p) == "" {
		return errors.New("a repository needs the path of its repository, or unlinked: true")
	}
	abs := vault.Expand(p)
	if !strings.HasPrefix(abs, "/") {
		return fmt.Errorf("%s is not absolute; give the full path or ~/…", p)
	}
	if !gitx.IsRoot(abs) {
		return fmt.Errorf("%s is not the root of a git work tree", p)
	}
	if vault.Within(abs, c.idx.V.Root) {
		return fmt.Errorf("%s is inside the vault; a vault never holds another repository", p)
	}
	for _, r := range c.idx.Of("repository") {
		if r.ID() != self && r.Str("path") != "" && !r.Front.Bool("unlinked") && vault.Within(vault.Expand(r.Str("path")), abs) && vault.Within(abs, vault.Expand(r.Str("path"))) {
			return fmt.Errorf("%s is already linked by %s", p, vault.Title(r))
		}
	}
	return nil
}

// fields normalizes the model's fields: code's are dropped with a warning, links become
// wikilinks by title, tags take their form, and values take their field's form. It
// returns the fields to set and the ones to remove.
func (c *check) fields(o *op, t *schema.Type, in map[string]any) (map[string]any, []string) {
	out := map[string]any{}
	var removed []string
	// In key order, so the refusals and the new tags come out the same on every run.
	keys := make([]string, 0, len(in))
	for k := range in {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := in[k]
		if k == "kind" && o.Kind == OpCreate {
			continue
		}
		if k == "kind" {
			c.refuse("%s: kind stays %s; a topic's kind changes through a new topic", o.label(), c.idx.ByID(o.ID).Str("kind"))
			continue
		}
		if f := t.Field(k); f != nil && f.Owner == schema.Code {
			c.warn("%s: %s is code's; the value given was dropped", o.label(), k)
			continue
		}
		if v == nil {
			removed = append(removed, k)
			continue
		}
		f := t.Field(k)
		value, err := c.value(f, v)
		if err != nil {
			c.refuse("%s: %s: %v", o.label(), k, err)
			continue
		}
		if f != nil && k == "path" && o.Type == "repository" {
			if s, ok := value.(string); ok && s != "" {
				value = vault.Shorten(vault.Expand(s))
			}
		}
		out[k] = value
	}
	sort.Strings(removed)
	return out, removed
}

// value gives a field's value its form: a link as [[Title]], a tag normalized, a list as
// strings, a number as an int.
func (c *check) value(f *schema.Field, v any) (any, error) {
	switch x := v.(type) {
	case map[string]any:
		return nil, errors.New("a field holds one line or a list, not a map")
	case float64:
		if x == math.Trunc(x) {
			v = int(x)
		} else {
			v = strconv.FormatFloat(x, 'f', -1, 64)
		}
	}
	if f == nil {
		switch x := v.(type) {
		case []any:
			return stringList(x)
		}
		return v, nil
	}
	switch f.Kind {
	case schema.Link:
		s, err := scalar(v)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(s) == "" {
			return "", nil
		}
		title, _ := c.titleOf(s)
		if title == "" {
			return nil, fmt.Errorf("%s names no document", s)
		}
		return doc.Link(title), nil
	case schema.Links:
		items, err := listOf(v)
		if err != nil {
			return nil, err
		}
		out := make([]string, 0, len(items))
		for _, s := range items {
			title, _ := c.titleOf(s)
			if title == "" {
				return nil, fmt.Errorf("%s names no document that exists or that this change creates", s)
			}
			out = append(out, doc.Link(title))
		}
		return out, nil
	case schema.Tags:
		items, err := listOf(v)
		if err != nil {
			return nil, err
		}
		list, err := tags.NormalizeAll(items)
		if err != nil {
			return nil, err
		}
		for _, t := range list {
			if err := c.tagOK(t); err != nil {
				return nil, err
			}
		}
		return list, nil
	case schema.Tag:
		s, err := scalar(v)
		if err != nil || strings.TrimSpace(s) == "" {
			return s, err
		}
		t, err := tags.Normalize(s)
		if err != nil {
			return nil, err
		}
		return t, c.tagOK(t)
	case schema.List:
		return listOf(v)
	case schema.Int:
		switch x := v.(type) {
		case int:
			return x, nil
		case string:
			n, err := strconv.Atoi(strings.TrimSpace(x))
			if err != nil {
				return nil, fmt.Errorf("%q is not a whole number", x)
			}
			return n, nil
		}
		return nil, fmt.Errorf("%v is not a whole number", v)
	case schema.Bool:
		if b, ok := v.(bool); ok {
			return b, nil
		}
		return nil, fmt.Errorf("%v is not true or false", v)
	}
	return scalar(v)
}

// tagOK records a new tag, and refuses one in tagging: known unless the plan allows it.
func (c *check) tagOK(t string) error {
	if c.idx.TagExists(t) || slices.Contains(c.newTags, t) {
		return nil
	}
	if c.idx.V.Tagging() == "known" && !c.allowNew {
		return fmt.Errorf("the tag %q is new, and this vault uses known tags; use a tag that exists, or ask the user and propose again with new_tags: true", t)
	}
	c.newTags = append(c.newTags, t)
	return nil
}

func scalar(v any) (string, error) {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x), nil
	case int:
		return strconv.Itoa(x), nil
	case bool:
		return strconv.FormatBool(x), nil
	case []any:
		if len(x) == 1 {
			return scalar(x[0])
		}
	}
	return "", fmt.Errorf("%v is not one line of text", v)
}

func listOf(v any) ([]string, error) {
	switch x := v.(type) {
	case []any:
		return stringList(x)
	case []string:
		return x, nil
	case string:
		if strings.TrimSpace(x) == "" {
			return []string{}, nil
		}
		return []string{strings.TrimSpace(x)}, nil
	}
	return nil, fmt.Errorf("%v is not a list", v)
}

func stringList(items []any) ([]string, error) {
	out := make([]string, 0, len(items))
	for _, item := range items {
		s, err := scalar(item)
		if err != nil {
			return nil, err
		}
		if s != "" {
			out = append(out, s)
		}
	}
	return out, nil
}

func fieldOrder(t *schema.Type, k string) int {
	for i, f := range t.Fields {
		if f.Name == k {
			return i
		}
	}
	return len(t.Fields)
}

// skeleton is a new document's body when the plan gives none: the sections of its type
// and kind that the model writes.
func skeleton(t *schema.Type, kind string) string {
	var b strings.Builder
	for _, s := range t.SectionsOf(kind) {
		if slices.Contains(t.CodeSections, s) || s == "Notes" {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString("## " + s + "\n")
	}
	return b.String()
}

func normalizeBody(body string) string {
	body = strings.TrimLeft(body, "\n")
	if body != "" && !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	return body
}

// retitles are the titles the change's renames and removes free, with where links go.
func (out *planned) retitles(c *check) []vault.Retitle {
	var list []vault.Retitle
	for _, o := range out.Ops {
		switch o.Kind {
		case OpRename:
			if o.NewTitle != "" {
				list = append(list, vault.Retitle{Old: o.Title, New: o.NewTitle})
			}
		case OpRemove:
			if o.Redirect == "" {
				continue
			}
			r := c.idx.ByID(o.Redirect)
			list = append(list, vault.Retitle{Old: o.Title, New: c.finalTitle(r), Redirect: r.Type()})
		}
	}
	return list
}

// rewrites runs the link and tag rewrite passes. A knowledge document a pass rewrites
// becomes a modify of the change, or joins the one the plan has; any other file is listed
// as outside.
func (c *check) rewrites(out *planned) {
	contents := map[string]string{}
	skip := map[string]bool{}
	byPath := map[string]*op{}
	for _, o := range out.Ops {
		switch {
		case o.writesContent() && o.Kind != OpCreate:
			contents[o.Path] = o.Content
			byPath[o.Path] = o
		case o.Kind == OpRemove:
			skip[o.Path] = true
		}
	}
	outside := map[string]*vault.Rewrite{}
	var order []string
	take := func(rws []vault.Rewrite) {
		for _, rw := range rws {
			if skip[rw.Path] {
				continue
			}
			contents[rw.Path] = rw.Content
			if o := byPath[rw.Path]; o != nil {
				o.Content = rw.Content
				continue
			}
			d := c.idx.ByPath(rw.Path)
			if d != nil && knowledge(d.Type()) && c.idx.ByID(d.ID()) == d {
				o := &op{Kind: OpModify, ID: d.ID(), Type: d.Type(), Title: vault.Title(d), Path: d.Path, Base: BaseHash(d), Content: rw.Content, Rewrite: true}
				c.byID[o.ID] = append(c.byID[o.ID], o)
				out.Ops = append(out.Ops, o)
				byPath[rw.Path] = o
				continue
			}
			if prev := outside[rw.Path]; prev != nil {
				prev.Content = rw.Content
				prev.Links = append(prev.Links, rw.Links...)
				continue
			}
			copy := rw
			outside[rw.Path] = &copy
			order = append(order, rw.Path)
		}
	}
	if retitles := out.retitles(c); len(retitles) > 0 {
		rws, warnings := vault.Rewrites(c.idx, retitles, contents, skip)
		c.warnings = append(c.warnings, warnings...)
		take(rws)
		// Created documents may link the old titles too.
		rename := links.Rename{}
		for _, r := range retitles {
			rename[r.Old] = r.New
		}
		for _, o := range out.Ops {
			if o.Kind == OpCreate {
				o.Content, _ = links.Rewrite(o.Content, rename)
			}
		}
	}
	for _, o := range out.Ops {
		if o.Kind != OpRetag {
			continue
		}
		rws := vault.TagRewrites(c.idx, o.From, o.To, contents)
		o.Files = len(rws)
		take(rws)
		for _, other := range out.Ops {
			if other.Kind == OpCreate {
				other.Content = vault.RetagContent(doc.Parse(other.Path, []byte(other.Content)), other.Content, o.From, o.To)
			}
		}
	}
	for _, p := range order {
		out.Outside = append(out.Outside, *outside[p])
	}
	c.removedLinks(out)
}

// removedLinks warns of the links a remove with no redirect leaves dead.
func (c *check) removedLinks(out *planned) {
	for _, o := range out.Ops {
		if o.Kind != OpRemove || o.Redirect != "" {
			continue
		}
		var from []string
		for _, d := range append(append([]*doc.Doc{}, c.idx.Docs...), c.idx.Notes...) {
			if d.Path == o.Path || c.removed(d.ID()) {
				continue
			}
			for _, l := range links.Find(d.Content) {
				if links.BaseKey(l.Target) == links.Key(o.Title) {
					from = append(from, vault.Title(d))
					break
				}
			}
		}
		if len(from) > 0 {
			sort.Strings(from)
			c.warn("%s: %d documents still link it (%s); give a redirect to keep their links", o.label(), len(from), strings.Join(firstN(from, 5), ", "))
		}
	}
}

func firstN(s []string, n int) []string {
	if len(s) > n {
		return append(s[:n:n], "…")
	}
	return s
}

// deadLinks warns of links in new content that resolve to nothing once the change
// applies. A later change may create the target, so they are warnings.
func (c *check) deadLinks(out *planned) {
	for _, o := range out.Ops {
		if !o.writesContent() || o.Rewrite {
			continue
		}
		d := doc.Parse(o.Path, []byte(o.Content))
		seen := map[string]bool{}
		for _, l := range links.Find(d.Body) {
			if seen[l.Target] {
				continue
			}
			seen[l.Target] = true
			if typ, _ := c.TypeOfLink(l.Target); typ == "" && len(c.idx.LinkPaths(l.Target)) == 0 {
				c.warn("%s: the link [[%s]] resolves to nothing", o.Title, l.Target)
			}
		}
	}
}

// BaseHash is the hash of what a change must find unchanged in a document: the fields
// the model or the user gives, and the prose. It leaves out what code derives (the
// code-owned fields, the lead callout, the code sections), so a sync between a proposal
// and its apply, which may refresh a repository's git facts or a callout, is no conflict.
func BaseHash(d *doc.Doc) string {
	var b strings.Builder
	t := schema.Get(d.Type())
	if d.Front != nil {
		keys := d.Front.Keys()
		sort.Strings(keys)
		for _, k := range keys {
			if t != nil && t.Owned(k) {
				continue
			}
			b.WriteString(k + "=" + strings.Join(d.Front.List(k), "\x1f") + "\n")
		}
	}
	var code []string
	if t != nil {
		code = t.CodeSections
	}
	b.WriteString(doc.ProseHash(d.Body, code))
	return doc.FileHash([]byte(b.String()))
}

// sameBase reports whether a recorded base is the document as it is now. A change
// proposed by 7.x recorded the hash of the whole file.
func sameBase(base string, d *doc.Doc) bool {
	return doc.SameHash(base, BaseHash(d)) || doc.SameHash(base, doc.FileHash([]byte(d.Content)))
}
