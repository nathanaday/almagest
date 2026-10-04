package plugin

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// codexServer is the atlas entry of the Codex manifest.
type codexServer struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Cwd     string            `json:"cwd"`
	Env     map[string]string `json:"env"`
	EnvVars []string          `json:"env_vars"`
}

func readCodexServer(t *testing.T) codexServer {
	t.Helper()
	var m struct {
		MCPServers map[string]codexServer `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(read(t, ".codex-plugin/plugin.json")), &m); err != nil {
		t.Fatalf(".codex-plugin/plugin.json must hold its mcpServers inline, since Codex expands no placeholder in a file it points to: %v", err)
	}
	s, ok := m.MCPServers["atlas"]
	if !ok {
		t.Fatal(".codex-plugin/plugin.json has no atlas server")
	}
	return s
}

// Codex expands no placeholder and would start the server in the plugin's folder with a
// cwd, so its entry carries the wrapper's lookup itself.
func TestTheCodexServerEntryNeedsNoPlaceholder(t *testing.T) {
	s := readCodexServer(t)
	if strings.Contains(s.Command+strings.Join(s.Args, " "), "PLUGIN_ROOT") {
		t.Errorf("the Codex entry names a plugin-root placeholder, which Codex leaves as text: %s %v", s.Command, s.Args)
	}
	if s.Cwd != "" {
		t.Errorf("the Codex entry sets cwd %q; the server must start in the session's folder to find its vault", s.Cwd)
	}
	if !filepath.IsAbs(s.Command) {
		t.Errorf("the Codex entry's command %q is not absolute, so it depends on PATH", s.Command)
	}
	for _, v := range []string{"ATLAS_BIN", "ATLAS_HOME", "ATLAS_VAULT"} {
		found := false
		for _, x := range s.EnvVars {
			found = found || x == v
		}
		if !found {
			t.Errorf("the Codex entry does not pass %s through env_vars; Codex drops it otherwise", v)
		}
	}
}

// The Codex entry and scripts/atlas-obsidian find the same binary in every case, and
// neither looks on PATH.
func TestTheCodexEntryFindsTheBinaryAsTheWrapperDoes(t *testing.T) {
	s := readCodexServer(t)
	wrapper, err := filepath.Abs(filepath.Join(root, "scripts/atlas-obsidian"))
	if err != nil {
		t.Fatal(err)
	}
	fake := func(t *testing.T, path, name string, exe bool) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		mode := os.FileMode(0o644)
		if exe {
			mode = 0o755
		}
		if err := os.WriteFile(path, []byte("#!/bin/sh\necho "+name+" \"$@\"\n"), mode); err != nil {
			t.Fatal(err)
		}
	}
	cases := []struct {
		name  string
		setup func(t *testing.T, home, other string) []string
		want  string
	}{
		{"ATLAS_BIN first", func(t *testing.T, home, other string) []string {
			fake(t, filepath.Join(other, "bin"), "atlas-bin", true)
			fake(t, filepath.Join(home, ".atlas/bin/atlas-obsidian"), "atlas-home", true)
			return []string{"ATLAS_BIN=" + filepath.Join(other, "bin")}
		}, "atlas-bin mcp"},
		{"ATLAS_BIN not executable", func(t *testing.T, home, other string) []string {
			fake(t, filepath.Join(other, "bin"), "atlas-bin", false)
			fake(t, filepath.Join(home, ".atlas/bin/atlas-obsidian"), "atlas-home", true)
			return []string{"ATLAS_BIN=" + filepath.Join(other, "bin")}
		}, "atlas-home mcp"},
		{"ATLAS_HOME", func(t *testing.T, home, other string) []string {
			fake(t, filepath.Join(other, "bin/atlas-obsidian"), "atlas-home-env", true)
			fake(t, filepath.Join(home, ".atlas/bin/atlas-obsidian"), "atlas-home", true)
			return []string{"ATLAS_HOME=" + other}
		}, "atlas-home-env mcp"},
		{"~/.atlas before ~/go", func(t *testing.T, home, other string) []string {
			fake(t, filepath.Join(home, ".atlas/bin/atlas-obsidian"), "atlas-home", true)
			fake(t, filepath.Join(home, "go/bin/atlas-obsidian"), "go-bin", true)
			return nil
		}, "atlas-home mcp"},
		{"~/go alone", func(t *testing.T, home, other string) []string {
			fake(t, filepath.Join(home, "go/bin/atlas-obsidian"), "go-bin", true)
			return nil
		}, "go-bin mcp"},
		{"none", func(t *testing.T, home, other string) []string { return nil }, ""},
	}
	type result struct {
		stdout, stderr string
		code           int
	}
	runIt := func(t *testing.T, env []string, name string, args ...string) result {
		t.Helper()
		cmd := exec.Command(name, args...)
		cmd.Env = env
		cmd.Dir = t.TempDir()
		var out, errOut bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &errOut
		err := cmd.Run()
		code := 0
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			code = exit.ExitCode()
		} else if err != nil {
			t.Fatal(err)
		}
		return result{out.String(), errOut.String(), code}
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			home, other, decoy := t.TempDir(), t.TempDir(), t.TempDir()
			fake(t, filepath.Join(decoy, "atlas-obsidian"), "decoy", true)
			env := append([]string{"HOME=" + home, "PATH=" + decoy + ":/usr/bin:/bin"}, c.setup(t, home, other)...)
			w := runIt(t, env, wrapper, "mcp")
			e := runIt(t, env, s.Command, s.Args...)
			if e != w {
				t.Fatalf("the Codex entry and the wrapper differ:\nentry:   %+v\nwrapper: %+v", e, w)
			}
			if c.want != "" {
				if got := strings.TrimSpace(e.stdout); got != c.want || e.code != 0 {
					t.Fatalf("ran %q (exit %d), want %q", got, e.code, c.want)
				}
				return
			}
			if e.code == 0 || !strings.Contains(e.stderr, "make install") || strings.Contains(e.stdout, "decoy") {
				t.Fatalf("with no binary the entry must fail and say how to install, without PATH: %+v", e)
			}
		})
	}
}
