// Package gitx runs the git commands the core needs and parses their output. A Repo is
// always the top of a work tree: the vault, or a linked repository.
package gitx

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Repo is a git work tree rooted at Dir.
type Repo struct {
	Dir string
}

// Available reports whether git is on PATH.
func Available() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

func (r Repo) cmd(args ...string) *exec.Cmd {
	full := append([]string{"-c", "core.quotePath=false", "-c", "commit.gpgsign=false"}, args...)
	cmd := exec.Command("git", full...)
	cmd.Dir = r.Dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0")
	return cmd
}

func (r Repo) run(args ...string) (string, error) {
	cmd := r.cmd(args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = strings.TrimSpace(stdout.String())
		}
		if detail == "" {
			detail = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", args[0], detail)
	}
	return stdout.String(), nil
}

// Top is the top of the work tree that holds dir, or "" when dir is in none.
func Top(dir string) string {
	out, err := Repo{Dir: dir}.run("rev-parse", "--show-toplevel")
	if err != nil {
		return ""
	}
	top, err := filepath.EvalSymlinks(strings.TrimSpace(out))
	if err != nil {
		return strings.TrimSpace(out)
	}
	return top
}

// IsRoot reports whether dir is the top of a work tree.
func IsRoot(dir string) bool {
	top := Top(dir)
	if top == "" {
		return false
	}
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return false
	}
	return top == real
}

// Init makes Dir a repository with main as its first branch.
func (r Repo) Init() error {
	if _, err := r.run("init", "-q"); err != nil {
		return err
	}
	_, err := r.run("symbolic-ref", "HEAD", "refs/heads/main")
	return err
}

// GitDir is the absolute path of the repository's .git directory.
func (r Repo) GitDir() string {
	out, err := r.run("rev-parse", "--absolute-git-dir")
	if err != nil {
		return filepath.Join(r.Dir, ".git")
	}
	return strings.TrimSpace(out)
}

// HasHead reports whether the repository holds a commit.
func (r Repo) HasHead() bool {
	_, err := r.run("rev-parse", "--verify", "-q", "HEAD")
	return err == nil
}

