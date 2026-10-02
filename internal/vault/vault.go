// Package vault is one vault on disk: its layout, the vault document Atlas.md, the machine
// file that lists every vault, the lock every write takes, and the index of every document
// the other packages read.
package vault

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/gitx"
	"github.com/nathanaday/atlas-obsidian/internal/schema"
)

// The layout, relative to the vault.
const (
	Marker     = "Atlas.md"
	Inbox      = "inbox"
	Scratchpad = "scratchpad"
	Sessions   = "sessions"
	Changes    = "changes"
	Chords     = "chords"
	Wiki       = "wiki"
	Documents  = "wiki/documents"
	Assets     = "wiki/assets"
	Views      = "views"
	Settings   = ".claude/settings.local.json"
	Obsidian   = ".obsidian"
	PluginDir  = ".obsidian/plugins/atlas"
	AppJSON    = ".obsidian/app.json"
)

// Layout is the layout version this binary reads and writes, kept in Atlas.md's layout
// field: 4 is the threads and chords of 8.0; 3 is the flat wiki/documents of 7.0, with
// plans; below that is a 6.x vault. Only the migrate command writes a vault of an older
// layout.
const (
	Layout     = 4
	LayoutFlat = 3
)

// Folders are every folder of the layout. EnsureFolders makes the ones a clone left out,
// because git keeps no empty folder.
var Folders = []string{Inbox, Scratchpad, Sessions, Changes, Chords, Wiki, Documents, Assets, Views}

// Excluded are the patterns kept out of the vault's history on each machine: the views,
// which code derives; the harness settings, which hold this machine's paths; and the
// Obsidian files it rewrites on every click and zoom, and the plugin on every change.
var Excluded = []string{"/views/", "/.claude/settings.local.json", "/.obsidian/workspace.json", "/.obsidian/workspace-mobile.json", "/.obsidian/graph.json", ".DS_Store", ".atlas-*"}

// Defaults of the vault document.
var (
	DefaultWikify     = []string{"source", "spec", "verification", "chord", "event"}
	DefaultStaleHours = 12
	TaggingModes      = []string{"open", "known"}
)

// Reserved title prefixes belong to the view notes; no document takes one.
var ReservedPrefixes = []string{"Tag · ", "View · "}

// ReservedTitle reports whether a title begins with a prefix the views own.
func ReservedTitle(title string) bool {
	for _, p := range ReservedPrefixes {
		if strings.HasPrefix(strings.TrimSpace(title), p) {
			return true
		}
	}
	return false
}

// DocPath is where a document of wiki/documents with a title lives.
func DocPath(title string) string { return Documents + "/" + title + ".md" }

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

// Tagging is how freely the agent adds a tag: open, or known (only tags some document
// holds, unless the user agreed to a new one).
func (v *Vault) Tagging() string {
	if t := v.Doc.Str("tagging"); slices.Contains(TaggingModes, t) {
		return t
	}
	return "open"
}

// LayoutVersion is the layout the vault document records; a vault without the field is
// a vault of the 6.x layouts.
func (v *Vault) LayoutVersion() int { return v.Doc.Front.Int("layout") }

// ErrLegacy is the refusal of every write on a vault of an older layout.
var ErrLegacy = errors.New("this vault has the layout of an earlier release; run `atlas-obsidian vault migrate --dry-run` to see the move to 8.0, then `atlas-obsidian vault migrate` (type it yourself, or with ! in a session)")

