package vault

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

// LockWait is how long a write waits for another write to finish.
const LockWait = 15 * time.Second

var (
	processLocksMu sync.Mutex
	processLocks   = map[string]*sync.Mutex{}
)

func processLock(root string) *sync.Mutex {
	processLocksMu.Lock()
	defer processLocksMu.Unlock()
	m := processLocks[root]
	if m == nil {
		m = &sync.Mutex{}
		processLocks[root] = m
	}
	return m
}

// Lock takes .git/atlas.lock for one write: a hook's, a tool's, or sync's. Two processes,
// and two calls in one process, never write the vault at once. The lock is not
// re-entrant; a write takes it once.
func (v *Vault) Lock() (func(), error) {
	m := processLock(v.Root)
	m.Lock()
	dir := v.Git().GitDir()
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		// A vault that is not a repository yet (during init) locks nothing on disk.
		return m.Unlock, nil
	}
	f, err := os.OpenFile(filepath.Join(dir, "atlas.lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		m.Unlock()
		return nil, err
	}
	deadline := time.Now().Add(LockWait)
	for {
		err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			f.Close()
			m.Unlock()
			return nil, fmt.Errorf("another atlas write holds %s; try again", filepath.Join(dir, "atlas.lock"))
		}
		time.Sleep(15 * time.Millisecond)
	}
	return func() {
		syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		f.Close()
		m.Unlock()
	}, nil
}

// Tx is one write that ends in one commit: it holds the lock, starts from a clean tree,
// and commits the paths it wrote.
type Tx struct {
	V       *Vault
	unlock  func()
	written map[string]bool
	// before holds each touched path as the write found it, so a write that ends without
	// its commit puts the vault back.
	before    map[string]*found
	committed bool
	// staged are contents the commit records in place of what the disk holds.
	staged map[string][]byte
	// Snapshot is the commit of the hand edits the write found, or "".
	Snapshot string
}

// found is a path as a write found it: its bytes and mode, or that it did not exist.
type found struct {
	exists bool
	data   []byte
	mode   os.FileMode
}

// Begin takes the lock, runs recover (the repair of a write a crash left half done), and
// commits a dirty tree as a snapshot, so the write's own commit holds only its paths and
// an undo never takes back a hand edit.
func Begin(v *Vault, recover func() error) (*Tx, error) {
	unlock, err := v.Lock()
	if err != nil {
		return nil, err
	}
	tx := &Tx{V: v, unlock: unlock, written: map[string]bool{}, before: map[string]*found{}}
	if err := v.Git().CheckIdle(); err != nil {
		tx.Close()
		return nil, err
	}
	if err := v.EnsureFolders(); err != nil {
		tx.Close()
		return nil, err
	}
	if recover != nil {
		if err := recover(); err != nil {
			tx.Close()
			return nil, err
		}
	}
	sha, err := CommitSnapshot(v)
	if err != nil {
		tx.Close()
		return nil, fmt.Errorf("commit the hand edits before this write: %w; nothing was written, so the vault is as it was", err)
	}
	tx.Snapshot = sha
	return tx, nil
}

// CommitSnapshot commits every hand edit in the tree as one snapshot commit, and returns
// it, or "" when the tree was clean. The caller holds the lock.
func CommitSnapshot(v *Vault) (string, error) {
	g := v.Git()
	entries, err := g.Status()
	if err != nil || len(entries) == 0 {
		return "", err
	}
	if err := g.AddAll(); err != nil {
		return "", err
	}
	if staged, err := g.Staged(); err != nil || !staged {
		return "", err
	}
	n := len(entries)
	noun := "files"
	if n == 1 {
		noun = "file"
	}
	return g.Commit(fmt.Sprintf("snapshot: %d %s edited by hand", n, noun))
}

// Close lets the lock go. A write that ends without its commit, because a step or the
// commit failed, first puts every path it touched back as it found it, and unstages them.
func (tx *Tx) Close() {
	if tx.unlock == nil {
		return
	}
	if !tx.committed && len(tx.before) > 0 {
		tx.rollback()
	}
	tx.unlock()
	tx.unlock = nil
}