// Head is the current commit.
func (r Repo) Head() (string, error) {
	out, err := r.run("rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// Branch is the current branch, or "HEAD" when detached.
func (r Repo) Branch() string {
	out, err := r.run("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		out, err = r.run("symbolic-ref", "--short", "HEAD")
		if err != nil {
			return ""
		}
	}
	return strings.TrimSpace(out)
}

// Remote is the fetch URL of origin, or "".
func (r Repo) Remote() string {
	out, err := r.run("remote", "get-url", "origin")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// Entry is one line of git status: a two-letter code and a path.
type Entry struct {
	Code string
	Path string
}

// Status lists every changed or untracked path, untracked folders expanded to files.
func (r Repo) Status() ([]Entry, error) {
	out, err := r.run("status", "--porcelain=v1", "-z", "-uall")
	if err != nil {
		return nil, err
	}
	var entries []Entry
	fields := strings.Split(out, "\x00")
	for i := 0; i < len(fields); i++ {
		f := fields[i]
		if len(f) < 4 {
			continue
		}
		code, p := f[:2], f[3:]
		if code[0] == 'R' || code[0] == 'C' {
			i++
		}
		entries = append(entries, Entry{Code: code, Path: p})
	}
	return entries, nil
}

// AddAll stages every change in the tree, deletions included.
func (r Repo) AddAll() error {
	_, err := r.run("add", "-A", "--", ".")
	return err
}

// Add stages the given paths, deletions included. A path that names nothing on disk or
// in the index is left out, because git refuses a pathspec that matches nothing.
func (r Repo) Add(paths ...string) error {
	var specs []string
	for _, p := range paths {
		if _, err := os.Lstat(filepath.Join(r.Dir, filepath.FromSlash(p))); err == nil {
			specs = append(specs, p)
			continue
		}
		if r.Tracked(p) {
			specs = append(specs, p)
		}
	}
	if len(specs) == 0 {
		return nil
	}
	_, err := r.run(append([]string{"add", "-A", "--"}, specs...)...)
	return err
}

// Untrack removes the paths the index holds from it, keeps the files on disk, and
// returns the paths it removed.
func (r Repo) Untrack(paths ...string) ([]string, error) {
	var removed []string
	for _, p := range paths {
		if !r.Tracked(p) {
			continue
		}
		if _, err := r.run("rm", "--cached", "-q", "--", p); err != nil {
			return removed, err
		}
		removed = append(removed, p)
	}
	return removed, nil
}

// Unstage puts the index entries of paths back as HEAD has them, and drops the entries
// HEAD lacks. The files on disk stay as they are. With the index locked by another
// program, it does what git allows and returns the error.
func (r Repo) Unstage(paths ...string) error {
	if len(paths) == 0 {
		return nil
	}
	if !r.HasHead() {
		_, err := r.run(append([]string{"rm", "--cached", "-q", "-r", "--ignore-unmatch", "--"}, paths...)...)
		return err
	}
	_, err := r.run(append([]string{"reset", "-q", "HEAD", "--"}, paths...)...)
	return err
}

// StageContent puts data in the index as the content of path, whatever the file on disk
// holds, so a commit can record a file in a state the disk does not show yet.
func (r Repo) StageContent(p string, data []byte) error {
	cmd := r.cmd("hash-object", "-w", "--stdin")
	cmd.Stdin = bytes.NewReader(data)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git hash-object: %s", strings.TrimSpace(stderr.String()))
	}
	sha := strings.TrimSpace(stdout.String())
	_, err := r.run("update-index", "--add", "--cacheinfo", "100644,"+sha+","+p)
	return err
}

// Tracked reports whether the index holds path.
func (r Repo) Tracked(p string) bool {
	out, err := r.run("ls-files", "-z", "--", p)
	return err == nil && strings.TrimRight(out, "\x00") != ""
}

// Staged reports whether anything is staged.
func (r Repo) Staged() (bool, error) {
	if !r.HasHead() {
		out, err := r.run("ls-files", "-z")
		return strings.TrimRight(out, "\x00") != "", err
	}
	out, err := r.run("diff", "--cached", "--name-only", "-z")
	return strings.TrimRight(out, "\x00") != "", err
}

func (r Repo) identity() []string {
	out, err := r.run("config", "--get", "user.email")
	if err == nil && strings.TrimSpace(out) != "" {
		return nil
	}
	return []string{"-c", "user.name=atlas", "-c", "user.email=atlas@localhost"}
}

// Commit records the index with message and returns the new commit. It skips commit
// hooks and falls back to a local identity when the user has none.
func (r Repo) Commit(message string) (string, error) {
	args := append(r.identity(), "commit", "-q", "--no-verify", "-m", message)
	if _, err := r.run(args...); err != nil {
		return "", err
	}
	return r.Head()
}

// CommitOnly commits the given paths as they are in the work tree, and nothing else the
// index holds; what else was staged stays staged. A path git does not track yet must be
// added first.
func (r Repo) CommitOnly(message string, paths ...string) (string, error) {
	args := append(r.identity(), "commit", "-q", "--no-verify", "--only", "-m", message, "--")
	if _, err := r.run(append(args, paths...)...); err != nil {
		return "", err
	}
	return r.Head()
}

// unfinished are the files git leaves while an operation of several steps is open.
var unfinished = []struct{ path, what string }{
	{"MERGE_HEAD", "merge"},
	{"rebase-merge", "rebase"},
	{"rebase-apply", "rebase"},
	{"CHERRY_PICK_HEAD", "cherry-pick"},
	{"REVERT_HEAD", "revert"},
}

// CheckIdle refuses while the repository is in the middle of a merge, a rebase, a
// cherry-pick, or a revert: a commit then would join that operation.
func (r Repo) CheckIdle() error {
	dir := r.GitDir()
	for _, u := range unfinished {
		if _, err := os.Lstat(filepath.Join(dir, u.path)); err == nil {
			return fmt.Errorf("%s is in the middle of a %s; finish or abort it first", r.Dir, u.what)
		}
	}
	return nil
}

// Commit is one entry of the log.
type Commit struct {
	SHA      string
	Date     time.Time
	Subject  string
	Body     string
	Trailers map[string]string
}

const logFormat = "--format=%H%x00%aI%x00%s%x00%b%x1e"

// Log returns the newest n commits, or all when n is 0, that touched paths when given.
func (r Repo) Log(n int, paths ...string) ([]Commit, error) {
	if !r.HasHead() {
		return nil, nil
	}
	args := []string{"log", logFormat}
	if n > 0 {
		args = append(args, fmt.Sprintf("-n%d", n))
	}
	if len(paths) > 0 {
		args = append(append(args, "--"), paths...)
	}
	out, err := r.run(args...)
	if err != nil {
		return nil, err
	}
	return parseLog(out), nil
}

// FindTrailer is the newest commit whose trailer key holds value, or "".
func (r Repo) FindTrailer(key, value string) (string, error) {
	if !r.HasHead() {
		return "", nil
	}
	out, err := r.run("log", "--fixed-strings", "--grep", key+": "+value, logFormat)
	if err != nil {
		return "", err
	}
	for _, c := range parseLog(out) {
		if c.Trailers[key] == value {
			return c.SHA, nil
		}
	}
	return "", nil
}

func parseLog(out string) []Commit {
	var commits []Commit
	for _, record := range strings.Split(out, "\x1e") {
		record = strings.TrimLeft(record, "\n")
		fields := strings.SplitN(record, "\x00", 4)
		if len(fields) < 4 {
			continue
		}
		date, _ := time.Parse(time.RFC3339, fields[1])
		body := strings.TrimSpace(fields[3])
		commits = append(commits, Commit{SHA: fields[0], Date: date, Subject: fields[2], Body: body, Trailers: trailers(body)})
	}
	return commits
}

// trailers parses key: value lines from the last paragraph of a commit body.
func trailers(body string) map[string]string {
	out := map[string]string{}
	paragraphs := strings.Split(strings.TrimSpace(body), "\n\n")
	for _, line := range strings.Split(paragraphs[len(paragraphs)-1], "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok || strings.ContainsAny(key, " \t") {
			continue
		}
		out[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return out
}

// ChangedPaths lists the paths one commit touched, renames as both sides.
func (r Repo) ChangedPaths(sha string) ([]string, error) {
	out, err := r.run("show", "--format=", "--name-only", "--no-renames", "-z", sha)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, p := range strings.Split(out, "\x00") {
		if p = strings.TrimSpace(p); p != "" {
			paths = append(paths, p)
		}
	}
	return paths, nil
}

// Parent is the first parent of rev, or "" for a root commit.
func (r Repo) Parent(rev string) string {
	out, err := r.run("rev-parse", "--verify", "-q", rev+"^")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// Has reports whether an object exists, such as "HEAD:wiki/x.md".
func (r Repo) Has(object string) bool {
	_, err := r.run("cat-file", "-e", object)
	return err == nil
}

// RestoreFrom puts each path back to what rev holds, in the tree and the index, and
// removes the paths rev does not hold. It touches no other path, so it works while the
// rest of the tree has changes, where git revert refuses. An empty rev removes every path.
func (r Repo) RestoreFrom(rev string, paths ...string) error {
	var restore, remove []string
	for _, p := range paths {
		if !filepath.IsLocal(filepath.FromSlash(p)) {
			return fmt.Errorf("restore %q: the path leaves the work tree", p)
		}
		if rev != "" && r.Has(rev+":"+p) {
			restore = append(restore, p)
			continue
		}
		remove = append(remove, p)
	}
	if len(restore) > 0 {
		if _, err := r.run(append([]string{"checkout", rev, "--"}, restore...)...); err != nil {
			return err
		}
	}
	if len(remove) == 0 {
		return nil
	}
	// The root keeps a removal inside the tree when a folder on the way is a link out.
	root, err := os.OpenRoot(r.Dir)
	if err != nil {
		return err
	}
	defer root.Close()
	for _, p := range remove {
		if r.Tracked(p) {
			if _, err := r.run("rm", "-q", "-f", "--cached", "--", p); err != nil {
				return err
			}
		}
		if err := root.Remove(filepath.FromSlash(p)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

// Unchanged reports whether every path is the same in the tree as in rev.
func (r Repo) Unchanged(rev string, paths ...string) (bool, error) {
	if len(paths) == 0 {
		return true, nil
	}
	// A path rev does not hold and the tree does not hold is unchanged too; git diff
	// refuses a pathspec that matches nothing, so those are left out first.
	var specs []string
	for _, p := range paths {
		onDisk := true
		if _, err := os.Lstat(filepath.Join(r.Dir, filepath.FromSlash(p))); err != nil {
			onDisk = false
		}
		inRev := r.Has(rev + ":" + p)
		switch {
		case !onDisk && !inRev:
			continue
		case onDisk != inRev:
			return false, nil
		}
		specs = append(specs, p)
	}
	if len(specs) == 0 {
		return true, nil
	}
	cmd := r.cmd(append([]string{"diff", "--quiet", rev, "--"}, specs...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("git diff: %s", strings.TrimSpace(stderr.String()))
}

// ShowFile is the content of path at rev.
func (r Repo) ShowFile(rev, p string) ([]byte, error) {
	cmd := r.cmd("show", rev+":"+p)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git show %s:%s: %s", rev, p, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// InHistory reports whether id is the full id of a commit that HEAD contains.
func (r Repo) InHistory(id string) bool {
	if len(id) != 40 && len(id) != 64 || strings.Trim(id, "0123456789abcdef") != "" {
		return false
	}
	_, err := r.run("merge-base", "--is-ancestor", id, "HEAD")
	return err == nil
}

// Behind counts the commits from rev to HEAD. It is an error when rev is no commit here.
func (r Repo) Behind(rev string) (int, error) {
	out, err := r.run("rev-list", "--count", rev+"..HEAD")
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(out))
}

// LsFiles lists the tracked paths.
func (r Repo) LsFiles() ([]string, error) {
	out, err := r.run("ls-files", "-z")
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, p := range strings.Split(out, "\x00") {
		if p != "" {
			paths = append(paths, p)
		}
	}
	return paths, nil
}

// Exclude adds patterns to .git/info/exclude when it lacks them, so a machine's own files
// stay out of the history without editing a file the user owns.
func (r Repo) Exclude(patterns ...string) error {
	file := filepath.Join(r.GitDir(), "info", "exclude")
	data, _ := os.ReadFile(file)
	have := map[string]bool{}
	for _, l := range strings.Split(string(data), "\n") {
		have[strings.TrimSpace(l)] = true
	}
	var add []string
	for _, p := range patterns {
		if !have[p] {
			add = append(add, p)
		}
	}
	if len(add) == 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	text := string(data)
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	text += strings.Join(add, "\n") + "\n"
	return os.WriteFile(file, []byte(text), 0o644)
}

// HeadTime is the commit time of HEAD, in the local zone, or the zero time.
func (r Repo) HeadTime() time.Time {
	out, err := r.run("log", "-1", "--format=%cI")
	if err != nil {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(out))
	if err != nil {
		return time.Time{}
	}
	return t.Local()
}

// AheadBehind counts the commits HEAD has that its upstream lacks, and the reverse, as of
// the last fetch. ok is false when the branch has no upstream.
func (r Repo) AheadBehind() (ahead, behind int, ok bool) {
	out, err := r.run("rev-list", "--left-right", "--count", "HEAD...@{upstream}")
	if err != nil {
		return 0, 0, false
	}
	f := strings.Fields(out)
	if len(f) != 2 {
		return 0, 0, false
	}
	ahead, _ = strconv.Atoi(f[0])
	behind, _ = strconv.Atoi(f[1])
	return ahead, behind, true
}
