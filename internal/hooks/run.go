package hooks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

// EnvLog names a file that receives every hook event, one JSON line each, for checking
// what a host sends.
const EnvLog = "ALMAGEST_HOOK_LOG"

// Deadlines are how long each hook waits for the vault lock: below its timeout in
// hooks.json, so a hook that cannot get the lock gives up with an error before the host
// kills it. The guard takes no lock.
var Deadlines = map[string]time.Duration{
	"session-start":  8 * time.Second,
	"prompt":         4 * time.Second,
	"touched":        6 * time.Second,
	"notify":         4 * time.Second,
	"stop":           4 * time.Second,
	"subagent-start": 4 * time.Second,
	"subagent-stop":  4 * time.Second,
	"session-end":    time.Second,
}

// Run runs one hook command.
func Run(command string, r io.Reader, w io.Writer, env Env) error {
	env.wait = Deadlines[command]
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
