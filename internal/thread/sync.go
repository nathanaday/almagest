package thread

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/links"
	"github.com/nathanaday/atlas-obsidian/internal/schema"
	"github.com/nathanaday/atlas-obsidian/internal/tags"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

// WriteFunc writes a file when its content differs, and reports whether it wrote.
type WriteFunc func(rel string, content []byte) (bool, error)

// Bounds of what a stub's Thread section lists.
const (
	MaxCardKnowledge = 12
	MaxCardSessions  = 3
)

// Sync makes every thread document, chord, canvas, and event agree with the vault: the
// derived fields, the lead callouts, and the sections code writes. It writes a file only
// when its content differs, and never changes updated. It returns the paths it wrote.
func (b *Board) Sync(write WriteFunc) ([]string, error) {
	var out []string
	for _, c := range b.Chords {
		rel, wrote, err := b.syncCanvas(c, write)
		if err != nil {
			return out, err
		}
		if wrote {
			out = append(out, rel)
		}
	}
	docs := b.Idx.Of("stub", "spec", "tasks", "verification", "chord", "event")
	for _, d := range docs {
		if d.Front == nil {
			continue
		}
		content := b.Derived(d)
		if content == d.Content {
			continue
		}
		wrote, err := write(d.Path, []byte(content))
		if err != nil {
			return out, err
		}
		if wrote {
			out = append(out, d.Path)
		}
	}
	return out, nil
}

// Derived is a document's content with its derived fields, its lead callout, and the
// sections code writes made current.
func (b *Board) Derived(d *doc.Doc) string {
	switch d.Type() {
	case "stub":
		return b.derivedStub(d)
	case "spec", "tasks", "verification":
		return b.derivedPart(d)
	case "chord":
		return b.derivedChord(d)
	case "event":
		return b.DerivedEvent(d)
	}
	return d.Content
}

func setFields(d *doc.Doc, set []doc.Field) string {
	content := d.Content
	for _, f := range set {
		if !d.Front.Has(f.Key) || !d.Front.Equal(f.Key, f.Value) {
			content = doc.SetField(content, f.Key, f.Value)
		}
	}
	return content
}

func putSection(content, typ, title, text string) string {
	front, body, _ := doc.Split(content)
	return doc.Join(front, doc.PutSection(body, title, text, schema.Get(typ).Sections))
}

func (b *Board) derivedStub(d *doc.Doc) string {
	t := b.threads[d.ID()]
	spec := ""
	if t.Spec != nil {
		spec = doc.Link(t.Spec.Title())
	}
	var set []doc.Field
	// A stub of 7.x has no chord or after; a view filters on them, so each stub holds both.
	for _, f := range []doc.Field{{Key: "chord", Value: ""}, {Key: "after", Value: []string{}}} {
		if !d.Front.Has(f.Key) {
			set = append(set, f)
		}
	}
	content := setFields(d, append(set, []doc.Field{
		{Key: "status", Value: b.Status(d)},
		{Key: "spec", Value: spec},
		{Key: "tasks", Value: t.CountsText()},
		{Key: "verification", Value: b.VerificationText(t)},
		{Key: "repositories", Value: t.Repositories()},
		{Key: "blocked", Value: b.Blocked(d)},
		{Key: "active", Value: b.Active(d)},
		{Key: "rank", Value: b.Rank(d)},
		{Key: "refreshed", Value: b.Refreshed(d)},
	}...))
	content = doc.ReplaceLead(content, b.stubLead(d))
	return putSection(content, "stub", "Thread", b.ThreadSection(d))
}

