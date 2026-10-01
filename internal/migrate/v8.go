package migrate

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/links"
	"github.com/nathanaday/atlas-obsidian/internal/thread"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

// The second step takes a 7.x vault to 8.0. A plan becomes a thread in place: the plan's
// document keeps its id, its title, and its file, and becomes the stub, so every link
// holds. Its Goal, Done when, Decisions, Out of scope, Conventions, and Open questions
// become the spec; each line of Done when becomes a requirement. A done plan gets a task
// list with one checked task and a verification made from its completed event. What a
// spec does not hold (Where, Verify, Progress, and any section an agent added) goes into
// one note event, so nothing is lost. A plan with parts becomes a chord, and its parts
// its threads. A design becomes a draft topic.

// planCode are the sections of a 7.x spec that code wrote.
var planCode = []string{"Parts", "History", "Implemented by", "Origin"}

// planSpec maps the sections of a 7.x plan that the new spec keeps to their new names.
var planSpec = [][2]string{{"Goal", "Goal"}, {"Done when", "Requirements"}, {"Conventions", "Rules"}, {"Decisions", "Decisions"}, {"Out of scope", "Out of scope"}, {"Open questions", "Open questions"}}

// planFields are the fields of a 7.x spec that a stub or a chord does not hold.
var planFields = []string{"kind", "parent", "repositories", "depends", "order", "implements", "supersedes", "from", "parts", "root", "status", "blocked", "active"}

var listItem = regexp.MustCompile(`^(?:[-*+]|\d+[.)])\s+(?:\[.\]\s+)?(.*)$`)

type step2 struct {
	*plan
	idx      *vault.Index
	events   map[string][]*doc.Doc // subject id → its events, oldest first
	parts    map[string][]*doc.Doc // plan id → its child plans
	absorbed map[string][]string   // document id → the hashes applied changes absorbed
	ev       *events
	// kept are the new documents that stand for text the wiki absorbed before.
	kept []string
	// used are the completed events that became verifications.
	used map[string]bool
}

// build8 reads a 7.x vault and computes every write of the second step.
func build8(v *vault.Vault, now time.Time, report *Report) (*plan, error) {
	idx, err := vault.Load(v)
	if err != nil {
		return nil, err
	}
	p := &plan{v: v, now: now.Truncate(time.Second), report: report, titles: &titles{taken: map[string]string{}}, rename: links.Rename{}}
	var waiting []string
	for _, c := range idx.Of("change") {
		if s := c.Str("status"); s == "proposed" || s == "applying" {
			waiting = append(waiting, vault.Title(c))
		}
	}
	if len(waiting) > 0 {
		return nil, fmt.Errorf("apply or reject these changes first, since a change may absorb a plan, and a plan becomes a thread: %s", strings.Join(waiting, ", "))
	}
	for _, s := range idx.Of("session") {
		if st := s.Str("status"); st == "running" || st == "waiting" {
			p.report.Warnings = append(p.report.Warnings, fmt.Sprintf("the session %s is %s; the plan it started becomes a thread, and its next edit in a repository needs a task (thread tasks)", s.Title(), st))
		}
	}
	for _, d := range append(append(append([]*doc.Doc{}, idx.Docs...), idx.Notes...), idx.Misplaced...) {
		p.titles.take(vault.Title(d), d.Path)
	}
	s := &step2{plan: p, idx: idx, events: map[string][]*doc.Doc{}, parts: map[string][]*doc.Doc{}, absorbed: map[string][]string{}, used: map[string]bool{}}
	s.ev = &events{p: p, titles: thread.NewTitles(idx), last: map[string]time.Time{}}
	for _, e := range idx.Of("event") {
		id := e.Str("subject_id")
		if id == "" {
			if d := idx.Linked(e.Str("subject")); d != nil {
				id = d.ID()
			}
		}
		s.events[id] = append(s.events[id], e)
	}
	for id, list := range s.events {
		sort.SliceStable(list, func(i, j int) bool { return list[i].Str("at") < list[j].Str("at") })
		if at, ok := vault.ParseTime(list[len(list)-1].Str("at")); ok {
			s.ev.last[id] = at
		}
	}
	for _, c := range idx.Of("change") {
		if c.Str("status") != "applied" {
			continue
		}
		for _, a := range vault.ParseAbsorbed(c.Body) {
			s.absorbed[a.ID] = append(s.absorbed[a.ID], a.Hash)
		}
	}
	var plans []*doc.Doc
	for _, d := range idx.Of("spec") {
		switch {
		case d.Front == nil || d.Front.Has("thread"):
			// A spec of 8.0 already.
		case d.Str("kind") == "design":
			s.design(d)
		default:
			plans = append(plans, d)
			if parent := s.parent(d); parent != nil {
				s.parts[parent.ID()] = append(s.parts[parent.ID()], d)
			}
		}
	}
	for _, d := range plans {
		if s.parent(d) == nil && len(s.parts[d.ID()]) > 0 {
			s.chord(d)
		} else {
			s.thread(d)
		}
	}
	for _, d := range idx.Of("stub") {
		s.stub(d)
	}
	s.results()
	s.atlas8()
	p.linkRewrites(nil)
	s.keep()
	return p, nil
}

