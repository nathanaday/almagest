// Package sessions keeps one document per agent session in sessions/. The hooks create
// it and keep every field; the agent writes three sections of its own document with
// Edit: a description, its progress, and a summary.
package sessions

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/doc"
	"github.com/nathanaday/atlas-obsidian/v2-exp/internal/vault"
)

// Statuses.
const (
	Running = "running"
	Waiting = "waiting"
	Idle    = "idle"
	Ended   = "ended"
	Lost    = "lost"
)

// Live reports whether a status holds its threads and tasks.
func Live(status string) bool {
	return status == Running || status == Waiting || status == Idle
}

// ReadOnlyAgents are the plugin's read-only workers. The guard refuses their writes.
var ReadOnlyAgents = []string{"wiki-extract", "wiki-draft", "wiki-audit", "thread-review"}

// quietAgents are the host's own read-only agent types, which also get a line in their
// parent's document and no document of their own.
var quietAgents = []string{"Explore", "Plan", "claude-code-guide", "statusline-setup"}

// AgentName is an agent type without its plugin prefix: "plugin:wiki-extract" is
// "wiki-extract".
func AgentName(agentType string) string {
	if i := strings.LastIndex(agentType, ":"); i >= 0 {
		return agentType[i+1:]
	}
	return agentType
}

// ReadOnly reports whether an agent type is one of the plugin's read-only workers.
func ReadOnly(agentType string) bool {
	return slices.Contains(ReadOnlyAgents, AgentName(agentType))
}

// Worker reports whether a subagent gets a line in its parent's document instead of a
// document of its own.
func Worker(agentType string) bool {
	return ReadOnly(agentType) || slices.Contains(quietAgents, AgentName(agentType))
}

// Event is what a hook knows of the session it runs for.
type Event struct {
	Harness   string
	SessionID string
	AgentID   string
	AgentType string
	Cwd       string
	// Source is how the session started: startup, resume, clear, or compact.
	Source string
	// Detail is a subagent's task, when the host gives it.
	Detail string
}

// Key is the id the hooks look a session up by: the agent id of a subagent, else the
// session id.
func (e Event) Key() string {
	if e.AgentID != "" {
		return e.AgentID
	}
	return e.SessionID
}

// Short is the first six hex characters of a harness id.
func Short(id string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(id) {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') {
			b.WriteRune(r)
			if b.Len() == 6 {
				break
			}
		}
	}
	s := b.String()
	for len(s) < 6 {
		s += "0"
	}
	return s
}

// Find is the session document of a harness id, or nil.
func Find(v *vault.Vault, key string) *doc.Doc {
	if key == "" {
		return nil
	}
	matches, _ := filepath.Glob(v.Abs(vault.Sessions + "/*/*" + Short(key) + ".md"))
	for _, abs := range matches {
		data, err := os.ReadFile(abs)
		if err != nil {
			continue
		}
		d := doc.Parse(v.Rel(abs), data)
		if d.Str("harness_id") == key {
			return d
		}
	}
	return nil
}

// fields is a new session document's frontmatter, in order.
func fields(id string, e Event, now time.Time, parent, agent string) []doc.Field {
	harness := e.Harness
	if harness == "" {
		harness = "claude"
	}
	return []doc.Field{
		{Key: "id", Value: id},
		{Key: "type", Value: "session"},
		{Key: "created", Value: vault.Date(now)},
		{Key: "updated", Value: vault.Stamp(now)},
		{Key: "harness", Value: harness},
		{Key: "harness_id", Value: e.Key()},
		{Key: "status", Value: Running},
		{Key: "started", Value: vault.Stamp(now)},
		{Key: "ended", Value: ""},
		{Key: "last_prompt", Value: ""},
		{Key: "cwd", Value: vault.Shorten(e.Cwd)},
		{Key: "parent", Value: parent},
		{Key: "agent", Value: agent},
		{Key: "threads", Value: []string{}},
		{Key: "tasks", Value: []string{}},
		{Key: "repositories", Value: []string{}},
		{Key: "changes", Value: []string{}},
		{Key: "description", Value: ""},
		{Key: "reminded", Value: []string{}},
	}
}

const newBody = "## Description\n\n## Progress\n\n## Summary\n\n## Subagents\n"

// Start creates the session document, or sets the existing one running again. The
// caller holds the lock. It returns the document's path.
func Start(v *vault.Vault, e Event, now time.Time) (string, error) {
	if d := Find(v, e.Key()); d != nil {
		content := doc.SetFields(d.Content, []doc.Field{{Key: "status", Value: Running}, {Key: "ended", Value: ""}, {Key: "updated", Value: vault.Stamp(now)}})
		return d.Path, write(v, d.Path, content)
	}
	name := now.Format("2006-01-02 1504") + " " + Short(e.Key())
	rel := fmt.Sprintf("%s/%s/%s.md", vault.Sessions, now.Format("2006-01"), name)
	content := doc.Render(fields("ses-"+Short(e.Key()), e, now, "", ""), newBody)
	return rel, write(v, rel, content)
}

