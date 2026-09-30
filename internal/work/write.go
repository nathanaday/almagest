package work

import (
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/schema"
	"github.com/nathanaday/atlas-obsidian/internal/tags"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

// Opts are what every write needs besides its input: the time, and who acts.
type Opts struct {
	Now time.Time
	// By is agent for a tool call, user for the CLI and the Obsidian plugin.
	By string
}

// Result is what a work write returns: the view of the document it acted on, the
// commit, and the documents and events it wrote, which the hook links to the session.
type Result struct {
	View    *View       `json:"view,omitempty"`
	Commit  string      `json:"commit,omitempty"`
	Wrote   []vault.Ref `json:"wrote"`
	Events  []vault.Ref `json:"events"`
	Started string      `json:"started,omitempty" jsonschema:"the plan work start bound this session to"`
}

// StubIn plants a stub.
type StubIn struct {
	Text        string   `json:"text" jsonschema:"the user's words, as given; they become the stub's Idea"`
	Title       string   `json:"title,omitempty" jsonschema:"a short name; the first line of text when empty"`
	Description string   `json:"description,omitempty" jsonschema:"one sentence; the first sentence of text when empty"`
	Tags        []string `json:"tags,omitempty" jsonschema:"the categories: tags like work/p3 or self-driving"`
	Priority    string   `json:"priority,omitempty" jsonschema:"high, normal, low, or someday"`
	Inbox       string   `json:"inbox,omitempty" jsonschema:"a note in inbox/ that the stub replaces; it leaves the inbox in the same commit"`
	NewTags     bool     `json:"new_tags,omitempty" jsonschema:"allow a tag no document holds, in tagging: known; set it only after the user agreed"`
}

// SpecIn is one new spec.
type SpecIn struct {
	Title        string   `json:"title" jsonschema:"the spec's title; unique in the vault"`
	Kind         string   `json:"kind" jsonschema:"plan (work that ends) or design (how something must work)"`
	Text         string   `json:"text" jsonschema:"the body: for a plan Goal, Done when, Decisions, Out of scope, Conventions, Where, Verify; for a design Purpose, Behavior, Interfaces, Constraints, Decisions"`
	Description  string   `json:"description,omitempty" jsonschema:"one sentence; the first line of the Goal or Purpose when empty"`
	Tags         []string `json:"tags,omitempty"`
	Parent       string   `json:"parent,omitempty" jsonschema:"plan: the plan this is a part of, by id or title, or the title of a plan in the same call"`
	Repositories []string `json:"repositories,omitempty" jsonschema:"the repositories the work touches or the design describes, by id or title"`
	Depends      []string `json:"depends,omitempty" jsonschema:"plan: sibling plans that must be done first, by id or title"`
	Order        int      `json:"order,omitempty" jsonschema:"plan: the place among its siblings; the next free one when 0"`
	Priority     string   `json:"priority,omitempty" jsonschema:"plan: high, normal, low, or someday"`
	Implements   []string `json:"implements,omitempty" jsonschema:"plan: the design specs this work builds"`
	Supersedes   string   `json:"supersedes,omitempty" jsonschema:"design: the design spec this one replaces"`
	From         string   `json:"from,omitempty" jsonschema:"the stub this spec grew from"`
}

// SpecsIn writes one or more specs in one commit.
type SpecsIn struct {
	Specs   []SpecIn `json:"specs" jsonschema:"the specs: a plan and its parts, or one spec"`
	Resolve bool     `json:"resolve,omitempty" jsonschema:"close the from stub as resolved, with these specs in its became"`
	NewTags bool     `json:"new_tags,omitempty"`
}

// PromoteIn makes an open stub a spec in place.
type PromoteIn struct {
	Stub         string   `json:"stub" jsonschema:"the open stub, by id or title"`
	Kind         string   `json:"kind" jsonschema:"plan or design"`
	Text         string   `json:"text" jsonschema:"the spec's sections; the stub's Idea is kept as Origin"`
	Title        string   `json:"title,omitempty" jsonschema:"a new title; links follow"`
	Description  string   `json:"description,omitempty"`
	Tags         []string `json:"tags,omitempty" jsonschema:"replaces the stub's tags when given"`
	Repositories []string `json:"repositories,omitempty"`
	Parent       string   `json:"parent,omitempty"`
	Priority     string   `json:"priority,omitempty"`
	NewTags      bool     `json:"new_tags,omitempty"`
}

// ResultIn is the result of done work: the sections of the completed event.
type ResultIn struct {
	Delivered string `json:"delivered" jsonschema:"what changed, with the commits"`
	Verified  string `json:"verified" jsonschema:"how it was verified: the commands and their results"`
	FollowUps string `json:"follow_ups,omitempty" jsonschema:"links to new stubs for what is left"`
	Learned   string `json:"learned,omitempty" jsonschema:"what the wiki should absorb, in a few lines"`
}

// SetIn changes the fields of a stub or a spec. A field left out stays.
type SetIn struct {
	Doc          string    `json:"doc" jsonschema:"the stub or spec, by id or title"`
	Title        *string   `json:"title,omitempty" jsonschema:"a new title; links follow"`
	Description  *string   `json:"description,omitempty"`
	Tags         *[]string `json:"tags,omitempty"`
	Aliases      *[]string `json:"aliases,omitempty"`
	Priority     *string   `json:"priority,omitempty"`
	Parent       *string   `json:"parent,omitempty"`
	Repositories *[]string `json:"repositories,omitempty"`
	Depends      *[]string `json:"depends,omitempty"`
	Order        *int      `json:"order,omitempty"`
	Implements   *[]string `json:"implements,omitempty"`
	Supersedes   *string   `json:"supersedes,omitempty"`
	NewTags      bool      `json:"new_tags,omitempty"`
}

// writer is one work write: the transaction, and the vault as it was when it began.
type writer struct {
	v      *vault.Vault
	tx     *vault.Tx
	idx    *vault.Index
	b      *Board
	now    time.Time
	by     string
	titles *Titles
	wrote  []string // ids of the stubs and specs written
	events []string // ids of the events written
	newTag map[string]bool
	last   map[string]time.Time // subject id → the time of its last event
}

func begin(v *vault.Vault, o Opts) (*writer, error) {
	tx, err := vault.BeginWrite(v)
	if err != nil {
		return nil, err
	}
	idx, err := vault.Load(v)
	if err != nil {
		tx.Close()
		return nil, err
	}
	by := o.By
	if by == "" {
		by = ByAgent
	}
	now := o.Now
	if now.IsZero() {
		now = time.Now()
	}
	return &writer{v: v, tx: tx, idx: idx, b: Load(idx), now: now.Truncate(time.Second), by: by, titles: NewTitles(idx), newTag: map[string]bool{}}, nil
}

// finish syncs the derived parts, commits, and returns the view of the document focus
// (an id).
func (w *writer) finish(subject, focus string) (*Result, error) {
	idx, err := vault.Load(w.v)
	if err != nil {
		return nil, err
	}
	if _, err := Load(idx).Sync(w.tx.WriteIfChanged); err != nil {
		return nil, err
	}
	sha, err := w.tx.Commit("work: "+oneLine(subject, 72), Trailer+": "+focus)
	if err != nil {
		return nil, err
	}
	if idx, err = vault.Load(w.v); err != nil {
		return nil, err
	}
	b := Load(idx)
	res := &Result{Commit: sha, Wrote: []vault.Ref{}, Events: []vault.Ref{}}
	if d := idx.ByID(focus); d != nil {
		res.View = b.View(d)
	}
	for _, id := range w.wrote {
		if d := idx.ByID(id); d != nil {
			res.Wrote = append(res.Wrote, b.Ref(d))
		}
	}
	for _, id := range w.events {
		if d := idx.ByID(id); d != nil {
			res.Events = append(res.Events, idx.Ref(d))
		}
	}
	return res, nil
}

// event writes one event. Its time is at least a second after the subject's last event,
// so the order of a document's events is strict even when two come in one second.
func (w *writer) event(in EventIn) error {
	in.At = w.now
	if w.last == nil {
		w.last = map[string]time.Time{}
	}
	last, ok := w.last[in.SubjectID]
	if !ok {
		if d := w.idx.ByID(in.SubjectID); d != nil {
			if e := w.b.LastEvent(d); e != nil {
				last, _ = vault.ParseTime(e.Str("at"))
			}
		}
	}
	if !last.IsZero() && !in.At.After(last) {
		in.At = last.Add(time.Second)
	}
	w.last[in.SubjectID] = in.At
	in.By = w.by
	rel, content, id := NewEvent(w.titles, in)
	if err := w.tx.Write(rel, []byte(content)); err != nil {
		return err
	}
	w.events = append(w.events, id)
	return nil
}

// on writes an event about a document as the write leaves it.
func (w *writer) on(d *doc.Doc, kind string, prose map[string]string) error {
	return w.event(EventIn{Kind: kind, SubjectTitle: d.Title(), SubjectID: d.ID(), SubjectTags: d.List("tags"), Prose: prose})
}

// title checks a new title: clean, not a view's, and free in the vault and this write.
func (w *writer) title(raw string) (string, error) {
	t := doc.CleanTitle(raw)
	switch {
	case t == "":
		return "", errors.New("a title is needed")
	case vault.ReservedTitle(t):
		return "", fmt.Errorf("%q begins as a view's title does; choose another", t)
	case !w.titles.Free(t):
		holders := w.idx.TitleHolders(t)
		if len(holders) == 0 {
			holders = []string{"another document of this call"}
		}
		return "", fmt.Errorf("the title %q is held by %s; titles are unique in the vault, so choose another", t, strings.Join(holders, ", "))
	}
	w.titles.Take(t)
	return t, nil
}

// tagList normalizes tags and, in tagging: known, refuses a tag no document holds
// unless the call allows new tags.
func (w *writer) tagList(list []string, allowNew bool) ([]string, error) {
	out, err := tags.NormalizeAll(list)
	if err != nil {
		return nil, err
	}
	for _, t := range out {
		if w.idx.TagExists(t) || w.newTag[t] {
			continue
		}
		if w.v.Tagging() == "known" && !allowNew {
			return nil, fmt.Errorf("the tag %q is new, and this vault uses known tags; use a tag that exists, or ask the user and call again with new_tags: true", t)
		}
		w.newTag[t] = true
	}
	return out, nil
}

// repos resolves repository documents that are linked, as links.
func (w *writer) repos(list []string) ([]string, error) {
	out := []string{}
	for _, r := range list {
		if strings.TrimSpace(r) == "" {
			continue
		}
		d, err := w.idx.ResolveType(r, "repository")
		if err != nil {
			return nil, fmt.Errorf("repositories: %w", err)
		}
		if d.Front.Bool("unlinked") {
			return nil, fmt.Errorf("repositories: %s is unlinked; link it again first (repo-link)", d.Title())
		}
		if l := doc.Link(d.Title()); !slices.Contains(out, l) {
			out = append(out, l)
		}
	}
	return out, nil
}

func (w *writer) specsOf(list []string, kind, field string) ([]string, error) {
	out := []string{}
	for _, s := range list {
		if strings.TrimSpace(s) == "" {
			continue
		}
		d, err := w.idx.ResolveType(s, "spec")
		if err != nil {
			return nil, fmt.Errorf("%s: %w", field, err)
		}
		if d.Str("kind") != kind {
			return nil, fmt.Errorf("%s: %s is a %s, not a %s", field, d.Title(), d.Str("kind"), kind)
		}
		out = append(out, doc.Link(d.Title()))
	}
	return out, nil
}

func checkPriority(p string) error {
	if p != "" && !slices.Contains(Priorities, p) {
		return fmt.Errorf("priority is %q; it takes high, normal, low, or someday", p)
	}
	return nil
}

// TitleFromText is a title from the first line of text, cut at 60 characters at a word.
func TitleFromText(text string) string {
	line := doc.CleanTitle(doc.FirstLine(text))
	if len([]rune(line)) <= 60 {
		return line
	}
	r := []rune(line)[:60]
	if i := strings.LastIndex(string(r), " "); i > 20 {
		return strings.TrimSpace(string(r)[:i])
	}
	return strings.TrimSpace(string(r))
}

// firstSentence is a description from text: its first line that is no heading or
// callout, cut at the end of its first sentence.
func firstSentence(text string) string {
	line := ""
	for _, l := range strings.Split(text, "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "#") || strings.HasPrefix(l, ">") || strings.HasPrefix(l, "|") || strings.HasPrefix(l, "```") {
			continue
		}
		line = strings.TrimLeft(l, "-*0123456789. ")
		break
	}
	if i := strings.Index(line, ". "); i > 0 {
		line = line[:i+1]
	}
	return oneLine(line, 200)
}

