package plugin

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/nathanaday/almagest/internal/release"
)

// codexServer is the almagest entry of the Codex manifest.
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
	s, ok := m.MCPServers["almagest"]
	if !ok {
		t.Fatal(".codex-plugin/plugin.json has no almagest server")
	}
	return s
}

// Codex expands no placeholder and would start the server in the plugin's folder with a
// cwd, so its entry carries the wrapper's lookup itself.
func TestTheCodexServerEntryNeedsNoPlaceholder(t *testing.T) {
	s := readCodexServer(t)
	if strings.Contains(s.Command+strings.Join(s.Args, " "), "${") {
		t.Errorf("the Codex entry holds ${, which reads as a placeholder Codex leaves as text; test variables with [ -n ] instead: %s %v", s.Command, s.Args)
	}
	if s.Cwd != "" {
		t.Errorf("the Codex entry sets cwd %q; the server must start in the session's folder to find its vault", s.Cwd)
	}
	if !filepath.IsAbs(s.Command) {
		t.Errorf("the Codex entry's command %q is not absolute, so it depends on PATH", s.Command)
	}
	for _, v := range []string{"ALMAGEST_BIN", "ALMAGEST_HOME", "ALMAGEST_VAULT", "ALMAGEST_NO_DOWNLOAD"} {
		found := false
		for _, x := range s.EnvVars {
			found = found || x == v
		}
		if !found {
			t.Errorf("the Codex entry does not pass %s through env_vars; Codex drops it otherwise", v)
		}
	}
}

// The Codex entry runs the launcher of bin/almagest, as it is, for the plugin's version
// and the checksums it pins.
func TestTheCodexEntryIsTheLauncher(t *testing.T) {
	s := readCodexServer(t)
	launcher := read(t, "bin/almagest")
	if !slices.Equal(s.Args, release.CodexArgs(launcher)) {
		t.Fatal("the Codex entry is not the launcher of bin/almagest; run make pin")
	}
	if s.Command != "/bin/sh" {
		t.Errorf("the Codex entry runs %s, not /bin/sh", s.Command)
	}
	st, err := os.Stat(filepath.Join(root, "bin/almagest"))
	if err != nil || st.Mode()&0o111 == 0 {
		t.Fatalf("bin/almagest is not executable: %v", err)
	}
}

