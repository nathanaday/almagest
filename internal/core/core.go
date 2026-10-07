// Package core is the vault tool's backend: the state of the vault in one read, the sync
// that rewrites every derived part and writes the views, and the snapshot of hand edits.
package core

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/nathanaday/almagest/internal/checkout"
	"github.com/nathanaday/almagest/internal/derive"
	"github.com/nathanaday/almagest/internal/journal"
	"github.com/nathanaday/almagest/internal/lint"
	"github.com/nathanaday/almagest/internal/schema"
	"github.com/nathanaday/almagest/internal/sessions"
	"github.com/nathanaday/almagest/internal/source"
	"github.com/nathanaday/almagest/internal/vault"
	"github.com/nathanaday/almagest/internal/views"
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

// SessionLists are the live sessions.
type SessionLists struct {
	Running []vault.Ref `json:"running"`
	Waiting []vault.Ref `json:"waiting"`
	Idle    []vault.Ref `json:"idle"`
}

// ChangeLists are the changes that wait, the work that runs, and the last applied.
type ChangeLists struct {
	Proposed []vault.Ref `json:"proposed"`
	Running  []vault.Ref `json:"running"`
	Recent   []vault.Ref `json:"recent"`
}

// IngestItem is one file that waits in ingest/.
type IngestItem struct {
	Name string `json:"name"`
	Size string `json:"size"`
	Kind string `json:"kind"`
}

// Status is the state of the vault in one read.
type Status struct {
	Vault     VaultInfo      `json:"vault"`
	Documents map[string]int `json:"documents"`
	Topics    TopicCounts    `json:"topics"`
	Tags      []TagCount     `json:"tags"`
	Sessions  SessionLists   `json:"sessions"`
	Ingest    []IngestItem   `json:"ingest"`
	Pending   []vault.Ref    `json:"pending"`
	Changes   ChangeLists    `json:"changes"`
	// Trash counts the files in tool/trash/.
	Trash int `json:"trash"`
	// Journals are the journal volumes, each with its latest edition.
	Journals []journal.Volume `json:"journals"`
	// Checkouts are the librarian's checkouts, newest first.
	Checkouts []checkout.Entry  `json:"checkouts"`
	Problems  int               `json:"problems"`
	Versions  map[string]string `json:"versions,omitempty"`
}

// Recent is how many applied changes the status lists.
const Recent = 5

// StatusOf reads the state of the vault.
func StatusOf(idx *vault.Index, now time.Time) *Status {
	v := idx.V
	st := &Status{
		Vault:     VaultInfo{ID: v.ID(), Name: v.Name(), Path: vault.Shorten(v.Root), Layout: v.LayoutVersion(), Tagging: v.Tagging()},
		Documents: map[string]int{},
		Topics:    TopicCounts{Kinds: map[string]int{}},
		Tags:      []TagCount{},
		Sessions:  SessionLists{Running: []vault.Ref{}, Waiting: []vault.Ref{}, Idle: []vault.Ref{}},
		Ingest:    Ingest(v),
		Pending:   idx.Refs(idx.PendingDocs()),
		Changes:   ChangeLists{Proposed: []vault.Ref{}, Running: []vault.Ref{}, Recent: []vault.Ref{}},
		Trash:     countFiles(v.Abs(vault.Trash)),
		Journals:  journal.Volumes(idx),
		Checkouts: checkout.List(v),
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
			switch d.Str("status") {
			case "proposed":
				st.Changes.Proposed = append(st.Changes.Proposed, idx.Ref(d))
			case "running":
				st.Changes.Running = append(st.Changes.Running, idx.Ref(d))
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
	if f, err := lint.Run(idx, lint.Options{Quick: true, Now: now}); err == nil {
		st.Problems = f.Counts[lint.Error]
	}
	return st
}

// Ingest lists what waits in ingest/.
func Ingest(v *vault.Vault) []IngestItem {
	out := []IngestItem{}
	root := v.Abs(vault.Ingest)
	filepath.WalkDir(root, func(abs string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			return nil
		}
		info, err := e.Info()
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, abs)
		out = append(out, IngestItem{Name: filepath.ToSlash(rel), Size: humanSize(info.Size()), Kind: source.Media(e.Name())})
		return nil
	})
	return out
}

