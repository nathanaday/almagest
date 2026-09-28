package vault

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EnvHome overrides the machine folder, ~/.atlas. Tests always set it.
const EnvHome = "ATLAS_HOME"

// ConfigSchema is the schema of the machine file.
const ConfigSchema = "atlas.config.v1"

// Home is the machine folder: the config file, and the binary setup installs.
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
		return Home{Root: ".atlas"}
	}
	return Home{Root: filepath.Join(user, ".atlas")}
}

// ConfigPath is the machine file.
func (h Home) ConfigPath() string { return filepath.Join(h.Root, "config.json") }

// BinPath is where setup installs the binary.
func (h Home) BinPath() string { return filepath.Join(h.Root, "bin", "atlas") }

// Config is the machine file: the paths of this machine's vaults, and nothing else.
type Config struct {
	Schema string   `json:"schema"`
	Vaults []string `json:"vaults"`
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
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("%s: %w", h.ConfigPath(), err)
	}
	if c.Schema != ConfigSchema {
		return nil, fmt.Errorf("%s: schema %q is not %s", h.ConfigPath(), c.Schema, ConfigSchema)
	}
	return &c, nil
}

// Save writes the machine file.
func (h Home) Save(c *Config) error {
	c.Schema = ConfigSchema
	if c.Vaults == nil {
		c.Vaults = []string{}
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(h.Root, 0o755); err != nil {
		return err
	}
	return writeAtomic(h.ConfigPath(), append(data, '\n'))
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

// writeAtomic writes a file through a temporary file in the same folder and a rename.
func writeAtomic(file string, data []byte) error {
	dir := filepath.Dir(file)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".atlas-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Chmod(name, 0o644); err != nil {
		os.Remove(name)
		return err
	}
	return os.Rename(name, file)
}
