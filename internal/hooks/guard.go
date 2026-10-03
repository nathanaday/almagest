package hooks

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/schema"
	"github.com/nathanaday/atlas-obsidian/internal/sessions"
	"github.com/nathanaday/atlas-obsidian/internal/thread"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

// deny prints the refusal a PreToolUse hook gives.
func deny(w io.Writer, reason string) error {
	return json.NewEncoder(w).Encode(map[string]any{"hookSpecificOutput": map[string]any{
		"hookEventName":            "PreToolUse",
		"permissionDecision":       "deny",
		"permissionDecisionReason": "atlas: " + reason,
	}})
}

// readActions are, per atlas tool, the actions that only read; a tool listed with nil
// only reads. A read-only agent makes no other call: an action or a tool this list does
// not know is a write until someone lists it here.
var readActions = map[string]map[string]bool{
	"search":  nil,
	"context": nil,
	"match":   nil,
	"lint":    nil,
	"vault":   {"": true, "status": true},
	"change":  {"": true, "show": true},
	"thread":  {"": true, "list": true, "load": true},
	"chord":   {"": true, "list": true, "load": true},
	"source":  {"chunks": true, "read": true},
}

// readsOnly reports whether a call of an atlas tool only reads.
func readsOnly(tool, action string) bool {
	reads, ok := readActions[tool]
	return ok && (reads == nil || reads[action])
}

// Guard refuses a call that breaks a rule. The first rule that matches decides.
func Guard(r io.Reader, w io.Writer, env Env) error {
	in := readInput(r)
	tool := atlasTool(in.ToolName)
	// Rule 1: a read-only agent writes nothing.
	if in.AgentType != "" && sessions.ReadOnly(in.AgentType) {
		if reason := readOnlyRefusal(in, tool); reason != "" {
			return deny(w, reason)
		}
		return nil
	}
	// Rule 2: the shell does not do what only the user does. The change tool keeps the
	// gate, and the prompt hook records the user's turns.
	if in.ToolName == "Bash" {
		if reason := atlasCommandRefusal(in.tool().Command); reason != "" {
			return deny(w, reason)
		}
		return nil
	}
	if tool == "thread" && in.tool().Action == "start" {
		if reason := restartRefusal(in, env); reason != "" {
			return deny(w, reason)
		}
		return nil
	}
	if !editTools[in.ToolName] {
		return nil
	}
	session := findVault(in, env)
	for _, f := range in.paths() {
		// Every rule judges the file as the disk names it, whatever case, Unicode form,
		// or link the agent wrote.
		f.Path = canonical(f.Path)
		if f.Path == canonical(vault.HomeFrom(env.getenv).ConfigPath()) {
			return deny(w, f.Path+" holds the commands Atlas runs (terminal_command, agent_commands); the user changes it in the Atlas settings in Obsidian, or with atlas-obsidian config")
		}
		// A file belongs to the vault above it, wherever the session runs.
		v := session
		if root := vault.FindAbove(filepath.Dir(f.Path)); root != "" {
			if fv, err := vault.Open(root); err == nil {
				v = fv
			}
		}
		if v == nil {
			continue
		}
		if reason := pathRefusal(v, in, f); reason != "" {
			return deny(w, reason)
		}
	}
	return nil
}

var shellSeparator = regexp.MustCompile("[;&|()\n`]")

// redirect matches a redirect and its target, which bash takes out of a command's words.
var redirect = regexp.MustCompile(`\d*(?:<<<|<<|<>|>>|>\||<|>)&?\s*[^\s;&|()<>]*`)

// ansiQuoted matches bash's $'…' quoting, whose escapes bash decodes, and $"…".
var ansiQuoted = regexp.MustCompile(`\$'((?:[^'\\]|\\.)*)'|\$"`)

// shellWords is a command as bash reads it before it runs it: a backslash-newline joins
// two lines, redirects leave the words, and $'…' turns into its decoded text.
func shellWords(cmd string) string {
	cmd = strings.ReplaceAll(cmd, "\\\n", "")
	cmd = redirect.ReplaceAllString(cmd, " ")
	return ansiQuoted.ReplaceAllStringFunc(cmd, func(m string) string {
		if m == `$"` {
			return `"`
		}
		body := m[2 : len(m)-1]
		if text, err := strconv.Unquote(`"` + strings.ReplaceAll(body, `"`, `\"`) + `"`); err == nil {
			return "'" + text + "'"
		}
		return "'" + body + "'"
	})
}