// countFiles counts the files under root, but .DS_Store.
func countFiles(root string) int {
	n := 0
	filepath.WalkDir(root, func(_ string, e fs.DirEntry, err error) error {
		if err == nil && !e.IsDir() && e.Name() != ".DS_Store" {
			n++
		}
		return nil
	})
	return n
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

// Synced is what a sync changed.
type Synced struct {
	Moved     []string `json:"moved"`
	Lost      []string `json:"lost"`
	Knowledge []string `json:"knowledge"`
	Sessions  []string `json:"sessions"`
	Settings  bool     `json:"settings"`
	Views     int      `json:"views"`
	// Strays are the notes found in wiki-view/ that code did not write, moved to ingest/.
	Strays []vault.Moved `json:"strays"`
	// Skipped are the documents saved after the sync read them, which it left as saved.
	Skipped []string `json:"skipped"`
}

// SyncOptions select a sync.
type SyncOptions struct {
	// Views runs the steps that read no git: the statuses, the callouts, and the views.
	Views bool
}

// Sync rewrites every derived part of the vault and writes the views. It writes a file only
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
	out := &Synced{Moved: []string{}, Lost: []string{}, Knowledge: []string{}, Sessions: []string{}, Strays: []vault.Moved{}, Skipped: []string{}}
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
	if !o.Views {
		guard := vault.NewGuard(idx, v)
		facts, err := derive.GitFacts(idx, guard.Write, now)
		out.Skipped = append(out.Skipped, guard.Skipped...)
		if err != nil {
			return out, err
		}
		out.Knowledge = append(out.Knowledge, facts...)
	}
	if idx, err = vault.Load(v); err != nil {
		return nil, err
	}
	guard := vault.NewGuard(idx, v)
	knowledge, err := derive.Sync(idx, guard.Write)
	out.Skipped = append(out.Skipped, guard.Skipped...)
	if err != nil {
		return out, err
	}
	out.Knowledge = append(out.Knowledge, knowledge...)
	linked, err := sessions.LinkConversations(idx)
	if err != nil {
		return out, err
	}
	out.Sessions = append(out.Sessions, linked...)
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
	if !o.Views {
		if _, err := journal.WriteMissingHistories(idx); err != nil {
			return out, err
		}
	}
	written, strays, err := views.Write(idx, now)
	out.Strays = append(out.Strays, strays...)
	if err != nil {
		return out, err
	}
	out.Views = len(written)
	return out, nil
}

// FileByHand finishes a hand move: a typed document that lies anywhere else under
// tool/source-core/ goes back into tool/source-core/documents, when its title is free there. It
// commits nothing; the next snapshot records the move. The caller holds the lock.
func FileByHand(v *vault.Vault) ([]string, error) {
	idx, err := vault.Load(v)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, d := range idx.Misplaced {
		if !schema.IsDocument(d.Type()) || !strings.HasPrefix(d.Path, vault.Core+"/") || strings.HasPrefix(d.Path, vault.Originals+"/") {
			continue
		}
		to := vault.DocPath(path.Base(strings.TrimSuffix(d.Path, ".md")))
		if v.Exists(to) {
			continue // lint reports it
		}
		if v.Contain(d.Path) != nil || v.Contain(to) != nil {
			continue // a path through a link out of the vault is never moved
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
func Views(v *vault.Vault, now time.Time) ([]vault.Moved, error) {
	unlock, err := v.Lock()
	if err != nil {
		return nil, err
	}
	defer unlock()
	idx, err := vault.Load(v)
	if err != nil {
		return nil, err
	}
	_, strays, err := views.Write(idx, now)
	return strays, err
}

// StrayLine is the line that tells the user where a note found in wiki-view/ went.
func StrayLine(m vault.Moved) string {
	return fmt.Sprintf("Moved %s to %s: code writes every file in wiki-view/, so a note of yours waits in ingest/.", m.From, m.To)
}

// Snapshot commits every hand edit of the vault as one snapshot commit. It returns the
// commit and the count of files, or "" and 0 when the tree is clean. It takes the lock
// without waiting: a held lock means a write is running, and that write commits the
// hand edits itself.
func Snapshot(v *vault.Vault) (string, int, error) {
	unlock, err := v.LockWithin(0)
	if err != nil {
		return "", 0, err
	}
	defer unlock()
	if err := v.Git().CheckIdle(); err != nil {
		return "", 0, err
	}
	// An apply a crash stopped is put back first, so the snapshot never records its
	// half-written documents as hand edits.
	if err := vault.Recover(v); err != nil {
		return "", 0, err
	}
	entries, err := v.Git().Status()
	if err != nil || len(entries) == 0 {
		return "", 0, err
	}
	sha, err := vault.CommitSnapshot(v)
	if err != nil || sha == "" {
		return "", 0, err
	}
	return sha, len(entries), nil
}

// Init makes a new vault, writes its views, and returns its status.
func Init(opts vault.InitOptions, h vault.Home, now time.Time) (*Status, error) {
	v, err := vault.Init(opts, h, now)
	if err != nil {
		return nil, err
	}
	if _, err := Views(v, now); err != nil {
		return nil, err
	}
	idx, err := vault.Load(v)
	if err != nil {
		return nil, err
	}
	return StatusOf(idx, now), nil
}
