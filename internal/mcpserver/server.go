// Package mcpserver serves the eight tools, each a thin layer over one package. A tool
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
	"github.com/nathanaday/atlas-obsidian/internal/vault"
	"github.com/nathanaday/atlas-obsidian/internal/work"
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
	return []string{"vault", "search", "context", "match", "source", "change", "work", "lint"}
}

// open resolves the vault a call acts on.
func (s *Server) open(name string) (*vault.Vault, error) {
	return vault.Resolve(name, s.opts.Dir, vault.HomeFrom(s.opts.Getenv))
}

// index loads the vault a call acts on.
func (s *Server) index(name string) (*vault.Index, error) {
	v, err := s.open(name)
	if err != nil {
		return nil, err
	}
	return vault.Load(v)
}

// views writes the views after a write. A view that fails to write fails no call: the
// next sync writes it.
func (s *Server) views(v *vault.Vault) { core.Views(v, s.opts.Now()) }

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
	Status  *core.Status  `json:"status,omitempty"`
	Synced  *core.Synced  `json:"synced,omitempty"`
	Mention *core.Mention `json:"mention,omitempty"`
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
		s.views(v)
		return nil, VaultOut{Mention: m}, nil
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
	Captured []source.Captured `json:"captured,omitempty"`
	Events   []vault.Ref       `json:"events,omitempty"`
	Commit   string            `json:"commit,omitempty"`
	Chunks   []source.Chunk    `json:"chunks,omitempty"`
	Blob     *source.TextBlob  `json:"blob,omitempty"`
}

