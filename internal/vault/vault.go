// Package vault is one vault on disk: its layout, the vault document Atlas.md, the machine
// file that lists every vault, the lock every write takes, and the index of every document
// the other packages read.
package vault

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/gitx"
)

// The layout, relative to the vault.
const (
	Marker      = "Atlas.md"
	Inbox       = "inbox"
	Scratchpad  = "scratchpad"
	Sessions    = "sessions"
	Threads     = "threads"
	Changes     = "changes"
	Wiki        = "wiki"
	SourceFiles = "wiki/sources/files"
	Settings    = ".claude/settings.local.json"
	Obsidian    = ".obsidian"
	PluginDir   = ".obsidian/plugins/atlas"
)

// ThreadsCanvas is the board as a canvas: a card for each open thread, grouped by its
// home scope. Sync derives it.
const ThreadsCanvas = "threads/Threads.canvas"

// Folders are every folder of the layout. EnsureFolders makes the ones a clone left out,
// because git keeps no empty folder.
var Folders = []string{
	Inbox, Scratchpad, Sessions, Threads, Changes, Wiki,
	"wiki/areas", "wiki/repositories", "wiki/concepts", "wiki/entities", "wiki/policies",
	"wiki/sources", SourceFiles,
}

// Excluded are the patterns kept out of the vault's history on each machine: the
// harness settings hold this machine's paths, Obsidian rewrites its workspace on every
// click and its graph settings on every zoom, and the Obsidian plugin rewrites the
// graph's color groups when a document changes.
var Excluded = []string{"/.claude/settings.local.json", "/.obsidian/workspace.json", "/.obsidian/workspace-mobile.json", "/.obsidian/graph.json", ".DS_Store"}

// Defaults of the vault document.
var (
	DefaultWikify     = []string{"source", "spec", "receipt"}
	DefaultStaleHours = 12
	AreaSettings      = []string{"many", "few", "manual"}
)

// Vault is one vault: its folder and the settings of its vault document.
type Vault struct {
	Root string
	Doc  *doc.Doc
}

// ErrNoVault is returned when no vault holds a folder.
var ErrNoVault = errors.New("no vault")

// Open reads the vault at root. The folder must hold an Atlas.md of type vault.
func Open(root string) (*Vault, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(abs, Marker))
	if err != nil {
		return nil, fmt.Errorf("%s: %w: no %s", abs, ErrNoVault, Marker)
	}
	d := doc.Parse(Marker, data)
	if d.FrontErr != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Join(abs, Marker), d.FrontErr)
	}
	if d.Type() != "vault" {
		return nil, fmt.Errorf("%s: %w: %s is not of type vault", abs, ErrNoVault, Marker)
	}
	return &Vault{Root: abs, Doc: d}, nil
}

// ID is the vault's id.
func (v *Vault) ID() string { return v.Doc.ID() }

// Name is the vault's name.
func (v *Vault) Name() string {
	if n := v.Doc.Str("name"); n != "" {
		return n
	}
	return filepath.Base(v.Root)
}

// Description is the vault's one line.
func (v *Vault) Description() string { return v.Doc.Str("description") }

// Areas is how readily the agent proposes areas: many, few, or manual.
func (v *Vault) Areas() string {
	if a := v.Doc.Str("areas"); slices.Contains(AreaSettings, a) {
		return a
	}
	return "few"
}

// Wikify lists the types that are pending until a change absorbs them.
func (v *Vault) Wikify() []string {
	if v.Doc.Front != nil && v.Doc.Front.Has("wikify") {
		return v.Doc.List("wikify")
	}
	return DefaultWikify
}

// StaleHours is how long a live session may go without a hook event before it is lost.
func (v *Vault) StaleHours() int {
	if n := v.Doc.Front.Int("stale_hours"); n > 0 {
		return n
	}
	return DefaultStaleHours
}

// Context is the body of Atlas.md: the context every agent in the vault must know.
func (v *Vault) Context() string { return strings.TrimSpace(doc.StripLead(v.Doc.Body)) }

