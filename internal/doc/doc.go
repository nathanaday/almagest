// Package doc reads and writes one document of the vault: its frontmatter, its body, and
// its lead callout. Every edit keeps the lines it does not change, so a property the user
// added by hand, or a list Obsidian reformatted, survives a write by code.
package doc

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"
	"unicode"
)

// Doc is one markdown file of the vault.
type Doc struct {
	// Path is relative to the vault, with forward slashes.
	Path string
	// Front is nil when the file has no frontmatter; FrontErr says why it did not parse.
	Front    *Front
	FrontErr error
	// Body is everything after the frontmatter's closing fence.
	Body    string
	Content string
}

// Parse reads content as the document at path.
func Parse(p string, content []byte) *Doc {
	d := &Doc{Path: p, Content: string(content)}
	front, body, ok := Split(d.Content)
	d.Body = body
	if ok {
		d.Front, d.FrontErr = ParseFront(front)
	}
	return d
}

// Title is the document's title: the file name without its extension.
func (d *Doc) Title() string { return TitleOf(d.Path) }

// TitleOf is the title a path gives: the base name without .md.
func TitleOf(p string) string {
	base := path.Base(p)
	if strings.HasSuffix(strings.ToLower(base), ".md") {
		return base[:len(base)-3]
	}
	return base
}

// Str is a frontmatter string, or "" when the document has none.
func (d *Doc) Str(key string) string {
	if d.Front == nil {
		return ""
	}
	return d.Front.Str(key)
}

// List is a frontmatter list, or nil.
func (d *Doc) List(key string) []string {
	if d.Front == nil {
		return nil
	}
	return d.Front.List(key)
}

// ID is the document's id field.
func (d *Doc) ID() string { return d.Str("id") }

// Type is the document's type field.
func (d *Doc) Type() string { return d.Str("type") }

// Split cuts content into the frontmatter text, without its fences, and the body. ok is
// false when the content does not open with a frontmatter fence that closes.
func Split(content string) (front, body string, ok bool) {
	rest, found := strings.CutPrefix(content, "---\n")
	if !found {
		if rest, found = strings.CutPrefix(content, "---\r\n"); !found {
			return "", content, false
		}
	}
	offset := 0
	for {
		line, next, more := strings.Cut(rest[offset:], "\n")
		trimmed := strings.TrimRight(line, "\r")
		if trimmed == "---" || trimmed == "..." {
			front = rest[:offset]
			if more {
				return front, next, true
			}
			return front, "", true
		}
		if !more {
			return "", content, false
		}
		offset += len(line) + 1
	}
}

// Join puts frontmatter text and a body back together.
func Join(front, body string) string {
	if front != "" && !strings.HasSuffix(front, "\n") {
		front += "\n"
	}
	return "---\n" + front + "---\n" + body
}

// Field is one frontmatter entry, in the order Render writes it.
type Field struct {
	Key   string
	Value any
}

// Render writes a new document from its fields, in order, and its body.
func Render(fields []Field, body string) string {
	var b strings.Builder
	b.WriteString("---\n")
	for _, f := range fields {
		b.WriteString(f.Key + ": " + FormatValue(f.Value) + "\n")
	}
	b.WriteString("---\n")
	if body != "" && !strings.HasPrefix(body, "\n") {
		b.WriteString("\n")
	}
	b.WriteString(body)
	return b.String()
}

// hashSpace matches the runs of whitespace a content hash folds into one space.
var hashSpace = regexp.MustCompile(`\s+`)

