// Package wikify marks a copy of a note with what the wiki knows: a link mark where a
// phrase names a document, a new mark where it names a subject worth a topic. The copy
// lies in the scratchpad and never enters the wiki by itself; the Obsidian plugin turns
// each mark into a link, or back into its phrase, when the user decides.
package wikify

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/nathanaday/almagest/internal/doc"
	"github.com/nathanaday/almagest/internal/links"
	"github.com/nathanaday/almagest/internal/vault"
)

// Suffix ends the name of every wikified copy.
const Suffix = " · wikified"

// Start copies a note into the scratchpad as "<name> · wikified.md", a number before the
// suffix's end when that name is taken, and returns the copy's path. The original stays.
func Start(v *vault.Vault, name string) (string, error) {
	rel := name
	if filepath.IsAbs(name) {
		rel = v.Rel(name)
	}
	rel = path.Clean(filepath.ToSlash(rel))
	if err := v.Contain(rel); err != nil {
		return "", err
	}
	rel = v.Spelled(rel)
	refused := func(dir string) bool { return vault.InFolder(rel, dir) }
	switch {
	case !strings.HasSuffix(strings.ToLower(rel), ".md"):
		return "", fmt.Errorf("%s is no markdown note", rel)
	case refused(vault.Core) || refused(vault.Changes) || refused(vault.Sessions) || refused(vault.WikiView) || refused(vault.Trash) || refused(vault.Obsidian) || vault.IsMarker(rel):
		return "", fmt.Errorf("%s is code's or the wiki's; wikify takes a note of yours", rel)
	}
	unlock, err := v.Lock()
	if err != nil {
		return "", err
	}
	defer unlock()
	data, err := v.Read(rel)
	if err != nil {
		return "", fmt.Errorf("%s is no note of the vault", rel)
	}
	base := strings.TrimSuffix(path.Base(rel), path.Ext(rel))
	to := path.Join(vault.Scratchpad, base+Suffix+".md")
	for n := 2; v.Exists(to); n++ {
		to = path.Join(vault.Scratchpad, fmt.Sprintf("%s%s (%d).md", base, Suffix, n))
	}
	if err := os.MkdirAll(v.Abs(vault.Scratchpad), 0o755); err != nil {
		return "", err
	}
	if err := v.Write(to, data); err != nil {
		return "", err
	}
	return to, nil
}

// Mark is one phrase to mark: a link to a document, or a new subject.
type Mark struct {
	Phrase string `json:"phrase" jsonschema:"the words as the note writes them"`
	Link   string `json:"link,omitempty" jsonschema:"the document the phrase names, by id or title"`
	New    string `json:"new,omitempty" jsonschema:"the title of the topic the subject is worth"`
}

// Marked is what a mark call did.
type Marked struct {
	Note    string   `json:"note"`
	Placed  []string `json:"placed"`
	Missing []string `json:"missing"`
}

// Text is a mark as the note holds it.
func Text(kind, title, phrase string) string {
	return "{{" + kind + ":" + title + "|" + phrase + "}}"
}

var (
	markText  = regexp.MustCompile(`\{\{(?:link|new):[^{}|\n]+\|[^{}\n]+\}\}`)
	mdLink    = regexp.MustCompile(`\[[^\]\n]*\]\([^)\n]*\)`)
	urlText   = regexp.MustCompile(`https?://\S+`)
	headingLn = regexp.MustCompile(`(?m)^ {0,3}#{1,6}[ \t].*$`)
	tableRow  = regexp.MustCompile(`(?m)^[ \t]*\|.*$`)
)