// Abs is a vault-relative path on disk.
func (v *Vault) Abs(rel string) string { return filepath.Join(v.Root, filepath.FromSlash(rel)) }

// Rel is the vault-relative path of a file inside the vault, or "" when it lies outside.
func (v *Vault) Rel(abs string) string {
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(v.Root, abs)
	}
	rel, err := filepath.Rel(realPath(v.Root), realPath(abs))
	if err != nil || rel == ".." || strings.HasPrefix(rel, "../") {
		// The file may not exist yet; compare without resolving it.
		rel, err = filepath.Rel(v.Root, filepath.Clean(abs))
		if err != nil || rel == ".." || strings.HasPrefix(rel, "../") {
			return ""
		}
	}
	return filepath.ToSlash(rel)
}

// Git is the vault's repository.
func (v *Vault) Git() gitx.Repo { return gitx.Repo{Dir: v.Root} }

// Read reads a vault-relative file.
func (v *Vault) Read(rel string) ([]byte, error) { return os.ReadFile(v.Abs(rel)) }

// Exists reports whether a vault-relative path exists.
func (v *Vault) Exists(rel string) bool {
	_, err := os.Lstat(v.Abs(rel))
	return err == nil
}

// Write writes a vault-relative file atomically, making its folder.
func (v *Vault) Write(rel string, content []byte) error {
	return writeAtomic(v.Abs(rel), content)
}

// WriteIfChanged writes the file only when its content differs, and reports whether it
// wrote.
func (v *Vault) WriteIfChanged(rel string, content []byte) (bool, error) {
	if have, err := v.Read(rel); err == nil && string(have) == string(content) {
		return false, nil
	}
	return true, v.Write(rel, content)
}

