package hooks

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
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

// readActions are, per atlas tool, the actions that only read; a tool listed with nil
// only reads. A read-only agent makes no other call: an action or a tool this list does
// not know is a write until someone lists it here.
var readActions = map[string]map[string]bool{
	"search":   nil,
	"context":  nil,
	"match":    nil,
	"lint":     nil,
	"checkout": {"": true, "list": true, "candidates": true},
	"vault":    {"": true, "status": true},
	"change":   {"": true, "show": true},
	"source":   {"chunks": true, "read": true},
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
	if !editTools[in.ToolName] {
		return nil
	}
	for _, f := range in.paths() {
		// Every rule judges the file as the disk names it, whatever case, Unicode form,
		// or link the agent wrote.
		f.Path = canonical(f.Path)
		if strings.EqualFold(f.Path, canonical(vault.HomeFrom(env.getenv).ConfigPath())) {
			return deny(w, f.Path+" holds the commands Atlas runs (terminal_command, agent_commands); the user changes it in the Atlas settings in Obsidian, or with atlas-obsidian config")
		}
		// A file belongs to the vault above it, wherever the session runs.
		root := vault.FindAbove(filepath.Dir(f.Path))
		if root == "" {
			continue
		}
		v, err := vault.Open(root)
		if err != nil {
			continue
		}
		if reason := pathRefusal(v, in, f); reason != "" {
			return deny(w, reason)
		}
	}
	return nil
}

// binaryNames are the binary's name and the name it had in 6.0 to 6.2, which an older
// install may still hold.
var binaryNames = []string{"atlas-obsidian", "atlas"}

// atlasCommandRefusal is why a shell command may not run the atlas-obsidian binary, or "":
// a change apply, which would skip the gate, and a hook, which would forge an event.
func atlasCommandRefusal(cmd string) string {
	for _, c := range splitCommands(cmd, 4) {
		if why := commandRefusal(c.words); why != "" {
			if c.pipe < 0 {
				why += ". The guard read this from quoted text or a heredoc, because a shell in its pipeline runs that text as a command; text meant only as text goes in a file (git commit -F <file>) or in a line with no shell"
			}
			return why
		}
	}
	return ""
}

// commandRefusal is why one simple command may not run, or "".
func commandRefusal(words []string) string {
	for i, word := range words {
		// zsh runs =name as the path of name.
		name := path.Base(strings.TrimPrefix(word, "="))
		if !slices.ContainsFunc(binaryNames, func(n string) bool { return strings.EqualFold(n, name) }) {
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
			return "apply a change with the change tool after the user's yes; the user can also press Approve in the change document in Obsidian, or run the command with !"
		case rest[0] == "config" && (slices.Contains(rest[1:], "set") || slices.Contains(rest[1:], "unset")) && slices.ContainsFunc(rest[1:], func(w string) bool { return w == "terminal_command" || strings.HasPrefix(w, "agent_commands") }):
			return "terminal_command and agent_commands are the commands Atlas runs, so only the user sets them: in the Atlas settings in Obsidian, or by typing the command with !"
		case rest[0] == "journal" && slices.Contains(rest[1:], "publish"):
			return "a journal is published when the user decides: ask the user to press Publish in the Atlas palette"
		case rest[0] == "vault" && slices.Contains(rest[1:], "trash"):
			return "safe delete is the user's act: it applies a remove at once, with no yes; propose a remove with the change tool, or ask the user to press Safe delete in the Atlas palette"
		case rest[0] == "vault" && slices.Contains(rest[1:], "migrate"):
			return "the migration rewrites the whole vault, so only the user runs it: ask the user to type atlas-obsidian vault migrate, or run it with !"
		}
	}
	return ""
}

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
		return agent + " is read-only and runs no shell command"
	}
	return ""
}

// pathRefusal is why a write tool may not touch a file, or "".
func pathRefusal(v *vault.Vault, in Input, f patchFile) string {
	rel := v.Rel(f.Path)
	if rel == "" {
		return ""
	}
	// A part that does not exist yet keeps the case the agent typed, which a disk that
	// ignores case folds into the real folder, so fixed names compare without case.
	key := strings.ToLower(rel)
	is := func(name string) bool { return key == strings.ToLower(name) }
	under := func(dir string) bool { return strings.HasPrefix(key, strings.ToLower(dir)+"/") }
	switch {
	case is(vault.VaultConfigFile):
		return rel + " holds the commands Atlas runs for this vault (terminal_command, agent_commands), which win over the machine's; the user changes it in the Atlas settings in Obsidian, or with atlas-obsidian config"
	case is(vault.PluginDir) || under(vault.PluginDir):
		return rel + " is the Atlas plugin, whose code and binaryPath decide what runs; vault init and vault sync install it, and the user sets binaryPath in the Atlas settings in Obsidian"
	case strings.EqualFold(path.Dir(rel), vault.Documents):
		return documentRefusal(v, f, rel)
	case under(vault.Originals):
		return rel + " is a captured original or an attachment; source capture writes the originals, and you add attachments in Obsidian"
	case under(vault.Core):
		return rel + " is in " + vault.Core + "/, which holds " + vault.Documents + " and " + vault.Originals + " only; a document comes from change propose or source capture"
	case under(vault.Changes):
		return rel + " is a change document, and only the change tool writes it. To change a proposed change, propose a new one with supersedes; the user can edit one in Obsidian before pressing Approve"
	case under(vault.WikiView):
		return rel + " is a view, which code writes from the documents; change the documents instead"
	case is(vault.Marker):
		return "Atlas.md is the user's; ask the user to edit it"
	case strings.HasSuffix(key, ".base"):
		return rel + " is a Base that vault init ships; ask the user to change it in Obsidian"
	case is(vault.Settings):
		return rel + " lists the linked repositories; vault sync keeps it"
	case under(vault.Sessions):
		return sessionRefusal(v, in, f, rel)
	case under(vault.Checkout):
		return rel + " is in checkout/, which the checkout tool writes; the user reads and edits the copies, and Return proposes their edits"
	case under(vault.Trash):
		return rel + " is in the trash, which holds what the user deleted; the user empties it"
	case under(vault.Journals):
		return rel + " is in a journal, the user's own writing, which no agent changes; a journal reaches the wiki when the user publishes it"
	}
	return ""
}

