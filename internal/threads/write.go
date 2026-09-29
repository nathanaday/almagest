package threads

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/change"
	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/links"
	"github.com/nathanaday/atlas-obsidian/internal/scope"
	"github.com/nathanaday/atlas-obsidian/internal/sessions"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

// OpenIn opens a thread.
type OpenIn struct {
	Text     string   `json:"text" jsonschema:"the user's words, as given; they become the stub"`
	Title    string   `json:"title,omitempty" jsonschema:"a short name for the work; the first line of text when empty"`
	Scope    []string `json:"scope,omitempty" jsonschema:"the repositories and areas the work touches, by id or title"`
	Priority string   `json:"priority,omitempty" jsonschema:"high, normal, low, or someday"`
	Inbox    string   `json:"inbox,omitempty" jsonschema:"a note in inbox/ that the stub replaces; it leaves the inbox in the same commit"`
}

// TaskIn is one new task.
type TaskIn struct {
	Title      string   `json:"title" jsonschema:"the task's own title"`
	Text       string   `json:"text" jsonschema:"the sections What, Where, Conventions, and Verify"`
	Repository string   `json:"repository,omitempty" jsonschema:"the one repository the task works in, by id or title"`
	Depends    []string `json:"depends,omitempty" jsonschema:"tasks of this thread that must be done first: T1, a title, or an id"`
	Order      int      `json:"order,omitempty" jsonschema:"the task's number; the next free one when 0"`
}

// TaskDo is one action on a task.
type TaskDo struct {
	Do         string    `json:"do" jsonschema:"start, done, drop, reopen, or set"`
	Result     string    `json:"result,omitempty" jsonschema:"done: what changed, the commits, how it was verified"`
	Take       bool      `json:"take,omitempty" jsonschema:"start: take a task another live session holds"`
	Title      *string   `json:"title,omitempty"`
	Repository *string   `json:"repository,omitempty"`
	Depends    *[]string `json:"depends,omitempty"`
	Order      *int      `json:"order,omitempty"`
	Blocked    *string   `json:"blocked,omitempty"`
}

// SetIn changes a thread's own fields.
type SetIn struct {
	Title    *string   `json:"title,omitempty"`
	Priority *string   `json:"priority,omitempty"`
	Blocked  *string   `json:"blocked,omitempty"`
	Scope    *[]string `json:"scope,omitempty"`
}

// Result is what a thread write returns: the view of the thread, and the commit.
type Result struct {
	View   *View
	Commit string
}

// write is one thread write: the transaction, and the board as it was when it began.
type write struct {
	v   *vault.Vault
	tx  *vault.Tx
	idx *vault.Index
	b   *Board
	now time.Time
}

func begin(v *vault.Vault, now time.Time) (*write, error) {
	tx, err := change.Begin(v)
	if err != nil {
		return nil, err
	}
	idx, err := vault.Load(v)
	if err != nil {
		tx.Close()
		return nil, err
	}
	return &write{v: v, tx: tx, idx: idx, b: Load(idx), now: now}, nil
}

// finish syncs the derived fields, commits, and returns the view of the thread.
func (w *write) finish(subject, threadID string) (*Result, error) {
	idx, err := vault.Load(w.v)
	if err != nil {
		return nil, err
	}
	b := Load(idx)
	if _, err := b.Sync(w.tx.WriteIfChanged); err != nil {
		return nil, err
	}
	if idx, err = vault.Load(w.v); err != nil {
		return nil, err
	}
	if _, err := scope.Heal(idx, w.tx.WriteIfChanged); err != nil {
		return nil, err
	}
	sha, err := w.tx.Commit("thread: "+subject, Trailer+": "+threadID)
	if err != nil {
		return nil, err
	}
	idx, err = vault.Load(w.v)
	if err != nil {
		return nil, err
	}
	b = Load(idx)
	t := b.byID[threadID]
	if t == nil {
		return &Result{Commit: sha}, nil
	}
	return &Result{View: b.View(t), Commit: sha}, nil
}

