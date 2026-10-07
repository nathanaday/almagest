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
	"github.com/nathanaday/atlas-obsidian/internal/checkout"
	"github.com/nathanaday/atlas-obsidian/internal/core"
	"github.com/nathanaday/atlas-obsidian/internal/lint"
	"github.com/nathanaday/atlas-obsidian/internal/match"
	"github.com/nathanaday/atlas-obsidian/internal/search"
	"github.com/nathanaday/atlas-obsidian/internal/sessions"
	"github.com/nathanaday/atlas-obsidian/internal/source"
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
	return []string{"vault", "search", "context", "match", "source", "change", "checkout", "lint"}
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

// views writes the views after a write, and returns the notes it moved out of wiki-view/. A
// view that fails to write fails no call: the next sync writes it.
func (s *Server) views(v *vault.Vault) []vault.Moved {
	moved, _ := core.Views(v, s.opts.Now())
	return moved
}

// VaultIn is the vault tool's input.
type VaultIn struct {
	Action      string `json:"action,omitempty" jsonschema:"status (the default), init, or sync"`
	Vault       string `json:"vault,omitempty" jsonschema:"the vault, by path or by name from ~/.atlas/config.json; the session's vault when empty"`
	Name        string `json:"name,omitempty" jsonschema:"init: the vault's name"`
	Path        string `json:"path,omitempty" jsonschema:"init: the folder that becomes the vault"`
	Tagging     string `json:"tagging,omitempty" jsonschema:"init: open (the default; the agent adds tags freely) or known (only tags that exist, unless the user agrees)"`
	Description string `json:"description,omitempty" jsonschema:"init: one or two sentences on what the vault is for; the body of Atlas.md"`
	Views       bool   `json:"views,omitempty" jsonschema:"sync: the statuses, callouts, and views only, with no git"`
}

// VaultOut is the vault tool's output.
type VaultOut struct {
	Status *core.Status `json:"status,omitempty"`
	Synced *core.Synced `json:"synced,omitempty"`
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
	}
	return nil, VaultOut{}, fmt.Errorf("vault takes action status, init, or sync, not %q", in.Action)
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
	Captured          []source.Captured `json:"captured,omitempty"`
	Commit            string            `json:"commit,omitempty"`
	Chunks            []source.Chunk    `json:"chunks,omitempty"`
	Blob              *source.TextBlob  `json:"blob,omitempty"`
	MovedFromWikiView []vault.Moved     `json:"moved_from_wiki_view,omitempty" jsonschema:"notes of the user's found in wiki-view/, moved to ingest/; tell the user where each went"`
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
		return nil, SourceOut{Captured: res.Captured, Commit: res.Commit, MovedFromWikiView: s.views(v)}, nil
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
	Action     string         `json:"action,omitempty" jsonschema:"show (the default), start, progress, propose, apply, reject, or undo"`
	Vault      string         `json:"vault,omitempty" jsonschema:"the vault, by path or by name from ~/.atlas/config.json; the session's vault when empty"`
	ID         string         `json:"id,omitempty" jsonschema:"show, apply, reject, undo: the change (id or title); progress: the work document; propose: the running work document to fill, if the work started with start"`
	Kind       string         `json:"kind,omitempty" jsonschema:"start: ingest or repair"`
	Files      []string       `json:"files,omitempty" jsonschema:"start, kind ingest: the names of the files in ingest/ the work takes"`
	Text       string         `json:"text,omitempty" jsonschema:"progress: the step just done, in one line"`
	Reason     string         `json:"reason,omitempty" jsonschema:"reject: why, in one line"`
	Title      string         `json:"title,omitempty" jsonschema:"propose, start: a short name; the file name and the commit subject"`
	Notes      string         `json:"notes,omitempty" jsonschema:"propose: what the change does and why, and each skipped subject with its reason"`
	Absorbs    []string       `json:"absorbs,omitempty" jsonschema:"propose: ids of the sources the change absorbs into the wiki"`
	Supersedes string         `json:"supersedes,omitempty" jsonschema:"propose: a proposed change this one replaces"`
	NewTags    bool           `json:"new_tags,omitempty" jsonschema:"propose: allow tags no document holds, in tagging: known, after the user agreed"`
	Writes     []change.Write `json:"writes,omitempty" jsonschema:"propose: the writes: create (type topic or repository, kind, title, fields, body), modify (id, fields, body, base), rename (id, title), remove (id, redirect), confirm (id), retag (from, to)"`
}

