package thread

import (
	"errors"
	"fmt"
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

// State is where a thread stands after a call.
type State struct {
	Thread  vault.Ref `json:"thread"`
	Missing []string  `json:"missing" jsonschema:"what stands between the thread and closed"`
	Next    Step      `json:"next"`
}

// Result is what a write returns: the state of the thread or the chord it acted on, the
// commit, and the documents and events it wrote, which the hook links to the session.
type Result struct {
	State   *State      `json:"state,omitempty"`
	Chord   *ChordView  `json:"chord,omitempty"`
	Commit  string      `json:"commit,omitempty"`
	Wrote   []vault.Ref `json:"wrote"`
	Events  []vault.Ref `json:"events"`
	Started string      `json:"started,omitempty" jsonschema:"the thread that start bound this session to"`
}

// StubIn plants a stub.
type StubIn struct {
	Text        string   `json:"text" jsonschema:"the user's words, as given; they become the stub's Idea"`
	Title       string   `json:"title,omitempty" jsonschema:"a short name; the first line of text when empty"`
	Description string   `json:"description,omitempty" jsonschema:"one sentence; the first sentence of text when empty"`
	Tags        []string `json:"tags,omitempty" jsonschema:"the categories: tags like work/p3 or self-driving"`
	Priority    string   `json:"priority,omitempty" jsonschema:"high, normal, low, or someday"`
	Chord       string   `json:"chord,omitempty" jsonschema:"the chord the thread belongs to, by id or title"`
	After       []string `json:"after,omitempty" jsonschema:"the threads that must be verified first, by id or title"`
	Inbox       string   `json:"inbox,omitempty" jsonschema:"a note in inbox/ that the stub replaces; it leaves the inbox in the same commit"`
	NewTags     bool     `json:"new_tags,omitempty" jsonschema:"allow a tag no document holds, in tagging: known; set it only after the user agreed"`
}

// SpecIn writes or revises a thread's spec.
type SpecIn struct {
	Thread      string `json:"thread" jsonschema:"the thread, by id or title"`
	Text        string `json:"text" jsonschema:"the spec's body: Goal, Requirements (each line '- R1: …'), Rules, Decisions, Out of scope, Knowledge, Open questions"`
	Description string `json:"description,omitempty" jsonschema:"one sentence; the first sentence of the Goal when empty"`
}

// TaskIn is one new task.
type TaskIn struct {
	Text         string   `json:"text" jsonschema:"the task, in one line"`
	Requirements []string `json:"requirements" jsonschema:"the requirements the task serves: R1, R2"`
	Details      string   `json:"details,omitempty" jsonschema:"where, how, and what to watch for"`
}

// TasksIn writes a thread's task list for one repository, or appends to it.
type TasksIn struct {
	Thread     string   `json:"thread" jsonschema:"the thread, by id or title"`
	Repository string   `json:"repository,omitempty" jsonschema:"the repository the tasks change, by id or title; empty for work in no repository"`
	Tasks      []TaskIn `json:"tasks" jsonschema:"the tasks, in order"`
}

// CheckIn checks, drops, or opens one task.
type CheckIn struct {
	Thread  string   `json:"thread" jsonschema:"the thread, by id or title"`
	Task    string   `json:"task" jsonschema:"the task's id: T3"`
	State   string   `json:"state,omitempty" jsonschema:"done (the default), dropped, or open"`
	Commits []string `json:"commits,omitempty" jsonschema:"done: the commits that did the task"`
	Note    string   `json:"note,omitempty" jsonschema:"done: one line on what was done; needed when there is no commit"`
	Reason  string   `json:"reason,omitempty" jsonschema:"dropped: why"`
}

// ResultIn is a verification's result for one requirement.
type ResultIn struct {
	Requirement string `json:"requirement" jsonschema:"the requirement's id: R1"`
	Result      string `json:"result" jsonschema:"pass or fail"`
	Evidence    string `json:"evidence" jsonschema:"the command and its output, the file and line, or the commit that shows it"`
}

// VerifyIn files one verification of a thread.
type VerifyIn struct {
	Thread   string     `json:"thread" jsonschema:"the thread, by id or title"`
	Scope    string     `json:"scope" jsonschema:"what was checked: each repository with its commits"`
	Results  []ResultIn `json:"results" jsonschema:"one result for every requirement of the spec"`
	Findings []string   `json:"findings,omitempty" jsonschema:"what the check found that the spec or the tasks did not foresee, one line each"`
	Notes    string     `json:"notes,omitempty"`
}

// FindingIn gives one open finding of a thread's last verification its outcome.
type FindingIn struct {
	Thread     string  `json:"thread" jsonschema:"the thread, by id or title"`
	Finding    string  `json:"finding" jsonschema:"the finding's id: F1"`
	Outcome    string  `json:"outcome" jsonschema:"task (a new task fixes it), spec (the spec changed for it), stub (work for another thread), knowledge (the wiki takes it), or accepted (the user accepts it as it is)"`
	Task       *TaskIn `json:"task,omitempty" jsonschema:"task: the new task"`
	Repository string  `json:"repository,omitempty" jsonschema:"task: the repository of the task list it joins"`
	Link       string  `json:"link,omitempty" jsonschema:"stub: a stub that exists; knowledge: the change or the topic that holds it"`
	Text       string  `json:"text,omitempty" jsonschema:"stub: the words of a new stub, when link names none"`
	Reason     string  `json:"reason,omitempty" jsonschema:"accepted: why, in the user's words"`
}

// SetIn changes the fields of a stub or a chord. A field left out stays.
type SetIn struct {
	Doc         string    `json:"doc" jsonschema:"the thread or the chord, by id or title"`
	Title       *string   `json:"title,omitempty" jsonschema:"a new title; the thread's documents and every link follow"`
	Description *string   `json:"description,omitempty"`
	Tags        *[]string `json:"tags,omitempty"`
	Aliases     *[]string `json:"aliases,omitempty"`
	Priority    *string   `json:"priority,omitempty"`
	Chord       *string   `json:"chord,omitempty" jsonschema:"a thread's chord; empty takes the thread out of its chord"`
	After       *[]string `json:"after,omitempty" jsonschema:"the threads that must be verified first; an empty list clears it"`
	NewTags     bool      `json:"new_tags,omitempty"`
}

// writer is one write: the transaction, and the vault as it was when it began.
type writer struct {
	v      *vault.Vault
	tx     *vault.Tx
	idx    *vault.Index
	b      *Board
	now    time.Time
	by     string
	titles *Titles
	wrote  []string // ids of the documents written
	events []string // ids of the events written
	newTag map[string]bool
	last   map[string]time.Time // subject id → the time of its last event
	// force and tidy name the chords whose canvas the write lays out again.
	force map[string]bool
	tidy  map[string]bool
	// joined names the stubs that join a chord in this write.
	joined map[string]bool
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
	return &writer{v: v, tx: tx, idx: idx, b: Load(idx), now: now.Truncate(time.Second), by: by, titles: NewTitles(idx), newTag: map[string]bool{}, force: map[string]bool{}, tidy: map[string]bool{}, joined: map[string]bool{}}, nil
}

// StateOf is where a thread stands.
func (b *Board) StateOf(stub *doc.Doc) *State {
	return &State{Thread: b.Ref(stub), Missing: b.Missing(stub), Next: b.Next(stub)}
}

// finish syncs the derived parts, commits, and returns the state of the document focus
// (an id): a thread, a part of one, or a chord.
func (w *writer) finish(subject, focus string) (*Result, error) {
	idx, err := vault.Load(w.v)
	if err != nil {
		return nil, err
	}
	nb := Load(idx)
	nb.ForceCanvas, nb.TidyCanvas, nb.Joined = w.force, w.tidy, w.joined
	if _, err := nb.Sync(vault.Guarded(idx, w.tx.WriteIfChanged, nil)); err != nil {
		return nil, err
	}
	sha, err := w.tx.Commit("thread: "+oneLine(subject, 72), Trailer+": "+focus)
	if err != nil {
		return nil, err
	}
	if idx, err = vault.Load(w.v); err != nil {
		return nil, err
	}
	b := Load(idx)
	res := &Result{Commit: sha, Wrote: []vault.Ref{}, Events: []vault.Ref{}}
	if d := idx.ByID(focus); d != nil {
		switch {
		case d.Type() == "chord":
			cv := b.ChordView(d)
			res.Chord = &cv
		case b.Thread(d) != nil:
			res.State = b.StateOf(b.Thread(d).Stub)
		}
	}
	seen := map[string]bool{}
	for _, id := range w.wrote {
		if d := idx.ByID(id); d != nil && !seen[id] {
			seen[id] = true
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

// named checks a title the caller gave, for a stub or a chord: the shared title check,
// then title.
func (w *writer) named(raw string) (string, error) {
	if err := doc.CheckTitle(doc.CleanTitle(raw)); err != nil {
		return "", err
	}
	return w.title(raw)
}

// title checks a new title, given or derived: clean, short enough for a file name, not a
// view's, and free in the vault and this write.
func (w *writer) title(raw string) (string, error) {
	t := doc.CleanTitle(raw)
	switch {
	case t == "":
		return "", errors.New("a title is needed")
	case len(t)+len(".md") > vault.MaxNameBytes:
		return "", fmt.Errorf("the title %.40q… would make a file name of %d bytes, and a file name holds at most %d; give the thread a shorter title (thread set)", t, len(t)+len(".md"), vault.MaxNameBytes)
	case vault.ReservedTitle(t):
		return "", fmt.Errorf("%q begins as a view's title does; choose another", t)
	case !w.titles.Free(t):
		holders := w.idx.TitleHolders(t)
		if len(holders) == 0 && w.v.Occupied(vault.DocPath(t)) {
			return "", fmt.Errorf("the title %q names a file that already exists on disk under another case or Unicode form (%s); choose another title", t, w.v.OnDisk(vault.DocPath(t)))
		}
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

// repo resolves a linked repository document, or nil for an empty key.
func (w *writer) repo(key string) (*doc.Doc, error) {
	if strings.TrimSpace(key) == "" {
		return nil, nil
	}
	d, err := w.idx.ResolveType(key, "repository")
	if err != nil {
		return nil, fmt.Errorf("repository: %w", err)
	}
	if d.Front.Bool("unlinked") {
		return nil, fmt.Errorf("repository: %s is unlinked; link it again first (repo-link)", d.Title())
	}
	return d, nil
}

// thread resolves the thread a key names: its stub, or one of its documents.
func (w *writer) thread(key string) (*Thread, error) {
	d, err := w.idx.ResolveType(key, "stub", "spec", "tasks", "verification")
	if err != nil {
		return nil, err
	}
	t := w.b.Thread(d)
	if t == nil {
		return nil, fmt.Errorf("%s names no thread: its thread field links no stub", d.Title())
	}
	return t, nil
}

// open resolves a thread that is not dropped or resolved.
func (w *writer) open(key string) (*Thread, error) {
	t, err := w.thread(key)
	if err != nil {
		return nil, err
	}
	if s := w.b.Status(t.Stub); s == Dropped || s == Resolved {
		return nil, fmt.Errorf("%s is %s; thread reopen takes it up again", t.Stub.Title(), s)
	}
	return t, nil
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
	if len([]rune(line)) > 60 {
		r := []rune(line)[:60]
		line = strings.TrimSpace(string(r))
		if i := strings.LastIndex(string(r), " "); i > 20 {
			line = strings.TrimSpace(string(r)[:i])
		}
	}
	// Sixty runes of a script with wide characters pass the byte limit of a title.
	return doc.CutTitle(line, doc.MaxTitleBytes)
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

// Titles of a thread's documents, from the stub's title.
func SpecTitle(stub string) string { return stub + " · Spec" }

// TasksTitle is the title of a thread's task list for a repository, or for none.
func TasksTitle(stub, repository string) string {
	if repository == "" {
		return stub + " · Tasks"
	}
	return stub + " · Tasks (" + repository + ")"
}

// VerificationTitle is the title of a thread's verification of a round.
func VerificationTitle(stub string, round int) string {
	return fmt.Sprintf("%s · Verification %d", stub, round)
}

// CheckSections refuses a body with a level-two heading its type does not name, so a
// document holds only what it is for.
func CheckSections(typ, text string) error {
	t := schema.Get(typ)
	for _, h := range doc.Headings(text) {
		if h.Level != 2 {
			continue
		}
		if !slices.ContainsFunc(t.Sections, func(s string) bool { return strings.EqualFold(s, h.Title) }) {
			return fmt.Errorf("## %s is no section of a %s; a %s holds %s. %s", h.Title, typeName(typ), typeName(typ), strings.Join(t.Sections, ", "), Elsewhere)
		}
	}
	return nil
}

// Elsewhere says where the content a thread document refuses belongs.
const Elsewhere = "Tasks go to a task list (thread tasks), what the work found to a verification (thread verify), the story of a session to its session document, and lasting knowledge to the wiki (a change)"

func typeName(typ string) string {
	if typ == "tasks" {
		return "task list"
	}
	return typ
}

// newDoc renders a stub or a chord, or a part of a thread.
func (w *writer) newDoc(typ, title, description string, tagList []string, extra []doc.Field, body string) (id, rel string, err error) {
	id = doc.NewID(schema.DocPrefix, func(s string) bool { return w.idx.ByID(s) != nil })
	stamp := vault.Stamp(w.now)
	fields := append([]doc.Field{
		{Key: "id", Value: id},
		{Key: "type", Value: typ},
		{Key: "description", Value: oneLine(description, 200)},
		{Key: "tags", Value: nonNil(tagList)},
		{Key: "aliases", Value: []string{}},
		{Key: "created", Value: stamp},
		{Key: "updated", Value: stamp},
		{Key: "refreshed", Value: stamp},
	}, extra...)
	rel = vault.DocPath(title)
	if err := w.tx.Write(rel, []byte(doc.Render(fields, body))); err != nil {
		return "", "", err
	}
	w.wrote = append(w.wrote, id)
	return id, rel, nil
}

// stubFields are the fields of a new stub past the common ones.
func stubFields(priority, chord string, after []string) []doc.Field {
	return []doc.Field{
		{Key: "priority", Value: orDefault(priority, "normal")},
		{Key: "chord", Value: chord},
		{Key: "after", Value: nonNil(after)},
		{Key: "status", Value: StatusStub},
		{Key: "spec", Value: ""},
		{Key: "tasks", Value: ""},
		{Key: "verification", Value: "none"},
		{Key: "repositories", Value: []string{}},
		{Key: "blocked", Value: ""},
		{Key: "active", Value: false},
		{Key: "rank", Value: 0},
		{Key: "became", Value: []string{}},
	}
}

// afterLinks resolves the stubs a thread comes after, as links.
func (w *writer) afterLinks(keys []string, self string) ([]string, []*doc.Doc, error) {
	links := []string{}
	var docs []*doc.Doc
	for _, k := range keys {
		if strings.TrimSpace(k) == "" {
			continue
		}
		t, err := w.thread(k)
		if err != nil {
			return nil, nil, fmt.Errorf("after: %w", err)
		}
		if t.Stub.ID() == self {
			return nil, nil, errors.New("after: a thread does not come after itself")
		}
		if l := doc.Link(t.Stub.Title()); !slices.Contains(links, l) {
			links = append(links, l)
			docs = append(docs, t.Stub)
		}
	}
	return links, docs, nil
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
	title, err := w.named(raw)
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
	chord := ""
	if strings.TrimSpace(in.Chord) != "" {
		c, err := w.chord(in.Chord)
		if err != nil {
			return nil, err
		}
		chord = doc.Link(c.Title())
		// A thread planted in a chord with no tags of its own takes the chord's.
		if len(tg) == 0 {
			tg = nonNil(c.List("tags"))
		}
	}
	after, _, err := w.afterLinks(in.After, "")
	if err != nil {
		return nil, err
	}
	if in.Inbox != "" {
		rel, err := w.v.InboxFile(in.Inbox)
		if err != nil {
			return nil, err
		}
		if err := w.tx.Remove(rel); err != nil {
			return nil, err
		}
	}
	desc := strings.TrimSpace(in.Description)
	if desc == "" {
		desc = firstSentence(text)
	}
	id, _, err := w.newDoc("stub", title, desc, tg, stubFields(in.Priority, chord, after), "## Idea\n\n"+text+"\n")
	if err != nil {
		return nil, err
	}
	if chord != "" {
		w.joined[id] = true
	}
	return w.finish("stub "+title, id)
}

// CheckSpec refuses a spec body that is not a spec: a section out of place, a check box,
// no goal, or requirements that are not numbered lines.
func CheckSpec(text string) error {
	if err := CheckSections("spec", text); err != nil {
		return err
	}
	for _, l := range strings.Split(outsideFences(text), "\n") {
		if boxLine.MatchString(l) {
			return errors.New("a spec holds no check box; it says what must be true, and the task list holds the steps (thread tasks)")
		}
	}
	if goal, _ := doc.Section(text, "Goal"); strings.TrimSpace(goal) == "" {
		return errors.New("the spec needs ## Goal: one paragraph on what the work is for")
	}
	reqs := Requirements(text)
	if len(reqs) == 0 {
		return errors.New("the spec needs ## Requirements: one line per requirement that a reviewer can check, each written `- R1: …`")
	}
	seen := map[string]bool{}
	for _, r := range reqs {
		if seen[r.ID] {
			return fmt.Errorf("two requirements are %s; each id is used once, and an id never changes", r.ID)
		}
		seen[r.ID] = true
	}
	section, _ := doc.Section(text, "Requirements")
	for _, l := range strings.Split(section, "\n") {
		if (strings.HasPrefix(l, "- ") || strings.HasPrefix(l, "* ")) && !reqLine.MatchString(l) {
			return fmt.Errorf("under ## Requirements, write each requirement as `- R1: …`; this line has no id: %s", oneLine(l, 80))
		}
	}
	return nil
}

// outsideFences is text without its code fences, so a check box inside an example is no
// task.
func outsideFences(text string) string {
	var out []string
	fence := false
	for _, l := range strings.Split(text, "\n") {
		if fenceToggle.MatchString(l) {
			fence = !fence
			continue
		}
		if !fence {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}

// Spec writes a thread's spec, or replaces the text of the one it has.
func Spec(v *vault.Vault, in SpecIn, o Opts) (*Result, error) {
	w, err := begin(v, o)
	if err != nil {
		return nil, err
	}
	defer w.tx.Close()
	t, err := w.open(in.Thread)
	if err != nil {
		return nil, err
	}
	text := strings.TrimSpace(in.Text)
	if err := CheckSpec(text); err != nil {
		return nil, err
	}
	desc := strings.TrimSpace(in.Description)
	if desc == "" {
		goal, _ := doc.Section(text, "Goal")
		desc = firstSentence(goal)
	}
	if t.Spec != nil {
		front, _, _ := doc.Split(t.Spec.Content)
		content := doc.Join(front, "\n"+text+"\n")
		content = doc.SetFields(content, []doc.Field{{Key: "description", Value: oneLine(desc, 200)}, {Key: "updated", Value: vault.Stamp(w.now)}})
		if err := w.tx.Write(t.Spec.Path, []byte(content)); err != nil {
			return nil, err
		}
		w.wrote = append(w.wrote, t.Spec.ID())
		return w.finish("revise the spec of "+t.Stub.Title(), t.Stub.ID())
	}
	title, err := w.title(SpecTitle(t.Stub.Title()))
	if err != nil {
		return nil, err
	}
	extra := []doc.Field{{Key: "thread", Value: doc.Link(t.Stub.Title())}, {Key: "status", Value: NotImplemented}}
	if _, _, err := w.newDoc("spec", title, desc, t.Stub.List("tags"), extra, text+"\n"); err != nil {
		return nil, err
	}
	return w.finish("spec of "+t.Stub.Title(), t.Stub.ID())
}

// addTasks appends tasks to the thread's list for a repository, making the list when the
// thread has none for it. It returns the ids it gave.
func (w *writer) addTasks(t *Thread, repository string, tasks []TaskIn, trail string) ([]string, error) {
	if t.Spec == nil || len(Requirements(t.Spec.Body)) == 0 {
		return nil, fmt.Errorf("%s has no spec with requirements; write it first (thread spec)", t.Stub.Title())
	}
	if len(tasks) == 0 {
		return nil, errors.New("tasks needs at least one task")
	}
	known := map[string]bool{}
	for _, r := range Requirements(t.Spec.Body) {
		known[r.ID] = true
	}
	repo, err := w.repo(repository)
	if err != nil {
		return nil, err
	}
	repoTitle, repoLink := "", ""
	if repo != nil {
		repoTitle, repoLink = repo.Title(), doc.Link(repo.Title())
	}
	next := 1
	for _, task := range t.Tasks() {
		next = max(next, number(task.ID)+1)
	}
	var lines, details, ids []string
	for i, task := range tasks {
		if strings.TrimSpace(task.Text) == "" {
			return nil, fmt.Errorf("task %d has no text", i+1)
		}
		var reqs []string
		for _, r := range task.Requirements {
			r = strings.ToUpper(strings.TrimSpace(r))
			if !reqID.MatchString(r) || !known[r] {
				return nil, fmt.Errorf("task %d serves %q, which is no requirement of the spec", i+1, r)
			}
			if !slices.Contains(reqs, r) {
				reqs = append(reqs, r)
			}
		}
		if len(reqs) == 0 {
			return nil, fmt.Errorf("task %d names no requirement; each task serves at least one (requirements: [R1])", i+1)
		}
		id := fmt.Sprintf("T%d", next)
		next++
		ids = append(ids, id)
		lines = append(lines, TaskLine(Task{ID: id, State: TaskOpen, Text: TaskText(task.Text, reqs), Trail: trail}))
		if d := strings.TrimSpace(task.Details); d != "" {
			details = append(details, "### "+id+"\n\n"+d)
		}
	}
	var list *doc.Doc
	for _, l := range t.Lists {
		if strings.EqualFold(doc.LinkTarget(l.Str("repository")), repoTitle) {
			list = l
			break
		}
	}
	order := schema.Get("tasks").Sections
	if list == nil {
		title, err := w.title(TasksTitle(t.Stub.Title(), repoTitle))
		if err != nil {
			return nil, err
		}
		body := "## Tasks\n\n" + strings.Join(lines, "\n") + "\n"
		if len(details) > 0 {
			body += "\n## Details\n\n" + strings.Join(details, "\n\n") + "\n"
		}
		desc := "The tasks of " + t.Stub.Title()
		if repoTitle != "" {
			desc += " in " + repoTitle
		}
		extra := []doc.Field{{Key: "thread", Value: doc.Link(t.Stub.Title())}, {Key: "repository", Value: repoLink}, {Key: "done", Value: 0}, {Key: "total", Value: len(lines)}}
		_, _, err = w.newDoc("tasks", title, desc+".", t.Stub.List("tags"), extra, body)
		return ids, err
	}
	front, body, _ := doc.Split(list.Content)
	have, _ := doc.Section(body, "Tasks")
	body = doc.PutSection(body, "Tasks", strings.TrimSpace(have+"\n"+strings.Join(lines, "\n")), order)
	if len(details) > 0 {
		have, _ := doc.Section(body, "Details")
		body = doc.PutSection(body, "Details", strings.TrimSpace(have+"\n\n"+strings.Join(details, "\n\n")), order)
	}
	content := doc.SetField(doc.Join(front, body), "updated", vault.Stamp(w.now))
	if err := w.tx.Write(list.Path, []byte(content)); err != nil {
		return nil, err
	}
	w.wrote = append(w.wrote, list.ID())
	return ids, nil
}

// Tasks writes a thread's task list for one repository, or appends tasks to it.
func TasksWrite(v *vault.Vault, in TasksIn, o Opts) (*Result, error) {
	w, err := begin(v, o)
	if err != nil {
		return nil, err
	}
	defer w.tx.Close()
	t, err := w.open(in.Thread)
	if err != nil {
		return nil, err
	}
	ids, err := w.addTasks(t, in.Repository, in.Tasks, "")
	if err != nil {
		return nil, err
	}
	return w.finish(fmt.Sprintf("tasks %s of %s", strings.Join(ids, ", "), t.Stub.Title()), t.Stub.ID())
}

// Start starts work on a thread, or continues it, and binds the session to it.
func Start(v *vault.Vault, key string, take bool, o Opts) (*Result, error) {
	w, err := begin(v, o)
	if err != nil {
		return nil, err
	}
	defer w.tx.Close()
	t, err := w.open(key)
	if err != nil {
		return nil, err
	}
	d := t.Stub
	switch status := w.b.Status(d); status {
	case StatusStub:
		return nil, fmt.Errorf("%s has no spec with requirements; write it first (thread-spec), then its tasks", d.Title())
	case Specified:
		return nil, fmt.Errorf("%s has no task list; write the tasks first (thread-tasks)", d.Title())
	case Unverified, Verified, Closed:
		return nil, fmt.Errorf("%s is %s: every task is done. New work needs a new task first (thread tasks, or thread finding with outcome task)", d.Title(), status)
	}
	if waits := w.b.Waits(d); len(waits) > 0 {
		return nil, fmt.Errorf("%s comes after %s, which %s not verified yet; work on that first", d.Title(), titles(waits), plural(len(waits), "is", "are"))
	}
	if hs := w.b.Holders(d); len(hs) > 0 && !take {
		return nil, fmt.Errorf("%s is held by the live session %s. If you mean to take it over, call start again with take: true", d.Title(), hs[0].Title())
	}
	kind := "started"
	if w.b.started(t) {
		kind = "continued"
	}
	if err := w.on(d, kind, nil); err != nil {
		return nil, err
	}
	res, err := w.finish(kind+" "+d.Title(), d.ID())
	if err != nil {
		return nil, err
	}
	res.Started = d.Title()
	return res, nil
}

// Check checks one task with what did it, drops it with the reason, or opens it again.
func Check(v *vault.Vault, in CheckIn, o Opts) (*Result, error) {
	w, err := begin(v, o)
	if err != nil {
		return nil, err
	}
	defer w.tx.Close()
	t, err := w.open(in.Thread)
	if err != nil {
		return nil, err
	}
	id := strings.ToUpper(strings.TrimSpace(in.Task))
	var task *Task
	var open []string
	for _, x := range t.Tasks() {
		if x.ID != "" && x.ID == id {
			found := x
			task = &found
		}
		if x.State == TaskOpen && x.ID != "" {
			open = append(open, x.ID)
		}
	}
	if task == nil {
		return nil, fmt.Errorf("%s has no task %q; its open tasks are %s", t.Stub.Title(), in.Task, orDefault(strings.Join(open, ", "), "none"))
	}
	state := orDefault(in.State, TaskDone)
	switch state {
	case TaskDone:
		if w.by == ByAgent && !w.b.started(t) {
			return nil, fmt.Errorf("%s is not started; call thread start first, so the session is bound to it", t.Stub.Title())
		}
		var trail []string
		if len(in.Commits) > 0 {
			var short []string
			for _, c := range in.Commits {
				if c = strings.TrimSpace(c); c != "" {
					short = append(short, c[:min(len(c), 10)])
				}
			}
			if len(short) > 0 {
				trail = append(trail, strings.Join(short, ", "))
			}
		}
		if note := strings.ReplaceAll(oneLine(in.Note, 200), trailSep, ", "); note != "" {
			trail = append(trail, note)
		}
		if len(trail) == 0 {
			return nil, errors.New("a done task carries what did it: commits, or note (one line) when the work made no commit")
		}
		task.Trail = strings.Join(trail, trailSep)
	case TaskDropped:
		reason := strings.ReplaceAll(oneLine(in.Reason, 200), trailSep, ", ")
		if reason == "" {
			return nil, errors.New("a dropped task needs the reason")
		}
		task.Trail = "dropped: " + reason
	case TaskOpen:
		task.Trail = ""
	default:
		return nil, fmt.Errorf("state is %q; a task is done, dropped, or open", in.State)
	}
	task.State = state
	lines := strings.Split(task.List.Content, "\n")
	lines[task.Line] = TaskLine(*task)
	content := doc.SetField(strings.Join(lines, "\n"), "updated", vault.Stamp(w.now))
	if err := w.tx.Write(task.List.Path, []byte(content)); err != nil {
		return nil, err
	}
	w.wrote = append(w.wrote, task.List.ID())
	return w.finish(fmt.Sprintf("%s %s of %s", state, id, t.Stub.Title()), t.Stub.ID())
}

// Verify files one verification of a thread: the result for every requirement, with the
// evidence, and the findings.
func Verify(v *vault.Vault, in VerifyIn, o Opts) (*Result, error) {
	w, err := begin(v, o)
	if err != nil {
		return nil, err
	}
	defer w.tx.Close()
	t, err := w.open(in.Thread)
	if err != nil {
		return nil, err
	}
	if t.Spec == nil || len(Requirements(t.Spec.Body)) == 0 {
		return nil, fmt.Errorf("%s has no spec with requirements to verify against", t.Stub.Title())
	}
	var open []string
	for _, task := range t.Tasks() {
		if task.State == TaskOpen {
			open = append(open, orDefault(task.ID, task.Text))
		}
	}
	switch {
	case len(t.Tasks()) == 0:
		return nil, fmt.Errorf("%s has no task; a verification checks done work", t.Stub.Title())
	case len(open) > 0:
		return nil, fmt.Errorf("%s has %d open %s (%s); finish or drop them first (thread check)", t.Stub.Title(), len(open), plural(len(open), "task", "tasks"), strings.Join(open, ", "))
	case strings.TrimSpace(in.Scope) == "":
		return nil, errors.New("verify needs scope: what was checked, each repository with its commits")
	}
	results := map[string]ResultIn{}
	for _, r := range in.Results {
		id := strings.ToUpper(strings.TrimSpace(r.Requirement))
		r.Result = strings.ToLower(strings.TrimSpace(r.Result))
		switch {
		case r.Result != Pass && r.Result != Fail:
			return nil, fmt.Errorf("%s: result is %q; it is pass or fail", id, r.Result)
		case strings.TrimSpace(r.Evidence) == "":
			return nil, fmt.Errorf("%s: a result needs its evidence: the command and its output, the file, or the commit", id)
		}
		if _, dup := results[id]; dup {
			return nil, fmt.Errorf("%s has two results", id)
		}
		results[id] = r
	}
	rows := []string{"| Requirement | Result | Evidence |", "|---|---|---|"}
	failed := 0
	var missing []string
	for _, r := range Requirements(t.Spec.Body) {
		res, ok := results[r.ID]
		if !ok {
			missing = append(missing, r.ID)
			continue
		}
		delete(results, r.ID)
		if res.Result == Fail {
			failed++
		}
		rows = append(rows, fmt.Sprintf("| %s: %s | %s | %s |", r.ID, cell(r.Text), res.Result, cell(res.Evidence)))
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("the verification has no result for %s; it gives one for every requirement of the spec", strings.Join(missing, ", "))
	}
	for id := range results {
		return nil, fmt.Errorf("%s is no requirement of the spec", id)
	}
	var findings []string
	for _, f := range in.Findings {
		if f = strings.ReplaceAll(oneLine(f, 400), outcomeSep, " - "); f != "" {
			findings = append(findings, fmt.Sprintf("- [ ] F%d: %s", len(findings)+1, f))
		}
	}
	if failed > 0 && len(findings) == 0 {
		return nil, errors.New("a failed requirement needs a finding that says what is wrong, so the next step has something to act on")
	}
	round := 1
	if last := t.Last(); last != nil {
		round = last.Front.Int("round") + 1
	}
	title, err := w.title(VerificationTitle(t.Stub.Title(), round))
	if err != nil {
		return nil, err
	}
	body := "## Scope\n\n" + strings.TrimSpace(in.Scope) + "\n\n## Requirements\n\n" + strings.Join(rows, "\n") + "\n\n## Findings\n\n"
	if len(findings) == 0 {
		body += "None.\n"
	} else {
		body += strings.Join(findings, "\n") + "\n"
	}
	if notes := strings.TrimSpace(in.Notes); notes != "" {
		body += "\n## Notes\n\n" + notes + "\n"
	}
	verdict := Pass
	switch {
	case failed > 0:
		verdict = Fail
	case len(findings) > 0:
		verdict = OpenFindings
	}
	extra := []doc.Field{
		{Key: "thread", Value: doc.Link(t.Stub.Title())},
		{Key: "round", Value: round},
		{Key: "at", Value: vault.Stamp(w.now)},
		{Key: "by", Value: w.by},
		{Key: "session", Value: ""},
		{Key: "spec_hash", Value: SpecHash(t.Spec)},
		{Key: "tasks_hash", Value: TasksHash(t.Lists)},
		{Key: "verdict", Value: verdict},
	}
	desc := fmt.Sprintf("Round %d of the verification of %s: %d of %d requirements pass, %d %s.", round, t.Stub.Title(), len(rows)-2-failed, len(rows)-2, len(findings), plural(len(findings), "finding", "findings"))
	if _, _, err := w.newDoc("verification", title, desc, t.Stub.List("tags"), extra, body); err != nil {
		return nil, err
	}
	return w.finish(fmt.Sprintf("verification %d of %s", round, t.Stub.Title()), t.Stub.ID())
}

// Outcomes of a finding.
var Outcomes = []string{"task", "spec", "stub", "knowledge", "accepted"}

// FindingOutcome gives one open finding of a thread's last verification its outcome.
func FindingOutcome(v *vault.Vault, in FindingIn, o Opts) (*Result, error) {
	w, err := begin(v, o)
	if err != nil {
		return nil, err
	}
	defer w.tx.Close()
	t, err := w.open(in.Thread)
	if err != nil {
		return nil, err
	}
	last := t.Last()
	if last == nil {
		return nil, fmt.Errorf("%s has no verification", t.Stub.Title())
	}
	id := strings.ToUpper(strings.TrimSpace(in.Finding))
	var finding *Finding
	var open []string
	for _, f := range Findings(last) {
		if f.ID == id {
			found := f
			finding = &found
		}
		if f.Open {
			open = append(open, f.ID)
		}
	}
	switch {
	case finding == nil:
		return nil, fmt.Errorf("%s has no finding %q; its open findings are %s", last.Title(), in.Finding, orDefault(strings.Join(open, ", "), "none"))
	case !finding.Open:
		return nil, fmt.Errorf("%s of %s has its outcome already: %s", id, last.Title(), finding.Outcome)
	}
	outcome := ""
	switch in.Outcome {
	case "task":
		if in.Task == nil {
			return nil, errors.New("the outcome task needs task: the new task, with the requirements it serves")
		}
		ids, err := w.addTasks(t, in.Repository, []TaskIn{*in.Task}, "")
		if err != nil {
			return nil, err
		}
		outcome = "task " + ids[0]
	case "spec":
		if SpecHash(t.Spec) == last.Str("spec_hash") {
			return nil, errors.New("the spec's requirements and rules are as the verification read them; revise the spec first, after the user agreed (thread spec, or Edit), then give the outcome")
		}
		outcome = "spec"
	case "stub":
		var title string
		switch {
		case strings.TrimSpace(in.Link) != "":
			s, err := w.idx.ResolveType(in.Link, "stub")
			if err != nil {
				return nil, fmt.Errorf("link: %w", err)
			}
			title = s.Title()
		case strings.TrimSpace(in.Text) != "":
			if title, err = w.named(TitleFromText(in.Text)); err != nil {
				return nil, err
			}
			text := strings.TrimSpace(in.Text) + "\n\nFrom " + id + " of " + doc.Link(last.Title()) + "."
			if _, _, err := w.newDoc("stub", title, firstSentence(in.Text), t.Stub.List("tags"), stubFields("", "", nil), "## Idea\n\n"+text+"\n"); err != nil {
				return nil, err
			}
		default:
			return nil, errors.New("the outcome stub needs link (a stub that exists) or text (the words of a new one)")
		}
		outcome = doc.Link(title)
	case "knowledge":
		d, err := w.idx.Resolve(in.Link)
		if err != nil {
			return nil, fmt.Errorf("the outcome knowledge needs link, the change or the topic that holds it: %w", err)
		}
		if !slices.Contains([]string{"change", "topic", "source", "repository"}, d.Type()) {
			return nil, fmt.Errorf("link: %s is a %s; knowledge lives in a change, a topic, a source, or a repository", vault.Title(d), d.Type())
		}
		outcome = "knowledge " + doc.Link(vault.Title(d))
	case "accepted":
		reason := oneLine(in.Reason, 200)
		if reason == "" {
			return nil, errors.New("the outcome accepted needs reason: why the user accepts the finding as it is")
		}
		outcome = "accepted: " + reason
	default:
		return nil, fmt.Errorf("outcome is %q; a finding becomes a task, a spec change, a stub, knowledge, or is accepted", in.Outcome)
	}
	lines := strings.Split(last.Content, "\n")
	lines[finding.Line] = "- [x] " + id + ": " + finding.Text + outcomeSep + outcome
	content := doc.SetField(strings.Join(lines, "\n"), "updated", vault.Stamp(w.now))
	if err := w.tx.Write(last.Path, []byte(content)); err != nil {
		return nil, err
	}
	w.wrote = append(w.wrote, last.ID())
	return w.finish(fmt.Sprintf("finding %s of %s: %s", id, t.Stub.Title(), in.Outcome), t.Stub.ID())
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

// Resolve closes a stub with no spec as resolved, with the documents it became.
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
	if s := w.b.Status(d); s != StatusStub {
		return nil, fmt.Errorf("%s is %s; only a stub with no spec resolves into other documents", d.Title(), s)
	}
	var list []string
	for _, b := range became {
		bd, err := w.idx.Resolve(b)
		if err != nil {
			return nil, fmt.Errorf("became: %w", err)
		}
		if bd.ID() == d.ID() {
			return nil, errors.New("became: a stub does not become itself")
		}
		list = append(list, vault.Title(bd))
	}
	if err := w.resolve(d, list); err != nil {
		return nil, err
	}
	return w.finish("resolve "+d.Title(), d.ID())
}

// ResolveInTx closes a stub with no spec as resolved inside another write's transaction:
// a capture that the stub asked for. It returns the id of the event.
func ResolveInTx(tx *vault.Tx, idx *vault.Index, stub *doc.Doc, became []string, o Opts) (string, error) {
	w := &writer{v: tx.V, tx: tx, idx: idx, b: Load(idx), now: o.Now.Truncate(time.Second), by: orDefault(o.By, ByAgent), titles: NewTitles(idx), newTag: map[string]bool{}}
	for _, b := range became {
		w.titles.Take(b)
	}
	if s := w.b.Status(stub); s != StatusStub {
		return "", fmt.Errorf("%s is %s; only a stub with no spec resolves", stub.Title(), s)
	}
	if err := w.resolve(stub, became); err != nil {
		return "", err
	}
	return w.events[0], nil
}

// Drop drops a thread or a chord with the reason. A dropped chord drops each of its
// threads that is not ended.
func Drop(v *vault.Vault, key, reason string, o Opts) (*Result, error) {
	if strings.TrimSpace(reason) == "" {
		return nil, errors.New("drop needs the reason")
	}
	w, err := begin(v, o)
	if err != nil {
		return nil, err
	}
	defer w.tx.Close()
	d, err := w.subject(key)
	if err != nil {
		return nil, err
	}
	if s := w.b.Status(d); Ended(s) || s == ChordDropped {
		return nil, fmt.Errorf("%s is %s already", d.Title(), s)
	}
	if err := w.on(d, "dropped", map[string]string{"Why": reason}); err != nil {
		return nil, err
	}
	if d.Type() == "chord" {
		for _, s := range w.b.Members(d) {
			if !Ended(w.b.Status(s)) {
				if err := w.on(s, "dropped", map[string]string{"Why": "Dropped with " + doc.Link(d.Title()) + "."}); err != nil {
					return nil, err
				}
			}
		}
	}
	return w.finish("drop "+d.Title(), d.ID())
}

// subject resolves a thread's stub or a chord for an action on either.
func (w *writer) subject(key string) (*doc.Doc, error) {
	d, err := w.idx.ResolveType(key, "stub", "chord", "spec", "tasks", "verification")
	if err != nil {
		return nil, err
	}
	if d.Type() == "chord" {
		return d, nil
	}
	t := w.b.Thread(d)
	if t == nil {
		return nil, fmt.Errorf("%s names no thread", d.Title())
	}
	return t.Stub, nil
}

// Reopen takes up a dropped or resolved thread, or a dropped chord, again. A chord's
// threads that were dropped with it come back with it.
func Reopen(v *vault.Vault, key, reason string, o Opts) (*Result, error) {
	w, err := begin(v, o)
	if err != nil {
		return nil, err
	}
	defer w.tx.Close()
	d, err := w.subject(key)
	if err != nil {
		return nil, err
	}
	switch s := w.b.Status(d); {
	case s == Closed || s == ChordClosed:
		return nil, fmt.Errorf("%s is closed; new work on it starts with a new task (thread tasks) or a revised spec, and its status follows", d.Title())
	case s != Dropped && s != Resolved:
		return nil, fmt.Errorf("%s is %s; reopen takes up a dropped or resolved thread, or a dropped chord", d.Title(), s)
	}
	var prose map[string]string
	if strings.TrimSpace(reason) != "" {
		prose = map[string]string{"Why": reason}
	}
	if err := w.on(d, "reopened", prose); err != nil {
		return nil, err
	}
	if d.Type() == "chord" {
		// The threads that were dropped with the chord come back with it.
		with := "Dropped with " + doc.Link(d.Title()) + "."
		for _, s := range w.b.Members(d) {
			e := w.b.Result(s)
			if w.b.Status(s) != Dropped || e == nil {
				continue
			}
			if why, _ := doc.Section(e.Body, "Why"); strings.TrimSpace(why) == with {
				if err := w.on(s, "reopened", map[string]string{"Why": "Reopened with " + doc.Link(d.Title()) + "."}); err != nil {
					return nil, err
				}
			}
		}
	}
	return w.finish("reopen "+d.Title(), d.ID())
}

// Block marks a thread blocked, with the one line it waits on.
func Block(v *vault.Vault, key, reason string, o Opts) (*Result, error) {
	reason = oneLine(reason, 180)
	if reason == "" {
		return nil, errors.New("block needs the reason: what the thread waits on, in one line")
	}
	w, err := begin(v, o)
	if err != nil {
		return nil, err
	}
	defer w.tx.Close()
	t, err := w.thread(key)
	if err != nil {
		return nil, err
	}
	d := t.Stub
	if s := w.b.Status(d); Ended(s) {
		return nil, fmt.Errorf("%s is %s", d.Title(), s)
	}
	if err := w.event(EventIn{Kind: "blocked", SubjectTitle: d.Title(), SubjectID: d.ID(), SubjectTags: d.List("tags"), Line: reason}); err != nil {
		return nil, err
	}
	return w.finish("block "+d.Title(), d.ID())
}

// Unblock clears a thread's block.
func Unblock(v *vault.Vault, key string, o Opts) (*Result, error) {
	w, err := begin(v, o)
	if err != nil {
		return nil, err
	}
	defer w.tx.Close()
	t, err := w.thread(key)
	if err != nil {
		return nil, err
	}
	d := t.Stub
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
	// A note on a part of a thread is a note on the thread.
	if t := w.b.Thread(d); t != nil {
		d = t.Stub
	}
	if err := w.on(d, "note", map[string]string{"Note": text}); err != nil {
		return nil, err
	}
	return w.finish("note on "+d.Title(), d.ID())
}

// retitle moves documents to new titles and rewrites every link to the old ones.
// contents holds, by its current path, the new text of each document the write also
// changed.
func (w *writer) retitle(moves []vault.Retitle, contents map[string]string) error {
	rewrites, _ := vault.Rewrites(w.idx, moves, contents, nil)
	final := map[string]string{}
	for p, c := range contents {
		final[p] = c
	}
	for _, rw := range rewrites {
		final[rw.Path] = rw.Content
	}
	for _, m := range moves {
		from, to := vault.DocPath(m.Old), vault.DocPath(m.New)
		content, ok := final[from]
		if !ok {
			data, err := w.v.Read(from)
			if err != nil {
				return err
			}
			content = string(data)
		}
		delete(final, from)
		if err := w.tx.Remove(from); err != nil {
			return err
		}
		if err := w.tx.Write(to, []byte(content)); err != nil {
			return err
		}
	}
	for p, c := range final {
		if err := w.tx.Write(p, []byte(c)); err != nil {
			return err
		}
	}
	return nil
}

// Set changes the fields of a thread's stub or of a chord. A new title renames the
// document, the thread's other documents, and a chord's canvas, and rewrites every link.
func Set(v *vault.Vault, in SetIn, o Opts) (*Result, error) {
	w, err := begin(v, o)
	if err != nil {
		return nil, err
	}
	defer w.tx.Close()
	d, err := w.subject(in.Doc)
	if err != nil {
		return nil, err
	}
	stub := d.Type() == "stub"
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
		if err := checkPriority(*in.Priority); err != nil {
			return nil, err
		}
		set("priority", orDefault(*in.Priority, "normal"))
	}
	if (in.Chord != nil || in.After != nil) && !stub {
		return nil, errors.New("only a thread has a chord and an after; a chord's order lives on its threads (chord order)")
	}
	if in.Chord != nil {
		link := ""
		if strings.TrimSpace(*in.Chord) != "" {
			c, err := w.chord(*in.Chord)
			if err != nil {
				return nil, err
			}
			link = doc.Link(c.Title())
		}
		set("chord", link)
	}
	if in.After != nil {
		links, after, err := w.afterLinks(*in.After, d.ID())
		if err != nil {
			return nil, err
		}
		if loop := w.b.loop(map[string][]*doc.Doc{d.ID(): after}); loop != "" {
			return nil, fmt.Errorf("after: the threads would wait on each other: %s", loop)
		}
		set("after", links)
	}
	set("updated", vault.Stamp(w.now))
	title := d.Title()
	if in.Title != nil && doc.CleanTitle(*in.Title) != d.Title() {
		w.titles.Release(d.Title())
		if title, err = w.named(*in.Title); err != nil {
			return nil, err
		}
		moves := []vault.Retitle{{Old: d.Title(), New: title}}
		if t := w.b.Thread(d); stub && t != nil {
			for _, part := range t.Docs()[1:] {
				rest, ok := strings.CutPrefix(part.Title(), d.Title()+" · ")
				if !ok {
					continue
				}
				to, err := w.title(title + " · " + rest)
				if err != nil {
					return nil, err
				}
				moves = append(moves, vault.Retitle{Old: part.Title(), New: to})
			}
		}
		if err := w.retitle(moves, map[string]string{d.Path: content}); err != nil {
			return nil, err
		}
		if d.Type() == "chord" && w.v.Exists(CanvasPath(d)) {
			if err := w.tx.Move(CanvasPath(d), vault.Chords+"/"+title+".canvas"); err != nil {
				return nil, err
			}
		}
	} else if err := w.tx.Write(d.Path, []byte(content)); err != nil {
		return nil, err
	}
	w.wrote = append(w.wrote, d.ID())
	return w.finish("set "+title, d.ID())
}

// Loop names a cycle in the order of the threads, or "" when there is none.
func (b *Board) Loop() string { return b.loop(nil) }

// loop names a cycle in the order of the threads, with after replaced for the stubs in
// override, or "" when there is none.
func (b *Board) loop(override map[string][]*doc.Doc) string {
	after := func(d *doc.Doc) []*doc.Doc {
		if list, ok := override[d.ID()]; ok {
			return list
		}
		return b.After(d)
	}
	const (
		visiting = 1
		done     = 2
	)
	state := map[string]int{}
	var walk func(d *doc.Doc, trail []string) string
	walk = func(d *doc.Doc, trail []string) string {
		switch state[d.ID()] {
		case visiting:
			return strings.Join(append(trail, d.Title()), " waits on ")
		case done:
			return ""
		}
		state[d.ID()] = visiting
		for _, a := range after(d) {
			if l := walk(a, append(trail, d.Title())); l != "" {
				return l
			}
		}
		state[d.ID()] = done
		return ""
	}
	for _, s := range b.Stubs {
		if l := walk(s, nil); l != "" {
			return l
		}
	}
	return ""
}