// title checks that a new document's title is free in the vault.
func (w *write) free(title string) error {
	if holders := w.idx.TitleHolders(title); len(holders) > 0 {
		return fmt.Errorf("the title %q is held by %s; titles are unique in the vault, so choose another", title, strings.Join(holders, ", "))
	}
	return nil
}

func (w *write) scopes(values []string) ([]string, error) {
	out := []string{}
	for _, s := range values {
		if strings.TrimSpace(s) == "" {
			continue
		}
		d, err := w.idx.ResolveType(s, "area", "repository")
		if err != nil {
			return nil, fmt.Errorf("scope: %w", err)
		}
		out = append(out, doc.Link(vault.Title(d)))
	}
	return out, nil
}

// ErrClosed refuses a new document on a closed thread.
var ErrClosed = errors.New("the thread is closed; thread reopen takes it up again")

// homeFolder is the folder under threads/ that a thread's folder goes in: the mirror of
// its first scope's folder in the wiki, or threads/ itself when the thread names no scope
// yet, or one whose page lies outside a folder of its own.
func (w *write) homeFolder(scope []string) string {
	if len(scope) == 0 {
		return vault.Threads
	}
	d := w.idx.Linked(scope[0])
	if d == nil {
		return vault.Threads
	}
	if f := w.idx.Folder(d); f != "" {
		return vault.Mirror(f)
	}
	return vault.Threads
}

// moveFolder moves every file of a thread's folder, the user's files too, to a new folder.
// A merge moves the files left in from into a folder that exists.
func (w *write) moveFolder(from, to string, merge bool) error {
	if from == to {
		return nil
	}
	if !merge && w.v.Exists(to) && !strings.EqualFold(from, to) {
		return fmt.Errorf("the folder %s exists", to)
	}
	var files []string
	err := filepath.WalkDir(w.v.Abs(from), func(abs string, e fs.DirEntry, err error) error {
		if err == nil && !e.IsDir() {
			files = append(files, w.v.Rel(abs))
		}
		return err
	})
	if err != nil {
		return err
	}
	for _, rel := range files {
		if err := w.tx.Move(rel, to+strings.TrimPrefix(rel, from)); err != nil {
			return err
		}
	}
	return nil
}

// Open opens a thread: its folder and its stub, in the user's words.
func Open(v *vault.Vault, in OpenIn, now time.Time) (*Result, error) {
	text := strings.TrimSpace(in.Text)
	if text == "" {
		return nil, errors.New("open needs text: the user's words")
	}
	title := doc.CleanTitle(in.Title)
	if title == "" {
		title = TitleFromText(text)
	}
	if title == "" {
		return nil, errors.New("open needs a title")
	}
	priority := in.Priority
	if priority == "" {
		priority = "normal"
	}
	if !slices.Contains(Priorities, priority) {
		return nil, fmt.Errorf("priority %q; it is one of %s", priority, strings.Join(Priorities, ", "))
	}
	w, err := begin(v, now)
	if err != nil {
		return nil, err
	}
	defer w.tx.Close()
	if err := w.free(title); err != nil {
		return nil, err
	}
	scope, err := w.scopes(in.Scope)
	if err != nil {
		return nil, err
	}
	folder := path.Join(w.homeFolder(scope), title)
	if v.Exists(folder) {
		return nil, fmt.Errorf("the folder %s exists; choose another title", folder)
	}
	var inbox string
	if in.Inbox != "" {
		inbox = path.Join(vault.Inbox, path.Base(in.Inbox))
		if !v.Exists(inbox) {
			return nil, fmt.Errorf("inbox: %s is not in inbox/", in.Inbox)
		}
	}
	id := doc.NewID("thr", func(s string) bool { return w.idx.ByID(s) != nil })
	content := doc.Render([]doc.Field{
		{Key: "id", Value: id},
		{Key: "type", Value: "stub"},
		{Key: "created", Value: today(now)},
		{Key: "updated", Value: today(now)},
		{Key: "scope", Value: scope},
		{Key: "priority", Value: priority},
		{Key: "blocked", Value: ""},
		{Key: "stage", Value: StageStub},
		{Key: "outcome", Value: ""},
		{Key: "active", Value: false},
		{Key: "tasks", Value: "0/0"},
	}, "## Stub\n\n"+text+"\n\n## Notes\n")
	if err := w.tx.Write(path.Join(folder, title+".md"), []byte(content)); err != nil {
		return nil, err
	}
	if inbox != "" {
		if err := w.tx.Remove(inbox); err != nil {
			return nil, err
		}
	}
	return w.finish("open "+title, id)
}

