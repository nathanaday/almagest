// Package core is the vault tool's backend: the state of the vault in one read, the sync
// that heals every derived part, and the mentions that ask the agent for work.
package core

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/change"
	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/doc"
	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/lint"
	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/sessions"
	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/threads"
	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/vault"
)

// VaultInfo names a vault.
type VaultInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}

// Status is the state of the vault in one read.
type Status struct {
	Vault    VaultInfo         `json:"vault"`
	Scopes   ScopeCounts       `json:"scopes"`
	Wiki     WikiCounts        `json:"wiki"`
	Threads  ThreadCounts      `json:"threads"`
	Sessions SessionLists      `json:"sessions"`
	Inbox    []InboxItem       `json:"inbox"`
	Pending  []vault.Ref       `json:"pending"`
	Changes  ChangeLists       `json:"changes"`
	Mentions []Mention         `json:"mentions"`
	Problems int               `json:"problems"`
	Versions map[string]string `json:"versions,omitempty"`
}

// ScopeCounts count the scope pages.
type ScopeCounts struct {
	Areas        int `json:"areas"`
	Repositories int `json:"repositories"`
}

// WikiCounts count the knowledge pages.
type WikiCounts struct {
	Pages     map[string]int `json:"pages"`
	Draft     int            `json:"draft"`
	Contested int            `json:"contested"`
}

// ThreadCounts count the open threads by stage, and list the active ones.
type ThreadCounts struct {
	Open   map[string]int `json:"open"`
	Active []vault.Ref    `json:"active"`
	// List is every open thread in board order, for the opening context.
	List []vault.Ref `json:"list,omitempty"`
}

// SessionLists are the live sessions.
type SessionLists struct {
	Running []vault.Ref `json:"running"`
	Waiting []vault.Ref `json:"waiting"`
	Idle    []vault.Ref `json:"idle"`
}

// ChangeLists are the changes that wait, and the last applied.
type ChangeLists struct {
	Proposed []vault.Ref `json:"proposed"`
	Recent   []vault.Ref `json:"recent"`
}

// InboxItem is one file in the inbox.
type InboxItem struct {
	Name string `json:"name"`
	Size string `json:"size"`
	Kind string `json:"kind"`
}

// Mention is an open task line addressed to the agent.
type Mention struct {
	Doc  vault.Ref `json:"doc"`
	Line int       `json:"line"`
	Text string    `json:"text"`
}

// Recent is how many applied changes the status lists.
const Recent = 5

// StatusOf reads the state of the vault.
func StatusOf(idx *vault.Index, now time.Time) *Status {
	v := idx.V
	st := &Status{
		Vault:    VaultInfo{ID: v.ID(), Name: v.Name(), Path: vault.Shorten(v.Root)},
		Wiki:     WikiCounts{Pages: map[string]int{}},
		Threads:  ThreadCounts{Open: map[string]int{"stub": 0, "spec": 0, "tasks": 0}, Active: []vault.Ref{}},
		Sessions: SessionLists{Running: []vault.Ref{}, Waiting: []vault.Ref{}, Idle: []vault.Ref{}},
		Inbox:    Inbox(v),
		Pending:  idx.Refs(idx.PendingDocs()),
		Changes:  ChangeLists{Proposed: []vault.Ref{}, Recent: []vault.Ref{}},
		Mentions: Mentions(idx),
	}
	for _, d := range idx.Docs {
		switch d.Type() {
		case "area":
			st.Scopes.Areas++
		case "repository":
			st.Scopes.Repositories++
		case "concept", "entity", "policy", "source":
			st.Wiki.Pages[d.Type()]++
			switch d.Str("status") {
			case "draft":
				st.Wiki.Draft++
			case "contested":
				st.Wiki.Contested++
			}
		case "session":
			ref := idx.Ref(d)
			switch d.Str("status") {
			case sessions.Running:
				st.Sessions.Running = append(st.Sessions.Running, ref)
			case sessions.Waiting:
				st.Sessions.Waiting = append(st.Sessions.Waiting, ref)
			case sessions.Idle:
				st.Sessions.Idle = append(st.Sessions.Idle, ref)
			}
		case "change":
			if d.Str("status") == change.Proposed {
				st.Changes.Proposed = append(st.Changes.Proposed, idx.Ref(d))
			}
		}
	}
	b := threads.Load(idx)
	for _, t := range b.Open() {
		st.Threads.Open[t.Stage()]++
		ref := idx.Ref(t.Stub)
		st.Threads.List = append(st.Threads.List, ref)
		if t.Stub.Front.Bool("active") {
			st.Threads.Active = append(st.Threads.Active, ref)
		}
	}
	applied := idx.Of("change")
	sort.SliceStable(applied, func(i, j int) bool { return applied[i].Str("applied") > applied[j].Str("applied") })
	for _, c := range applied {
		if c.Str("status") != change.Applied {
			continue
		}
		if len(st.Changes.Recent) == Recent {
			break
		}
		st.Changes.Recent = append(st.Changes.Recent, idx.Ref(c))
	}
	if f, err := lint.Run(idx, lint.Options{Quick: true, Now: now}); err == nil {
		st.Problems = f.Counts[lint.Error]
	}
	return st
}