// ContentHash is the hash of a body with its lead callout removed and each run of
// whitespace made one space, so a reflowed line or a new stage callout is not an edit.
func ContentHash(body string) string {
	text := strings.TrimSpace(hashSpace.ReplaceAllString(StripLead(body), " "))
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// FileHash is the hash of a file's exact bytes, which a change records as a page's base.
func FileHash(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

// Short is the prefix of a hash that a document shows.
func Short(hash string) string {
	if len(hash) > 12 {
		return hash[:12]
	}
	return hash
}

// SameHash reports whether two hashes agree on the prefix both carry, so a short hash in a
// table matches the full hash it came from.
func SameHash(a, b string) bool {
	n := min(len(a), len(b))
	if n < 8 {
		return a == b
	}
	return strings.EqualFold(a[:n], b[:n])
}

// idAlphabet is lowercase Crockford base32: digits and letters without i, l, o, and u.
const idAlphabet = "0123456789abcdefghjkmnpqrstvwxyz"

// IDPattern matches an id code mints, and a session id.
var IDPattern = regexp.MustCompile(`^[a-z]{3}-[0-9a-z]{6}$`)

// NewID mints an id with prefix that taken does not hold.
func NewID(prefix string, taken func(string) bool) string {
	for {
		var raw [6]byte
		if _, err := rand.Read(raw[:]); err != nil {
			panic(err)
		}
		var b strings.Builder
		b.WriteString(prefix + "-")
		for _, c := range raw {
			b.WriteByte(idAlphabet[int(c)%len(idAlphabet)])
		}
		id := b.String()
		if taken == nil || !taken(id) {
			return id
		}
	}
}

// titleStrip holds the characters a file name cannot hold, the ones that break a
// wikilink to it, and "→", which splits the old and new titles of a change heading.
var titleStrip = strings.NewReplacer("/", "", "\\", "", ":", "", "*", "", "?", "", "\"", "", "<", "", ">", "", "|", "", "[", "", "]", "", "#", "", "^", "", "→", "")

// CleanTitle makes a title safe as a file name and as a link target: it removes the
// characters neither may hold, control characters, and a leading dot, and folds
// whitespace.
func CleanTitle(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
	s = titleStrip.Replace(s)
	s = strings.Join(strings.Fields(s), " ")
	s = strings.TrimLeft(s, ".")
	return strings.TrimSpace(s)
}

// MaxTitleBytes is the longest title a document takes, so the titles code derives from it
// (an event, a spec, a task list) still fit in a file name.
const MaxTitleBytes = 150

// CheckTitle refuses a clean title that is empty or longer than MaxTitleBytes.
func CheckTitle(t string) error {
	switch {
	case t == "":
		return errors.New("the title is empty once cleaned; give a title of words")
	case len(t) > MaxTitleBytes:
		return fmt.Errorf("the title %.40q… has %d bytes, and a title holds at most %d; give a shorter one", t, len(t), MaxTitleBytes)
	}
	return nil
}

// CutTitle shortens a title to at most n bytes, at a character boundary.
func CutTitle(t string, n int) string {
	if len(t) <= n {
		return t
	}
	cut := 0
	for i := range t {
		if i > n {
			break
		}
		cut = i
	}
	return strings.TrimSpace(t[:cut])
}

// Link is the wikilink to a title.
func Link(title string) string {
	if title == "" {
		return ""
	}
	return "[[" + title + "]]"
}

// Links are the wikilinks to titles.
func Links(titles []string) []string {
	out := make([]string, 0, len(titles))
	for _, t := range titles {
		if t != "" {
			out = append(out, Link(t))
		}
	}
	return out
}

// LinkTarget is the title a wikilink names, without its alias, heading, or block; a value
// that is not a wikilink comes back trimmed.
func LinkTarget(s string) string {
	s = strings.TrimSpace(s)
	inner, ok := strings.CutPrefix(s, "[[")
	if !ok {
		return s
	}
	inner, ok = strings.CutSuffix(inner, "]]")
	if !ok {
		return s
	}
	inner, _, _ = strings.Cut(inner, "|")
	inner, _, _ = strings.Cut(inner, "#")
	return strings.TrimSpace(inner)
}

// IsLink reports whether s is one wikilink.
func IsLink(s string) bool {
	s = strings.TrimSpace(s)
	return strings.HasPrefix(s, "[[") && strings.HasSuffix(s, "]]")
}
