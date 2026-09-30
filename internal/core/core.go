// Package core is the vault tool's backend: the state of the vault in one read, the sync
// that heals every derived part and writes the views, and the mentions that ask the
// agent for work.
package core

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/derive"
	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/lint"
	"github.com/nathanaday/atlas-obsidian/internal/schema"
	"github.com/nathanaday/atlas-obsidian/internal/sessions"
	"github.com/nathanaday/atlas-obsidian/internal/source"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
	"github.com/nathanaday/atlas-obsidian/internal/views"
	"github.com/nathanaday/atlas-obsidian/internal/work"
)

// VaultInfo names a vault.
type VaultInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Path    string `json:"path"`
	Layout  int    `json:"layout"`
	Tagging string `json:"tagging"`
}

// TagCount is one tag, its count, and its page.
type TagCount struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
	Page  string `json:"page,omitempty"`
}

// TopicCounts count the topics by kind, and the ones that need care.
type TopicCounts struct {
	Kinds     map[string]int `json:"kinds"`
	Draft     int            `json:"draft"`
	Contested int            `json:"contested"`
}

// WorkCounts count the open work, and list it in board order.
type WorkCounts struct {
	Stubs   int         `json:"stubs"`
	Open    int         `json:"open"`
	Started int         `json:"started"`
	Blocked int         `json:"blocked"`
	Active  []vault.Ref `json:"active"`
	// List is the open plans and stubs in board order, for the opening context.
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

// Status is the state of the vault in one read.
type Status struct {
	Vault     VaultInfo         `json:"vault"`
	Documents map[string]int    `json:"documents"`
	Topics    TopicCounts       `json:"topics"`
	Tags      []TagCount        `json:"tags"`
	Work      WorkCounts        `json:"work"`
	Sessions  SessionLists      `json:"sessions"`
	Inbox     []InboxItem       `json:"inbox"`
	Pending   []vault.Ref       `json:"pending"`
	Changes   ChangeLists       `json:"changes"`
	Recent    []vault.Ref       `json:"recent"`
	Mentions  []Mention         `json:"mentions"`
	Problems  int               `json:"problems"`
	Versions  map[string]string `json:"versions,omitempty"`
}

// Recent is how many applied changes, and ten times how many events, the status lists.
const Recent = 5

// StatusOf reads the state of the vault.
func StatusOf(idx *vault.Index, now time.Time) *Status {
	v := idx.V
	st := &Status{
		Vault:     VaultInfo{ID: v.ID(), Name: v.Name(), Path: vault.Shorten(v.Root), Layout: v.LayoutVersion(), Tagging: v.Tagging()},
		Documents: map[string]int{},
		Topics:    TopicCounts{Kinds: map[string]int{}},
		Tags:      []TagCount{},
		Work:      WorkCounts{Active: []vault.Ref{}},
		Sessions:  SessionLists{Running: []vault.Ref{}, Waiting: []vault.Ref{}, Idle: []vault.Ref{}},
		Inbox:     Inbox(v),
		Pending:   idx.Refs(idx.PendingDocs()),
		Changes:   ChangeLists{Proposed: []vault.Ref{}, Recent: []vault.Ref{}},
		Recent:    []vault.Ref{},
		Mentions:  Mentions(idx),
	}
	for _, t := range schema.DocumentTypes {
		st.Documents[t] = 0
	}
	for _, d := range idx.Docs {
		switch d.Type() {
		case "topic":
			st.Topics.Kinds[d.Str("kind")]++
			switch d.Str("status") {
			case "draft":
				st.Topics.Draft++
			case "contested":
				st.Topics.Contested++
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
			if d.Str("status") == "proposed" {
				st.Changes.Proposed = append(st.Changes.Proposed, idx.Ref(d))
			}
		}
		if schema.IsDocument(d.Type()) {
			st.Documents[d.Type()]++
		}
	}
	counts := idx.TagCounts()
	var all []string
	for t := range counts {
		all = append(all, t)
	}
	idx.SortByCount(all)
	for _, t := range all {
		tc := TagCount{Tag: t, Count: counts[t]}
		if p := idx.TagPage(t); p != nil {
			tc.Page = p.Title()
		}
		st.Tags = append(st.Tags, tc)
	}
	b := work.Load(idx)
	bv := b.BoardView(work.Filter{})
	st.Work.Stubs = len(bv.Stubs)
	st.Work.Started = len(bv.Started) + len(bv.Active)
	st.Work.Blocked = len(bv.Blocked)
	st.Work.Open = len(bv.Ready) + len(bv.Waiting)
	st.Work.Active = bv.Active
	st.Work.List = append(append(append(append(append(append([]vault.Ref{}, bv.Active...), bv.Started...), bv.Blocked...), bv.Ready...), bv.Waiting...), bv.Stubs...)
	applied := idx.Of("change")
	sort.SliceStable(applied, func(i, j int) bool { return applied[i].Str("applied") > applied[j].Str("applied") })
	for _, c := range applied {
		if c.Str("status") != "applied" {
			continue
		}
		if len(st.Changes.Recent) == Recent {
			break
		}
		st.Changes.Recent = append(st.Changes.Recent, idx.Ref(c))
	}
	events := idx.Of("event")
	sort.SliceStable(events, func(i, j int) bool { return events[i].Str("at") > events[j].Str("at") })
	for i, e := range events {
		if i == 2*Recent {
			break
		}
		st.Recent = append(st.Recent, idx.Ref(e))
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
		out = append(out, InboxItem{Name: filepath.ToSlash(rel), Size: humanSize(info.Size()), Kind: source.Media(e.Name())})
		return nil
	})
	return out
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

// mentionSkip are the folders whose files hold no mention.
var mentionSkip = []string{vault.Documents, vault.Changes, vault.Sessions, vault.Views, vault.Scratchpad}

// isMention reports whether a task line addresses the agent.
func isMention(text string) bool {
	i := strings.Index(text, "@atlas")
	if i < 0 {
		return false
	}
	before := ""
	if i > 0 {
		before = text[i-1 : i]
	}
	after := ""
	if i+6 < len(text) {
		after = text[i+6 : i+7]
	}
	word := func(s string) bool {
		return s != "" && (s[0] == '_' || s[0] == '-' || s[0] == '.' || s[0] == '@' || (s[0] >= 'a' && s[0] <= 'z') || (s[0] >= 'A' && s[0] <= 'Z') || (s[0] >= '0' && s[0] <= '9'))
	}
	return !word(before) && !word(after)
}

// Mentions finds every open @atlas task line outside the documents, the changes, the
// sessions, the views, and the scratchpad.
func Mentions(idx *vault.Index) []Mention {
	out := []Mention{}
	for _, t := range idx.OpenTasks(isMention, mentionSkip...) {
		ref := vault.Ref{Title: vault.Title(t.Doc), Path: t.Doc.Path, Tags: []string{}}
		if idx.ByID(t.Doc.ID()) == t.Doc {
			ref = idx.Ref(t.Doc)
		}
		out = append(out, Mention{Doc: ref, Line: t.Line, Text: t.Text})
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
	text, ok := vault.OpenTaskText(lines[line-1])
	if !ok || !isMention(text) {
		return nil, fmt.Errorf("line %d of %s is no open @atlas mention; it may be closed already", line, rel)
	}
	lines[line-1] = strings.Replace(lines[line-1], "[ ]", "[x]", 1) + " → " + doc.Link(vault.Title(answer))
	if err := v.Write(rel, []byte(strings.Join(lines, "\n"))); err != nil {
		return nil, err
	}
	return &Mention{Doc: vault.Ref{Title: vault.NoteTitle(rel), Path: rel, Tags: []string{}}, Line: line, Text: strings.TrimSpace(lines[line-1])}, nil
}

// Synced is what a sync changed.
type Synced struct {
	Moved     []string `json:"moved"`
	Lost      []string `json:"lost"`
	Work      []string `json:"work"`
	Knowledge []string `json:"knowledge"`
	Sessions  []string `json:"sessions"`
	Settings  bool     `json:"settings"`
	Views     int      `json:"views"`
}

// SyncOptions select a sync.
type SyncOptions struct {
	// Views runs the steps that read no git: the statuses, the callouts, and the views.
	Views bool
}

// Sync heals every derived part of the vault and writes the views. It writes a file only
// when its derived content differs, never changes updated, and makes no commit.
func Sync(v *vault.Vault, now time.Time, o SyncOptions) (*Synced, error) {
	if err := v.CheckLayout(); err != nil {
		return nil, err
	}
	unlock, err := v.Lock()
	if err != nil {
		return nil, err
	}
	defer unlock()
	return SyncLocked(v, now, o)
}

// SyncLocked is Sync for a caller that holds the lock.
func SyncLocked(v *vault.Vault, now time.Time, o SyncOptions) (*Synced, error) {
	out := &Synced{Moved: []string{}, Lost: []string{}, Work: []string{}, Knowledge: []string{}, Sessions: []string{}}
	if !o.Views {
		if err := vault.Recover(v); err != nil {
			return nil, err
		}
		if err := v.EnsureFolders(); err != nil {
			return nil, err
		}
		moved, err := FileByHand(v)
		if err != nil {
			return nil, err
		}
		out.Moved = append(out.Moved, moved...)
		lost, err := sessions.MarkLost(v, now, v.StaleHours())
		if err != nil {
			return nil, err
		}
		out.Lost = append(out.Lost, lost...)
	}
	idx, err := vault.Load(v)
	if err != nil {
		return nil, err
	}
	wrote, err := work.Load(idx).Sync(v.WriteIfChanged)
	if err != nil {
		return nil, err
	}
	out.Work = append(out.Work, wrote...)
	if !o.Views {
		if idx, err = vault.Load(v); err != nil {
			return nil, err
		}
		facts, err := derive.GitFacts(idx, v.WriteIfChanged, now)
		if err != nil {
			return out, err
		}
		out.Knowledge = append(out.Knowledge, facts...)
	}
	if idx, err = vault.Load(v); err != nil {
		return nil, err
	}
	knowledge, err := derive.Sync(idx, v.WriteIfChanged)
	if err != nil {
		return out, err
	}
	out.Knowledge = append(out.Knowledge, knowledge...)
	if !o.Views {
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
	}
	if idx, err = vault.Load(v); err != nil {
		return nil, err
	}
	written, err := views.Write(idx, now)
	if err != nil {
		return out, err
	}
	out.Views = len(written)
	return out, nil
}

// FileByHand finishes a hand move: a typed document of wiki/documents that lies anywhere
// else under wiki/ goes back into wiki/documents, when its title is free there. It
// commits nothing; the next snapshot records the move. The caller holds the lock.
func FileByHand(v *vault.Vault) ([]string, error) {
	idx, err := vault.Load(v)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, d := range idx.Misplaced {
		if !schema.IsDocument(d.Type()) || !strings.HasPrefix(d.Path, vault.Wiki+"/") || strings.HasPrefix(d.Path, vault.Assets+"/") {
			continue
		}
		to := vault.DocPath(path.Base(strings.TrimSuffix(d.Path, ".md")))
		if v.Exists(to) {
			continue // lint reports it
		}
		if err := os.MkdirAll(filepath.Dir(v.Abs(to)), 0o755); err != nil {
			return out, err
		}
		if err := os.Rename(v.Abs(d.Path), v.Abs(to)); err != nil {
			return out, err
		}
		v.Remove(d.Path)
		out = append(out, to)
	}
	return out, nil
}

// Views writes the views alone, under the lock: what a write tool runs after its commit.
func Views(v *vault.Vault, now time.Time) error {
	unlock, err := v.Lock()
	if err != nil {
		return err
	}
	defer unlock()
	idx, err := vault.Load(v)
	if err != nil {
		return err
	}
	_, err = views.Write(idx, now)
	return err
}

// Init makes a new vault, writes its views, and returns its status.
func Init(opts vault.InitOptions, h vault.Home, now time.Time) (*Status, error) {
	v, err := vault.Init(opts, h, now)
	if err != nil {
		return nil, err
	}
	if err := Views(v, now); err != nil {
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
