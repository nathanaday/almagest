// Package testvault builds scratch vaults and scratch repositories for tests. Nothing
// here touches the real ~/.atlas: every vault gets its own machine folder.
package testvault

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/gitx"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

// Now is the fixed time tests run at.
var Now = time.Date(2026, 9, 27, 14, 32, 0, 0, time.Local)

// T is a scratch machine: its machine folder, one vault, and a folder for repositories.
type T struct {
	t     *testing.T
	Dir   string
	Home  vault.Home
	V     *vault.Vault
	Code  string
	ids   map[string]string
	Clock time.Time
}

// New makes a scratch vault named Work. It skips the test when git is missing.
func New(t *testing.T) *T {
	t.Helper()
	if !gitx.Available() {
		t.Skip("git is not installed")
	}
	dir := t.TempDir()
	real, err := filepath.EvalSymlinks(dir)
	if err == nil {
		dir = real
	}
	h := vault.Home{Root: filepath.Join(dir, "home")}
	t.Setenv(vault.EnvHome, h.Root)
	t.Setenv("GIT_AUTHOR_NAME", "test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")
	v, err := vault.Init(vault.InitOptions{Path: filepath.Join(dir, "work"), Name: "Work", Description: "Work notes: the p3 product.", Tagging: "open"}, h, Now)
	if err != nil {
		t.Fatal(err)
	}
	code := filepath.Join(dir, "code")
	if err := os.MkdirAll(code, 0o755); err != nil {
		t.Fatal(err)
	}
	return &T{t: t, Dir: dir, Home: h, V: v, Code: code, ids: map[string]string{}, Clock: Now}
}

// Tick moves the clock on and returns the new time.
func (tv *T) Tick(d time.Duration) time.Time {
	tv.Clock = tv.Clock.Add(d)
	return tv.Clock
}

// Repo makes a git repository with a README, one commit, and the files given.
func (tv *T) Repo(name string, files map[string]string) string {
	tv.t.Helper()
	dir := filepath.Join(tv.Code, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		tv.t.Fatal(err)
	}
	g := gitx.Repo{Dir: dir}
	if err := g.Init(); err != nil {
		tv.t.Fatal(err)
	}
	all := map[string]string{"README.md": "# " + name + "\n\nThe " + name + " service.\n"}
	for k, v := range files {
		all[k] = v
	}
	for rel, content := range all {
		tv.WriteFile(filepath.Join(dir, rel), content)
	}
	if err := g.AddAll(); err != nil {
		tv.t.Fatal(err)
	}
	if _, err := g.Commit("first"); err != nil {
		tv.t.Fatal(err)
	}
	return dir
}

// WriteFile writes an absolute file, making its folder.
func (tv *T) WriteFile(abs, content string) {
	tv.t.Helper()
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		tv.t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		tv.t.Fatal(err)
	}
}

// Write writes a vault-relative file.
func (tv *T) Write(rel, content string) {
	tv.t.Helper()
	tv.WriteFile(tv.V.Abs(rel), content)
}

// Read reads a vault-relative file.
func (tv *T) Read(rel string) string {
	tv.t.Helper()
	data, err := tv.V.Read(rel)
	if err != nil {
		tv.t.Fatal(err)
	}
	return string(data)
}

// Doc writes a typed document straight to wiki/documents, the way a hand edit or an
// earlier write would leave it, and returns its id. fields may name links by title; a
// field set to nil is left out. A document without a description gets one.
func (tv *T) Doc(typ, title string, fields map[string]any, body string) string {
	tv.t.Helper()
	id := doc.NewID("doc", nil)
	if s, ok := fields["id"].(string); ok {
		id = s
	}
	stamp := vault.Stamp(Now)
	list := []doc.Field{{Key: "id", Value: id}, {Key: "type", Value: typ}}
	if _, ok := fields["description"]; !ok {
		list = append(list, doc.Field{Key: "description", Value: "The " + title + " document."})
	}
	list = append(list, doc.Field{Key: "created", Value: stamp}, doc.Field{Key: "updated", Value: stamp})
	keys := make([]string, 0, len(fields))
	for k := range fields {
		if k != "id" && fields[k] != nil {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	for _, k := range keys {
		list = append(list, doc.Field{Key: k, Value: fields[k]})
	}
	tv.Write(vault.DocPath(title), doc.Render(list, body))
	tv.ids[title] = id
	return id
}

// ID is the id Page gave a title.
func (tv *T) ID(title string) string { return tv.ids[title] }

// Commit commits the vault's tree as a hand edit.
func (tv *T) Commit() {
	tv.t.Helper()
	unlock, err := tv.V.Lock()
	if err != nil {
		tv.t.Fatal(err)
	}
	defer unlock()
	if _, err := vault.CommitSnapshot(tv.V); err != nil {
		tv.t.Fatal(err)
	}
}

// Log lists the vault's commit subjects, newest first.
func (tv *T) Log() []string {
	tv.t.Helper()
	commits, err := tv.V.Git().Log(0)
	if err != nil {
		tv.t.Fatal(err)
	}
	out := make([]string, len(commits))
	for i, c := range commits {
		out[i] = c.Subject
	}
	return out
}

// Clean fails the test when the vault's tree has changes git would commit.
func (tv *T) Clean() {
	tv.t.Helper()
	entries, err := tv.V.Git().Status()
	if err != nil {
		tv.t.Fatal(err)
	}
	if len(entries) > 0 {
		var paths []string
		for _, e := range entries {
			paths = append(paths, e.Code+" "+e.Path)
		}
		tv.t.Fatalf("the tree is dirty: %s", strings.Join(paths, ", "))
	}
}

// Index loads the vault.
func (tv *T) Index() *vault.Index {
	tv.t.Helper()
	idx, err := vault.Load(tv.V)
	if err != nil {
		tv.t.Fatal(err)
	}
	return idx
}
