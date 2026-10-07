// Package plugin holds tests only: the agent plugin's skills, agents, links, hooks, and
// versions against the code that serves them.
package plugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/almagest/internal/doc"
	"github.com/nathanaday/almagest/internal/hooks"
	"github.com/nathanaday/almagest/internal/mcpserver"
	"github.com/nathanaday/almagest/internal/release"
	"github.com/nathanaday/almagest/internal/sessions"
)

// root is the plugin's folder: the repository root.
const root = "../.."

// Skills are the skills of the design's map, by noun.
var Skills = map[string][]string{
	"almagest": {"almagest", "almagest-onboard"},
	"repo":     {"repo-link", "repo-unlink", "repo-ingest"},
	"wiki":     {"wiki-ingest", "wiki-sync", "wiki-save", "wiki-query", "wiki-edit", "wiki-map", "wiki-review", "wiki-checkout", "wiki-wikify"},
}

func allSkills() []string {
	var out []string
	for _, list := range Skills {
		out = append(out, list...)
	}
	sort.Strings(out)
	return out
}

func read(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestEverySkillHasItsForm(t *testing.T) {
	entries, err := os.ReadDir(filepath.Join(root, "skills"))
	if err != nil {
		t.Fatal(err)
	}
	var found []string
	for _, e := range entries {
		if e.IsDir() {
			found = append(found, e.Name())
		}
	}
	sort.Strings(found)
	if strings.Join(found, ",") != strings.Join(allSkills(), ",") {
		t.Fatalf("skills/ holds %v; the map has %v", found, allSkills())
	}
	for _, name := range found {
		text := read(t, "skills/"+name+"/SKILL.md")
		d := doc.Parse(name, []byte(text))
		if d.FrontErr != nil || d.Str("name") != name {
			t.Errorf("%s: frontmatter name is %q (%v)", name, d.Str("name"), d.FrontErr)
		}
		desc := d.Str("description")
		if !strings.Contains(desc, "Use for") || len(desc) > 1024 {
			t.Errorf("%s: the description says what it does, then Use for, within 1024 characters", name)
		}
		prefix, _, _ := strings.Cut(name, "-")
		if _, ok := Skills[prefix]; !ok {
			t.Errorf("%s: a skill is named <noun>-<verb> on the three nouns", name)
		}
		for _, want := range []string{"\n# " + name + "\n", "\nTools: ", "\n## Hand off\n"} {
			if !strings.Contains(text, want) {
				t.Errorf("%s: lacks %q", name, strings.TrimSpace(want))
			}
		}
		if !strings.Contains(text, "\n## Procedure\n") && name != "almagest" {
			t.Errorf("%s: lacks its Procedure", name)
		}
		if !strings.Contains(text, "\n## Gate\n") && !strings.Contains(text, "\n## Gates\n") {
			t.Errorf("%s: lacks its Gate", name)
		}
	}
}

var mdLink = regexp.MustCompile(`\]\(([^)#\s]+\.md)(#[^)]*)?\)`)

func TestEveryLinkResolves(t *testing.T) {
	for _, dir := range []string{"skills", "agents"} {
		filepath.WalkDir(filepath.Join(root, dir), func(p string, e os.DirEntry, err error) error {
			if err != nil || e.IsDir() || !strings.HasSuffix(p, ".md") {
				return nil
			}
			data, _ := os.ReadFile(p)
			for _, m := range mdLink.FindAllStringSubmatch(string(data), -1) {
				target := filepath.Join(filepath.Dir(p), m[1])
				if _, err := os.Stat(target); err != nil {
					t.Errorf("%s links %s, which is missing", p, m[1])
				}
			}
			return nil
		})
	}
}

func TestAgentsAreTheReadOnlyWorkers(t *testing.T) {
	tools := map[string]bool{}
	for _, n := range mcpserver.ToolNames() {
		tools["mcp__plugin_"+hooks.PluginName+"_almagest__"+n] = true
	}
	entries, _ := os.ReadDir(filepath.Join(root, "agents"))
	var names []string
	for _, e := range entries {
		name := strings.TrimSuffix(e.Name(), ".md")
		names = append(names, name)
		d := doc.Parse(e.Name(), []byte(read(t, "agents/"+e.Name())))
		if d.Str("name") != name {
			t.Errorf("%s: frontmatter name %q", e.Name(), d.Str("name"))
		}
		for _, tool := range strings.Split(d.Str("tools"), ",") {
			tool = strings.TrimSpace(tool)
			if strings.HasPrefix(tool, "mcp__") && !tools[tool] {
				t.Errorf("%s: no tool %s", name, tool)
			}
			if slices.Contains([]string{"Write", "Edit", "MultiEdit", "NotebookEdit"}, tool) {
				t.Errorf("%s: a read-only agent lists %s", name, tool)
			}
			if strings.HasSuffix(tool, "__change") {
				t.Errorf("%s: a read-only agent lists the change tool", name)
			}
		}
	}
	sort.Strings(names)
	want := slices.Clone(sessions.ReadOnlyAgents)
	sort.Strings(want)
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("agents/ holds %v; the guard knows %v", names, want)
	}
}

