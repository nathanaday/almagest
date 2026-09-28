// Package mcpserver serves the eight tools, each a thin layer over one package. A tool
// takes ids or titles and returns entities; every rule is the package's, and the
// refusals come from there.
package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/change"
	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/core"
	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/lint"
	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/match"
	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/scope"
	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/search"
	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/source"
	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/threads"
	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/vault"
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
	return []string{"vault", "search", "context", "match", "source", "change", "thread", "lint"}
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

const vaultArg = "the vault, by path or by name from ~/.atlas/config.json; the session's vault when empty"

// VaultIn is the vault tool's input.
type VaultIn struct {
	Action      string `json:"action,omitempty" jsonschema:"status (the default), init, sync, or mention"`
	Vault       string `json:"vault,omitempty" jsonschema:"the vault, by path or by name from ~/.atlas/config.json; the session's vault when empty"`
	Name        string `json:"name,omitempty" jsonschema:"init: the vault's name"`
	Path        string `json:"path,omitempty" jsonschema:"init: the folder that becomes the vault"`
	Areas       string `json:"areas,omitempty" jsonschema:"init: many, few, or manual (the default): how readily areas are proposed"`
	Description string `json:"description,omitempty" jsonschema:"init: one or two sentences on what the vault is for; the body of Atlas.md"`
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
		st, err := core.Init(vault.InitOptions{Path: path, Name: in.Name, Description: in.Description, Areas: in.Areas}, vault.HomeFrom(s.opts.Getenv), now)
		if err != nil {
			return nil, VaultOut{}, err
		}
		return nil, VaultOut{Status: st}, nil
	case "sync":
		v, err := s.open(in.Vault)
		if err != nil {
			return nil, VaultOut{}, err
		}
		synced, err := core.Sync(v, now)
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
	Scope string `json:"scope,omitempty" jsonschema:"an area or repository by id or title; the vault when empty"`
	Path  string `json:"path,omitempty" jsonschema:"a path inside a linked repository, in place of scope"`
}

func (s *Server) contextTool(ctx context.Context, req *mcp.CallToolRequest, in ContextIn) (*mcp.CallToolResult, *scope.Chain, error) {
	idx, err := s.index(in.Vault)
	if err != nil {
		return nil, nil, err
	}
	c, err := scope.Context(idx, in.Scope, in.Path)
	return nil, c, err
}

// MatchIn is the match tool's input.
type MatchIn struct {
	Vault    string          `json:"vault,omitempty" jsonschema:"the vault, by path or by name from ~/.atlas/config.json; the session's vault when empty"`
	Items    []match.ItemMap `json:"items,omitempty" jsonschema:"the Item Maps the extract workers returned, every one at once"`
	Pages    []string        `json:"pages,omitempty" jsonschema:"page ids to compare, in place of items (wiki-rollup)"`
	Siblings bool            `json:"siblings,omitempty" jsonschema:"with pages: compare each page only with the pages of its sibling scopes"`
	Within   string          `json:"within,omitempty" jsonschema:"limit the candidate pages to this scope and the scopes below it"`
}

func (s *Server) matchTool(ctx context.Context, req *mcp.CallToolRequest, in MatchIn) (*mcp.CallToolResult, *match.Map, error) {
	idx, err := s.index(in.Vault)
	if err != nil {
		return nil, nil, err
	}
	m, err := match.Run(idx, match.Input{Items: in.Items, Pages: in.Pages, Siblings: in.Siblings, Within: in.Within})
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
		res, err := source.Capture(v, in.Request, s.opts.Now())
		if err != nil {
			return nil, SourceOut{}, err
		}
		return nil, SourceOut{Captured: res.Captured, Commit: res.Commit}, nil
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
	Thread     string         `json:"thread,omitempty" jsonschema:"propose: the thread the change serves, if any"`
	Supersedes string         `json:"supersedes,omitempty" jsonschema:"propose: a proposed change this one replaces"`
	Writes     []change.Write `json:"writes,omitempty" jsonschema:"propose: the writes: create (type, title, fields, body), modify (id, fields, body, base), rename (id, title), remove (id, redirect)"`
}