// Attach checks the thread a session will work on. It writes nothing; the hook binds the
// session to the thread.
func Attach(idx *vault.Index, key string) (*View, error) {
	b := Load(idx)
	t, err := b.Find(key)
	if err != nil {
		return nil, err
	}
	if t.Closed() {
		return nil, fmt.Errorf("%s is closed; thread reopen takes it up again, or open a new thread", t.Title())
	}
	return b.View(t), nil
}

// Show is a thread's view.
func Show(idx *vault.Index, key string) (*View, error) {
	b := Load(idx)
	t, err := b.Find(key)
	if err != nil {
		return nil, err
	}
	return b.View(t), nil
}

// List is the board.
func List(idx *vault.Index) *BoardView { return Load(idx).BoardView() }

// File files a thread's spec or its receipt from the model's text.
func File(v *vault.Vault, key, part, text, outcome string, now time.Time) (*Result, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("file needs the %s's text", part)
	}
	w, err := begin(v, now)
	if err != nil {
		return nil, err
	}
	defer w.tx.Close()
	t, err := w.b.Find(key)
	if err != nil {
		return nil, err
	}
	if t.Closed() {
		return nil, ErrClosed
	}
	base := []doc.Field{
		{Key: "thread", Value: doc.Link(t.Title())},
		{Key: "thread_id", Value: t.ID()},
		{Key: "created", Value: today(now)},
		{Key: "updated", Value: today(now)},
	}
	var title, prefix, subject string
	var fields []doc.Field
	switch part {
	case "spec":
		if t.Spec != nil {
			return nil, fmt.Errorf("%s has a spec: %s. A thread has one; revise it with Edit", t.Title(), t.Spec.Title())
		}
		title, prefix, subject = t.Title()+" — Spec", "spc", "spec for "+t.Title()
	case "receipt":
		switch outcome {
		case "completed":
			if open := t.OpenTasks(); len(open) > 0 {
				var names []string
				for _, o := range open {
					names = append(names, Label(o)+" "+TaskTitle(o))
				}
				return nil, fmt.Errorf("a completed receipt waits for every task: %s still open. Finish each (thread task done) or drop it (thread task drop)", strings.Join(names, ", "))
			}
		case "killed":
			for _, o := range t.OpenTasks() {
				content := doc.SetFields(o.Content, []doc.Field{{Key: "status", Value: TaskDropped}, {Key: "updated", Value: today(now)}})
				if err := w.tx.Write(o.Path, []byte(content)); err != nil {
					return nil, err
				}
			}
		default:
			return nil, fmt.Errorf("a receipt needs outcome completed or killed, not %q", outcome)
		}
		title, prefix, subject = t.Title()+" — Receipt", "rcp", fmt.Sprintf("receipt for %s (%s)", t.Title(), outcome)
		fields = append(fields, doc.Field{Key: "outcome", Value: outcome}, doc.Field{Key: "superseded", Value: false})
	default:
		return nil, fmt.Errorf("part %q; file takes spec or receipt. Tasks come from thread tasks", part)
	}
	if err := w.free(title); err != nil {
		return nil, err
	}
	id := doc.NewID(prefix, func(s string) bool { return w.idx.ByID(s) != nil })
	list := append([]doc.Field{{Key: "id", Value: id}, {Key: "type", Value: part}}, base...)
	list = append(list, fields...)
	content := doc.Render(list, text+"\n")
	if err := w.tx.Write(path.Join(t.Folder(), title+".md"), []byte(content)); err != nil {
		return nil, err
	}
	if err := w.touchStub(t); err != nil {
		return nil, err
	}
	return w.finish(subject, t.ID())
}