// Stub plants a stub, in the user's words.
func Stub(v *vault.Vault, in StubIn, o Opts) (*Result, error) {
	text := strings.TrimSpace(in.Text)
	if text == "" {
		return nil, errors.New("stub needs text: the user's words")
	}
	w, err := begin(v, o)
	if err != nil {
		return nil, err
	}
	defer w.tx.Close()
	raw := in.Title
	if strings.TrimSpace(raw) == "" {
		raw = TitleFromText(text)
	}
	title, err := w.title(raw)
	if err != nil {
		return nil, err
	}
	if err := checkPriority(in.Priority); err != nil {
		return nil, err
	}
	tg, err := w.tagList(in.Tags, in.NewTags)
	if err != nil {
		return nil, err
	}
	if in.Inbox != "" {
		rel := path.Clean(vault.Inbox + "/" + strings.TrimPrefix(in.Inbox, vault.Inbox+"/"))
		if !strings.HasPrefix(rel, vault.Inbox+"/") || !w.v.Exists(rel) {
			return nil, fmt.Errorf("inbox: %s is not in inbox/", in.Inbox)
		}
		if err := w.tx.Remove(rel); err != nil {
			return nil, err
		}
	}
	desc := strings.TrimSpace(in.Description)
	if desc == "" {
		desc = firstSentence(text)
	}
	id := doc.NewID(schema.DocPrefix, func(s string) bool { return w.idx.ByID(s) != nil })
	stamp := vault.Stamp(w.now)
	content := doc.Render([]doc.Field{
		{Key: "id", Value: id},
		{Key: "type", Value: "stub"},
		{Key: "description", Value: oneLine(desc, 200)},
		{Key: "tags", Value: tg},
		{Key: "aliases", Value: []string{}},
		{Key: "created", Value: stamp},
		{Key: "updated", Value: stamp},
		{Key: "refreshed", Value: stamp},
		{Key: "priority", Value: orDefault(in.Priority, "normal")},
		{Key: "status", Value: Open},
		{Key: "became", Value: []string{}},
	}, "## Idea\n\n"+text+"\n")
	if err := w.tx.Write(vault.DocPath(title), []byte(content)); err != nil {
		return nil, err
	}
	w.wrote = append(w.wrote, id)
	return w.finish("stub "+title, id)
}

