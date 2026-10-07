// Package vault is one vault on disk: its layout, the vault document Almagest.md, the machine
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

	"github.com/nathanaday/almagest/internal/doc"
	"github.com/nathanaday/almagest/internal/gitx"
	"github.com/nathanaday/almagest/internal/schema"
)

// The layout, relative to the vault.
const (
	Marker     = "Almagest.md"
	Ingest     = "ingest"
	Scratchpad = "scratchpad"
	Changes    = "changes"
	// Tool holds what Almagest keeps for itself, which the user need not open: the store
	// of the wiki, the session records, and the trash.
	Tool     = "tool"
	Sessions = "tool/sessions"
	// Core is the store of the wiki: the documents, and the originals they describe.
	Core      = "tool/source-core"
	Documents = "tool/source-core/documents"
	Originals = "tool/source-core/originals"
	// WikiView is the reading layer that code writes from the documents.
	WikiView = "wiki-view"
	// Journals holds the user's own writing, one volume per folder; agents never change it.
	Journals = "journals"
	// Checkout holds the librarian's copies of documents, and its ledger.
	Checkout = "checkout"
	// Trash holds what safe delete removed; the user empties it.
	Trash     = "tool/trash"
	Settings  = ".claude/settings.local.json"
	Obsidian  = ".obsidian"
	PluginDir = ".obsidian/plugins/almagest"
	// PluginLink opens the Obsidian plugin in Obsidian's community plugins, where the user
	// installs it. No tool installs it in a vault.
	PluginLink = "obsidian://show-plugin?id=almagest"
	AppJSON    = ".obsidian/app.json"
)

// Layout is the version of the vault's layout that this binary reads and writes, kept in
// Almagest.md's layout field.
const Layout = 8

// Folders are the folders every vault has. EnsureFolders makes the ones a clone left
// out, because git keeps no empty folder. Checkout and Trash appear when first used.
var Folders = []string{Ingest, Scratchpad, Changes, Tool, Sessions, Core, Documents, Originals, WikiView, Journals}

// Excluded are the patterns kept out of the vault's history on each machine: the views,
// which code derives; the harness settings, which hold this machine's paths; and the
// Obsidian files it rewrites on every click and zoom, and the plugin on every change.
var Excluded = []string{"/wiki-view/", "/.claude/settings.local.json", "/.obsidian/workspace.json", "/.obsidian/workspace-mobile.json", "/.obsidian/graph.json", ".DS_Store", ".almagest-*"}

// Defaults of the vault document.
var (
	DefaultStaleHours = 12
	TaggingModes      = []string{"open", "known"}
)

// Reserved title prefixes belong to the view notes; no document takes one.
var ReservedPrefixes = []string{"Tag · ", "View · ", "Checkout · ", "Journal · "}

// ReservedTitle reports whether a title begins with a prefix the views own.
func ReservedTitle(title string) bool {
	for _, p := range ReservedPrefixes {
		if strings.HasPrefix(strings.TrimSpace(title), p) {
			return true
		}
	}
	return false
}

// TrashPath is where the trash keeps a file of the vault removed now: under the day, at
// its vault path, with a number before the extension when that place is taken on disk or
// in taken, which it marks.
func TrashPath(v *Vault, rel string, now time.Time, taken map[string]bool) string {
	ext := path.Ext(rel)
	base := Trash + "/" + Date(now) + "/" + strings.TrimSuffix(rel, ext)
	out := base + ext
	for n := 2; v.Exists(out) || taken[strings.ToLower(out)]; n++ {
		out = fmt.Sprintf("%s (%d)%s", base, n, ext)
	}
	taken[strings.ToLower(out)] = true
	return out
}

// DocPath is where a document with a title lives.
func DocPath(title string) string { return Documents + "/" + title + ".md" }

// Vault is one vault: its folder and the settings of its vault document.
type Vault struct {
	Root string
	Doc  *doc.Doc
}

// ErrNoVault is returned when no vault holds a folder.
var ErrNoVault = errors.New("no vault")

// Open reads the vault at root. The folder must hold an Almagest.md of type vault.
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

// IsMarker reports whether rel names the vault document, in any case.
func IsMarker(rel string) bool { return strings.EqualFold(rel, Marker) }

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

