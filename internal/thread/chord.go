package thread

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

// ChordThreadIn is one thread of a new chord: a new stub, or a stub that exists.
type ChordThreadIn struct {
	Thread      string   `json:"thread,omitempty" jsonschema:"a stub that exists, by id or title; it joins the chord"`
	Title       string   `json:"title,omitempty" jsonschema:"a new stub's title"`
	Text        string   `json:"text,omitempty" jsonschema:"a new stub's Idea: what this thread is for, in the user's words where they gave them"`
	Description string   `json:"description,omitempty"`
	Priority    string   `json:"priority,omitempty"`
	After       []string `json:"after,omitempty" jsonschema:"the threads that must be verified first: titles of this call, or ids or titles of stubs that exist"`
}

// ChordIn makes a chord, with its threads when the call gives them.
type ChordIn struct {
	Title       string          `json:"title" jsonschema:"the chord's title; unique in the vault"`
	Text        string          `json:"text" jsonschema:"the chord's Goal: what is true when every thread is closed"`
	Description string          `json:"description,omitempty" jsonschema:"one sentence; the first sentence of text when empty"`
	Tags        []string        `json:"tags,omitempty" jsonschema:"the chord's tags; each new stub takes them"`
	Priority    string          `json:"priority,omitempty" jsonschema:"high, normal, low, or someday"`
	Threads     []ChordThreadIn `json:"threads,omitempty" jsonschema:"the threads, each with the threads it comes after"`
	NewTags     bool            `json:"new_tags,omitempty"`
}

// OrderIn is the threads one thread of a chord comes after.
type OrderIn struct {
	Thread string   `json:"thread" jsonschema:"a thread of the chord, by id or title"`
	After  []string `json:"after" jsonschema:"the threads it comes after; an empty list for none"`
}

// chord resolves a chord that is not dropped.
func (w *writer) chord(key string) (*doc.Doc, error) {
	c, err := w.idx.ResolveType(key, "chord")
	if err != nil {
		return nil, fmt.Errorf("chord: %w", err)
	}
	if w.b.Status(c) == ChordDropped {
		return nil, fmt.Errorf("chord: %s is dropped; chord reopen takes it up again", c.Title())
	}
	return c, nil
}

