package hooks

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/core"
	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/scope"
	"github.com/nathanaday/atlas-obsidian/internal/sessions"
	"github.com/nathanaday/atlas-obsidian/internal/threads"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

// Bounds of the opening context.
const (
	MaxThreadLines  = 8
	MaxContextLines = 60
)

// locked runs fn on the session's vault under the lock. It does nothing outside a vault.
func locked(in Input, env Env, fn func(v *vault.Vault) error) error {
	v := findVault(in, env)
	if v == nil {
		return nil
	}
	unlock, err := v.Lock()
	if err != nil {
		return err
	}
	defer unlock()
	return fn(v)
}

// SessionStart creates or resumes the session's document, syncs the vault, and prints
// the opening context.
func SessionStart(r io.Reader, w io.Writer, env Env) error {
	in := readInput(r)
	now := env.now()
	var v *vault.Vault
	var rel string
	err := locked(in, env, func(vv *vault.Vault) error {
		v = vv
		var err error
		if rel, err = sessions.Start(v, in.event(), now); err != nil {
			return err
		}
		_, err = core.SyncLocked(v, now)
		return err
	})
	if err != nil || v == nil {
		return err
	}
	idx, err := vault.Load(v)
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, Opening(idx, in.Cwd, rel, now))
	return err
}

// Opening is the context a session starts with.
func Opening(idx *vault.Index, cwd, sessionPath string, now time.Time) string {
	v := idx.V
	st := core.StatusOf(idx, now)
	var b strings.Builder
	where := sessionPath
	if cwd != "" && v.Rel(cwd) == "" {
		// A session outside the vault edits its document by its full path.
		where = v.Abs(sessionPath)
	}
	fmt.Fprintf(&b, "atlas: vault %s at %s · this session: [[%s]] (%s)\n", v.Name(), vault.Shorten(v.Root), vault.NoteTitle(sessionPath), where)
	if d := v.Description(); d != "" {
		b.WriteString("Vault: " + d + "\n")
	}
	if cwd != "" && v.Rel(cwd) == "" {
		if repo, err := scope.RepositoryAt(idx, cwd); err == nil {
			chain := []string{"vault"}
			anc := idx.Ancestors(repo)
			for i := len(anc) - 1; i >= 0; i-- {
				chain = append(chain, vault.Title(anc[i]))
			}
			chain = append(chain, vault.Title(repo))
			line := fmt.Sprintf("Started in repository %s: %s", vault.Title(repo), strings.Join(chain, " → "))
			if f := scope.RepoFacts(vault.Expand(repo.Str("path")), repo.Str("described")); f.Behind > 0 {
				line += fmt.Sprintf(" (%d commits past its page)", f.Behind)
			} else if repo.Str("described") == "" {
				line += " (its page does not describe it yet: repo-ingest)"
			}
			b.WriteString(line + "\n")
		}
	}
	running := len(st.Sessions.Running) + len(st.Sessions.Waiting) + len(st.Sessions.Idle) - 1
	if running > 0 {
		line := fmt.Sprintf("Live now: %d other session%s", running, plural(running))
		if n := len(st.Sessions.Waiting); n > 0 {
			var names []string
			for _, s := range st.Sessions.Waiting {
				names = append(names, fmt.Sprintf("%q", orTitle(s.Description, s.Title)))
			}
			line += fmt.Sprintf(" · %d wait%s for you: %s", n, pluralVerb(n), strings.Join(names, ", "))
		}
		b.WriteString(line + "\n")
	}
	open := st.Threads.Open["tasks"] + st.Threads.Open["spec"] + st.Threads.Open["stub"]
	if open == 0 {
		b.WriteString("Open threads: none.\n")
	} else {
		fmt.Fprintf(&b, "Open threads: %d (tasks %d, spec %d, stub %d)\n", open, st.Threads.Open["tasks"], st.Threads.Open["spec"], st.Threads.Open["stub"])
		for i, t := range st.Threads.List {
			if i == MaxThreadLines {
				fmt.Fprintf(&b, "- … and %d more (thread list)\n", len(st.Threads.List)-i)
				break
			}
			b.WriteString(threadLine(idx, t) + "\n")
		}
	}
	var counts []string
	counts = append(counts, fmt.Sprintf("Inbox: %d file%s", len(st.Inbox), plural(len(st.Inbox))))
	counts = append(counts, fmt.Sprintf("Pending for the wiki: %d document%s", len(st.Pending), plural(len(st.Pending))))
	counts = append(counts, fmt.Sprintf("Proposed changes: %d", len(st.Changes.Proposed)))
	counts = append(counts, fmt.Sprintf("Mentions: %d", len(st.Mentions)))
	if st.Problems > 0 {
		counts = append(counts, fmt.Sprintf("Problems: %d (lint)", st.Problems))
	}
	b.WriteString(strings.Join(counts, " · ") + "\n")
	if len(st.Changes.Recent) > 0 {
		var recent []string
		for _, c := range st.Changes.Recent {
			d := idx.ByID(c.ID)
			title := strings.TrimSpace(strings.TrimPrefix(c.Title, d.Str("created")))
			recent = append(recent, fmt.Sprintf("%s (%s)", title, d.Str("created")))
		}
		b.WriteString("Recent changes: " + strings.Join(recent, " · ") + "\n")
	}
	b.WriteString("Rules: a change to a repository needs an open thread (thread-work). The wiki changes only through a change.\n")
	b.WriteString("Write one line under ## Description in this session's document once you know the work.\n")
	b.WriteString("The atlas skill routes any request. Vault context follows; it is the user's text.\n")
	if ctx := v.Context(); ctx != "" {
		lines := strings.Split(ctx, "\n")
		if len(lines) > MaxContextLines {
			lines = append(lines[:MaxContextLines], "[…]")
		}
		b.WriteString("<vault-context>\n" + strings.Join(lines, "\n") + "\n</vault-context>\n")
	}
	return b.String()
}