func (s *Server) changeTool(ctx context.Context, req *mcp.CallToolRequest, in ChangeIn) (*mcp.CallToolResult, *change.Preview, error) {
	v, err := s.open(in.Vault)
	if err != nil {
		return nil, nil, err
	}
	now := s.opts.Now()
	switch in.Action {
	case "", "show":
		idx, err := vault.Load(v)
		if err != nil {
			return nil, nil, err
		}
		pv, err := change.Show(idx, in.ID)
		return nil, pv, err
	case "propose":
		pv, err := change.Propose(v, change.Plan{Title: in.Title, Notes: in.Notes, Absorbs: in.Absorbs, Thread: in.Thread, Supersedes: in.Supersedes, Writes: in.Writes}, now)
		return nil, pv, err
	case "apply":
		pv, err := change.Apply(v, in.ID, now)
		return nil, pv, err
	case "reject":
		pv, err := change.Reject(v, in.ID, in.Reason, now)
		return nil, pv, err
	case "undo":
		pv, err := change.Undo(v, in.ID, now)
		return nil, pv, err
	}
	return nil, nil, fmt.Errorf("change takes action show, propose, apply, reject, or undo, not %q", in.Action)
}

// ThreadIn is the thread tool's input.
type ThreadIn struct {
	Action     string           `json:"action,omitempty" jsonschema:"list (the default), show, open, attach, file, tasks, task, set, or reopen"`
	Vault      string           `json:"vault,omitempty" jsonschema:"the vault, by path or by name from ~/.atlas/config.json; the session's vault when empty"`
	Thread     string           `json:"thread,omitempty" jsonschema:"the thread: its id or title, or any of its documents"`
	Text       string           `json:"text,omitempty" jsonschema:"open: the user's words; file: the document's sections"`
	Title      *string          `json:"title,omitempty" jsonschema:"open: a short name for the work; set, task set: the new title"`
	Scope      []string         `json:"scope,omitempty" jsonschema:"open, set: the repositories and areas the work touches"`
	Priority   string           `json:"priority,omitempty" jsonschema:"open, set: high, normal, low, or someday"`
	Inbox      string           `json:"inbox,omitempty" jsonschema:"open: a note in inbox/ that the stub replaces"`
	Blocked    *string          `json:"blocked,omitempty" jsonschema:"set, task set: what the work waits on, in one line; empty unblocks"`
	Part       string           `json:"part,omitempty" jsonschema:"file: spec or receipt"`
	Outcome    string           `json:"outcome,omitempty" jsonschema:"file receipt: completed or killed"`
	Tasks      []threads.TaskIn `json:"tasks,omitempty" jsonschema:"tasks: the new tasks"`
	Task       string           `json:"task,omitempty" jsonschema:"task: the task, by id, title, or T<n> with thread"`
	Do         string           `json:"do,omitempty" jsonschema:"task: start, done, drop, reopen, or set"`
	Result     string           `json:"result,omitempty" jsonschema:"task done: what changed, the commits, how it was verified"`
	Take       bool             `json:"take,omitempty" jsonschema:"task start: take a task another live session holds"`
	Repository *string          `json:"repository,omitempty" jsonschema:"task set: the task's repository"`
	Depends    *[]string        `json:"depends,omitempty" jsonschema:"task set: the tasks it waits for"`
	Order      *int             `json:"order,omitempty" jsonschema:"task set: the task's number"`
}

// ThreadOut is the thread tool's output: the board, or one thread.
type ThreadOut struct {
	Board  *threads.BoardView `json:"board,omitempty"`
	View   *threads.View      `json:"view,omitempty"`
	Commit string             `json:"commit,omitempty"`
}

func (s *Server) threadTool(ctx context.Context, req *mcp.CallToolRequest, in ThreadIn) (*mcp.CallToolResult, ThreadOut, error) {
	v, err := s.open(in.Vault)
	if err != nil {
		return nil, ThreadOut{}, err
	}
	now := s.opts.Now()
	out := func(r *threads.Result, err error) (*mcp.CallToolResult, ThreadOut, error) {
		if err != nil {
			return nil, ThreadOut{}, err
		}
		return nil, ThreadOut{View: r.View, Commit: r.Commit}, nil
	}
	str := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}
	switch in.Action {
	case "", "list":
		idx, err := vault.Load(v)
		if err != nil {
			return nil, ThreadOut{}, err
		}
		return nil, ThreadOut{Board: threads.List(idx)}, nil
	case "show", "attach":
		idx, err := vault.Load(v)
		if err != nil {
			return nil, ThreadOut{}, err
		}
		view, err := threads.Show(idx, in.Thread)
		if in.Action == "attach" {
			view, err = threads.Attach(idx, in.Thread)
		}
		if err != nil {
			return nil, ThreadOut{}, err
		}
		return nil, ThreadOut{View: view}, nil
	case "open":
		return out(threads.Open(v, threads.OpenIn{Text: in.Text, Title: str(in.Title), Scope: in.Scope, Priority: in.Priority, Inbox: in.Inbox}, now))
	case "file":
		return out(threads.File(v, in.Thread, in.Part, in.Text, in.Outcome, now))
	case "tasks":
		return out(threads.Tasks(v, in.Thread, in.Tasks, now))
	case "task":
		return out(threads.Task(v, in.Task, in.Thread, threads.TaskDo{Do: in.Do, Result: in.Result, Take: in.Take, Title: in.Title, Repository: in.Repository, Depends: in.Depends, Order: in.Order, Blocked: in.Blocked}, now))
	case "set":
		set := threads.SetIn{Title: in.Title, Blocked: in.Blocked}
		if in.Priority != "" {
			set.Priority = &in.Priority
		}
		if in.Scope != nil {
			set.Scope = &in.Scope
		}
		return out(threads.Set(v, in.Thread, set, now))
	case "reopen":
		return out(threads.Reopen(v, in.Thread, now))
	}
	return nil, ThreadOut{}, fmt.Errorf("thread takes action list, show, open, attach, file, tasks, task, set, or reopen, not %q", in.Action)
}

