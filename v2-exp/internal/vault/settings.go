package vault

import (
	"encoding/json"
	"errors"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

// SyncSettings keeps one list in .claude/settings.local.json, the harness's local
// settings: permissions.additionalDirectories holds the path of every repository page,
// so an agent that starts in the vault may edit a linked repository. It adds the paths
// the pages name, removes the paths in drop that no page names any more, and keeps every
// other key and entry. It reports whether it wrote.
func (v *Vault) SyncSettings(drop []string) (bool, error) {
	var want []string
	for _, r := range v.Repositories() {
		if r.Path != "" {
			want = append(want, r.Path)
		}
	}
	file := v.Abs(Settings)
	settings := map[string]any{}
	data, err := os.ReadFile(file)
	switch {
	case err == nil:
		if err := json.Unmarshal(data, &settings); err != nil {
			return false, errors.New(Settings + " is not valid JSON; fix it and sync again")
		}
	case errors.Is(err, os.ErrNotExist):
		if len(want) == 0 {
			return false, nil
		}
	default:
		return false, err
	}
	perms, _ := settings["permissions"].(map[string]any)
	if perms == nil {
		perms = map[string]any{}
	}
	var dirs []string
	if list, ok := perms["additionalDirectories"].([]any); ok {
		for _, item := range list {
			if s, ok := item.(string); ok {
				dirs = append(dirs, s)
			}
		}
	}
	before := slices.Clone(dirs)
	for _, d := range drop {
		d = Expand(d)
		if slices.Contains(want, d) {
			continue
		}
		dirs = slices.DeleteFunc(dirs, func(s string) bool { return Expand(s) == d })
	}
	for _, w := range want {
		if !slices.ContainsFunc(dirs, func(s string) bool { return Expand(s) == w }) {
			dirs = append(dirs, w)
		}
	}
	if slices.Equal(before, dirs) {
		return false, nil
	}
	list := make([]any, len(dirs))
	for i, d := range dirs {
		list[i] = d
	}
	perms["additionalDirectories"] = list
	settings["permissions"] = perms
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return false, err
	}
	return true, writeAtomic(file, append(out, '\n'))
}

// OpenNote is the vault-relative path of the note Obsidian shows in its active pane, read
// from .obsidian/workspace.json, which Obsidian keeps current; "" when there is none.
func (v *Vault) OpenNote() string {
	data, err := os.ReadFile(v.Abs(".obsidian/workspace.json"))
	if err != nil {
		return ""
	}
	var ws struct {
		Main          json.RawMessage `json:"main"`
		Left          json.RawMessage `json:"left"`
		Right         json.RawMessage `json:"right"`
		Active        string          `json:"active"`
		LastOpenFiles []string        `json:"lastOpenFiles"`
	}
	if err := json.Unmarshal(data, &ws); err != nil {
		return ""
	}
	if ws.Active != "" {
		for _, tree := range []json.RawMessage{ws.Main, ws.Left, ws.Right} {
			if f := findLeafFile(tree, ws.Active); f != "" {
				return f
			}
		}
	}
	for _, f := range ws.LastOpenFiles {
		if strings.HasSuffix(strings.ToLower(f), ".md") {
			return f
		}
	}
	return ""
}

// findLeafFile walks a workspace tree for the leaf with id and returns its file.
func findLeafFile(raw json.RawMessage, id string) string {
	if len(raw) == 0 {
		return ""
	}
	var node struct {
		ID       string            `json:"id"`
		Type     string            `json:"type"`
		Children []json.RawMessage `json:"children"`
		State    struct {
			State struct {
				File string `json:"file"`
			} `json:"state"`
		} `json:"state"`
	}
	if err := json.Unmarshal(raw, &node); err != nil {
		return ""
	}
	if node.ID == id && node.Type == "leaf" {
		return node.State.State.File
	}
	for _, c := range node.Children {
		if f := findLeafFile(c, id); f != "" {
			return f
		}
	}
	return ""
}

// NoteTitle is the title a vault-relative note path shows.
func NoteTitle(rel string) string {
	base := path.Base(filepath.ToSlash(rel))
	return strings.TrimSuffix(base, path.Ext(base))
}
