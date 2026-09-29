package vault

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// TypeFolders are the folders the knowledge types take inside the folder of their scope.
var TypeFolders = map[string]string{"concept": "concepts", "entity": "entities", "policy": "policies", "source": "sources"}

// Layout is the layout version this binary writes, kept in Atlas.md's layout field: 2 is
// a folder for each scope, in the wiki and in threads/. A vault without the field is moved
// once (MigrateLayout).
const Layout = 2

// legacyFolders held the scope pages before each scope had a folder of its own.
var legacyFolders = []string{"wiki/areas", "wiki/repositories"}

// ReservedTitle reports whether a title is the name of a type folder, which no area or
// repository may take: its folder would be a type folder of its parent.
func ReservedTitle(title string) bool {
	for _, f := range TypeFolders {
		if strings.EqualFold(strings.TrimSpace(title), f) {
			return true
		}
	}
	return false
}

// IsFolderPage reports whether rel is the page of its own folder under the wiki:
// wiki/…/X/X.md. Only such a page makes its folder a scope.
func IsFolderPage(rel string) bool {
	dir := path.Dir(rel)
	if !strings.HasPrefix(dir, Wiki+"/") {
		return false
	}
	return path.Base(dir)+".md" == path.Base(rel)
}

// Route is where a page of a type with a title lives, given the folder of its scope
// ("wiki" for the vault): an area or a repository in a folder of its own, a knowledge
// page in its type's folder.
func Route(scopeDir, typ, title string) string {
	switch typ {
	case "area", "repository":
		return scopeDir + "/" + title + "/" + title + ".md"
	}
	if f, ok := TypeFolders[typ]; ok {
		return scopeDir + "/" + f + "/" + title + ".md"
	}
	return scopeDir + "/" + title + ".md"
}

// Mirror is the folder under threads/ that stands for a scope folder of the wiki:
// wiki/ML/CS566 is threads/ML/CS566, and the wiki itself is threads/.
func Mirror(wikiDir string) string {
	return Threads + strings.TrimPrefix(wikiDir, Wiki)
}

// wikiDirOf is the scope folder of the wiki a folder of threads/ stands for, or "".
func wikiDirOf(dir string) string {
	if dir == Threads {
		return Wiki
	}
	if rest, ok := strings.CutPrefix(dir, Threads+"/"); ok {
		return Wiki + "/" + rest
	}
	return ""
}

// inLegacyFolder reports whether rel lies directly in a folder that held scope pages
// before each scope had a folder.
func inLegacyFolder(rel string) bool {
	dir := path.Dir(rel)
	for _, f := range legacyFolders {
		if dir == f {
			return true
		}
	}
	return false
}

// FindThreadFile is the path of a file named name.md in a thread's folder, anywhere under
// threads/, read without the index: a stub, when name is a thread's title, or any other
// thread document. "" when none holds it.
func (v *Vault) FindThreadFile(name string) string {
	want := name + ".md"
	found := ""
	filepath.WalkDir(v.Abs(Threads), func(abs string, e fs.DirEntry, err error) error {
		if err != nil || found != "" {
			return filepath.SkipDir
		}
		if e.IsDir() && strings.HasPrefix(e.Name(), ".") {
			return filepath.SkipDir
		}
		if !e.IsDir() && e.Name() == want {
			found = v.Rel(abs)
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

// FileByHand finishes what a hand move started, so that dropping a file on a folder is
// enough: it makes the folder under threads/ that stands for each scope, where a thread
// can be dropped; removes an empty folder of threads/ that stands for no scope; and moves
// a knowledge page that lies right in the wiki's folder or a scope's folder into its
// type's folder there. It returns the paths it moved to. Like sync, it commits nothing.
// The caller holds the lock.
func FileByHand(v *Vault) ([]string, error) {
	idx, err := Load(v)
	if err != nil || idx.Legacy() {
		return nil, err
	}
	folders := idx.ScopeFolders()
	mirrors := map[string]bool{}
	for dir := range folders {
		mirrors[Mirror(dir)] = true
		if err := os.MkdirAll(v.Abs(Mirror(dir)), 0o755); err != nil {
			return nil, err
		}
	}
	var stale []string
	filepath.WalkDir(v.Abs(Threads), func(abs string, e fs.DirEntry, err error) error {
		if err == nil && e.IsDir() && !mirrors[v.Rel(abs)] && v.Rel(abs) != Threads {
			stale = append(stale, abs)
		}
		return nil
	})
	for i := len(stale) - 1; i >= 0; i-- {
		os.Remove(stale[i]) // only an empty one goes
	}
	var moved []string
	for _, d := range idx.Of("concept", "entity", "policy", "source") {
		dir := path.Dir(d.Path)
		if dir != Wiki && folders[dir] == nil {
			continue
		}
		to := Route(dir, d.Type(), d.Title())
		if v.Exists(to) {
			continue // lint reports it
		}
		if err := os.MkdirAll(filepath.Dir(v.Abs(to)), 0o755); err != nil {
			return moved, err
		}
		if err := os.Rename(v.Abs(d.Path), v.Abs(to)); err != nil {
			return moved, err
		}
		moved = append(moved, to)
	}
	return moved, nil
}