// touchStub sets the stub's updated to today, since the thread moved.
func (w *write) touchStub(t *Thread) error {
	content := doc.SetField(t.Stub.Content, "updated", today(w.now))
	_, err := w.tx.WriteIfChanged(t.Stub.Path, []byte(content))
	return err
}

// Tasks adds tasks to a thread, one document each.
func Tasks(v *vault.Vault, key string, in []TaskIn, now time.Time) (*Result, error) {
	if len(in) == 0 {
		return nil, errors.New("tasks needs at least one task")
	}
	w, err := begin(v, now)
	if err != nil {
		return nil, err
	}
	defer w.tx.Close()
	t, err := w.b.Find(key)
	if err != nil {
		return nil, err
	}
	if t.Closed() {
		return nil, ErrClosed
	}
	next := 0
	for _, task := range t.Tasks {
		next = max(next, Order(task))
	}
	type planned struct {
		in    TaskIn
		order int
		title string
		id    string
		repo  string
	}
	var list []*planned
	taken := map[int]bool{}
	for _, task := range t.Tasks {
		taken[Order(task)] = true
	}
	ids := map[string]bool{}
	for _, ti := range in {
		name := doc.CleanTitle(ti.Title)
		if name == "" {
			return nil, errors.New("each task needs a title")
		}
		order := ti.Order
		if order <= 0 {
			next++
			for taken[next] {
				next++
			}
			order = next
		}
		if taken[order] {
			return nil, fmt.Errorf("T%d is taken in %s; give another order or leave it out", order, t.Title())
		}
		taken[order] = true
		next = max(next, order)
		p := &planned{in: ti, order: order, title: fmt.Sprintf("%s — T%d %s", t.Title(), order, name)}
		if err := w.free(p.title); err != nil {
			return nil, err
		}
		for _, other := range list {
			if strings.EqualFold(other.title, p.title) {
				return nil, fmt.Errorf("two tasks are titled %q", p.title)
			}
		}
		if ti.Repository != "" {
			r, err := w.idx.ResolveType(ti.Repository, "repository")
			if err != nil {
				return nil, fmt.Errorf("task %s: repository: %w", name, err)
			}
			p.repo = doc.Link(vault.Title(r))
		}
		p.id = doc.NewID("tsk", func(s string) bool { return w.idx.ByID(s) != nil || ids[s] })
		ids[p.id] = true
		list = append(list, p)
	}
	// Dependencies name tasks of this thread: ones that exist, or ones of this call.
	resolve := func(dep string) (string, string, error) {
		dep = strings.TrimSpace(doc.LinkTarget(dep))
		for _, p := range list {
			name := doc.CleanTitle(p.in.Title)
			if strings.EqualFold(dep, name) || strings.EqualFold(dep, p.title) || strings.EqualFold(dep, "T"+strconv.Itoa(p.order)) || dep == strconv.Itoa(p.order) || dep == p.id {
				return p.title, p.id, nil
			}
		}
		if task := t.Task(dep); task != nil {
			return task.Title(), task.ID(), nil
		}
		return "", "", fmt.Errorf("depends: %q is no task of %s", dep, t.Title())
	}
	edges := map[string][]string{}
	for _, task := range t.Tasks {
		for _, dep := range task.List("depends") {
			if d := w.idx.Linked(dep); d != nil {
				edges[task.ID()] = append(edges[task.ID()], d.ID())
			}
		}
	}
	deps := map[*planned][]string{}
	for _, p := range list {
		for _, dep := range p.in.Depends {
			title, id, err := resolve(dep)
			if err != nil {
				return nil, err
			}
			if id == p.id {
				return nil, fmt.Errorf("task %s cannot depend on itself", p.title)
			}
			deps[p] = append(deps[p], doc.Link(title))
			edges[p.id] = append(edges[p.id], id)
		}
	}
	if loop := findLoop(edges); loop != "" {
		return nil, fmt.Errorf("depends makes a loop through %s", loop)
	}
	for _, p := range list {
		depends := deps[p]
		if depends == nil {
			depends = []string{}
		}
		content := doc.Render([]doc.Field{
			{Key: "id", Value: p.id},
			{Key: "type", Value: "task"},
			{Key: "thread", Value: doc.Link(t.Title())},
			{Key: "thread_id", Value: t.ID()},
			{Key: "created", Value: today(now)},
			{Key: "updated", Value: today(now)},
			{Key: "order", Value: p.order},
			{Key: "repository", Value: p.repo},
			{Key: "depends", Value: depends},
			{Key: "status", Value: TaskOpen},
			{Key: "blocked", Value: ""},
			{Key: "active", Value: false},
		}, taskBody(p.in.Text))
		if err := w.tx.Write(path.Join(t.Folder(), p.title+".md"), []byte(content)); err != nil {
			return nil, err
		}
	}
	if err := w.touchStub(t); err != nil {
		return nil, err
	}
	noun := "tasks"
	if len(list) == 1 {
		noun = "task"
	}
	return w.finish(fmt.Sprintf("%d %s for %s", len(list), noun, t.Title()), t.ID())
}

