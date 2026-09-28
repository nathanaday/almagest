package hooks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// EnvLog names a file that receives every hook event, one JSON line each, for checking
// what a host sends.
const EnvLog = "ATLAS_HOOK_LOG"

// Run runs one hook command.
func Run(command string, r io.Reader, w io.Writer, env Env) error {
	if file := env.getenv(EnvLog); file != "" {
		data, _ := io.ReadAll(r)
		logEvent(file, command, data)
		r = bytes.NewReader(data)
	}
	switch command {
	case "session-start":
		return SessionStart(r, w, env)
	case "prompt":
		return Prompt(r, w, env)
	case "guard":
		return Guard(r, w, env)
	case "touched":
		return Touched(r, env)
	case "notify":
		return Notify(r, env)
	case "stop":
		return Stop(r, w, env)
	case "subagent-start":
		return SubagentStart(r, env)
	case "subagent-stop":
		return SubagentStop(r, env)
	case "session-end":
		return SessionEnd(r, env)
	}
	return fmt.Errorf("no hook %q; the hooks are session-start, prompt, guard, touched, notify, stop, subagent-start, subagent-stop, session-end", command)
}

func logEvent(file, command string, data []byte) {
	f, err := os.OpenFile(file, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	line, _ := json.Marshal(map[string]any{"command": command, "input": json.RawMessage(bytes.TrimSpace(data))})
	if !json.Valid(line) {
		line, _ = json.Marshal(map[string]any{"command": command, "raw": string(data)})
	}
	f.Write(append(line, '\n'))
}
