// Package hooks is every hook command: atlas-obsidian hook <event> reads the event's JSON on stdin.
// Hooks keep the rules that must hold (the guard) and the facts about sessions (every
// session document and every link between a session and another document). A hook that
// finds no vault for its session does nothing, so Atlas stays out of sessions that are
// not its own.
package hooks

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/sessions"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

// PluginName is the agent plugin's name, as its tools carry it.
const PluginName = "atlas-obsidian"

// Events are the hook commands, by the event that runs each.
var Events = map[string]string{
	"session-start":  "SessionStart",
	"prompt":         "UserPromptSubmit",
	"guard":          "PreToolUse",
	"touched":        "PostToolUse",
	"notify":         "Notification",
	"stop":           "Stop",
	"subagent-start": "SubagentStart",
	"subagent-stop":  "SubagentStop",
	"session-end":    "SessionEnd",
}

// Input is a hook event, as Claude Code and Codex send it.
type Input struct {
	SessionID        string          `json:"session_id"`
	Cwd              string          `json:"cwd"`
	HookEventName    string          `json:"hook_event_name"`
	Source           string          `json:"source"`
	Prompt           string          `json:"prompt"`
	ToolName         string          `json:"tool_name"`
	ToolInput        json.RawMessage `json:"tool_input"`
	ToolResponse     json.RawMessage `json:"tool_response"`
	AgentID          string          `json:"agent_id"`
	AgentType        string          `json:"agent_type"`
	ParentSessionID  string          `json:"parentSessionId"`
	ParentSession2   string          `json:"parent_session_id"`
	NotificationType string          `json:"notification_type"`
	Message          string          `json:"message"`
	StopHookActive   bool            `json:"stop_hook_active"`
	Reason           string          `json:"reason"`
	TurnID           string          `json:"turn_id"`
	TranscriptPath   string          `json:"transcript_path"`
	Description      string          `json:"description"`
}

// Env is what a hook reads from the process: the environment and the clock.
type Env struct {
	Getenv func(string) string
	Now    func() time.Time
	// wait is how long the hook waits for the vault lock; Run sets it from Deadlines.
	wait time.Duration
}

func (e Env) now() time.Time {
	if e.Now == nil {
		return time.Now()
	}
	return e.Now()
}

func (e Env) getenv(k string) string {
	if e.Getenv == nil {
		return os.Getenv(k)
	}
	return e.Getenv(k)
}

func readInput(r io.Reader) Input {
	var in Input
	json.NewDecoder(r).Decode(&in)
	return in
}

// event is the session a hook runs for.
func (in Input) event() sessions.Event {
	harness := "claude"
	if in.TurnID != "" || in.ToolName == "apply_patch" {
		harness = "codex"
	}
	sessionID := in.SessionID
	if sessionID == "" {
		sessionID = in.ParentSessionID
	}
	if sessionID == "" {
		sessionID = in.ParentSession2
	}
	return sessions.Event{Harness: harness, SessionID: sessionID, AgentID: in.AgentID, AgentType: in.AgentType, Cwd: in.Cwd, Source: in.Source, Detail: in.Description, Transcript: in.TranscriptPath}
}

// findVault resolves the vault of a hook's session, or nil.
func findVault(in Input, env Env) *vault.Vault {
	dir := in.Cwd
	if dir == "" {
		dir, _ = os.Getwd()
	}
	if p := env.getenv("ATLAS_VAULT"); p != "" {
		if v, err := vault.Open(vault.Expand(p)); err == nil {
			return v
		}
	}
	v, err := vault.Find(dir, vault.HomeFrom(env.getenv))
	if err != nil {
		return nil
	}
	return v
}

// atlasTool is the atlas tool a tool name calls, or "".
func atlasTool(name string) string {
	if !strings.HasPrefix(name, "mcp__") {
		return ""
	}
	i := strings.LastIndex(name, "__")
	server, tool := name[len("mcp__"):i], name[i+2:]
	if server == "atlas" || strings.HasSuffix(server, PluginName+"_atlas") {
		return tool
	}
	return ""
}

// toolInput decodes the fields of a tool call that the hooks read.
type toolInput struct {
	FilePath     string `json:"file_path"`
	NotebookPath string `json:"notebook_path"`
	Command      string `json:"command"`
	OldString    string `json:"old_string"`
	NewString    string `json:"new_string"`
	Edits        []struct {
		OldString string `json:"old_string"`
		NewString string `json:"new_string"`
	} `json:"edits"`
	Action string `json:"action"`
	ID     string `json:"id"`
	Thread string `json:"thread"`
	Take   bool   `json:"take"`
}

func (in Input) tool() toolInput {
	var t toolInput
	json.Unmarshal(in.ToolInput, &t)
	return t
}

// editTools are the tools that write a file.
var editTools = map[string]bool{"Write": true, "Edit": true, "MultiEdit": true, "NotebookEdit": true, "apply_patch": true}

