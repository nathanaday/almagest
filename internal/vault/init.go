package vault

import (
	"embed"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/nathanaday/almagest/internal/doc"
	"github.com/nathanaday/almagest/internal/gitx"
)

//go:embed template
var templates embed.FS

// InitOptions are the answers of onboarding.
type InitOptions struct {
	Path        string
	Name        string
	Description string
	// Tagging is open or known.
	Tagging string
	// Context is the body of Almagest.md; the description when empty.
	Context string
}

// Init makes a folder a vault: the layout, Almagest.md, the two Bases, the Obsidian plugin,
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
	almagest := doc.Render([]doc.Field{
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
	if err := v.Write(Marker, []byte(almagest)); err != nil {
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
	return written, nil
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