// Subagent handles SubagentStart: a subagent that may write gets its own document, which
// inherits its parent's threads and tasks so the thread rule holds inside it; a worker
// gets a line under its parent's Subagents. The caller holds the lock.
func Subagent(v *vault.Vault, e Event, now time.Time) (string, error) {
	parent := Find(v, e.SessionID)
	if parent == nil {
		p, err := Start(v, Event{Harness: e.Harness, SessionID: e.SessionID, Cwd: e.Cwd}, now)
		if err != nil {
			return "", err
		}
		data, _ := v.Read(p)
		parent = doc.Parse(p, data)
	}
	agent := AgentName(e.AgentType)
	if Worker(e.AgentType) {
		line := fmt.Sprintf("- %s · `%s` · started %s", agent, Short(e.AgentID), now.Format("15:04"))
		if e.Detail != "" {
			line += " · " + oneLine(e.Detail, 120)
		}
		front, body, _ := doc.Split(parent.Content)
		content := doc.Join(front, doc.AppendSection(body, "Subagents", line))
		return parent.Path, write(v, parent.Path, content)
	}
	if d := Find(v, e.AgentID); d != nil {
		return Start(v, e, now)
	}
	rel := strings.TrimSuffix(parent.Path, ".md") + " · " + Short(e.AgentID) + ".md"
	list := fields("ses-"+Short(e.AgentID), e, now, doc.Link(parent.Title()), agent)
	for i := range list {
		switch list[i].Key {
		case "threads", "tasks":
			list[i].Value = nonNil(parent.List(list[i].Key))
		}
	}
	content := doc.Render(list, newBody)
	if err := write(v, rel, content); err != nil {
		return "", err
	}
	return rel, nil
}

// SubagentStop ends a subagent's document, or its worker line. The caller holds the lock.
func SubagentStop(v *vault.Vault, e Event, now time.Time) error {
	if d := Find(v, e.AgentID); d != nil {
		return SetStatus(v, d, Ended, now)
	}
	parent := Find(v, e.SessionID)
	if parent == nil {
		return nil
	}
	front, body, _ := doc.Split(parent.Content)
	section, _ := doc.Section(body, "Subagents")
	marker := "`" + Short(e.AgentID) + "`"
	lines := strings.Split(section, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.Contains(lines[i], marker) && !strings.Contains(lines[i], "· ended") {
			lines[i] += " · ended " + now.Format("15:04")
			break
		}
	}
	content := doc.Join(front, doc.SetSection(body, "Subagents", strings.Join(lines, "\n")))
	return write(v, parent.Path, content)
}

// SetStatus sets a session's status, and its end time when it ends. The caller holds the
// lock.
func SetStatus(v *vault.Vault, d *doc.Doc, status string, now time.Time) error {
	set := []doc.Field{{Key: "status", Value: status}, {Key: "updated", Value: vault.Stamp(now)}}
	if status == Ended {
		set = append(set, doc.Field{Key: "ended", Value: vault.Stamp(now)})
	}
	return write(v, d.Path, doc.SetFields(d.Content, set))
}

// Touch applies fn to a session's content and writes it when it changed, with updated
// at most once a minute unless the status changed. The caller holds the lock. A session
// with no document gets one, so every session has its record even when the hooks came
// late.
func Touch(v *vault.Vault, e Event, now time.Time, fn func(content string) string) (*doc.Doc, error) {
	if e.AgentID != "" && Worker(e.AgentType) {
		return nil, nil
	}
	d := Find(v, e.Key())
	if d == nil {
		var p string
		var err error
		if e.AgentID != "" {
			p, err = Subagent(v, e, now)
			if Worker(e.AgentType) {
				return nil, err
			}
		} else {
			p, err = Start(v, Event{Harness: e.Harness, SessionID: e.SessionID, Cwd: e.Cwd, Source: "resume"}, now)
		}
		if err != nil {
			return nil, err
		}
		if d = Find(v, e.Key()); d == nil {
			return nil, fmt.Errorf("session document %s did not appear", p)
		}
	}
	content := d.Content
	if fn != nil {
		content = fn(content)
	}
	// The agent may write its Description by any means; the field follows it.
	content = doc.SetField(content, "description", DescriptionLine(doc.Parse("", []byte(content)).Body))
	next := doc.Parse(d.Path, []byte(content))
	last, _ := vault.ParseTime(d.Str("updated"))
	if next.Str("status") != d.Str("status") || now.Sub(last) >= time.Minute {
		content = doc.SetField(content, "updated", vault.Stamp(now))
	}
	if content == d.Content {
		return d, nil
	}
	if err := write(v, d.Path, content); err != nil {
		return nil, err
	}
	return doc.Parse(d.Path, []byte(content)), nil
}

