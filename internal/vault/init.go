package vault

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/gitx"
)

//go:embed template
var templates embed.FS

// PluginFiles are the Obsidian plugin's files, as init installs them.
var PluginFiles = []string{"manifest.json", "main.js", "styles.css"}

// InitOptions are the answers of onboarding.
type InitOptions struct {
	Path        string
	Name        string
	Description string
	// Tagging is open or known.
	Tagging string
	// Context is the body of Atlas.md; the description when empty.
	Context string
}

// Init makes a folder a vault: the layout, Atlas.md, the two Bases, the Obsidian plugin,
// git init when the folder is no repository, and one setup commit; then it lists the
// folder in the machine file. A folder that holds notes, or that is already the root of a
// repository, is adopted: init adds its files and never moves or edits one that is there.
func Init(opts InitOptions, h Home, now time.Time) (*Vault, error) {
	if strings.TrimSpace(opts.Path) == "" {
		return nil, errors.New("init needs the vault's folder")
	}
	name := strings.TrimSpace(opts.Name)
	if name == "" {
		return nil, errors.New("init needs the vault's name")
	}
	tagging := opts.Tagging
	if tagging == "" {
		tagging = "open"
	}
	if !slices.Contains(TaggingModes, tagging) {
		return nil, fmt.Errorf("tagging is %q; it must be open or known", tagging)
	}
	root, err := filepath.Abs(Expand(opts.Path))
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	if _, err := os.Stat(filepath.Join(root, Marker)); err == nil {
		return nil, fmt.Errorf("%s already holds an %s; it is a vault", root, Marker)
	}
	if top := gitx.Top(root); top != "" && !gitx.IsRoot(root) {
		return nil, fmt.Errorf("%s is inside the repository at %s; a vault must be the root of its own repository, so choose a folder outside it", root, top)
	}
	cfg, err := h.Load()
	if err != nil {
		return nil, err
	}
	if cfg.Lists(root) {
		return nil, fmt.Errorf("the machine file already lists %s", root)
	}
	g := gitx.Repo{Dir: root}
	if !gitx.IsRoot(root) {
		if err := g.Init(); err != nil {
			return nil, err
		}
	}
	v := &Vault{Root: root}
	unlock, err := v.Lock()
	if err != nil {
		return nil, err
	}
	defer unlock()
	hadHead := g.HasHead()
	if hadHead {
		if _, err := CommitSnapshot(v); err != nil {
			return nil, err
		}
	}
	written, err := writeLayout(v, name, strings.TrimSpace(opts.Description), tagging, opts.Context, now)
	if err != nil {
		return nil, err
	}
	if hadHead {
		err = g.Add(written...)
	} else {
		err = g.AddAll()
	}
	if err != nil {
		return nil, err
	}
	if _, err := g.Commit("setup: " + name); err != nil {
		return nil, err
	}
	cfg.Add(root)
	if err := h.Save(cfg); err != nil {
		return nil, err
	}
	return Open(root)
}

func writeLayout(v *Vault, name, description, tagging, context string, now time.Time) ([]string, error) {
	if err := v.EnsureFolders(); err != nil {
		return nil, err
	}
	var written []string
	body := strings.TrimSpace(context)
	if body == "" {
		body = description
	}
	if body != "" {
		body += "\n"
	}
	atlas := doc.Render([]doc.Field{
		{Key: "id", Value: doc.NewID("vlt", nil)},
		{Key: "type", Value: "vault"},
		{Key: "name", Value: name},
		{Key: "description", Value: description},
		{Key: "created", Value: Stamp(now)},
		{Key: "updated", Value: Stamp(now)},
		{Key: "tagging", Value: tagging},
		{Key: "stale_hours", Value: DefaultStaleHours},
		{Key: "layout", Value: Layout},
	}, body)
	if err := v.Write(Marker, []byte(atlas)); err != nil {
		return nil, err
	}
	written = append(written, Marker)
	for name, rel := range Bases {
		if v.Exists(rel) {
			continue
		}
		data, err := templates.ReadFile("template/" + name)
		if err != nil {
			return nil, err
		}
		if err := v.Write(rel, data); err != nil {
			return nil, err
		}
		written = append(written, rel)
	}
	if wrote, err := ObsidianSettings(v); err != nil {
		return nil, err
	} else if wrote {
		written = append(written, AppJSON)
	}
	plugin, err := InstallPlugin(v)
	if err != nil {
		return nil, err
	}
	return append(written, plugin...), nil
}

// InstallPlugin copies the Obsidian plugin this binary carries into the vault, and
// returns the paths it wrote. The user turns the plugin on once in Obsidian.
func InstallPlugin(v *Vault) ([]string, error) {
	var written []string
	for _, f := range PluginFiles {
		data, err := templates.ReadFile("template/obsidian/" + f)
		if err != nil {
			return nil, err
		}
		rel := path.Join(PluginDir, f)
		wrote, err := v.WriteMachineIfChanged(rel, data)
		if err != nil {
			return nil, err
		}
		if wrote {
			written = append(written, rel)
		}
	}
	return written, nil
}

// PluginVersion is the version of the Obsidian plugin this binary carries.
func PluginVersion() string {
	data, err := fs.ReadFile(templates, "template/obsidian/manifest.json")
	if err != nil {
		return ""
	}
	return manifestVersion(data)
}

// InstalledPluginVersion is the version of the Obsidian plugin in the vault, or "".
func (v *Vault) InstalledPluginVersion() string {
	data, err := v.Read(path.Join(PluginDir, "manifest.json"))
	if err != nil {
		return ""
	}
	return manifestVersion(data)
}

func manifestVersion(data []byte) string {
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if rest, ok := strings.CutPrefix(line, `"version":`); ok {
			return strings.Trim(strings.TrimSpace(rest), `",`)
		}
	}
	return ""
}

// Template is the content of one template file, for tests.
func Template(name string) ([]byte, error) { return templates.ReadFile("template/" + name) }
