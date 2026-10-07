// Package journal publishes the user's journals. A volume is a folder directly under
// journals/; its notes are the user's own words, which no agent changes. Publish
// captures the whole volume as one source, an edition, which the wiki then cites like
// any source; every edition stays in source-core/originals.
package journal

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/source"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

// HistoryPrefix begins the title of the note code writes at the root of a volume, its
// publication history: "Journal · <volume>". The prefix is reserved, so no document
// takes the title, and each volume's folder name keeps it unique.
const HistoryPrefix = "Journal · "

// HistoryNotice opens the publication history: code writes it again at each publish.
const HistoryNotice = "> [!atlas] Written by Atlas at each publish. Edits here are lost at the next one."

// Volume is one journal volume and its state against its latest edition.
type Volume struct {
	Volume  string `json:"volume"`
	Name    string `json:"name"`
	Notes   int    `json:"notes"`
	Edition string `json:"edition"`
	Changed bool   `json:"changed"`
}

// Name is how a volume's folder reads in an edition's title: "-" and "_" are spaces; a
// word that mixes letters and digits is upper case, any other word starts with a capital.
func Name(folder string) string {
	words := strings.FieldsFunc(folder, func(r rune) bool { return r == '-' || r == '_' || unicode.IsSpace(r) })
	for i, w := range words {
		letters, digits := false, false
		for _, r := range w {
			letters = letters || unicode.IsLetter(r)
			digits = digits || unicode.IsDigit(r)
		}
		if letters && digits {
			words[i] = strings.ToUpper(w)
		} else {
			words[i] = doc.Capital(w)
		}
	}
	return strings.Join(words, " ")
}

// Title is the title of a volume's edition of a day: "User Journal CS566 Notes - 6
// October 2026 Edition".
func Title(folder string, day time.Time) string {
	return fmt.Sprintf("User Journal %s - %d %s %d Edition", Name(folder), day.Day(), day.Month(), day.Year())
}

// Volumes lists the volumes, each with its notes and its latest edition.
func Volumes(idx *vault.Index) []Volume {
	out := []Volume{}
	entries, _ := os.ReadDir(idx.V.Abs(vault.Journals))
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		notes, err := notesOf(idx.V, e.Name())
		if err != nil {
			continue
		}
		vol := Volume{Volume: e.Name(), Name: Name(e.Name()), Notes: len(notes)}
		latest := Latest(idx, e.Name())
		if latest != nil {
			vol.Edition = vault.Title(latest)
		}
		if len(notes) > 0 {
			_, hash, err := text(idx.V, e.Name(), notes)
			vol.Changed = err == nil && (latest == nil || latest.Str("journal_hash") != hash)
		}
		out = append(out, vol)
	}
	return out
}

// Latest is a volume's latest edition, or nil.
func Latest(idx *vault.Index, folder string) *doc.Doc {
	var best *doc.Doc
	for _, s := range idx.Of("source") {
		if s.Str("volume") != folder {
			continue
		}
		if best == nil || s.Str("captured") > best.Str("captured") || (s.Str("captured") == best.Str("captured") && s.Path > best.Path) {
			best = s
		}
	}
	return best
}

// notesOf lists a volume's notes, in path order, but its publication history.
func notesOf(v *vault.Vault, folder string) ([]string, error) {
	root := path.Join(vault.Journals, folder)
	if err := v.Contain(root); err != nil {
		return nil, err
	}
	st, err := os.Stat(v.Abs(root))
	if err != nil || !st.IsDir() {
		return nil, fmt.Errorf("no journal volume %q: a volume is a folder directly under %s/", folder, vault.Journals)
	}
	var out []string
	err = filepath.WalkDir(v.Abs(root), func(abs string, e fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if strings.HasPrefix(e.Name(), ".") && abs != v.Abs(root) {
			if e.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel := v.Rel(abs)
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(rel), ".md") || rel == historyPath(folder) {
			return nil
		}
		if e.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		out = append(out, rel)
		return nil
	})
	sort.Strings(out)
	return out, err
}

func historyPath(folder string) string {
	return path.Join(vault.Journals, folder, HistoryPrefix+folder+".md")
}

