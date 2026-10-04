// Package mcpserver serves the nine tools, each a thin layer over one package. A tool
// takes ids or titles and returns entities; every rule is the package's, and the
// refusals come from there. After a write, the server writes the views.
package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nathanaday/atlas-obsidian/internal/brief"
	"github.com/nathanaday/atlas-obsidian/internal/change"
	"github.com/nathanaday/atlas-obsidian/internal/core"
	"github.com/nathanaday/atlas-obsidian/internal/lint"
	"github.com/nathanaday/atlas-obsidian/internal/match"
	"github.com/nathanaday/atlas-obsidian/internal/search"
	"github.com/nathanaday/atlas-obsidian/internal/sessions"
	"github.com/nathanaday/atlas-obsidian/internal/source"
	"github.com/nathanaday/atlas-obsidian/internal/thread"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

// Name is the server's name; hosts name its tools mcp__plugin_<plugin>_atlas__<tool>.
const Name = "atlas"

// Options configure a server.
type Options struct {
	Version string
	// Dir is where the session started.
	Dir    string
	Getenv func(string) string
	Now    func() time.Time
}

// Server serves one session.
type Server struct {
	opts Options
}

// New builds a server.
func New(opts Options) *Server {
	if opts.Getenv == nil {
		opts.Getenv = os.Getenv
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Dir == "" {
		opts.Dir, _ = os.Getwd()
	}
	return &Server{opts: opts}
}

// ToolNames are the tools, in the order the server lists them.
func ToolNames() []string {
	return []string{"vault", "search", "context", "match", "source", "change", "thread", "chord", "lint"}
}

// open resolves the vault a call acts on.
func (s *Server) open(name string) (*vault.Vault, error) {
	return vault.Select(name, s.opts.Dir, vault.HomeFrom(s.opts.Getenv), s.opts.Getenv(vault.EnvVault))
}

// index loads the vault a call acts on.
func (s *Server) index(name string) (*vault.Index, error) {
	v, err := s.open(name)
	if err != nil {
		return nil, err
	}
	return vault.Load(v)
}

// views writes the views after a write, and returns the notes it moved out of views/. A
// view that fails to write fails no call: the next sync writes it.
func (s *Server) views(v *vault.Vault) []vault.Moved {
	moved, _ := core.Views(v, s.opts.Now())
	return moved
}

// VaultIn is the vault tool's input.
type VaultIn struct {
	Action      string `json:"action,omitempty" jsonschema:"status (the default), init, sync, or mention"`
	Vault       string `json:"vault,omitempty" jsonschema:"the vault, by path or by name from ~/.atlas/config.json; the session's vault when empty"`
	Name        string `json:"name,omitempty" jsonschema:"init: the vault's name"`
	Path        string `json:"path,omitempty" jsonschema:"init: the folder that becomes the vault"`
	Tagging     string `json:"tagging,omitempty" jsonschema:"init: open (the default; the agent adds tags freely) or known (only tags that exist, unless the user agrees)"`
	Description string `json:"description,omitempty" jsonschema:"init: one or two sentences on what the vault is for; the body of Atlas.md"`
	Views       bool   `json:"views,omitempty" jsonschema:"sync: the statuses, callouts, and views only, with no git"`
	Note        string `json:"note,omitempty" jsonschema:"mention: the path of the note that holds the mention"`
	Line        int    `json:"line,omitempty" jsonschema:"mention: the mention's line"`
	Link        string `json:"link,omitempty" jsonschema:"mention: the document that answers it (id or title)"`
}

// VaultOut is the vault tool's output.
type VaultOut struct {
	Status         *core.Status  `json:"status,omitempty"`
	Synced         *core.Synced  `json:"synced,omitempty"`
	Mention        *core.Mention `json:"mention,omitempty"`
	MovedFromViews []vault.Moved `json:"moved_from_views,omitempty" jsonschema:"notes of the user's found in views/, moved to inbox/; tell the user where each went"`
}

func (s *Server) vaultTool(ctx context.Context, req *mcp.CallToolRequest, in VaultIn) (*mcp.CallToolResult, VaultOut, error) {
	now := s.opts.Now()
	switch in.Action {
	case "", "status":
		idx, err := s.index(in.Vault)
		if err != nil {
			return nil, VaultOut{}, err
		}
		st := core.StatusOf(idx, now)
		st.Versions = map[string]string{"binary": s.opts.Version, "obsidian_plugin": idx.V.InstalledPluginVersion()}
		return nil, VaultOut{Status: st}, nil
	case "init":
		path := in.Path
		if path == "" {
			path = s.opts.Dir
		}
		st, err := core.Init(vault.InitOptions{Path: path, Name: in.Name, Description: in.Description, Tagging: in.Tagging}, vault.HomeFrom(s.opts.Getenv), now)
		if err != nil {
			return nil, VaultOut{}, err
		}
		return nil, VaultOut{Status: st}, nil
	case "sync":
		v, err := s.open(in.Vault)
		if err != nil {
			return nil, VaultOut{}, err
		}
		synced, err := core.Sync(v, now, core.SyncOptions{Views: in.Views})
		if err != nil {
			return nil, VaultOut{}, err
		}
		return nil, VaultOut{Synced: synced}, nil
	case "mention":
		v, err := s.open(in.Vault)
		if err != nil {
			return nil, VaultOut{}, err
		}
		m, err := core.CloseMention(v, in.Note, in.Line, in.Link)
		if err != nil {
			return nil, VaultOut{}, err
		}
		return nil, VaultOut{Mention: m, MovedFromViews: s.views(v)}, nil
	}
	return nil, VaultOut{}, fmt.Errorf("vault takes action status, init, sync, or mention, not %q", in.Action)
}

// SearchIn is the search tool's input.
type SearchIn struct {
	Vault string `json:"vault,omitempty" jsonschema:"the vault, by path or by name from ~/.atlas/config.json; the session's vault when empty"`
	search.Query
}

func (s *Server) searchTool(ctx context.Context, req *mcp.CallToolRequest, in SearchIn) (*mcp.CallToolResult, *search.Hits, error) {
	idx, err := s.index(in.Vault)
	if err != nil {
		return nil, nil, err
	}
	hits, err := search.Search(idx, in.Query)
	return nil, hits, err
}

// ContextIn is the context tool's input.
type ContextIn struct {
	Vault string `json:"vault,omitempty" jsonschema:"the vault, by path or by name from ~/.atlas/config.json; the session's vault when empty"`
	brief.Input
}

func (s *Server) contextTool(ctx context.Context, req *mcp.CallToolRequest, in ContextIn) (*mcp.CallToolResult, *brief.Brief, error) {
	idx, err := s.index(in.Vault)
	if err != nil {
		return nil, nil, err
	}
	b, err := brief.Of(idx, in.Input)
	return nil, b, err
}

// MatchIn is the match tool's input.
type MatchIn struct {
	Vault string `json:"vault,omitempty" jsonschema:"the vault, by path or by name from ~/.atlas/config.json; the session's vault when empty"`
	match.Input
}

func (s *Server) matchTool(ctx context.Context, req *mcp.CallToolRequest, in MatchIn) (*mcp.CallToolResult, *match.Map, error) {
	idx, err := s.index(in.Vault)
	if err != nil {
		return nil, nil, err
	}
	m, err := match.Run(idx, in.Input)
	return nil, m, err
}

// SourceIn is the source tool's input.
type SourceIn struct {
	Action string `json:"action,omitempty" jsonschema:"capture, chunks, or read"`
	Vault  string `json:"vault,omitempty" jsonschema:"the vault, by path or by name from ~/.atlas/config.json; the session's vault when empty"`
	source.Request
	Doc   string `json:"doc,omitempty" jsonschema:"chunks, read: the document (id or title); any document, not only a source"`
	Chunk int    `json:"chunk,omitempty" jsonschema:"read: the chunk's index, from 1"`
}

// SourceOut is the source tool's output.
type SourceOut struct {
	Captured       []source.Captured `json:"captured,omitempty"`
	Events         []vault.Ref       `json:"events,omitempty"`
	Commit         string            `json:"commit,omitempty"`
	Chunks         []source.Chunk    `json:"chunks,omitempty"`
	Blob           *source.TextBlob  `json:"blob,omitempty"`
	MovedFromViews []vault.Moved     `json:"moved_from_views,omitempty" jsonschema:"notes of the user's found in views/, moved to inbox/; tell the user where each went"`
}

func (s *Server) sourceTool(ctx context.Context, req *mcp.CallToolRequest, in SourceIn) (*mcp.CallToolResult, SourceOut, error) {
	switch in.Action {
	case "capture":
		v, err := s.open(in.Vault)
		if err != nil {
			return nil, SourceOut{}, err
		}
		res, err := source.Capture(v, in.Request, thread.Opts{Now: s.opts.Now(), By: thread.ByAgent})
		if err != nil {
			return nil, SourceOut{}, err
		}
		return nil, SourceOut{Captured: res.Captured, Events: res.Events, Commit: res.Commit, MovedFromViews: s.views(v)}, nil
	case "chunks":
		idx, err := s.index(in.Vault)
		if err != nil {
			return nil, SourceOut{}, err
		}
		chunks, err := source.Chunks(idx, in.Doc)
		return nil, SourceOut{Chunks: chunks}, err
	case "read":
		idx, err := s.index(in.Vault)
		if err != nil {
			return nil, SourceOut{}, err
		}
		blob, err := source.Read(idx, in.Doc, in.Chunk)
		return nil, SourceOut{Blob: blob}, err
	}
	return nil, SourceOut{}, fmt.Errorf("source takes action capture, chunks, or read, not %q", in.Action)
}

// ChangeIn is the change tool's input.
type ChangeIn struct {
	Action     string         `json:"action,omitempty" jsonschema:"show (the default), propose, apply, reject, or undo"`
	Vault      string         `json:"vault,omitempty" jsonschema:"the vault, by path or by name from ~/.atlas/config.json; the session's vault when empty"`
	ID         string         `json:"id,omitempty" jsonschema:"show, apply, reject, undo: the change (id or title)"`
	Reason     string         `json:"reason,omitempty" jsonschema:"reject: why, in one line"`
	Title      string         `json:"title,omitempty" jsonschema:"propose: a short name; the file name and the commit subject"`
	Notes      string         `json:"notes,omitempty" jsonschema:"propose: what the change does and why, and each skipped subject with its reason"`
	Absorbs    []string       `json:"absorbs,omitempty" jsonschema:"propose: ids of the documents the change absorbs into the wiki"`
	Work       string         `json:"work,omitempty" jsonschema:"propose: the thread or the chord the change serves, if any"`
	Supersedes string         `json:"supersedes,omitempty" jsonschema:"propose: a proposed change this one replaces"`
	NewTags    bool           `json:"new_tags,omitempty" jsonschema:"propose: allow tags no document holds, in tagging: known, after the user agreed"`
	Writes     []change.Write `json:"writes,omitempty" jsonschema:"propose: the writes: create (type topic or repository, kind, title, fields, body), modify (id, fields, body, base), promote (id of an open stub, kind, fields, body, title), rename (id, title), remove (id, redirect), confirm (id), retag (from, to)"`
}

func (s *Server) changeTool(ctx context.Context, req *mcp.CallToolRequest, in ChangeIn) (*mcp.CallToolResult, *change.Preview, error) {
	v, err := s.open(in.Vault)
	if err != nil {
		return nil, nil, err
	}
	now := s.opts.Now()
	done := func(pv *change.Preview, err error) (*mcp.CallToolResult, *change.Preview, error) {
		if err == nil {
			pv.MovedFromViews = s.views(v)
		}
		return nil, pv, err
	}
	switch in.Action {
	case "", "show":
		idx, err := vault.Load(v)
		if err != nil {
			return nil, nil, err
		}
		pv, err := change.Show(idx, in.ID)
		return nil, pv, err
	case "propose":
		return done(change.Propose(v, change.Plan{Title: in.Title, Notes: in.Notes, Absorbs: in.Absorbs, Work: in.Work, Supersedes: in.Supersedes, NewTags: in.NewTags, Writes: in.Writes}, now))
	case "apply":
		return done(change.Apply(v, in.ID, now, sessions.UserAnswered(v)))
	case "reject":
		return done(change.Reject(v, in.ID, in.Reason, now))
	case "undo":
		return done(change.Undo(v, in.ID, now))
	}
	return nil, nil, fmt.Errorf("change takes action show, propose, apply, reject, or undo, not %q", in.Action)
}

// ThreadIn is the thread tool's input.
type ThreadIn struct {
	Action string `json:"action,omitempty" jsonschema:"list (the default), load, stub, spec, tasks, start, check, verify, finding, drop, reopen, block, unblock, resolve, note, or set"`
	Vault  string `json:"vault,omitempty" jsonschema:"the vault, by path or by name from ~/.atlas/config.json; the session's vault when empty"`
	Thread string `json:"thread,omitempty" jsonschema:"every action on one thread: the thread, by the id or title of its stub or of any of its documents"`
	Doc    string `json:"doc,omitempty" jsonschema:"note: any document, by id or title"`
	// list.
	Tags       []string `json:"tags,omitempty" jsonschema:"stub: the categories; list: only threads that hold every one"`
	Repository string   `json:"repository,omitempty" jsonschema:"tasks, finding: the repository of the task list; list: only threads with a task list for it"`
	Chord      string   `json:"chord,omitempty" jsonschema:"stub: the chord the new thread joins; list: only the threads of this chord"`
	// stub, spec, note.
	Text        string   `json:"text,omitempty" jsonschema:"stub: the user's words, as given; spec: the spec's body (Goal, Requirements as '- R1: …' lines, Rules, Decisions, Out of scope, Knowledge, Open questions); note: the note; finding with outcome stub: the words of a new stub"`
	Title       string   `json:"title,omitempty" jsonschema:"stub: a short title"`
	Description string   `json:"description,omitempty" jsonschema:"stub, spec: one sentence"`
	Priority    string   `json:"priority,omitempty" jsonschema:"stub: high, normal, low, or someday"`
	After       []string `json:"after,omitempty" jsonschema:"stub: the threads that must be verified first"`
	Inbox       string   `json:"inbox,omitempty" jsonschema:"stub: a note in inbox/ that the stub replaces"`
	NewTags     bool     `json:"new_tags,omitempty" jsonschema:"stub: allow a tag no document holds, in tagging: known, after the user agreed"`
	// tasks.
	Tasks []thread.TaskIn `json:"tasks,omitempty" jsonschema:"tasks: the tasks to write or append, each with the requirements it serves"`
	// start.
	Take bool `json:"take,omitempty" jsonschema:"start: take a thread another live session started"`
	// check.
	Task    string   `json:"task,omitempty" jsonschema:"check: the task's id (T3)"`
	State   string   `json:"state,omitempty" jsonschema:"check: done (the default), dropped, or open"`
	Commits []string `json:"commits,omitempty" jsonschema:"check: the commits that did the task"`
	Note    string   `json:"note,omitempty" jsonschema:"check: one line on what was done; needed when there is no commit"`
	// verify.
	Scope    string            `json:"scope,omitempty" jsonschema:"verify: what was checked, each repository with its commits"`
	Results  []thread.ResultIn `json:"results,omitempty" jsonschema:"verify: one result (pass or fail, with evidence) for every requirement of the spec"`
	Findings []string          `json:"findings,omitempty" jsonschema:"verify: what the check found, one line each"`
	Notes    string            `json:"notes,omitempty" jsonschema:"verify: anything else the round should record"`
	// finding.
	Finding string         `json:"finding,omitempty" jsonschema:"finding: the finding's id (F1) in the thread's last verification"`
	Outcome string         `json:"outcome,omitempty" jsonschema:"finding: task, spec, stub, knowledge, or accepted"`
	NewTask *thread.TaskIn `json:"new_task,omitempty" jsonschema:"finding with outcome task: the task that fixes it"`
	Link    string         `json:"link,omitempty" jsonschema:"finding: with stub, a stub that exists; with knowledge, the change or the topic that holds it"`
	// drop, reopen, block, check, finding.
	Reason string   `json:"reason,omitempty" jsonschema:"drop: why; reopen: why, optional; block: what the thread waits on, in one line; check with state dropped: why; finding with outcome accepted: why the user accepts it"`
	Became []string `json:"became,omitempty" jsonschema:"resolve: the documents the stub became"`
	// set.
	Set *thread.SetIn `json:"set,omitempty" jsonschema:"set: the thread (doc) and the fields to change; a field left out stays"`
}

// ThreadOut is the thread tool's output: the board, the load of one thread, or the state
// of the thread a write acted on with what the call wrote.
type ThreadOut struct {
	Board          *thread.BoardView `json:"board,omitempty"`
	Thread         *thread.Loaded    `json:"thread,omitempty"`
	State          *thread.State     `json:"state,omitempty"`
	Commit         string            `json:"commit,omitempty"`
	Wrote          []vault.Ref       `json:"wrote,omitempty"`
	Events         []vault.Ref       `json:"events,omitempty"`
	Started        string            `json:"started,omitempty"`
	MovedFromViews []vault.Moved     `json:"moved_from_views,omitempty" jsonschema:"notes of the user's found in views/, moved to inbox/; tell the user where each went"`
}

const threadActions = "list, load, stub, spec, tasks, start, check, verify, finding, drop, reopen, block, unblock, resolve, note, or set"

func (s *Server) threadTool(ctx context.Context, req *mcp.CallToolRequest, in ThreadIn) (*mcp.CallToolResult, ThreadOut, error) {
	v, err := s.open(in.Vault)
	if err != nil {
		return nil, ThreadOut{}, err
	}
	o := thread.Opts{Now: s.opts.Now(), By: thread.ByAgent}
	out := func(r *thread.Result, err error) (*mcp.CallToolResult, ThreadOut, error) {
		if err != nil {
			return nil, ThreadOut{}, err
		}
		return nil, ThreadOut{State: r.State, Commit: r.Commit, Wrote: r.Wrote, Events: r.Events, Started: r.Started, MovedFromViews: s.views(v)}, nil
	}
	switch in.Action {
	case "", "list":
		idx, err := vault.Load(v)
		if err != nil {
			return nil, ThreadOut{}, err
		}
		return nil, ThreadOut{Board: thread.Load(idx).BoardView(thread.Filter{Tags: in.Tags, Repository: in.Repository, Chord: in.Chord})}, nil
	case "load":
		idx, err := vault.Load(v)
		if err != nil {
			return nil, ThreadOut{}, err
		}
		loaded, err := thread.LoadThread(idx, in.Thread)
		if err != nil {
			return nil, ThreadOut{}, err
		}
		return nil, ThreadOut{Thread: loaded}, nil
	case "stub":
		return out(thread.Stub(v, thread.StubIn{Text: in.Text, Title: in.Title, Description: in.Description, Tags: in.Tags, Priority: in.Priority, Chord: in.Chord, After: in.After, Inbox: in.Inbox, NewTags: in.NewTags}, o))
	case "spec":
		return out(thread.Spec(v, thread.SpecIn{Thread: in.Thread, Text: in.Text, Description: in.Description}, o))
	case "tasks":
		return out(thread.TasksWrite(v, thread.TasksIn{Thread: in.Thread, Repository: in.Repository, Tasks: in.Tasks}, o))
	case "start":
		return out(thread.Start(v, in.Thread, in.Take, o))
	case "check":
		return out(thread.Check(v, thread.CheckIn{Thread: in.Thread, Task: in.Task, State: in.State, Commits: in.Commits, Note: in.Note, Reason: in.Reason}, o))
	case "verify":
		return out(thread.Verify(v, thread.VerifyIn{Thread: in.Thread, Scope: in.Scope, Results: in.Results, Findings: in.Findings, Notes: in.Notes}, o))
	case "finding":
		return out(thread.FindingOutcome(v, thread.FindingIn{Thread: in.Thread, Finding: in.Finding, Outcome: in.Outcome, Task: in.NewTask, Repository: in.Repository, Link: in.Link, Text: in.Text, Reason: in.Reason}, o))
	case "drop":
		return out(thread.Drop(v, in.Thread, in.Reason, o))
	case "reopen":
		return out(thread.Reopen(v, in.Thread, in.Reason, o))
	case "block":
		return out(thread.Block(v, in.Thread, in.Reason, o))
	case "unblock":
		return out(thread.Unblock(v, in.Thread, o))
	case "resolve":
		return out(thread.Resolve(v, in.Thread, in.Became, o))
	case "note":
		doc := in.Doc
		if doc == "" {
			doc = in.Thread
		}
		return out(thread.Note(v, doc, in.Text, o))
	case "set":
		if in.Set == nil {
			return nil, ThreadOut{}, errors.New("set takes set: {doc, and the fields to change}")
		}
		return out(thread.Set(v, *in.Set, o))
	}
	return nil, ThreadOut{}, fmt.Errorf("thread takes action %s, not %q", threadActions, in.Action)
}

// ChordIn is the chord tool's input.
type ChordIn struct {
	Action string `json:"action,omitempty" jsonschema:"list (the default), load, create, add, remove, order, drop, reopen, or set"`
	Vault  string `json:"vault,omitempty" jsonschema:"the vault, by path or by name from ~/.atlas/config.json; the session's vault when empty"`
	Chord  string `json:"chord,omitempty" jsonschema:"every action on one chord: the chord, by id or title"`
	// create.
	Title       string                 `json:"title,omitempty" jsonschema:"create: the chord's title"`
	Text        string                 `json:"text,omitempty" jsonschema:"create: the chord's Goal: what is true when every thread is closed"`
	Description string                 `json:"description,omitempty" jsonschema:"create: one sentence"`
	Tags        []string               `json:"tags,omitempty" jsonschema:"create: the chord's tags; each new stub takes them. list: only chords that hold every one"`
	Priority    string                 `json:"priority,omitempty" jsonschema:"create: high, normal, low, or someday"`
	Threads     []thread.ChordThreadIn `json:"threads,omitempty" jsonschema:"create: the threads, each a new stub (title, text) or a stub that exists (thread), with the threads it comes after"`
	NewTags     bool                   `json:"new_tags,omitempty"`
	// add, remove.
	Thread string   `json:"thread,omitempty" jsonschema:"add, remove: the thread, by id or title"`
	After  []string `json:"after,omitempty" jsonschema:"add: the threads it comes after"`
	// order.
	Order []thread.OrderIn `json:"order,omitempty" jsonschema:"order: each thread whose place changes, with every thread it comes after"`
	// drop, reopen.
	Reason string `json:"reason,omitempty" jsonschema:"drop: why; reopen: why, optional"`
	// set.
	Set *thread.SetIn `json:"set,omitempty" jsonschema:"set: the chord (doc) and its title, description, tags, aliases, or priority"`
}

// ChordOut is the chord tool's output: the chords, the load of one, or the chord a write
// acted on with what the call wrote.
type ChordOut struct {
	Chords         []thread.ChordView  `json:"chords,omitempty"`
	Chord          *thread.ChordLoaded `json:"chord,omitempty"`
	View           *thread.ChordView   `json:"view,omitempty"`
	Commit         string              `json:"commit,omitempty"`
	Wrote          []vault.Ref         `json:"wrote,omitempty"`
	Events         []vault.Ref         `json:"events,omitempty"`
	MovedFromViews []vault.Moved       `json:"moved_from_views,omitempty" jsonschema:"notes of the user's found in views/, moved to inbox/; tell the user where each went"`
}

const chordActions = "list, load, create, add, remove, order, drop, reopen, or set"

func (s *Server) chordTool(ctx context.Context, req *mcp.CallToolRequest, in ChordIn) (*mcp.CallToolResult, ChordOut, error) {
	v, err := s.open(in.Vault)
	if err != nil {
		return nil, ChordOut{}, err
	}
	o := thread.Opts{Now: s.opts.Now(), By: thread.ByAgent}
	out := func(r *thread.Result, err error) (*mcp.CallToolResult, ChordOut, error) {
		if err != nil {
			return nil, ChordOut{}, err
		}
		return nil, ChordOut{View: r.Chord, Commit: r.Commit, Wrote: r.Wrote, Events: r.Events, MovedFromViews: s.views(v)}, nil
	}
	switch in.Action {
	case "", "list":
		idx, err := vault.Load(v)
		if err != nil {
			return nil, ChordOut{}, err
		}
		chords := thread.Load(idx).BoardView(thread.Filter{Tags: in.Tags}).Chords
		return nil, ChordOut{Chords: chords}, nil
	case "load":
		idx, err := vault.Load(v)
		if err != nil {
			return nil, ChordOut{}, err
		}
		loaded, err := thread.LoadChord(idx, in.Chord)
		if err != nil {
			return nil, ChordOut{}, err
		}
		return nil, ChordOut{Chord: loaded}, nil
	case "create":
		return out(thread.ChordCreate(v, thread.ChordIn{Title: in.Title, Text: in.Text, Description: in.Description, Tags: in.Tags, Priority: in.Priority, Threads: in.Threads, NewTags: in.NewTags}, o))
	case "add":
		return out(thread.ChordAdd(v, in.Chord, in.Thread, in.After, o))
	case "remove":
		return out(thread.ChordRemove(v, in.Chord, in.Thread, o))
	case "order":
		return out(thread.ChordOrder(v, in.Chord, in.Order, o))
	case "drop":
		return out(thread.Drop(v, in.Chord, in.Reason, o))
	case "reopen":
		return out(thread.Reopen(v, in.Chord, in.Reason, o))
	case "set":
		if in.Set == nil {
			return nil, ChordOut{}, errors.New("set takes set: {doc, and the fields to change}")
		}
		return out(thread.Set(v, *in.Set, o))
	}
	return nil, ChordOut{}, fmt.Errorf("chord takes action %s, not %q", chordActions, in.Action)
}

// LintIn is the lint tool's input.
type LintIn struct {
	Vault string   `json:"vault,omitempty" jsonschema:"the vault, by path or by name from ~/.atlas/config.json; the session's vault when empty"`
	Tags  []string `json:"tags,omitempty" jsonschema:"check only the documents that hold every one of these tags"`
}

func (s *Server) lintTool(ctx context.Context, req *mcp.CallToolRequest, in LintIn) (*mcp.CallToolResult, *lint.Findings, error) {
	idx, err := s.index(in.Vault)
	if err != nil {
		return nil, nil, err
	}
	f, err := lint.Run(idx, lint.Options{Tags: in.Tags, Now: s.opts.Now()})
	return nil, f, err
}

func readOnly() *mcp.ToolAnnotations { return &mcp.ToolAnnotations{ReadOnlyHint: true} }

// MCP builds the protocol server with every tool.
func (s *Server) MCP() *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: Name, Title: "Atlas", Version: s.opts.Version}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "vault",
		Description: "The state of the vault in one read (status: documents by type, tags with counts, open threads, live sessions, inbox, pending documents, proposed and recent changes, recent events, @atlas mentions, problems); init makes a vault; sync rewrites the derived fields that are out of date and writes the views; mention closes an @atlas mention with a link to its answer."}, safe("vault", s.vaultTool))
	mcp.AddTool(server, &mcp.Tool{Name: "search", Annotations: readOnly(),
		Description: "Ranked search (BM25 over title, aliases, tags, description, body) over the documents of wiki/documents (source, repository, topic, stub, spec, tasks, verification, chord, event). Filter by types, kinds, tags (a document must hold every one), status, and repository. Returns Doc Refs with snippets, and facets: the counts of the tags, types, and statuses of every match, to narrow a broad query."}, safe("search", s.searchTool))
	mcp.AddTool(server, &mcp.Tool{Name: "context", Annotations: readOnly(),
		Description: "Everything an agent needs to work in a repository (by id or title, or a path inside it) or under tags: the tag pages from the top down with their Context, the repositories, the policies that apply (most specific first), the open threads, and for a repository its AGENTS.md and CLAUDE.md and git facts now (branch, head, dirty files, ahead and behind, recent commits, commits past its description)."}, safe("context", s.contextTool))
	mcp.AddTool(server, &mcp.Tool{Name: "match", Annotations: readOnly(),
		Description: "Join the subjects of Item Maps across chunks and match each against the topics and sources: hit (a document holds its name or alias), near (neighbors above the threshold), or new. With docs, or one tag and across, compare topics with topics under a different child tag of that tag, for wiki-map."}, safe("match", s.matchTool))
	mcp.AddTool(server, &mcp.Tool{Name: "source",
		Description: "capture brings inbox files, pasted text, or a snapshot of a linked repository into wiki/documents as sources, with the originals in wiki/assets, as one commit (a source is pending until a change absorbs it; resolves closes the stub that asked for it); chunks splits any document for reading; read returns one chunk as a Text Blob (a PDF chunk names the file and pages to Read)."}, safe("source", s.sourceTool))
	mcp.AddTool(server, &mcp.Tool{Name: "change",
		Description: "The only way knowledge changes: sources, repositories, and topics. propose validates a Wiki Change Plan (create, modify, promote a stub to a topic, rename, remove, confirm, retag) and writes a change document (no commit) for the user to review; apply reads it again and makes one commit, only after the user's yes; reject records why; undo restores the change's paths; show previews one."}, safe("change", s.changeTool))
	mcp.AddTool(server, &mcp.Tool{Name: "thread",
		Description: "Every write to a thread, and the reads. A thread is one piece of work: a stub (its front page), a spec (numbered requirements), task lists (check boxes), and verifications (a result per requirement, and findings). Code derives its status (stub, specified, planned, started, unverified, verified, closed); no call sets it. list is the board; load returns everything needed to take a thread up, with its next step and the skill that does it. stub plants an idea in the user's words; spec writes or revises the spec; tasks writes or appends a task list for one repository; start binds this session to the thread (an edit in a repository needs a started thread with an open task for it); check marks one task done with its commits, or drops it; verify files one round from the verifier's report; finding gives a finding its outcome (task, spec, stub, knowledge, accepted); drop, reopen, block, unblock, resolve, note, and set do what they say. Each write is one commit. A thread closes with no call: when it is verified and the user applies the change that absorbs its spec and verification."}, safe("thread", s.threadTool))
	mcp.AddTool(server, &mcp.Tool{Name: "chord",
		Description: "A chord is a goal that needs several threads, with the order between them: each thread names the threads it comes after, and code refuses a loop. list shows the open chords with their threads in order; load returns one chord with its ready threads and its next step; create makes a chord and its stubs in one commit; add and remove move a thread in or out; order sets what each thread comes after; drop drops the chord and its open threads; reopen and set do what they say. A thread is ready when every thread it comes after is verified. Code writes a canvas per chord in chords/."}, safe("chord", s.chordTool))
	mcp.AddTool(server, &mcp.Tool{Name: "lint", Annotations: readOnly(),
		Description: "The health check over every typed document, optionally the documents under tags: schema, duplicate titles, dead links, tag pages, repository paths, thread, task, and event rules, misplaced and untyped files; orphans, uncited and stale topics; near-duplicate tags, repositories behind, old pending documents and proposals, lost sessions. Each finding names its fix. Reads only."}, safe("lint", s.lintTool))
	return server
}

// Serve runs the server over stdio until the client goes.
func (s *Server) Serve(ctx context.Context) error {
	err := s.MCP().Run(ctx, &mcp.StdioTransport{})
	if errors.Is(err, context.Canceled) || (err != nil && strings.Contains(err.Error(), "EOF")) {
		return nil
	}
	return err
}

// safe turns a panic in a tool handler into the call's error, so one bad call does not end
// the server for the rest of the session.
func safe[In, Out any](tool string, h mcp.ToolHandlerFor[In, Out]) mcp.ToolHandlerFor[In, Out] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in In) (res *mcp.CallToolResult, out Out, err error) {
		defer func() {
			if p := recover(); p != nil {
				var zero Out
				res, out, err = nil, zero, fmt.Errorf("%s failed inside atlas-obsidian: %v; the server keeps running, and this is a bug to report", tool, p)
			}
		}()
		return h(ctx, req, in)
	}
}
