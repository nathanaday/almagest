// Command pin writes the launcher of the plugin's version, with the checksums of
// release/checksums.txt, to bin/almagest and into the Codex server entry. With -set
// VERSION, it first sets the version in both plugin manifests and the marketplace entry
// (its version and its ref). Run it from the repository root (make pin, make version).
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/nathanaday/almagest/internal/release"
)

func main() {
	if len(os.Args) == 3 && os.Args[1] == "-set" {
		if err := setVersion(os.Args[2]); err != nil {
			fmt.Fprintln(os.Stderr, "pin:", err)
			os.Exit(1)
		}
		return
	}
	if err := pin(); err != nil {
		fmt.Fprintln(os.Stderr, "pin:", err)
		os.Exit(1)
	}
}

func pin() error {
	version, err := pluginVersion()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(release.ChecksumsFile)
	if err != nil {
		return err
	}
	sums, err := release.ParseChecksums(string(data))
	if err != nil {
		return err
	}
	if missing := release.Missing(version, sums); len(missing) > 0 {
		return fmt.Errorf("%s names no checksum for %v; run make release first", release.ChecksumsFile, missing)
	}
	launcher := release.Launcher(version, string(data))
	if err := os.MkdirAll("bin", 0o755); err != nil {
		return err
	}
	if err := os.WriteFile("bin/almagest", []byte(launcher), 0o755); err != nil {
		return err
	}
	return writeCodexEntry(launcher)
}

// semver is a release version as its tag names it: no v in front.
var semver = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// setVersion replaces the plugin's version in every manifest that names it. The launcher
// and its checksums follow with make pin.
func setVersion(version string) error {
	if !semver.MatchString(version) {
		return fmt.Errorf("%q is not major.minor.patch, such as 11.0.1 (no v in front)", version)
	}
	old, err := pluginVersion()
	if err != nil {
		return err
	}
	edits := []struct {
		file  string
		from  string
		count int
	}{
		{".claude-plugin/plugin.json", `"version": "` + old + `"`, 1},
		// The Codex manifest's launcher names the version too; make pin writes it again.
		{".codex-plugin/plugin.json", `"version": "` + old + `"`, 1},
		{".claude-plugin/marketplace.json", `"version": "` + old + `"`, 2},
		{".claude-plugin/marketplace.json", `"ref": "` + old + `"`, 1},
	}
	for _, e := range edits {
		data, err := os.ReadFile(e.file)
		if err != nil {
			return err
		}
		text := string(data)
		if n := strings.Count(text, e.from); n != e.count {
			return fmt.Errorf("%s holds %s %d times, not %d; set the version by hand", e.file, e.from, n, e.count)
		}
		to := strings.Replace(e.from, old, version, 1)
		if err := os.WriteFile(e.file, []byte(strings.ReplaceAll(text, e.from, to)), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func pluginVersion() (string, error) {
	var m struct {
		Version string `json:"version"`
	}
	data, err := os.ReadFile(".claude-plugin/plugin.json")
	if err != nil {
		return "", err
	}
	if err := json.Unmarshal(data, &m); err != nil || m.Version == "" {
		return "", fmt.Errorf(".claude-plugin/plugin.json holds no version: %v", err)
	}
	return m.Version, nil
}

// writeCodexEntry puts the launcher into the almagest server of the Codex manifest, and
// keeps every other key in its order.
func writeCodexEntry(launcher string) error {
	const file = ".codex-plugin/plugin.json"
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	var m ordered
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	servers, err := m.object("mcpServers")
	if err != nil {
		return err
	}
	server, err := servers.object("almagest")
	if err != nil {
		return err
	}
	var args bytes.Buffer
	enc := json.NewEncoder(&args)
	// The launcher's > and & stay as they are, readable in the manifest.
	enc.SetEscapeHTML(false)
	if err := enc.Encode(release.CodexArgs(launcher)); err != nil {
		return err
	}
	server.set("args", bytes.TrimSpace(args.Bytes()))
	servers.setObject("almagest", server)
	m.setObject("mcpServers", servers)
	out, err := m.MarshalIndent()
	if err != nil {
		return err
	}
	return os.WriteFile(file, out, 0o644)
}

// ordered is a JSON object that keeps its keys in the order of the file.
type ordered struct {
	keys   []string
	values map[string]json.RawMessage
}

func (o *ordered) UnmarshalJSON(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	if _, err := dec.Token(); err != nil {
		return err
	}
	o.values = map[string]json.RawMessage{}
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return err
		}
		key := t.(string)
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return err
		}
		o.keys = append(o.keys, key)
		o.values[key] = v
	}
	_, err := dec.Token()
	return err
}

func (o *ordered) object(key string) (*ordered, error) {
	var child ordered
	if err := json.Unmarshal(o.values[key], &child); err != nil {
		return nil, fmt.Errorf("%s: %w", key, err)
	}
	return &child, nil
}

func (o *ordered) set(key string, v json.RawMessage) {
	if _, ok := o.values[key]; !ok {
		o.keys = append(o.keys, key)
	}
	o.values[key] = v
}

func (o *ordered) setObject(key string, child *ordered) {
	raw, _ := child.marshal()
	o.set(key, raw)
}

func (o *ordered) marshal() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, k := range o.keys {
		if i > 0 {
			b.WriteByte(',')
		}
		key, _ := json.Marshal(k) // keys hold no < > &
		b.Write(key)
		b.WriteByte(':')
		b.Write(o.values[k])
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// MarshalIndent is the object as the manifest holds it: two spaces, a final newline.
func (o *ordered) MarshalIndent() ([]byte, error) {
	raw, err := o.marshal()
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	if err := json.Indent(&b, raw, "", "  "); err != nil {
		return nil, err
	}
	b.WriteByte('\n')
	return b.Bytes(), nil
}