// derivedPart is a spec, a task list, or a verification: its tags follow its thread's.
func (b *Board) derivedPart(d *doc.Doc) string {
	t := b.Thread(d)
	if t == nil {
		return doc.ReplaceLead(d.Content, doc.Callout(d.Type(), "No thread", "Its thread field names no stub. Lint names the fix."))
	}
	set := []doc.Field{{Key: "tags", Value: nonNil(t.Stub.List("tags"))}}
	back := "Thread: " + doc.Link(t.Stub.Title()) + " (" + b.Status(t.Stub) + ")"
	var lead string
	switch d.Type() {
	case "spec":
		status := b.Status(d)
		set = append(set, doc.Field{Key: "status", Value: status})
		n := len(Requirements(d.Body))
		lead = doc.Callout("spec", fmt.Sprintf("%s · %d %s", capital(status), n, plural(n, "requirement", "requirements")), back)
	case "tasks":
		done, total := ListCounts(d)
		set = append(set, doc.Field{Key: "done", Value: done}, doc.Field{Key: "total", Value: total})
		title := fmt.Sprintf("%d/%d done", done, total)
		if r := d.Str("repository"); r != "" {
			title += " · " + r
		}
		lead = doc.Callout("tasks", title, back)
	case "verification":
		verdict := b.Verdict(t, d)
		set = append(set, doc.Field{Key: "verdict", Value: verdict})
		kind := "verification"
		if verdict == Pass || verdict == Fail {
			kind += "-" + verdict
		}
		title := fmt.Sprintf("Round %d · %s", d.Front.Int("round"), map[string]string{Pass: "pass", Fail: "fail", OpenFindings: "open findings", Stale: "stale: the spec or the tasks changed after it"}[verdict])
		who := "by the agent"
		if d.Str("by") == ByUser {
			who = "by you"
		}
		if s := d.Str("session"); s != "" {
			who += " in " + s
		}
		lead = doc.Callout(kind, title, back, minute(d.Str("at"))+" · "+who)
	}
	return doc.ReplaceLead(setFields(d, set), lead)
}

func (b *Board) derivedChord(d *doc.Doc) string {
	closed, total := b.ChordCounts(d)
	set := []doc.Field{
		{Key: "status", Value: b.Status(d)},
		{Key: "threads", Value: fmt.Sprintf("%d/%d", closed, total)},
		{Key: "refreshed", Value: b.Refreshed(d)},
	}
	if h, ok := b.canvas[d.ID()]; ok {
		set = append(set, doc.Field{Key: "canvas", Value: h})
	}
	content := doc.ReplaceLead(setFields(d, set), b.chordLead(d))
	return putSection(content, "chord", "Threads", b.ChordSection(d))
}

// DerivedEvent is an event's content with its lead callout current.
func (b *Board) DerivedEvent(e *doc.Doc) string {
	subject := doc.LinkTarget(e.Str("subject"))
	part := ""
	if s := b.Subject(e); s != nil {
		subject = s.Title()
		if s.Type() == "stub" {
			if c := b.Chord(s); c != nil {
				part = c.Title()
			}
		}
	}
	return doc.ReplaceLead(e.Content, EventLead(e, subject, part))
}

// Refreshed is when a stub or a chord last changed state: its last event, or its
// creation.
func (b *Board) Refreshed(d *doc.Doc) string {
	at := d.Str("created")
	if e := b.LastEvent(d); e != nil {
		at = e.Str("at")
	}
	if t := b.threads[d.ID()]; t != nil {
		for _, x := range t.Docs()[1:] {
			if u := x.Str("updated"); u > at {
				at = u
			}
		}
	}
	return at
}

// afterLine names the threads a stub comes after, each with its status.
func (b *Board) afterLine(d *doc.Doc) string {
	after := b.After(d)
	if len(after) == 0 {
		return ""
	}
	parts := make([]string, len(after))
	for i, a := range after {
		parts[i] = doc.Link(a.Title()) + " (" + b.Status(a) + ")"
	}
	return "after " + strings.Join(parts, ", ")
}

