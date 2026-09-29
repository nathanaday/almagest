// Package schema holds the thirteen document types: each type's id prefix, folder,
// fields, the owner of each field, and the sections of its body. Every check of a
// document against its type reads these tables.
package schema

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
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
	Bool
	Int
	Date // 2006-01-02
	Time // 2006-01-02T15:04:05
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
	Scope     Family = "scope"
	Knowledge Family = "knowledge"
	Thread    Family = "thread"
	Record    Family = "record"
)

// Type is one document type.
type Type struct {
	Name   string
	Prefix string
	// Folder is where the type's documents live, relative to the vault. A page of the wiki
	// lives in the folder of its scope under it (vault.Route); a thread document in its
	// thread's folder; a session or change in a month folder.
	Folder   string
	Family   Family
	Fields   []Field
	Sections []string
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

// Wiki reports whether the type is a page of the wiki: a scope page other than the vault,
// or a knowledge page.
func (t *Type) Wiki() bool {
	return t.Name != "vault" && (t.Family == Scope || t.Family == Knowledge)
}

var (
	scopeTargets  = []string{"area", "repository"}
	statusValues  = []string{"stable", "draft", "contested", "deprecated"}
	priorityValue = []string{"high", "normal", "low", "someday"}
)

func common(extra ...Field) []Field {
	return append([]Field{
		{Name: "id", Kind: Text, Owner: Code, Required: true},
		{Name: "type", Kind: Text, Owner: Code, Required: true},
		{Name: "created", Kind: Date, Owner: Code, Required: true},
		{Name: "updated", Kind: Text, Owner: Code, Required: true},
	}, extra...)
}

// knowledge are the fields every knowledge page has, then the type's own.
func knowledge(extra ...Field) []Field {
	return common(append([]Field{
		{Name: "scope", Kind: Link, Targets: scopeTargets},
		{Name: "description", Kind: Text, Required: true},
		{Name: "aliases", Kind: List},
		{Name: "tags", Kind: List},
		{Name: "status", Kind: Enum, Values: statusValues},
		{Name: "sources", Kind: Links},
		{Name: "chain", Kind: Links, Owner: Code, Targets: scopeTargets},
	}, extra...)...)
}

func threadDoc(extra ...Field) []Field {
	return common(append([]Field{
		{Name: "thread", Kind: Link, Owner: Code, Required: true, Targets: []string{"stub"}},
		{Name: "thread_id", Kind: Text, Owner: Code, Required: true},
	}, extra...)...)
}

// Types are the thirteen document types, in the order the design lists them.
var Types = []*Type{
	{Name: "vault", Prefix: "vlt", Folder: "", Family: Scope, Fields: common(
		Field{Name: "name", Kind: Text, Required: true},
		Field{Name: "description", Kind: Text},
		Field{Name: "areas", Kind: Enum, Values: []string{"many", "few", "manual"}},
		Field{Name: "wikify", Kind: List},
		Field{Name: "stale_hours", Kind: Int},
		Field{Name: "layout", Kind: Int, Owner: Code},
	)},
	{Name: "area", Prefix: "are", Folder: "wiki", Family: Scope, Fields: common(
		Field{Name: "parent", Kind: Link, Targets: []string{"area"}},
		Field{Name: "description", Kind: Text, Required: true},
		Field{Name: "aliases", Kind: List},
		Field{Name: "chain", Kind: Links, Owner: Code, Targets: []string{"area"}},
	)},
	{Name: "repository", Prefix: "rep", Folder: "wiki", Family: Scope, Fields: common(
		Field{Name: "parent", Kind: Link, Targets: []string{"area"}},
		Field{Name: "description", Kind: Text, Required: true},
		Field{Name: "aliases", Kind: List},
		Field{Name: "path", Kind: Text, Required: true},
		Field{Name: "remote", Kind: Text, Owner: Code},
		Field{Name: "branch", Kind: Text, Owner: Code},
		Field{Name: "described", Kind: Text, Owner: Code},
		Field{Name: "chain", Kind: Links, Owner: Code, Targets: []string{"area"}},
	), Sections: []string{"What it is", "How it is built", "Layout", "Components", "Instructions"}},
	{Name: "concept", Prefix: "con", Folder: "wiki", Family: Knowledge, Fields: knowledge(),
		Sections: []string{"Definition", "Explanation", "Related", "Sources"}},
	{Name: "entity", Prefix: "ent", Folder: "wiki", Family: Knowledge, Fields: knowledge(
		Field{Name: "kind", Kind: Enum, Values: []string{"person", "organization", "tool", "component", "service", "dataset", "document", "other"}},
	), Sections: []string{"What it is", "Facts", "Related", "Sources"}},
	{Name: "policy", Prefix: "pol", Folder: "wiki", Family: Knowledge, Fields: knowledge(
		Field{Name: "strength", Kind: Enum, Values: []string{"must", "should", "may"}},
	), Sections: []string{"Rule", "Why", "Applies when", "Exceptions", "Sources"}},
	{Name: "source", Prefix: "src", Folder: "wiki", Family: Knowledge, Fields: knowledge(
		Field{Name: "file", Kind: Text, Owner: Code, Required: true},
		Field{Name: "sha256", Kind: Text, Owner: Code, Required: true},
		Field{Name: "origin", Kind: Enum, Owner: Code, Values: []string{"inbox", "pasted", "url", "repository"}},
		Field{Name: "locator", Kind: Text, Owner: Code},
		Field{Name: "measure", Kind: Text, Owner: Code},
		Field{Name: "captured", Kind: Date, Owner: Code},
		Field{Name: "authority", Kind: Enum, Values: []string{"official", "primary", "secondary", "community", "synthetic", "unknown"}},
	), Sections: []string{"Summary", "Structure"}},
	{Name: "stub", Prefix: "thr", Folder: "threads", Family: Thread, Fields: common(
		Field{Name: "scope", Kind: Links, Targets: scopeTargets},
		Field{Name: "priority", Kind: Enum, Values: priorityValue},
		Field{Name: "blocked", Kind: Text},
		Field{Name: "stage", Kind: Enum, Owner: Code, Values: []string{"stub", "spec", "tasks", "closed"}},
		Field{Name: "outcome", Kind: Enum, Owner: Code, Values: []string{"completed", "killed"}},
		Field{Name: "active", Kind: Bool, Owner: Code},
		Field{Name: "tasks", Kind: Text, Owner: Code},
		Field{Name: "chain", Kind: Links, Owner: Code, Targets: scopeTargets},
	), Sections: []string{"Stub", "Notes"}},
	{Name: "spec", Prefix: "spc", Folder: "threads", Family: Thread, Fields: threadDoc(),
		Sections: []string{"Goal", "Done when", "Decisions", "Out of scope", "Conventions", "Open questions"}},
	{Name: "task", Prefix: "tsk", Folder: "threads", Family: Thread, Fields: threadDoc(
		Field{Name: "order", Kind: Int, Required: true},
		Field{Name: "repository", Kind: Link, Targets: []string{"repository"}},
		Field{Name: "depends", Kind: Links, Targets: []string{"task"}},
		Field{Name: "status", Kind: Enum, Required: true, Values: []string{"open", "done", "dropped"}},
		Field{Name: "blocked", Kind: Text},
		Field{Name: "active", Kind: Bool, Owner: Code},
	), Sections: []string{"What", "Where", "Conventions", "Verify", "Progress", "Result"}},
	{Name: "receipt", Prefix: "rcp", Folder: "threads", Family: Thread, Fields: threadDoc(
		Field{Name: "outcome", Kind: Enum, Required: true, Values: []string{"completed", "killed"}},
		Field{Name: "superseded", Kind: Bool, Owner: Code},
	), Sections: []string{"Delivered", "Verified", "Follow-ups", "Learned"}},
	{Name: "session", Prefix: "ses", Folder: "sessions", Family: Record, Fields: common(
		Field{Name: "harness", Kind: Enum, Owner: Code, Values: []string{"claude", "codex"}},
		Field{Name: "harness_id", Kind: Text, Owner: Code, Required: true},
		Field{Name: "status", Kind: Enum, Owner: Code, Required: true, Values: []string{"running", "waiting", "idle", "ended", "lost"}},
		Field{Name: "started", Kind: Time, Owner: Code},
		Field{Name: "ended", Kind: Time, Owner: Code},
		Field{Name: "last_prompt", Kind: Time, Owner: Code},
		Field{Name: "cwd", Kind: Text, Owner: Code},
		Field{Name: "parent", Kind: Link, Owner: Code, Targets: []string{"session"}},
		Field{Name: "agent", Kind: Text, Owner: Code},
		Field{Name: "threads", Kind: Links, Owner: Code, Targets: []string{"stub"}},
		Field{Name: "tasks", Kind: Links, Owner: Code, Targets: []string{"task"}},
		Field{Name: "repositories", Kind: Links, Owner: Code, Targets: []string{"repository"}},
		Field{Name: "changes", Kind: Links, Owner: Code, Targets: []string{"change"}},
		Field{Name: "description", Kind: Text, Owner: Code},
		Field{Name: "reminded", Kind: List, Owner: Code},
	), Sections: []string{"Description", "Progress", "Summary", "Subagents"}},
	{Name: "change", Prefix: "chg", Folder: "changes", Family: Record, Fields: common(
		Field{Name: "status", Kind: Enum, Owner: Code, Required: true, Values: []string{"proposed", "applying", "applied", "rejected", "superseded", "undone"}},
		Field{Name: "absorbs", Kind: Links, Owner: Code},
		Field{Name: "thread", Kind: Link, Owner: Code, Targets: []string{"stub"}},
		Field{Name: "proposed", Kind: Time, Owner: Code},
		Field{Name: "session", Kind: Link, Owner: Code, Targets: []string{"session"}},
		Field{Name: "counts", Kind: Text, Owner: Code},
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

var byPrefix = func() map[string]*Type {
	m := map[string]*Type{}
	for _, t := range Types {
		m[t.Prefix] = t
	}
	return m
}()

// Get returns the type of that name, or nil.
func Get(name string) *Type { return byName[name] }

// ByID returns the type an id's prefix names, or nil.
func ByID(id string) *Type {
	prefix, _, ok := strings.Cut(id, "-")
	if !ok {
		return nil
	}
	return byPrefix[prefix]
}

// Names lists every type name.
func Names() []string {
	out := make([]string, len(Types))
	for i, t := range Types {
		out[i] = t.Name
	}
	return out
}

// Is reports whether name is a document type.
func Is(name string) bool { return byName[name] != nil }

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
	if id := v.str("id"); id != "" && t.Name != "session" && !strings.HasPrefix(id, t.Prefix+"-") {
		add("id", "%s does not start with %s-", id, t.Prefix)
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