func (s *step2) parent(d *doc.Doc) *doc.Doc {
	p := s.idx.Linked(d.Str("parent"))
	if p == nil || p.Type() != "spec" || p.ID() == d.ID() {
		return nil
	}
	return p
}

// root is the top plan above a plan, or the plan itself.
func (s *step2) root(d *doc.Doc) *doc.Doc {
	seen := map[string]bool{d.ID(): true}
	for p := s.parent(d); p != nil && !seen[p.ID()]; p = s.parent(d) {
		seen[p.ID()] = true
		d = p
	}
	return d
}

// wasAbsorbed reports whether an applied change absorbed a 7.x document as it is now.
func (s *step2) wasAbsorbed(d *doc.Doc) bool {
	h := doc.ProseHash(d.Body, planCode)
	if d.Type() == "event" {
		h = doc.ProseHash(d.Body, nil)
	}
	for _, have := range s.absorbed[d.ID()] {
		if doc.SameHash(have, h) {
			return true
		}
	}
	return false
}

// status7 is a 7.x plan's status, from its lifecycle events.
func (s *step2) status7(d *doc.Doc) (status string, result *doc.Doc) {
	status = "open"
	for _, e := range s.events[d.ID()] {
		switch e.Str("kind") {
		case "started", "continued":
			status = "started"
		case "completed":
			status, result = "done", e
		case "dropped":
			status = "dropped"
		case "reopened":
			status, result = "open", nil
		}
	}
	return status, result
}

// sections cuts a body into its level-two sections, in order, with the text above the
// first one under "".
func sections(body string) (order []string, text map[string]string) {
	text = map[string]string{}
	cur := ""
	var buf []string
	flush := func() {
		if t := strings.TrimSpace(strings.Join(buf, "\n")); t != "" || cur != "" {
			if _, ok := text[cur]; !ok {
				order = append(order, cur)
			}
			text[cur] = strings.TrimSpace(text[cur] + "\n\n" + t)
		}
		buf = nil
	}
	heads := map[int]string{}
	for _, h := range doc.Headings(body) {
		if h.Level == 2 {
			heads[h.Line] = h.Title
		}
	}
	for i, l := range strings.Split(body, "\n") {
		if title, ok := heads[i]; ok {
			flush()
			cur = title
			continue
		}
		buf = append(buf, l)
	}
	flush()
	return order, text
}

