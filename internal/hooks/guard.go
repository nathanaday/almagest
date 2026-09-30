package hooks

import (
	"encoding/json"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/schema"
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
	"work":   {"stub": true, "spec": true, "promote": true, "start": true, "done": true, "drop": true, "reopen": true, "block": true, "unblock": true, "resolve": true, "note": true, "set": true},
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
	// Rule 2: the shell does not do what only the user does. The change tool keeps the
	// gate, and the prompt hook records the user's turns.
	if in.ToolName == "Bash" {
		if reason := atlasCommandRefusal(in.tool().Command); reason != "" {
			return deny(w, reason)
		}
		return nil
	}
	if tool == "work" && in.tool().Action == "start" {
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

// binaryNames are the binary's name and the name it had in 6.0 to 6.2, which an older
// install may still hold.
var binaryNames = []string{"atlas-obsidian", "atlas"}

// atlasCommandRefusal is why a shell command may not run the atlas-obsidian binary, or "":
// a change apply, which would skip the gate, and a hook, which would forge an event.
func atlasCommandRefusal(cmd string) string {
	for _, part := range shellSeparator.Split(cmd, -1) {
		words := strings.Fields(strings.NewReplacer(`"`, "", "'", "").Replace(part))
		for i, word := range words {
			if !slices.Contains(binaryNames, path.Base(word)) {
				continue
			}
			rest := words[i+1:]
			for len(rest) > 0 && strings.HasPrefix(rest[0], "-") {
				rest = rest[1:]
			}
			switch {
			case len(rest) > 0 && rest[0] == "hook":
				return "atlas-obsidian hook runs only from the host; the shell does not send hook events"
			case len(rest) > 0 && rest[0] == "change" && slices.Contains(rest[1:], "apply"):
				return "apply a change with the change tool after the user's yes; the user can also apply it with Apply in Obsidian, or run the command with !"
			case len(rest) > 1 && rest[0] == "vault" && rest[1] == "migrate":
				return "the migration rewrites the whole vault, so only the user runs it: ask the user to type atlas-obsidian vault migrate, or run it with !"
			}
		}
	}
	return ""
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
		if agent == "spec-review" && gitRead.MatchString(cmd) && !shellOperator.MatchString(cmd) {
			for _, f := range strings.Fields(cmd) {
				if f == "-c" || strings.HasPrefix(f, "--output") || f == "--ext-diff" || f == "--textconv" {
					return "spec-review may run git log, git diff, and git show without " + f
				}
			}
			return ""
		}
		if agent == "spec-review" {
			return "spec-review may run only git log, git diff, and git show (git -C <repository> log …), with no shell operator"
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
	case path.Dir(rel) == vault.Documents:
		return documentRefusal(v, in, f, rel)
	case strings.HasPrefix(rel, vault.Assets+"/"):
		return rel + " is a captured original or an attachment; source capture writes the originals, and you add attachments in Obsidian"
	case strings.HasPrefix(rel, vault.Wiki+"/"):
		return rel + " is in wiki/, which holds wiki/documents and wiki/assets only; a document comes from the work tool, change propose, or source capture"
	case strings.HasPrefix(rel, vault.Changes+"/"):
		return rel + " is a change document; the change tool writes it. Edit a proposed document inside it only when the user asks"
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

// documentRefusal keeps the documents of wiki/documents to their writers: a new file
// comes from a tool; knowledge changes through a change; a work document's prose is the
// model's, and its frontmatter, lead callout, and code sections are code's.
func documentRefusal(v *vault.Vault, in Input, f patchFile, rel string) string {
	data, err := v.Read(rel)
	if err != nil {
		if f.Op == "delete" {
			return ""
		}
		return rel + " would be a new document; a stub or a spec comes from the work tool, a topic or a repository from change propose, a source from source capture"
	}
	d := doc.Parse(rel, data)
	t := schema.Get(d.Type())
	switch {
	case t == nil || d.Front == nil:
		return ""
	case t.Family == schema.Knowledge:
		return rel + " is a " + d.Type() + ", which changes only through a change: build a plan, call change propose, show the preview, and apply after the user's yes"
	case f.Op != "update" || in.ToolName == "Write":
		return rel + " is a " + d.Type() + "; revise its prose with Edit, and its fields with work set"
	case touchesPrefix(in, f, d):
		return "the frontmatter and the lead callout of " + d.Title() + " are code's; use work set, or the work action that fits"
	}
	for _, s := range t.CodeSections {
		if start, end := sectionBounds(d.Content, s); start >= 0 && touchesRange(in, f, d.Content, start, end) {
			return "## " + s + " is code's; it follows the documents and the events"
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

// repositoryRefusal is the edit rule: an edit inside a linked repository needs a plan
// this session started, still started, that names the repository.
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
	if s != nil {
		for _, title := range s.List("specs") {
			if covers(v, doc.LinkTarget(title), repo.Title) {
				return ""
			}
		}
	}
	return fmt.Sprintf("an edit in %s needs a started plan that names it. Find or write one with the spec-work skill, then call work start on it, and edit", repo.Title)
}

// covers reports whether a plan is started and names the repository.
func covers(v *vault.Vault, title, repo string) bool {
	spec := sessions.SpecDoc(v, title)
	if spec == nil || spec.Type() != "spec" || spec.Str("status") != "started" {
		return false
	}
	for _, r := range spec.List("repositories") {
		if strings.EqualFold(doc.LinkTarget(r), repo) {
			return true
		}
	}
	return false
}

// restartRefusal refuses a second start of a plan this session already started and that
// is still started: one continued event per session, and the session is bound already.
func restartRefusal(in Input, env Env) string {
	key := strings.TrimSpace(in.tool().Spec)
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
	spec := sessions.SpecDoc(v, doc.LinkTarget(key))
	if spec == nil {
		if idx, err := vault.Load(v); err == nil {
			if d := idx.ByID(key); d != nil {
				spec = d
			}
		}
	}
	if spec == nil || spec.Str("status") != "started" {
		return ""
	}
	for _, l := range s.List("specs") {
		if strings.EqualFold(doc.LinkTarget(l), spec.Title()) {
			return spec.Title() + " is started in this session already; go on with the work, and add a line to its ## Progress"
		}
	}
	return ""
}