// binaryNames are the binary's name and the name it had in 6.0 to 6.2, which an older
// install may still hold.
var binaryNames = []string{"atlas-obsidian", "atlas"}

// atlasCommandRefusal is why a shell command may not run the atlas-obsidian binary, or "":
// a change apply, which would skip the gate, and a hook, which would forge an event.
func atlasCommandRefusal(cmd string) string {
	for _, part := range shellSeparator.Split(shellWords(cmd), -1) {
		words := strings.Fields(shellUnquote.Replace(part))
		for i, word := range words {
			if !slices.ContainsFunc(binaryNames, func(n string) bool { return strings.EqualFold(n, path.Base(word)) }) {
				continue
			}
			rest := positional(words[i+1:])
			if len(rest) == 0 {
				continue
			}
			switch {
			case rest[0] == "hook":
				return "atlas-obsidian hook runs only from the host; the shell does not send hook events"
			case rest[0] == "change" && slices.Contains(rest[1:], "apply"):
				return "apply a change with the change tool after the user's yes; the user can also apply it with Apply in Obsidian, or run the command with !"
			case rest[0] == "vault" && slices.Contains(rest[1:], "migrate"):
				return "the migration rewrites the whole vault, so only the user runs it: ask the user to type atlas-obsidian vault migrate, or run it with !"
			}
		}
	}
	return ""
}

// shellUnquote drops what the shell drops from a word before it runs it: quotes and
// backslashes.
var shellUnquote = strings.NewReplacer(`"`, "", "'", "", `\`, "")

// positional are the words after the binary without its options. An option's value, such
// as W in --vault W, stays, so the rule looks for an action anywhere after its
// subcommand.
func positional(words []string) []string {
	var out []string
	for _, w := range words {
		if !strings.HasPrefix(w, "-") {
			out = append(out, w)
		}
	}
	return out
}

// readOnlyRefusal is why a read-only agent may not make a call, or "".
func readOnlyRefusal(in Input, tool string) string {
	agent := sessions.AgentName(in.AgentType)
	switch {
	case editTools[in.ToolName]:
		return agent + " is read-only; it returns its entity and the skill that sent it writes"
	case tool != "" && !readsOnly(tool, in.tool().Action):
		return fmt.Sprintf("%s is read-only, and %s %q is not one of the calls that only read; the skill that sent it makes that call", agent, tool, in.tool().Action)
	case in.ToolName == "Bash":
		// thread-audit checks work by running its tests and reading git; it edits nothing.
		if agent == "thread-audit" {
			return atlasCommandRefusal(in.tool().Command)
		}
		return agent + " is read-only and runs no shell command"
	}
	return ""
}

// pathRefusal is why a write tool may not touch a file, or "".
func pathRefusal(v *vault.Vault, in Input, f patchFile) string {
	rel := v.Rel(f.Path)
	if rel == "" {
		return repositoryRefusal(v, in, f.Path)
	}
	name := path.Base(rel)
	switch {
	case rel == vault.VaultConfigFile:
		return rel + " holds the commands Atlas runs for this vault (terminal_command, agent_commands), which win over the machine's; the user changes it in the Atlas settings in Obsidian, or with atlas-obsidian config"
	case rel == vault.PluginDir || strings.HasPrefix(rel, vault.PluginDir+"/"):
		return rel + " is the Atlas plugin, whose code and binaryPath decide what runs; vault init and vault sync install it, and the user sets binaryPath in the Atlas settings in Obsidian"
	case path.Dir(rel) == vault.Documents:
		return documentRefusal(v, in, f, rel)
	case strings.HasPrefix(rel, vault.Assets+"/"):
		return rel + " is a captured original or an attachment; source capture writes the originals, and you add attachments in Obsidian"
	case strings.HasPrefix(rel, vault.Wiki+"/"):
		return rel + " is in wiki/, which holds wiki/documents and wiki/assets only; a document comes from the thread tool, the chord tool, change propose, or source capture"
	case strings.HasPrefix(rel, vault.Changes+"/"):
		return rel + " is a change document; the change tool writes it. Edit a proposed document inside it only when the user asks"
	case strings.HasPrefix(rel, vault.Chords+"/"):
		return rel + " is a chord's canvas, which code writes from the stubs; change the order with chord order, and the user redraws it in Obsidian"
	case strings.HasPrefix(rel, vault.Views+"/"):
		return rel + " is a view, which code writes from the documents; change the documents instead"
	case rel == vault.Marker:
		return "Atlas.md is the user's; ask the user to edit it"
	case strings.HasSuffix(name, ".base"):
		return rel + " is a Base that vault init ships; ask the user to change it in Obsidian"
	case rel == vault.Settings:
		return rel + " lists the linked repositories; vault sync keeps it"
	case strings.HasPrefix(rel, vault.Sessions+"/"):
		return sessionRefusal(v, in, f, rel)
	}
	return ""
}

// MaxCardLines is how many lines of prose a stub's Idea and Notes hold together before
// the guard refuses an agent's edit that adds more: the card stays short.
const MaxCardLines = 40

// codeSection says why an agent does not edit a section code owns, by type.
var codeSection = map[string]string{
	"stub":         "## Thread is code's; it follows the thread's documents",
	"tasks":        "## Tasks is code's for an agent: check, drop, or open a task with thread check, and add one with thread tasks. Edit ## Details freely",
	"verification": "a verification is the record of one round: a correction is a new round (thread verify), and a finding's outcome is thread finding. Edit ## Notes freely",
	"chord":        "## Threads is code's; it follows the stubs. Change the order with chord order",
}

var (
	headingTwo = regexp.MustCompile(`^## +(.+?)\s*$`)
	checkBox   = regexp.MustCompile(`^\s*[-*] \[.\] `)
)