// Inbox lists what waits in inbox/.
func Inbox(v *vault.Vault) []InboxItem {
	out := []InboxItem{}
	root := v.Abs(vault.Inbox)
	filepath.WalkDir(root, func(abs string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			return nil
		}
		info, err := e.Info()
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, abs)
		out = append(out, InboxItem{Name: filepath.ToSlash(rel), Size: humanSize(info.Size()), Kind: Kind(e.Name())})
		return nil
	})
	return out
}

// Kind is what a file is, by its extension.
func Kind(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".pdf":
		return "pdf"
	case ".md", ".markdown":
		return "markdown"
	case ".txt", ".text", ".csv", ".tsv", ".json", ".yaml", ".yml", ".html", ".htm", ".xml", ".log":
		return "text"
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg":
		return "image"
	}
	return "other"
}

func humanSize(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}

var (
	openTask  = regexp.MustCompile(`^\s*(?:[-*+]|\d+[.)]) \[ \] (.*)$`)
	atlasWord = regexp.MustCompile(`(?:^|[^\w@.])@atlas(?:[^\w-]|$)`)
	fenceLine = regexp.MustCompile("^\\s*(```|~~~)")
)

// mention reads an open task line that addresses the agent, or nil.
func mention(line string) []string {
	m := openTask.FindStringSubmatch(line)
	if m == nil || !atlasWord.MatchString(m[1]) {
		return nil
	}
	return m
}

// mentionSkip are the folders whose documents hold no mention.
var mentionSkip = []string{vault.Wiki + "/", vault.Changes + "/", vault.Sessions + "/", vault.Scratchpad + "/"}

// Mentions finds every open @atlas task line outside the wiki, the changes, the sessions,
// and the scratchpad.
func Mentions(idx *vault.Index) []Mention {
	out := []Mention{}
	for _, d := range append(append([]*doc.Doc{}, idx.Notes...), idx.Docs...) {
		skip := false
		for _, p := range mentionSkip {
			if strings.HasPrefix(d.Path, p) {
				skip = true
			}
		}
		if skip || !strings.Contains(d.Content, "@atlas") {
			continue
		}
		inFence := false
		for i, line := range strings.Split(d.Content, "\n") {
			if fenceLine.MatchString(line) {
				inFence = !inFence
				continue
			}
			if inFence {
				continue
			}
			if m := mention(line); m != nil {
				ref := idx.Ref(d)
				if d.Front == nil || d.Type() == "" {
					ref = vault.Ref{Title: d.Title(), Path: d.Path, Scope: []string{}}
				}
				out = append(out, Mention{Doc: ref, Line: i + 1, Text: strings.TrimSpace(m[1])})
			}
		}
	}
	return out
}

