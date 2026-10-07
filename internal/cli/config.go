package cli

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/nathanaday/almagest/internal/vault"
)

// configView is what config prints: the preferences Almagest acts on, and each file's own.
type configView struct {
	Preferences vault.Effective    `json:"preferences"`
	Global      vault.Preferences  `json:"global"`
	Vault       *vault.Preferences `json:"vault"`
	Files       map[string]string  `json:"files"`
}

func (c *CLI) configCmd(argv []string) error {
	a := parse(argv, "global")
	home := c.home()
	global, err := home.Load()
	if err != nil {
		return err
	}
	switch a.arg(0) {
	case "", "show":
		view := configView{Global: global.Preferences, Files: map[string]string{"global": home.ConfigPath()}}
		var local vault.Preferences
		// Outside a vault, config shows the machine's preferences; a vault named with --vault
		// or $ALMAGEST_VAULT must open.
		if v, err := c.open(a); err == nil {
			vc, err := v.LoadConfig()
			if err != nil {
				return err
			}
			local = vc.Preferences
			view.Vault = &local
			view.Files["vault"] = v.ConfigPath()
		} else if c.namesVault(a) {
			return err
		}
		view.Preferences = vault.Merge(global.Preferences, local)
		return c.emit(a, view, func(w io.Writer) { printConfig(w, view) })
	case "set", "unset":
		key, value := a.arg(1), a.arg(2)
		if a.arg(0) == "unset" {
			value = ""
		} else if value == "" {
			return fmt.Errorf("usage: almagest config set KEY VALUE [--global]; the keys are %s", strings.Join(vault.PreferenceKeys(), ", "))
		}
		if a.has("global") {
			// The result shows the vault a call names; check it before the write, so a bad
			// --vault or $ALMAGEST_VAULT fails with nothing written.
			if c.namesVault(a) {
				if _, err := c.open(a); err != nil {
					return err
				}
			}
			if err := global.Preferences.Set(key, value); err != nil {
				return err
			}
			if err := home.Save(global); err != nil {
				return err
			}
			show := []string{"show"}
			if a.has("vault") {
				show = append(show, "--vault", a.get("vault"))
			}
			return c.configCmd(append(show, jsonFlag(a)...))
		}
		v, err := c.open(a)
		if err != nil {
			if c.namesVault(a) {
				return err
			}
			return fmt.Errorf("%w; to set it for every vault, add --global", err)
		}
		vc, err := v.LoadConfig()
		if err != nil {
			return err
		}
		if err := vc.Preferences.Set(key, value); err != nil {
			return err
		}
		if err := v.SaveConfig(vc); err != nil {
			return err
		}
		return c.configCmd(append([]string{"show", "--vault", v.Root}, jsonFlag(a)...))
	default:
		return fmt.Errorf("no config command %q; use show, set, or unset", a.arg(0))
	}
}

func jsonFlag(a args) []string {
	if a.has("json") {
		return []string{"--json"}
	}
	return nil
}

func printConfig(w io.Writer, view configView) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, k := range vault.PreferenceKeys() {
		value := view.Preferences.Get(k)
		if value == "" {
			value = "-"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\n", k, value, view.Preferences.Sources[k])
	}
	tw.Flush()
	fmt.Fprintf(w, "\nGlobal: %s\n", view.Files["global"])
	if f := view.Files["vault"]; f != "" {
		fmt.Fprintf(w, "Vault:  %s (wins over the global file)\n", f)
	} else {
		fmt.Fprintln(w, "No vault here; the global file and the defaults apply.")
	}
}

// namesVault reports whether a call names its vault, with --vault or $ALMAGEST_VAULT, as
// vault.Select reads them; a vault so named must open.
func (c *CLI) namesVault(a args) bool {
	return strings.TrimSpace(a.get("vault")) != "" || strings.TrimSpace(c.Getenv(vault.EnvVault)) != ""
}
