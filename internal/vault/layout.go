package vault

import (
	"path"
	"strings"
)

// TypeFolders are the folders the knowledge types take inside the folder of their scope.
var TypeFolders = map[string]string{"concept": "concepts", "entity": "entities", "policy": "policies", "source": "sources"}

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
