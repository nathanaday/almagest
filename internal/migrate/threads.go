package migrate

import (
	"cmp"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/schema"
	"github.com/nathanaday/atlas-obsidian/internal/thread"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

// oldThread is a 6.x thread: its stub and the documents that name it.
type oldThread struct {
	stub     *doc.Doc
	spec     *doc.Doc
	tasks    []*doc.Doc
	receipts []*doc.Doc
}

// events tracks the events the migration writes, and the last time of each subject, so a
// document's events keep a strict order.
type events struct {
	p      *plan
	titles *thread.Titles
	last   map[string]time.Time
}

func (e *events) add(in thread.EventIn) *write {
	if last, ok := e.last[in.SubjectID]; ok && !in.At.After(last) {
		in.At = last.Add(time.Second)
	}
	e.last[in.SubjectID] = in.At
	in.By = thread.ByUser
	rel, content, _ := thread.NewEvent(e.titles, in)
	w := &write{to: rel, content: content}
	e.p.writes = append(e.p.writes, w)
	e.p.report.Events++
	e.p.titles.take(doc.TitleOf(rel), rel)
	return w
}

// taskPrefix matches a 6.x task title: <Thread> — T<n> <own title>.
var taskPrefix = regexp.MustCompile(`^(.*) — T(\d+) (.+)$`)

// reopenedDate reads the date a superseded receipt's title holds.
var reopenedDate = regexp.MustCompile(`\(reopened (\d{4}-\d{2}-\d{2})\)`)

var progressDate = regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)

// threads converts every thread: a stub alone stays a stub; a thread past its stub
// becomes a plan in place, its tasks the plan's parts, its receipts and task results
// events.
func (p *plan) threads(o *old, scopeTag map[string]string, absorbed map[string][]string) error {
	idx, err := vault.Load(p.v)
	if err != nil {
		return err
	}
	ev := &events{p: p, titles: thread.NewTitles(idx), last: map[string]time.Time{}}
	byID := map[string]*oldThread{}
	var list []*oldThread
	for _, s := range o.of("stub") {
		t := &oldThread{stub: s}
		byID[s.ID()] = t
		list = append(list, t)
	}
	for _, d := range o.of("spec", "task", "receipt") {
		t := byID[d.Str("thread_id")]
		if t == nil {
			if s := o.linked(d.Str("thread")); s != nil {
				t = byID[s.ID()]
			}
		}
		if t == nil {
			p.report.Warnings = append(p.report.Warnings, fmt.Sprintf("%s names no thread; it moves to the inbox", vault.Title(d)))
			o.notes = append(o.notes, d)
			continue
		}
		switch d.Type() {
		case "spec":
			if t.spec == nil {
				t.spec = d
			} else {
				p.report.Warnings = append(p.report.Warnings, fmt.Sprintf("%s is a second spec of %s; it moves to the inbox", vault.Title(d), vault.Title(t.stub)))
				o.notes = append(o.notes, d)
			}
		case "task":
			t.tasks = append(t.tasks, d)
		case "receipt":
			t.receipts = append(t.receipts, d)
		}
	}
	for _, t := range list {
		sort.SliceStable(t.tasks, func(i, j int) bool { return t.tasks[i].Front.Int("order") < t.tasks[j].Front.Int("order") })
		sort.SliceStable(t.receipts, func(i, j int) bool { return t.receipts[i].Str("created") < t.receipts[j].Str("created") })
		p.thread(o, t, scopeTag, absorbed, ev)
	}
	return nil
}