func (s *Server) changeTool(ctx context.Context, req *mcp.CallToolRequest, in ChangeIn) (*mcp.CallToolResult, *change.Preview, error) {
	v, err := s.open(in.Vault)
	if err != nil {
		return nil, nil, err
	}
	now := s.opts.Now()
	done := func(pv *change.Preview, err error) (*mcp.CallToolResult, *change.Preview, error) {
		if err == nil {
			pv.MovedFromWikiView = s.views(v)
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
		return done(change.Propose(v, change.Plan{ID: in.ID, Title: in.Title, Notes: in.Notes, Absorbs: in.Absorbs, Supersedes: in.Supersedes, NewTags: in.NewTags, Writes: in.Writes}, now))
	case "start":
		return done(change.Start(v, change.StartIn{Title: in.Title, Kind: in.Kind, Files: in.Files}, now))
	case "progress":
		return done(change.Progress(v, in.ID, in.Text, now))
	case "apply":
		return done(change.Apply(v, in.ID, now, sessions.UserAnswered(v)))
	case "reject":
		return done(change.Reject(v, in.ID, in.Reason, now))
	case "undo":
		return done(change.Undo(v, in.ID, now))
	}
	return nil, nil, fmt.Errorf("change takes action show, start, progress, propose, apply, reject, or undo, not %q", in.Action)
}

// CheckoutIn is the checkout tool's input.
type CheckoutIn struct {
	Action string   `json:"action,omitempty" jsonschema:"list (the default), candidates, make, or return"`
	Vault  string   `json:"vault,omitempty" jsonschema:"the vault, by path or by name from ~/.atlas/config.json; the session's vault when empty"`
	Text   string   `json:"text,omitempty" jsonschema:"candidates: the request"`
	Tags   []string `json:"tags,omitempty" jsonschema:"candidates: only documents that hold every one of these tags"`
	Types  []string `json:"types,omitempty" jsonschema:"candidates: the types to rank; topic and repository when empty; add source to include sources"`
	Limit  int      `json:"limit,omitempty" jsonschema:"candidates: at most this many; 40 when 0"`
	checkout.Order
	Folder string `json:"folder,omitempty" jsonschema:"return: the checkout, by its folder's name"`
}

// CheckoutOut is the checkout tool's output.
type CheckoutOut struct {
	Checkouts  []checkout.Entry     `json:"checkouts,omitempty"`
	Candidates []checkout.Candidate `json:"candidates,omitempty"`
	Made       *checkout.Made       `json:"made,omitempty"`
	Returned   *checkout.Returned   `json:"returned,omitempty"`
}

func (s *Server) checkoutTool(ctx context.Context, req *mcp.CallToolRequest, in CheckoutIn) (*mcp.CallToolResult, CheckoutOut, error) {
	v, err := s.open(in.Vault)
	if err != nil {
		return nil, CheckoutOut{}, err
	}
	switch in.Action {
	case "", "list":
		return nil, CheckoutOut{Checkouts: checkout.List(v)}, nil
	case "candidates":
		idx, err := vault.Load(v)
		if err != nil {
			return nil, CheckoutOut{}, err
		}
		c, err := checkout.Candidates(idx, in.Text, in.Tags, in.Types, in.Limit)
		return nil, CheckoutOut{Candidates: c}, err
	case "make":
		m, err := checkout.Make(v, in.Order, s.opts.Now())
		return nil, CheckoutOut{Made: m}, err
	case "return":
		r, err := checkout.Return(v, in.Folder, s.opts.Now())
		if err != nil {
			return nil, CheckoutOut{}, err
		}
		return nil, CheckoutOut{Returned: r}, nil
	}
	return nil, CheckoutOut{}, fmt.Errorf("checkout takes action list, candidates, make, or return, not %q", in.Action)
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
		Description: "The state of the vault in one read (status: documents by type, tags with counts, live sessions, the files in ingest/, pending sources, proposed and recent changes, problems); init makes a vault; sync rewrites the derived fields that are out of date and writes the views."}, safe("vault", s.vaultTool))
	mcp.AddTool(server, &mcp.Tool{Name: "search", Annotations: readOnly(),
		Description: "Ranked search (BM25 over title, aliases, tags, description, body) over the documents of source-core/documents (source, repository, topic). Filter by types, kinds, tags (a document must hold every one), status, and repository. Returns Doc Refs with snippets, and facets: the counts of the tags, types, and statuses of every match, to narrow a broad query."}, safe("search", s.searchTool))
	mcp.AddTool(server, &mcp.Tool{Name: "context", Annotations: readOnly(),
		Description: "Everything an agent needs to work in a repository (by id or title, or a path inside it) or under tags: the tag pages from the top down with their Context, the repositories, the policies that apply (most specific first), and for a repository its AGENTS.md and CLAUDE.md and git facts now (branch, head, dirty files, ahead and behind, recent commits, commits past its description)."}, safe("context", s.contextTool))
	mcp.AddTool(server, &mcp.Tool{Name: "match", Annotations: readOnly(),
		Description: "Join the subjects of Item Maps across chunks and match each against the topics and sources: hit (a document holds its name or alias), near (neighbors above the threshold), or new. With docs, or one tag and across, compare topics with topics under a different child tag of that tag, for wiki-map."}, safe("match", s.matchTool))
	mcp.AddTool(server, &mcp.Tool{Name: "source",
		Description: "capture brings files from ingest/, pasted text, or a snapshot of a linked repository into source-core/documents as sources, with the originals in source-core/originals, as one commit (a source is pending until a change absorbs it); chunks splits any document for reading; read returns one chunk as a Text Blob (a PDF chunk names the file and pages to Read)."}, safe("source", s.sourceTool))
	mcp.AddTool(server, &mcp.Tool{Name: "change",
		Description: "The only way knowledge changes: sources, repositories, and topics. start opens a work document (a running change for an ingest or a repair) that the user watches; progress adds one line of what was done to it; propose validates a Wiki Change Plan (create, modify, rename, remove, confirm, retag) and writes a change document, or fills the running one named by id (no commit), for the user to review; a remove moves the document to trash/; apply reads it again and makes one commit, only after the user's yes; reject records why; undo restores the change's paths; show previews one."}, safe("change", s.changeTool))
	mcp.AddTool(server, &mcp.Tool{Name: "checkout",
		Description: "The librarian's desk. candidates ranks the documents a request may need: the search's best hits and the documents linked to or from them within two steps, each with its score, distance, and the title it was reached from; the librarian chooses. make copies the chosen documents, in reading order with a why each, into a new folder of checkout/ as \"<Title> (checkout)\" notes the user reads and edits, with a reading list and the ledger, in one commit. return proposes the edited copies as one change of their originals (the user decides in the change document). list shows every checkout."}, safe("checkout", s.checkoutTool))
	mcp.AddTool(server, &mcp.Tool{Name: "lint", Annotations: readOnly(),
		Description: "The health check over every typed document, optionally the documents under tags: schema, duplicate titles, dead links, tag pages, repository paths, misplaced, untyped, and archived files; orphans, uncited and stale topics; near-duplicate tags, repositories behind, old pending sources and proposals. Each finding names its fix. Reads only."}, safe("lint", s.lintTool))
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
