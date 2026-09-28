// Package threads keeps a thread's documents: the stub, which is the thread's root and
// carries its state; the spec; the tasks; and the receipt. Nothing sets a thread's
// stage. It is the furthest document that exists, and Sync writes it onto the stub.
package threads

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/sessions"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

// Stages of a thread.
const (
	StageStub   = "stub"
	StageSpec   = "spec"
	StageTasks  = "tasks"
	StageClosed = "closed"
)

// Task statuses.
const (
	TaskOpen    = "open"
	TaskDone    = "done"
	TaskDropped = "dropped"
)

// Priorities, highest first.
var Priorities = []string{"high", "normal", "low", "someday"}

// Trailer names the thread a commit wrote.
const Trailer = "Atlas-Thread"

// Thread is a stub and the documents that name it.
type Thread struct {
	Stub    *doc.Doc
	Spec    *doc.Doc
	Tasks   []*doc.Doc
	Receipt *doc.Doc
	// Old are superseded receipts and any extra spec or receipt, which lint reports.
	Old []*doc.Doc
}

// ID is the thread's id: its stub's.
func (t *Thread) ID() string { return t.Stub.ID() }

// Title is the thread's title: its stub's.
func (t *Thread) Title() string { return t.Stub.Title() }

// Folder is the thread's folder.
func (t *Thread) Folder() string {
	i := strings.LastIndex(t.Stub.Path, "/")
	return t.Stub.Path[:i]
}

// Stage is the furthest document that exists.
func (t *Thread) Stage() string {
	switch {
	case t.Receipt != nil:
		return StageClosed
	case len(t.Tasks) > 0:
		return StageTasks
	case t.Spec != nil:
		return StageSpec
	}
	return StageStub
}

// Closed reports whether a receipt closes the thread.
func (t *Thread) Closed() bool { return t.Receipt != nil }

// Counts are done tasks over all tasks, dropped ones left out.
func (t *Thread) Counts() (done, total int) {
	for _, task := range t.Tasks {
		switch task.Str("status") {
		case TaskDone:
			done++
			total++
		case TaskOpen:
			total++
		}
	}
	return
}

// OpenTasks are the tasks still open.
func (t *Thread) OpenTasks() []*doc.Doc {
	var out []*doc.Doc
	for _, task := range t.Tasks {
		if task.Str("status") == TaskOpen {
			out = append(out, task)
		}
	}
	return out
}

// Order is a task's order.
func Order(task *doc.Doc) int { return task.Front.Int("order") }

// Label is a task's short name: T1.
func Label(task *doc.Doc) string { return "T" + strconv.Itoa(Order(task)) }

// TaskTitle is a task's own title, without the thread's title and its label.
func TaskTitle(task *doc.Doc) string {
	title := task.Title()
	thread := doc.LinkTarget(task.Str("thread"))
	rest := strings.TrimPrefix(title, thread+" — ")
	if label, name, ok := strings.Cut(rest, " "); ok && strings.HasPrefix(label, "T") {
		if _, err := strconv.Atoi(label[1:]); err == nil {
			return name
		}
	}
	return rest
}

// Board is every thread of a vault, read from one index.
type Board struct {
	Idx     *vault.Index
	Threads []*Thread
	byID    map[string]*Thread
	// live maps a lowercase title of a thread or task to the live sessions that list it.
	live map[string][]*doc.Doc
}

// Load reads every thread of the index.
func Load(idx *vault.Index) *Board {
	b := &Board{Idx: idx, byID: map[string]*Thread{}, live: map[string][]*doc.Doc{}}
	for _, s := range idx.Of("stub") {
		t := &Thread{Stub: s}
		b.Threads = append(b.Threads, t)
		b.byID[s.ID()] = t
	}
	for _, d := range idx.Of("spec", "task", "receipt") {
		t := b.owner(d)
		if t == nil {
			continue
		}
		switch d.Type() {
		case "spec":
			if t.Spec == nil {
				t.Spec = d
			} else {
				t.Old = append(t.Old, d)
			}
		case "task":
			t.Tasks = append(t.Tasks, d)
		case "receipt":
			if d.Front.Bool("superseded") || t.Receipt != nil {
				t.Old = append(t.Old, d)
			} else {
				t.Receipt = d
			}
		}
	}
	for _, t := range b.Threads {
		sort.SliceStable(t.Tasks, func(i, j int) bool { return Order(t.Tasks[i]) < Order(t.Tasks[j]) })
	}
	for _, s := range idx.Of("session") {
		if !sessions.Live(s.Str("status")) {
			continue
		}
		for _, field := range []string{"threads", "tasks"} {
			for _, l := range s.List(field) {
				k := strings.ToLower(doc.LinkTarget(l))
				b.live[k] = append(b.live[k], s)
			}
		}
	}
	return b
}

