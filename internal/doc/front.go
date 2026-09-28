package doc

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Front is a document's frontmatter, in the order its keys appear.
type Front struct {
	keys  []string
	nodes map[string]*yaml.Node
}

// ParseFront reads frontmatter text. Text that is empty or holds only comments is an
// empty Front.
func ParseFront(text string) (*Front, error) {
	f := &Front{nodes: map[string]*yaml.Node{}}
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(text), &root); err != nil {
		return nil, fmt.Errorf("frontmatter: %w", err)
	}
	if root.Kind == 0 || len(root.Content) == 0 {
		return f, nil
	}
	m := root.Content[0]
	if m.Kind == yaml.ScalarNode && m.Tag == "!!null" {
		return f, nil
	}
	if m.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("frontmatter: not a mapping")
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		k := m.Content[i].Value
		if _, dup := f.nodes[k]; !dup {
			f.keys = append(f.keys, k)
		}
		f.nodes[k] = m.Content[i+1]
	}
	return f, nil
}

// Keys lists the keys in order.
func (f *Front) Keys() []string { return slices.Clone(f.keys) }

// Has reports whether key is present.
func (f *Front) Has(key string) bool {
	_, ok := f.nodes[key]
	return ok
}

// Str is key's scalar value as written, or "" when it is absent, null, or not a scalar. A
// list of one item gives that item, since a user may write a single link either way.
func (f *Front) Str(key string) string {
	n := f.nodes[key]
	if n == nil {
		return ""
	}
	switch n.Kind {
	case yaml.ScalarNode:
		if n.Tag == "!!null" {
			return ""
		}
		return n.Value
	case yaml.SequenceNode:
		items := seqItems(n)
		if len(items) == 1 {
			return items[0]
		}
		if len(items) == 0 {
			return ""
		}
	}
	return ""
}

// List is key's items. A scalar is a list of one; an empty or absent key is nil.
func (f *Front) List(key string) []string {
	n := f.nodes[key]
	if n == nil {
		return nil
	}
	switch n.Kind {
	case yaml.ScalarNode:
		if n.Tag == "!!null" || strings.TrimSpace(n.Value) == "" {
			return nil
		}
		return []string{n.Value}
	case yaml.SequenceNode:
		return seqItems(n)
	}
	return nil
}

// seqItems reads a sequence of scalars. An item that is itself a one-item sequence is an
// unquoted wikilink, [[Title]], which YAML reads as a list in a list.
func seqItems(n *yaml.Node) []string {
	var out []string
	for _, item := range n.Content {
		switch item.Kind {
		case yaml.ScalarNode:
			if item.Tag != "!!null" && item.Value != "" {
				out = append(out, item.Value)
			}
		case yaml.SequenceNode:
			if len(item.Content) == 1 && item.Content[0].Kind == yaml.ScalarNode {
				out = append(out, "[["+item.Content[0].Value+"]]")
			}
		}
	}
	return out
}

// IsList reports whether key holds a list.
func (f *Front) IsList(key string) bool {
	n := f.nodes[key]
	return n != nil && n.Kind == yaml.SequenceNode
}

// Bool is key as a boolean; anything but true is false.
func (f *Front) Bool(key string) bool {
	v := strings.ToLower(f.Str(key))
	return v == "true" || v == "yes"
}

// Int is key as an integer, or 0.
func (f *Front) Int(key string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(f.Str(key)))
	return n
}

// Kind of a value, for comparison: a list compares as a list, anything else as its text.
func (f *Front) value(key string) any {
	n := f.nodes[key]
	if n == nil {
		return nil
	}
	if n.Kind == yaml.SequenceNode {
		return f.List(key)
	}
	return f.Str(key)
}

// Equal reports whether key already holds v, compared by meaning: a list by its items, a
// scalar by its text, so a list Obsidian wrote as a block equals the same list in flow
// style.
func (f *Front) Equal(key string, v any) bool {
	if f == nil {
		return false
	}
	have := f.value(key)
	switch want := v.(type) {
	case []string:
		got, _ := have.([]string)
		if have == nil && len(want) == 0 {
			return true
		}
		if s, ok := have.(string); ok {
			if s == "" {
				return len(want) == 0
			}
			got = []string{s}
		}
		return slices.Equal(got, want)
	case nil:
		return have == nil
	default:
		s, ok := have.(string)
		if !ok {
			return false
		}
		return s == scalarText(v)
	}
}

// Map is every key as a string or a list of strings, for code that reads a whole page.
func (f *Front) Map() map[string]any {
	out := make(map[string]any, len(f.keys))
	for _, k := range f.keys {
		out[k] = f.value(k)
	}
	return out
}

// scalarText is how a scalar value reads back after a write.
func scalarText(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case bool:
		return strconv.FormatBool(x)
	case int:
		return strconv.Itoa(x)
	case nil:
		return ""
	}
	return fmt.Sprint(v)
}

