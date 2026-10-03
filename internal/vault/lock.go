package vault

import (
	"errors"
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
	before map[string]*found
	// after holds what the write last left at each path, so rollback can tell a save
	// that landed since from the write's own bytes.
	after      map[string]*found
	committed  bool
	rolledBack bool
	// added is set once the commit staged the paths, so rollback knows to unstage them.
	added bool
	// all makes the commit stage the whole tree, for a write that changes too many paths
	// to mark (the migration).
	all bool
	// saved are the paths rollback left as saved, because they changed after the write.
	saved []string
	// staged are contents the commit records in place of what the disk holds.
	staged map[string][]byte
	// Snapshot is the commit of the hand edits the write found, or "".
	Snapshot string
}

// found is a path as a write found it: a file with its bytes and mode, a link with its
// target, or nothing.
type found struct {
	exists bool
	data   []byte
	mode   os.FileMode
	link   string
}

// Begin takes the lock, runs recover (the repair of a write a crash left half done), and
// commits a dirty tree as a snapshot, so the write's own commit holds only its paths and
// an undo never takes back a hand edit.
func Begin(v *Vault, recover func() error) (*Tx, error) {
	unlock, err := v.Lock()
	if err != nil {
		return nil, err
	}
	tx := &Tx{V: v, unlock: unlock, written: map[string]bool{}, before: map[string]*found{}, after: map[string]*found{}}
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

// Close lets the lock go. A write that ends without its commit, because a step failed,
// first puts every path it touched back as it found it, and unstages them. (A failed
// commit has put them back already, and said how it went.)
func (tx *Tx) Close() {
	if tx.unlock == nil {
		return
	}
	if !tx.committed && !tx.rolledBack && len(tx.before) > 0 {
		tx.rollback()
	}
	tx.unlock()
	tx.unlock = nil
}

// End closes a write and, when it failed before its commit, rolls it back and adds what
// the rollback did to the error. A write defers it with its error result: defer
// tx.End(&err).
func (tx *Tx) End(errp *error) {
	if tx.unlock == nil {
		return
	}
	if errp != nil && *errp != nil && !tx.committed && !tx.rolledBack && len(tx.before) > 0 {
		*errp = tx.outcome(*errp, tx.rollback())
	}
	tx.Close()
}

// outcome is the error of a failed write with what its rollback did.
func (tx *Tx) outcome(err error, failed []string) error {
	kept := ""
	if len(tx.saved) > 0 {
		kept = "; left as saved, since they changed after this call wrote them: " + strings.Join(tx.saved, ", ")
	}
	if len(failed) > 0 {
		return fmt.Errorf("%s; nothing was saved, and these could not be put back as they were: %s%s", firstLine(err), strings.Join(failed, "; "), kept)
	}
	return fmt.Errorf("%s; nothing was saved: the vault is back as it was before this call%s", firstLine(err), kept)
}

// rollback puts each touched path back as the write found it, and returns the paths it
// could not put back. It runs under the lock.
func (tx *Tx) rollback() []string {
	tx.rolledBack = true
	var failed []string
	paths := make([]string, 0, len(tx.before))
	for rel := range tx.before {
		paths = append(paths, rel)
	}
	sort.Strings(paths)
	for _, rel := range paths {
		if a := tx.after[rel]; a != nil && !tx.holds(rel, a) {
			// Someone saved the path after this write wrote it: the save stays.
			tx.saved = append(tx.saved, rel)
			continue
		}
		if err := tx.putBack(rel, tx.before[rel]); err != nil {
			failed = append(failed, fmt.Sprintf("%s (%v)", rel, firstLine(err)))
		}
	}
	tx.V.Prune(paths...)
	if tx.added && tx.all {
		// The write began from a clean index, so the whole index goes back to HEAD.
		if err := tx.V.Git().Unstage("."); err != nil {
			failed = append(failed, fmt.Sprintf("the index (%v)", firstLine(err)))
		}
	} else if tx.added {
		if err := tx.V.Git().Unstage(paths...); err != nil {
			failed = append(failed, fmt.Sprintf("the index entries of %s (%v)", strings.Join(paths, ", "), firstLine(err)))
		}
	}
	return failed
}

// holds reports whether a path is still what the write left there.
func (tx *Tx) holds(rel string, a *found) bool {
	data, err := os.ReadFile(tx.V.Abs(rel))
	if !a.exists {
		return errors.Is(err, os.ErrNotExist)
	}
	return err == nil && string(data) == string(a.data)
}

// putBack makes one path what it was.
func (tx *Tx) putBack(rel string, f *found) error {
	abs := tx.V.Abs(rel)
	if err := os.Remove(abs); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	switch {
	case f.link != "":
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return err
		}
		return os.Symlink(f.link, abs)
	case f.exists:
		if err := writeAtomic(abs, f.data); err != nil {
			return err
		}
		return os.Chmod(abs, f.mode)
	}
	return nil
}