func (b *Board) placeLine(d *doc.Doc) string {
	var parts []string
	if c := b.Chord(d); c != nil {
		parts = append(parts, "Chord: "+doc.Link(c.Title()))
	}
	if a := b.afterLine(d); a != "" {
		if len(parts) == 0 {
			a = capital(a)
		}
		parts = append(parts, a)
	}
	return strings.Join(parts, " · ")
}

// stubLead is the lead callout of a thread's front page.
func (b *Board) stubLead(d *doc.Doc) string {
	t := b.threads[d.ID()]
	status := b.Status(d)
	place := b.placeLine(d)
	lines := func(list ...string) []string {
		var out []string
		for _, l := range list {
			if l != "" {
				out = append(out, l)
			}
		}
		return out
	}
	switch status {
	case Resolved:
		title := "Resolved"
		if e := b.Result(d); e != nil {
			title += " " + day(e.Str("at"))
		}
		if became := d.List("became"); len(became) > 0 {
			title += " → " + strings.Join(became, ", ")
		}
		return doc.Callout("thread-resolved", title, lines(place)...)
	case Dropped:
		title, why, see := "Dropped", "", ""
		if e := b.Result(d); e != nil {
			title += " " + day(e.Str("at"))
			text, _ := doc.Section(e.Body, "Why")
			why = oneLine(doc.FirstLine(text), 160)
			see = "See " + doc.Link(e.Title())
		}
		return doc.Callout("thread-dropped", title, lines(why, see, place)...)
	case Closed:
		title := "Closed"
		if at := b.EndedAt(d); at != "" {
			title += " " + day(at)
		}
		if c := b.Closer(d); c != nil {
			title += " · absorbed by " + doc.Link(vault.Title(c))
		}
		if n := t.CountsText(); n != "" {
			title += " · " + n + " tasks"
		}
		return doc.Callout("thread-closed", title, lines(place)...)
	}
	priority := orDefault(d.Str("priority"), "normal")
	var title, tagsLine string
	if status == StatusStub {
		title = "Stub · " + priority + " · planted " + day(d.Str("created"))
		if s := b.plantedIn(d); s != "" {
			title += " in " + doc.Link(s)
		}
		tagsLine = tagLine(d)
	} else {
		title = capital(status)
		if n := t.CountsText(); n != "" {
			title += " · " + n + " tasks"
		}
		switch last := t.Last(); {
		case status == Verified:
			// The status says it.
		case last == nil:
			title += " · not verified"
		case b.Verdict(t, last) == Pass:
			title += " · round " + fmt.Sprint(last.Front.Int("round")) + " passed"
		default:
			title += " · " + b.VerificationText(t)
		}
		title += " · " + priority
	}
	if hs := b.Holders(d); len(hs) > 0 {
		title += " · active in " + doc.Link(hs[0].Title())
	}
	blocked := ""
	if line := b.Blocked(d); line != "" {
		blocked = "Blocked: " + oneLine(line, 160)
	}
	missing := ""
	if m := b.Missing(d); len(m) > 0 {
		missing = "Missing: " + strings.Join(m, " · ")
	}
	next := b.Next(d)
	nextLine := "Next: " + next.Reason
	if next.Skill != "" {
		nextLine += " (" + next.Skill + ")"
	}
	return doc.Callout("thread", title, lines(tagsLine, place, blocked, missing, nextLine)...)
}

// plantedIn is the first session whose work lists a document, or "".
func (b *Board) plantedIn(d *doc.Doc) string {
	var first *doc.Doc
	for _, s := range b.Idx.Of("session") {
		for _, l := range s.List("work") {
			if strings.EqualFold(doc.LinkTarget(l), d.Title()) && (first == nil || s.Str("started") < first.Str("started")) {
				first = s
			}
		}
	}
	if first == nil {
		return ""
	}
	return first.Title()
}