// AddLink adds a link to a list field of a session's content, once.
func AddLink(content, field, title string) string {
	d := doc.Parse("", []byte(content))
	list := d.List(field)
	link := doc.Link(title)
	for _, l := range list {
		if strings.EqualFold(doc.LinkTarget(l), title) {
			return content
		}
	}
	return doc.SetField(content, field, append(nonNil(list), link))
}

// MarkLost sets every live session with no hook event for staleHours to lost, and
// returns the paths it wrote. The caller holds the lock.
func MarkLost(v *vault.Vault, now time.Time, staleHours int) ([]string, error) {
	var out []string
	for _, d := range All(v) {
		if !Live(d.Str("status")) {
			continue
		}
		last, ok := vault.ParseTime(d.Str("updated"))
		if !ok || now.Sub(last) < time.Duration(staleHours)*time.Hour {
			continue
		}
		if err := write(v, d.Path, doc.SetField(d.Content, "status", Lost)); err != nil {
			return out, err
		}
		out = append(out, d.Path)
	}
	return out, nil
}

// All reads every session document.
func All(v *vault.Vault) []*doc.Doc {
	matches, _ := filepath.Glob(v.Abs(vault.Sessions + "/*/*.md"))
	var out []*doc.Doc
	for _, abs := range matches {
		data, err := os.ReadFile(abs)
		if err != nil {
			continue
		}
		d := doc.Parse(v.Rel(abs), data)
		if d.Type() == "session" {
			out = append(out, d)
		}
	}
	return out
}

// Lead is a session's lead callout: its status, the task it started last that is still
// open with that task's last progress line, and the repository it edited last.
func Lead(v *vault.Vault, d *doc.Doc) string {
	parts := []string{d.Str("status")}
	var quote string
	tasks := d.List("tasks")
	for i := len(tasks) - 1; i >= 0; i-- {
		title := doc.LinkTarget(tasks[i])
		task := ThreadDoc(v, title)
		if task == nil || task.Str("status") != "open" {
			continue
		}
		thread := doc.LinkTarget(task.Str("thread"))
		label := title
		if rest, ok := strings.CutPrefix(title, thread+" — "); ok {
			label, _, _ = strings.Cut(rest, " ")
		}
		parts = append(parts, label+" of "+doc.Link(thread))
		progress, _ := doc.Section(task.Body, "Progress")
		quote = doc.LastLine(progress)
		break
	}
	if len(parts) == 1 {
		if threads := d.List("threads"); len(threads) > 0 {
			parts = append(parts, threads[len(threads)-1])
		}
	}
	if repos := d.List("repositories"); len(repos) > 0 {
		parts = append(parts, repos[len(repos)-1])
	}
	var lines []string
	if quote != "" {
		lines = append(lines, oneLine(quote, 160))
	} else if desc := d.Str("description"); desc != "" {
		lines = append(lines, oneLine(desc, 160))
	}
	return doc.Callout("session", strings.Join(parts, " · "), lines...)
}

// ThreadDoc finds a thread document by its title: threads/<thread>/<title>.md.
func ThreadDoc(v *vault.Vault, title string) *doc.Doc {
	matches, _ := filepath.Glob(v.Abs(path.Join(vault.Threads, "*", escapeGlob(title)+".md")))
	for _, abs := range matches {
		if data, err := os.ReadFile(abs); err == nil {
			return doc.Parse(v.Rel(abs), data)
		}
	}
	return nil
}

func escapeGlob(s string) string {
	r := strings.NewReplacer("*", `\*`, "?", `\?`, "[", `\[`, "]", `\]`)
	return r.Replace(s)
}

// write writes a session document with its lead callout current.
func write(v *vault.Vault, rel, content string) error {
	d := doc.Parse(rel, []byte(content))
	content = doc.ReplaceLead(content, Lead(v, d))
	_, err := v.WriteIfChanged(rel, []byte(content))
	return err
}

// Refresh rewrites a session's lead callout, for sync. The caller holds the lock.
func Refresh(v *vault.Vault, d *doc.Doc) (bool, error) {
	content := doc.ReplaceLead(d.Content, Lead(v, d))
	return v.WriteIfChanged(d.Path, []byte(content))
}

func nonNil(list []string) []string {
	if list == nil {
		return []string{}
	}
	return list
}

func oneLine(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len([]rune(s)) > n {
		s = string([]rune(s)[:n-1]) + "…"
	}
	return s
}

// DescriptionLine is the first line of a session's Description section, which the
// touched hook copies into the description field.
func DescriptionLine(body string) string {
	text, _ := doc.Section(body, "Description")
	return oneLine(doc.FirstLine(text), 200)
}