// owner is the thread a document names: by thread_id, else by its thread link.
func (b *Board) owner(d *doc.Doc) *Thread {
	if t := b.byID[d.Str("thread_id")]; t != nil {
		return t
	}
	if s := b.Idx.Linked(d.Str("thread")); s != nil {
		return b.byID[s.ID()]
	}
	return nil
}

// Find resolves a thread from any key: its id or title, or any of its documents.
func (b *Board) Find(key string) (*Thread, error) {
	d, err := b.Idx.Resolve(key)
	if err != nil {
		return nil, err
	}
	switch d.Type() {
	case "stub":
		return b.byID[d.ID()], nil
	case "spec", "task", "receipt":
		if t := b.owner(d); t != nil {
			return t, nil
		}
		return nil, fmt.Errorf("%s names no thread", vault.Title(d))
	}
	return nil, fmt.Errorf("%s is a %s, not a thread; a thread is named by its stub", vault.Title(d), d.Type())
}

// FindTask resolves a task: its id or title, or "T2" with a thread.
func (b *Board) FindTask(key, thread string) (*Thread, *doc.Doc, error) {
	if thread != "" {
		t, err := b.Find(thread)
		if err != nil {
			return nil, nil, err
		}
		if task := t.Task(key); task != nil {
			return t, task, nil
		}
	}
	d, err := b.Idx.ResolveType(key, "task")
	if err != nil {
		return nil, nil, err
	}
	t := b.owner(d)
	if t == nil {
		return nil, nil, fmt.Errorf("%s names no thread", vault.Title(d))
	}
	return t, d, nil
}

// Task finds a task of the thread by its id, its title, its own title, its label, or its
// order.
func (t *Thread) Task(key string) *doc.Doc {
	key = strings.TrimSpace(doc.LinkTarget(key))
	for _, task := range t.Tasks {
		if key == task.ID() || strings.EqualFold(key, task.Title()) || strings.EqualFold(key, TaskTitle(task)) ||
			strings.EqualFold(key, Label(task)) || key == strconv.Itoa(Order(task)) {
			return task
		}
	}
	return nil
}

// Ready reports whether a task is open and every task it depends on is done or dropped.
func (b *Board) Ready(t *Thread, task *doc.Doc) bool {
	if task.Str("status") != TaskOpen {
		return false
	}
	for _, dep := range task.List("depends") {
		d := b.Idx.Linked(dep)
		if d != nil && d.Str("status") == TaskOpen {
			return false
		}
	}
	return true
}

// Holders are the live sessions that list a thread or a task.
func (b *Board) Holders(title string) []*doc.Doc {
	return b.live[strings.ToLower(title)]
}

// Next is the thread's next step: the furthest missing document, or the first ready
// task.
func (b *Board) Next(t *Thread) string {
	switch {
	case t.Closed():
		return "none"
	case len(t.Tasks) > 0:
		for _, task := range t.Tasks {
			if b.Ready(t, task) {
				return "task " + Label(task)
			}
		}
		if len(t.OpenTasks()) > 0 {
			return "task " + Label(t.OpenTasks()[0])
		}
		return "receipt"
	case t.Spec != nil:
		return "tasks"
	}
	return "spec"
}

// TaskView is a task of a Thread View.
type TaskView struct {
	Ref     vault.Ref `json:"ref"`
	Depends []string  `json:"depends"`
	Ready   bool      `json:"ready"`
}

// View is a thread with all its documents.
type View struct {
	Stub     vault.Ref   `json:"stub"`
	Spec     *vault.Ref  `json:"spec,omitempty"`
	Tasks    []TaskView  `json:"tasks"`
	Receipt  *vault.Ref  `json:"receipt,omitempty"`
	Sessions []vault.Ref `json:"sessions"`
	Changes  []vault.Ref `json:"changes"`
	Next     string      `json:"next"`
	Warnings []string    `json:"warnings,omitempty"`
}