// taskBody is the model's text, with Progress and Result sections when it has none.
func taskBody(text string) string {
	body := strings.TrimSpace(text) + "\n"
	for _, s := range []string{"Progress", "Result"} {
		if _, ok := doc.Section(body, s); !ok {
			body = doc.SetSection(body, s, "")
		}
	}
	return body
}

// findLoop names a task on a loop of dependencies, or "".
func findLoop(edges map[string][]string) string {
	state := map[string]int{}
	var visit func(string) bool
	visit = func(n string) bool {
		switch state[n] {
		case 1:
			return true
		case 2:
			return false
		}
		state[n] = 1
		for _, m := range edges[n] {
			if visit(m) {
				return true
			}
		}
		state[n] = 2
		return false
	}
	for n := range edges {
		if visit(n) {
			return n
		}
	}
	return ""
}

// Task acts on one task: start, done, drop, reopen, or set.
func Task(v *vault.Vault, key, thread string, in TaskDo, now time.Time) (*Result, error) {
	if in.Do == "start" {
		idx, err := vault.Load(v)
		if err != nil {
			return nil, err
		}
		return start(idx, key, thread, in.Take)
	}
	w, err := begin(v, now)
	if err != nil {
		return nil, err
	}
	defer w.tx.Close()
	t, task, err := w.b.FindTask(key, thread)
	if err != nil {
		return nil, err
	}
	label := Label(task)
	status := task.Str("status")
	content := task.Content
	set := []doc.Field{{Key: "updated", Value: today(now)}}
	var subject string
	switch in.Do {
	case "done":
		if status != TaskOpen {
			return nil, fmt.Errorf("%s is %s; only an open task can be done", label, status)
		}
		if strings.TrimSpace(in.Result) == "" {
			return nil, errors.New("done needs the result: what changed, the commits, and how it was verified")
		}
		front, body, _ := doc.Split(content)
		content = doc.Join(front, doc.SetSection(body, "Result", in.Result))
		set = append(set, doc.Field{Key: "status", Value: TaskDone}, doc.Field{Key: "blocked", Value: ""})
		subject = fmt.Sprintf("%s done in %s", label, t.Title())
	case "drop":
		if status != TaskOpen {
			return nil, fmt.Errorf("%s is %s; only an open task can be dropped", label, status)
		}
		set = append(set, doc.Field{Key: "status", Value: TaskDropped})
		subject = fmt.Sprintf("%s dropped in %s", label, t.Title())
	case "reopen":
		if t.Closed() {
			return nil, ErrClosed
		}
		if status == TaskOpen {
			return nil, fmt.Errorf("%s is open already", label)
		}
		set = append(set, doc.Field{Key: "status", Value: TaskOpen})
		subject = fmt.Sprintf("%s reopened in %s", label, t.Title())
	case "set":
		return w.setTask(t, task, in)
	default:
		return nil, fmt.Errorf("do %q; a task takes start, done, drop, reopen, or set", in.Do)
	}
	content = doc.SetFields(content, set)
	if err := w.tx.Write(task.Path, []byte(content)); err != nil {
		return nil, err
	}
	if err := w.touchStub(t); err != nil {
		return nil, err
	}
	return w.finish(subject, t.ID())
}

