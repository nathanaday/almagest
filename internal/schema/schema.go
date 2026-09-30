// Package schema holds the document types: the six of wiki/documents (source, repository,
// topic, stub, spec, event), the vault document, and the two records (session, change).
// For each: its fields, the owner of each field, its kinds, and the sections of its body.
// Every check of a document against its type reads these tables.
package schema

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/tags"
)

// Owner says who may write a field.
type Owner int

const (
	// Model fields are the model's to give, through the tool that writes the type.
	Model Owner = iota
	// Code fields are derived or minted; the model never writes them.
	Code
)

// Kind is the form of a field's value.
type Kind int

const (
	Text  Kind = iota // one line of text
	Enum              // one of Values
	Link              // one wikilink to a document of a type in Targets, or empty
	Links             // a list of wikilinks
	List              // a list of plain strings
	Tags              // a list of tags
	Tag               // one tag
	Bool
	Int
	Time // 2006-01-02T15:04:05; a date alone is read too
)

// Field is one frontmatter field of a type.
type Field struct {
	Name     string
	Kind     Kind
	Owner    Owner
	Required bool
	// Values are the allowed values of an Enum.
	Values []string
	// Targets are the types a Link or Links may name; empty means any document.
	Targets []string
}

// Family groups the types.
type Family string

const (
	// Root is the vault document, Atlas.md.
	Root Family = "root"
	// Knowledge is what the vault knows: only a change writes it.
	Knowledge Family = "knowledge"
	// Work is what the user means to do: the work tool writes it.
	Work Family = "work"
	// Record is what happened: code writes it.
	Record Family = "record"
)

// Type is one document type.
type Type struct {
	Name   string
	Prefix string
	Family Family
	// Folder is where the type's documents live: wiki/documents for the six, a month
	// folder of sessions/ or changes/ for the records, the root for the vault.
	Folder string
	Fields []Field
	// Kinds are the values of the kind field, for a type that has one.
	Kinds []string
	// Sections are the body's sections in order; KindSections replace them for a kind.
	Sections     []string
	KindSections map[string][]string
	// CodeSections are the sections code writes; the model never edits them.
	CodeSections []string
}

// Field returns the named field of the type, or nil.
func (t *Type) Field(name string) *Field {
	for i := range t.Fields {
		if t.Fields[i].Name == name {
			return &t.Fields[i]
		}
	}
	return nil
}

// Owned reports whether code owns the named field. A field the type does not name is
// the user's, never code's.
func (t *Type) Owned(name string) bool {
	f := t.Field(name)
	return f != nil && f.Owner == Code
}

// SectionsOf are the sections of a document of this type and kind, in order.
func (t *Type) SectionsOf(kind string) []string {
	if s, ok := t.KindSections[kind]; ok {
		return s
	}
	return t.Sections
}

// Document reports whether the type lives in wiki/documents.
func (t *Type) Document() bool {
	return t.Family == Knowledge || t.Family == Work || t.Name == "event"
}

// The prefix of every new document of wiki/documents. A document from a 6.x vault keeps
// its old prefix, since an id never changes.
const DocPrefix = "doc"

// Kinds of the types that have one.
var (
	TopicKinds = []string{"concept", "entity", "policy", "overview"}
	SpecKinds  = []string{"plan", "design"}
	EventKinds = []string{"started", "continued", "completed", "dropped", "reopened", "blocked", "unblocked", "promoted", "resolved", "note"}
	Priorities = []string{"high", "normal", "low", "someday"}
)

var topicStatus = []string{"draft", "stable", "contested", "deprecated"}

// common are the fields every document of wiki/documents has, then the type's own.
func common(extra ...Field) []Field {
	return append([]Field{
		{Name: "id", Kind: Text, Owner: Code, Required: true},
		{Name: "type", Kind: Text, Owner: Code, Required: true},
		{Name: "description", Kind: Text, Required: true},
		{Name: "tags", Kind: Tags},
		{Name: "aliases", Kind: List},
		{Name: "created", Kind: Time, Owner: Code, Required: true},
		{Name: "updated", Kind: Time, Owner: Code, Required: true},
		{Name: "refreshed", Kind: Time, Owner: Code},
	}, extra...)
}