// Remove deletes a vault-relative file, and the folders it leaves empty up to the
// layout's own folders.
func (v *Vault) Remove(rel string) error {
	if err := os.Remove(v.Abs(rel)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	dir := path.Dir(rel)
	for dir != "." && dir != "" && !slices.Contains(Folders, dir) {
		if err := os.Remove(v.Abs(dir)); err != nil {
			break
		}
		dir = path.Dir(dir)
	}
	return nil
}

// EnsureFolders makes every folder of the layout that is missing.
func (v *Vault) EnsureFolders() error {
	for _, f := range Folders {
		if err := os.MkdirAll(v.Abs(f), 0o755); err != nil {
			return err
		}
	}
	// The scope callouts once embedded a Base file; they hold the view inline now. The
	// file goes when nobody edited it.
	if data, err := v.Read(oldScopeBase); err == nil && sameYAML(string(data), oldScopeBaseContent) {
		if err := v.Remove(oldScopeBase); err != nil {
			return err
		}
	}
	if err := upgradeBases(v); err != nil {
		return err
	}
	g := v.Git()
	if err := g.Exclude(Excluded...); err != nil {
		return err
	}
	return untrackExcluded(g)
}

// untrackExcluded removes from git the files of Excluded that a vault tracked before
// they were excluded, in a commit of its own. The files stay on disk.
func untrackExcluded(g gitx.Repo) error {
	var files []string
	for _, p := range Excluded {
		if rel, ok := strings.CutPrefix(p, "/"); ok {
			files = append(files, rel)
		}
	}
	removed, err := g.Untrack(files...)
	if err != nil || len(removed) == 0 || !g.HasHead() {
		return err
	}
	_, err = g.Commit("untrack machine files: " + strings.Join(removed, ", "))
	return err
}

// FindAbove is the nearest folder at or above dir that holds a vault document, or "".
func FindAbove(dir string) string {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return ""
	}
	for {
		if isVaultDoc(filepath.Join(dir, Marker)) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// isVaultDoc reads just enough of a file to say whether it is a vault document.
func isVaultDoc(file string) bool {
	data, err := os.ReadFile(file)
	if err != nil {
		return false
	}
	front, _, ok := doc.Split(string(data))
	if !ok {
		return false
	}
	f, err := doc.ParseFront(front)
	return err == nil && f.Str("type") == "vault"
}

// Find resolves the vault of a session: the nearest vault at or above dir; else the one
// vault in the machine file whose repository page holds dir.
func Find(dir string, h Home) (*Vault, error) {
	if root := FindAbove(dir); root != "" {
		return Open(root)
	}
	cfg, err := h.Load()
	if err != nil {
		return nil, err
	}
	for _, root := range cfg.Paths() {
		v, err := Open(root)
		if err != nil {
			continue
		}
		for _, r := range v.Repositories() {
			if r.Path != "" && Within(dir, r.Path) {
				return v, nil
			}
		}
	}
	return nil, fmt.Errorf("%w for %s: no Atlas.md at or above it, and no vault in %s links a repository that holds it", ErrNoVault, dir, h.ConfigPath())
}

// Resolve finds the vault a call names: a path, or a vault's name from the machine file.
// An empty name finds the vault of dir.
func Resolve(name, dir string, h Home) (*Vault, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Find(dir, h)
	}
	if st, err := os.Stat(Expand(name)); err == nil && st.IsDir() {
		return Open(Expand(name))
	}
	cfg, err := h.Load()
	if err != nil {
		return nil, err
	}
	var names []string
	for _, root := range cfg.Paths() {
		v, err := Open(root)
		if err != nil {
			continue
		}
		if strings.EqualFold(v.Name(), name) || strings.EqualFold(filepath.Base(root), name) {
			return v, nil
		}
		names = append(names, v.Name())
	}
	return nil, fmt.Errorf("%w named %q; the machine file lists: %s", ErrNoVault, name, strings.Join(names, ", "))
}

// Repo is what the fast reader knows of one repository page.
type Repo struct {
	ID     string
	Title  string
	Path   string // absolute
	Parent string // the parent's title, or ""
}

// Repositories reads the repository pages' frontmatter only, fast enough for a hook.
func (v *Vault) Repositories() []Repo {
	var out []Repo
	for _, d := range v.readFolder("wiki/repositories") {
		if d.Type() != "repository" {
			continue
		}
		p := d.Str("path")
		if p != "" {
			p = Expand(p)
			if !filepath.IsAbs(p) {
				p = filepath.Join(v.Root, p)
			}
		}
		out = append(out, Repo{ID: d.ID(), Title: d.Title(), Path: p, Parent: doc.LinkTarget(d.Str("parent"))})
	}
	return out
}

// AreaParents maps each area's title, without case, to its parent's title, read from the
// area pages' frontmatter only.
func (v *Vault) AreaParents() map[string]string {
	out := map[string]string{}
	for _, d := range v.readFolder("wiki/areas") {
		if d.Type() == "area" {
			out[strings.ToLower(d.Title())] = doc.LinkTarget(d.Str("parent"))
		}
	}
	return out
}

// readFolder parses the markdown files directly in a folder.
func (v *Vault) readFolder(rel string) []*doc.Doc {
	entries, err := os.ReadDir(v.Abs(rel))
	if err != nil {
		return nil
	}
	var out []*doc.Doc
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
			continue
		}
		p := rel + "/" + e.Name()
		if d, err := cachedDoc(v, p); err == nil {
			out = append(out, d)
		}
	}
	return out
}

// Now is the time format of code-owned times.
const TimeFormat = "2006-01-02T15:04:05"

// DateFormat is the format of created and updated dates.
const DateFormat = "2006-01-02"

// Stamp is t as a code-owned time.
func Stamp(t time.Time) string { return t.Format(TimeFormat) }

// Date is t as a date.
func Date(t time.Time) string { return t.Format(DateFormat) }

// ParseTime reads a code-owned time, a date, or a time without seconds.
func ParseTime(s string) (time.Time, bool) {
	for _, layout := range []string{TimeFormat, "2006-01-02T15:04", DateFormat, time.RFC3339} {
		if t, err := time.ParseInLocation(layout, strings.TrimSpace(s), time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