func (p *plan) thread(o *old, t *oldThread, scopeTag map[string]string, absorbed map[string][]string, ev *events) {
	s := t.stub
	title := vault.Title(s)
	idea, _ := doc.Section(s.Body, "Stub")
	notes, _ := doc.Section(s.Body, "Notes")
	var repos, allTags []string
	for _, v := range s.List("scope") {
		d := o.linked(v)
		if d == nil {
			continue
		}
		if tg := scopeTag[d.ID()]; tg != "" {
			allTags = append(allTags, tg)
		}
		if d.Type() == "repository" {
			repos = append(repos, doc.Link(vault.Title(d)))
		}
	}
	created := stamp(s.Str("created"), p.now)
	updated := stamp(s.Str("updated"), p.now)
	if t.spec == nil && len(t.tasks) == 0 && len(t.receipts) == 0 {
		if b := strings.TrimSpace(s.Str("blocked")); b != "" {
			notes = strings.TrimSpace(notes + "\n\nBlocked before 7.0: " + b)
		}
		body := "## Idea\n\n" + strings.TrimSpace(idea) + "\n"
		if strings.TrimSpace(notes) != "" {
			body += "\n## Notes\n\n" + strings.TrimSpace(notes) + "\n"
		}
		fields := []doc.Field{
			{Key: "id", Value: s.ID()},
			{Key: "type", Value: "stub"},
			{Key: "description", Value: cmp.Or(doc.FirstSentence(idea), title)},
			{Key: "tags", Value: union(allTags)},
			{Key: "aliases", Value: doc.NonNil(s.List("aliases"))},
			{Key: "created", Value: created},
			{Key: "updated", Value: updated},
			{Key: "refreshed", Value: updated},
			{Key: "priority", Value: cmp.Or(s.Str("priority"), "normal")},
			{Key: "status", Value: "open"},
			{Key: "became", Value: []string{}},
		}
		p.put(s, title, doc.Render(append(fields, p.keepUser(s, "stub")...), body))
		return
	}
	// The thread becomes a plan in place: the stub's id and title, the spec's sections.
	specBody := ""
	desc := ""
	if t.spec != nil {
		specBody = strings.TrimSpace(doc.StripLead(t.spec.Body))
		goal, _ := doc.Section(t.spec.Body, "Goal")
		desc = doc.FirstSentence(goal)
		p.rename[vault.Title(t.spec)] = title
		p.removes = append(p.removes, t.spec.Path)
		p.report.Retitles = append(p.report.Retitles, Retitle{Old: vault.Title(t.spec), New: title})
	}
	body := specBody + "\n"
	if strings.TrimSpace(idea) != "" {
		body += "\n## Origin\n\n" + strings.TrimSpace(idea) + "\n"
	}
	if strings.TrimSpace(notes) != "" {
		body += "\n## Notes\n\n" + strings.TrimSpace(notes) + "\n"
	}
	rootTags := union(allTags)
	root := []doc.Field{
		{Key: "id", Value: s.ID()},
		{Key: "type", Value: "spec"},
		{Key: "kind", Value: "plan"},
		{Key: "description", Value: cmp.Or(desc, cmp.Or(doc.FirstSentence(idea), title))},
		{Key: "tags", Value: rootTags},
		{Key: "aliases", Value: doc.NonNil(s.List("aliases"))},
		{Key: "created", Value: created},
		{Key: "updated", Value: updated},
		{Key: "refreshed", Value: updated},
		{Key: "repositories", Value: union(repos)},
		{Key: "priority", Value: cmp.Or(s.Str("priority"), "normal")},
		{Key: "status", Value: "open"},
	}
	rootWrite := &write{from: s.Path, to: vault.DocPath(title), content: doc.Render(append(root, p.keepUser(s, "spec")...), strings.TrimLeft(body, "\n"))}
	p.writes = append(p.writes, rootWrite)
	p.report.Documents++
	if t.spec != nil && wasAbsorbed(absorbed, t.spec) {
		p.absorbed = append(p.absorbed, rootWrite.to)
	}
	// The tasks become parts.
	newTitle := map[string]string{}
	for _, task := range t.tasks {
		old := vault.Title(task)
		nt := old
		if m := taskPrefix.FindStringSubmatch(old); m != nil {
			candidate := doc.CleanTitle(m[3])
			if candidate != "" && p.titles.free(candidate) {
				nt = candidate
			}
		}
		if nt != old {
			p.titles.take(nt, task.Path)
			p.rename[old] = nt
			p.report.Retitles = append(p.report.Retitles, Retitle{Old: old, New: nt})
		}
		newTitle[strings.ToLower(old)] = nt
	}
	var earliest time.Time
	note := func(at time.Time) {
		if earliest.IsZero() || at.Before(earliest) {
			earliest = at
		}
	}
	type pending struct {
		in       thread.EventIn
		absorbed bool
	}
	var taskEvents []pending
	for _, task := range t.tasks {
		nt := newTitle[strings.ToLower(vault.Title(task))]
		what, _ := doc.Section(task.Body, "What")
		result, _ := doc.Section(task.Body, "Result")
		progress, _ := doc.Section(task.Body, "Progress")
		tb := strings.TrimSpace(doc.StripLead(task.Body))
		tb = doc.RemoveSection(tb, "Result")
		tb = regexp.MustCompile(`(?m)^## What[ \t]*$`).ReplaceAllString(tb, "## Goal")
		var deps []string
		for _, d := range task.List("depends") {
			if dd := o.linked(d); dd != nil {
				deps = append(deps, doc.Link(newTitle[strings.ToLower(vault.Title(dd))]))
			}
		}
		var taskRepos []string
		if r := o.linked(task.Str("repository")); r != nil {
			taskRepos = []string{doc.Link(vault.Title(r))}
		}
		tCreated := stamp(task.Str("created"), p.now)
		tUpdated := stamp(task.Str("updated"), p.now)
		fields := []doc.Field{
			{Key: "id", Value: task.ID()},
			{Key: "type", Value: "spec"},
			{Key: "kind", Value: "plan"},
			{Key: "description", Value: cmp.Or(doc.FirstSentence(what), nt)},
			{Key: "tags", Value: rootTags},
			{Key: "aliases", Value: []string{}},
			{Key: "created", Value: tCreated},
			{Key: "updated", Value: tUpdated},
			{Key: "refreshed", Value: tUpdated},
			{Key: "parent", Value: doc.Link(title)},
			{Key: "repositories", Value: doc.NonNil(taskRepos)},
			{Key: "depends", Value: doc.NonNil(deps)},
			{Key: "order", Value: task.Front.Int("order")},
			{Key: "priority", Value: "normal"},
			{Key: "status", Value: "open"},
		}
		p.put(task, nt, doc.Render(append(fields, p.keepUser(task, "spec")...), strings.TrimSpace(tb)+"\n"))
		base := thread.EventIn{SubjectTitle: nt, SubjectID: task.ID(), SubjectTags: rootTags}
		at, _ := schema.ParseTime(tUpdated)
		if first := progressDate.FindString(progress); first != "" {
			if st, ok := schema.ParseTime(first + "T09:00:00"); ok {
				in := base
				in.Kind, in.At = "started", st
				taskEvents = append(taskEvents, pending{in: in})
				note(st)
			}
		}
		switch task.Str("status") {
		case "done":
			in := base
			in.Kind, in.At = "completed", at
			in.Prose = map[string]string{"Delivered": cmp.Or(result, "Done before 7.0."), "Verified": "Recorded before 7.0 in the task's result."}
			taskEvents = append(taskEvents, pending{in: in, absorbed: wasAbsorbed(absorbed, task)})
			note(at)
		case "dropped":
			in := base
			in.Kind, in.At = "dropped", at
			in.Prose = map[string]string{"Why": "Dropped before 7.0."}
			taskEvents = append(taskEvents, pending{in: in})
			note(at)
		}
	}
	for _, te := range taskEvents {
		w := ev.add(te.in)
		if te.absorbed {
			p.absorbed = append(p.absorbed, w.to)
		}
	}
	rootIn := thread.EventIn{SubjectTitle: title, SubjectID: s.ID(), SubjectTags: rootTags}
	if !earliest.IsZero() {
		in := rootIn
		in.Kind, in.At = "started", earliest.Add(-time.Second)
		ev.add(in)
	}
	for _, r := range t.receipts {
		at, _ := schema.ParseTime(stamp(r.Str("created"), p.now))
		in := rootIn
		in.At = at
		if r.Str("outcome") == "killed" {
			why, _ := doc.Section(r.Body, "Why killed")
			in.Kind = "dropped"
			in.Prose = map[string]string{"Why": cmp.Or(why, "Killed before 7.0.")}
		} else {
			in.Kind = "completed"
			in.Prose = map[string]string{}
			for _, sec := range []string{"Delivered", "Verified", "Follow-ups", "Learned"} {
				text, _ := doc.Section(r.Body, sec)
				in.Prose[sec] = text
			}
			if strings.TrimSpace(in.Prose["Delivered"]) == "" {
				in.Prose["Delivered"] = "Completed before 7.0."
			}
		}
		w := ev.add(in)
		p.rename[vault.Title(r)] = doc.TitleOf(w.to)
		p.removes = append(p.removes, r.Path)
		p.report.Retitles = append(p.report.Retitles, Retitle{Old: vault.Title(r), New: doc.TitleOf(w.to)})
		if wasAbsorbed(absorbed, r) {
			p.absorbed = append(p.absorbed, w.to)
		}
		if r.Front.Bool("superseded") {
			reopenAt := at.Add(time.Second)
			if m := reopenedDate.FindStringSubmatch(vault.Title(r)); m != nil {
				if rt, ok := schema.ParseTime(m[1] + "T12:00:00"); ok && rt.After(at) {
					reopenAt = rt
				}
			}
			ro := rootIn
			ro.Kind, ro.At = "reopened", reopenAt
			ev.add(ro)
		}
	}
	if b := strings.TrimSpace(s.Str("blocked")); b != "" && (len(t.receipts) == 0 || t.receipts[len(t.receipts)-1].Front.Bool("superseded")) {
		in := rootIn
		in.Kind, in.At, in.Line = "blocked", mustTime(updated), b
		ev.add(in)
	}
}

