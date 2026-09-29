package threads

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

// Refile moves each thread whose folder lies in a folder of threads/ that stands for no
// scope, such as the mirror of an area renamed outside a change, into the mirror of its
// first scope's folder, or to the top of threads/ when it names none. A thread at the top,
// or in the mirror of a scope, stays: that is where the user filed it. It returns the
// threads' new folders. Like sync, it commits nothing. The caller holds the lock.
func Refile(v *vault.Vault) ([]string, error) {
	idx, err := vault.Load(v)
	if err != nil {
		return nil, err
	}
	folders := idx.ScopeFolders()
	mirrors := map[string]bool{vault.Threads: true}
	for dir := range folders {
		mirrors[vault.Mirror(dir)] = true
	}
	var moved []string
	for _, stub := range idx.Of("stub") {
		folder := path.Dir(stub.Path)
		if !strings.HasPrefix(folder, vault.Threads+"/") || path.Base(folder) != stub.Title() || mirrors[path.Dir(folder)] {
			continue
		}
		home := vault.Threads
		if list := stub.List("scope"); len(list) > 0 {
			if s := idx.Linked(list[0]); s != nil && idx.Folder(s) != "" {
				home = vault.Mirror(idx.Folder(s))
			}
		}
		to := home + "/" + path.Base(folder)
		if v.Exists(to) {
			continue // lint reports it
		}
		var files []string
		filepath.WalkDir(v.Abs(folder), func(abs string, e fs.DirEntry, err error) error {
			if err == nil && !e.IsDir() {
				files = append(files, v.Rel(abs))
			}
			return err
		})
		for _, rel := range files {
			dest := v.Abs(to + strings.TrimPrefix(rel, folder))
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				return moved, err
			}
			if err := os.Rename(v.Abs(rel), dest); err != nil {
				return moved, err
			}
			v.Prune(rel)
		}
		moved = append(moved, to)
	}
	return moved, nil
}