// Knowledge are the wiki pages a thread's spec cites: the topics, sources, and
// repositories it links, in order. Its Knowledge section lists what it relied on, and a
// link in a rule or a decision counts too.
func (b *Board) Knowledge(t *Thread) []*doc.Doc {
	if t.Spec == nil {
		return nil
	}
	var out []*doc.Doc
	seen := map[string]bool{}
	for _, l := range links.Find(t.Spec.Body) {
		d := b.Idx.Linked(doc.Link(l.Target))
		if d == nil || seen[d.ID()] {
			continue
		}
		switch d.Type() {
		case "topic", "source", "repository":
			seen[d.ID()] = true
			out = append(out, d)
		}
	}
	return out
}

// Sessions are the sessions that started a thread or wrote one of its documents, the
// newest first.
func (b *Board) Sessions(t *Thread) []*doc.Doc {
	names := map[string]bool{}
	for _, d := range t.Docs() {
		names[strings.ToLower(d.Title())] = true
	}
	var out []*doc.Doc
	for _, s := range b.Idx.Of("session") {
		found := false
		for _, field := range []string{"threads", "specs", "work"} {
			if slices.ContainsFunc(s.List(field), func(l string) bool { return names[strings.ToLower(doc.LinkTarget(l))] }) {
				found = true
				break
			}
		}
		if found {
			out = append(out, s)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Str("started") > out[j].Str("started") })
	return out
}

// Handoff is the line a user gives an agent to take up a thread or a chord.
func Handoff(d *doc.Doc) string {
	kind := "thread"
	if d.Type() == "chord" {
		kind = "chord"
	}
	return "Resume Atlas " + kind + " " + d.ID()
}

func handoffCallout(d *doc.Doc) string {
	return doc.Callout("handoff", "Hand off to an agent", "```", Handoff(d), "```")
}

// ThreadSection is the Thread section of a stub: the documents of the thread with their
// counts, the knowledge its spec cites, its sessions, and the hand-off line.
func (b *Board) ThreadSection(d *doc.Doc) string {
	t := b.threads[d.ID()]
	var lines []string
	if t.Spec == nil {
		lines = append(lines, "- Spec: none")
	} else {
		n := len(Requirements(t.Spec.Body))
		lines = append(lines, fmt.Sprintf("- Spec: %s · %d %s · %s", doc.Link(t.Spec.Title()), n, plural(n, "requirement", "requirements"), b.Status(t.Spec)))
	}
	if len(t.Lists) == 0 {
		lines = append(lines, "- Tasks: none")
	} else {
		parts := make([]string, len(t.Lists))
		for i, l := range t.Lists {
			done, total := ListCounts(l)
			parts[i] = fmt.Sprintf("%s %d/%d", doc.Link(l.Title()), done, total)
		}
		lines = append(lines, "- Tasks: "+strings.Join(parts, " · "))
	}
	if last := t.Last(); last == nil {
		lines = append(lines, "- Verification: none")
	} else {
		line := "- Verification: " + doc.Link(last.Title()) + " · " + b.Verdict(t, last)
		if n := openFindings(last); n > 0 {
			line += fmt.Sprintf(" · %d open %s", n, plural(n, "finding", "findings"))
		}
		if n := len(t.Rounds) - 1; n > 0 {
			var earlier []string
			for _, r := range t.Rounds[:n] {
				earlier = append(earlier, doc.Link(fmt.Sprintf("%s|%d", r.Title(), r.Front.Int("round"))))
			}
			line += " · earlier: " + strings.Join(earlier, ", ")
		}
		lines = append(lines, line)
	}
	if k := b.Knowledge(t); len(k) > 0 {
		more := ""
		if len(k) > MaxCardKnowledge {
			more = fmt.Sprintf(", and %d more", len(k)-MaxCardKnowledge)
			k = k[:MaxCardKnowledge]
		}
		lines = append(lines, "- Knowledge: "+titles(k)+more)
	}
	if s := b.Sessions(t); len(s) > 0 {
		if len(s) > MaxCardSessions {
			s = s[:MaxCardSessions]
		}
		lines = append(lines, "- Sessions: "+titles(s))
	}
	out := strings.Join(lines, "\n")
	if !Ended(b.Status(d)) {
		out += "\n\n" + handoffCallout(d)
	}
	return out
}