// ChordCreate makes a chord and its threads in one commit.
func ChordCreate(v *vault.Vault, in ChordIn, o Opts) (*Result, error) {
	goal := strings.TrimSpace(in.Text)
	if goal == "" {
		return nil, errors.New("a chord needs text: its goal, what is true when every thread is closed")
	}
	w, err := begin(v, o)
	if err != nil {
		return nil, err
	}
	defer w.tx.Close()
	title, err := w.title(in.Title)
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
	body := goal + "\n"
	if _, ok := doc.Section(goal, "Goal"); !ok {
		body = "## Goal\n\n" + goal + "\n"
	}
	if err := CheckSections("chord", body); err != nil {
		return nil, err
	}
	desc := strings.TrimSpace(in.Description)
	if desc == "" {
		text, _ := doc.Section(body, "Goal")
		desc = firstSentence(text)
	}
	extra := []doc.Field{{Key: "priority", Value: orDefault(in.Priority, "normal")}, {Key: "status", Value: ChordOpen}, {Key: "threads", Value: "0/0"}, {Key: "canvas", Value: ""}}
	id, _, err := w.newDoc("chord", title, desc, tg, extra, body)
	if err != nil {
		return nil, err
	}
	// Each thread of the call gets its title first, so after may name a later one.
	type member struct {
		in    ChordThreadIn
		title string
		have  *doc.Doc
	}
	members := make([]*member, len(in.Threads))
	byTitle := map[string]*member{}
	for i, t := range in.Threads {
		m := &member{in: t}
		switch {
		case strings.TrimSpace(t.Thread) != "":
			th, err := w.open(t.Thread)
			if err != nil {
				return nil, fmt.Errorf("thread %d: %w", i+1, err)
			}
			if c := w.b.Chord(th.Stub); c != nil {
				return nil, fmt.Errorf("thread %d: %s belongs to the chord %s; a thread belongs to one chord (chord remove takes it out)", i+1, th.Stub.Title(), c.Title())
			}
			m.have, m.title = th.Stub, th.Stub.Title()
		case strings.TrimSpace(t.Text) == "":
			return nil, fmt.Errorf("thread %d needs text (a new stub's idea) or thread (a stub that exists)", i+1)
		default:
			raw := t.Title
			if strings.TrimSpace(raw) == "" {
				raw = TitleFromText(t.Text)
			}
			if m.title, err = w.title(raw); err != nil {
				return nil, fmt.Errorf("thread %d: %w", i+1, err)
			}
			if err := checkPriority(t.Priority); err != nil {
				return nil, fmt.Errorf("thread %d: %w", i+1, err)
			}
		}
		if byTitle[strings.ToLower(m.title)] != nil {
			return nil, fmt.Errorf("thread %d: %s is in the call twice", i+1, m.title)
		}
		members[i] = m
		byTitle[strings.ToLower(m.title)] = m
	}
	after := map[string][]string{} // lower title → the titles it comes after
	for _, m := range members {
		var list []string
		for _, a := range m.in.After {
			name := doc.LinkTarget(a)
			if other := byTitle[strings.ToLower(doc.CleanTitle(name))]; other != nil {
				name = other.title
			} else {
				th, err := w.thread(a)
				if err != nil {
					return nil, fmt.Errorf("%s: after: %w", m.title, err)
				}
				name = th.Stub.Title()
			}
			if strings.EqualFold(name, m.title) {
				return nil, fmt.Errorf("%s: after: a thread does not come after itself", m.title)
			}
			if !slices.Contains(list, name) {
				list = append(list, name)
			}
		}
		after[strings.ToLower(m.title)] = list
	}
	if loop := titleLoop(after); loop != "" {
		return nil, fmt.Errorf("the threads would wait on each other: %s", loop)
	}
	for _, m := range members {
		links := doc.Links(after[strings.ToLower(m.title)])
		if m.have != nil {
			content := doc.SetFields(m.have.Content, []doc.Field{{Key: "chord", Value: doc.Link(title)}, {Key: "updated", Value: vault.Stamp(w.now)}})
			if len(m.in.After) > 0 {
				content = doc.SetField(content, "after", links)
			}
			if err := w.tx.Write(m.have.Path, []byte(content)); err != nil {
				return nil, err
			}
			w.wrote = append(w.wrote, m.have.ID())
			continue
		}
		desc := strings.TrimSpace(m.in.Description)
		if desc == "" {
			desc = firstSentence(m.in.Text)
		}
		if _, _, err := w.newDoc("stub", m.title, desc, tg, stubFields(m.in.Priority, doc.Link(title), links), "## Idea\n\n"+strings.TrimSpace(m.in.Text)+"\n"); err != nil {
			return nil, err
		}
	}
	subject := "chord " + title
	if n := len(members); n > 0 {
		subject += fmt.Sprintf(" with %d %s", n, plural(n, "thread", "threads"))
	}
	return w.finish(subject, id)
}

// titleLoop names a cycle among the titles of one call, or "".
func titleLoop(after map[string][]string) string {
	state := map[string]int{}
	var walk func(t string, trail []string) string
	walk = func(t string, trail []string) string {
		k := strings.ToLower(t)
		switch state[k] {
		case 1:
			return strings.Join(append(trail, t), " waits on ")
		case 2:
			return ""
		}
		state[k] = 1
		for _, a := range after[k] {
			if l := walk(a, append(trail, t)); l != "" {
				return l
			}
		}
		state[k] = 2
		return ""
	}
	for t := range after {
		if l := walk(t, nil); l != "" {
			return l
		}
	}
	return ""
}