// LayoutVersion is the layout the vault document records.
func (v *Vault) LayoutVersion() int { return v.Doc.Front.Int("layout") }

// LayoutBeforeTool is the layout before tool/ held sessions/, source-core/, and trash/;
// almagest vault migrate takes a vault of it to Layout.
const LayoutBeforeTool = 7

// ErrLayout is the refusal of a vault whose layout this binary does not read.
var ErrLayout = errors.New("this vault's layout is not the one this almagest reads; update the almagest plugin (claude plugin update almagest@nathanaday-almagest), then start a new session")

// ErrMigrate is the refusal of a vault that keeps sessions/, source-core/, and trash/ at its
// root. Only the user migrates it: the guard refuses the command to an agent.
var ErrMigrate = errors.New("this vault keeps sessions/, source-core/, and trash/ at its root, and this almagest keeps them in tool/; the user runs almagest vault migrate in the vault, which moves them in one commit, then starts a new session")

// CheckLayout refuses a vault of another layout than this binary's.
func (v *Vault) CheckLayout() error {
	switch v.LayoutVersion() {
	case Layout:
		return nil
	case LayoutBeforeTool:
		return fmt.Errorf("%w (vault layout %d, almagest reads %d)", ErrMigrate, v.LayoutVersion(), Layout)
	}
	return fmt.Errorf("%w (vault layout %d, almagest reads %d)", ErrLayout, v.LayoutVersion(), Layout)
}

// StaleHours is how long a live session may go without a hook event before it is lost.
func (v *Vault) StaleHours() int {
	if n := v.Doc.Front.Int("stale_hours"); n > 0 {
		return n
	}
	return DefaultStaleHours
}

// Context is the body of Almagest.md: the context every agent in the vault must know.
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

// MaxNameBytes is the longest file name the file systems Almagest runs on accept.
const MaxNameBytes = 255