// record are the fields of a session or a change.
func record(extra ...Field) []Field {
	return append([]Field{
		{Name: "id", Kind: Text, Owner: Code, Required: true},
		{Name: "type", Kind: Text, Owner: Code, Required: true},
		{Name: "created", Kind: Time, Owner: Code, Required: true},
		{Name: "updated", Kind: Time, Owner: Code, Required: true},
	}, extra...)
}

// Types are the document types.
var Types = []*Type{
	{Name: "vault", Prefix: "vlt", Family: Root, Fields: []Field{
		{Name: "id", Kind: Text, Owner: Code, Required: true},
		{Name: "type", Kind: Text, Owner: Code, Required: true},
		{Name: "name", Kind: Text, Required: true},
		{Name: "description", Kind: Text},
		{Name: "created", Kind: Time, Owner: Code, Required: true},
		{Name: "updated", Kind: Time, Owner: Code, Required: true},
		{Name: "tagging", Kind: Enum, Values: []string{"open", "known"}},
		{Name: "wikify", Kind: List},
		{Name: "stale_hours", Kind: Int},
		{Name: "layout", Kind: Int, Owner: Code},
	}},
	{Name: "source", Prefix: DocPrefix, Family: Knowledge, Folder: "wiki/documents", Fields: common(
		Field{Name: "authority", Kind: Enum, Values: []string{"official", "primary", "secondary", "community", "synthetic", "unknown"}},
		Field{Name: "authors", Kind: List},
		Field{Name: "published", Kind: Text},
		Field{Name: "status", Kind: Enum, Owner: Code, Values: []string{"pending", "absorbed"}},
		Field{Name: "file", Kind: Text, Owner: Code, Required: true},
		Field{Name: "media", Kind: Enum, Owner: Code, Values: []string{"pdf", "image", "markdown", "text", "office", "audio", "video", "other"}},
		Field{Name: "sha256", Kind: Text, Owner: Code, Required: true},
		Field{Name: "origin", Kind: Enum, Owner: Code, Values: []string{"inbox", "pasted", "url", "repository"}},
		Field{Name: "locator", Kind: Text, Owner: Code},
		Field{Name: "measure", Kind: Text, Owner: Code},
		Field{Name: "captured", Kind: Time, Owner: Code},
		Field{Name: "from", Kind: Link, Owner: Code, Targets: []string{"stub"}},
	), Sections: []string{"Summary", "Structure", "Notes"}},
	{Name: "repository", Prefix: DocPrefix, Family: Knowledge, Folder: "wiki/documents", Fields: common(
		Field{Name: "defines", Kind: Tag},
		Field{Name: "path", Kind: Text},
		Field{Name: "unlinked", Kind: Bool},
		Field{Name: "remote", Kind: Text, Owner: Code},
		Field{Name: "branch", Kind: Text, Owner: Code},
		Field{Name: "head", Kind: Text, Owner: Code},
		Field{Name: "head_time", Kind: Time, Owner: Code},
		Field{Name: "described", Kind: Text, Owner: Code},
		Field{Name: "behind", Kind: Int, Owner: Code},
	), Sections: []string{"What it is", "How it is built", "Layout", "Components", "Instructions", "Work", "Knowledge", "Notes"},
		CodeSections: []string{"Work", "Knowledge"}},
	{Name: "topic", Prefix: DocPrefix, Family: Knowledge, Folder: "wiki/documents", Fields: common(
		Field{Name: "kind", Kind: Enum, Required: true, Values: TopicKinds},
		Field{Name: "status", Kind: Enum, Values: topicStatus},
		Field{Name: "sources", Kind: Links},
		Field{Name: "strength", Kind: Enum, Values: []string{"must", "should", "may"}},
		Field{Name: "defines", Kind: Tag},
		Field{Name: "from", Kind: Link, Targets: []string{"stub"}},
	), Kinds: TopicKinds, KindSections: map[string][]string{
		"concept":  {"Definition", "Explanation", "Related", "Sources", "Origin", "Notes"},
		"entity":   {"What it is", "Facts", "Related", "Sources", "Origin", "Notes"},
		"policy":   {"Rule", "Why", "Applies when", "Exceptions", "Sources", "Origin", "Notes"},
		"overview": {"Summary", "Context", "Map", "Related", "Sources", "Origin", "Notes"},
	}, CodeSections: []string{"Map", "Origin"}},
	{Name: "stub", Prefix: DocPrefix, Family: Work, Folder: "wiki/documents", Fields: common(
		Field{Name: "priority", Kind: Enum, Values: Priorities},
		Field{Name: "status", Kind: Enum, Owner: Code, Values: []string{"open", "resolved", "dropped"}},
		Field{Name: "became", Kind: Links, Owner: Code},
	), Sections: []string{"Idea", "Notes"}},
	{Name: "spec", Prefix: DocPrefix, Family: Work, Folder: "wiki/documents", Fields: common(
		Field{Name: "kind", Kind: Enum, Required: true, Values: SpecKinds},
		Field{Name: "parent", Kind: Link, Targets: []string{"spec"}},
		Field{Name: "repositories", Kind: Links, Targets: []string{"repository"}},
		Field{Name: "depends", Kind: Links, Targets: []string{"spec"}},
		Field{Name: "order", Kind: Int},
		Field{Name: "priority", Kind: Enum, Values: Priorities},
		Field{Name: "implements", Kind: Links, Targets: []string{"spec"}},
		Field{Name: "supersedes", Kind: Link, Targets: []string{"spec"}},
		Field{Name: "from", Kind: Link, Targets: []string{"stub"}},
		Field{Name: "status", Kind: Enum, Owner: Code, Values: []string{"open", "started", "done", "dropped", "current", "superseded"}},
		Field{Name: "blocked", Kind: Text, Owner: Code},
		Field{Name: "active", Kind: Bool, Owner: Code},
		Field{Name: "parts", Kind: Text, Owner: Code},
		Field{Name: "root", Kind: Link, Owner: Code, Targets: []string{"spec"}},
	), Kinds: SpecKinds, KindSections: map[string][]string{
		"plan":   {"Goal", "Done when", "Decisions", "Out of scope", "Conventions", "Where", "Verify", "Parts", "Progress", "History", "Open questions", "Origin", "Notes"},
		"design": {"Purpose", "Behavior", "Interfaces", "Constraints", "Decisions", "Open questions", "Implemented by", "Origin", "Notes"},
	}, CodeSections: []string{"Parts", "History", "Implemented by", "Origin"}},
	{Name: "event", Prefix: DocPrefix, Family: Record, Folder: "wiki/documents", Fields: common(
		Field{Name: "kind", Kind: Enum, Owner: Code, Required: true, Values: EventKinds},
		Field{Name: "at", Kind: Time, Owner: Code, Required: true},
		Field{Name: "subject", Kind: Link, Owner: Code, Required: true},
		Field{Name: "subject_id", Kind: Text, Owner: Code},
		Field{Name: "session", Kind: Link, Owner: Code, Targets: []string{"session"}},
		Field{Name: "by", Kind: Enum, Owner: Code, Values: []string{"agent", "user"}},
		Field{Name: "from_type", Kind: Text, Owner: Code},
		Field{Name: "to_type", Kind: Text, Owner: Code},
		Field{Name: "became", Kind: Links, Owner: Code},
		Field{Name: "change", Kind: Link, Owner: Code, Targets: []string{"change"}},
	), Kinds: EventKinds, KindSections: map[string][]string{
		"completed": {"Delivered", "Verified", "Follow-ups", "Learned", "Notes"},
		"dropped":   {"Why", "Follow-ups", "Notes"},
		"reopened":  {"Why", "Notes"},
		"note":      {"Note", "Notes"},
	}, Sections: []string{"Notes"}},
	{Name: "session", Prefix: "ses", Family: Record, Folder: "sessions", Fields: record(
		Field{Name: "harness", Kind: Enum, Owner: Code, Values: []string{"claude", "codex"}},
		Field{Name: "harness_id", Kind: Text, Owner: Code, Required: true},
		Field{Name: "status", Kind: Enum, Owner: Code, Required: true, Values: []string{"running", "waiting", "idle", "ended", "lost"}},
		Field{Name: "started", Kind: Time, Owner: Code},
		Field{Name: "ended", Kind: Time, Owner: Code},
		Field{Name: "last_prompt", Kind: Time, Owner: Code},
		Field{Name: "cwd", Kind: Text, Owner: Code},
		Field{Name: "parent", Kind: Link, Owner: Code, Targets: []string{"session"}},
		Field{Name: "agent", Kind: Text, Owner: Code},
		Field{Name: "specs", Kind: Links, Owner: Code, Targets: []string{"spec"}},
		Field{Name: "work", Kind: Links, Owner: Code, Targets: []string{"stub", "spec"}},
		Field{Name: "repositories", Kind: Links, Owner: Code, Targets: []string{"repository"}},
		Field{Name: "changes", Kind: Links, Owner: Code, Targets: []string{"change"}},
		Field{Name: "events", Kind: Int, Owner: Code},
		Field{Name: "description", Kind: Text, Owner: Code},
		Field{Name: "reminded", Kind: List, Owner: Code},
	), Sections: []string{"Description", "Progress", "Summary", "Subagents"}},
	{Name: "change", Prefix: "chg", Family: Record, Folder: "changes", Fields: record(
		Field{Name: "status", Kind: Enum, Owner: Code, Required: true, Values: []string{"proposed", "applying", "applied", "rejected", "superseded", "undone"}},
		Field{Name: "absorbs", Kind: Links, Owner: Code},
		Field{Name: "work", Kind: Link, Owner: Code, Targets: []string{"stub", "spec"}},
		Field{Name: "proposed", Kind: Time, Owner: Code},
		Field{Name: "session", Kind: Link, Owner: Code, Targets: []string{"session"}},
		Field{Name: "counts", Kind: Text, Owner: Code},
		Field{Name: "new_tags", Kind: List, Owner: Code},
		Field{Name: "applied", Kind: Time, Owner: Code},
		Field{Name: "supersedes", Kind: Link, Owner: Code, Targets: []string{"change"}},
		Field{Name: "reason", Kind: Text, Owner: Code},
		Field{Name: "paths", Kind: List, Owner: Code},
	), Sections: []string{"Notes", "Absorbed", "Writes"}},
}