// newSpec is a spec of one call, as validated.
type newSpec struct {
	in     SpecIn
	id     string
	title  string
	parent string // the parent's title
	fields []doc.Field
	body   string
}

// Specs writes one or more specs in one commit: a plan and its parts, or one spec.
func Specs(v *vault.Vault, in SpecsIn, o Opts) (*Result, error) {
	if len(in.Specs) == 0 {
		return nil, errors.New("specs needs at least one spec")
	}
	w, err := begin(v, o)
	if err != nil {
		return nil, err
	}
	defer w.tx.Close()
	list := make([]*newSpec, len(in.Specs))
	byTitle := map[string]*newSpec{}
	for i, s := range in.Specs {
		title, err := w.title(s.Title)
		if err != nil {
			return nil, fmt.Errorf("spec %d: %w", i+1, err)
		}
		ns := &newSpec{in: s, title: title, id: doc.NewID(schema.DocPrefix, func(x string) bool { return w.idx.ByID(x) != nil })}
		list[i] = ns
		byTitle[strings.ToLower(title)] = ns
	}
	// A title the call gives names a spec of the call before one of the vault.
	resolveSpec := func(key string) (string, *doc.Doc, *newSpec, error) {
		t := doc.LinkTarget(key)
		if ns := byTitle[strings.ToLower(t)]; ns != nil {
			return ns.title, nil, ns, nil
		}
		d, err := w.idx.ResolveType(t, "spec")
		if err != nil {
			return "", nil, nil, err
		}
		return d.Title(), d, nil, nil
	}
	var from *doc.Doc
	for _, ns := range list {
		s := ns.in
		kind := s.Kind
		if kind != Plan && kind != Design {
			return nil, fmt.Errorf("%s: kind is %q; a spec is a plan or a design", ns.title, kind)
		}
		if err := checkPriority(s.Priority); err != nil {
			return nil, fmt.Errorf("%s: %w", ns.title, err)
		}
		tg, err := w.tagList(s.Tags, in.NewTags)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", ns.title, err)
		}
		repos, err := w.repos(s.Repositories)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", ns.title, err)
		}
		if kind == Design && (s.Parent != "" || len(s.Depends) > 0 || s.Order != 0 || len(s.Implements) > 0) {
			return nil, fmt.Errorf("%s: a design takes no parent, depends, order, or implements; a plan implements a design", ns.title)
		}
		if kind == Plan && s.Supersedes != "" {
			return nil, fmt.Errorf("%s: only a design supersedes a design", ns.title)
		}
		if s.Parent != "" {
			title, d, _, err := resolveSpec(s.Parent)
			if err != nil {
				return nil, fmt.Errorf("%s: parent: %w", ns.title, err)
			}
			if d != nil {
				if d.Str("kind") != Plan {
					return nil, fmt.Errorf("%s: parent: %s is a design; a part's parent is a plan", ns.title, d.Title())
				}
				if Closed(w.b.Status(d)) {
					return nil, fmt.Errorf("%s: parent: %s is %s; work reopen takes it up again first", ns.title, d.Title(), w.b.Status(d))
				}
			} else if p := byTitle[strings.ToLower(title)]; p != nil && p.in.Kind != Plan {
				return nil, fmt.Errorf("%s: parent: %s is a design; a part's parent is a plan", ns.title, title)
			}
			ns.parent = title
		}
		implements, err := w.specsOf(s.Implements, Design, "implements")
		if err != nil {
			return nil, fmt.Errorf("%s: %w", ns.title, err)
		}
		supersedes := ""
		if s.Supersedes != "" {
			l, err := w.specsOf([]string{s.Supersedes}, Design, "supersedes")
			if err != nil {
				return nil, fmt.Errorf("%s: %w", ns.title, err)
			}
			supersedes = l[0]
		}
		fromLink := ""
		if s.From != "" {
			f, err := w.idx.ResolveType(s.From, "stub")
			if err != nil {
				return nil, fmt.Errorf("%s: from: %w", ns.title, err)
			}
			if w.b.Status(f) != Open {
				return nil, fmt.Errorf("%s: from: %s is %s", ns.title, f.Title(), w.b.Status(f))
			}
			if from != nil && from.ID() != f.ID() {
				return nil, errors.New("the specs of one call grow from one stub at most")
			}
			from = f
			fromLink = doc.Link(f.Title())
		}
		text := strings.TrimSpace(doc.StripSections(s.Text, schema.Get("spec").CodeSections))
		desc := strings.TrimSpace(s.Description)
		if desc == "" {
			lead := "Goal"
			if kind == Design {
				lead = "Purpose"
			}
			sec, _ := doc.Section(text, lead)
			desc = firstSentence(sec)
			if desc == "" {
				desc = firstSentence(text)
			}
		}
		if desc == "" {
			desc = ns.title
		}
		stamp := vault.Stamp(w.now)
		ns.fields = []doc.Field{
			{Key: "id", Value: ns.id},
			{Key: "type", Value: "spec"},
			{Key: "kind", Value: kind},
			{Key: "description", Value: oneLine(desc, 200)},
			{Key: "tags", Value: tg},
			{Key: "aliases", Value: []string{}},
			{Key: "created", Value: stamp},
			{Key: "updated", Value: stamp},
			{Key: "refreshed", Value: stamp},
		}
		if kind == Plan {
			ns.fields = append(ns.fields,
				doc.Field{Key: "parent", Value: doc.Link(ns.parent)},
				doc.Field{Key: "repositories", Value: repos},
				doc.Field{Key: "depends", Value: []string{}},
				doc.Field{Key: "order", Value: s.Order},
				doc.Field{Key: "priority", Value: orDefault(s.Priority, "normal")},
				doc.Field{Key: "implements", Value: implements},
			)
		} else {
			ns.fields = append(ns.fields, doc.Field{Key: "repositories", Value: repos}, doc.Field{Key: "supersedes", Value: supersedes})
		}
		if fromLink != "" {
			ns.fields = append(ns.fields, doc.Field{Key: "from", Value: fromLink})
		}
		ns.fields = append(ns.fields, doc.Field{Key: "status", Value: map[string]string{Plan: Open, Design: Current}[kind]})
		if text != "" {
			text += "\n"
		}
		ns.body = text
	}
	// The parent chain of the call's specs must not loop.
	for _, ns := range list {
		seen := map[string]bool{strings.ToLower(ns.title): true}
		for p := ns.parent; p != ""; {
			k := strings.ToLower(p)
			if seen[k] {
				return nil, fmt.Errorf("%s: the parents loop", ns.title)
			}
			seen[k] = true
			if next := byTitle[k]; next != nil {
				p = next.parent
			} else {
				p = ""
			}
		}
	}
	// Orders: the next free place among the siblings, for a part that names none.
	orders := map[string][]int{}
	for _, s := range w.b.Specs {
		if p := w.b.ParentOf(s); p != nil {
			orders[strings.ToLower(p.Title())] = append(orders[strings.ToLower(p.Title())], s.Front.Int("order"))
		}
	}
	for _, ns := range list {
		if ns.in.Kind != Plan || ns.parent == "" {
			continue
		}
		k := strings.ToLower(ns.parent)
		order := ns.in.Order
		if order == 0 {
			order = 1
			for slices.Contains(orders[k], order) {
				order++
			}
		}
		orders[k] = append(orders[k], order)
		setField(ns.fields, "order", order)
	}
	// Depends: siblings, by title of the call or of the vault.
	for _, ns := range list {
		if len(ns.in.Depends) == 0 {
			continue
		}
		var deps []string
		for _, dep := range ns.in.Depends {
			title, d, other, err := resolveSpec(dep)
			if err != nil {
				return nil, fmt.Errorf("%s: depends: %w", ns.title, err)
			}
			parent := ""
			switch {
			case other != nil:
				parent = other.parent
			case d != nil:
				if p := w.b.ParentOf(d); p != nil {
					parent = p.Title()
				}
			}
			if !strings.EqualFold(parent, ns.parent) || strings.EqualFold(title, ns.title) {
				return nil, fmt.Errorf("%s: depends: %s is no sibling; a plan depends only on plans with the same parent", ns.title, title)
			}
			deps = append(deps, doc.Link(title))
		}
		setField(ns.fields, "depends", deps)
	}
	for _, ns := range list {
		if ns.in.Kind == Plan && ns.parent == "" {
			setField(ns.fields, "order", 0)
		}
		content := doc.Render(dropEmpty(ns.fields, "parent", "order", "implements", "supersedes"), ns.body)
		if err := w.tx.Write(vault.DocPath(ns.title), []byte(content)); err != nil {
			return nil, err
		}
		w.wrote = append(w.wrote, ns.id)
	}
	if in.Resolve {
		if from == nil {
			return nil, errors.New("resolve closes the stub the specs grew from; give from")
		}
		var became []string
		for _, ns := range list {
			became = append(became, ns.title)
		}
		if err := w.resolve(from, became); err != nil {
			return nil, err
		}
	}
	focus := list[0]
	for focus.parent != "" && byTitle[strings.ToLower(focus.parent)] != nil {
		focus = byTitle[strings.ToLower(focus.parent)]
	}
	subject := "spec " + focus.title
	if len(list) > 1 {
		subject = fmt.Sprintf("specs %s and %d more", focus.title, len(list)-1)
	}
	return w.finish(subject, focus.id)
}