// Contain refuses a vault-relative path that does not stay inside the vault: an absolute
// or unclean path, one that climbs with "..", one under .git in any case, a name longer
// than a file system takes, and one whose folders or final link resolve outside the
// vault. A path that does not exist yet is judged by its nearest folder that does.
func (v *Vault) Contain(rel string) error {
	refuse := func(why string) error {
		return fmt.Errorf("%q: %w: %s; give a clean vault-relative path such as %s", rel, ErrOutside, why, DocPath("Title"))
	}
	// A path through a link is code's or the caller's, but the fix is the user's: the
	// refusal names the link, not a path to give instead.
	linked := func() error {
		at := v.linkOut(rel)
		return fmt.Errorf("%q: %w: %s is a link that leads out of the vault; make %s a plain folder or file inside the vault", rel, ErrOutside, at, at)
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
			return linked()
		}
		return nil
	}
	dir := filepath.Dir(abs)
	for {
		if _, err := os.Lstat(dir); err == nil {
			if !Within(dir, v.Root) {
				return linked()
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

// linkOut is the first part of rel, from the top, that is a link whose target lies
// outside the vault; rel when it finds none.
func (v *Vault) linkOut(rel string) string {
	parts := strings.Split(rel, "/")
	for i := 1; i <= len(parts); i++ {
		at := strings.Join(parts[:i], "/")
		st, err := os.Lstat(v.Abs(at))
		if err != nil {
			break
		}
		if st.Mode()&fs.ModeSymlink == 0 {
			continue
		}
		if real, err := filepath.EvalSymlinks(v.Abs(at)); err != nil || !Within(real, v.Root) {
			return at
		}
	}
	return rel
}

// InFolder reports whether a vault-relative path is the folder dir or lies under it, in
// any case.
func InFolder(rel, dir string) bool {
	key, d := strings.ToLower(rel), strings.ToLower(dir)
	return key == d || strings.HasPrefix(key, d+"/")
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

// IngestFile resolves a file the caller names in ingest/ (with or without the folder) to
// its vault-relative path. It refuses anything but a regular file whose real path lies in
// ingest/: a folder, a link, or a file behind a folder that links out.
func (v *Vault) IngestFile(name string) (string, error) {
	rel := path.Join(Ingest, strings.TrimPrefix(path.Clean("/"+strings.TrimPrefix(name, Ingest+"/")), "/"))
	missing := fmt.Errorf("ingest: %s is not a file in ingest/; vault status lists what waits there", name)
	if rel == Ingest {
		return "", missing
	}
	st, err := os.Lstat(v.Abs(rel))
	switch {
	case err != nil:
		return "", missing
	case st.Mode()&fs.ModeSymlink != 0:
		return "", fmt.Errorf("ingest: %s is a link; capture takes only a file kept in ingest/, so copy the file there", name)
	case !st.Mode().IsRegular():
		return "", missing
	}
	if real, err := filepath.EvalSymlinks(v.Abs(rel)); err != nil || !Within(real, v.Abs(Ingest)) {
		return "", fmt.Errorf("ingest: %s lies behind a folder that links out of ingest/; capture takes only a file kept in ingest/, so copy the file there", name)
	}
	if err := v.Contain(rel); err != nil {
		return "", fmt.Errorf("ingest: %w", err)
	}
	return rel, nil
}

// Occupied reports whether a file already sits at rel on disk, under any case or Unicode
// form the file system folds to it, and is none of the files at allowed. A create or a
// rename asks it, since the index compares titles only by lower case.
func (v *Vault) Occupied(rel string, allowed ...string) bool {
	st, err := os.Lstat(v.Abs(rel))
	if err != nil {
		return false
	}
	for _, a := range allowed {
		if a == "" {
			continue
		}
		if ast, err := os.Lstat(v.Abs(a)); err == nil && os.SameFile(st, ast) {
			return false
		}
	}
	return true
}

// OnDisk is the vault-relative path under which the file system stores the file at rel,
// which may differ from rel in case or Unicode form; rel when it finds no other.
func (v *Vault) OnDisk(rel string) string {
	st, err := os.Lstat(v.Abs(rel))
	if err != nil {
		return rel
	}
	entries, err := os.ReadDir(filepath.Dir(v.Abs(rel)))
	if err != nil {
		return rel
	}
	for _, e := range entries {
		info, err := os.Lstat(filepath.Join(filepath.Dir(v.Abs(rel)), e.Name()))
		if err == nil && os.SameFile(st, info) {
			return path.Join(path.Dir(rel), e.Name())
		}
	}
	return rel
}

// Spelled is rel as the disk spells each of its parts, which may differ from rel in case
// or Unicode form on a disk that folds them. A part that does not exist keeps its own
// spelling.
func (v *Vault) Spelled(rel string) string {
	out := ""
	for _, p := range strings.Split(rel, "/") {
		out = v.OnDisk(path.Join(out, p))
	}
	return out
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

// Write writes a vault-relative file atomically, making its folder. It refuses a path
// that Contain refuses, so no caller can write outside the vault by forgetting a check.
func (v *Vault) Write(rel string, content []byte) error {
	if err := v.Contain(rel); err != nil {
		return err
	}
	return writeAtomic(v.Abs(rel), content)
}

// WriteIfChanged writes the file only when its content differs, and reports whether it
// wrote. Like Write, it refuses a path that Contain refuses, before it reads.
func (v *Vault) WriteIfChanged(rel string, content []byte) (bool, error) {
	if err := v.Contain(rel); err != nil {
		return false, err
	}
	if have, err := v.Read(rel); err == nil && string(have) == string(content) {
		return false, nil
	}
	return true, writeAtomic(v.Abs(rel), content)
}

// Remove deletes a vault-relative file, and the folders it leaves empty up to the
// layout's own folders.
func (v *Vault) Remove(rel string) error {
	if err := v.Contain(rel); err != nil {
		return err
	}
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

// EnsureFolders makes every folder of the layout that is missing, and keeps the machine
// files out of git.
func (v *Vault) EnsureFolders() error {
	for _, f := range Folders {
		if err := os.MkdirAll(v.Abs(f), 0o755); err != nil {
			return err
		}
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
	return nil, fmt.Errorf("%w for %s: no Almagest.md at or above it, and no vault in %s links a repository that holds it", ErrNoVault, dir, h.ConfigPath())
}

// EnvVault names the variable that chooses the vault when a call names none.
const EnvVault = "ALMAGEST_VAULT"

// Select is the one rule that chooses a vault for the CLI, the MCP server, and the hooks:
// the vault the call names, else the vault $ALMAGEST_VAULT names (a path or a name from the
// machine file), else the vault above dir.
func Select(name, dir string, h Home, envVault string) (*Vault, error) {
	if strings.TrimSpace(name) == "" && strings.TrimSpace(envVault) != "" {
		v, err := Resolve(envVault, dir, h)
		if err != nil {
			return nil, fmt.Errorf("%s=%s: %w; unset it, or set it to a vault's folder or name", EnvVault, envVault, err)
		}
		return v, nil
	}
	return Resolve(name, dir, h)
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
// hook.
func (v *Vault) Repositories() []Repo {
	var out []Repo
	filepath.WalkDir(v.Abs(Documents), func(abs string, e fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if e.IsDir() {
			if abs != v.Abs(Documents) {
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
	// A read error leaves first short of the opening line, which the check refuses.
	first, _ := r.ReadString('\n')
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

// The layouts of times that code writes for a reader or in a file name.
const (
	MonthFormat      = "2006-01"
	ClockFormat      = "15:04"
	MinuteFormat     = "2006-01-02 15:04"
	SecondFormat     = "2006-01-02 15:04:05"
	FileMinuteFormat = "2006-01-02 1504"
	FileSecondFormat = "2006-01-02 150405"
)

// Stamp is t as a code-owned time.
func Stamp(t time.Time) string { return t.Format(schema.TimeFormat) }

// Date is t as a day.
func Date(t time.Time) string { return t.Format(schema.DateFormat) }

// Day is a stored time as a day, or the stamp as given when it does not parse.
func Day(stamp string) string {
	if t, ok := schema.ParseTime(stamp); ok {
		return Date(t)
	}
	return stamp
}

// ErrChangedSince is a guarded write's refusal of a file whose bytes changed after the
// write read it.
var ErrChangedSince = errors.New("the file changed after the write read it")

// WriteIfUnchanged writes a file only while it holds want: it compares before it writes,
// and again after the temporary file is synced, right before the rename. A file that
// holds other bytes gives ErrChangedSince and stays as it is.
func (v *Vault) WriteIfUnchanged(rel string, content []byte, want string) (bool, error) {
	if err := v.Contain(rel); err != nil {
		return false, err
	}
	same := func() bool {
		disk, err := v.Read(rel)
		return err == nil && string(disk) == want
	}
	if !same() {
		return false, ErrChangedSince
	}
	if want == string(content) {
		return false, nil
	}
	wrote, err := writeAtomicIf(v.Abs(rel), content, same)
	if err == nil && !wrote {
		return false, ErrChangedSince
	}
	return wrote, err
}

// unchangedWriter is what a Guard writes through: the vault itself, or a Tx.
type unchangedWriter interface {
	WriteIfChanged(rel string, content []byte) (bool, error)
	WriteIfUnchanged(rel string, content []byte, want string) (bool, error)
}

// Guard writes derived content only while each file still holds the bytes the write read:
// a document's bytes as the index loaded them. A file saved in between, by Obsidian or by an agent's Edit, keeps the save; the
// guard records it in Skipped, and the next sync derives it again.
type Guard struct {
	idx     *Index
	w       unchangedWriter
	Skipped []string
}

// NewGuard guards the writes of one sync step through w, against what idx read.
func NewGuard(idx *Index, w unchangedWriter) *Guard {
	return &Guard{idx: idx, w: w}
}

// Write is the guarded write; its signature fits the writers of derive.
func (g *Guard) Write(rel string, content []byte) (bool, error) {
	d := g.idx.ByPath(rel)
	if d == nil {
		return g.w.WriteIfChanged(rel, content)
	}
	wrote, err := g.w.WriteIfUnchanged(rel, content, d.Content)
	if errors.Is(err, ErrChangedSince) {
		g.Skipped = append(g.Skipped, rel)
		return false, nil
	}
	return wrote, err
}

// Moved is a note code moved to keep it: from where it was to where it went.
type Moved struct {
	From string `json:"from"`
	To   string `json:"to"`
}