var byName = func() map[string]*Type {
	m := map[string]*Type{}
	for _, t := range Types {
		m[t.Name] = t
	}
	return m
}()

// Get returns the type of that name, or nil.
func Get(name string) *Type { return byName[name] }

// Names lists every type name.
func Names() []string {
	out := make([]string, len(Types))
	for i, t := range Types {
		out[i] = t.Name
	}
	return out
}

// DocumentTypes are the six types of wiki/documents.
var DocumentTypes = []string{"source", "repository", "topic", "stub", "spec", "event"}

// Is reports whether name is a document type.
func Is(name string) bool { return byName[name] != nil }

// IsDocument reports whether name is one of the six types of wiki/documents.
func IsDocument(name string) bool { return slices.Contains(DocumentTypes, name) }

// KindsOf lists the kinds of a type, or nil when it has none.
func KindsOf(name string) []string {
	if t := byName[name]; t != nil {
		return t.Kinds
	}
	return nil
}

// AllKinds lists every kind of every type.
func AllKinds() []string {
	var out []string
	for _, t := range Types {
		for _, k := range t.Kinds {
			if !slices.Contains(out, k) {
				out = append(out, k)
			}
		}
	}
	return out
}

// Statuses lists every status any type has.
func Statuses() []string {
	var out []string
	for _, t := range Types {
		if f := t.Field("status"); f != nil {
			for _, v := range f.Values {
				if !slices.Contains(out, v) {
					out = append(out, v)
				}
			}
		}
	}
	return out
}