func setField(fields []doc.Field, key string, v any) {
	for i := range fields {
		if fields[i].Key == key {
			fields[i].Value = v
		}
	}
}

// dropEmpty leaves out the named fields when they hold nothing.
func dropEmpty(fields []doc.Field, keys ...string) []doc.Field {
	var out []doc.Field
	for _, f := range fields {
		if slices.Contains(keys, f.Key) {
			switch x := f.Value.(type) {
			case string:
				if x == "" {
					continue
				}
			case int:
				if x == 0 {
					continue
				}
			case []string:
				if len(x) == 0 {
					continue
				}
			}
		}
		out = append(out, f)
	}
	return out
}

// resolve closes a stub as resolved, with what it became.
func (w *writer) resolve(stub *doc.Doc, became []string) error {
	content := doc.SetField(stub.Content, "became", doc.Links(became))
	if err := w.tx.Write(stub.Path, []byte(content)); err != nil {
		return err
	}
	w.wrote = append(w.wrote, stub.ID())
	return w.event(EventIn{Kind: "resolved", SubjectTitle: stub.Title(), SubjectID: stub.ID(), SubjectTags: stub.List("tags"), Became: became})
}

// Resolve closes an open stub as resolved, with the documents it became.
func Resolve(v *vault.Vault, stub string, became []string, o Opts) (*Result, error) {
	if len(became) == 0 {
		return nil, errors.New("resolve needs became: the documents the stub became")
	}
	w, err := begin(v, o)
	if err != nil {
		return nil, err
	}
	defer w.tx.Close()
	d, err := w.idx.ResolveType(stub, "stub")
	if err != nil {
		return nil, err
	}
	if s := w.b.Status(d); s != Open {
		return nil, fmt.Errorf("%s is %s; only an open stub resolves", d.Title(), s)
	}
	var titles []string
	for _, b := range became {
		bd, err := w.idx.Resolve(b)
		if err != nil {
			return nil, fmt.Errorf("became: %w", err)
		}
		if bd.ID() == d.ID() {
			return nil, errors.New("became: a stub does not become itself")
		}
		titles = append(titles, bd.Title())
	}
	if err := w.resolve(d, titles); err != nil {
		return nil, err
	}
	return w.finish("resolve "+d.Title(), d.ID())
}

