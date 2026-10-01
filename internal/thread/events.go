package thread

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/schema"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

// Who acts: the agent through a tool, or the user through the CLI or Obsidian.
const (
	ByAgent = "agent"
	ByUser  = "user"
)

// legacySections are the sections of a 7.0 completed event.
var legacySections = []string{"Delivered", "Verified", "Follow-ups", "Learned"}

// EventIn is one event to write.
type EventIn struct {
	Kind string
	// Subject is the document as the write leaves it: its title, id, and tags.
	SubjectTitle string
	SubjectID    string
	SubjectTags  []string
	At           time.Time
	By           string
	// Prose are the texts of the kind's sections, by section title.
	Prose map[string]string
	// Line is a blocked event's one line.
	Line     string
	FromType string
	ToType   string
	Became   []string // titles
	Change   string   // the change document's title, for a promotion through a change
}

// Titles tracks the titles a write takes, so two new documents of one write never share
// one, and none takes a title the vault holds.
type Titles struct {
	idx   *vault.Index
	taken map[string]bool
}

// NewTitles starts the titles of one write.
func NewTitles(idx *vault.Index) *Titles { return &Titles{idx: idx, taken: map[string]bool{}} }

// Free reports whether a title is free in the vault and in this write.
func (t *Titles) Free(title string) bool {
	return !t.taken[strings.ToLower(title)] && len(t.idx.TitleHolders(title)) == 0
}

// Take claims a title.
func (t *Titles) Take(title string) { t.taken[strings.ToLower(title)] = true }

// Release gives a title back, when the write renames away from it.
func (t *Titles) Release(title string) { delete(t.taken, strings.ToLower(title)) }

// EventTitle is the title of a new event: the subject's title, the kind, and the minute;
// the seconds when the minute is taken; then a number.
func EventTitle(t *Titles, subject, kind string, at time.Time) string {
	base := subject + " · " + kind + " "
	for _, candidate := range []string{base + at.Format("2006-01-02 1504"), base + at.Format("2006-01-02 150405")} {
		if c := doc.CleanTitle(candidate); t.Free(c) {
			return c
		}
	}
	for n := 2; ; n++ {
		if c := doc.CleanTitle(fmt.Sprintf("%s%s (%d)", base, at.Format("2006-01-02 150405"), n)); t.Free(c) {
			return c
		}
	}
}

// Describe is an event's description: the kind and the subject, or a block's line.
func Describe(kind, subject, line string) string {
	if kind == "blocked" && line != "" {
		return oneLine("Blocked: "+line, 200)
	}
	return oneLine(capital(kind)+": "+subject, 200)
}

// NewEvent renders a new event. It returns the event's path, content, and id, and
// claims its title.
func NewEvent(t *Titles, in EventIn) (rel, content, id string) {
	title := EventTitle(t, in.SubjectTitle, in.Kind, in.At)
	t.Take(title)
	id = doc.NewID(schema.DocPrefix, func(s string) bool { return t.idx.ByID(s) != nil })
	by := in.By
	if by == "" {
		by = ByAgent
	}
	stamp := vault.Stamp(in.At)
	fields := []doc.Field{
		{Key: "id", Value: id},
		{Key: "type", Value: "event"},
		{Key: "kind", Value: in.Kind},
		{Key: "description", Value: Describe(in.Kind, in.SubjectTitle, in.Line)},
		{Key: "tags", Value: nonNil(in.SubjectTags)},
		{Key: "aliases", Value: []string{}},
		{Key: "created", Value: stamp},
		{Key: "updated", Value: stamp},
		{Key: "refreshed", Value: stamp},
		{Key: "at", Value: stamp},
		{Key: "subject", Value: doc.Link(in.SubjectTitle)},
		{Key: "subject_id", Value: in.SubjectID},
		{Key: "session", Value: ""},
		{Key: "by", Value: by},
	}
	if in.FromType != "" {
		fields = append(fields, doc.Field{Key: "from_type", Value: in.FromType}, doc.Field{Key: "to_type", Value: in.ToType})
	}
	if len(in.Became) > 0 {
		fields = append(fields, doc.Field{Key: "became", Value: doc.Links(in.Became)})
	}
	if in.Change != "" {
		fields = append(fields, doc.Field{Key: "change", Value: doc.Link(in.Change)})
	}
	body := ""
	sections := schema.Get("event").SectionsOf(in.Kind)
	// The first step of a migration from 6.x writes the results of 7.0, which the second
	// step reads.
	for _, s := range legacySections {
		if _, ok := in.Prose[s]; ok && !slices.Contains(sections, s) {
			sections = append(slices.Clone(sections), s)
		}
	}
	for _, s := range sections {
		if text := strings.TrimSpace(in.Prose[s]); text != "" {
			body += "## " + s + "\n\n" + text + "\n\n"
		}
	}
	d := doc.Parse(vault.DocPath(title), []byte(doc.Render(fields, body)))
	content = doc.ReplaceLead(d.Content, EventLead(d, in.SubjectTitle, ""))
	return vault.DocPath(title), content, id
}

// EventLead is an event's lead callout: the kind, the subject, the time, and who acted.
// chord is the chord the subject belongs to, or "".
func EventLead(e *doc.Doc, subject, chord string) string {
	kind := e.Str("kind")
	at, _ := vault.ParseTime(e.Str("at"))
	title := capital(kind) + " · " + doc.Link(subject) + " · " + at.Format("2006-01-02 15:04:05")
	var line []string
	switch kind {
	case "promoted":
		line = append(line, "Promoted from "+e.Str("from_type")+" to "+e.Str("to_type"))
	case "resolved":
		if b := e.List("became"); len(b) > 0 {
			line = append(line, "Became "+strings.Join(b, ", "))
		}
	case "blocked":
		line = append(line, "Blocked: "+strings.TrimPrefix(e.Str("description"), "Blocked: "))
	}
	who := "By the agent"
	if e.Str("by") == ByUser {
		who = "By you"
	}
	if s := e.Str("session"); s != "" {
		who += " in " + s
	}
	if c := e.Str("change"); c != "" {
		who += " · through " + c
	}
	if chord != "" && !strings.EqualFold(chord, subject) {
		who += " · in the chord " + doc.Link(chord)
	}
	line = append(line, who)
	return doc.Callout("event-"+kind, title, line...)
}

func capital(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func oneLine(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len([]rune(s)) > n {
		s = string([]rune(s)[:n-1]) + "…"
	}
	return s
}

func nonNil(list []string) []string {
	if list == nil {
		return []string{}
	}
	return list
}