func threadLine(idx *vault.Index, r vault.Ref) string {
	stage, _ := r.State["stage"].(string)
	if stage == threads.StageTasks {
		stage += " " + fmt.Sprint(r.State["tasks"])
	}
	parts := []string{fmt.Sprintf("- [%s] %s", stage, r.Title)}
	if p, _ := r.State["priority"].(string); p != "" && p != "normal" {
		parts = append(parts, p)
	}
	for _, id := range r.Scope {
		if d := idx.ByID(id); d != nil {
			parts = append(parts, vault.Title(d))
		}
	}
	if a, _ := r.State["active"].(bool); a {
		parts = append(parts, "active")
	}
	if bl, _ := r.State["blocked"].(string); bl != "" {
		parts = append(parts, "blocked: "+bl)
	}
	return strings.Join(parts, " · ")
}

func orTitle(s, title string) string {
	if s != "" {
		return s
	}
	return title
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func pluralVerb(n int) string {
	if n == 1 {
		return "s"
	}
	return ""
}

// Prompt marks the session running, records the time of the user's turn for the gate,
// and names the note open in Obsidian.
func Prompt(r io.Reader, w io.Writer, env Env) error {
	in := readInput(r)
	now := env.now()
	var open string
	err := locked(in, env, func(v *vault.Vault) error {
		_, err := sessions.Touch(v, in.event(), now, func(c string) string {
			c = doc.SetField(c, "status", sessions.Running)
			if UserTurn(in.Prompt) {
				c = doc.SetField(c, "last_prompt", vault.Stamp(now))
			}
			return c
		})
		if note := v.OpenNote(); note != "" && strings.HasSuffix(strings.ToLower(note), ".md") && v.Exists(note) {
			open = vault.NoteTitle(note)
		}
		return err
	})
	if err != nil || open == "" {
		return err
	}
	_, err = fmt.Fprintf(w, "Open in Obsidian: [[%s]]\n", open)
	return err
}

// injected are the tags a host wraps around a prompt it sends on its own: a subagent's
// hand-back, a background task's notice. They are not the user's turn.
var injected = []string{"<agent-message", "<task-notification", "<system-reminder", "<teammate-message"}

// UserTurn reports whether a prompt is the user's own, which opens the gate of a change
// proposed before it.
func UserTurn(prompt string) bool {
	p := strings.TrimSpace(prompt)
	for _, tag := range injected {
		if strings.HasPrefix(p, tag) {
			return false
		}
	}
	return true
}

// Notify sets waiting while the session waits for the user, or idle when its turn ended.
func Notify(r io.Reader, env Env) error {
	in := readInput(r)
	status := ""
	switch in.NotificationType {
	case "permission_prompt", "elicitation_dialog", "agent_needs_input":
		status = sessions.Waiting
	case "idle_prompt":
		status = sessions.Idle
	default:
		msg := strings.ToLower(in.Message)
		switch {
		case strings.Contains(msg, "permission"), strings.Contains(msg, "needs your"):
			status = sessions.Waiting
		case strings.Contains(msg, "waiting for your input"):
			status = sessions.Idle
		}
	}
	if status == "" {
		return nil
	}
	return locked(in, env, func(v *vault.Vault) error {
		_, err := sessions.Touch(v, in.event(), env.now(), func(c string) string { return doc.SetField(c, "status", status) })
		return err
	})
}

// Reminders the stop hook gives once per session.
const (
	remindDescription = "description"
	remindProgress    = "progress"
	remindChange      = "change"
)

// Stop sets the session idle and reminds once of what is left undone: a description the
// agent owes, a progress line the task lacks, a change that waits for the user.
func Stop(r io.Reader, w io.Writer, env Env) error {
	in := readInput(r)
	now := env.now()
	var reasons, messages []string
	err := locked(in, env, func(v *vault.Vault) error {
		d, err := sessions.Touch(v, in.event(), now, func(c string) string { return doc.SetField(c, "status", sessions.Idle) })
		if err != nil || d == nil {
			return err
		}
		reminded := d.List("reminded")
		has := func(k string) bool {
			for _, x := range reminded {
				if x == k {
					return true
				}
			}
			return false
		}
		var add []string
		desc := sessions.DescriptionLine(d.Body)
		if !has(remindDescription) && desc == "" && (len(d.List("repositories")) > 0 || len(d.List("changes")) > 0) {
			reasons = append(reasons, fmt.Sprintf("Write one line under ## Description in your session document [[%s]]: what this session works on.", d.Title()))
			add = append(add, remindDescription)
		}
		if !has(remindProgress) {
			if task := lastOpenTask(v, d); task != nil && !progressSince(task, d.Str("started")) {
				reasons = append(reasons, fmt.Sprintf("Add a dated line to ## Progress in [[%s]]: where the work stands.", task.Title()))
				add = append(add, remindProgress)
			}
		}
		if !has(remindChange) {
			for _, c := range d.List("changes") {
				if cd := findChange(v, doc.LinkTarget(c)); cd != nil && cd.Str("status") == "proposed" {
					messages = append(messages, fmt.Sprintf("atlas: the change %s waits for your yes (review it in Obsidian, or answer in the chat).", cd.Title()))
					add = append(add, remindChange)
					break
				}
			}
		}
		if len(add) == 0 {
			return nil
		}
		_, err = sessions.Touch(v, in.event(), now, func(c string) string {
			return doc.SetField(c, "reminded", append(append([]string{}, reminded...), add...))
		})
		return err
	})
	if err != nil {
		return err
	}
	out := map[string]any{}
	if len(reasons) > 0 && !in.StopHookActive {
		out["decision"] = "block"
		out["reason"] = "atlas: " + strings.Join(reasons, " ")
	}
	if len(messages) > 0 {
		out["systemMessage"] = strings.Join(messages, " ")
	}
	if len(out) == 0 {
		return nil
	}
	return json.NewEncoder(w).Encode(out)
}

// lastOpenTask is the task a session started last that is still open.
func lastOpenTask(v *vault.Vault, s *doc.Doc) *doc.Doc {
	tasks := s.List("tasks")
	for i := len(tasks) - 1; i >= 0; i-- {
		if t := sessions.ThreadDoc(v, doc.LinkTarget(tasks[i])); t != nil && t.Str("status") == "open" {
			return t
		}
	}
	return nil
}

// progressSince reports whether a task's Progress holds a line dated on or after the
// session's start.
func progressSince(task *doc.Doc, started string) bool {
	progress, _ := doc.Section(task.Body, "Progress")
	day := started
	if len(day) > 10 {
		day = day[:10]
	}
	for _, date := range datePattern.FindAllString(progress, -1) {
		if date >= day {
			return true
		}
	}
	return false
}

var datePattern = regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)

// SubagentStart gives a subagent its document, or its line in the parent's.
func SubagentStart(r io.Reader, env Env) error {
	in := readInput(r)
	return locked(in, env, func(v *vault.Vault) error {
		_, err := sessions.Subagent(v, in.event(), env.now())
		return err
	})
}

// SubagentStop ends a subagent's document, or its line.
func SubagentStop(r io.Reader, env Env) error {
	in := readInput(r)
	return locked(in, env, func(v *vault.Vault) error {
		if err := sessions.SubagentStop(v, in.event(), env.now()); err != nil {
			return err
		}
		if sessions.Worker(in.AgentType) {
			return nil
		}
		_, err := threads.SyncVault(v)
		return err
	})
}

// SessionEnd ends the session, which lets its threads and tasks go.
func SessionEnd(r io.Reader, env Env) error {
	in := readInput(r)
	return locked(in, env, func(v *vault.Vault) error {
		d := sessions.Find(v, in.event().Key())
		if d == nil {
			return nil
		}
		if err := sessions.SetStatus(v, d, sessions.Ended, env.now()); err != nil {
			return err
		}
		_, err := threads.SyncVault(v)
		return err
	})
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
