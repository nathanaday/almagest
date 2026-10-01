package thread

import (
	"fmt"
	"sort"
	"strings"

	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

// Bounds of what a load carries.
const (
	MaxSpecLines    = 400
	MaxDetailLines  = 40
	MaxNoteLines    = 60
	MaxNotes        = 3
	MaxLoadSessions = 3
	MaxProgress     = 5
	MaxLoadEvents   = 8
)

// Text is a document with its body.
type Text struct {
	Ref       vault.Ref `json:"ref"`
	Text      string    `json:"text"`
	Truncated bool      `json:"truncated,omitempty"`
}

// TaskView is an open task with its details.
type TaskView struct {
	ID           string   `json:"id"`
	Text         string   `json:"text"`
	Requirements []string `json:"requirements"`
	Details      string   `json:"details,omitempty"`
}

// RepoView is a repository a task list changes.
type RepoView struct {
	Title string `json:"title"`
	Path  string `json:"path"`
}

// ListView is one task list: its open tasks in full, the rest as their lines.
type ListView struct {
	Ref        vault.Ref  `json:"ref"`
	Repository *RepoView  `json:"repository,omitempty"`
	Open       []TaskView `json:"open"`
	Closed     []string   `json:"closed" jsonschema:"the done and dropped tasks, one line each"`
}

// RoundView is a thread's last verification: its verdict, the requirements that do not
// pass, and the findings with no outcome yet.
type RoundView struct {
	Ref      vault.Ref `json:"ref"`
	Round    int       `json:"round"`
	Verdict  string    `json:"verdict"`
	Failed   []Row     `json:"failed"`
	Findings []Finding `json:"findings" jsonschema:"the open findings"`
}

// Place is a thread's place in its chord and among the threads it waits on.
type Place struct {
	Chord  *vault.Ref  `json:"chord,omitempty"`
	After  []vault.Ref `json:"after"`
	Before []vault.Ref `json:"before"`
	Ready  bool        `json:"ready"`
}

// SessionView is a session that worked on a thread, with its last progress lines.
type SessionView struct {
	Ref      vault.Ref `json:"ref"`
	Progress []string  `json:"progress"`
}

// Loaded is everything an agent needs to take up a thread, in one read.
type Loaded struct {
	Thread       vault.Ref     `json:"thread"`
	Handoff      string        `json:"handoff"`
	Idea         string        `json:"idea"`
	Blocked      string        `json:"blocked,omitempty"`
	Missing      []string      `json:"missing"`
	Next         Step          `json:"next"`
	Spec         *Text         `json:"spec,omitempty"`
	Requirements []Requirement `json:"requirements"`
	Lists        []ListView    `json:"lists"`
	Verification *RoundView    `json:"verification,omitempty"`
	Place        Place         `json:"place"`
	Knowledge    []vault.Ref   `json:"knowledge" jsonschema:"the wiki pages the spec cites; read the ones the next step needs"`
	Notes        []Text        `json:"notes" jsonschema:"the last notes on the thread"`
	Sessions     []SessionView `json:"sessions"`
	Events       []vault.Ref   `json:"events"`
	Changes      []vault.Ref   `json:"changes"`
}

func bounded(text string, n int) (string, bool) {
	text = strings.TrimSpace(text)
	lines := strings.Split(text, "\n")
	if len(lines) <= n {
		return text, false
	}
	return strings.Join(lines[:n], "\n") + "\n[…]", true
}

// LoadThread builds the load of a thread, named by its stub or by one of its documents.
func LoadThread(idx *vault.Index, key string) (*Loaded, error) {
	d, err := idx.ResolveType(key, "stub", "spec", "tasks", "verification")
	if err != nil {
		return nil, err
	}
	b := Load(idx)
	t := b.Thread(d)
	if t == nil {
		return nil, fmt.Errorf("%s names no thread: its thread field links no stub", d.Title())
	}
	return b.Loaded(t), nil
}

// Loaded builds the load of a thread.
func (b *Board) Loaded(t *Thread) *Loaded {
	idx := b.Idx
	stub := t.Stub
	idea, _ := doc.Section(stub.Body, "Idea")
	out := &Loaded{
		Thread: b.Ref(stub), Handoff: Handoff(stub), Idea: idea, Blocked: b.Blocked(stub),
		Missing: b.Missing(stub), Next: b.Next(stub),
		Requirements: []Requirement{}, Lists: []ListView{}, Knowledge: []vault.Ref{}, Notes: []Text{},
		Sessions: []SessionView{}, Events: []vault.Ref{}, Changes: []vault.Ref{},
		Place: Place{After: b.refs(b.After(stub)), Before: b.refs(b.Before(stub)), Ready: b.Ready(stub)},
	}
	if c := b.Chord(stub); c != nil {
		ref := b.Ref(c)
		out.Place.Chord = &ref
	}
	if t.Spec != nil {
		text, cut := bounded(doc.StripLead(t.Spec.Body), MaxSpecLines)
		out.Spec = &Text{Ref: b.Ref(t.Spec), Text: text, Truncated: cut}
		out.Requirements = append(out.Requirements, Requirements(t.Spec.Body)...)
	}
	for _, l := range t.Lists {
		lv := ListView{Ref: b.Ref(l), Open: []TaskView{}, Closed: []string{}}
		if r := idx.Linked(l.Str("repository")); r != nil {
			lv.Repository = &RepoView{Title: r.Title(), Path: r.Str("path")}
		}
		for _, task := range Tasks(l) {
			if task.State != TaskOpen {
				lv.Closed = append(lv.Closed, strings.TrimPrefix(TaskLine(task), "- "))
				continue
			}
			details, _ := bounded(Details(l, task.ID), MaxDetailLines)
			if task.ID == "" {
				details = ""
			}
			lv.Open = append(lv.Open, TaskView{ID: task.ID, Text: task.Text, Requirements: task.Requirements, Details: details})
		}
		out.Lists = append(out.Lists, lv)
	}
	if last := t.Last(); last != nil {
		rv := &RoundView{Ref: b.Ref(last), Round: last.Front.Int("round"), Verdict: b.Verdict(t, last), Failed: []Row{}, Findings: []Finding{}}
		for _, r := range Rows(last) {
			if r.Result != Pass {
				rv.Failed = append(rv.Failed, r)
			}
		}
		for _, f := range Findings(last) {
			if f.Open {
				rv.Findings = append(rv.Findings, f)
			}
		}
		out.Verification = rv
	}
	out.Knowledge = append(out.Knowledge, idx.Refs(b.Knowledge(t))...)
	events := b.events[stub.ID()]
	for i := len(events) - 1; i >= 0; i-- {
		e := events[i]
		if len(out.Events) < MaxLoadEvents {
			out.Events = append(out.Events, idx.Ref(e))
		}
		if e.Str("kind") == "note" && len(out.Notes) < MaxNotes {
			text, _ := doc.Section(e.Body, "Note")
			text, cut := bounded(text, MaxNoteLines)
			out.Notes = append(out.Notes, Text{Ref: idx.Ref(e), Text: text, Truncated: cut})
		}
	}
	for i, s := range b.Sessions(t) {
		if i == MaxLoadSessions {
			break
		}
		sv := SessionView{Ref: idx.Ref(s), Progress: []string{}}
		progress, _ := doc.Section(s.Body, "Progress")
		for _, l := range strings.Split(progress, "\n") {
			if l = strings.TrimSpace(l); l != "" {
				sv.Progress = append(sv.Progress, l)
			}
		}
		if n := len(sv.Progress); n > MaxProgress {
			sv.Progress = sv.Progress[n-MaxProgress:]
		}
		out.Sessions = append(out.Sessions, sv)
	}
	names := map[string]bool{}
	for _, x := range t.Docs() {
		names[strings.ToLower(x.Title())] = true
	}
	var changes []*doc.Doc
	for _, c := range idx.Of("change") {
		if names[strings.ToLower(doc.LinkTarget(c.Str("work")))] {
			changes = append(changes, c)
			continue
		}
		for _, a := range c.List("absorbs") {
			if names[strings.ToLower(doc.LinkTarget(a))] {
				changes = append(changes, c)
				break
			}
		}
	}
	sort.SliceStable(changes, func(i, j int) bool { return changes[i].Str("created") > changes[j].Str("created") })
	out.Changes = append(out.Changes, idx.Refs(changes)...)
	return out
}

// ChordLoaded is everything an agent needs to take up a chord.
type ChordLoaded struct {
	ChordView
	Handoff string       `json:"handoff"`
	Goal    string       `json:"goal"`
	Ready   []vault.Ref  `json:"ready" jsonschema:"the threads work can go on with now, in order"`
	Canvas  *CanvasState `json:"canvas,omitempty"`
}

// LoadChord builds the load of a chord.
func LoadChord(idx *vault.Index, key string) (*ChordLoaded, error) {
	c, err := idx.ResolveType(key, "chord")
	if err != nil {
		return nil, err
	}
	b := Load(idx)
	goal, _ := doc.Section(c.Body, "Goal")
	out := &ChordLoaded{ChordView: b.ChordView(c), Handoff: Handoff(c), Goal: goal, Ready: b.refs(b.ChordReady(c))}
	if cs, err := CanvasStatus(idx, c.ID()); err == nil && cs.Exists {
		out.Canvas = cs
	}
	return out, nil
}