// Place marks the first free mention of each phrase in a wikified copy, writes the note
// once, and says which phrases it could not place. It commits nothing: the note is the
// user's, and the next snapshot keeps it.
func Place(idx *vault.Index, note string, marks []Mark) (*Marked, error) {
	v := idx.V
	rel := path.Clean(filepath.ToSlash(note))
	if filepath.IsAbs(note) {
		rel = v.Rel(note)
	}
	if path.Dir(rel) != vault.Scratchpad || !strings.Contains(path.Base(rel), Suffix) || !strings.HasSuffix(rel, ".md") {
		return nil, fmt.Errorf("%s is no wikified copy; wikify start makes one in %s/", rel, vault.Scratchpad)
	}
	if err := v.Contain(rel); err != nil {
		return nil, err
	}
	unlock, err := v.Lock()
	if err != nil {
		return nil, err
	}
	defer unlock()
	data, err := v.Read(rel)
	if err != nil {
		return nil, fmt.Errorf("%s is no note of the vault", rel)
	}
	if len(marks) == 0 {
		return nil, errors.New("mark needs at least one mark")
	}
	front, body, hasFront := doc.Split(string(data))
	if !hasFront {
		body = string(data)
	}
	out := &Marked{Note: rel, Placed: []string{}, Missing: []string{}}
	for _, m := range marks {
		phrase := strings.TrimSpace(m.Phrase)
		kind, title, err := resolve(idx, m)
		switch {
		case err != nil:
			return nil, fmt.Errorf("%q: %w", phrase, err)
		case phrase == "" || strings.ContainsAny(phrase, "|{}\n"):
			return nil, fmt.Errorf("%q: a phrase is words of one line, with no | or braces", phrase)
		}
		at, end := find(body, phrase)
		if at < 0 {
			out.Missing = append(out.Missing, phrase)
			continue
		}
		body = body[:at] + Text(kind, title, body[at:end]) + body[end:]
		out.Placed = append(out.Placed, phrase)
	}
	content := body
	if hasFront {
		content = doc.Join(front, body)
	}
	if err := v.Write(rel, []byte(content)); err != nil {
		return nil, err
	}
	return out, nil
}

// resolve gives a mark its kind and its title: a link names a document that exists; a
// new subject's title must be free, and one a document holds is a link.
func resolve(idx *vault.Index, m Mark) (string, string, error) {
	link, fresh := strings.TrimSpace(m.Link), doc.CleanTitle(m.New)
	switch {
	case (link == "") == (fresh == ""):
		return "", "", errors.New("give exactly one of link and new")
	case link != "":
		d, err := idx.Resolve(link)
		if err != nil {
			return "", "", err
		}
		return "link", vault.Title(d), nil
	}
	if err := doc.CheckTitle(fresh); err != nil {
		return "", "", err
	}
	if d, err := idx.Resolve(fresh); err == nil && d != nil {
		return "link", vault.Title(d), nil
	}
	return "new", fresh, nil
}

// find is the byte span of the first mention of phrase in body (its length may differ
// from the phrase's, as case folding may match a letter of another width), as whole words and
// without regard to case, outside headings, table rows (a mark's | would split a cell),
// code, comments, links, URLs, and marks; or -1.
func find(body, phrase string) (int, int) {
	masked := links.Mask(body)
	for _, re := range []*regexp.Regexp{markText, mdLink, urlText, headingLn, tableRow} {
		masked = re.ReplaceAllStringFunc(masked, spaces)
	}
	for _, l := range links.Find(masked) {
		masked = masked[:l.Start] + spaces(masked[l.Start:l.End]) + masked[l.End:]
	}
	re := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(phrase))
	for _, loc := range re.FindAllStringIndex(masked, -1) {
		if wordEdge(masked, loc[0], true) && wordEdge(masked, loc[1], false) {
			return loc[0], loc[1]
		}
	}
	return -1, -1
}

// wordEdge reports whether offset i starts (or ends) a whole word.
func wordEdge(s string, i int, start bool) bool {
	var r rune
	if start {
		if i == 0 {
			return true
		}
		r, _ = utf8.DecodeLastRuneInString(s[:i])
	} else {
		if i >= len(s) {
			return true
		}
		r, _ = utf8.DecodeRuneInString(s[i:])
	}
	return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_'
}

func spaces(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c != '\n' {
			b[i] = ' '
		}
	}
	return string(b)
}