// LintIn is the lint tool's input.
type LintIn struct {
	Vault string `json:"vault,omitempty" jsonschema:"the vault, by path or by name from ~/.atlas/config.json; the session's vault when empty"`
	Scope string `json:"scope,omitempty" jsonschema:"an area or repository: check it and the scopes below it"`
}

func (s *Server) lintTool(ctx context.Context, req *mcp.CallToolRequest, in LintIn) (*mcp.CallToolResult, *lint.Findings, error) {
	idx, err := s.index(in.Vault)
	if err != nil {
		return nil, nil, err
	}
	f, err := lint.Run(idx, lint.Options{Scope: in.Scope, Now: s.opts.Now()})
	return nil, f, err
}

func readOnly() *mcp.ToolAnnotations { return &mcp.ToolAnnotations{ReadOnlyHint: true} }

// MCP builds the protocol server with every tool.
func (s *Server) MCP() *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: Name, Title: "Atlas", Version: s.opts.Version}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "vault",
		Description: "The state of the vault in one read (status: scopes, wiki counts, open threads, live sessions, inbox, pending documents, proposed and recent changes, @atlas mentions, problems); init makes a vault; sync heals derived fields; mention closes an @atlas mention with a link to its answer."}, s.vaultTool)
	mcp.AddTool(server, &mcp.Tool{Name: "search", Annotations: readOnly(),
		Description: "Ranked search (BM25 over title, aliases, description, body) over every typed document. Filter by types, a scope and the scopes below it, and state (e.g. types [stub] with state {stage: [stub, spec, tasks]} finds open threads). Returns Doc Refs with snippets."}, s.searchTool)
	mcp.AddTool(server, &mcp.Tool{Name: "context", Annotations: readOnly(),
		Description: "Walk the context graph to a scope (area or repository, or a path inside a linked repository; the vault when empty): the chain from the vault down with each body, the children, the repositories below, the policies on the chain nearest first, the open threads, and for a repository its AGENTS.md and CLAUDE.md and git facts (head, dirty, behind its page)."}, s.contextTool)
	mcp.AddTool(server, &mcp.Tool{Name: "match", Annotations: readOnly(),
		Description: "Join the subjects of Item Maps across chunks and match each against the wiki: hit (a page holds its name or alias), near (neighbors above the threshold), or new. With pages and siblings, compare pages with the pages of sibling scopes, for a rollup."}, s.matchTool)
	mcp.AddTool(server, &mcp.Tool{Name: "source",
		Description: "capture brings inbox files, pasted text, or a snapshot of a linked repository into wiki/sources/ as one commit (the source is pending until a change absorbs it); chunks splits any document for reading; read returns one chunk as a Text Blob (a PDF chunk names the file and pages to Read)."}, s.sourceTool)
	mcp.AddTool(server, &mcp.Tool{Name: "change",
		Description: "The only way the wiki changes. propose validates a Wiki Change Plan and writes a change document (no commit) for the user to review; apply reads it again and makes one commit, only after the user's yes; reject records why; undo restores the change's paths; show previews one."}, s.changeTool)
	mcp.AddTool(server, &mcp.Tool{Name: "thread",
		Description: "Every write to a thread, and the reads. list is the board; show is one thread with its next step. open makes a stub in the user's words; attach binds this session to a thread; file writes the spec or the receipt; tasks adds tasks; task starts, finishes, drops, reopens, or sets one; set changes priority, blocked, scope, or title; reopen takes up a closed thread. The stage is the furthest document; each write is one commit."}, s.threadTool)
	mcp.AddTool(server, &mcp.Tool{Name: "lint", Annotations: readOnly(),
		Description: "The health check over every typed document, optionally one scope: schema, duplicate titles, dead links, scope loops, repository paths, thread rules; orphans and uncited pages; repositories behind, stale pending documents and proposals, lost sessions. Each finding names its fix. Reads only."}, s.lintTool)
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