// patchFile is one file of a Codex patch.
type patchFile struct {
	Path    string
	Op      string // add, update, delete, move
	Removed []string
	Added   []string
	Hunks   []hunk
}

// hunk is one @@ block of a patch: its header's text and its lines, each with its mark
// (' ' context, '-' removed, '+' added).
type hunk struct {
	Header string
	Lines  []string
}

// touches reports whether the hunk changes content[start:end]. It is placed where its
// context and removed lines stand in the file; a removed line inside the range touches
// it, and so does an added line that lands after the range's first byte and up to its
// end. A hunk that cannot be placed is judged by its removed lines alone.
func (h hunk) touches(content string, start, end int) bool {
	var old []string
	for _, l := range h.Lines {
		if l[0] != '+' {
			old = append(old, l[1:])
		}
	}
	text := strings.Join(old, "\n")
	from := 0
	if header := strings.TrimSpace(h.Header); header != "" {
		if i := strings.Index(content, header); i >= 0 {
			from = i
		}
	}
	placed := false
	for at := from; text != "" && at <= len(content); {
		i := strings.Index(content[at:], text)
		if i < 0 {
			break
		}
		p := at + i
		at = p + 1
		if p > 0 && content[p-1] != '\n' {
			continue
		}
		placed = true
		offset := p
		for _, l := range h.Lines {
			switch l[0] {
			case '+':
				if offset > start && offset <= end {
					return true
				}
			case '-':
				if offset < end && offset+len(l)-1 > start {
					return true
				}
				offset += len(l)
			default:
				offset += len(l)
			}
		}
	}
	if placed {
		return false
	}
	region := content[start:end]
	for _, l := range h.Lines {
		if l[0] == '-' && strings.TrimSpace(l[1:]) != "" && strings.Contains(region, l[1:]) {
			return true
		}
	}
	return false
}

// paths are the files a write tool touches: the file of an edit, or every file of a
// patch with both sides of a move. Relative paths are taken from the working directory.
func (in Input) paths() []patchFile {
	t := in.tool()
	var out []patchFile
	if in.ToolName == "apply_patch" {
		var cur *patchFile
		for _, line := range strings.Split(t.Command, "\n") {
			for prefix, op := range map[string]string{"*** Add File: ": "add", "*** Update File: ": "update", "*** Delete File: ": "delete", "*** Move to: ": "move"} {
				if rest, ok := strings.CutPrefix(line, prefix); ok {
					out = append(out, patchFile{Path: strings.TrimSpace(rest), Op: op})
					cur = &out[len(out)-1]
				}
			}
			if cur == nil || strings.HasPrefix(line, "*** ") {
				continue
			}
			if header, ok := strings.CutPrefix(line, "@@"); ok {
				cur.Hunks = append(cur.Hunks, hunk{Header: header})
				continue
			}
			if line == "" {
				line = " "
			}
			switch line[0] {
			case '-':
				cur.Removed = append(cur.Removed, line[1:])
			case '+':
				cur.Added = append(cur.Added, line[1:])
			case ' ':
			default:
				continue
			}
			if len(cur.Hunks) == 0 {
				cur.Hunks = append(cur.Hunks, hunk{})
			}
			h := &cur.Hunks[len(cur.Hunks)-1]
			h.Lines = append(h.Lines, line)
		}
	} else {
		for _, p := range []string{t.FilePath, t.NotebookPath} {
			if p != "" {
				op := "update"
				if in.ToolName == "Write" {
					op = "add"
				}
				out = append(out, patchFile{Path: p, Op: op})
			}
		}
	}
	for i := range out {
		p := out[i].Path
		if !filepath.IsAbs(p) && in.Cwd != "" {
			p = filepath.Join(in.Cwd, p)
		}
		out[i].Path = filepath.Clean(p)
	}
	return out
}

// oldStrings are the texts an Edit or a MultiEdit replaces.
func (in Input) oldStrings() []string {
	t := in.tool()
	out := []string{}
	if t.OldString != "" {
		out = append(out, t.OldString)
	}
	for _, e := range t.Edits {
		if e.OldString != "" {
			out = append(out, e.OldString)
		}
	}
	return out
}

// added are the lines an edit puts into a file, and removed the lines it takes out.
func (in Input) added(f patchFile) (added, removed []string) {
	if in.ToolName == "apply_patch" {
		return f.Added, f.Removed
	}
	t := in.tool()
	pairs := [][2]string{{t.OldString, t.NewString}}
	for _, e := range t.Edits {
		pairs = append(pairs, [2]string{e.OldString, e.NewString})
	}
	for _, p := range pairs {
		if p[0] != "" {
			removed = append(removed, strings.Split(p[0], "\n")...)
		}
		if p[1] != "" {
			added = append(added, strings.Split(p[1], "\n")...)
		}
	}
	return added, removed
}