// text is a volume's notes as one markdown text, one section per note, and the hash of
// the notes alone, which no date changes.
func text(v *vault.Vault, folder string, notes []string) (string, string, error) {
	var b strings.Builder
	h := sha256.New()
	for _, rel := range notes {
		data, err := v.Read(rel)
		if err != nil {
			return "", "", err
		}
		name := strings.TrimSuffix(strings.TrimPrefix(rel, path.Join(vault.Journals, folder)+"/"), path.Ext(rel))
		_, body, ok := doc.Split(string(data))
		if !ok {
			body = string(data)
		}
		body = strings.TrimSpace(body)
		fmt.Fprintf(h, "%s\x00%s\x00", name, body)
		fmt.Fprintf(&b, "## %s\n\n%s\n\n", name, body)
	}
	return strings.TrimSpace(b.String()) + "\n", hex.EncodeToString(h.Sum(nil)), nil
}

// Published is what a publish captured.
type Published struct {
	Source  vault.Ref `json:"source"`
	Volume  string    `json:"volume"`
	Edition string    `json:"edition"`
	Hash    string    `json:"hash"`
	Commit  string    `json:"commit,omitempty"`
}

// Publish captures a volume as one edition, the user's act, in one commit with the
// volume's publication history. It refuses a volume with no change since its latest
// edition.
func Publish(v *vault.Vault, folder string, now time.Time) (*Published, error) {
	if err := v.CheckLayout(); err != nil {
		return nil, err
	}
	folder = strings.Trim(strings.TrimPrefix(filepath.ToSlash(folder), vault.Journals+"/"), "/")
	if folder == "" || strings.Contains(folder, "/") || folder == "." || folder == ".." || path.Clean(folder) != folder {
		return nil, fmt.Errorf("%q: name a volume, a folder directly under %s/", folder, vault.Journals)
	}
	notes, err := notesOf(v, folder)
	if err != nil {
		return nil, err
	}
	if len(notes) == 0 {
		return nil, fmt.Errorf("the volume %s holds no note", folder)
	}
	body, hash, err := text(v, folder, notes)
	if err != nil {
		return nil, err
	}
	idx, err := vault.Load(v)
	if err != nil {
		return nil, err
	}
	var tags []string
	if latest := Latest(idx, folder); latest != nil {
		if latest.Str("journal_hash") == hash {
			return nil, fmt.Errorf("%s has no change since %s", folder, vault.Title(latest))
		}
		tags = latest.List("tags")
	}
	title := Title(folder, now)
	res, err := source.Capture(v, source.Request{
		Text:    "# " + title + "\n\n" + body,
		Title:   title,
		Locator: path.Join(vault.Journals, folder),
		Tags:    tags,
		NewTags: true,
		Journal: &source.Edition{Volume: folder, Date: vault.Date(now), Hash: hash, Also: map[string]string{historyPath(folder): HistoryNote(folder)}},
	}, now)
	if err != nil {
		return nil, err
	}
	if len(res.Captured) == 0 || res.Captured[0].Duplicate != "" {
		return nil, errors.New("the edition holds the same text as a source captured before; nothing was published")
	}
	return &Published{Source: res.Captured[0].Ref, Volume: folder, Edition: vault.Date(now), Hash: hash, Commit: res.Commit}, nil
}

// HistoryNote is the publication history of a volume: its editions, as an inline Base.
func HistoryNote(folder string) string {
	return HistoryNotice + "\n\n" +
		"Each edition is a copy of this volume, captured as a source when you pressed Publish. The wiki cites the edition; this volume stays yours.\n\n" +
		"```base\nfilters:\n  and:\n    - file.inFolder(\"" + vault.Documents + "\")\n    - 'type == \"source\"'\n    - 'volume == " + baseString(folder) + "'\n" +
		"views:\n  - type: table\n    name: Editions\n    order:\n      - file.name\n      - edition\n      - measure\n      - status\n    sort:\n      - property: edition\n        direction: DESC\n```\n"
}

// baseString is a folder name as a string of a Base expression inside a YAML single-quoted
// scalar: the expression's quotes and backslashes escaped, then YAML's single quote
// doubled.
func baseString(s string) string {
	s = `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
	return strings.ReplaceAll(s, "'", "''")
}

// WriteMissingHistories writes the publication history of each volume that has an
// edition and lacks the note. It commits nothing; the next snapshot keeps them.
func WriteMissingHistories(idx *vault.Index) ([]string, error) {
	var out []string
	for _, vol := range Volumes(idx) {
		rel := historyPath(vol.Volume)
		if vol.Edition == "" || idx.V.Exists(rel) {
			continue
		}
		if err := idx.V.Write(rel, []byte(HistoryNote(vol.Volume))); err != nil {
			return out, err
		}
		out = append(out, rel)
	}
	return out, nil
}
