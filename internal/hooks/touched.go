package hooks

import (
	"encoding/json"
	"io"
	"regexp"
	"strings"

	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/sessions"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

// Touched links the session to what a call touched: the repository an edit landed in,
// and the change a proposal wrote. It copies the session's own description into its
// frontmatter.
func Touched(r io.Reader, env Env) error {
	in := readInput(r)
	now := env.now()
	e := in.event()
	if e.AgentID != "" && sessions.Worker(e.AgentType) {
		return nil
	}
	return locked(in, env, func(v *vault.Vault) error {
		var edits []func(string) string
		link := func(field, title string) {
			edits = append(edits, func(c string) string { return sessions.AddLink(c, field, title) })
		}
		if in.ToolName == "Bash" {
			if repo := shellWrite(v, in); repo != "" {
				link("repositories", repo)
			}
		}
		switch {
		case editTools[in.ToolName]:
			for _, f := range in.paths() {
				rel := v.Rel(f.Path)
				switch {
				case rel == "":
					var best *vault.Repo
					for _, repo := range v.Repositories() {
						if repo.Path != "" && vault.Within(f.Path, repo.Path) && (best == nil || len(repo.Path) > len(best.Path)) {
							rr := repo
							best = &rr
						}
					}
					if best != nil {
						link("repositories", best.Title)
					}
				case strings.HasPrefix(rel, vault.Sessions+"/"):
					if d := sessions.Find(v, e.Key()); d != nil && d.Path == rel {
						edits = append(edits, func(c string) string {
							body := doc.Parse("", []byte(c)).Body
							return doc.SetField(c, "description", sessions.DescriptionLine(body))
						})
					}
				}
			}
		case atlasTool(in.ToolName) == "change" && in.tool().Action == "propose" && !failed(decodeAll(in.ToolResponse)):
			ref := findObject(decodeAll(in.ToolResponse), "ref")
			title, _ := ref["title"].(string)
			p, _ := ref["path"].(string)
			if title != "" && p != "" {
				link("changes", title)
				if s := sessions.Find(v, e.Key()); s != nil {
					if data, err := v.Read(p); err == nil {
						v.WriteIfChanged(p, []byte(doc.SetField(string(data), "session", doc.Link(s.Title()))))
					}
				}
			}
		}
		_, err := sessions.Touch(v, e, now, func(c string) string {
			c = doc.SetField(c, "status", sessions.Running)
			for _, edit := range edits {
				c = edit(c)
			}
			return c
		})
		return err
	})
}

// shellWrites are the marks of a shell command that writes a file.
var shellWrites = regexp.MustCompile(`(^|[^<>&0-9])>>?[^&>]|\btee\b|\bsed\s+-i|\bperl\s+(-\w*\s+)*-\w*i|\bgit\s+(-C\s+\S+\s+)?(commit|apply|am|merge|rebase|reset|checkout|restore|mv|rm)\b|\b(mv|cp|rm|touch|mkdir)\s`)

// shellWrite is the title of the linked repository a Bash command wrote in, or "": the
// command runs in the repository or names its path, and it holds a mark of a write. It
// feeds the session's record only; the guard does not parse shell.
func shellWrite(v *vault.Vault, in Input) string {
	cmd := in.tool().Command
	if !shellWrites.MatchString(cmd) {
		return ""
	}
	best := ""
	bestLen := 0
	for _, r := range v.Repositories() {
		if r.Path == "" {
			continue
		}
		if (in.Cwd != "" && vault.Within(in.Cwd, r.Path)) || strings.Contains(cmd, r.Path) || strings.Contains(cmd, vault.Shorten(r.Path)) {
			if len(r.Path) > bestLen {
				best, bestLen = r.Title, len(r.Path)
			}
		}
	}
	return best
}

// decodeAll decodes a tool response, and any string inside it that holds JSON, since a
// host may pass an MCP result as its content blocks.
func decodeAll(raw json.RawMessage) []any {
	var root any
	if len(raw) == 0 || json.Unmarshal(raw, &root) != nil {
		return nil
	}
	var out []any
	var walk func(any)
	walk = func(x any) {
		out = append(out, x)
		switch v := x.(type) {
		case map[string]any:
			for _, c := range v {
				walk(c)
			}
		case []any:
			for _, c := range v {
				walk(c)
			}
		case string:
			s := strings.TrimSpace(v)
			if strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[") {
				var inner any
				if json.Unmarshal([]byte(s), &inner) == nil {
					walk(inner)
				}
			}
		}
	}
	walk(root)
	return out
}

// failed reports whether a tool response is an error.
func failed(values []any) bool {
	for _, x := range values {
		if m, ok := x.(map[string]any); ok {
			if b, _ := m["isError"].(bool); b {
				return true
			}
			if b, _ := m["is_error"].(bool); b {
				return true
			}
		}
	}
	return false
}

// findObject is the first object held under key that names a document, or nil.
func findObject(values []any, key string) map[string]any {
	for _, x := range values {
		m, ok := x.(map[string]any)
		if !ok {
			continue
		}
		if inner, ok := m[key].(map[string]any); ok {
			if _, ok := inner["id"]; ok {
				return inner
			}
		}
	}
	return nil
}
