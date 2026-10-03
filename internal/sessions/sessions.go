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

	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/schema"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

// Statuses.
const (
	Running = "running"
	Waiting = "waiting"
	Idle    = "idle"
	Ended   = "ended"
	Lost    = "lost"
)

// Live reports whether a status holds the threads it started.
func Live(status string) bool {
	return status == Running || status == Waiting || status == Idle
}

// ReadOnlyAgents are the plugin's read-only workers. The guard refuses their writes.
// thread-audit alone runs shell commands, since it checks work by running its tests.
var ReadOnlyAgents = []string{"wiki-extract", "wiki-draft", "wiki-audit", "thread-audit"}

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
	// Transcript is the file where the host keeps the conversation; resume reads it.
	Transcript string
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

// ByTitle finds a session document by its title: sessions/<month>/<title>.md.
func ByTitle(v *vault.Vault, title string) *doc.Doc {
	if title == "" || strings.ContainsAny(title, `/\`) {
		return nil
	}
	matches, _ := filepath.Glob(v.Abs(path.Join(vault.Sessions, "*", escapeGlob(title)+".md")))
	for _, abs := range matches {
		if data, err := os.ReadFile(abs); err == nil {
			if d := doc.Parse(v.Rel(abs), data); d.Type() == "session" {
				return d
			}
		}
	}
	return nil
}

// UserAnswered is the gate of a change the model applies: a change that writes, or that
// closes a thread or a chord by absorbing it, applies only when the user of the session
// that proposed it had a turn after the proposal. A change with no writes that absorbs
// only sources and events needs no answer.
func UserAnswered(v *vault.Vault) func(d *doc.Doc, writes int) error {
	return func(d *doc.Doc, writes int) error {
		if writes == 0 && !closes(v, d) {
			return nil
		}
		wait := fmt.Errorf("show the preview of %s and wait for the user's yes; apply runs once the user has answered after the proposal", vault.Title(d))
		proposed, ok := schema.ParseTime(d.Str("proposed"))
		if !ok {
			return wait
		}
		s := ByTitle(v, doc.LinkTarget(d.Str("session")))
		if s == nil {
			return fmt.Errorf("%s names no session that proposed it, so no user answered it here; the user applies it with Apply in Obsidian", vault.Title(d))
		}
		if last, ok := schema.ParseTime(s.Str("last_prompt")); ok && last.After(proposed) {
			return nil
		}
		return wait
	}
}

// closes reports whether a change absorbs a spec, a verification, or a chord: its apply
// closes a thread or a chord, which is the user's to do.
func closes(v *vault.Vault, d *doc.Doc) bool {
	for _, a := range d.List("absorbs") {
		if x := Document(v, doc.LinkTarget(a)); x != nil {
			switch x.Type() {
			case "spec", "verification", "chord":
				return true
			}
		}
	}
	return false
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
		{Key: "created", Value: vault.Stamp(now)},
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
		{Key: "work", Value: []string{}},
		{Key: "repositories", Value: []string{}},
		{Key: "changes", Value: []string{}},
		{Key: "events", Value: 0},
		{Key: "checked", Value: 0},
		{Key: "description", Value: ""},
		{Key: "reminded", Value: []string{}},
		{Key: "pid", Value: 0},
		{Key: "transcript", Value: e.Transcript},
	}
}

// process gives a session's content the agent's process id and the conversation's file,
// when it lacks them. A subagent's document keeps no transcript: only its parent resumes.
func process(content string, e Event) string {
	d := doc.Parse("", []byte(content))
	if d.Front == nil {
		return content
	}
	if d.Front.Int("pid") == 0 {
		if pid := HarnessPID(); pid > 0 {
			content = doc.SetField(content, "pid", pid)
		}
	}
	if e.Transcript != "" && e.AgentID == "" && d.Str("transcript") == "" {
		content = doc.SetField(content, "transcript", e.Transcript)
	}
	return content
}

const newBody = "## Description\n\n## Progress\n\n## Summary\n\n## Subagents\n"

// Start creates the session document, or sets the existing one running again. The
// caller holds the lock. It returns the document's path.
func Start(v *vault.Vault, e Event, now time.Time) (string, error) {
	if d := Find(v, e.Key()); d != nil {
		content := doc.SetFields(d.Content, []doc.Field{{Key: "status", Value: Running}, {Key: "ended", Value: ""}, {Key: "updated", Value: vault.Stamp(now)}})
		// A resumed session runs in a new process.
		content = process(doc.SetField(content, "pid", 0), e)
		return d.Path, write(v, d.Path, content)
	}
	name := now.Format("2006-01-02 1504") + " " + Short(e.Key())
	rel := fmt.Sprintf("%s/%s/%s.md", vault.Sessions, now.Format("2006-01"), name)
	content := process(doc.Render(fields("ses-"+Short(e.Key()), e, now, "", ""), newBody), e)
	return rel, write(v, rel, content)
}

// Subagent handles SubagentStart: a subagent that may write gets its own document, which
// inherits its parent's started threads so the edit rule holds inside it; a worker
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
		if list[i].Key == "threads" {
			list[i].Value = doc.Links(Threads(parent))
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
	content := process(d.Content, e)
	if fn != nil {
		content = fn(content)
	}
	// The agent may write its Description by any means; the field follows it.
	content = doc.SetField(content, "description", DescriptionLine(doc.Parse("", []byte(content)).Body))
	next := doc.Parse(d.Path, []byte(content))
	last, _ := schema.ParseTime(d.Str("updated"))
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

// MarkLost ends every live session whose agent process is gone, sets every other live
// session with no hook event for staleHours to lost, and returns the paths it wrote. The
// caller holds the lock.
func MarkLost(v *vault.Vault, now time.Time, staleHours int) ([]string, error) {
	var out []string
	for _, d := range All(v) {
		if !Live(d.Str("status")) {
			continue
		}
		if pid := d.Front.Int("pid"); pid > 0 {
			if Alive(pid) {
				continue
			}
			if err := SetStatus(v, d, Ended, now); err != nil {
				return out, err
			}
			out = append(out, d.Path)
			continue
		}
		last, ok := schema.ParseTime(d.Str("updated"))
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

// Threads are the titles of the threads a session started. A session of 7.x holds the
// plans it started in specs, and a plan became a stub with the same title.
func Threads(d *doc.Doc) []string {
	var out []string
	for _, field := range []string{"specs", "threads"} {
		for _, l := range d.List(field) {
			if t := doc.LinkTarget(l); !slices.Contains(out, t) {
				out = append(out, t)
			}
		}
	}
	return out
}

// Lead is a session's lead callout: its status, the thread it started last that is still
// started, the repository it edited last, and its last progress line or its description.
func Lead(v *vault.Vault, d *doc.Doc) string {
	parts := []string{d.Str("status")}
	list := Threads(d)
	for i := len(list) - 1; i >= 0; i-- {
		if stub := Document(v, list[i]); stub != nil && stub.Type() == "stub" && stub.Str("status") == "started" {
			parts = append(parts, doc.Link(list[i]))
			break
		}
	}
	if len(parts) == 1 {
		if w := d.List("work"); len(w) > 0 {
			parts = append(parts, w[len(w)-1])
		}
	}
	if repos := d.List("repositories"); len(repos) > 0 {
		parts = append(parts, repos[len(repos)-1])
	}
	var lines []string
	progress, _ := doc.Section(d.Body, "Progress")
	if quote := doc.LastLine(progress); quote != "" {
		lines = append(lines, oneLine(quote, 160))
	} else if desc := d.Str("description"); desc != "" {
		lines = append(lines, oneLine(desc, 160))
	}
	return doc.Callout("session", strings.Join(parts, " · "), lines...)
}

// Document reads a document of wiki/documents by its title, or nil.
func Document(v *vault.Vault, title string) *doc.Doc {
	if title == "" || strings.ContainsAny(title, `/\`) {
		return nil
	}
	rel := vault.DocPath(title)
	data, err := v.Read(rel)
	if err != nil {
		return nil
	}
	return doc.Parse(rel, data)
}

// TaskLists reads the task lists of a thread by its stub's title: the documents named
// "<title> · Tasks…", fast enough for a hook.
func TaskLists(v *vault.Vault, title string) []*doc.Doc {
	matches, _ := filepath.Glob(v.Abs(vault.DocPath(escapeGlob(title) + " · Tasks*")))
	var out []*doc.Doc
	for _, abs := range matches {
		data, err := os.ReadFile(abs)
		if err != nil {
			continue
		}
		if d := doc.Parse(v.Rel(abs), data); d.Type() == "tasks" && strings.EqualFold(doc.LinkTarget(d.Str("thread")), title) {
			out = append(out, d)
		}
	}
	return out
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

func escapeGlob(s string) string {
	r := strings.NewReplacer("*", `\*`, "?", `\?`, "[", `\[`, "]", `\]`)
	return r.Replace(s)
}