// documentRefusal keeps the documents of source-core/documents to their writers: a new file
// comes from a tool, and knowledge changes through a change.
func documentRefusal(v *vault.Vault, f patchFile, rel string) string {
	data, err := v.Read(rel)
	if err != nil {
		if f.Op == "delete" {
			return ""
		}
		return rel + " would be a new document; a topic or a repository comes from change propose, a source from source capture"
	}
	d := doc.Parse(rel, data)
	if t := schema.Get(d.Type()); t != nil && d.Front != nil && t.Document() {
		return rel + " is a " + d.Type() + ", which changes only through a change: build a plan, call change propose, show the preview, and apply after the user's yes"
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
	if unplaced(in, f, d.Content) {
		return unplacedWhy(rel)
	}
	after, applies := in.leaves(d.Content, f)
	whole := applies && in.ToolName != "apply_patch"
	if !whole && touchesPrefix(in, f, d) {
		return "the frontmatter and the lead callout are the hooks'; " + allowed
	}
	if start, end := sectionBounds(d.Content, "Subagents"); !whole && start >= 0 && touchesRange(in, f, d.Content, start, end) {
		return "Subagents is the hooks'; " + allowed
	}
	if applies {
		switch codeChanged(d, after, []string{"Subagents"}) {
		case "":
		case prefixPart:
			return "the frontmatter and the lead callout are the hooks'; " + allowed
		default:
			return "Subagents is the hooks'; " + allowed
		}
	}
	return ""
}

// unplaced reports whether a patch holds a hunk that fits nowhere in the file, so the
// guard cannot tell where it lands.
func unplaced(in Input, f patchFile, content string) bool {
	if in.ToolName != "apply_patch" {
		return false
	}
	_, ok := in.leaves(content, f)
	return !ok
}

func unplacedWhy(rel string) string {
	return "the guard cannot tell where a hunk of this patch lands in " + rel + ": its context and removed lines match no lines of the file. Read the file again, and give each hunk context lines as the file holds them"
}

// prefixPart names the frontmatter and the lead callout in codeChanged's answer.
const prefixPart = "the frontmatter and the lead"

// codeChanged is what code owns that the content after an edit changes: prefixPart, the
// name of a code section, or "". A code section changes when its text differs, or when
// the document no longer holds its heading as often as before, by doc.Headings.
func codeChanged(d *doc.Doc, after string, code []string) string {
	a := doc.Parse(d.Path, []byte(after))
	if d.Content[:prefixEnd(d)] != after[:prefixEnd(a)] {
		return prefixPart
	}
	count := func(body, title string) int {
		n := 0
		for _, h := range doc.Headings(body) {
			if h.Level == 2 && strings.EqualFold(strings.TrimSpace(h.Title), title) {
				n++
			}
		}
		return n
	}
	for _, s := range code {
		was, _ := doc.Section(d.Body, s)
		is, _ := doc.Section(a.Body, s)
		if count(d.Body, s) != count(a.Body, s) || was != is {
			return s
		}
	}
	return ""
}

// prefixEnd is the offset where a document's own text begins: after the frontmatter and
// the code-owned lead callout.
func prefixEnd(d *doc.Doc) int {
	_, body, ok := doc.Split(d.Content)
	end := 0
	if ok {
		end = len(d.Content) - len(body)
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

// canonical is a path as the disk names it: links on the part that exists are resolved,
// and each part is spelled as its folder entry, which on a disk that ignores case or
// Unicode form may differ from what was written. A part that does not exist yet keeps its
// spelling.
func canonical(p string) string { return canonicalWithin(p, 40) }

// canonicalWithin is canonical that follows at most hops links whose target does not
// exist.
func canonicalWithin(p string, hops int) string {
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
		// The last part that exists is a link to nothing yet; a write through it lands at
		// its target.
		target, lerr := os.Readlink(have)
		if lerr != nil || hops == 0 {
			return p
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(have), target)
		}
		return canonicalWithin(filepath.Join(target, tail), hops-1)
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