// documentRefusal keeps the documents of wiki/documents to their writers: a new file
// comes from a tool; knowledge changes through a change; a thread document's prose is
// the model's, and its frontmatter, lead callout, and code sections are code's. A thread
// document holds only the sections of its type, and a stub stays short.
func documentRefusal(v *vault.Vault, in Input, f patchFile, rel string) string {
	data, err := v.Read(rel)
	if err != nil {
		if f.Op == "delete" {
			return ""
		}
		return rel + " would be a new document; a stub, a spec, a task list, or a verification comes from the thread tool, a chord from the chord tool, a topic or a repository from change propose, a source from source capture"
	}
	d := doc.Parse(rel, data)
	t := schema.Get(d.Type())
	switch {
	case t == nil || d.Front == nil:
		return ""
	case t.Family == schema.Knowledge:
		return rel + " is a " + d.Type() + ", which changes only through a change: build a plan, call change propose, show the preview, and apply after the user's yes"
	case f.Op != "update" || in.ToolName == "Write":
		return rel + " is a " + d.Type() + "; revise its prose with Edit, and its fields with thread set"
	case touchesPrefix(in, f, d):
		return "the frontmatter and the lead callout of " + d.Title() + " are code's; use thread set, or the thread action that fits"
	}
	for _, s := range t.CodeSections {
		if start, end := sectionBounds(d.Content, s); start >= 0 && touchesRange(in, f, d.Content, start, end) {
			if why := codeSection[d.Type()]; why != "" {
				return why
			}
			return "## " + s + " is code's; it follows the documents and the events"
		}
	}
	if !schema.IsThread(d.Type()) {
		return ""
	}
	added, removed := in.added(f)
	for _, l := range added {
		if m := headingTwo.FindStringSubmatch(l); m != nil && !slices.ContainsFunc(t.Sections, func(s string) bool { return strings.EqualFold(s, m[1]) }) {
			return fmt.Sprintf("## %s is no section of a %s; it holds %s. %s", m[1], d.Type(), strings.Join(t.Sections, ", "), thread.Elsewhere)
		}
		if d.Type() == "spec" && checkBox.MatchString(l) {
			return "a spec holds no check box; it says what must be true, and the task list holds the steps (thread tasks)"
		}
	}
	if d.Type() == "stub" {
		idea, _ := doc.Section(d.Body, "Idea")
		notes, _ := doc.Section(d.Body, "Notes")
		have := len(strings.Split(strings.TrimSpace(idea+"\n"+notes), "\n"))
		if grow := len(added) - len(removed); grow > 0 && have+grow > MaxCardLines {
			return fmt.Sprintf("a stub is the front page of its thread and stays short (%d lines of Idea and Notes at most). %s", MaxCardLines, thread.Elsewhere)
		}
	}
	return ""
}