// Keep records paths as they are now, before another function changes them, so a write
// that ends without its commit can put them back. Each path is kept once, at its first
// touch. A path that exists but cannot be kept is refused: the write could not undo it.
func (tx *Tx) Keep(paths ...string) error {
	for _, rel := range paths {
		if _, ok := tx.before[rel]; ok {
			continue
		}
		abs := tx.V.Abs(rel)
		st, err := os.Lstat(abs)
		switch {
		case errors.Is(err, os.ErrNotExist):
			tx.before[rel] = &found{}
		case err != nil:
			return fmt.Errorf("%s cannot be read (%v), so a write could not undo a change to it; nothing was written", rel, err)
		case st.Mode()&os.ModeSymlink != 0:
			target, err := os.Readlink(abs)
			if err != nil {
				return fmt.Errorf("%s is a link that cannot be read (%v); nothing was written", rel, err)
			}
			tx.before[rel] = &found{link: target}
		case st.Mode().IsRegular():
			data, err := os.ReadFile(abs)
			if err != nil {
				return fmt.Errorf("%s cannot be read (%v), so a write could not undo a change to it; nothing was written", rel, err)
			}
			tx.before[rel] = &found{exists: true, data: data, mode: st.Mode().Perm()}
		default:
			return fmt.Errorf("%s is a folder or a special file, not a document; nothing was written", rel)
		}
	}
	return nil
}

// Write writes a file and marks it for the commit. Like every write of a Tx, it refuses a
// path that Contain refuses.
func (tx *Tx) Write(rel string, content []byte) error {
	if err := tx.V.Contain(rel); err != nil {
		return err
	}
	if err := tx.Keep(rel); err != nil {
		return err
	}
	tx.written[rel] = true
	if err := tx.V.Write(rel, content); err != nil {
		return err
	}
	tx.after[rel] = &found{exists: true, data: content}
	return nil
}

// WriteIfChanged writes a file only when it differs, and marks it when it wrote.
func (tx *Tx) WriteIfChanged(rel string, content []byte) (bool, error) {
	if err := tx.V.Contain(rel); err != nil {
		return false, err
	}
	if err := tx.Keep(rel); err != nil {
		return false, err
	}
	wrote, err := tx.V.WriteIfChanged(rel, content)
	if wrote {
		tx.written[rel] = true
		if err == nil {
			tx.after[rel] = &found{exists: true, data: content}
		}
	}
	return wrote, err
}

// WriteIfUnchanged is Vault.WriteIfUnchanged within the write, kept for its rollback.
func (tx *Tx) WriteIfUnchanged(rel string, content []byte, want string) (bool, error) {
	if err := tx.V.Contain(rel); err != nil {
		return false, err
	}
	if err := tx.Keep(rel); err != nil {
		return false, err
	}
	wrote, err := tx.V.WriteIfUnchanged(rel, content, want)
	if wrote && err == nil {
		tx.written[rel] = true
		tx.after[rel] = &found{exists: true, data: content}
	}
	return wrote, err
}

// Remove deletes a file and marks it for the commit.
func (tx *Tx) Remove(rel string) error {
	if err := tx.V.Contain(rel); err != nil {
		return err
	}
	if err := tx.Keep(rel); err != nil {
		return err
	}
	tx.written[rel] = true
	if err := tx.V.Remove(rel); err != nil {
		return err
	}
	tx.after[rel] = &found{}
	return nil
}

// Move renames a file and marks both paths.
func (tx *Tx) Move(from, to string) error {
	if err := tx.V.Contain(from); err != nil {
		return err
	}
	if err := tx.V.Contain(to); err != nil {
		return err
	}
	if err := tx.Keep(from, to); err != nil {
		return err
	}
	tx.written[from] = true
	tx.written[to] = true
	if err := os.MkdirAll(filepath.Dir(tx.V.Abs(to)), 0o755); err != nil {
		return err
	}
	if err := os.Rename(tx.V.Abs(from), tx.V.Abs(to)); err != nil {
		return err
	}
	if b := tx.before[from]; b != nil && b.exists {
		tx.after[to] = &found{exists: true, data: b.data}
	}
	if strings.EqualFold(from, to) {
		// A new case of one name: on a disk that ignores case, from is still the file.
		return nil
	}
	if err := tx.V.Remove(from); err != nil {
		return err
	}
	tx.after[from] = &found{}
	return nil
}

// Indexed marks that git changed the index for this write (a checkout stages what it
// restores), so a rollback unstages the kept paths even before the commit staged them.
func (tx *Tx) Indexed() { tx.added = true }

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

// CommitAll is Commit for a write that stages the whole tree: every path it changed must
// have been kept with Keep, so a failed commit can put them back.
func (tx *Tx) CommitAll(message string) (string, error) {
	tx.all = true
	return tx.Commit(message)
}

// Commit stages the marked paths and commits them with the subject and trailers
// ("Key: value"). It returns "" when nothing changed.
func (tx *Tx) Commit(subject string, trailers ...string) (string, error) {
	sha, err := tx.commit(subject, trailers)
	if err != nil {
		return "", tx.outcome(err, tx.rollback())
	}
	tx.committed = true
	return sha, nil
}

func (tx *Tx) commit(subject string, trailers []string) (string, error) {
	g := tx.V.Git()
	stage := func() error { return g.Add(tx.Paths()...) }
	if tx.all {
		stage = g.AddAll
	}
	if err := stage(); err != nil {
		return "", err
	}
	tx.added = true
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

// firstLine is the first line of an error: git explains a held lock in a paragraph.
func firstLine(err error) string {
	msg, _, _ := strings.Cut(err.Error(), "\n")
	return strings.TrimSpace(msg)
}