func mustTime(s string) time.Time {
	t, _ := schema.ParseTime(s)
	return t
}

// records renames the thread fields of the sessions and the changes.
func (p *plan) records(o *old) {
	for _, s := range o.of("session") {
		c := s.Content
		if s.Front.Has("threads") {
			c = doc.SetField(c, "work", doc.NonNil(s.List("threads")))
			c = doc.RemoveField(c, "threads")
		}
		if s.Front.Has("tasks") {
			c = doc.SetField(c, "specs", doc.NonNil(s.List("tasks")))
			c = doc.RemoveField(c, "tasks")
		}
		if c != s.Content {
			p.writes = append(p.writes, &write{from: s.Path, to: s.Path, content: c})
		}
	}
	for _, ch := range o.of("change") {
		if !ch.Front.Has("thread") {
			continue
		}
		c := doc.SetField(ch.Content, "work", ch.Str("thread"))
		c = doc.RemoveField(c, "thread")
		p.writes = append(p.writes, &write{from: ch.Path, to: ch.Path, content: c})
	}
}

// files moves the captured originals and the attachments into wiki/assets, the notes
// with no type into the inbox, and the Bases and the canvas of 6.x out.
func (p *plan) files(o *old) {
	taken := map[string]bool{}
	for _, f := range o.files {
		name := path.Base(f)
		switch {
		case strings.HasSuffix(f, ".base") || strings.HasSuffix(f, ".canvas"):
			if template, err := vault.OldTemplate(name); err == nil {
				if data, err := p.v.Read(f); err == nil && vault.SameYAML(string(data), template) {
					p.removes = append(p.removes, f)
					p.report.Removed = append(p.report.Removed, f)
					continue
				}
			}
			if name == "Scope.base" {
				p.removes = append(p.removes, f)
				p.report.Removed = append(p.report.Removed, f)
				continue
			}
			to := "scratchpad/from 6.x/" + name
			p.moves = append(p.moves, move{f, to})
			p.report.Scratchpad = append(p.report.Scratchpad, to)
		default:
			to := vault.Assets + "/" + name
			for n := 2; taken[strings.ToLower(to)] || (p.v.Exists(to) && to != f); n++ {
				ext := path.Ext(name)
				to = fmt.Sprintf("%s/%s (%d)%s", vault.Assets, strings.TrimSuffix(name, ext), n, ext)
			}
			taken[strings.ToLower(to)] = true
			if !strings.HasPrefix(f, "wiki/sources/files/") && path.Base(to) != name {
				p.report.Warnings = append(p.report.Warnings, fmt.Sprintf("%s takes the name %s in wiki/assets", f, path.Base(to)))
			}
			p.moves = append(p.moves, move{f, to})
			p.report.Assets++
		}
	}
	for _, n := range o.notes {
		to := "inbox/from 6.x/" + n.Path
		p.writes = append(p.writes, &write{from: n.Path, to: to, content: n.Content})
		p.report.Inbox = append(p.report.Inbox, to)
	}
}