// CloseMention checks a mention's box and appends a link to what answered it. It is the
// one write code makes into a note the user owns, and it is the answer asked for. It
// commits nothing; the next snapshot keeps it.
func CloseMention(v *vault.Vault, rel string, line int, link string) (*Mention, error) {
	unlock, err := v.Lock()
	if err != nil {
		return nil, err
	}
	defer unlock()
	idx, err := vault.Load(v)
	if err != nil {
		return nil, err
	}
	answer, err := idx.Resolve(link)
	if err != nil {
		return nil, fmt.Errorf("link: %w", err)
	}
	data, err := v.Read(rel)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", rel, err)
	}
	lines := strings.Split(string(data), "\n")
	if line < 1 || line > len(lines) {
		return nil, fmt.Errorf("%s has no line %d", rel, line)
	}
	m := mention(lines[line-1])
	if m == nil {
		return nil, fmt.Errorf("line %d of %s is no open @atlas mention; it may be closed already", line, rel)
	}
	lines[line-1] = strings.Replace(lines[line-1], "[ ]", "[x]", 1) + " → " + doc.Link(vault.Title(answer))
	if err := v.Write(rel, []byte(strings.Join(lines, "\n"))); err != nil {
		return nil, err
	}
	return &Mention{Doc: vault.Ref{Title: vault.NoteTitle(rel), Path: rel, Scope: []string{}}, Line: line, Text: strings.TrimSpace(lines[line-1])}, nil
}

// Synced is what a sync changed.
type Synced struct {
	Threads  []string `json:"threads"`
	Lost     []string `json:"lost"`
	Sessions []string `json:"sessions"`
	Settings bool     `json:"settings"`
}

// Sync heals every derived part of the vault: the recovery of a change left applying,
// the layout's folders, thread stages and callouts, active flags, lost sessions, session
// callouts, and the harness settings. It writes a file only when its derived content
// differs, never changes updated, and makes no commit.
func Sync(v *vault.Vault, now time.Time) (*Synced, error) {
	unlock, err := v.Lock()
	if err != nil {
		return nil, err
	}
	defer unlock()
	return SyncLocked(v, now)
}

// SyncLocked is Sync for a caller that holds the lock.
func SyncLocked(v *vault.Vault, now time.Time) (*Synced, error) {
	out := &Synced{Threads: []string{}, Lost: []string{}, Sessions: []string{}}
	if err := change.Recover(v); err != nil {
		return nil, err
	}
	if err := v.EnsureFolders(); err != nil {
		return nil, err
	}
	lost, err := sessions.MarkLost(v, now, v.StaleHours())
	if err != nil {
		return nil, err
	}
	out.Lost = append(out.Lost, lost...)
	wrote, err := threads.SyncVault(v)
	if err != nil {
		return nil, err
	}
	out.Threads = append(out.Threads, wrote...)
	for _, s := range sessions.All(v) {
		if !sessions.Live(s.Str("status")) {
			continue
		}
		if ok, err := sessions.Refresh(v, s); err == nil && ok {
			out.Sessions = append(out.Sessions, s.Path)
		}
	}
	settings, err := v.SyncSettings(nil)
	if err != nil {
		return out, err
	}
	out.Settings = settings
	return out, nil
}

// Init makes a new vault and returns its status.
func Init(opts vault.InitOptions, h vault.Home, now time.Time) (*Status, error) {
	v, err := vault.Init(opts, h, now)
	if err != nil {
		return nil, err
	}
	idx, err := vault.Load(v)
	if err != nil {
		return nil, err
	}
	return StatusOf(idx, now), nil
}

// ErrUsage is returned for a call that names no action it knows.
var ErrUsage = errors.New("unknown action")