// ResolveInTx closes an open stub as resolved inside another write's transaction: a
// capture that the stub asked for. It returns the id of the event.
func ResolveInTx(tx *vault.Tx, idx *vault.Index, stub *doc.Doc, became []string, o Opts) (string, error) {
	w := &writer{v: tx.V, tx: tx, idx: idx, b: Load(idx), now: o.Now.Truncate(time.Second), by: orDefault(o.By, ByAgent), titles: NewTitles(idx), newTag: map[string]bool{}}
	for _, b := range became {
		w.titles.Take(b)
	}
	if s := w.b.Status(stub); s != Open {
		return "", fmt.Errorf("%s is %s; only an open stub resolves", stub.Title(), s)
	}
	if err := w.resolve(stub, became); err != nil {
		return "", err
	}
	return w.events[0], nil
}

// PromotedEvent writes the promoted event of a stub a change made a topic, in the
// change's transaction. taken holds the titles the change takes.
func PromotedEvent(tx *vault.Tx, idx *vault.Index, title, id string, tagList []string, change string, taken []string, now time.Time) (string, error) {
	t := NewTitles(idx)
	for _, x := range taken {
		t.Take(x)
	}
	rel, content, eid := NewEvent(t, EventIn{Kind: "promoted", SubjectTitle: title, SubjectID: id, SubjectTags: tagList, At: now.Truncate(time.Second), By: ByAgent, FromType: "stub", ToType: "topic", Change: change})
	return eid, tx.Write(rel, []byte(content))
}

