package host

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// HookTrust counts the plugin's hooks in Codex by trust status: managed, trusted,
// untrusted, or modified. Codex runs only managed and trusted hooks.
type HookTrust map[string]int

// Total is the number of the plugin's hooks Codex lists.
func (h HookTrust) Total() int {
	n := 0
	for _, c := range h {
		n += c
	}
	return n
}

// Off is the number of the plugin's hooks Codex will not run.
func (h HookTrust) Off() int { return h.Total() - h["trusted"] - h["managed"] }

// CodexHookTrust asks Codex, through codex app-server's hooks/list for dir, the trust
// status of each of the plugin's hooks. Codex keeps the trusted hash; only it can say
// whether a hook matches it.
func CodexHookTrust(dir string) (HookTrust, error) {
	ctx, cancel := context.WithTimeout(context.Background(), ProbeTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "codex", "app-server")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("codex app-server: %w", err)
	}
	defer func() {
		stdin.Close()
		cancel()
		cmd.Wait()
	}()
	cwd, _ := json.Marshal(dir)
	requests := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"clientInfo":{"name":"atlas-obsidian","version":"1"}}}` + "\n" +
		`{"jsonrpc":"2.0","method":"initialized"}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"hooks/list","params":{"cwds":[` + string(cwd) + `]}}` + "\n"
	if _, err := io.WriteString(stdin, requests); err != nil {
		return nil, fmt.Errorf("codex app-server: %w", err)
	}
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		var msg struct {
			ID     json.RawMessage           `json:"id"`
			Error  *struct{ Message string } `json:"error"`
			Result struct {
				Data []struct {
					Hooks []struct {
						PluginID    string `json:"pluginId"`
						TrustStatus string `json:"trustStatus"`
					} `json:"hooks"`
				} `json:"data"`
			} `json:"result"`
		}
		if json.Unmarshal(scanner.Bytes(), &msg) != nil || strings.TrimSpace(string(msg.ID)) != "2" {
			continue
		}
		if msg.Error != nil {
			return nil, fmt.Errorf("codex app-server hooks/list: %s", msg.Error.Message)
		}
		trust := HookTrust{}
		for _, entry := range msg.Result.Data {
			for _, h := range entry.Hooks {
				if h.PluginID == PluginID {
					trust[h.TrustStatus]++
				}
			}
		}
		return trust, nil
	}
	if ctx.Err() != nil {
		return nil, fmt.Errorf("codex app-server did not answer hooks/list within %s", ProbeTimeout)
	}
	return nil, errors.New("codex app-server ended without answering hooks/list")
}