// rollback puts each touched path back as the write found it. It runs under the lock.
func (tx *Tx) rollback() {
	paths := make([]string, 0, len(tx.before))
	for rel, f := range tx.before {
		paths = append(paths, rel)
		abs := tx.V.Abs(rel)
		if !f.exists {
			os.Remove(abs)
			continue
		}
		if writeAtomic(abs, f.data) == nil {
			os.Chmod(abs, f.mode)
		}
	}
	sort.Strings(paths)
	tx.V.Prune(paths...)
	tx.V.Git().Unstage(paths...)
}

// Keep records paths as they are now, before another function changes them, so a write
// that ends without its commit can put them back. Each path is kept once, at its first
// touch.
func (tx *Tx) Keep(paths ...string) {
	for _, rel := range paths {
		if _, ok := tx.before[rel]; ok {
			continue
		}
		f := &found{}
		if st, err := os.Lstat(tx.V.Abs(rel)); err == nil && st.Mode().IsRegular() {
			if data, err := os.ReadFile(tx.V.Abs(rel)); err == nil {
				f = &found{exists: true, data: data, mode: st.Mode().Perm()}
			}
		}
		tx.before[rel] = f
	}
}

// Write writes a file and marks it for the commit. Like every write of a Tx, it refuses a
// path that Contain refuses.
func (tx *Tx) Write(rel string, content []byte) error {
	if err := tx.V.Contain(rel); err != nil {
		return err
	}
	tx.Keep(rel)
	tx.written[rel] = true
	return tx.V.Write(rel, content)
}

// WriteIfChanged writes a file only when it differs, and marks it when it wrote.
func (tx *Tx) WriteIfChanged(rel string, content []byte) (bool, error) {
	if err := tx.V.Contain(rel); err != nil {
		return false, err
	}
	tx.Keep(rel)
	wrote, err := tx.V.WriteIfChanged(rel, content)
	if wrote {
		tx.written[rel] = true
	}
	return wrote, err
}

// Remove deletes a file and marks it for the commit.
func (tx *Tx) Remove(rel string) error {
	if err := tx.V.Contain(rel); err != nil {
		return err
	}
	tx.Keep(rel)
	tx.written[rel] = true
	return tx.V.Remove(rel)
}

// Move renames a file and marks both paths.
func (tx *Tx) Move(from, to string) error {
	if err := tx.V.Contain(from); err != nil {
		return err
	}
	if err := tx.V.Contain(to); err != nil {
		return err
	}
	tx.Keep(from, to)
	tx.written[from] = true
	tx.written[to] = true
	if err := os.MkdirAll(filepath.Dir(tx.V.Abs(to)), 0o755); err != nil {
		return err
	}
	if err := os.Rename(tx.V.Abs(from), tx.V.Abs(to)); err != nil {
		return err
	}
	if strings.EqualFold(from, to) {
		// A new case of one name: on a disk that ignores case, from is still the file.
		return nil
	}
	return tx.V.Remove(from)
}

// Mark adds paths another function wrote to the commit. Call Keep on them before that
// function writes, so a failed write can put them back.
func (tx *Tx) Mark(paths ...string) {
	for _, p := range paths {
		tx.written[p] = true
	}
}

// Stage makes the commit record content for rel in place of what the disk holds. Apply
// commits a change document as applied while the file still marks the apply in flight.
func (tx *Tx) Stage(rel string, content []byte) {
	if tx.staged == nil {
		tx.staged = map[string][]byte{}
	}
	tx.Mark(rel)
	tx.staged[rel] = content
}

// Paths are the paths the write marked, sorted.
func (tx *Tx) Paths() []string {
	out := make([]string, 0, len(tx.written))
	for p := range tx.written {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// Commit stages the marked paths and commits them with the subject and trailers
// ("Key: value"). It returns "" when nothing changed.
func (tx *Tx) Commit(subject string, trailers ...string) (string, error) {
	sha, err := tx.commit(subject, trailers)
	if err != nil {
		return "", fmt.Errorf("%w; nothing was saved: the vault is back as it was before this call", err)
	}
	tx.committed = true
	return sha, nil
}

func (tx *Tx) commit(subject string, trailers []string) (string, error) {
	g := tx.V.Git()
	if err := g.Add(tx.Paths()...); err != nil {
		return "", err
	}
	for rel, data := range tx.staged {
		if err := g.StageContent(rel, data); err != nil {
			return "", err
		}
	}
	staged, err := g.Staged()
	if err != nil || !staged {
		return "", err
	}
	msg := subject
	if len(trailers) > 0 {
		msg += "\n\n" + strings.Join(trailers, "\n")
	}
	return g.Commit(msg)
}