func (s *Server) sourceTool(ctx context.Context, req *mcp.CallToolRequest, in SourceIn) (*mcp.CallToolResult, SourceOut, error) {
	switch in.Action {
	case "capture":
		v, err := s.open(in.Vault)
		if err != nil {
			return nil, SourceOut{}, err
		}
		res, err := source.Capture(v, in.Request, work.Opts{Now: s.opts.Now(), By: work.ByAgent})
		if err != nil {
			return nil, SourceOut{}, err
		}
		s.views(v)
		return nil, SourceOut{Captured: res.Captured, Events: res.Events, Commit: res.Commit}, nil
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
	Work       string         `json:"work,omitempty" jsonschema:"propose: the stub or spec the change serves, if any"`
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
			s.views(v)
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

// WorkIn is the work tool's input.
type WorkIn struct {
	Action string `json:"action,omitempty" jsonschema:"list (the default), show, stub, spec, promote, start, done, drop, reopen, block, unblock, resolve, note, or set"`
	Vault  string `json:"vault,omitempty" jsonschema:"the vault, by path or by name from ~/.atlas/config.json; the session's vault when empty"`
	// The document an action acts on.
	Doc  string `json:"doc,omitempty" jsonschema:"show, drop, reopen, note: the stub or spec (a note: any document), by id or title"`
	Spec string `json:"spec,omitempty" jsonschema:"start, done, block, unblock: the plan, by id or title"`
	Stub string `json:"stub,omitempty" jsonschema:"promote, resolve: the open stub, by id or title"`
	// stub and promote.
	Text        string   `json:"text,omitempty" jsonschema:"stub: the user's words, as given; promote: the spec's sections; note: the note"`
	Title       string   `json:"title,omitempty" jsonschema:"stub, promote: a short title"`
	Description string   `json:"description,omitempty" jsonschema:"stub, promote: one sentence"`
	Tags        []string `json:"tags,omitempty" jsonschema:"stub, promote: the categories; list: only work that holds every one"`
	Priority    string   `json:"priority,omitempty" jsonschema:"stub, promote: high, normal, low, or someday"`
	Inbox       string   `json:"inbox,omitempty" jsonschema:"stub: a note in inbox/ that the stub replaces"`
	NewTags     bool     `json:"new_tags,omitempty" jsonschema:"stub, spec, promote: allow a tag no document holds, in tagging: known, after the user agreed"`
	// spec.
	Specs   []work.SpecIn `json:"specs,omitempty" jsonschema:"spec: the specs to write in one commit (a plan and its parts, or one spec)"`
	Resolve bool          `json:"resolve,omitempty" jsonschema:"spec: close the from stub as resolved, with the specs in its became"`
	// promote.
	Kind         string   `json:"kind,omitempty" jsonschema:"promote: plan or design"`
	Repositories []string `json:"repositories,omitempty" jsonschema:"promote: the repositories the work touches"`
	Parent       string   `json:"parent,omitempty" jsonschema:"promote: the plan this is a part of"`
	Repository   string   `json:"repository,omitempty" jsonschema:"list: only plans that name this repository"`
	// start, done, drop, reopen, block, resolve.
	Take   bool           `json:"take,omitempty" jsonschema:"start: take a plan another live session started"`
	Result *work.ResultIn `json:"result,omitempty" jsonschema:"done: the result: delivered and verified, and follow_ups and learned"`
	Reason string         `json:"reason,omitempty" jsonschema:"drop: why; reopen: why, optional; block: what the plan waits on, in one line"`
	Became []string       `json:"became,omitempty" jsonschema:"resolve: the documents the stub became"`
	// set.
	Set *work.SetIn `json:"set,omitempty" jsonschema:"set: the stub or spec (doc) and the fields to change; a field left out stays"`
}

// WorkOut is the work tool's output: the board, or one document with what the call wrote.
type WorkOut struct {
	Board   *work.BoardView `json:"board,omitempty"`
	View    *work.View      `json:"view,omitempty"`
	Commit  string          `json:"commit,omitempty"`
	Wrote   []vault.Ref     `json:"wrote,omitempty"`
	Events  []vault.Ref     `json:"events,omitempty"`
	Started string          `json:"started,omitempty"`
}

func (s *Server) workTool(ctx context.Context, req *mcp.CallToolRequest, in WorkIn) (*mcp.CallToolResult, WorkOut, error) {
	v, err := s.open(in.Vault)
	if err != nil {
		return nil, WorkOut{}, err
	}
	o := work.Opts{Now: s.opts.Now(), By: work.ByAgent}
	out := func(r *work.Result, err error) (*mcp.CallToolResult, WorkOut, error) {
		if err != nil {
			return nil, WorkOut{}, err
		}
		s.views(v)
		return nil, WorkOut{View: r.View, Commit: r.Commit, Wrote: r.Wrote, Events: r.Events, Started: r.Started}, nil
	}
	switch in.Action {
	case "", "list":
		idx, err := vault.Load(v)
		if err != nil {
			return nil, WorkOut{}, err
		}
		return nil, WorkOut{Board: work.Load(idx).BoardView(work.Filter{Tags: in.Tags, Repository: in.Repository})}, nil
	case "show":
		idx, err := vault.Load(v)
		if err != nil {
			return nil, WorkOut{}, err
		}
		view, err := work.Show(idx, in.Doc)
		if err != nil {
			return nil, WorkOut{}, err
		}
		return nil, WorkOut{View: view}, nil
	case "stub":
		return out(work.Stub(v, work.StubIn{Text: in.Text, Title: in.Title, Description: in.Description, Tags: in.Tags, Priority: in.Priority, Inbox: in.Inbox, NewTags: in.NewTags}, o))
	case "spec":
		return out(work.Specs(v, work.SpecsIn{Specs: in.Specs, Resolve: in.Resolve, NewTags: in.NewTags}, o))
	case "promote":
		return out(work.Promote(v, work.PromoteIn{Stub: in.Stub, Kind: in.Kind, Text: in.Text, Title: in.Title, Description: in.Description, Tags: in.Tags, Repositories: in.Repositories, Parent: in.Parent, Priority: in.Priority, NewTags: in.NewTags}, o))
	case "start":
		return out(work.Start(v, in.Spec, in.Take, o))
	case "done":
		var r work.ResultIn
		if in.Result != nil {
			r = *in.Result
		}
		return out(work.Complete(v, in.Spec, r, o))
	case "drop":
		return out(work.Drop(v, in.Doc, in.Reason, o))
	case "reopen":
		return out(work.Reopen(v, in.Doc, in.Reason, o))
	case "block":
		return out(work.Block(v, in.Spec, in.Reason, o))
	case "unblock":
		return out(work.Unblock(v, in.Spec, o))
	case "resolve":
		return out(work.Resolve(v, in.Stub, in.Became, o))
	case "note":
		return out(work.Note(v, in.Doc, in.Text, o))
	case "set":
		if in.Set == nil {
			return nil, WorkOut{}, errors.New("set takes set: {doc, and the fields to change}")
		}
		return out(work.Set(v, *in.Set, o))
	}
	return nil, WorkOut{}, fmt.Errorf("work takes action list, show, stub, spec, promote, start, done, drop, reopen, block, unblock, resolve, note, or set, not %q", in.Action)
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
		Description: "The state of the vault in one read (status: documents by type, tags with counts, open work, live sessions, inbox, pending documents, proposed and recent changes, recent events, @atlas mentions, problems); init makes a vault; sync heals derived fields and writes the views; mention closes an @atlas mention with a link to its answer."}, s.vaultTool)
	mcp.AddTool(server, &mcp.Tool{Name: "search", Annotations: readOnly(),
		Description: "Ranked search (BM25 over title, aliases, tags, description, body) over the documents of wiki/documents (source, repository, topic, stub, spec, event). Filter by types, kinds, tags (a document must hold every one), status, and repository. Returns Doc Refs with snippets, and facets: the counts of the tags, types, and statuses of every match, to narrow a broad query."}, s.searchTool)
	mcp.AddTool(server, &mcp.Tool{Name: "context", Annotations: readOnly(),
		Description: "Everything an agent needs to work in a repository (by id or title, or a path inside it) or under tags: the tag pages from the top down with their Context, the repositories, the policies that apply (most specific first), the open work, and for a repository its AGENTS.md and CLAUDE.md and git facts now (branch, head, dirty files, ahead and behind, recent commits, commits past its description)."}, s.contextTool)
	mcp.AddTool(server, &mcp.Tool{Name: "match", Annotations: readOnly(),
		Description: "Join the subjects of Item Maps across chunks and match each against the topics and sources: hit (a document holds its name or alias), near (neighbors above the threshold), or new. With docs, or one tag and across, compare topics with topics under a different child tag of that tag, for wiki-map."}, s.matchTool)
	mcp.AddTool(server, &mcp.Tool{Name: "source",
		Description: "capture brings inbox files, pasted text, or a snapshot of a linked repository into wiki/documents as sources, with the originals in wiki/assets, as one commit (a source is pending until a change absorbs it; resolves closes the stub that asked for it); chunks splits any document for reading; read returns one chunk as a Text Blob (a PDF chunk names the file and pages to Read)."}, s.sourceTool)
	mcp.AddTool(server, &mcp.Tool{Name: "change",
		Description: "The only way knowledge changes: sources, repositories, and topics. propose validates a Wiki Change Plan (create, modify, promote a stub to a topic, rename, remove, confirm, retag) and writes a change document (no commit) for the user to review; apply reads it again and makes one commit, only after the user's yes; reject records why; undo restores the change's paths; show previews one."}, s.changeTool)
	mcp.AddTool(server, &mcp.Tool{Name: "work",
		Description: "Every write to stubs, specs, and events, and the reads of work. list is the board; show is a stub or spec with its parts, events, and next step. stub plants an idea in the user's words; spec writes a plan and its parts, or a design; promote makes a stub a spec in place; start binds this session to a plan with no parts (an edit in a repository needs one); done completes it with the result; drop, reopen, block, unblock, resolve, note, and set do what they say. Each write is one commit, and writes its events."}, s.workTool)
	mcp.AddTool(server, &mcp.Tool{Name: "lint", Annotations: readOnly(),
		Description: "The health check over every typed document, optionally the documents under tags: schema, duplicate titles, dead links, tag pages, repository paths, spec and event rules, misplaced and untyped files; orphans, uncited and stale topics; near-duplicate tags, repositories behind, old pending documents and proposals, lost sessions. Each finding names its fix. Reads only."}, s.lintTool)
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