// IDPattern matches an id: three letters, a hyphen, and six characters.
var IDPattern = regexp.MustCompile(`^[a-z]{3}-[0-9a-z]{6}$`)

// Problem is one way a document breaks its type's schema.
type Problem struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (p Problem) String() string { return p.Field + ": " + p.Message }

// Resolver answers what a link names, for the checks that need the vault.
type Resolver interface {
	// TypeOfLink is the type of the one document a link or title names, "" when it
	// names none, and an error when it names two.
	TypeOfLink(target string) (string, error)
}

// Values is a document's fields as Front.Map gives them: each a string or a list.
type Values map[string]any

func (v Values) str(k string) string {
	switch x := v[k].(type) {
	case string:
		return x
	case []string:
		if len(x) == 1 {
			return x[0]
		}
	}
	return ""
}

func (v Values) list(k string) []string {
	switch x := v[k].(type) {
	case []string:
		return x
	case string:
		if strings.TrimSpace(x) != "" {
			return []string{x}
		}
	}
	return nil
}

// ParseTime reads a time as code writes it, or a date, or a time without seconds.
func ParseTime(s string) (time.Time, bool) {
	for _, layout := range []string{"2006-01-02T15:04:05", "2006-01-02T15:04", "2006-01-02", time.RFC3339} {
		if t, err := time.ParseInLocation(layout, strings.TrimSpace(s), time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// Check reads a document's values against its type. With a resolver it also checks
// that each link names one document of a type the field allows.
func (t *Type) Check(v Values, r Resolver) []Problem {
	var out []Problem
	add := func(field, format string, args ...any) {
		out = append(out, Problem{Field: field, Message: fmt.Sprintf(format, args...)})
	}
	if got := v.str("type"); got != t.Name {
		add("type", "is %q, not %q", got, t.Name)
	}
	if id := v.str("id"); id != "" {
		switch {
		case !IDPattern.MatchString(id):
			add("id", "%s is not an id: three letters, a hyphen, and six characters", id)
		case (t.Name == "session" || t.Name == "change" || t.Name == "vault") && !strings.HasPrefix(id, t.Prefix+"-"):
			add("id", "%s does not start with %s-", id, t.Prefix)
		}
	}
	for _, f := range t.Fields {
		raw, present := v[f.Name]
		if !present || empty(raw) {
			if f.Required {
				add(f.Name, "is required")
			}
			continue
		}
		switch f.Kind {
		case Enum:
			if s := v.str(f.Name); !slices.Contains(f.Values, s) {
				add(f.Name, "is %q; allowed: %s", s, strings.Join(f.Values, ", "))
			}
		case Int:
			if _, err := strconv.Atoi(v.str(f.Name)); err != nil {
				add(f.Name, "is %q, not a whole number", v.str(f.Name))
			}
		case Bool:
			if s := strings.ToLower(v.str(f.Name)); s != "true" && s != "false" {
				add(f.Name, "is %q, not true or false", s)
			}
		case Time:
			if _, ok := ParseTime(v.str(f.Name)); !ok {
				add(f.Name, "is %q, not a time like 2026-09-29T14:32:05", v.str(f.Name))
			}
		case Tag:
			if s := v.str(f.Name); !tags.Valid(s) {
				add(f.Name, "%q is no valid tag", s)
			}
		case Tags:
			for _, s := range v.list(f.Name) {
				if !tags.Valid(s) {
					add(f.Name, "%q is no valid tag", s)
				}
			}
		case Link:
			if _, isList := raw.([]string); isList && len(v.list(f.Name)) > 1 {
				add(f.Name, "holds %d links; it takes one", len(v.list(f.Name)))
				continue
			}
			out = append(out, t.checkLink(f, v.str(f.Name), r)...)
		case Links:
			for _, l := range v.list(f.Name) {
				out = append(out, t.checkLink(f, l, r)...)
			}
		}
	}
	return out
}

func (t *Type) checkLink(f Field, target string, r Resolver) []Problem {
	if r == nil || strings.TrimSpace(target) == "" {
		return nil
	}
	got, err := r.TypeOfLink(target)
	if err != nil {
		return []Problem{{Field: f.Name, Message: err.Error()}}
	}
	if got == "" {
		return []Problem{{Field: f.Name, Message: fmt.Sprintf("%s names no document", target)}}
	}
	if len(f.Targets) > 0 && !slices.Contains(f.Targets, got) {
		return []Problem{{Field: f.Name, Message: fmt.Sprintf("%s is a %s; it must be a %s", target, got, strings.Join(f.Targets, " or "))}}
	}
	return nil
}

func empty(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(x) == ""
	case []string:
		return len(x) == 0
	}
	return false
}