// start checks a task can start. It writes nothing: the hook adds the task to the
// session, and sync marks it active.
func start(idx *vault.Index, key, thread string, take bool) (*Result, error) {
	b := Load(idx)
	t, task, err := b.FindTask(key, thread)
	if err != nil {
		return nil, err
	}
	if t.Closed() {
		return nil, ErrClosed
	}
	if s := task.Str("status"); s != TaskOpen {
		return nil, fmt.Errorf("%s is %s; thread task %s reopen opens it again", Label(task), s, Label(task))
	}
	for _, dep := range task.List("depends") {
		if d := idx.Linked(dep); d != nil && d.Str("status") == TaskOpen {
			return nil, fmt.Errorf("%s depends on %s, which is open; call thread task %s done first", Label(task), Label(d), Label(d))
		}
	}
	if holders := b.Holders(task.Title()); len(holders) > 0 && !take {
		return nil, fmt.Errorf("%s is held by the live session %s. If that session is this one, or you mean to take the task over, call start again with take: true", Label(task), holders[0].Title())
	}
	return &Result{View: b.View(t)}, nil
}

// setTask changes a task's title, repository, dependencies, order, or blocked line. A
// new title or order renames the file and rewrites every link to it.
func (w *write) setTask(t *Thread, task *doc.Doc, in TaskDo) (*Result, error) {
	content := task.Content
	set := []doc.Field{{Key: "updated", Value: today(w.now)}}
	if in.Repository != nil {
		repo := ""
		if strings.TrimSpace(*in.Repository) != "" {
			r, err := w.idx.ResolveType(*in.Repository, "repository")
			if err != nil {
				return nil, fmt.Errorf("repository: %w", err)
			}
			repo = doc.Link(vault.Title(r))
		}
		set = append(set, doc.Field{Key: "repository", Value: repo})
	}
	if in.Blocked != nil {
		set = append(set, doc.Field{Key: "blocked", Value: oneLine(*in.Blocked)})
	}
	if in.Depends != nil {
		edges := map[string][]string{}
		for _, other := range t.Tasks {
			if other.ID() == task.ID() {
				continue
			}
			for _, dep := range other.List("depends") {
				if d := w.idx.Linked(dep); d != nil {
					edges[other.ID()] = append(edges[other.ID()], d.ID())
				}
			}
		}
		deps := []string{}
		for _, key := range *in.Depends {
			d := t.Task(key)
			if d == nil {
				return nil, fmt.Errorf("depends: %q is no task of %s", key, t.Title())
			}
			if d.ID() == task.ID() {
				return nil, errors.New("a task cannot depend on itself")
			}
			deps = append(deps, doc.Link(d.Title()))
			edges[task.ID()] = append(edges[task.ID()], d.ID())
		}
		if loop := findLoop(edges); loop != "" {
			return nil, fmt.Errorf("depends makes a loop through %s", loop)
		}
		set = append(set, doc.Field{Key: "depends", Value: deps})
	}
	order := Order(task)
	name := TaskTitle(task)
	if in.Order != nil && *in.Order != order {
		if *in.Order <= 0 {
			return nil, errors.New("order is a number above 0")
		}
		if other := t.Task(strconv.Itoa(*in.Order)); other != nil {
			return nil, fmt.Errorf("T%d is taken by %s", *in.Order, other.Title())
		}
		order = *in.Order
		set = append(set, doc.Field{Key: "order", Value: order})
	}
	if in.Title != nil {
		name = doc.CleanTitle(*in.Title)
		if name == "" {
			return nil, errors.New("a task needs a title")
		}
	}
	content = doc.SetFields(content, set)
	newTitle := fmt.Sprintf("%s — T%d %s", t.Title(), order, name)
	if newTitle == task.Title() {
		if err := w.tx.Write(task.Path, []byte(content)); err != nil {
			return nil, err
		}
		return w.finish(fmt.Sprintf("set %s in %s", Label(task), t.Title()), t.ID())
	}
	if err := w.free(newTitle); err != nil {
		return nil, err
	}
	if err := w.retitle([]change.Retitle{{Old: task.Title(), New: newTitle}}, map[string]string{task.Path: content}, map[string]string{task.Path: path.Join(t.Folder(), newTitle+".md")}); err != nil {
		return nil, err
	}
	return w.finish(fmt.Sprintf("rename %s to %s", task.Title(), newTitle), t.ID())
}