// Promote makes an open stub a spec in place: same id, same file (or a new title, with
// its links rewritten), the stub's words kept as Origin.
func Promote(v *vault.Vault, in PromoteIn, o Opts) (*Result, error) {
	w, err := begin(v, o)
	if err != nil {
		return nil, err
	}
	defer w.tx.Close()
	d, err := w.idx.ResolveType(in.Stub, "stub")
	if err != nil {
		return nil, err
	}
	if s := w.b.Status(d); s != Open {
		return nil, fmt.Errorf("%s is %s; only an open stub is promoted", d.Title(), s)
	}
	if in.Kind != Plan && in.Kind != Design {
		return nil, fmt.Errorf("kind is %q; a stub becomes a plan or a design here, and a topic through a change", in.Kind)
	}
	title := d.Title()
	if strings.TrimSpace(in.Title) != "" && doc.CleanTitle(in.Title) != title {
		w.titles.Release(title)
		if title, err = w.title(in.Title); err != nil {
			return nil, err
		}
	}
	if err := checkPriority(in.Priority); err != nil {
		return nil, err
	}
	tg := d.List("tags")
	if in.Tags != nil {
		if tg, err = w.tagList(in.Tags, in.NewTags); err != nil {
			return nil, err
		}
	}
	repos, err := w.repos(in.Repositories)
	if err != nil {
		return nil, err
	}
	parent := ""
	if in.Parent != "" {
		if in.Kind != Plan {
			return nil, errors.New("a design takes no parent")
		}
		p, err := w.idx.ResolveType(in.Parent, "spec")
		if err != nil {
			return nil, fmt.Errorf("parent: %w", err)
		}
		if p.Str("kind") != Plan || Closed(w.b.Status(p)) {
			return nil, fmt.Errorf("parent: %s is no open plan", p.Title())
		}
		parent = doc.Link(p.Title())
	}
	desc := d.Str("description")
	if strings.TrimSpace(in.Description) != "" {
		desc = oneLine(in.Description, 200)
	}
	idea, _ := doc.Section(d.Body, "Idea")
	notes, _ := doc.Section(d.Body, "Notes")
	text := strings.TrimSpace(doc.StripSections(in.Text, schema.Get("spec").CodeSections))
	body := text + "\n"
	order := schema.Get("spec").SectionsOf(in.Kind)
	body = doc.PutSection(body, "Origin", idea, order)
	body = doc.PutSection(body, "Notes", notes, order)
	fields := []doc.Field{
		{Key: "id", Value: d.ID()},
		{Key: "type", Value: "spec"},
		{Key: "kind", Value: in.Kind},
		{Key: "description", Value: desc},
		{Key: "tags", Value: nonNil(tg)},
		{Key: "aliases", Value: nonNil(d.List("aliases"))},
		{Key: "created", Value: d.Str("created")},
		{Key: "updated", Value: vault.Stamp(w.now)},
		{Key: "refreshed", Value: vault.Stamp(w.now)},
	}
	if in.Kind == Plan {
		fields = append(fields, doc.Field{Key: "parent", Value: parent}, doc.Field{Key: "repositories", Value: repos},
			doc.Field{Key: "priority", Value: orDefault(in.Priority, orDefault(d.Str("priority"), "normal"))}, doc.Field{Key: "status", Value: Open})
	} else {
		fields = append(fields, doc.Field{Key: "repositories", Value: repos}, doc.Field{Key: "status", Value: Current})
	}
	content := doc.Render(dropEmpty(fields, "parent"), body)
	target := vault.DocPath(title)
	if target != d.Path {
		if err := w.tx.Remove(d.Path); err != nil {
			return nil, err
		}
		if err := w.rename(d, title, map[string]string{target: content}); err != nil {
			return nil, err
		}
	}
	if err := w.tx.Write(target, []byte(content)); err != nil {
		return nil, err
	}
	w.wrote = append(w.wrote, d.ID())
	if err := w.event(EventIn{Kind: "promoted", SubjectTitle: title, SubjectID: d.ID(), SubjectTags: tg, FromType: "stub", ToType: "spec"}); err != nil {
		return nil, err
	}
	return w.finish("promote "+title+" to a "+in.Kind, d.ID())
}

// rename rewrites every link to a document's old title in the vault, in this write.
// contents holds the new text of files the write also changes.
func (w *writer) rename(d *doc.Doc, title string, contents map[string]string) error {
	rewrites, _ := vault.Rewrites(w.idx, []vault.Retitle{{Old: d.Title(), New: title}}, contents, map[string]bool{d.Path: true})
	for _, rw := range rewrites {
		if err := w.tx.Write(rw.Path, []byte(rw.Content)); err != nil {
			return err
		}
	}
	return nil
}

// plan resolves a plan for an action.
func (w *writer) plan(key string) (*doc.Doc, error) {
	d, err := w.idx.ResolveType(key, "spec")
	if err != nil {
		return nil, err
	}
	if d.Str("kind") != Plan {
		return nil, fmt.Errorf("%s is a design; a design is not started or finished, a plan that implements it is", d.Title())
	}
	return d, nil
}

// Start starts work on a plan with no parts, or continues it, and binds the session to
// it. The first start of a part starts each open plan above it.
func Start(v *vault.Vault, key string, take bool, o Opts) (*Result, error) {
	w, err := begin(v, o)
	if err != nil {
		return nil, err
	}
	defer w.tx.Close()
	d, err := w.plan(key)
	if err != nil {
		return nil, err
	}
	status := w.b.Status(d)
	if Closed(status) {
		return nil, fmt.Errorf("%s is %s; work reopen takes it up again", d.Title(), status)
	}
	if parts := w.b.Parts(d); len(parts) > 0 {
		return nil, fmt.Errorf("%s has %d parts; start a part (the next is %s), and the plan is started with it", d.Title(), len(parts), strings.TrimPrefix(w.b.Next(d), "start "))
	}
	for _, dep := range d.List("depends") {
		if s := w.idx.Linked(dep); s != nil && !Closed(w.b.Status(s)) {
			return nil, fmt.Errorf("%s depends on %s, which is %s; call work done on %s first", d.Title(), s.Title(), w.b.Status(s), s.Title())
		}
	}
	if done, _ := doc.Section(d.Body, "Done when"); strings.TrimSpace(done) == "" {
		return nil, fmt.Errorf("%s has no Done when list; write it first (spec-write), so a reviewer can check the work", d.Title())
	}
	if hs := w.b.Holders(d); len(hs) > 0 && !take {
		return nil, fmt.Errorf("%s is held by the live session %s. If you mean to take it over, call start again with take: true", d.Title(), hs[0].Title())
	}
	kind := "started"
	if status == Started {
		kind = "continued"
	}
	if err := w.on(d, kind, nil); err != nil {
		return nil, err
	}
	for _, a := range w.b.Ancestors(d) {
		if w.b.Status(a) == Open {
			if err := w.on(a, "started", nil); err != nil {
				return nil, err
			}
		}
	}
	res, err := w.finish(kind+" "+d.Title(), d.ID())
	if err != nil {
		return nil, err
	}
	res.Started = d.Title()
	return res, nil
}