// sessionRefusal keeps each session to its own document, and to three sections of it.
func sessionRefusal(v *vault.Vault, in Input, f patchFile, rel string) string {
	const allowed = "edit only Description, Progress, and Summary of your own session document"
	if f.Op != "update" {
		return "session documents come from the hooks; " + allowed
	}
	data, err := v.Read(rel)
	if err != nil {
		return ""
	}
	d := doc.Parse(rel, data)
	key := in.event().Key()
	if d.Str("harness_id") != key {
		return rel + " is another session's document; " + allowed
	}
	if touchesPrefix(in, f, d) {
		return "the frontmatter and the lead callout are the hooks'; " + allowed
	}
	if start, end := sectionBounds(d.Content, "Subagents"); start >= 0 && touchesRange(in, f, d.Content, start, end) {
		return "Subagents is the hooks'; " + allowed
	}
	return ""
}

// prefixEnd is the offset where a document's own text begins: after the frontmatter and
// the code-owned lead callout.
func prefixEnd(d *doc.Doc) int {
	front, body, ok := doc.Split(d.Content)
	end := 0
	if ok {
		end = len(d.Content) - len(body)
		_ = front
	}
	if lead := doc.Lead(body); lead != "" {
		if i := strings.Index(body, lead); i >= 0 {
			end += i + len(lead)
		}
	}
	return end
}

// touchesPrefix reports whether an edit changes text in the frontmatter or the lead.
func touchesPrefix(in Input, f patchFile, d *doc.Doc) bool {
	return touchesRange(in, f, d.Content, 0, prefixEnd(d))
}

// touchesRange reports whether an edit's old text overlaps content[start:end]. A patch
// is judged hunk by hunk, where each lands in the file.
func touchesRange(in Input, f patchFile, content string, start, end int) bool {
	if in.ToolName == "apply_patch" {
		return slices.ContainsFunc(f.Hunks, func(h hunk) bool { return h.touches(content, start, end) })
	}
	for _, old := range in.oldStrings() {
		offset := 0
		for {
			i := strings.Index(content[offset:], old)
			if i < 0 {
				break
			}
			at := offset + i
			if at < end && at+len(old) > start {
				return true
			}
			offset = at + max(1, len(old))
		}
	}
	return false
}

// sectionBounds are the offsets in content of a level-two section of the body, heading
// included, or -1.
func sectionBounds(content, title string) (int, int) {
	base := 0
	body := content
	if _, b, ok := doc.Split(content); ok {
		base, body = len(content)-len(b), b
	}
	start, end := doc.SectionOffsets(body, title)
	if start < 0 {
		return -1, -1
	}
	return base + start, base + end
}

// repositoryRefusal is the edit rule: an edit inside a linked repository needs a thread
// this session started that has a task list for the repository with an open task.
func repositoryRefusal(v *vault.Vault, in Input, target string) string {
	var repo *vault.Repo
	for _, r := range v.Repositories() {
		if r.Path != "" && vault.Within(target, canonical(r.Path)) {
			rr := r
			if repo == nil || len(rr.Path) > len(repo.Path) {
				repo = &rr
			}
		}
	}
	if repo == nil {
		return ""
	}
	s := sessions.Find(v, in.event().Key())
	if s == nil && in.AgentID != "" {
		s = sessions.Find(v, in.event().SessionID)
	}
	why := ""
	if s != nil {
		for _, title := range sessions.Threads(s) {
			reason := covers(v, title, repo.Title)
			if reason == "" {
				return ""
			}
			if why == "" || strings.Contains(reason, "open task") {
				why = reason
			}
		}
	}
	if why == "" {
		why = "this session started no thread"
	}
	return fmt.Sprintf("an edit in %s needs a thread this session started, with a task list for %s that has an open task; %s. The thread-work skill finds or plants the thread; thread start binds the session; a fix after the last task needs a new task first (thread tasks)", repo.Title, repo.Title, why)
}