// retitle moves documents to new paths and rewrites every link to their old titles, in
// the write's commit. contents holds new content for a path, moves the new path of a
// document that moves.
func (w *write) retitle(retitles []change.Retitle, contents, moves map[string]string) error {
	rewrites, _ := change.Rewrites(w.idx, retitles, contents, nil)
	for _, rw := range rewrites {
		contents[rw.Path] = rw.Content
	}
	for from, to := range moves {
		content, ok := contents[from]
		if !ok {
			content = w.idx.ByPath(from).Content
		}
		if err := w.tx.Remove(from); err != nil {
			return err
		}
		if err := w.tx.Write(to, []byte(content)); err != nil {
			return err
		}
		delete(contents, from)
	}
	for rel, content := range contents {
		if _, err := w.tx.WriteIfChanged(rel, []byte(content)); err != nil {
			return err
		}
	}
	return nil
}

// Set changes a thread's title, priority, blocked line, or scope. A new title renames
// the folder and every document, and rewrites every link to them.
func Set(v *vault.Vault, key string, in SetIn, now time.Time) (*Result, error) {
	w, err := begin(v, now)
	if err != nil {
		return nil, err
	}
	defer w.tx.Close()
	t, err := w.b.Find(key)
	if err != nil {
		return nil, err
	}
	set := []doc.Field{{Key: "updated", Value: today(now)}}
	var what []string
	if in.Priority != nil {
		if !slices.Contains(Priorities, *in.Priority) {
			return nil, fmt.Errorf("priority %q; it is one of %s", *in.Priority, strings.Join(Priorities, ", "))
		}
		set = append(set, doc.Field{Key: "priority", Value: *in.Priority})
		what = append(what, "priority "+*in.Priority)
	}
	if in.Blocked != nil {
		set = append(set, doc.Field{Key: "blocked", Value: oneLine(*in.Blocked)})
		if strings.TrimSpace(*in.Blocked) == "" {
			what = append(what, "unblocked")
		} else {
			what = append(what, "blocked")
		}
	}
	// A new first scope files the thread's folder under that scope; no scope, at the top.
	home := path.Dir(t.Folder())
	if in.Scope != nil {
		scope, err := w.scopes(*in.Scope)
		if err != nil {
			return nil, err
		}
		set = append(set, doc.Field{Key: "scope", Value: scope})
		what = append(what, "scope")
		home = w.homeFolder(scope)
	}
	content := doc.SetFields(t.Stub.Content, set)
	if in.Title == nil || doc.CleanTitle(*in.Title) == t.Title() {
		if len(what) == 0 {
			return nil, errors.New("set needs a title, priority, blocked, or scope")
		}
		if err := w.tx.Write(t.Stub.Path, []byte(content)); err != nil {
			return nil, err
		}
		if err := w.moveFolder(t.Folder(), path.Join(home, t.Title()), false); err != nil {
			return nil, err
		}
		return w.finish(fmt.Sprintf("%s: %s", t.Title(), strings.Join(what, ", ")), t.ID())
	}
	title := doc.CleanTitle(*in.Title)
	if title == "" {
		return nil, errors.New("a thread needs a title")
	}
	if links.Key(title) != links.Key(t.Title()) {
		if err := w.free(title); err != nil {
			return nil, err
		}
	}
	folder := path.Join(home, title)
	if v.Exists(folder) && links.Key(title) != links.Key(t.Title()) {
		return nil, fmt.Errorf("the folder %s exists", folder)
	}
	var retitles []change.Retitle
	moves := map[string]string{}
	contents := map[string]string{t.Stub.Path: content}
	for _, d := range append([]*doc.Doc{t.Stub}, t.Docs()...) {
		newTitle := title + strings.TrimPrefix(d.Title(), t.Title())
		if !strings.HasPrefix(d.Title(), t.Title()) {
			newTitle = d.Title()
		}
		if newTitle != d.Title() && links.Key(newTitle) != links.Key(d.Title()) {
			if err := w.free(newTitle); err != nil {
				return nil, err
			}
		}
		retitles = append(retitles, change.Retitle{Old: d.Title(), New: newTitle})
		moves[d.Path] = path.Join(folder, newTitle+".md")
	}
	if err := w.retitle(retitles, contents, moves); err != nil {
		return nil, err
	}
	// The user's own files in the folder go with the documents.
	if w.v.Exists(t.Folder()) {
		if err := w.moveFolder(t.Folder(), folder, true); err != nil {
			return nil, err
		}
	}
	return w.finish(fmt.Sprintf("rename %s to %s", t.Title(), title), t.ID())
}