// Complete finishes a started plan with its result: the completed event.
func Complete(v *vault.Vault, key string, r ResultIn, o Opts) (*Result, error) {
	if strings.TrimSpace(r.Delivered) == "" || strings.TrimSpace(r.Verified) == "" {
		return nil, errors.New("done needs the result: delivered (what changed, with the commits) and verified (how, with the commands and their results)")
	}
	w, err := begin(v, o)
	if err != nil {
		return nil, err
	}
	defer w.tx.Close()
	d, err := w.plan(key)
	if err != nil {
		return nil, err
	}
	if s := w.b.Status(d); s != Started {
		return nil, fmt.Errorf("%s is %s; only started work is done (work start first)", d.Title(), s)
	}
	if done, _ := doc.Section(d.Body, "Done when"); strings.TrimSpace(done) == "" {
		return nil, fmt.Errorf("%s has no Done when list", d.Title())
	}
	for _, p := range w.b.Parts(d) {
		if s := w.b.Status(p); !Closed(s) {
			return nil, fmt.Errorf("%s has a part that is %s: %s; finish it or drop it first", d.Title(), s, p.Title())
		}
	}
	prose := map[string]string{"Delivered": r.Delivered, "Verified": r.Verified, "Follow-ups": r.FollowUps, "Learned": r.Learned}
	if err := w.on(d, "completed", prose); err != nil {
		return nil, err
	}
	return w.finish("done "+d.Title(), d.ID())
}

// Drop drops a stub or a plan with the reason, and every open or started plan below it.
func Drop(v *vault.Vault, key, reason string, o Opts) (*Result, error) {
	if strings.TrimSpace(reason) == "" {
		return nil, errors.New("drop needs the reason")
	}
	w, err := begin(v, o)
	if err != nil {
		return nil, err
	}
	defer w.tx.Close()
	d, err := w.idx.ResolveType(key, "stub", "spec")
	if err != nil {
		return nil, err
	}
	if d.Type() == "spec" && d.Str("kind") != Plan {
		return nil, fmt.Errorf("%s is a design; a later design supersedes it", d.Title())
	}
	if s := w.b.Status(d); Closed(s) {
		return nil, fmt.Errorf("%s is %s already", d.Title(), s)
	}
	why := map[string]string{"Why": reason}
	if err := w.on(d, "dropped", why); err != nil {
		return nil, err
	}
	if d.Type() == "spec" {
		for _, p := range w.b.Below(d) {
			if !Closed(w.b.Status(p)) {
				if err := w.on(p, "dropped", map[string]string{"Why": "Dropped with " + doc.Link(d.Title()) + "."}); err != nil {
					return nil, err
				}
			}
		}
	}
	return w.finish("drop "+d.Title(), d.ID())
}

// Reopen takes up a done, dropped, or resolved stub or plan again.
func Reopen(v *vault.Vault, key, reason string, o Opts) (*Result, error) {
	w, err := begin(v, o)
	if err != nil {
		return nil, err
	}
	defer w.tx.Close()
	d, err := w.idx.ResolveType(key, "stub", "spec")
	if err != nil {
		return nil, err
	}
	if d.Type() == "spec" && d.Str("kind") != Plan {
		return nil, fmt.Errorf("%s is a design; it is not closed", d.Title())
	}
	if s := w.b.Status(d); !Closed(s) {
		return nil, fmt.Errorf("%s is %s; reopen takes up closed work", d.Title(), s)
	}
	var prose map[string]string
	if strings.TrimSpace(reason) != "" {
		prose = map[string]string{"Why": reason}
	}
	if err := w.on(d, "reopened", prose); err != nil {
		return nil, err
	}
	return w.finish("reopen "+d.Title(), d.ID())
}

// Block marks a plan blocked, with the one line it waits on.
func Block(v *vault.Vault, key, reason string, o Opts) (*Result, error) {
	reason = oneLine(reason, 180)
	if reason == "" {
		return nil, errors.New("block needs the reason: what the plan waits on, in one line")
	}
	w, err := begin(v, o)
	if err != nil {
		return nil, err
	}
	defer w.tx.Close()
	d, err := w.plan(key)
	if err != nil {
		return nil, err
	}
	if s := w.b.Status(d); Closed(s) {
		return nil, fmt.Errorf("%s is %s", d.Title(), s)
	}
	if err := w.event(EventIn{Kind: "blocked", SubjectTitle: d.Title(), SubjectID: d.ID(), SubjectTags: d.List("tags"), Line: reason}); err != nil {
		return nil, err
	}
	return w.finish("block "+d.Title(), d.ID())
}

// Unblock clears a plan's block.
func Unblock(v *vault.Vault, key string, o Opts) (*Result, error) {
	w, err := begin(v, o)
	if err != nil {
		return nil, err
	}
	defer w.tx.Close()
	d, err := w.plan(key)
	if err != nil {
		return nil, err
	}
	if w.b.Blocked(d) == "" {
		return nil, fmt.Errorf("%s is not blocked", d.Title())
	}
	if err := w.on(d, "unblocked", nil); err != nil {
		return nil, err
	}
	return w.finish("unblock "+d.Title(), d.ID())
}

// Note records a note about any document of wiki/documents.
func Note(v *vault.Vault, key, text string, o Opts) (*Result, error) {
	if strings.TrimSpace(text) == "" {
		return nil, errors.New("note needs text")
	}
	w, err := begin(v, o)
	if err != nil {
		return nil, err
	}
	defer w.tx.Close()
	d, err := w.idx.Resolve(key)
	if err != nil {
		return nil, err
	}
	if !schema.IsDocument(d.Type()) {
		return nil, fmt.Errorf("%s is a %s; a note is about a document of the wiki", vault.Title(d), d.Type())
	}
	if err := w.on(d, "note", map[string]string{"Note": text}); err != nil {
		return nil, err
	}
	return w.finish("note on "+d.Title(), d.ID())
}