func TestSkillsNameOnlyWhatExists(t *testing.T) {
	known := map[string]bool{}
	for _, s := range allSkills() {
		known[s] = true
	}
	for _, a := range sessions.ReadOnlyAgents {
		known[a] = true
	}
	tools := map[string]bool{}
	for _, n := range mcpserver.ToolNames() {
		tools[n] = true
	}
	name := regexp.MustCompile(`\[((?:almagest|repo|wiki|thread|chord)-[a-z]+)\]\(`)
	toolsLine := regexp.MustCompile("(?m)^Tools: (.*)$")
	for _, s := range allSkills() {
		text := read(t, "skills/"+s+"/SKILL.md")
		for _, m := range name.FindAllStringSubmatch(text, -1) {
			if !known[m[1]] {
				t.Errorf("%s names %s, which does not exist", s, m[1])
			}
		}
		line := toolsLine.FindStringSubmatch(text)
		if line == nil {
			continue
		}
		for _, m := range regexp.MustCompile("`([a-z]+)`").FindAllStringSubmatch(line[1], -1) {
			if !tools[m[1]] {
				t.Errorf("%s: its Tools line names %s, which is no tool", s, m[1])
			}
		}
	}
}

func TestHooksFileMatchesTheCommands(t *testing.T) {
	var file struct {
		Hooks map[string][]struct {
			Matcher string `json:"matcher"`
			Hooks   []struct {
				Command string `json:"command"`
				Timeout int    `json:"timeout"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal([]byte(read(t, "hooks/hooks.json")), &file); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for event, groups := range file.Hooks {
		for _, g := range groups {
			for _, h := range g.Hooks {
				command := h.Command[strings.LastIndex(h.Command, " ")+1:]
				if hooks.Events[command] != event {
					t.Errorf("%s runs %s, which is the %s hook", event, command, hooks.Events[command])
				}
				seen[command] = true
				// A hook waits for the lock half its timeout at most, so the work after
				// the wait has time too.
				// The host gives SessionEnd little time when the user quits.
				if command == "session-end" && h.Timeout != 3 {
					t.Errorf("session-end times out at %d s, not 3", h.Timeout)
				}
				if wait := hooks.Deadlines[command]; h.Timeout == 0 || 2*wait > time.Duration(h.Timeout)*time.Second {
					t.Errorf("%s waits %s for the lock, more than half its timeout of %d s", command, wait, h.Timeout)
				}
			}
		}
	}
	for command := range hooks.Events {
		if !seen[command] {
			t.Errorf("hooks.json never runs %s", command)
		}
	}
	// The hosts test the matcher as written, unanchored, so it anchors itself.
	guard := regexp.MustCompile(file.Hooks["PreToolUse"][0].Matcher)
	for _, tool := range []string{"Write", "Edit", "MultiEdit", "NotebookEdit", "Bash", "apply_patch", "mcp__plugin_" + hooks.PluginName + "_almagest__change", "mcp__plugin_" + hooks.PluginName + "_almagest__source", "mcp__plugin_" + hooks.PluginName + "_almagest__vault", "mcp__almagest__change", "mcp__almagest__vault"} {
		if !guard.MatchString(tool) {
			t.Errorf("the guard does not see %s", tool)
		}
	}
	for _, tool := range []string{"WriteFile", "mcp__x_almagest__changelog", "mcp__plugin_other_almagest__change", "mcp__almagest__change_log", "mcp__almagest__search", "mcp__plugin_" + hooks.PluginName + "_almagest__thread", "mcp__almagest__chord", "Read"} {
		if guard.MatchString(tool) {
			t.Errorf("the guard sees %s", tool)
		}
	}
	// Every almagest tool that can write reaches the guard on both hosts; a new tool fails
	// here until the matcher names it, or this list says it only reads.
	readsOnly := map[string]bool{"search": true, "context": true, "match": true, "lint": true}
	for _, tool := range mcpserver.ToolNames() {
		for _, name := range []string{"mcp__plugin_" + hooks.PluginName + "_almagest__" + tool, "mcp__almagest__" + tool} {
			if !readsOnly[tool] && !guard.MatchString(name) {
				t.Errorf("the guard does not see %s, which can write", name)
			}
		}
	}
}

func TestOneVersion(t *testing.T) {
	version := func(rel string) string {
		var m struct {
			Version string `json:"version"`
		}
		if err := json.Unmarshal([]byte(read(t, rel)), &m); err != nil {
			t.Fatal(err)
		}
		return m.Version
	}
	v := version(".claude-plugin/plugin.json")
	// The launcher pins the binary of this version, for every platform a release builds.
	sums, err := release.ParseChecksums(read(t, release.ChecksumsFile))
	if err != nil {
		t.Fatal(err)
	}
	if missing := release.Missing(v, sums); len(missing) > 0 {
		t.Errorf("%s names no checksum for %v; run make pin", release.ChecksumsFile, missing)
	}
	if read(t, "bin/almagest") != release.Launcher(v, read(t, release.ChecksumsFile)) {
		t.Error("bin/almagest is not the launcher of this version and its checksums; run make pin")
	}
	if got := version(".codex-plugin/plugin.json"); got != v {
		t.Errorf("the Codex plugin is %s, the Claude plugin %s", got, v)
	}
	var market struct {
		Plugins []struct {
			Name    string `json:"name"`
			Version string `json:"version"`
			Source  struct {
				Source string `json:"source"`
				Repo   string `json:"repo"`
				Ref    string `json:"ref"`
			} `json:"source"`
		} `json:"plugins"`
	}
	json.Unmarshal([]byte(read(t, ".claude-plugin/marketplace.json")), &market)
	found := false
	for _, p := range market.Plugins {
		if p.Name == hooks.PluginName {
			found = true
			if p.Version != v {
				t.Errorf("the marketplace lists %s, the plugin is %s", p.Version, v)
			}
			// A user installs the release tag, whose GitHub release holds the pinned binaries,
			// never a commit between releases.
			if p.Source.Source != "github" || p.Source.Repo != "nathanaday/almagest" || p.Source.Ref != v {
				t.Errorf("the marketplace entry must pin github nathanaday/almagest at the tag %s: %+v", v, p.Source)
			}
		}
	}
	if !found {
		t.Error("the marketplace does not list the plugin")
	}
}