// Reopen takes up a closed thread again. The receipt stays, renamed as reopened and
// marked superseded, and every link to it follows.
func Reopen(v *vault.Vault, key string, now time.Time) (*Result, error) {
	w, err := begin(v, now)
	if err != nil {
		return nil, err
	}
	defer w.tx.Close()
	t, err := w.b.Find(key)
	if err != nil {
		return nil, err
	}
	if !t.Closed() {
		return nil, fmt.Errorf("%s is open; reopen takes up a closed thread", t.Title())
	}
	r := t.Receipt
	base := fmt.Sprintf("%s — Receipt (reopened %s)", t.Title(), today(now))
	title := base
	for n := 2; len(w.idx.TitleHolders(title)) > 0; n++ {
		title = fmt.Sprintf("%s %d", base, n)
	}
	content := doc.SetFields(r.Content, []doc.Field{{Key: "superseded", Value: true}, {Key: "updated", Value: today(now)}})
	stub := doc.SetField(t.Stub.Content, "updated", today(now))
	if err := w.retitle([]change.Retitle{{Old: r.Title(), New: title}}, map[string]string{r.Path: content, t.Stub.Path: stub}, map[string]string{r.Path: path.Join(t.Folder(), title+".md")}); err != nil {
		return nil, err
	}
	return w.finish("reopen "+t.Title(), t.ID())
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

// Holding lists the live sessions that list a thread, for the hooks.
func Holding(idx *vault.Index, title string) []*doc.Doc {
	var out []*doc.Doc
	for _, s := range idx.Of("session") {
		if !sessions.Live(s.Str("status")) {
			continue
		}
		for _, l := range s.List("threads") {
			if strings.EqualFold(doc.LinkTarget(l), title) {
				out = append(out, s)
			}
		}
	}
	return out
}
