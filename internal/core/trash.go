package core

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/change"
	"github.com/nathanaday/atlas-obsidian/internal/schema"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

// Trashed is what safe delete did with a file: its backlinks, which keep it, or where it
// went, and the change that removed a knowledge document.
type Trashed struct {
	Path      string      `json:"path"`
	Backlinks []vault.Ref `json:"backlinks"`
	Moved     string      `json:"moved"`
	Change    *vault.Ref  `json:"change,omitempty"`
}

// Trash is safe delete, the user's own act: a file that nothing links goes to trash/,
// and a file that something links stays, with its backlinks named. A knowledge document
// leaves through a change applied at once, so its record and its undo are a change's;
// any other file moves in a commit of its own.
func Trash(v *vault.Vault, name string, now time.Time) (*Trashed, error) {
	rel := name
	if filepath.IsAbs(name) {
		rel = v.Rel(name)
	}
	rel = path.Clean(filepath.ToSlash(rel))
	if err := v.Contain(rel); err != nil {
		return nil, err
	}
	// The rules and the index name the file as the disk does; a path in another case
	// meets the same rules.
	rel = v.Spelled(rel)
	if why := trashRefusal(rel); why != "" {
		return nil, fmt.Errorf("%s %s", rel, why)
	}
	st, err := os.Lstat(v.Abs(rel))
	switch {
	case err != nil:
		return nil, fmt.Errorf("%s is no file of the vault", rel)
	case !st.Mode().IsRegular():
		return nil, fmt.Errorf("%s is a folder or a link; safe delete takes one file", rel)
	}
	idx, err := vault.Load(v)
	if err != nil {
		return nil, err
	}
	out := &Trashed{Path: rel, Backlinks: []vault.Ref{}}
	for _, p := range idx.Backlinks(rel) {
		ref := vault.Ref{Title: vault.NoteTitle(p), Path: p, Tags: []string{}}
		if d := idx.ByPath(p); d != nil && d.ID() != "" && idx.ByID(d.ID()) == d {
			ref = idx.Ref(d)
		}
		out.Backlinks = append(out.Backlinks, ref)
	}
	if len(out.Backlinks) > 0 {
		return out, nil
	}
	if d := idx.ByPath(rel); d != nil && schema.IsDocument(d.Type()) && vault.InPlace(d) {
		title := vault.Title(d)
		pv, err := change.Propose(v, change.Plan{
			Title:  "Delete " + title,
			Notes:  "Safe delete from Obsidian: no document linked " + title + ".",
			Writes: []change.Write{{Op: change.OpRemove, ID: d.ID(), Why: "safe delete; nothing links it"}},
		}, now)
		if err != nil {
			return nil, err
		}
		id := pv.Ref.ID
		if pv, err = change.Apply(v, id, now, nil); err != nil {
			// The delete stops whole: no proposal of it stays behind.
			change.Reject(v, id, "safe delete stopped: "+err.Error(), now)
			return nil, err
		}
		for _, w := range pv.Writes {
			if w.Op == change.OpRemove {
				out.Moved = w.Trash
			}
		}
		out.Change = &pv.Ref
		return out, nil
	}
	if out.Moved, err = trashFile(v, rel, now); err != nil {
		return nil, err
	}
	return out, nil
}

// trashFile moves a file that is no knowledge document to the trash, in a commit.
func trashFile(v *vault.Vault, rel string, now time.Time) (_ string, err error) {
	tx, err := vault.BeginWrite(v)
	if err != nil {
		return "", err
	}
	defer tx.End(&err)
	to := vault.TrashPath(v, rel, now, map[string]bool{})
	if err := tx.Move(rel, to); err != nil {
		return "", err
	}
	v.Prune(rel)
	if _, err := tx.Commit("trash: " + rel); err != nil {
		return "", err
	}
	return to, nil
}

// trashRefusal says why safe delete does not take a path, or "".
func trashRefusal(rel string) string {
	top, _, _ := strings.Cut(rel, "/")
	is := func(names ...string) bool {
		return slices.ContainsFunc(names, func(n string) bool { return strings.EqualFold(top, n) })
	}
	switch {
	case strings.EqualFold(rel, vault.Marker):
		return "is the vault's own document"
	case is(vault.Obsidian, ".claude", ".atlas"):
		return "is a setting of the vault, not a note"
	case is(vault.Changes, vault.Sessions):
		return "is a record that code keeps"
	case is(vault.WikiView):
		return "is a view, which code writes again from the documents"
	case is(vault.Trash):
		return "is in the trash already; empty the trash to delete it"
	case slices.ContainsFunc(shippedBases(), func(b string) bool { return strings.EqualFold(b, rel) }):
		return "is a Base that Atlas ships"
	}
	return ""
}

func shippedBases() []string {
	var out []string
	for _, rel := range vault.Bases {
		out = append(out, rel)
	}
	return out
}
