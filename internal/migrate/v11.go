package migrate

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/nathanaday/almagest/internal/doc"
	"github.com/nathanaday/almagest/internal/links"
	"github.com/nathanaday/almagest/internal/vault"
)

// The names that 11.0 changes in what code wrote into a vault, when the project was Atlas.
const (
	legacyConfigDir    = ".atlas"
	legacyVaultSchema  = "atlas.vault-config.v1"
	legacyHistoryLine  = "> [!atlas] Written by Atlas at each publish. Edits here are lost at the next one."
	legacyCalloutKind  = "atlas"
	legacyChangeClass  = "atlas-change"
	legacyChangeBlock  = "atlas-change"
	legacyRepoBlock    = "atlas-repo"
	legacyMarkerTitle  = "Atlas"
	almagestCalloutKey = "almagest"
)

var (
	legacyFence   = regexp.MustCompile("(?m)^([ \t]*(?:```+|~~~+)[ \t]*)(" + legacyChangeBlock + "|" + legacyRepoBlock + ")([ \t]*)$")
	legacyCallout = regexp.MustCompile(`(?m)^([ \t]*>[ \t]*)\[!` + legacyCalloutKind + `\]([+-]?)(.*)$`)
)

// build11 computes the step from 10.0 to 11.0, the rename of Atlas to Almagest in what
// code owns in a vault: Atlas.md becomes Almagest.md with layout 7, a link to it follows;
// the change and repository blocks, the change cssclass, and the callouts of the
// checkouts and the publication histories take the new names; .atlas/ becomes .almagest/.
// The user's own prose stays as written, and so do the views, which code writes again.
func build11(v *vault.Vault, report *Report) (*plan, error) {
	p := &plan{}
	marker := v.Doc.Path
	if marker == vault.LegacyMarker {
		if v.Exists(vault.Marker) {
			return nil, fmt.Errorf("both %s and %s exist; keep the one of type vault, then migrate", vault.LegacyMarker, vault.Marker)
		}
		p.moves = append(p.moves, Move{From: vault.LegacyMarker, To: vault.Marker})
	}
	if st, err := os.Stat(v.Abs(legacyConfigDir)); err == nil && st.IsDir() {
		if v.Exists(".almagest") {
			return nil, fmt.Errorf("both %s/ and .almagest/ exist; merge them into .almagest/, then migrate", legacyConfigDir)
		}
		err := walkFiles(v, legacyConfigDir, func(rel string) error {
			p.moves = append(p.moves, Move{From: rel, To: ".almagest" + strings.TrimPrefix(rel, legacyConfigDir)})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	if err := claim(v, p.moves); err != nil {
		return nil, err
	}
	report.Moved = append(report.Moved, p.moves...)
	dest := map[string]string{}
	for _, m := range p.moves {
		dest[m.From] = m.To
	}
	// A link to the vault document follows it, unless another note holds the old title,
	// which the link may then name.
	rename := links.Rename{legacyMarkerTitle: strings.TrimSuffix(vault.Marker, ".md")}
	if others := notesTitled(v, legacyMarkerTitle); others > 1 || (others == 1 && marker != vault.LegacyMarker) {
		rename = nil
		report.Warnings = append(report.Warnings, "another note is titled Atlas, so the links to [[Atlas]] stay as written; point the ones that mean the vault document at [[Almagest]]")
	}
	err := filepath.WalkDir(v.Root, func(abs string, e fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel := v.Rel(abs)
		if e.IsDir() {
			if abs != v.Root && (strings.HasPrefix(e.Name(), ".") || rel == vault.Trash || rel == vault.WikiView || rel == vault.Ingest || rel == vault.Originals || e.Name() == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(path.Ext(rel))
		if ext != ".md" && ext != ".canvas" && ext != ".base" {
			return nil
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			return err
		}
		content, _ := links.Rewrite(string(data), rename)
		if ext == ".md" {
			content = rename11(rel, content)
		}
		if content == string(data) {
			return nil
		}
		at := rel
		if to, ok := dest[rel]; ok {
			at = to
		}
		p.edits = append(p.edits, edit{at, content})
		report.Edited = append(report.Edited, at)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if data, err := v.Read(legacyConfigDir + "/config.json"); err == nil {
		if content, ok := reschema(data); ok {
			p.edits = append(p.edits, edit{vault.VaultConfigFile, content})
			report.Edited = append(report.Edited, vault.VaultConfigFile)
		}
	}
	if st, err := os.Stat(v.Abs(vault.LegacyPluginDir)); err == nil && st.IsDir() {
		report.Warnings = append(report.Warnings, fmt.Sprintf("%s/ holds the Atlas plugin, which runs the binary of before 11.0; turn it off and remove it in Obsidian (Settings, Community plugins), then install Almagest from the community plugins: %s", vault.LegacyPluginDir, vault.PluginLink))
	}
	return p, nil
}

// rename11 gives one note the names of 11.0, where code wrote the old ones.
func rename11(rel, content string) string {
	d := doc.Parse(rel, []byte(content))
	front, body, hasFront := doc.Split(content)
	switch {
	case vault.IsMarker(rel) && d.Type() == "vault":
		return doc.SetField(content, "layout", vault.Layout)
	case d.Type() == "change":
		if hasFront {
			front = strings.ReplaceAll(front, legacyChangeClass, "almagest-change")
			content = doc.Join(front, body)
		}
		return legacyFence.ReplaceAllString(content, "${1}almagest-change$3")
	case d.Type() == "repository":
		return legacyFence.ReplaceAllString(content, "${1}almagest-repo$3")
	case strings.HasPrefix(rel, vault.Checkout+"/"):
		return legacyCallout.ReplaceAllStringFunc(content, renameCallout)
	case strings.HasPrefix(rel, vault.Journals+"/") && strings.HasPrefix(path.Base(rel), "Journal · "):
		return strings.Replace(content, legacyHistoryLine, "> [!almagest] Written by Almagest at each publish. Edits here are lost at the next one.", 1)
	}
	return content
}

// renameCallout gives a callout code wrote in a checkout the new kind and the new name.
func renameCallout(line string) string {
	m := legacyCallout.FindStringSubmatch(line)
	return m[1] + "[!" + almagestCalloutKey + "]" + m[2] + strings.Replace(m[3], "Written by Atlas", "Written by Almagest", 1)
}

// reschema is the vault config of before 11.0 under the new schema, and whether it was one.
func reschema(data []byte) (string, bool) {
	var m map[string]any
	if json.Unmarshal(data, &m) != nil || m["schema"] != legacyVaultSchema {
		return "", false
	}
	return strings.Replace(string(data), `"`+legacyVaultSchema+`"`, `"`+vault.VaultConfigSchema+`"`, 1), true
}

// notesTitled counts the markdown files of the vault whose title is title, without regard
// to case, outside the dot folders and trash/.
func notesTitled(v *vault.Vault, title string) int {
	n := 0
	filepath.WalkDir(v.Root, func(abs string, e fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if e.IsDir() {
			if abs != v.Root && (strings.HasPrefix(e.Name(), ".") || v.Rel(abs) == vault.Trash) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.EqualFold(e.Name(), title+".md") {
			n++
		}
		return nil
	})
	return n
}