// atlas writes the settings of Atlas.md for 7.0.
func (p *plan) atlas() {
	data, err := p.v.Read(vault.Marker)
	if err != nil {
		return
	}
	c := string(data)
	d := doc.Parse(vault.Marker, data)
	tagging := "open"
	if d.Str("areas") == "manual" {
		tagging = "known"
	}
	if d.Front != nil && d.Front.Has("wikify") {
		var wikify []string
		for _, t := range d.List("wikify") {
			switch t {
			case "receipt":
				t = "event"
			case "task", "concept", "entity", "policy", "area":
				continue
			}
			if !slices.Contains(wikify, t) {
				wikify = append(wikify, t)
			}
		}
		c = doc.SetField(c, "wikify", doc.NonNil(wikify))
	}
	c = doc.RemoveField(c, "areas")
	c = doc.SetField(c, "tagging", tagging)
	c = doc.SetField(c, "layout", vault.Layout)
	p.writes = append(p.writes, &write{from: vault.Marker, to: vault.Marker, content: c})
}

// linkRewrites rewrites the links to every title the migration changes, in every file it
// writes and in every other note of the vault.
func (p *plan) linkRewrites(o *old) {
	if len(p.rename) == 0 {
		return
	}
	written := map[string]bool{}
	for _, w := range p.writes {
		if w.from != "" {
			written[w.from] = true
		}
		written[w.to] = true
		d := doc.Parse(w.to, []byte(w.content))
		w.content, _, _ = vault.RewriteContent(d, w.content, p.rename, nil)
	}
	idx, err := vault.Load(p.v)
	if err != nil {
		return
	}
	removed := map[string]bool{}
	for _, r := range p.removes {
		removed[r] = true
	}
	for _, d := range append(append(append([]*doc.Doc{}, idx.Notes...), idx.Docs...), idx.Misplaced...) {
		if written[d.Path] || removed[d.Path] {
			continue
		}
		c, _, _ := vault.RewriteContent(d, d.Content, p.rename, nil)
		if c != d.Content {
			p.writes = append(p.writes, &write{from: d.Path, to: d.Path, content: c})
		}
	}
}