// member resolves a thread of a chord.
func (w *writer) member(chord *doc.Doc, key string) (*doc.Doc, error) {
	t, err := w.thread(key)
	if err != nil {
		return nil, err
	}
	if c := w.b.Chord(t.Stub); c == nil || c.ID() != chord.ID() {
		return nil, fmt.Errorf("%s is no thread of %s", t.Stub.Title(), chord.Title())
	}
	return t.Stub, nil
}

// ChordAdd puts a stub into a chord, after the threads given.
func ChordAdd(v *vault.Vault, chord, key string, after []string, o Opts) (*Result, error) {
	w, err := begin(v, o)
	if err != nil {
		return nil, err
	}
	defer w.tx.Close()
	c, err := w.chord(chord)
	if err != nil {
		return nil, err
	}
	t, err := w.open(key)
	if err != nil {
		return nil, err
	}
	d := t.Stub
	if have := w.b.Chord(d); have != nil && have.ID() != c.ID() {
		return nil, fmt.Errorf("%s belongs to the chord %s; a thread belongs to one chord (chord remove takes it out)", d.Title(), have.Title())
	}
	content := doc.SetFields(d.Content, []doc.Field{{Key: "chord", Value: doc.Link(c.Title())}, {Key: "updated", Value: vault.Stamp(w.now)}})
	if after != nil {
		links, docs, err := w.afterLinks(after, d.ID())
		if err != nil {
			return nil, err
		}
		if loop := w.b.loop(map[string][]*doc.Doc{d.ID(): docs}); loop != "" {
			return nil, fmt.Errorf("after: the threads would wait on each other: %s", loop)
		}
		content = doc.SetField(content, "after", links)
	}
	if err := w.tx.Write(d.Path, []byte(content)); err != nil {
		return nil, err
	}
	w.wrote = append(w.wrote, d.ID())
	return w.finish("add "+d.Title()+" to "+c.Title(), c.ID())
}

// ChordRemove takes a thread out of its chord. The thread stays, and keeps its after.
func ChordRemove(v *vault.Vault, chord, key string, o Opts) (*Result, error) {
	w, err := begin(v, o)
	if err != nil {
		return nil, err
	}
	defer w.tx.Close()
	c, err := w.idx.ResolveType(chord, "chord")
	if err != nil {
		return nil, err
	}
	d, err := w.member(c, key)
	if err != nil {
		return nil, err
	}
	content := doc.SetFields(d.Content, []doc.Field{{Key: "chord", Value: ""}, {Key: "updated", Value: vault.Stamp(w.now)}})
	if err := w.tx.Write(d.Path, []byte(content)); err != nil {
		return nil, err
	}
	w.wrote = append(w.wrote, d.ID())
	return w.finish("remove "+d.Title()+" from "+c.Title(), c.ID())
}

// ChordOrder sets the threads each given thread of a chord comes after.
func ChordOrder(v *vault.Vault, chord string, order []OrderIn, o Opts) (*Result, error) {
	if len(order) == 0 {
		return nil, errors.New("order needs order: each thread with the threads it comes after")
	}
	w, err := begin(v, o)
	if err != nil {
		return nil, err
	}
	defer w.tx.Close()
	c, err := w.chord(chord)
	if err != nil {
		return nil, err
	}
	override := map[string][]*doc.Doc{}
	links := map[string][]string{}
	var stubs []*doc.Doc
	for _, e := range order {
		d, err := w.member(c, e.Thread)
		if err != nil {
			return nil, err
		}
		l, docs, err := w.afterLinks(e.After, d.ID())
		if err != nil {
			return nil, fmt.Errorf("%s: %w", d.Title(), err)
		}
		if _, dup := override[d.ID()]; dup {
			return nil, fmt.Errorf("%s is in the order twice", d.Title())
		}
		override[d.ID()], links[d.ID()] = docs, l
		stubs = append(stubs, d)
	}
	if loop := w.b.loop(override); loop != "" {
		return nil, fmt.Errorf("the threads would wait on each other: %s", loop)
	}
	if err := w.setAfter(stubs, links); err != nil {
		return nil, err
	}
	return w.finish("order "+c.Title(), c.ID())
}