func (b *Board) chordLead(d *doc.Doc) string {
	status := b.Status(d)
	closed, total := b.ChordCounts(d)
	title := fmt.Sprintf("%s · %d/%d threads closed", capital(status), closed, total)
	switch status {
	case ChordDropped:
		lines := []string{}
		for i := len(b.events[d.ID()]) - 1; i >= 0; i-- {
			if e := b.events[d.ID()][i]; e.Str("kind") == "dropped" {
				text, _ := doc.Section(e.Body, "Why")
				if why := oneLine(doc.FirstLine(text), 160); why != "" {
					lines = append(lines, why)
				}
				lines = append(lines, "See "+doc.Link(e.Title()))
				break
			}
		}
		return doc.Callout("chord-dropped", "Dropped", lines...)
	case ChordClosed:
		if c := b.Idx.AbsorbedBy(d); c != nil {
			title += " · absorbed by " + doc.Link(vault.Title(c))
		}
		return doc.Callout("chord-closed", title)
	}
	title += " · " + orDefault(d.Str("priority"), "normal")
	var lines []string
	if t := tagLine(d); t != "" {
		lines = append(lines, t)
	}
	if ready := b.ChordReady(d); len(ready) > 0 {
		lines = append(lines, "Ready: "+titles(ready))
	}
	next := b.ChordNext(d)
	line := "Next: " + next.Reason
	if next.Skill != "" {
		line += " (" + next.Skill + ")"
	}
	return doc.Callout("chord", title, append(lines, line)...)
}

// ChordSection is the Threads section of a chord: its threads in order, a link to its
// canvas, and the hand-off line.
func (b *Board) ChordSection(d *doc.Doc) string {
	members := b.Members(d)
	var out []string
	if len(members) == 0 {
		out = append(out, "No thread yet. The chord-create skill plants them, or `chord add` takes a stub that exists.")
	} else {
		rows := []string{"| Step | Thread | Status | Tasks | After | Repositories |", "|---|---|---|---|---|---|"}
		for _, s := range members {
			status := b.Status(s)
			switch {
			case b.Blocked(s) != "":
				status += ", blocked"
			case b.Active(s):
				status += ", active"
			case b.ReadyToStart(s):
				status += ", ready"
			}
			var after []string
			for _, a := range b.After(s) {
				after = append(after, doc.Link(a.Title()))
			}
			t := b.threads[s.ID()]
			rows = append(rows, fmt.Sprintf("| %d | %s | %s | %s | %s | %s |", b.Rank(s)+1, doc.Link(s.Title()), status, t.CountsText(), strings.Join(after, ", "), strings.Join(t.Repositories(), ", ")))
		}
		out = append(out, strings.Join(rows, "\n"))
		out = append(out, "Canvas: "+doc.Link(CanvasPath(d)+"|the order as a graph"))
	}
	if s := b.Status(d); s != ChordDropped && s != ChordClosed {
		out = append(out, handoffCallout(d))
	}
	return strings.Join(out, "\n\n")
}

func tagLine(d *doc.Doc) string {
	list := d.List("tags")
	out := make([]string, 0, len(list))
	for _, t := range list {
		if tags.Valid(t) {
			out = append(out, "#"+t)
		}
	}
	sort.Strings(out)
	return strings.Join(out, " · ")
}

func day(stamp string) string {
	if t, ok := vault.ParseTime(stamp); ok {
		return vault.Date(t)
	}
	return stamp
}

func minute(stamp string) string {
	if t, ok := vault.ParseTime(stamp); ok {
		return t.Format("2006-01-02 15:04")
	}
	return stamp
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