// Set changes the fields of a stub or a spec. A new title renames the file and rewrites
// every link to it.
func Set(v *vault.Vault, in SetIn, o Opts) (*Result, error) {
	w, err := begin(v, o)
	if err != nil {
		return nil, err
	}
	defer w.tx.Close()
	d, err := w.idx.ResolveType(in.Doc, "stub", "spec")
	if err != nil {
		return nil, err
	}
	plan := d.Type() == "spec" && d.Str("kind") == Plan
	design := d.Type() == "spec" && d.Str("kind") == Design
	content := d.Content
	set := func(k string, v any) { content = doc.SetField(content, k, v) }
	if in.Description != nil {
		if strings.TrimSpace(*in.Description) == "" {
			return nil, errors.New("description: one sentence is required")
		}
		set("description", oneLine(*in.Description, 200))
	}
	if in.Tags != nil {
		tg, err := w.tagList(*in.Tags, in.NewTags)
		if err != nil {
			return nil, err
		}
		set("tags", tg)
	}
	if in.Aliases != nil {
		for _, a := range *in.Aliases {
			for _, p := range w.idx.TitleHolders(a) {
				if p != d.Path {
					return nil, fmt.Errorf("aliases: %q is held by %s", a, p)
				}
			}
		}
		set("aliases", nonNil(*in.Aliases))
	}
	if in.Priority != nil {
		if d.Type() == "spec" && !plan {
			return nil, errors.New("a design has no priority")
		}
		if err := checkPriority(*in.Priority); err != nil {
			return nil, err
		}
		set("priority", orDefault(*in.Priority, "normal"))
	}
	structural := in.Parent != nil || in.Depends != nil || in.Repositories != nil
	if structural && plan && Closed(w.b.Status(d)) {
		return nil, fmt.Errorf("%s is %s; its parent, depends, and repositories stay as they were (work reopen first)", d.Title(), w.b.Status(d))
	}
	if (in.Parent != nil || in.Depends != nil || in.Order != nil || in.Implements != nil || in.Repositories != nil) && d.Type() == "stub" {
		return nil, errors.New("a stub has no parent, depends, order, implements, or repositories; promote it to a spec first")
	}
	if in.Repositories != nil {
		repos, err := w.repos(*in.Repositories)
		if err != nil {
			return nil, err
		}
		set("repositories", repos)
	}
	if in.Parent != nil {
		if design {
			return nil, errors.New("a design takes no parent")
		}
		link := ""
		if strings.TrimSpace(*in.Parent) != "" {
			p, err := w.plan(*in.Parent)
			if err != nil {
				return nil, fmt.Errorf("parent: %w", err)
			}
			if p.ID() == d.ID() || slices.ContainsFunc(w.b.Below(d), func(x *doc.Doc) bool { return x.ID() == p.ID() }) {
				return nil, fmt.Errorf("parent: %s lies below %s; the parents would loop", p.Title(), d.Title())
			}
			if Closed(w.b.Status(p)) {
				return nil, fmt.Errorf("parent: %s is %s", p.Title(), w.b.Status(p))
			}
			link = doc.Link(p.Title())
		}
		set("parent", link)
		set("depends", []string{})
	}
	if in.Depends != nil {
		if design {
			return nil, errors.New("a design takes no depends")
		}
		parent := doc.LinkTarget(doc.Parse(d.Path, []byte(content)).Str("parent"))
		var deps []string
		for _, dep := range *in.Depends {
			s, err := w.plan(dep)
			if err != nil {
				return nil, fmt.Errorf("depends: %w", err)
			}
			sp := ""
			if p := w.b.ParentOf(s); p != nil {
				sp = p.Title()
			}
			if !strings.EqualFold(sp, parent) || s.ID() == d.ID() {
				return nil, fmt.Errorf("depends: %s is no sibling of %s", s.Title(), d.Title())
			}
			deps = append(deps, doc.Link(s.Title()))
		}
		if loops(w.b, d, deps) {
			return nil, errors.New("depends: the plans would wait on each other")
		}
		set("depends", nonNil(deps))
	}
	if in.Order != nil {
		if !plan {
			return nil, errors.New("only a plan has an order")
		}
		set("order", *in.Order)
	}
	if in.Implements != nil {
		if !plan {
			return nil, errors.New("only a plan implements a design")
		}
		l, err := w.specsOf(*in.Implements, Design, "implements")
		if err != nil {
			return nil, err
		}
		set("implements", l)
	}
	if in.Supersedes != nil {
		if !design {
			return nil, errors.New("only a design supersedes a design")
		}
		link := ""
		if strings.TrimSpace(*in.Supersedes) != "" {
			l, err := w.specsOf([]string{*in.Supersedes}, Design, "supersedes")
			if err != nil {
				return nil, err
			}
			link = l[0]
		}
		set("supersedes", link)
	}
	target := d.Path
	title := d.Title()
	if in.Title != nil && doc.CleanTitle(*in.Title) != d.Title() {
		w.titles.Release(d.Title())
		if title, err = w.title(*in.Title); err != nil {
			return nil, err
		}
		target = vault.DocPath(title)
	}
	set("updated", vault.Stamp(w.now))
	if target != d.Path {
		if err := w.tx.Remove(d.Path); err != nil {
			return nil, err
		}
		if err := w.rename(d, title, map[string]string{}); err != nil {
			return nil, err
		}
	}
	if err := w.tx.Write(target, []byte(content)); err != nil {
		return nil, err
	}
	w.wrote = append(w.wrote, d.ID())
	return w.finish("set "+title, d.ID())
}

// loops reports whether d's new depends would make siblings wait on each other.
func loops(b *Board, d *doc.Doc, deps []string) bool {
	seen := map[string]bool{}
	var visit func(links []string) bool
	visit = func(links []string) bool {
		for _, l := range links {
			s := b.Idx.Linked(l)
			if s == nil {
				continue
			}
			if s.ID() == d.ID() {
				return true
			}
			if seen[s.ID()] {
				continue
			}
			seen[s.ID()] = true
			if visit(s.List("depends")) {
				return true
			}
		}
		return false
	}
	return visit(deps)
}

// Show is the view of a stub or a spec.
func Show(idx *vault.Index, key string) (*View, error) {
	d, err := idx.ResolveType(key, "stub", "spec")
	if err != nil {
		return nil, err
	}
	return Load(idx).View(d), nil
}