// absorbChange writes the one change document that keeps what the wiki learned before
// absorbed: a spec or a receipt whose body only moved is not pending again.
func (p *plan) absorbChange(o *old) {
	if len(p.absorbed) == 0 {
		return
	}
	byPath := map[string]*write{}
	for _, w := range p.writes {
		byPath[w.to] = w
	}
	var b strings.Builder
	b.WriteString("> [!change] Applied · the migration to 7.0\n> The documents below were absorbed before the migration, which moved their text.\n\n## Notes\n\nThe migration to the 7.0 layout moved these documents. The wiki had absorbed each before, so none is pending again only because its text moved.\n\n## Absorbed\n\n| Document | Id | Hash |\n|---|---|---|\n")
	for _, rel := range p.absorbed {
		w := byPath[rel]
		if w == nil {
			continue
		}
		d := doc.Parse(rel, []byte(w.content))
		fmt.Fprintf(&b, "| %s | %s | %s |\n", doc.Link(d.Title()), d.ID(), doc.Short(vault.Hash(d)))
	}
	b.WriteString("\n## Writes\n")
	st := vault.Stamp(p.now)
	id := doc.NewID("chg", nil)
	content := doc.Render([]doc.Field{
		{Key: "id", Value: id},
		{Key: "type", Value: "change"},
		{Key: "created", Value: st},
		{Key: "updated", Value: st},
		{Key: "status", Value: "applied"},
		{Key: "absorbs", Value: []string{}},
		{Key: "work", Value: ""},
		{Key: "proposed", Value: st},
		{Key: "session", Value: ""},
		{Key: "counts", Value: "0 create, 0 modify, 0 promote, 0 rename, 0 remove, 0 confirm, 0 retag, 0 link rewrites, 0 tag rewrites"},
		{Key: "new_tags", Value: []string{}},
		{Key: "applied", Value: st},
		{Key: "supersedes", Value: ""},
		{Key: "reason", Value: ""},
	}, b.String())
	rel := fmt.Sprintf("%s/%s/%s Migrate to 7.0.md", vault.Changes, p.now.Format(vault.MonthFormat), vault.Date(p.now))
	p.writes = append(p.writes, &write{to: rel, content: content})
}

