package vault

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nathanaday/atlas-obsidian/internal/doc"
)

// MigrateLayout moves a vault to the layout where each scope is a folder, once: the wiki
// from wiki/areas and wiki/repositories, where scope pages lived, and each knowledge page
// from its type's folder at the top of the wiki into its type's folder of its scope; and
// each thread's folder from the top of threads/ into the mirror of its first scope's
// folder. The fields scope and parent say where. A thread that names no scope stays at the
// top. It commits hand edits first, then the moves and Atlas.md's layout field as one
// commit, and returns that commit, or "" when the vault needs no move. The caller holds
// the lock.
func MigrateLayout(v *Vault) (string, error) {
	g := v.Git()
	if !g.HasHead() {
		return "", nil
	}
	data, err := v.Read(Marker)
	if err != nil {
		return "", nil
	}
	atlas := doc.Parse(Marker, data)
	current := atlas.Front != nil && atlas.Front.Int("layout") >= Layout
	idx, err := Load(v)
	if err != nil || (current && !idx.Legacy()) {
		return "", err
	}
	moves, folders := map[string]string{}, map[string]string{}
	if idx.Legacy() {
		if moves, folders, err = legacyMoves(idx); err != nil {
			return "", err
		}
	}
	if !current {
		if err := threadMoves(v, idx, folders, moves); err != nil {
			return "", err
		}
	}
	if err := checkMoves(idx, moves); err != nil {
		return "", err
	}
	if _, err := CommitSnapshot(v); err != nil {
		return "", err
	}
	froms := make([]string, 0, len(moves))
	for from := range moves {
		froms = append(froms, from)
	}
	sort.Strings(froms)
	var paths []string
	for _, from := range froms {
		to := moves[from]
		if err := os.MkdirAll(filepath.Dir(v.Abs(to)), 0o755); err != nil {
			return "", err
		}
		if err := os.Rename(v.Abs(from), v.Abs(to)); err != nil {
			return "", err
		}
		paths = append(paths, from, to)
		v.Prune(from)
	}
	for _, f := range legacyFolders {
		os.Remove(v.Abs(f))
	}
	if !current {
		if data, err := v.Read(Marker); err == nil {
			if err := v.Write(Marker, []byte(doc.SetField(string(data), "layout", Layout))); err != nil {
				return "", err
			}
			paths = append(paths, Marker)
		}
	}
	if err := g.Add(paths...); err != nil {
		return "", err
	}
	if staged, err := g.Staged(); err != nil || !staged {
		return "", err
	}
	return g.Commit(fmt.Sprintf("layout: move %d files into the folders of their scopes", len(moves)))
}

// threadMoves adds the files of each thread at the top of threads/ that names a scope, to
// go to the mirror of that scope's folder. folders holds the scope folders a move of the
// wiki gives; the others are where the index finds them.
func threadMoves(v *Vault, idx *Index, folders, moves map[string]string) error {
	for _, stub := range idx.Of("stub") {
		folder := path.Dir(stub.Path)
		if path.Dir(folder) != Threads || path.Base(folder) != stub.Title() {
			continue
		}
		list := stub.List("scope")
		if len(list) == 0 {
			continue
		}
		s := idx.Linked(list[0])
		if s == nil {
			continue
		}
		dir, ok := folders[s.ID()]
		if !ok {
			dir = idx.Folder(s)
		}
		if dir == "" {
			continue
		}
		to := Mirror(dir) + "/" + path.Base(folder)
		err := filepath.WalkDir(v.Abs(folder), func(abs string, e fs.DirEntry, err error) error {
			if err == nil && !e.IsDir() {
				rel := v.Rel(abs)
				moves[rel] = to + strings.TrimPrefix(rel, folder)
			}
			return err
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// checkMoves refuses two files on one path and a file that lands on a file that stays.
func checkMoves(idx *Index, moves map[string]string) error {
	taken := map[string]string{}
	for from, to := range moves {
		key := strings.ToLower(to)
		if other, ok := taken[key]; ok {
			return fmt.Errorf("%s and %s would both move to %s", other, from, to)
		}
		taken[key] = from
		if idx.V.Exists(to) && !strings.EqualFold(from, to) {
			return fmt.Errorf("%s would move to %s, which holds a file", from, to)
		}
	}
	return nil
}

// legacyMoves maps each page of the old wiki layout to where it goes, and each scope's id to
// its folder.
func legacyMoves(idx *Index) (map[string]string, map[string]string, error) {
	scopes := map[string]*doc.Doc{}
	for _, d := range idx.Of("area", "repository") {
		if inLegacyFolder(d.Path) && !IsFolderPage(d.Path) {
			if ReservedTitle(Title(d)) {
				return nil, nil, fmt.Errorf("the %s %s takes the name of a type folder; rename it before the move", d.Type(), Title(d))
			}
			scopes[d.ID()] = d
		}
	}
	folders := map[string]string{}
	var folder func(d *doc.Doc, seen map[string]bool) string
	folder = func(d *doc.Doc, seen map[string]bool) string {
		if f, ok := folders[d.ID()]; ok {
			return f
		}
		parentDir := Wiki
		if p := idx.Linked(d.Str("parent")); p != nil && p.Type() == "area" && scopes[p.ID()] != nil && !seen[p.ID()] {
			seen[d.ID()] = true
			parentDir = folder(p, seen)
		}
		folders[d.ID()] = parentDir + "/" + Title(d)
		return folders[d.ID()]
	}
	moves := map[string]string{}
	for _, d := range scopes {
		moves[d.Path] = folder(d, map[string]bool{}) + "/" + Title(d) + ".md"
	}
	for _, d := range idx.Of("concept", "entity", "policy", "source") {
		tf := TypeFolders[d.Type()]
		if path.Dir(d.Path) != Wiki+"/"+tf {
			continue
		}
		s := idx.Linked(d.Str("scope"))
		if s == nil || scopes[s.ID()] == nil {
			continue
		}
		moves[d.Path] = folders[s.ID()] + "/" + tf + "/" + path.Base(d.Path)
	}
	return moves, folders, nil
}
