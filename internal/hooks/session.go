package hooks

import (
	"cmp"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nathanaday/almagest/internal/brief"
	"github.com/nathanaday/almagest/internal/core"
	"github.com/nathanaday/almagest/internal/doc"
	"github.com/nathanaday/almagest/internal/sessions"
	"github.com/nathanaday/almagest/internal/vault"
)

// Bounds of the opening context.
const (
	MaxTagLines     = 30
	MaxContextLines = 60
)

// locked runs fn on the session's vault under the lock. It does nothing outside a vault.
func locked(in Input, env Env, fn func(v *vault.Vault) error) error {
	v := findVault(in, env)
	if v == nil {
		return nil
	}
	wait := env.wait
	if wait == 0 {
		wait = vault.LockWait
	}
	unlock, err := v.LockWithin(wait)
	if err != nil {
		return fmt.Errorf("gave up after %s waiting for the vault lock: %w", wait, err)
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
		if v.CheckLayout() != nil {
			return nil
		}
		_, err = core.SyncLocked(v, now, core.SyncOptions{})
		return err
	})
	if err != nil || v == nil {
		return err
	}
	if err := v.CheckLayout(); err != nil {
		_, err = fmt.Fprintf(w, "almagest: vault %s at %s · %v\n", v.Name(), vault.Shorten(v.Root), err)
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
	fmt.Fprintf(&b, "almagest: vault %s at %s · this session: [[%s]] (%s)\n", v.Name(), vault.Shorten(v.Root), vault.NoteTitle(sessionPath), where)
	if d := v.Description(); d != "" {
		b.WriteString("Vault: " + d + "\n")
	}
	if cwd != "" && v.Rel(cwd) == "" {
		if repo, err := brief.RepositoryAt(idx, cwd); err == nil {
			line := fmt.Sprintf("Started in repository %s", vault.Title(repo))
			if def := repo.Str("defines"); def != "" {
				line += " (tag " + def
			} else {
				line += " ("
			}
			switch n := repo.Front.Int("behind"); {
			case repo.Str("described") == "":
				line += "; not described yet: repo-ingest)"
			case n > 0:
				line += fmt.Sprintf("; %d commits past its description)", n)
			default:
				line += ")"
			}
			b.WriteString(strings.Replace(line, " (; ", " (", 1) + "\n")
		}
	}
	running := len(st.Sessions.Running) + len(st.Sessions.Waiting) + len(st.Sessions.Idle) - 1
	if running > 0 {
		line := fmt.Sprintf("Live now: %d other session%s", running, doc.Plural(running, "", "s"))
		if n := len(st.Sessions.Waiting); n > 0 {
			var names []string
			for _, s := range st.Sessions.Waiting {
				names = append(names, fmt.Sprintf("%q", cmp.Or(s.Description, s.Title)))
			}
			line += fmt.Sprintf(" · %d wait%s for you: %s", n, doc.Plural(n, "s", ""), strings.Join(names, ", "))
		}
		b.WriteString(line + "\n")
	}
	if len(st.Tags) > 0 {
		var parts []string
		for i, t := range st.Tags {
			if i == MaxTagLines {
				parts = append(parts, "…")
				break
			}
			parts = append(parts, fmt.Sprintf("%s %d", t.Tag, t.Count))
		}
		fmt.Fprintf(&b, "Tags (%s mode): %s\n", v.Tagging(), strings.Join(parts, " · "))
	}
	var counts []string
	counts = append(counts, fmt.Sprintf("Ingest: %d file%s", len(st.Ingest), doc.Plural(len(st.Ingest), "", "s")))
	counts = append(counts, fmt.Sprintf("Pending for the wiki: %d document%s", len(st.Pending), doc.Plural(len(st.Pending), "", "s")))
	counts = append(counts, fmt.Sprintf("Proposed changes: %d", len(st.Changes.Proposed)))
	var changed []string
	for _, vol := range st.Journals {
		if vol.Changed {
			changed = append(changed, vol.Volume)
		}
	}
	if len(changed) > 0 {
		counts = append(counts, "Journals to publish: "+strings.Join(changed, ", "))
	}
	if st.Problems > 0 {
		counts = append(counts, fmt.Sprintf("Problems: %d (lint)", st.Problems))
	}
	b.WriteString(strings.Join(counts, " · ") + "\n")
	if len(st.Changes.Recent) > 0 {
		var recent []string
		for _, c := range st.Changes.Recent {
			d := idx.ByID(c.ID)
			day := d.Str("applied")
			if len(day) > 10 {
				day = day[:10]
			}
			title := strings.TrimSpace(strings.TrimPrefix(c.Title, day))
			recent = append(recent, fmt.Sprintf("%s (%s)", title, day))
		}
		b.WriteString("Recent changes: " + strings.Join(recent, " · ") + "\n")
	}
	b.WriteString("Rules: knowledge changes only through a change. Edit a linked repository directly; on long work, add a dated line to ## Progress in this session's document.\n")
	b.WriteString("Write one line under ## Description in this session's document once you know the work.\n")
	b.WriteString("The almagest skill routes any request. Vault context follows; it is the user's text.\n")
	if ctx := v.Context(); ctx != "" {
		lines := strings.Split(ctx, "\n")
		if len(lines) > MaxContextLines {
			lines = append(lines[:MaxContextLines], "[…]")
		}
		b.WriteString("<vault-context>\n" + strings.Join(lines, "\n") + "\n</vault-context>\n")
	}
	return b.String()
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
	remindChange      = "change"
)

// Stop sets the session idle and reminds once of what is left undone: a description the
// agent owes, and a change that waits for the user.
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
		if !has(remindChange) {
			for _, c := range d.List("changes") {
				if cd := findChange(v, doc.LinkTarget(c)); cd != nil && cd.Str("status") == "proposed" {
					messages = append(messages, fmt.Sprintf("almagest: the change %s waits for your yes (review it in Obsidian, or answer in the chat).", cd.Title()))
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
		out["reason"] = "almagest: " + strings.Join(reasons, " ")
	}
	if len(messages) > 0 {
		out["systemMessage"] = strings.Join(messages, " ")
	}
	if len(out) == 0 {
		return nil
	}
	return json.NewEncoder(w).Encode(out)
}

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
		return sessions.SubagentStop(v, in.event(), env.now())
	})
}

// SessionEnd ends the session.
func SessionEnd(r io.Reader, env Env) error {
	in := readInput(r)
	return locked(in, env, func(v *vault.Vault) error {
		d := sessions.Find(v, in.event().Key())
		if d == nil {
			return nil
		}
		return sessions.SetStatus(v, d, sessions.Ended, env.now())
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