// execute writes the plan to disk. The caller holds the lock and commits.
func (p *plan) execute(tx *vault.Tx) error {
	v := p.v
	// Every path is checked before anything moves, so a refusal leaves the vault as it was.
	var paths []string
	for _, w := range p.writes {
		paths = append(paths, w.to)
		if w.from != "" {
			paths = append(paths, w.from)
		}
	}
	for _, m := range p.moves {
		paths = append(paths, m.from, m.to)
	}
	paths = append(paths, p.removes...)
	for _, rel := range paths {
		if err := v.Contain(rel); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	// Kept first, so a migration that fails before its commit puts every path back.
	if err := tx.Keep(paths...); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	for _, w := range p.writes {
		if err := v.Write(w.to, []byte(w.content)); err != nil {
			return err
		}
	}
	for _, w := range p.writes {
		if w.from != "" && w.from != w.to {
			if err := os.Remove(v.Abs(w.from)); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	for _, m := range p.moves {
		if err := os.MkdirAll(filepath.Dir(v.Abs(m.to)), 0o755); err != nil {
			return err
		}
		if err := os.Rename(v.Abs(m.from), v.Abs(m.to)); err != nil {
			return err
		}
	}
	for _, r := range p.removes {
		if err := os.Remove(v.Abs(r)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	// What the plan left is the migration's own, so a save that lands later survives a
	// rollback.
	tx.Settle(paths...)
	for _, top := range []string{"threads", "wiki"} {
		pruneEmpty(v.Abs(top), top == "threads")
	}
	return nil
}

// pruneEmpty removes the empty folders under root, and root itself when asked.
func pruneEmpty(root string, self bool) {
	var dirs []string
	filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err == nil && info.IsDir() && (self || p != root) {
			dirs = append(dirs, p)
		}
		return nil
	})
	sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) })
	for _, d := range dirs {
		if strings.HasSuffix(d, "/wiki/documents") || strings.HasSuffix(d, "/wiki/assets") {
			continue
		}
		os.Remove(d)
	}
}
