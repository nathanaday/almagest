package vault

import (
	"path"
	"reflect"
	"slices"

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

// OldTemplate is a file an earlier release shipped, from template/old: the 6.5 Bases.
func OldTemplate(name string) (string, error) {
	data, err := templates.ReadFile("template/old/" + name)
	return string(data), err
}

// shippedBases are the copies of a Base that earlier releases shipped: 6.5's, and the one
// 7.0 to 8.1 shipped.
func shippedBases(name string) []string {
	var out []string
	for _, p := range []string{"template/old/" + name, "template/old/7.0/" + name} {
		if data, err := templates.ReadFile(p); err == nil {
			out = append(out, string(data))
		}
	}
	return out
}

// upgradeBases replaces each Base that equals a copy an earlier release shipped, which
// nobody edited, with the one this binary ships.
func upgradeBases(v *Vault) error {
	for name, rel := range Bases {
		data, err := v.Read(rel)
		if err != nil {
			continue
		}
		if !slices.ContainsFunc(shippedBases(path.Base(rel)), func(old string) bool { return SameYAML(string(data), old) }) {
			continue
		}
		current, err := templates.ReadFile("template/" + name)
		if err != nil {
			return err
		}
		if _, err := v.WriteIfChanged(rel, current); err != nil {
			return err
		}
	}
	return nil
}