// The launcher runs the binary of its version: ALMAGEST_BIN when set, else the one
// installed, else a download that matches the pinned sha256. It never looks on PATH.
func TestTheLauncher(t *testing.T) {
	const version = "9.9.9"
	platform := runtime.GOOS + "/" + runtime.GOARCH
	asset := release.Asset(version, platform)
	fake := func(t *testing.T, path, name string, exe bool) []byte {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		mode := os.FileMode(0o644)
		if exe {
			mode = 0o755
		}
		body := []byte("#!/bin/sh\necho " + name + " \"$@\"\n")
		if err := os.WriteFile(path, body, mode); err != nil {
			t.Fatal(err)
		}
		return body
	}
	sum := func(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
	cases := []struct {
		name  string
		args  []string
		setup func(t *testing.T, home, other string) (env []string, checksums string)
		check func(t *testing.T, r launch, home, other string)
	}{
		{"ALMAGEST_BIN first", []string{"mcp"}, func(t *testing.T, home, other string) ([]string, string) {
			fake(t, filepath.Join(other, "bin"), "almagest-bin", true)
			fake(t, filepath.Join(home, ".almagest/bin", version, "almagest"), "installed", true)
			return []string{"ALMAGEST_BIN=" + filepath.Join(other, "bin")}, ""
		}, want("almagest-bin mcp")},
		{"ALMAGEST_BIN not executable", []string{"mcp"}, func(t *testing.T, home, other string) ([]string, string) {
			fake(t, filepath.Join(other, "bin"), "almagest-bin", false)
			fake(t, filepath.Join(home, ".almagest/bin", version, "almagest"), "installed", true)
			return []string{"ALMAGEST_BIN=" + filepath.Join(other, "bin")}, ""
		}, fails("which is no executable file")},
		{"the installed version", []string{"mcp"}, func(t *testing.T, home, other string) ([]string, string) {
			fake(t, filepath.Join(home, ".almagest/bin", version, "almagest"), "installed", true)
			fake(t, filepath.Join(home, ".almagest/bin/8.0.0/almagest"), "older", true)
			return nil, ""
		}, want("installed mcp")},
		{"ALMAGEST_HOME", []string{"mcp"}, func(t *testing.T, home, other string) ([]string, string) {
			fake(t, filepath.Join(other, "bin", version, "almagest"), "home-env", true)
			return []string{"ALMAGEST_HOME=" + other}, ""
		}, want("home-env mcp")},
		{"no download", []string{"mcp"}, func(t *testing.T, home, other string) ([]string, string) {
			return []string{"ALMAGEST_NO_DOWNLOAD=1"}, ""
		}, fails("ALMAGEST_NO_DOWNLOAD is 1")},
		{"a platform the plugin does not pin", []string{"mcp"}, func(t *testing.T, home, other string) ([]string, string) {
			return nil, strings.Repeat("a", 64) + "  almagest-9.9.9-plan9-mips\n"
		}, fails("pins no almagest 9.9.9 for")},
		{"a download that matches", []string{"mcp"}, func(t *testing.T, home, other string) ([]string, string) {
			body := fake(t, filepath.Join(other, version, asset), "downloaded", true)
			return []string{"ALMAGEST_RELEASE_BASE=file://" + other}, sum(body) + "  " + asset + "\n"
		}, func(t *testing.T, r launch, home, other string) {
			want("downloaded mcp")(t, r, home, other)
			bin := filepath.Join(home, ".almagest/bin", version, "almagest")
			if st, err := os.Stat(bin); err != nil || st.Mode()&0o111 == 0 {
				t.Fatalf("the binary is not installed: %v", err)
			}
			if target, err := os.Readlink(filepath.Join(home, ".almagest/bin/almagest")); err != nil || target != version+"/almagest" {
				t.Fatalf("the link: %q %v", target, err)
			}
			log, _ := os.ReadFile(filepath.Join(home, ".almagest/install.log"))
			if !strings.Contains(string(log), "installed almagest "+version) || !strings.Contains(r.stderr, "sha256 ") {
				t.Fatalf("the install is not recorded: %s / %s", log, r.stderr)
			}
			if entries, _ := os.ReadDir(filepath.Dir(bin)); len(entries) != 1 {
				t.Fatalf("the download left files: %v", entries)
			}
		}},
		{"a download that does not match", []string{"mcp"}, func(t *testing.T, home, other string) ([]string, string) {
			fake(t, filepath.Join(other, version, asset), "tampered", true)
			return []string{"ALMAGEST_RELEASE_BASE=file://" + other}, strings.Repeat("0", 64) + "  " + asset + "\n"
		}, func(t *testing.T, r launch, home, other string) {
			fails("not the "+strings.Repeat("0", 64))(t, r, home, other)
			if entries, _ := os.ReadDir(filepath.Join(home, ".almagest/bin", version)); len(entries) != 0 {
				t.Fatalf("a failed download left files: %v", entries)
			}
			if _, err := os.Lstat(filepath.Join(home, ".almagest/bin/almagest")); err == nil {
				t.Fatal("a failed download made the link")
			}
		}},
		{"a hook with no binary", []string{"hook", "session-start"}, func(t *testing.T, home, other string) ([]string, string) {
			return []string{"ALMAGEST_NO_DOWNLOAD=1"}, ""
		}, func(t *testing.T, r launch, home, other string) {
			if r.code != 0 || !strings.Contains(r.stdout, "the almagest tools and hooks are off") {
				t.Fatalf("a session start with no binary must pass and say why: %+v", r)
			}
		}},
		{"another hook with no binary", []string{"hook", "guard"}, func(t *testing.T, home, other string) ([]string, string) {
			return []string{"ALMAGEST_NO_DOWNLOAD=1"}, ""
		}, func(t *testing.T, r launch, home, other string) {
			if r.code != 0 || r.stdout != "" {
				t.Fatalf("a hook with no binary must pass quietly: %+v", r)
			}
		}},
	}
	for _, c := range cases {
		for _, how := range []string{"bin/almagest", "the Codex entry"} {
			t.Run(c.name+", "+how, func(t *testing.T) {
				home, other, decoy := t.TempDir(), t.TempDir(), t.TempDir()
				fake(t, filepath.Join(decoy, "almagest"), "decoy", true)
				env, checksums := c.setup(t, home, other)
				launcher := release.Launcher(version, checksums)
				argv := append([]string{"/bin/sh"}, append(release.CodexArgs(launcher)[:3], c.args...)...)
				if how == "bin/almagest" {
					script := filepath.Join(t.TempDir(), "almagest")
					if err := os.WriteFile(script, []byte(launcher), 0o755); err != nil {
						t.Fatal(err)
					}
					argv = append([]string{script}, c.args...)
				}
				cmd := exec.Command(argv[0], argv[1:]...)
				cmd.Env = append([]string{"HOME=" + home, "PATH=" + decoy + ":/usr/bin:/bin"}, env...)
				cmd.Dir = t.TempDir()
				cmd.Stdin = strings.NewReader("{}")
				var out, errOut bytes.Buffer
				cmd.Stdout, cmd.Stderr = &out, &errOut
				err := cmd.Run()
				r := launch{out.String(), errOut.String(), 0}
				var exit *exec.ExitError
				if errors.As(err, &exit) {
					r.code = exit.ExitCode()
				} else if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(r.stdout, "decoy") {
					t.Fatalf("the launcher ran a binary from PATH: %+v", r)
				}
				c.check(t, r, home, other)
			})
		}
	}
}

// launch is what one run of the launcher printed, and its exit code.
type launch struct {
	stdout, stderr string
	code           int
}

// want checks that the launcher ran a fake binary, which printed line.
func want(line string) func(t *testing.T, r launch, home, other string) {
	return func(t *testing.T, r launch, home, other string) {
		t.Helper()
		if got := strings.TrimSpace(r.stdout); got != line || r.code != 0 {
			t.Fatalf("ran %q (exit %d, %s), want %q", got, r.code, r.stderr, line)
		}
	}
}

// fails checks that the launcher ran nothing, and said why.
func fails(why string) func(t *testing.T, r launch, home, other string) {
	return func(t *testing.T, r launch, home, other string) {
		t.Helper()
		if r.code == 0 || r.stdout != "" || !strings.Contains(r.stderr, why) {
			t.Fatalf("want a failure that says %q: %+v", why, r)
		}
	}
}