// requirements turns the lines of a Done when list into numbered requirements: each top
// list item gets an id, and every other line stays under the item it follows.
func requirements(done string) (string, int) {
	var out []string
	n := 0
	for _, l := range strings.Split(strings.TrimSpace(done), "\n") {
		if m := listItem.FindStringSubmatch(l); m != nil && strings.TrimSpace(m[1]) != "" {
			n++
			out = append(out, fmt.Sprintf("- R%d: %s", n, strings.TrimSpace(m[1])))
			continue
		}
		if strings.HasPrefix(l, "- ") || strings.HasPrefix(l, "* ") {
			l = "  " + l
		}
		out = append(out, l)
	}
	return strings.TrimSpace(strings.Join(out, "\n")), n
}

// demote puts a section's text under a level-three heading, with its own headings one
// level down, so it fits inside another section.
func demote(title, text string) string {
	var out []string
	fence := false
	for _, l := range strings.Split(strings.TrimSpace(text), "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), "```") || strings.HasPrefix(strings.TrimSpace(l), "~~~") {
			fence = !fence
		}
		if !fence && strings.HasPrefix(l, "###") {
			l = "#" + l
		}
		out = append(out, l)
	}
	return "### " + title + "\n\n" + strings.Join(out, "\n")
}

// free is a title no document holds: the one asked for, or it with a number.
func (s *step2) free(title, path string) string {
	t := title
	for n := 2; !s.titles.free(t); n++ {
		t = fmt.Sprintf("%s (%d)", title, n)
	}
	s.titles.take(t, path)
	return t
}

func (s *step2) common(id, typ, description string, d *doc.Doc, updated string) []doc.Field {
	return []doc.Field{
		{Key: "id", Value: id},
		{Key: "type", Value: typ},
		{Key: "description", Value: description},
		{Key: "tags", Value: nonNil(d.List("tags"))},
		{Key: "aliases", Value: []string{}},
		{Key: "created", Value: updated},
		{Key: "updated", Value: updated},
		{Key: "refreshed", Value: updated},
	}
}

func (s *step2) newID() string {
	return doc.NewID("doc", func(x string) bool { return s.idx.ByID(x) != nil })
}

// userFields are the fields of a 7.x plan that are the user's own.
func (s *step2) userFields(d *doc.Doc, typ string, skip ...string) []doc.Field {
	return s.keepUser(d, typ, append(skip, planFields...)...)
}

// thread turns a plan into a thread in place.
func (s *step2) thread(d *doc.Doc) {
	title := d.Title()
	// A plan made by hand may lack its id; the thread needs one.
	id := d.ID()
	if id == "" {
		id = s.newID()
		s.report.Warnings = append(s.report.Warnings, fmt.Sprintf("%s had no id; it gets %s", title, id))
	}
	status, result := s.status7(d)
	order, text := sections(doc.StripLead(d.Body))
	idea := orDefault(text["Origin"], orDefault(d.Str("description"), title))
	chord := ""
	if root := s.root(d); root.ID() != d.ID() && len(s.parts[root.ID()]) > 0 {
		chord = doc.Link(root.Title())
	}
	var after []string
	for _, dep := range d.List("depends") {
		if x := s.idx.Linked(dep); x != nil {
			after = append(after, doc.Link(x.Title()))
		}
	}
	updated := stamp(d.Str("updated"), s.now)
	body := "## Idea\n\n" + strings.TrimSpace(idea) + "\n"
	if notes := strings.TrimSpace(text["Notes"]); notes != "" {
		body += "\n## Notes\n\n" + notes + "\n"
	}
	fields := []doc.Field{
		{Key: "id", Value: id},
		{Key: "type", Value: "stub"},
		{Key: "description", Value: orDefault(d.Str("description"), title)},
		{Key: "tags", Value: nonNil(d.List("tags"))},
		{Key: "aliases", Value: nonNil(d.List("aliases"))},
		{Key: "created", Value: stamp(d.Str("created"), s.now)},
		{Key: "updated", Value: updated},
		{Key: "refreshed", Value: updated},
		{Key: "priority", Value: orDefault(d.Str("priority"), "normal")},
		{Key: "chord", Value: chord},
		{Key: "after", Value: nonNil(after)},
		{Key: "status", Value: thread.StatusStub},
		{Key: "spec", Value: ""},
		{Key: "tasks", Value: ""},
		{Key: "verification", Value: "none"},
		{Key: "repositories", Value: []string{}},
		{Key: "blocked", Value: ""},
		{Key: "active", Value: false},
		{Key: "rank", Value: 0},
		{Key: "became", Value: []string{}},
	}
	s.writes = append(s.writes, &write{from: d.Path, to: d.Path, content: doc.Render(append(fields, s.userFields(d, "stub", "id")...), body)})
	s.report.Threads++

	// The spec: the sections a spec holds, when the plan said what done means. A done
	// plan with no list gets one requirement, so its thread ends as the plan did.
	reqText, n := requirements(text["Done when"])
	if n == 0 && status == "done" && result != nil {
		reqText, n = strings.TrimSpace("- R1: The goal above is met.\n\n"+text["Done when"]), 1
	}
	var spec *doc.Doc
	var left []string // the sections that go into the note
	known := map[string]bool{"": true, "Origin": true, "Notes": true, "Parts": true, "History": true}
	if n > 0 {
		var b strings.Builder
		for _, pair := range planSpec {
			t := strings.TrimSpace(text[pair[0]])
			if pair[1] == "Requirements" {
				t = reqText
			}
			if pair[0] == "Goal" && t == "" {
				t = orDefault(d.Str("description"), title)
			}
			if t != "" {
				b.WriteString("## " + pair[1] + "\n\n" + t + "\n\n")
			}
			known[pair[0]] = true
		}
		specTitle := s.free(thread.SpecTitle(title), "")
		goal := text["Goal"]
		f := append(s.common(s.newID(), "spec", orDefault(firstSentence(goal), orDefault(d.Str("description"), title)), d, updated),
			doc.Field{Key: "thread", Value: doc.Link(title)}, doc.Field{Key: "status", Value: thread.NotImplemented})
		content := doc.Render(f, strings.TrimSpace(b.String())+"\n")
		rel := vault.DocPath(specTitle)
		s.writes = append(s.writes, &write{to: rel, content: content})
		spec = doc.Parse(rel, []byte(content))
		s.report.Specs++
	} else {
		s.report.Warnings = append(s.report.Warnings, fmt.Sprintf("%s has no Done when list, so it gets no spec; its sections are in a note on it, and thread-spec writes the spec", title))
	}
	for _, name := range order {
		if !known[name] && strings.TrimSpace(text[name]) != "" {
			left = append(left, demote(name+" (7.x)", text[name]))
		}
	}
	repos := d.List("repositories")
	if status == "done" && result != nil && spec != nil {
		s.done(d, spec, result, text, repos, updated)
	} else if len(repos) > 0 {
		left = append([]string{"Repositories of the 7.x plan: " + strings.Join(repos, ", ") + "."}, left...)
	}
	if len(left) > 0 {
		s.ev.add(thread.EventIn{Kind: "note", SubjectTitle: title, SubjectID: id, SubjectTags: d.List("tags"), At: mustTime(updated),
			Prose: map[string]string{"Note": "The migration to 8.0 made this plan a thread. A spec holds requirements only, so these sections of the plan are kept here.\n\n" + strings.Join(left, "\n\n")}})
		s.report.Notes++
	}
}

// stub keeps a 7.x stub short: a section a stub does not hold goes into a note on it.
func (s *step2) stub(d *doc.Doc) {
	if d.Front == nil {
		return
	}
	order, text := sections(doc.StripLead(d.Body))
	var left []string
	for _, name := range order {
		switch name {
		case "", "Idea", "Thread", "Notes":
		default:
			if strings.TrimSpace(text[name]) != "" {
				left = append(left, demote(name+" (7.x)", text[name]))
			}
		}
	}
	if len(left) == 0 {
		return
	}
	body := strings.TrimSpace(text[""])
	if body != "" {
		body += "\n\n"
	}
	body += "## Idea\n\n" + strings.TrimSpace(text["Idea"]) + "\n"
	if notes := strings.TrimSpace(text["Notes"]); notes != "" {
		body += "\n## Notes\n\n" + notes + "\n"
	}
	front, _, _ := doc.Split(d.Content)
	s.writes = append(s.writes, &write{from: d.Path, to: d.Path, content: doc.Join(front, "\n"+body)})
	s.ev.add(thread.EventIn{Kind: "note", SubjectTitle: d.Title(), SubjectID: d.ID(), SubjectTags: d.List("tags"), At: mustTime(stamp(d.Str("updated"), s.now)),
		Prose: map[string]string{"Note": "The migration to 8.0 keeps a stub short: it holds the idea and your notes. These sections of the stub are kept here.\n\n" + strings.Join(left, "\n\n")}})
	s.report.Notes++
}

// done gives a done plan its task list, with one checked task, and its verification, from
// the completed event.
func (s *step2) done(d, spec, result *doc.Doc, text map[string]string, repos []string, updated string) {
	title := d.Title()
	reqs := thread.Requirements(spec.Body)
	ids := make([]string, len(reqs))
	for i, r := range reqs {
		ids[i] = r.ID
	}
	repo, repoTitle := "", ""
	if len(repos) > 0 {
		repo, repoTitle = repos[0], doc.LinkTarget(repos[0])
	}
	task := thread.TaskLine(thread.Task{ID: "T1", State: thread.TaskDone, Text: thread.TaskText("The work of the 7.x plan", ids), Trail: "done before 8.0"})
	var details []string
	for _, name := range []string{"Where", "Verify"} {
		if t := strings.TrimSpace(text[name]); t != "" {
			details = append(details, "**"+name+" (7.x)**\n\n"+strings.TrimSpace(strings.ReplaceAll("\n"+t, "\n### ", "\n#### ")))
		}
	}
	if len(repos) > 1 {
		details = append(details, "The plan named these repositories: "+strings.Join(repos, ", ")+".")
	}
	body := "## Tasks\n\n" + task + "\n"
	if len(details) > 0 {
		body += "\n## Details\n\n### T1\n\n" + strings.Join(details, "\n\n") + "\n"
	}
	tasksTitle := s.free(thread.TasksTitle(title, repoTitle), "")
	f := append(s.common(s.newID(), "tasks", "The tasks of "+title+".", d, updated),
		doc.Field{Key: "thread", Value: doc.Link(title)}, doc.Field{Key: "repository", Value: repo}, doc.Field{Key: "done", Value: 1}, doc.Field{Key: "total", Value: 1})
	tasksRel := vault.DocPath(tasksTitle)
	tasksContent := doc.Render(f, body)
	s.writes = append(s.writes, &write{to: tasksRel, content: tasksContent})
	s.report.TaskLists++

	_, was := sections(doc.StripLead(result.Body))
	evidence := "Verified in 7.x: " + cellText(orDefault(was["Verified"], "see the scope above"), 300)
	rows := []string{"| Requirement | Result | Evidence |", "|---|---|---|"}
	for _, r := range reqs {
		rows = append(rows, fmt.Sprintf("| %s: %s | pass | %s |", r.ID, cellText(r.Text, 400), evidence))
	}
	vb := "## Scope\n\n" + orDefault(strings.TrimSpace(was["Delivered"]), "Completed before 8.0.") + "\n\n## Requirements\n\n" + strings.Join(rows, "\n") + "\n\n## Findings\n\nNone.\n"
	var notes []string
	if t := strings.TrimSpace(was["Verified"]); t != "" {
		notes = append(notes, demote("Verified (7.x)", t))
	}
	for _, name := range []string{"Follow-ups", "Learned", "Notes"} {
		if t := strings.TrimSpace(was[name]); t != "" {
			notes = append(notes, demote(name, t))
		}
	}
	if len(notes) > 0 {
		vb += "\n## Notes\n\n" + strings.Join(notes, "\n\n") + "\n"
	}
	at := stamp(result.Str("at"), s.now)
	verTitle := s.free(thread.VerificationTitle(title, 1), "")
	vf := append(s.common(s.newID(), "verification", "Round 1 of the verification of "+title+", from the result recorded in 7.x.", d, at),
		doc.Field{Key: "thread", Value: doc.Link(title)},
		doc.Field{Key: "round", Value: 1},
		doc.Field{Key: "at", Value: at},
		doc.Field{Key: "by", Value: orDefault(result.Str("by"), thread.ByAgent)},
		doc.Field{Key: "session", Value: result.Str("session")},
		doc.Field{Key: "spec_hash", Value: thread.SpecHash(spec)},
		doc.Field{Key: "tasks_hash", Value: thread.TasksHash([]*doc.Doc{doc.Parse(tasksRel, []byte(tasksContent))})},
		doc.Field{Key: "verdict", Value: thread.Pass},
	)
	verRel := vault.DocPath(verTitle)
	s.writes = append(s.writes, &write{from: result.Path, to: verRel, content: doc.Render(vf, vb)})
	s.used[result.Path] = true
	s.rename[result.Title()] = verTitle
	s.report.Retitles = append(s.report.Retitles, Retitle{Old: result.Title(), New: verTitle})
	s.report.Verifications++
	if s.wasAbsorbed(d) || s.wasAbsorbed(result) {
		s.kept = append(s.kept, spec.Path, verRel)
	}
}

func cellText(text string, n int) string {
	t := strings.ReplaceAll(strings.Join(strings.Fields(text), " "), "|", `\|`)
	if r := []rune(t); len(r) > n {
		t = string(r[:n-1]) + "…"
	}
	return t
}

// chordEvents are the event kinds that fit a chord; the rest of a root plan's events go.
var chordEvents = []string{"dropped", "reopened", "note"}

// chord turns a plan with parts into a chord in place; its parts are its threads.
func (s *step2) chord(d *doc.Doc) {
	title := d.Title()
	order, text := sections(doc.StripLead(d.Body))
	goal := strings.TrimSpace(text["Goal"])
	if done := strings.TrimSpace(text["Done when"]); done != "" {
		goal = strings.TrimSpace(goal + "\n\nDone when:\n\n" + done)
	}
	body := "## Goal\n\n" + orDefault(goal, d.Str("description")) + "\n"
	if notes := strings.TrimSpace(text["Notes"]); notes != "" {
		body += "\n## Notes\n\n" + notes + "\n"
	}
	updated := stamp(d.Str("updated"), s.now)
	fields := []doc.Field{
		{Key: "id", Value: d.ID()},
		{Key: "type", Value: "chord"},
		{Key: "description", Value: orDefault(d.Str("description"), title)},
		{Key: "tags", Value: nonNil(d.List("tags"))},
		{Key: "aliases", Value: nonNil(d.List("aliases"))},
		{Key: "created", Value: stamp(d.Str("created"), s.now)},
		{Key: "updated", Value: updated},
		{Key: "refreshed", Value: updated},
		{Key: "priority", Value: orDefault(d.Str("priority"), "normal")},
		{Key: "status", Value: thread.ChordOpen},
	}
	s.writes = append(s.writes, &write{from: d.Path, to: d.Path, content: doc.Render(append(fields, s.userFields(d, "chord")...), body)})
	s.report.Chords++
	var left []string
	for _, name := range order {
		switch name {
		case "", "Goal", "Done when", "Notes", "Parts", "History", "Origin":
		default:
			if strings.TrimSpace(text[name]) != "" {
				left = append(left, demote(name+" (7.x)", text[name]))
			}
		}
	}
	if o := strings.TrimSpace(text["Origin"]); o != "" {
		left = append(left, demote("Origin (7.x)", o))
	}
	blocked := ""
	for _, e := range s.events[d.ID()] {
		switch e.Str("kind") {
		case "blocked":
			blocked = thread.BlockedLine(e)
		case "unblocked":
			blocked = ""
		}
		if !slices.Contains(chordEvents, e.Str("kind")) {
			s.removes = append(s.removes, e.Path)
			s.report.Removed = append(s.report.Removed, e.Path)
		}
	}
	if blocked != "" {
		left = append([]string{"The plan was blocked in 7.x: " + blocked + ". A chord is not blocked; block the thread that waits (thread block)."}, left...)
	}
	if len(left) > 0 {
		s.ev.add(thread.EventIn{Kind: "note", SubjectTitle: title, SubjectID: d.ID(), SubjectTags: d.List("tags"), At: mustTime(updated),
			Prose: map[string]string{"Note": "The migration to 8.0 made this plan a chord, and its parts threads. A chord holds its goal only, so these sections of the plan are kept here.\n\n" + strings.Join(left, "\n\n")}})
		s.report.Notes++
	}
}

// design turns a design spec into a draft topic: how a part works is knowledge.
func (s *step2) design(d *doc.Doc) {
	order, text := sections(doc.StripLead(d.Body))
	var explain []string
	for _, name := range order {
		switch name {
		case "", "Purpose", "Notes", "Implemented by", "Origin":
		default:
			if strings.TrimSpace(text[name]) != "" {
				explain = append(explain, demote(name, text[name]))
			}
		}
	}
	body := "## Definition\n\n" + orDefault(strings.TrimSpace(text["Purpose"]), d.Str("description")) + "\n"
	if len(explain) > 0 {
		body += "\n## Explanation\n\n" + strings.Join(explain, "\n\n") + "\n"
	}
	if o := strings.TrimSpace(text["Origin"]); o != "" {
		body += "\n## Origin\n\n" + o + "\n"
	}
	if n := strings.TrimSpace(text["Notes"]); n != "" {
		body += "\n## Notes\n\n" + n + "\n"
	}
	status := "draft"
	if d.Str("status") == "superseded" {
		status = "deprecated"
	}
	updated := stamp(d.Str("updated"), s.now)
	fields := []doc.Field{
		{Key: "id", Value: d.ID()},
		{Key: "type", Value: "topic"},
		{Key: "kind", Value: "concept"},
		{Key: "description", Value: orDefault(d.Str("description"), d.Title())},
		{Key: "tags", Value: nonNil(d.List("tags"))},
		{Key: "aliases", Value: nonNil(d.List("aliases"))},
		{Key: "created", Value: stamp(d.Str("created"), s.now)},
		{Key: "updated", Value: updated},
		{Key: "refreshed", Value: updated},
		{Key: "status", Value: status},
		{Key: "sources", Value: []string{}},
	}
	s.writes = append(s.writes, &write{from: d.Path, to: d.Path, content: doc.Render(append(fields, s.userFields(d, "topic")...), body)})
	s.report.Topics++
	s.report.Warnings = append(s.report.Warnings, fmt.Sprintf("the design %s is a draft topic now; it cites no source (wiki-edit)", d.Title()))
}

// results turns each completed event that became no verification into a note: the
// result of a plan that was reopened, or of a plan with no Done when list. 8.0 has no
// completed event.
func (s *step2) results() {
	gone := map[string]bool{}
	for _, r := range s.removes {
		gone[r] = true
	}
	for _, e := range s.idx.Of("event") {
		if e.Str("kind") != "completed" || s.used[e.Path] || gone[e.Path] {
			continue
		}
		order, text := sections(doc.StripLead(e.Body))
		var parts []string
		for _, name := range order {
			if name != "" && strings.TrimSpace(text[name]) != "" {
				parts = append(parts, demote(name, text[name]))
			}
		}
		content := doc.SetFields(e.Content, []doc.Field{{Key: "kind", Value: "note"}, {Key: "description", Value: "Note: the result of " + doc.LinkTarget(e.Str("subject")) + " recorded in 7.x"}})
		front, _, _ := doc.Split(content)
		body := "\n## Note\n\nThe result recorded in 7.x. The plan became a thread with no verification from it.\n\n" + strings.Join(parts, "\n\n") + "\n"
		s.writes = append(s.writes, &write{from: e.Path, to: e.Path, content: doc.Join(front, body)})
		s.report.Notes++
	}
}

// atlas8 sets the layout of Atlas.md, and adds the new types the wiki absorbs to a
// wikify list that holds spec.
func (s *step2) atlas8() {
	data, err := s.v.Read(vault.Marker)
	if err != nil {
		return
	}
	c := string(data)
	for _, w := range s.writes {
		if w.to == vault.Marker {
			c = w.content
		}
	}
	d := doc.Parse(vault.Marker, []byte(c))
	if d.Front != nil && d.Front.Has("wikify") {
		wikify := d.List("wikify")
		if slices.Contains(wikify, "spec") {
			for _, t := range []string{"verification", "chord"} {
				if !slices.Contains(wikify, t) {
					wikify = append(wikify, t)
				}
			}
		}
		c = doc.SetField(c, "wikify", nonNil(wikify))
	}
	c = doc.SetField(c, "layout", vault.Layout)
	s.writes = append(s.writes, &write{from: vault.Marker, to: vault.Marker, content: c})
}

// keep writes the one change document that keeps what the wiki learned before absorbed:
// the spec and the verification of a plan the wiki had absorbed are not pending again.
func (s *step2) keep() {
	if len(s.kept) == 0 {
		return
	}
	byPath := map[string]*write{}
	for _, w := range s.writes {
		byPath[w.to] = w
	}
	var b strings.Builder
	b.WriteString("> [!change] Applied · the migration to 8.0\n> The wiki absorbed the plans of the documents below before the migration.\n\n## Notes\n\nThe migration to 8.0 made each plan a thread. The wiki had absorbed these plans and their results before, so the specs and verifications they became are not pending again, and their threads are closed.\n\n## Absorbed\n\n| Document | Id | Hash |\n|---|---|---|\n")
	var titlesOf []string
	for _, rel := range s.kept {
		w := byPath[rel]
		if w == nil {
			continue
		}
		d := doc.Parse(rel, []byte(w.content))
		fmt.Fprintf(&b, "| %s | %s | %s |\n", doc.Link(d.Title()), d.ID(), doc.Short(vault.Hash(d)))
		titlesOf = append(titlesOf, d.Title())
	}
	b.WriteString("\n## Writes\n")
	st := vault.Stamp(s.now)
	content := doc.Render([]doc.Field{
		{Key: "id", Value: doc.NewID("chg", nil)},
		{Key: "type", Value: "change"},
		{Key: "created", Value: st},
		{Key: "updated", Value: st},
		{Key: "status", Value: "applied"},
		{Key: "absorbs", Value: doc.Links(titlesOf)},
		{Key: "work", Value: ""},
		{Key: "proposed", Value: st},
		{Key: "session", Value: ""},
		{Key: "counts", Value: "0 create, 0 modify, 0 promote, 0 rename, 0 remove, 0 confirm, 0 retag, 0 link rewrites, 0 tag rewrites"},
		{Key: "new_tags", Value: []string{}},
		{Key: "applied", Value: st},
		{Key: "supersedes", Value: ""},
		{Key: "reason", Value: ""},
	}, b.String())
	rel := fmt.Sprintf("%s/%s/%s Migrate to 8.0.md", vault.Changes, s.now.Format("2006-01"), vault.Date(s.now))
	s.writes = append(s.writes, &write{to: rel, content: content})
}