// covers says why a thread does not let its session edit a repository, or "" when it
// does: the thread is not ended, and its task list for the repository has an open task.
func covers(v *vault.Vault, title, repo string) string {
	stub := sessions.Document(v, title)
	if stub == nil || stub.Type() != "stub" {
		return title + " is no thread"
	}
	if s := stub.Str("status"); s == thread.Dropped || s == thread.Resolved {
		return title + " is " + s
	}
	// The lists named after the thread answer most calls without reading the vault; a
	// list renamed by hand still names its thread, which the index finds.
	why := openTask(sessions.TaskLists(v, title), title, repo)
	if why == "" {
		return ""
	}
	if idx, err := vault.Load(v); err == nil {
		var lists []*doc.Doc
		for _, d := range idx.Of("tasks") {
			if strings.EqualFold(doc.LinkTarget(d.Str("thread")), title) {
				lists = append(lists, d)
			}
		}
		why = openTask(lists, title, repo)
	}
	return why
}

// openTask says why no list of a thread has an open task for a repository, or "".
func openTask(lists []*doc.Doc, title, repo string) string {
	why := title + " has no task list for " + repo
	for _, list := range lists {
		if !strings.EqualFold(doc.LinkTarget(list.Str("repository")), repo) {
			continue
		}
		for _, task := range thread.Tasks(list) {
			if task.State == thread.TaskOpen {
				return ""
			}
		}
		why = title + " has no open task for " + repo + ": every task of its list is done"
	}
	return why
}

// restartRefusal refuses a second start of a thread this session already started and
// that is still started: one continued event per session, and the session is bound
// already.
func restartRefusal(in Input, env Env) string {
	key := strings.TrimSpace(in.tool().Thread)
	if key == "" {
		return ""
	}
	v := findVault(in, env)
	if v == nil {
		return ""
	}
	s := sessions.Find(v, in.event().Key())
	if s == nil {
		return ""
	}
	stub := sessions.Document(v, doc.LinkTarget(key))
	if stub == nil {
		if idx, err := vault.Load(v); err == nil {
			if d := idx.ByID(key); d != nil {
				stub = d
			}
		}
	}
	if stub == nil || stub.Str("status") != thread.Started {
		return ""
	}
	for _, t := range sessions.Threads(s) {
		if strings.EqualFold(t, stub.Title()) {
			return stub.Title() + " is started in this session already; go on with its open tasks, and check each one when it is done (thread check)"
		}
	}
	return ""
}

// canonical is a path as the disk names it: links on the part that exists are resolved,
// and each part is spelled as its folder entry, which on a disk that ignores case or
// Unicode form may differ from what was written. A part that does not exist yet keeps its
// spelling.
func canonical(p string) string {
	p = filepath.Clean(p)
	have, tail := p, ""
	for {
		if _, err := os.Lstat(have); err == nil {
			break
		}
		parent := filepath.Dir(have)
		if parent == have {
			return p
		}
		tail = filepath.Join(filepath.Base(have), tail)
		have = parent
	}
	real, err := filepath.EvalSymlinks(have)
	if err != nil {
		return p
	}
	out := string(filepath.Separator)
	for _, part := range strings.Split(strings.TrimPrefix(real, string(filepath.Separator)), string(filepath.Separator)) {
		if part == "" {
			continue
		}
		out = filepath.Join(out, entryName(out, part))
	}
	if tail != "" {
		out = filepath.Join(out, tail)
	}
	return out
}

// entryName is the name under which dir holds part: part itself when an entry has that
// exact name, else the entry that is the same file.
func entryName(dir, part string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return part
	}
	for _, e := range entries {
		if e.Name() == part {
			return part
		}
	}
	want, err := os.Lstat(filepath.Join(dir, part))
	if err != nil {
		return part
	}
	for _, e := range entries {
		if info, err := os.Lstat(filepath.Join(dir, e.Name())); err == nil && os.SameFile(want, info) {
			return e.Name()
		}
	}
	return part
}
