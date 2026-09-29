package hooks

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/nathanaday/atlas-obsidian/internal/change"
	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/sessions"
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

// writeActions are, per atlas tool, the actions that write.
var writeActions = map[string]map[string]bool{
	"change": {"propose": true, "apply": true, "reject": true, "undo": true},
	"thread": {"open": true, "attach": true, "file": true, "tasks": true, "task": true, "set": true, "reopen": true},
	"source": {"capture": true},
	"vault":  {"init": true, "sync": true, "mention": true},
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
	v := findVault(in, env)
	if v == nil {
		return nil
	}
	// Rule 2: the model applies a change only after the user had a turn.
	if tool == "change" && in.tool().Action == "apply" {
		if reason := gate(v, in); reason != "" {
			return deny(w, reason)
		}
		return nil
	}
	if !editTools[in.ToolName] {
		return nil
	}
	for _, f := range in.paths() {
		if reason := pathRefusal(v, in, f); reason != "" {
			return deny(w, reason)
		}
	}
	return nil
}

var (
	shellOperator = regexp.MustCompile("[;&|<>$`\n\\\\]")
	gitRead       = regexp.MustCompile(`^git (-C ("[^"]*"|'[^']*'|\S+) )?(log|diff|show)(\s|$)`)
)

// readOnlyRefusal is why a read-only agent may not make a call, or "".
func readOnlyRefusal(in Input, tool string) string {
	agent := sessions.AgentName(in.AgentType)
	switch {
	case editTools[in.ToolName]:
		return agent + " is read-only; it returns its entity and the skill that sent it writes"
	case tool != "" && writeActions[tool][in.tool().Action]:
		return fmt.Sprintf("%s is read-only; %s %s writes, so the skill that sent it makes that call", agent, tool, in.tool().Action)
	case in.ToolName == "Bash":
		cmd := strings.TrimSpace(in.tool().Command)
		if agent == "thread-review" && gitRead.MatchString(cmd) && !shellOperator.MatchString(cmd) {
			for _, f := range strings.Fields(cmd) {
				if f == "-c" || strings.HasPrefix(f, "--output") || f == "--ext-diff" || f == "--textconv" {
					return "thread-review may run git log, git diff, and git show without " + f
				}
			}
			return ""
		}
		if agent == "thread-review" {
			return "thread-review may run only git log, git diff, and git show (git -C <repository> log …), with no shell operator"
		}
		return agent + " is read-only and runs no shell command"
	}
	return ""
}

// gate is the refusal of a change apply the user has not had a turn to see, or "".
func gate(v *vault.Vault, in Input) string {
	key := in.tool().ID
	c := findChange(v, key)
	if c == nil {
		return ""
	}
	if change.ParseCounts(c.Str("counts")).Writes() == 0 {
		return ""
	}
	proposed, ok := vault.ParseTime(c.Str("proposed"))
	if !ok {
		return ""
	}
	s := sessions.Find(v, in.event().Key())
	if s != nil {
		if last, ok := vault.ParseTime(s.Str("last_prompt")); ok && last.After(proposed) {
			return ""
		}
	}
	return fmt.Sprintf("show the preview of %s and wait for the user's yes; apply runs once the user has answered after the proposal", c.Title())
}

// findChange finds a change document by id or title, reading the change folders only.
func findChange(v *vault.Vault, key string) *doc.Doc {
	key = strings.TrimSpace(doc.LinkTarget(key))
	if key == "" {
		return nil
	}
	files, _ := filepath.Glob(v.Abs(vault.Changes + "/*/*.md"))
	for _, abs := range files {
		if strings.EqualFold(strings.TrimSuffix(filepath.Base(abs), ".md"), key) {
			if data, err := os.ReadFile(abs); err == nil {
				return doc.Parse(v.Rel(abs), data)
			}
		}
	}
	for _, abs := range files {
		data, err := os.ReadFile(abs)
		if err != nil || !strings.Contains(string(data), "id: "+key) {
			continue
		}
		if d := doc.Parse(v.Rel(abs), data); d.ID() == key {
			return d
		}
	}
	return nil
}

