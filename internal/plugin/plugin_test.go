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

	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/hooks"
	"github.com/nathanaday/atlas-obsidian/internal/mcpserver"
	"github.com/nathanaday/atlas-obsidian/internal/sessions"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

// root is the plugin's folder: the repository root.
const root = "../.."

// Skills are the skills of the design's map, by noun.
var Skills = map[string][]string{
	"atlas":  {"atlas", "atlas-onboard"},
	"repo":   {"repo-link", "repo-unlink", "repo-ingest"},
	"wiki":   {"wiki-ingest", "wiki-sync", "wiki-save", "wiki-query", "wiki-edit", "wiki-rollup", "wiki-review"},
	"thread": {"thread-work", "thread-stub", "thread-spec", "thread-plan", "thread-run", "thread-receipt"},
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
			t.Errorf("%s: a skill is named <noun>-<verb> on the four nouns", name)
		}
		for _, want := range []string{"\n# " + name + "\n", "\nTools: ", "\n## Hand off\n"} {
			if !strings.Contains(text, want) {
				t.Errorf("%s: lacks %q", name, strings.TrimSpace(want))
			}
		}
		if !strings.Contains(text, "\n## Procedure\n") && name != "atlas" {
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
		tools["mcp__plugin_"+hooks.PluginName+"_atlas__"+n] = true
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
	name := regexp.MustCompile(`\[((?:atlas|repo|wiki|thread)-[a-z]+)\]\(`)
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
			}
		}
	}
	for command := range hooks.Events {
		if !seen[command] {
			t.Errorf("hooks.json never runs %s", command)
		}
	}
	guard := file.Hooks["PreToolUse"][0].Matcher
	for _, tool := range []string{"Write", "Edit", "Bash", "apply_patch", "mcp__plugin_" + hooks.PluginName + "_atlas__change"} {
		if !regexp.MustCompile("^(" + guard + ")$").MatchString(tool) {
			t.Errorf("the guard does not see %s", tool)
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
	if got := version(".codex-plugin/plugin.json"); got != v {
		t.Errorf("the Codex plugin is %s, the Claude plugin %s", got, v)
	}
	if got := vault.PluginVersion(); got != v {
		t.Errorf("the Obsidian plugin the binary carries is %s, the agent plugin %s", got, v)
	}
	if got := version("obsidian/manifest.json"); got != v {
		t.Errorf("the Obsidian plugin's source is %s, the agent plugin %s", got, v)
	}
	if got := version("obsidian/package.json"); got != v {
		t.Errorf("the Obsidian plugin's package is %s, the agent plugin %s", got, v)
	}
	built, err := os.ReadFile(filepath.Join(root, "obsidian/dist/main.js"))
	if err == nil {
		carried, _ := vault.Template("obsidian/main.js")
		if string(built) != string(carried) {
			t.Error("the binary carries an older Obsidian plugin than obsidian/dist; run make obsidian")
		}
	}
	var market struct {
		Plugins []struct {
			Name    string `json:"name"`
			Version string `json:"version"`
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
		}
	}
	if !found {
		t.Error("the marketplace does not list the plugin")
	}
}
