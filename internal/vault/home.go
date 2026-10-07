package vault

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// EnvHome overrides the machine folder, ~/.almagest. Tests always set it.
const EnvHome = "ALMAGEST_HOME"

// ConfigSchema is the schema of the machine file.
const ConfigSchema = "almagest.config.v1"

// Home is the machine folder: the config file, and the binaries.
type Home struct {
	Root string
}

// HomeFrom resolves the machine folder from the environment.
func HomeFrom(env func(string) string) Home {
	if env == nil {
		env = os.Getenv
	}
	if dir := env(EnvHome); dir != "" {
		return Home{Root: dir}
	}
	user, err := os.UserHomeDir()
	if err != nil {
		return Home{Root: ".almagest"}
	}
	return Home{Root: filepath.Join(user, ".almagest")}
}

// ConfigPath is the machine file.
func (h Home) ConfigPath() string { return filepath.Join(h.Root, "config.json") }

// VersionBin is where a version of the binary lies: the plugin's launcher runs the one of
// the plugin's version, and installs it there.
func (h Home) VersionBin(version string) string {
	return filepath.Join(h.Root, "bin", version, "almagest")
}

// LinkPath is the link to the binary installed last, which the Obsidian plugin and a
// shell run.
func (h Home) LinkPath() string { return filepath.Join(h.Root, "bin", "almagest") }

// InstallBinary copies the binary at from to the folder of version, and points the link
// at it. Each step replaces its file at once, so a launch never sees half of one.
func (h Home) InstallBinary(from, version string) (string, error) {
	to := h.VersionBin(version)
	data, err := os.ReadFile(from)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return "", err
	}
	tmp := to + ".tmp"
	if err := os.WriteFile(tmp, data, 0o755); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, to); err != nil {
		return "", err
	}
	link := h.LinkPath() + ".tmp"
	os.Remove(link)
	if err := os.Symlink(filepath.Join(version, "almagest"), link); err != nil {
		return "", err
	}
	return to, os.Rename(link, h.LinkPath())
}

// Config is the machine file: the paths of this machine's vaults, and the preferences
// of every vault.
type Config struct {
	Schema      string      `json:"schema"`
	Vaults      []string    `json:"vaults"`
	Preferences Preferences `json:"preferences"`
}

// Load reads the machine file. A missing file is an empty config.
func (h Home) Load() (*Config, error) {
	data, err := os.ReadFile(h.ConfigPath())
	if errors.Is(err, os.ErrNotExist) {
		return &Config{Schema: ConfigSchema}, nil
	}
	if err != nil {
		return nil, err
	}
	var c Config
	if err := decodeStrict(data, &c); err != nil {
		return nil, fmt.Errorf("%s: %w", h.ConfigPath(), err)
	}
	if c.Schema != ConfigSchema {
		return nil, fmt.Errorf("%s: schema %q is not %s", h.ConfigPath(), c.Schema, ConfigSchema)
	}
	if err := c.Preferences.check(); err != nil {
		return nil, fmt.Errorf("%s: %w", h.ConfigPath(), err)
	}
	return &c, nil
}

// Save writes the machine file.
func (h Home) Save(c *Config) error {
	c.Schema = ConfigSchema
	if c.Vaults == nil {
		c.Vaults = []string{}
	}
	data, err := marshalConfig(c)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(h.Root, 0o755); err != nil {
		return err
	}
	return writeAtomic(h.ConfigPath(), data)
}

// Paths are the vault folders, absolute.
func (c *Config) Paths() []string {
	out := make([]string, 0, len(c.Vaults))
	for _, v := range c.Vaults {
		out = append(out, Expand(v))
	}
	return out
}

// Lists reports whether the config lists the folder.
func (c *Config) Lists(dir string) bool {
	for _, p := range c.Paths() {
		if samePath(p, dir) {
			return true
		}
	}
	return false
}

// Add lists a folder, written with ~ when it is under the user's home.
func (c *Config) Add(dir string) {
	if !c.Lists(dir) {
		c.Vaults = append(c.Vaults, Shorten(dir))
	}
}

// Remove drops a folder from the list.
func (c *Config) Remove(dir string) bool {
	for i, p := range c.Paths() {
		if samePath(p, dir) {
			c.Vaults = append(c.Vaults[:i], c.Vaults[i+1:]...)
			return true
		}
	}
	return false
}

// Expand turns a leading ~ into the user's home folder.
func Expand(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if user, err := os.UserHomeDir(); err == nil {
			return filepath.Join(user, strings.TrimPrefix(p, "~"))
		}
	}
	return p
}

// Shorten writes a path under the user's home with ~.
func Shorten(p string) string {
	user, err := os.UserHomeDir()
	if err != nil || user == "" {
		return p
	}
	if p == user {
		return "~"
	}
	if rest, ok := strings.CutPrefix(p, user+string(filepath.Separator)); ok {
		return "~/" + filepath.ToSlash(rest)
	}
	return p
}

// samePath compares two folders after resolving symbolic links.
func samePath(a, b string) bool {
	return realPath(a) == realPath(b)
}

// realPath is p cleaned, absolute, and with symbolic links resolved where it exists.
func realPath(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	if real, err := filepath.EvalSymlinks(p); err == nil {
		return real
	}
	return filepath.Clean(p)
}

// Within reports whether path is dir or inside it.
func Within(path, dir string) bool {
	path, dir = realPath(path), realPath(dir)
	if path == dir {
		return true
	}
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// writeAtomic writes a file through a temporary file in the same folder and a rename. It
// syncs the file before the rename and the folder after it, so a power loss leaves the old
// file or the new one, never an empty one.
func writeAtomic(file string, data []byte) error {
	_, err := writeAtomicIf(file, data, nil)
	return err
}

// beforeRename lets a test act inside the window between the sync of a temporary file and
// its rename. It is nil outside tests.
var beforeRename func(file string)

// writeAtomicIf is writeAtomic with a check that runs after the temporary file is synced,
// right before the rename: when the check fails, it writes nothing and reports false. A
// guarded write passes its byte comparison, so a save that lands during the sync is kept.
func writeAtomicIf(file string, data []byte, check func() bool) (bool, error) {
	dir := filepath.Dir(file)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false, err
	}
	tmp, err := os.CreateTemp(dir, ".almagest-*")
	if err != nil {
		return false, err
	}
	name := tmp.Name()
	fail := func(err error) (bool, error) {
		tmp.Close()
		os.Remove(name)
		return false, err
	}
	if _, err := tmp.Write(data); err != nil {
		return fail(err)
	}
	if err := tmp.Sync(); err != nil {
		return fail(err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return false, err
	}
	if err := os.Chmod(name, 0o644); err != nil {
		os.Remove(name)
		return false, err
	}
	if beforeRename != nil {
		beforeRename(file)
	}
	if check != nil && !check() {
		os.Remove(name)
		return false, nil
	}
	if err := os.Rename(name, file); err != nil {
		os.Remove(name)
		return false, err
	}
	return true, syncDir(dir)
}

// syncDir makes a rename in dir durable. A file system that cannot sync a folder says
// EINVAL, and the rename stands as it is there.
func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	if err := d.Sync(); err != nil && !errors.Is(err, syscall.EINVAL) {
		return err
	}
	return nil
}