// View builds a thread's view.
func (b *Board) View(t *Thread) *View {
	idx := b.Idx
	v := &View{Stub: idx.Ref(t.Stub), Tasks: []TaskView{}, Sessions: []vault.Ref{}, Changes: []vault.Ref{}, Next: b.Next(t)}
	if t.Spec != nil {
		r := idx.Ref(t.Spec)
		v.Spec = &r
	}
	if t.Receipt != nil {
		r := idx.Ref(t.Receipt)
		v.Receipt = &r
	}
	for _, task := range t.Tasks {
		tv := TaskView{Ref: idx.Ref(task), Depends: []string{}, Ready: b.Ready(t, task)}
		for _, dep := range task.List("depends") {
			if d := idx.Linked(dep); d != nil {
				tv.Depends = append(tv.Depends, d.ID())
			}
		}
		v.Tasks = append(v.Tasks, tv)
	}
	var sess []*doc.Doc
	for _, s := range idx.Of("session") {
		for _, l := range s.List("threads") {
			if strings.EqualFold(doc.LinkTarget(l), t.Title()) {
				sess = append(sess, s)
				break
			}
		}
	}
	sort.SliceStable(sess, func(i, j int) bool { return sess[i].Str("started") > sess[j].Str("started") })
	v.Sessions = idx.Refs(sess)
	for _, c := range idx.Of("change") {
		if strings.EqualFold(doc.LinkTarget(c.Str("thread")), t.Title()) {
			v.Changes = append(v.Changes, idx.Ref(c))
		}
	}
	return v
}

// Column is one stage of the board.
type Column struct {
	Stage   string      `json:"stage"`
	Threads []vault.Ref `json:"threads"`
}

// BoardView is the board: the open threads by stage, the furthest first, and the last
// ten closed.
type BoardView struct {
	Board  []Column    `json:"board"`
	Closed []vault.Ref `json:"closed"`
}

// MaxClosed is how many closed threads the board lists.
const MaxClosed = 10

// View of the whole board.
func (b *Board) BoardView() *BoardView {
	out := &BoardView{Closed: []vault.Ref{}}
	for _, stage := range []string{StageTasks, StageSpec, StageStub} {
		col := Column{Stage: stage, Threads: []vault.Ref{}}
		var list []*Thread
		for _, t := range b.Threads {
			if t.Stage() == stage {
				list = append(list, t)
			}
		}
		sort.SliceStable(list, func(i, j int) bool { return Less(list[i], list[j]) })
		for _, t := range list {
			col.Threads = append(col.Threads, b.Idx.Ref(t.Stub))
		}
		out.Board = append(out.Board, col)
	}
	var closed []*Thread
	for _, t := range b.Threads {
		if t.Closed() {
			closed = append(closed, t)
		}
	}
	sort.SliceStable(closed, func(i, j int) bool { return closed[i].Stub.Str("updated") > closed[j].Stub.Str("updated") })
	for i, t := range closed {
		if i == MaxClosed {
			break
		}
		out.Closed = append(out.Closed, b.Idx.Ref(t.Stub))
	}
	return out
}

// Less orders open threads: an active one first, then by priority, then the most
// recently updated.
func Less(a, b *Thread) bool {
	if aa, ba := a.Stub.Front.Bool("active"), b.Stub.Front.Bool("active"); aa != ba {
		return aa
	}
	if pa, pb := priorityRank(a.Stub.Str("priority")), priorityRank(b.Stub.Str("priority")); pa != pb {
		return pa < pb
	}
	return a.Stub.Str("updated") > b.Stub.Str("updated")
}

func priorityRank(p string) int {
	for i, x := range Priorities {
		if x == p {
			return i
		}
	}
	return 1
}

// Open lists the open threads, in board order: the furthest stage first.
func (b *Board) Open() []*Thread {
	var out []*Thread
	for _, stage := range []string{StageTasks, StageSpec, StageStub} {
		var list []*Thread
		for _, t := range b.Threads {
			if t.Stage() == stage {
				list = append(list, t)
			}
		}
		sort.SliceStable(list, func(i, j int) bool { return Less(list[i], list[j]) })
		out = append(out, list...)
	}
	return out
}

// TitleFromText is a thread's title when the model gives none: the first line of the
// text, cut at a word boundary near 60 characters.
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

// now formats for the documents.
func today(now time.Time) string { return vault.Date(now) }