// pathRefusal is why a write tool may not touch a file, or "".
func pathRefusal(v *vault.Vault, in Input, f patchFile) string {
	rel := v.Rel(f.Path)
	if rel == "" {
		return repositoryRefusal(v, in, f.Path)
	}
	name := path.Base(rel)
	switch {
	case strings.HasPrefix(rel, vault.Wiki+"/"):
		return rel + " is in the wiki, which changes only through a change: build a plan, call change propose, show the preview, and apply after the user's yes"
	case strings.HasPrefix(rel, vault.Changes+"/"):
		return rel + " is a change document; the change tool writes it. Edit a proposed page inside it only when the user asks"
	case rel == vault.Marker:
		return "Atlas.md is the user's; ask the user to edit it"
	case rel == vault.ThreadsCanvas:
		return rel + " is the board as a canvas, which sync derives from the threads; the user moves cards and draws edges in Obsidian"
	case strings.HasSuffix(name, ".base"):
		return rel + " is a Base that vault init ships; ask the user to change it in Obsidian"
	case rel == vault.Settings:
		return rel + " lists the linked repositories; vault sync keeps it"
	case strings.HasPrefix(rel, vault.Sessions+"/"):
		return sessionRefusal(v, in, f, rel)
	case strings.HasPrefix(rel, vault.Threads+"/"):
		return threadRefusal(v, in, f, rel)
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

// threadRefusal lets the model write a thread document's prose and nothing else.
func threadRefusal(v *vault.Vault, in Input, f patchFile, rel string) string {
	if f.Op != "update" {
		return "a thread document comes from the thread tool (open, file, tasks); revise its prose with Edit after"
	}
	data, err := v.Read(rel)
	if err != nil {
		return ""
	}
	d := doc.Parse(rel, data)
	if touchesPrefix(in, f, d) {
		return "a thread document's frontmatter and lead callout are the thread tool's; use thread set, task set, or task done"
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
// is judged by the lines it removes.
func touchesRange(in Input, f patchFile, content string, start, end int) bool {
	if in.ToolName == "apply_patch" {
		region := content[start:end]
		for _, l := range f.Removed {
			if strings.TrimSpace(l) != "" && strings.Contains(region, l) {
				return true
			}
		}
		return false
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

// sectionBounds are the offsets of a level-two section, heading included, or -1.
func sectionBounds(content, title string) (int, int) {
	lines := strings.SplitAfter(content, "\n")
	offset, start := 0, -1
	for _, l := range lines {
		if strings.HasPrefix(l, "## ") {
			if start >= 0 {
				return start, offset
			}
			if strings.EqualFold(strings.TrimSpace(strings.TrimPrefix(l, "## ")), title) {
				start = offset
			}
		}
		offset += len(l)
	}
	if start >= 0 {
		return start, len(content)
	}
	return -1, -1
}

// repositoryRefusal is the thread rule: an edit inside a linked repository needs an open
// thread of this session that covers the repository.
func repositoryRefusal(v *vault.Vault, in Input, target string) string {
	var repo *vault.Repo
	for _, r := range v.Repositories() {
		if r.Path != "" && vault.Within(target, r.Path) {
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
	var threads []string
	if s != nil {
		threads = s.List("threads")
	}
	chain := map[string]bool{strings.ToLower(repo.Title): true}
	parents := v.AreaParents()
	for p := strings.ToLower(repo.Parent); p != "" && !chain[p]; p = strings.ToLower(parents[p]) {
		chain[p] = true
	}
	for _, t := range threads {
		if covers(v, doc.LinkTarget(t), repo.Title, chain) {
			return ""
		}
	}
	return fmt.Sprintf("an edit in %s needs an open thread that covers it. Find or open one with the thread-work skill (thread open or thread attach), then edit", repo.Title)
}

// covers reports whether an open thread holds the repository: in its scope, through an
// area above it, or as a task's repository.
func covers(v *vault.Vault, title, repo string, chain map[string]bool) bool {
	folder := v.Abs(path.Join(vault.Threads, title))
	data, err := os.ReadFile(filepath.Join(folder, title+".md"))
	if err != nil {
		return false
	}
	stub := doc.Parse("", data)
	if stub.Type() != "stub" || stub.Str("stage") == "closed" {
		return false
	}
	for _, s := range stub.List("scope") {
		if chain[strings.ToLower(doc.LinkTarget(s))] {
			return true
		}
	}
	entries, _ := os.ReadDir(folder)
	for _, e := range entries {
		if !strings.Contains(e.Name(), " — T") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(folder, e.Name()))
		if err != nil {
			continue
		}
		task := doc.Parse("", data)
		if task.Type() == "task" && strings.EqualFold(doc.LinkTarget(task.Str("repository")), repo) {
			return true
		}
	}
	return false
}
