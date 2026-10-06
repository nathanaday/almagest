package vault

import (
	"path"
	"reflect"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// SameYAML reports whether two YAML texts hold the same data. Obsidian rewrites a Base
// it opens, quoting and spacing it its own way, and that is no edit.
func SameYAML(a, b string) bool {
	var x, y any
	if yaml.Unmarshal([]byte(a), &x) != nil || yaml.Unmarshal([]byte(b), &y) != nil {
		return false
	}
	return reflect.DeepEqual(x, y)
}

// Bases maps each Base init ships to its path in the vault.
var Bases = map[string]string{
	"Sessions.base": "sessions/Sessions.base",
	"Changes.base":  "changes/Changes.base",
}

// OldTemplate is a file an earlier release shipped, by its path under template/old, such
// as 8.1/Sessions.base.
func OldTemplate(name string) (string, error) {
	data, err := templates.ReadFile("template/old/" + name)
	return string(data), err
}

// shippedBases are the copies of a Base that earlier releases shipped: 6.5's, the one 7.0
// shipped, and the one 8.x shipped.
func shippedBases(name string) []string {
	var out []string
	for _, p := range []string{"template/old/" + name, "template/old/7.0/" + name, "template/old/8.1/" + name} {
		if data, err := templates.ReadFile(p); err == nil {
			out = append(out, string(data))
		}
	}
	return out
}

// staleBases are the Bases that equal a copy an earlier release shipped, which nobody
// edited, and differ from this release's copy, with this release's copy and the old one.
func staleBases(v *Vault) map[string][2][]byte {
	out := map[string][2][]byte{}
	for name, rel := range Bases {
		data, err := v.Read(rel)
		if err != nil || !slices.ContainsFunc(shippedBases(path.Base(rel)), func(c string) bool { return SameYAML(string(data), c) }) {
			continue
		}
		current, err := templates.ReadFile("template/" + name)
		if err != nil || string(current) == string(data) {
			continue
		}
		out[rel] = [2][]byte{current, data}
	}
	return out
}

// UpgradeBases writes this release's copy of each stale Base through write, for a caller
// whose own commit holds them.
func UpgradeBases(v *Vault, write func(rel string, content []byte) error) error {
	for rel, b := range staleBases(v) {
		if err := write(rel, b[0]); err != nil {
			return err
		}
	}
	return nil
}

// upgradeBases replaces each Base that equals a copy an earlier release shipped, which
// nobody edited, with the one this binary ships, and commits the new copies alone: no
// snapshot calls them a hand edit, and no change's undo counts them. When that commit
// fails, the old copies go back, and the next write tries again.
func upgradeBases(v *Vault) {
	old := map[string][]byte{}
	var paths []string
	for rel, b := range staleBases(v) {
		if err := v.Write(rel, b[0]); err != nil {
			continue
		}
		old[rel] = b[1]
		paths = append(paths, rel)
	}
	if len(paths) == 0 {
		return
	}
	sort.Strings(paths)
	if _, err := v.Git().CommitOnly("layout: upgrade "+strings.Join(paths, ", "), paths...); err != nil {
		for rel, data := range old {
			v.Write(rel, data)
		}
	}
}