// FormatValue writes v as one line of YAML: a string plain when YAML reads it back as the
// same string, else double-quoted; a list in flow style.
func FormatValue(v any) string {
	switch x := v.(type) {
	case nil:
		return `""`
	case string:
		return formatScalar(x, false)
	case bool:
		return strconv.FormatBool(x)
	case int:
		return strconv.Itoa(x)
	case []string:
		items := make([]string, len(x))
		for i, s := range x {
			items[i] = formatScalar(s, true)
		}
		return "[" + strings.Join(items, ", ") + "]"
	}
	return formatScalar(fmt.Sprint(v), false)
}

var (
	dateLike   = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}([T ]\d{2}:\d{2}(:\d{2})?)?$`)
	reserved   = map[string]bool{"true": true, "false": true, "yes": true, "no": true, "on": true, "off": true, "null": true, "~": true, "y": true, "n": true}
	plainStart = "-?:,[]{}#&*!|>'\"%@`"
)

func formatScalar(s string, inFlow bool) string {
	if s == "" {
		return `""`
	}
	if dateLike.MatchString(s) {
		return s
	}
	if plainSafe(s, inFlow) {
		return s
	}
	return strconv.Quote(s)
}

// plainSafe reports whether YAML reads s back unquoted as the same string.
func plainSafe(s string, inFlow bool) bool {
	if s != strings.TrimSpace(s) || strings.ContainsAny(s, "\n\r\t") {
		return false
	}
	if strings.ContainsRune(plainStart, rune(s[0])) {
		return false
	}
	if strings.Contains(s, ": ") || strings.HasSuffix(s, ":") || strings.Contains(s, " #") {
		return false
	}
	if inFlow && strings.ContainsAny(s, ",[]{}") {
		return false
	}
	if reserved[strings.ToLower(s)] {
		return false
	}
	if _, err := strconv.ParseFloat(s, 64); err == nil {
		return false
	}
	if lower := strings.ToLower(s); strings.HasPrefix(lower, "0x") || strings.HasPrefix(lower, "0o") || lower == ".inf" || lower == ".nan" || lower == "-.inf" {
		return false
	}
	return true
}

// frontKey matches a top-level key line.
var frontKey = regexp.MustCompile(`^([A-Za-z0-9_][A-Za-z0-9_ .-]*?)\s*:(?:\s|$)`)

// SetField sets key to v in content's frontmatter and returns the new content. It replaces
// the key's lines, a block list included, and keeps every other line as written; a new
// key goes at the end. When the key already holds v, content comes back unchanged. A
// document with no frontmatter gets one.
func SetField(content, key string, v any) string {
	front, body, ok := Split(content)
	if !ok {
		return Join(key+": "+FormatValue(v)+"\n", body)
	}
	if f, err := ParseFront(front); err == nil && f.Has(key) && f.Equal(key, v) {
		return content
	}
	lines := strings.Split(strings.TrimRight(front, "\n"), "\n")
	if front == "" {
		lines = nil
	}
	line := key + ": " + FormatValue(v)
	start, end := keyLines(lines, key)
	if start < 0 {
		lines = append(lines, line)
	} else {
		lines = append(lines[:start], append([]string{line}, lines[end:]...)...)
	}
	return Join(strings.Join(lines, "\n")+"\n", body)
}

// SetFields sets each field in order.
func SetFields(content string, fields []Field) string {
	for _, f := range fields {
		content = SetField(content, f.Key, f.Value)
	}
	return content
}

// RemoveField drops key and its lines from the frontmatter.
func RemoveField(content, key string) string {
	front, body, ok := Split(content)
	if !ok {
		return content
	}
	lines := strings.Split(strings.TrimRight(front, "\n"), "\n")
	start, end := keyLines(lines, key)
	if start < 0 {
		return content
	}
	lines = append(lines[:start], lines[end:]...)
	text := strings.Join(lines, "\n")
	if text != "" {
		text += "\n"
	}
	return Join(text, body)
}

// keyLines finds the lines a top-level key spans: its own line and every line after it
// that is indented, a block list item, or blank inside the block. It returns -1 when the
// key is absent.
func keyLines(lines []string, key string) (int, int) {
	for i, l := range lines {
		m := frontKey.FindStringSubmatch(l)
		if m == nil || m[1] != key {
			continue
		}
		end := i + 1
		for end < len(lines) {
			next := lines[end]
			if next == "" || next[0] == ' ' || next[0] == '\t' || strings.HasPrefix(next, "- ") || next == "-" {
				end++
				continue
			}
			break
		}
		// Blank lines at the end of the block belong to what follows.
		for end > i+1 && strings.TrimSpace(lines[end-1]) == "" {
			end--
		}
		return i, end
	}
	return -1, -1
}