// CheckLayout refuses a vault whose layout this binary does not write.
func (v *Vault) CheckLayout() error {
	if v.LayoutVersion() < Layout {
		return ErrLegacy
	}
	return nil
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

// ErrOutside is the refusal of a path that does not stay inside the vault.
var ErrOutside = errors.New("not a path inside the vault")

// MaxNameBytes is the longest file name the file systems Atlas runs on accept.
const MaxNameBytes = 255

// Contain refuses a vault-relative path that does not stay inside the vault: an absolute
// or unclean path, one that climbs with "..", one under .git in any case, a name longer
// than a file system takes, and one whose folders or final link resolve outside the
// vault. A path that does not exist yet is judged by its nearest folder that does.
func (v *Vault) Contain(rel string) error {
	refuse := func(why string) error {
		return fmt.Errorf("%q: %w: %s; give a clean vault-relative path such as %s", rel, ErrOutside, why, DocPath("Title"))
	}
	if rel == "" || !filepath.IsLocal(rel) || path.Clean(rel) != rel || strings.Contains(rel, `\`) {
		return refuse("it is absolute, empty, unclean, or climbs out with ..")
	}
	for i, part := range strings.Split(rel, "/") {
		if i == 0 && strings.EqualFold(part, ".git") {
			return refuse("it lies in the vault's .git folder")
		}
		if len(part) > MaxNameBytes {
			return refuse(fmt.Sprintf("the name %.40q… has %d bytes, and a file name holds at most %d", part, len(part), MaxNameBytes))
		}
	}
	abs := v.Abs(rel)
	if st, err := os.Lstat(abs); err == nil && st.Mode()&fs.ModeSymlink != 0 {
		real, err := filepath.EvalSymlinks(abs)
		if err != nil || !Within(real, v.Root) {
			return refuse("it is a link that leads out of the vault")
		}
		return nil
	}
	dir := filepath.Dir(abs)
	for {
		if _, err := os.Lstat(dir); err == nil {
			if !Within(dir, v.Root) {
				return refuse("a folder on its way is a link that leads out of the vault")
			}
			return nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return refuse("no folder of it exists")
		}
		dir = parent
	}
}

// Local reports whether a vault-relative path names a document a change may write: a .md
// path that Contain accepts, outside the machine folders .git, .obsidian, and .claude in
// any case.
func (v *Vault) Local(rel string) bool {
	if !strings.HasSuffix(rel, ".md") || v.Contain(rel) != nil {
		return false
	}
	top, _, _ := strings.Cut(rel, "/")
	return !strings.EqualFold(top, ".git") && !strings.EqualFold(top, Obsidian) && !strings.EqualFold(top, ".claude")
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

// Prune removes the folders that paths gone from disk leave empty, up to the layout's
// own folders.
func (v *Vault) Prune(paths ...string) {
	for _, p := range paths {
		if !v.Exists(p) {
			v.Remove(p)
		}
	}
}

// EnsureFolders makes every folder of the layout that is missing, replaces a Base of an
// earlier release that nobody edited, and keeps the machine files out of git.
func (v *Vault) EnsureFolders() error {
	for _, f := range Folders {
		if err := os.MkdirAll(v.Abs(f), 0o755); err != nil {
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
			files = append(files, strings.TrimSuffix(rel, "/"))
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
	f, err := ReadFront(file)
	return err == nil && f.Str("type") == "vault"
}

// Find resolves the vault of a session: the nearest vault at or above dir; else the one
// vault in the machine file whose repository document holds dir.
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

// Repo is what the fast reader knows of one repository document.
type Repo struct {
	ID      string
	Title   string
	Path    string // absolute; "" when the repository is unlinked
	Defines string
	Tags    []string
}

// Repositories reads the frontmatter of the repository documents only, fast enough for a
// hook. It reads every markdown file under wiki/, so a 6.x vault, whose repository pages
// lie in folders, is read the same way.
func (v *Vault) Repositories() []Repo {
	var out []Repo
	filepath.WalkDir(v.Abs(Wiki), func(abs string, e fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if e.IsDir() {
			if strings.HasPrefix(e.Name(), ".") || abs == v.Abs(Assets) || abs == v.Abs("wiki/sources/files") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
			return nil
		}
		f, err := ReadFront(abs)
		if err != nil || f.Str("type") != "repository" {
			return nil
		}
		p := f.Str("path")
		if f.Bool("unlinked") {
			p = ""
		}
		if p != "" {
			p = Expand(p)
			if !filepath.IsAbs(p) {
				p = filepath.Join(v.Root, p)
			}
		}
		out = append(out, Repo{ID: f.Str("id"), Title: doc.TitleOf(e.Name()), Path: p, Defines: f.Str("defines"), Tags: f.List("tags")})
		return nil
	})
	return out
}

// ReadFront reads a file's frontmatter only: the lines up to the closing fence.
func ReadFront(file string) (*doc.Front, error) {
	fh, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	r := bufio.NewReader(fh)
	first, err := r.ReadString('\n')
	if strings.TrimRight(first, "\r\n") != "---" {
		return nil, errors.New("no frontmatter")
	}
	var b bytes.Buffer
	for {
		line, err := r.ReadString('\n')
		t := strings.TrimRight(line, "\r\n")
		if t == "---" || t == "..." {
			return doc.ParseFront(b.String())
		}
		b.WriteString(line)
		if err != nil {
			return nil, errors.New("frontmatter does not close")
		}
	}
}

// TimeFormat is the format of code-owned times.
const TimeFormat = "2006-01-02T15:04:05"

// DateFormat is the format of a day.
const DateFormat = "2006-01-02"

// Stamp is t as a code-owned time.
func Stamp(t time.Time) string { return t.Format(TimeFormat) }

// Date is t as a day.
func Date(t time.Time) string { return t.Format(DateFormat) }

// ParseTime reads a code-owned time, a date, or a time without seconds.
func ParseTime(s string) (time.Time, bool) { return schema.ParseTime(s) }
