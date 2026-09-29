package vault

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nathanaday/atlas-obsidian/internal/doc"
)

// MigrateLayout moves a vault from the layout where scope pages lived in wiki/areas and
// wiki/repositories, and every knowledge page in its type's folder at the top of the
// wiki, to the layout where each scope is a folder: each scope page moves into a folder
// of its own under its parent's, and each knowledge page into its type's folder under its
// scope's. The fields scope and parent say where. It commits hand edits first, then the
// moves as one commit, and returns that commit, or "" when the vault needs no move. The
// caller holds the lock.
func MigrateLayout(v *Vault) (string, error) {
	idx, err := Load(v)
	if err != nil || !idx.Legacy() {
		return "", err
	}
	moves, err := legacyMoves(idx)
	if err != nil {
		return "", err
	}
	g := v.Git()
	if !g.HasHead() {
		return "", nil
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
	}
	for _, f := range legacyFolders {
		os.Remove(v.Abs(f))
	}
	if err := g.Add(paths...); err != nil {
		return "", err
	}
	if staged, err := g.Staged(); err != nil || !staged {
		return "", err
	}
	return g.Commit(fmt.Sprintf("layout: move %d pages into the folders of their scopes", len(moves)))
}

// legacyMoves maps each page the migration moves to where it goes.
func legacyMoves(idx *Index) (map[string]string, error) {
	scopes := map[string]*doc.Doc{}
	for _, d := range idx.Of("area", "repository") {
		if inLegacyFolder(d.Path) && !IsFolderPage(d.Path) {
			if ReservedTitle(Title(d)) {
				return nil, fmt.Errorf("the %s %s takes the name of a type folder; rename it before the move", d.Type(), Title(d))
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
	taken := map[string]string{}
	for from, to := range moves {
		key := strings.ToLower(to)
		if other, ok := taken[key]; ok {
			return nil, fmt.Errorf("%s and %s would both move to %s", other, from, to)
		}
		taken[key] = from
		if idx.V.Exists(to) && !strings.EqualFold(from, to) {
			return nil, fmt.Errorf("%s would move to %s, which holds a file", from, to)
		}
	}
	return moves, nil
}