func (w *writer) setAfter(stubs []*doc.Doc, links map[string][]string) error {
	for _, d := range stubs {
		content := doc.SetField(d.Content, "after", links[d.ID()])
		if content == d.Content {
			continue
		}
		content = doc.SetField(content, "updated", vault.Stamp(w.now))
		if err := w.tx.Write(d.Path, []byte(content)); err != nil {
			return err
		}
		w.wrote = append(w.wrote, d.ID())
	}
	return nil
}

// CanvasSave makes the stubs hold the order a chord's canvas shows: each stub on the
// canvas joins the chord, a stub taken off it leaves the chord, and each arrow from A to
// B puts B after A. A link from a thread to one outside the canvas stays.
func CanvasSave(v *vault.Vault, chord string, o Opts) (*Result, error) {
	w, err := begin(v, o)
	if err != nil {
		return nil, err
	}
	defer w.tx.Close()
	c, err := w.chord(chord)
	if err != nil {
		return nil, err
	}
	cv, exists, err := w.b.readCanvas(c)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("%s has no canvas at %s; a sync writes it once the chord has a thread", c.Title(), CanvasPath(c))
	}
	g, _ := w.b.canvasGraph(cv)
	was := w.b.chordGraph(c)
	override := map[string][]*doc.Doc{}
	contents := map[string]string{}
	var changed []*doc.Doc
	for id := range g.members {
		d := w.idx.ByID(id)
		if have := w.b.Chord(d); have != nil && have.ID() != c.ID() {
			return nil, fmt.Errorf("%s belongs to the chord %s; a thread belongs to one chord. Take it off this canvas, or out of that chord (chord remove)", d.Title(), have.Title())
		}
		if s := w.b.Status(d); s == Dropped || s == Resolved {
			if !was.members[id] {
				return nil, fmt.Errorf("%s is %s; reopen it before it joins a chord", d.Title(), s)
			}
		}
		// The links to threads off the canvas stay; the arrows give the rest.
		var after []*doc.Doc
		for _, a := range w.b.After(d) {
			if !g.members[a.ID()] {
				after = append(after, a)
			}
		}
		for e := range g.edges {
			from, to, _ := strings.Cut(e, ">")
			if to == id {
				after = append(after, w.idx.ByID(from))
			}
		}
		slices.SortStableFunc(after, func(x, y *doc.Doc) int {
			return strings.Compare(strings.ToLower(x.Title()), strings.ToLower(y.Title()))
		})
		override[id] = after
		content := doc.SetFields(d.Content, []doc.Field{{Key: "chord", Value: doc.Link(c.Title())}, {Key: "after", Value: doc.Links(titleList(after))}})
		if content != d.Content {
			contents[d.Path] = content
			changed = append(changed, d)
		}
	}
	for id := range was.members {
		if g.members[id] {
			continue
		}
		d := w.idx.ByID(id)
		contents[d.Path] = doc.SetField(d.Content, "chord", "")
		changed = append(changed, d)
	}
	if loop := w.b.loop(override); loop != "" {
		return nil, fmt.Errorf("the arrows make a loop: %s. Remove one arrow, then save again", loop)
	}
	for _, d := range changed {
		content := doc.SetField(contents[d.Path], "updated", vault.Stamp(w.now))
		if err := w.tx.Write(d.Path, []byte(content)); err != nil {
			return nil, err
		}
		w.wrote = append(w.wrote, d.ID())
	}
	return w.finish("order "+c.Title()+" from its canvas", c.ID())
}

// CanvasWrite writes a chord's canvas from its stubs, over what the user drew; tidy also
// places every card again.
func CanvasWrite(v *vault.Vault, chord string, tidy bool, o Opts) (*Result, error) {
	w, err := begin(v, o)
	if err != nil {
		return nil, err
	}
	defer w.tx.Close()
	c, err := w.chord(chord)
	if err != nil {
		return nil, err
	}
	w.force[c.ID()] = true
	w.tidy[c.ID()] = tidy
	return w.finish("canvas of "+c.Title(), c.ID())
}
