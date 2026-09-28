// Package change is the only way the wiki changes. The model proposes a plan; propose
// validates it and writes a change document; the user reads it in the chat or in
// Obsidian, and may edit it; apply reads the document again, validates it again, and
// makes one git commit. Undo restores the paths of one applied change.
package change

import (
	"errors"
	"fmt"
	"math"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/gitx"
	"github.com/nathanaday/atlas-obsidian/internal/links"
	"github.com/nathanaday/atlas-obsidian/internal/schema"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

// MaxWrites bounds the writes the model gives in one change. Link rewrites do not count.
const MaxWrites = 100

// Plan is the model's proposal: the Wiki Change Plan.
type Plan struct {
	Title      string   `json:"title" jsonschema:"a short name: the file name of the change document and the commit subject"`
	Notes      string   `json:"notes,omitempty" jsonschema:"what the change does and why, and every skipped subject with its reason; becomes the Notes section"`
	Absorbs    []string `json:"absorbs,omitempty" jsonschema:"ids of the documents this change absorbs into the wiki"`
	Thread     string   `json:"thread,omitempty" jsonschema:"the thread the change serves, if any"`
	Supersedes string   `json:"supersedes,omitempty" jsonschema:"a proposed change this one replaces"`
	Writes     []Write  `json:"writes" jsonschema:"the writes, in order"`
}

// Write is one write of a plan.
type Write struct {
	Op       string         `json:"op" jsonschema:"create, modify, rename, or remove"`
	Type     string         `json:"type,omitempty" jsonschema:"create: area, repository, concept, entity, or policy"`
	Title    string         `json:"title,omitempty" jsonschema:"create: the new page's title; rename: the new title"`
	ID       string         `json:"id,omitempty" jsonschema:"modify, rename, remove: the page's id or title"`
	Base     string         `json:"base,omitempty" jsonschema:"modify, rename, remove: the hash of the page as read; code records it when omitted"`
	Redirect string         `json:"redirect,omitempty" jsonschema:"remove: the page that links to the removed page now name"`
	Fields   map[string]any `json:"fields,omitempty" jsonschema:"create, modify: the type's fields; links as ids or titles; modify merges them over the page's fields"`
	Body     *string        `json:"body,omitempty" jsonschema:"create, modify: the body below the frontmatter; modify replaces the body only when given"`
}

// op is one validated write, as a change document records it.
type op struct {
	Kind     string // create, modify, rename, remove
	ID       string
	Type     string
	Title    string // the page's title; for a rename, the old one
	NewTitle string // rename
	Path     string // the page's path now; for a create, where it goes
	NewPath  string // rename
	Base     string // the hash of the page as read
	Content  string // create and modify: the whole new file
	Redirect string // remove: the id of the page links move to
	Rewrite  bool   // a modify the link rewrite pass made
}

// has reports whether the op writes content.
func (o *op) writesContent() bool { return o.Kind == "create" || o.Kind == "modify" }

// planned is a validated change, ready to write as a document or to apply.
type planned struct {
	Title      string
	Notes      string
	Absorbs    []*doc.Doc
	Thread     *doc.Doc
	Supersedes *doc.Doc
	Ops        []*op
	// Outside are the documents outside the wiki whose links the change rewrites.
	Outside  []Rewrite
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
	problems []string
	warnings []string
	// claimed holds each title key a create or rename takes, with the op that takes it.
	claimed map[string]*op
	// gone holds each title key a rename or remove frees.
	gone map[string]bool
	// byID holds the ops on each page.
	byID map[string][]*op
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
	return c.idx.TypeOfLink(t)
}

func (c *check) removed(id string) bool {
	for _, o := range c.byID[id] {
		if o.Kind == "remove" {
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
		if o.Kind == "rename" {
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

// finalTitle is a page's title after the change's renames.
func (c *check) finalTitle(d *doc.Doc) string {
	for _, o := range c.byID[d.ID()] {
		if o.Kind == "rename" {
			return o.NewTitle
		}
	}
	return vault.Title(d)
}

// takeTitle claims a title for a create or a rename, and refuses one that another
// document or another write holds.
func (c *check) takeTitle(o *op, title string, self string) bool {
	key := links.Key(title)
	if other := c.claimed[key]; other != nil && other != o {
		c.refuse("%s: another write of this change also takes the title %q", o.label(), title)
		return false
	}
	for _, p := range c.idx.TitleHolders(title) {
		if p == self {
			continue
		}
		if d := c.idx.ByPath(p); d != nil && c.gone[links.Key(vault.Title(d))] && links.Key(vault.Title(d)) == key {
			continue
		}
		c.refuse("%s: the title %q is held by %s; titles are unique in the vault", o.label(), title, p)
		return false
	}
	c.claimed[key] = o
	return true
}

func (o *op) label() string {
	switch o.Kind {
	case "create":
		return fmt.Sprintf("create %s %q", o.Type, o.Title)
	case "rename":
		return fmt.Sprintf("rename %q", o.Title)
	}
	return fmt.Sprintf("%s %q (%s)", o.Kind, o.Title, o.ID)
}

// Route is where a new page of a type with a title lives.
func Route(typ, title string) string {
	return schema.Get(typ).Folder + "/" + title + ".md"
}

// modelTypes are the types a change may create.
var modelTypes = []string{"area", "repository", "concept", "entity", "policy"}

// validate turns a plan into a planned change, or refuses it.
func validate(idx *vault.Index, p Plan, now time.Time) (*planned, error) {
	c := &check{idx: idx, now: now, claimed: map[string]*op{}, gone: map[string]bool{}, byID: map[string][]*op{}}
	out := &planned{Title: doc.CleanTitle(p.Title), Notes: strings.TrimSpace(p.Notes)}
	if out.Title == "" {
		c.refuse("the plan needs a title: a short name for the change")
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
		if !idx.Wikified(d.Type()) {
			c.refuse("absorbs: %s is a %s; the vault's wikify setting lists %s", vault.Title(d), d.Type(), strings.Join(idx.V.Wikify(), ", "))
			continue
		}
		out.Absorbs = append(out.Absorbs, d)
	}
	if strings.TrimSpace(p.Thread) != "" {
		d, err := idx.ResolveType(p.Thread, "stub")
		if err != nil {
			c.refuse("thread: %v", err)
		}
		out.Thread = d
	}
	if strings.TrimSpace(p.Supersedes) != "" {
		d, err := idx.ResolveType(p.Supersedes, "change")
		switch {
		case err != nil:
			c.refuse("supersedes: %v", err)
		case d.Str("status") != "proposed":
			c.refuse("supersedes: %s is %s; only a proposed change can be superseded", vault.Title(d), d.Str("status"))
		}
		out.Supersedes = d
	}
	// First pass: every op on an existing page, so the titles they free and take are
	// known before a create claims one.
	ops := make([]*op, len(p.Writes))
	for i, w := range p.Writes {
		w.Op = strings.ToLower(strings.TrimSpace(w.Op))
		switch w.Op {
		case "modify", "rename", "remove":
			ops[i] = c.existing(w)
		case "create":
		default:
			c.refuse("write %d: op %q; it must be create, modify, rename, or remove", i+1, w.Op)
		}
	}
	for i, w := range p.Writes {
		if strings.ToLower(strings.TrimSpace(w.Op)) == "create" {
			ops[i] = c.create(w)
		}
	}
	for i, w := range p.Writes {
		o := ops[i]
		if o == nil {
			continue
		}
		switch o.Kind {
		case "create":
			c.content(o, w, nil)
		case "modify":
			c.content(o, w, idx.ByID(o.ID))
		case "remove":
			if w.Redirect != "" {
				title, typ := c.titleOf(w.Redirect)
				r, err := idx.Resolve(w.Redirect)
				switch {
				case err != nil || title == "":
					c.refuse("%s: redirect %q names no page that stays", o.label(), w.Redirect)
				case r.ID() == o.ID:
					c.refuse("%s: a page cannot redirect to itself", o.label())
				default:
					o.Redirect = r.ID()
					_ = typ
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
	if len(c.problems) > 0 {
		return nil, &Refusal{Problems: c.problems}
	}
	return out, nil
}

// existing validates a modify, rename, or remove, which names a page of the wiki.
func (c *check) existing(w Write) *op {
	d, err := c.idx.Resolve(w.ID)
	if err != nil {
		c.refuse("%s: %v", w.Op, err)
		return nil
	}
	t := schema.Get(d.Type())
	o := &op{Kind: w.Op, ID: d.ID(), Type: d.Type(), Title: vault.Title(d), Path: d.Path}
	if t == nil || !t.Wiki() || !strings.HasPrefix(d.Path, vault.Wiki+"/") {
		c.refuse("%s: %s is a %s at %s; a change writes only pages of the wiki. Thread documents change through the thread tool", o.label(), o.Title, d.Type(), d.Path)
		return nil
	}
	for _, other := range c.byID[o.ID] {
		if other.Kind == o.Kind || other.Kind == "remove" || o.Kind == "remove" {
			c.refuse("%s: the plan already has a %s of this page; one page takes one modify, one rename, or one remove", o.label(), other.Kind)
			return nil
		}
	}
	current := doc.FileHash([]byte(d.Content))
	o.Base = current
	if w.Base != "" {
		if !doc.SameHash(w.Base, current) {
			c.refuse("%s: conflict: %s changed since you read it (base %s, now %s). Read it again and propose again", o.label(), d.Path, doc.Short(w.Base), doc.Short(current))
			return nil
		}
		o.Base = w.Base
	}
	switch o.Kind {
	case "rename":
		title := doc.CleanTitle(w.Title)
		if title == "" {
			c.refuse("%s: a rename needs the new title", o.label())
			return nil
		}
		if links.Key(title) == links.Key(o.Title) && title == o.Title {
			c.refuse("%s: the new title is the old one", o.label())
			return nil
		}
		o.NewTitle = title
		o.NewPath = path.Dir(d.Path) + "/" + title + ".md"
		c.gone[links.Key(o.Title)] = true
		if links.Key(title) != links.Key(o.Title) {
			c.takeTitle(o, title, d.Path)
		} else {
			c.claimed[links.Key(title)] = o
		}
	case "remove":
		c.gone[links.Key(o.Title)] = true
		if d.Type() == "source" {
			c.warn("%s: the captured file %s stays under %s", o.label(), d.Str("file"), vault.SourceFiles)
		}
	}
	c.byID[o.ID] = append(c.byID[o.ID], o)
	return o
}

// create validates the identity of a new page: its type and its title.
func (c *check) create(w Write) *op {
	typ := strings.ToLower(strings.TrimSpace(w.Type))
	title := doc.CleanTitle(w.Title)
	o := &op{Kind: "create", Type: typ, Title: title}
	switch {
	case typ == "source":
		c.refuse("%s: a source page comes from the source tool's capture; a change modifies it", o.label())
		return nil
	case !contains(modelTypes, typ):
		c.refuse("create: type %q; a change creates an %s", w.Type, strings.Join(modelTypes, ", "))
		return nil
	case title == "":
		c.refuse("%s: a create needs a title", o.label())
		return nil
	}
	o.Path = Route(typ, title)
	o.ID = doc.NewID(schema.Get(typ).Prefix, func(id string) bool { return c.idx.ByID(id) != nil })
	c.takeTitle(o, title, "")
	return o
}

// content builds and checks the whole new file of a create or a modify.
func (c *check) content(o *op, w Write, current *doc.Doc) {
	t := schema.Get(o.Type)
	fields, removed := c.fields(o, t, w.Fields)
	today := vault.Date(c.now)
	var content string
	if current == nil {
		list := []doc.Field{{Key: "id", Value: o.ID}, {Key: "type", Value: o.Type}, {Key: "created", Value: today}, {Key: "updated", Value: today}}
		for _, f := range t.Fields {
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
		if o.Type == "repository" {
			if p, ok := fields["path"].(string); ok {
				g := gitx.Repo{Dir: vault.Expand(p)}
				list = append(list, doc.Field{Key: "remote", Value: g.Remote()}, doc.Field{Key: "branch", Value: g.Branch()})
			}
		}
		body := skeleton(t)
		if w.Body != nil {
			body = *w.Body
		}
		content = doc.Render(list, normalizeBody(body))
	} else {
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
			front, _, _ := doc.Split(content)
			content = doc.Join(front, "\n"+normalizeBody(*w.Body))
		}
		content = doc.SetField(content, "updated", today)
	}
	o.Content = content
	c.checkPage(o, content)
}

// checkPage checks a page's whole content against its type.
func (c *check) checkPage(o *op, content string) {
	d := doc.Parse(o.Path, []byte(content))
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
	if o.Type == "repository" {
		if err := c.repoPath(d.Str("path"), o.ID); err != nil {
			c.refuse("%s: path: %v", o.label(), err)
		}
	}
	for _, a := range d.List("aliases") {
		for _, holder := range c.idx.TitleHolders(a) {
			if holder != o.Path && !(o.Kind == "modify" && holder == c.idx.ByID(o.ID).Path) {
				c.refuse("%s: the alias %q is held by %s; titles and aliases are unique in the vault", o.label(), a, holder)
			}
		}
		if other := c.claimed[links.Key(a)]; other != nil && other != o {
			c.refuse("%s: the alias %q is a title this change gives to %s", o.label(), a, other.label())
		}
	}
}

// repoPath checks a repository page's path: the root of a git work tree, outside the
// vault, that no other repository page holds.
func (c *check) repoPath(p, self string) error {
	if strings.TrimSpace(p) == "" {
		return errors.New("a repository page needs the path of its repository")
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
		if r.ID() != self && r.Str("path") != "" && vault.Within(vault.Expand(r.Str("path")), abs) && vault.Within(abs, vault.Expand(r.Str("path"))) {
			return fmt.Errorf("%s is already linked by %s", p, vault.Title(r))
		}
	}
	return nil
}

// fields normalizes the model's fields: code's are dropped with a warning, links become
// wikilinks by title, and values take their field's form. It returns the fields to set
// and the ones to remove.
func (c *check) fields(o *op, t *schema.Type, in map[string]any) (map[string]any, []string) {
	out := map[string]any{}
	var removed []string
	for k, v := range in {
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
			if s, ok := value.(string); ok {
				value = vault.Shorten(vault.Expand(s))
			}
		}
		out[k] = value
	}
	sort.Strings(removed)
	return out, removed
}

// value gives a field's value its form: a link as [[Title]], a list as strings, a number
// as an int.
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
		if d, err := c.idx.Resolve(s); err == nil && d.Type() == "vault" {
			return nil, fmt.Errorf("%s is the vault; leave %s empty for the vault", s, f.Name)
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

// skeleton is a new page's body when the plan gives none: the type's sections.
func skeleton(t *schema.Type) string {
	var b strings.Builder
	for i, s := range t.Sections {
		if i > 0 {
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
func (out *planned) retitles(c *check) []Retitle {
	var list []Retitle
	for _, o := range out.Ops {
		switch o.Kind {
		case "rename":
			list = append(list, Retitle{Old: o.Title, New: o.NewTitle})
		case "remove":
			if o.Redirect == "" {
				continue
			}
			r := c.idx.ByID(o.Redirect)
			list = append(list, Retitle{Old: o.Title, New: c.finalTitle(r), Redirect: r.Type()})
		}
	}
	return list
}

// rewrites runs the link rewrite pass. A page of the wiki it rewrites becomes a modify of
// the change, or joins the one the plan has; any other document is listed as outside.
func (c *check) rewrites(out *planned) {
	retitles := out.retitles(c)
	if len(retitles) == 0 {
		c.removedLinks(out)
		return
	}
	contents := map[string]string{}
	skip := map[string]bool{}
	content := map[string]*op{}
	for _, o := range out.Ops {
		switch {
		case o.writesContent() && o.Kind == "modify":
			contents[o.Path] = o.Content
			content[o.Path] = o
		case o.Kind == "remove":
			skip[o.Path] = true
		}
	}
	rewrites, warnings := Rewrites(c.idx, retitles, contents, skip)
	c.warnings = append(c.warnings, warnings...)
	for _, rw := range rewrites {
		d := c.idx.ByPath(rw.Path)
		t := schema.Get(d.Type())
		inWiki := t != nil && t.Wiki() && strings.HasPrefix(rw.Path, vault.Wiki+"/")
		switch {
		case content[rw.Path] != nil:
			content[rw.Path].Content = rw.Content
		case inWiki:
			o := &op{Kind: "modify", ID: d.ID(), Type: d.Type(), Title: vault.Title(d), Path: d.Path, Base: doc.FileHash([]byte(d.Content)), Content: doc.SetField(rw.Content, "updated", vault.Date(c.now)), Rewrite: true}
			c.byID[o.ID] = append(c.byID[o.ID], o)
			out.Ops = append(out.Ops, o)
		default:
			out.Outside = append(out.Outside, rw)
		}
	}
	// Created pages may link the old titles too.
	rename := links.Rename{}
	for _, r := range retitles {
		rename[r.Old] = r.New
	}
	for _, o := range out.Ops {
		if o.Kind == "create" {
			o.Content, _ = links.Rewrite(o.Content, rename)
		}
	}
	c.removedLinks(out)
}

// removedLinks warns of the links a remove with no redirect leaves dead.
func (c *check) removedLinks(out *planned) {
	for _, o := range out.Ops {
		if o.Kind != "remove" || o.Redirect != "" {
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
