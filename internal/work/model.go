// Package work keeps the work documents: the stub, an idea planted in a hurry; the spec,
// a plan for work or the design of how something must work; and the event, a record of
// what happened to a document. Nothing sets the status of a stub or a plan: it is the
// last lifecycle event, and Sync writes it into the document.
package work

import (
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

// Statuses of a stub and a spec.
const (
	Open       = "open"
	Resolved   = "resolved"
	Dropped    = "dropped"
	Started    = "started"
	Done       = "done"
	Current    = "current"
	Superseded = "superseded"
)

// Kinds of a spec.
const (
	Plan   = "plan"
	Design = "design"
)

// Trailer names the document a work commit wrote.
const Trailer = "Atlas-Work"

// Priorities, highest first.
var Priorities = []string{"high", "normal", "low", "someday"}

// liveStatuses are the session statuses that hold the plans a session started.
var liveStatuses = []string{"running", "waiting", "idle"}

// Board is every stub, spec, and event of a vault, read from one index.
type Board struct {
	Idx        *vault.Index
	Stubs      []*doc.Doc
	Specs      []*doc.Doc
	events     map[string][]*doc.Doc // subject id → its events, oldest first
	parts      map[string][]*doc.Doc // plan id → its child plans, in order
	live       map[string][]*doc.Doc // spec title, lower case → the live sessions that started it
	superseded map[string]bool       // design id → a later design supersedes it
}

// Load reads the work of an index.
func Load(idx *vault.Index) *Board {
	b := &Board{Idx: idx, events: map[string][]*doc.Doc{}, parts: map[string][]*doc.Doc{}, live: map[string][]*doc.Doc{}, superseded: map[string]bool{}}
	b.Stubs = idx.Of("stub")
	b.Specs = idx.Of("spec")
	for _, e := range idx.Of("event") {
		if s := b.Subject(e); s != nil {
			b.events[s.ID()] = append(b.events[s.ID()], e)
		}
	}
	for id := range b.events {
		sortEvents(b.events[id])
	}
	for _, s := range b.Specs {
		if s.Str("kind") == Design {
			if old := idx.Linked(s.Str("supersedes")); old != nil {
				b.superseded[old.ID()] = true
			}
			continue
		}
		if p := b.ParentOf(s); p != nil {
			b.parts[p.ID()] = append(b.parts[p.ID()], s)
		}
	}
	for id := range b.parts {
		sort.SliceStable(b.parts[id], func(i, j int) bool { return lessOrder(b.parts[id][i], b.parts[id][j]) })
	}
	for _, s := range idx.Of("session") {
		if !slices.Contains(liveStatuses, s.Str("status")) {
			continue
		}
		for _, l := range s.List("specs") {
			k := strings.ToLower(doc.LinkTarget(l))
			b.live[k] = append(b.live[k], s)
		}
	}
	return b
}

func sortEvents(list []*doc.Doc) {
	sort.SliceStable(list, func(i, j int) bool {
		a, b := list[i].Str("at"), list[j].Str("at")
		if a != b {
			return a < b
		}
		return list[i].Path < list[j].Path
	})
}

func lessOrder(a, b *doc.Doc) bool {
	oa, ob := a.Front.Int("order"), b.Front.Int("order")
	if oa != ob {
		return oa < ob
	}
	return a.Title() < b.Title()
}

// Subject is the document an event is about: by its subject_id, else by its link.
func (b *Board) Subject(e *doc.Doc) *doc.Doc {
	if d := b.Idx.ByID(e.Str("subject_id")); d != nil {
		return d
	}
	return b.Idx.Linked(e.Str("subject"))
}

// Events are a document's events, oldest first.
func (b *Board) Events(d *doc.Doc) []*doc.Doc { return b.events[d.ID()] }

// Parts are a plan's child plans, in order.
func (b *Board) Parts(d *doc.Doc) []*doc.Doc { return b.parts[d.ID()] }

// ParentOf is the plan a plan is a part of, or nil.
func (b *Board) ParentOf(d *doc.Doc) *doc.Doc {
	p := b.Idx.Linked(d.Str("parent"))
	if p == nil || p.Type() != "spec" || p.Str("kind") != Plan || p.ID() == d.ID() {
		return nil
	}
	return p
}

// Ancestors are the plans above a plan, the parent first, stopping at a loop.
func (b *Board) Ancestors(d *doc.Doc) []*doc.Doc {
	var out []*doc.Doc
	seen := map[string]bool{d.ID(): true}
	for p := b.ParentOf(d); p != nil && !seen[p.ID()]; p = b.ParentOf(p) {
		seen[p.ID()] = true
		out = append(out, p)
	}
	return out
}

// Root is the top plan of a plan's tree: itself at the top.
func (b *Board) Root(d *doc.Doc) *doc.Doc {
	if a := b.Ancestors(d); len(a) > 0 {
		return a[len(a)-1]
	}
	return d
}

// Below are every plan under a plan, depth first.
func (b *Board) Below(d *doc.Doc) []*doc.Doc {
	var out []*doc.Doc
	seen := map[string]bool{d.ID(): true}
	var walk func(p *doc.Doc)
	walk = func(p *doc.Doc) {
		for _, c := range b.parts[p.ID()] {
			if seen[c.ID()] {
				continue
			}
			seen[c.ID()] = true
			out = append(out, c)
			walk(c)
		}
	}
	walk(d)
	return out
}

// lifecycle maps the event kinds that set a status to the status they set, by type.
var lifecycle = map[string]map[string]string{
	"stub": {"resolved": Resolved, "dropped": Dropped, "reopened": Open},
	"spec": {"started": Started, "continued": Started, "completed": Done, "dropped": Dropped, "reopened": Open},
}

// Status is a stub's or a spec's status, derived: a plan's or a stub's last lifecycle
// event, open when it has none; a design's current or superseded.
func (b *Board) Status(d *doc.Doc) string {
	if d.Type() == "spec" && d.Str("kind") == Design {
		if b.superseded[d.ID()] {
			return Superseded
		}
		return Current
	}
	kinds := lifecycle[d.Type()]
	status := Open
	for _, e := range b.events[d.ID()] {
		if s, ok := kinds[e.Str("kind")]; ok {
			status = s
		}
	}
	return status
}

// Blocked is the line of a plan's last blocked event, until an unblocked event follows.
func (b *Board) Blocked(d *doc.Doc) string {
	line := ""
	for _, e := range b.events[d.ID()] {
		switch e.Str("kind") {
		case "blocked":
			line = BlockedLine(e)
		case "unblocked":
			line = ""
		}
	}
	return line
}

// BlockedLine is the line a blocked event holds.
func BlockedLine(e *doc.Doc) string {
	return strings.TrimSpace(strings.TrimPrefix(e.Str("description"), "Blocked: "))
}

// LastEvent is a document's newest event, or nil.
func (b *Board) LastEvent(d *doc.Doc) *doc.Doc {
	if list := b.events[d.ID()]; len(list) > 0 {
		return list[len(list)-1]
	}
	return nil
}

// Result is a done plan's completed event, or a dropped one's dropped event.
func (b *Board) Result(d *doc.Doc) *doc.Doc {
	want := map[string]string{Done: "completed", Dropped: "dropped", Resolved: "resolved"}[b.Status(d)]
	if want == "" {
		return nil
	}
	list := b.events[d.ID()]
	for i := len(list) - 1; i >= 0; i-- {
		if list[i].Str("kind") == want {
			return list[i]
		}
	}
	return nil
}

// Holders are the live sessions that started a spec.
func (b *Board) Holders(d *doc.Doc) []*doc.Doc { return b.live[strings.ToLower(d.Title())] }

// Active reports whether a live session started a plan.
func (b *Board) Active(d *doc.Doc) bool { return len(b.Holders(d)) > 0 }

// Closed reports whether a status ends the work: done, dropped, or resolved.
func Closed(status string) bool { return status == Done || status == Dropped || status == Resolved }

// Ready reports whether a plan is open and every plan it depends on is done or dropped.
func (b *Board) Ready(d *doc.Doc) bool {
	if d.Str("kind") != Plan || b.Status(d) != Open {
		return false
	}
	for _, dep := range d.List("depends") {
		if s := b.Idx.Linked(dep); s != nil && !Closed(b.Status(s)) {
			return false
		}
	}
	return true
}

// PartCounts are a plan's parts done over its parts, dropped ones left out.
func (b *Board) PartCounts(d *doc.Doc) (done, total int) {
	for _, p := range b.parts[d.ID()] {
		switch b.Status(p) {
		case Done:
			done++
			total++
		case Dropped:
		default:
			total++
		}
	}
	return
}

// PartsText is what the parts field holds: "1/2", or "" for a plan with no parts.
func (b *Board) PartsText(d *doc.Doc) string {
	if len(b.parts[d.ID()]) == 0 {
		return ""
	}
	done, total := b.PartCounts(d)
	return strconv.Itoa(done) + "/" + strconv.Itoa(total)
}

// Next is a document's next step: write, start <plan>, done, or none.
func (b *Board) Next(d *doc.Doc) string {
	if d.Type() == "stub" {
		if b.Status(d) == Open {
			return "write"
		}
		return "none"
	}
	if d.Str("kind") != Plan || Closed(b.Status(d)) {
		return "none"
	}
	if done, _ := doc.Section(d.Body, "Done when"); strings.TrimSpace(done) == "" {
		return "write"
	}
	parts := b.parts[d.ID()]
	if len(parts) == 0 {
		if b.Status(d) == Open {
			return "start " + d.Title()
		}
		return "done"
	}
	for _, p := range parts {
		if b.Status(p) == Started {
			return b.Next(p)
		}
	}
	for _, p := range parts {
		if b.Ready(p) {
			return b.Next(p)
		}
	}
	for _, p := range parts {
		if !Closed(b.Status(p)) {
			return b.Next(p)
		}
	}
	return "done"
}

// Less orders open work: an active plan first, then by priority, then the newest
// event or creation.
func (b *Board) Less(x, y *doc.Doc) bool {
	if ax, ay := b.Active(x), b.Active(y); ax != ay {
		return ax
	}
	if px, py := PriorityRank(x.Str("priority")), PriorityRank(y.Str("priority")); px != py {
		return px < py
	}
	return b.Touched(x) > b.Touched(y)
}

// Touched is the time of a document's last event, or its creation.
func (b *Board) Touched(d *doc.Doc) string {
	if e := b.LastEvent(d); e != nil {
		return e.Str("at")
	}
	return d.Str("created")
}

// PriorityRank is a priority's place, highest first; normal when unset.
func PriorityRank(p string) int {
	if p == "" {
		p = "normal"
	}
	if i := slices.Index(Priorities, p); i >= 0 {
		return i
	}
	return len(Priorities)
}

// PartView is a part of a Work View.
type PartView struct {
	Ref     vault.Ref `json:"ref"`
	Depends []string  `json:"depends"`
	Ready   bool      `json:"ready"`
}

// View is a stub or a spec with everything around it.
type View struct {
	Doc        vault.Ref   `json:"doc"`
	Root       *vault.Ref  `json:"root,omitempty"`
	Ancestors  []vault.Ref `json:"ancestors"`
	Parts      []PartView  `json:"parts"`
	Events     []vault.Ref `json:"events"`
	Result     *vault.Ref  `json:"result,omitempty"`
	From       *vault.Ref  `json:"from,omitempty"`
	Became     []vault.Ref `json:"became"`
	Implements []vault.Ref `json:"implements"`
	Sessions   []vault.Ref `json:"sessions"`
	Changes    []vault.Ref `json:"changes"`
	Next       string      `json:"next"`
}

// View builds the view of a stub or a spec.
func (b *Board) View(d *doc.Doc) *View {
	idx := b.Idx
	v := &View{Doc: b.Ref(d), Ancestors: []vault.Ref{}, Parts: []PartView{}, Events: []vault.Ref{}, Became: []vault.Ref{}, Implements: []vault.Ref{}, Sessions: []vault.Ref{}, Changes: []vault.Ref{}, Next: b.Next(d)}
	if d.Type() == "spec" && d.Str("kind") == Plan {
		r := b.Ref(b.Root(d))
		v.Root = &r
		anc := b.Ancestors(d)
		for i := len(anc) - 1; i >= 0; i-- {
			v.Ancestors = append(v.Ancestors, b.Ref(anc[i]))
		}
		for _, p := range b.parts[d.ID()] {
			pv := PartView{Ref: b.Ref(p), Depends: []string{}, Ready: b.Ready(p)}
			for _, dep := range p.List("depends") {
				if s := idx.Linked(dep); s != nil {
					pv.Depends = append(pv.Depends, s.ID())
				}
			}
			v.Parts = append(v.Parts, pv)
		}
	}
	list := b.events[d.ID()]
	for i := len(list) - 1; i >= 0; i-- {
		v.Events = append(v.Events, idx.Ref(list[i]))
	}
	if r := b.Result(d); r != nil {
		ref := idx.Ref(r)
		v.Result = &ref
	}
	if f := idx.Linked(d.Str("from")); f != nil {
		ref := idx.Ref(f)
		v.From = &ref
	}
	v.Became = idx.Refs(idx.LinkedAll(d.List("became")))
	v.Implements = idx.Refs(idx.LinkedAll(d.List("implements")))
	var sess []*doc.Doc
	for _, s := range idx.Of("session") {
		for _, field := range []string{"specs", "work"} {
			if slices.ContainsFunc(s.List(field), func(l string) bool { return strings.EqualFold(doc.LinkTarget(l), d.Title()) }) {
				sess = append(sess, s)
				break
			}
		}
	}
	sort.SliceStable(sess, func(i, j int) bool { return sess[i].Str("started") > sess[j].Str("started") })
	v.Sessions = idx.Refs(sess)
	for _, c := range idx.Of("change") {
		if strings.EqualFold(doc.LinkTarget(c.Str("work")), d.Title()) {
			v.Changes = append(v.Changes, idx.Ref(c))
		}
	}
	return v
}

// Ref is the index's reference to a document, with the state this board derives, so a
// view is current before sync has written the fields.
func (b *Board) Ref(d *doc.Doc) vault.Ref {
	r := b.Idx.Ref(d)
	switch d.Type() {
	case "stub", "spec":
		r.Status = b.Status(d)
	}
	if d.Type() == "spec" && d.Str("kind") == Plan {
		if r.State == nil {
			r.State = map[string]any{}
		}
		r.State["blocked"] = b.Blocked(d)
		r.State["active"] = b.Active(d)
		r.State["parts"] = b.PartsText(d)
		r.State["ready"] = b.Ready(d)
		r.State["root"] = b.Root(d).Title()
	}
	return r
}

// BoardView is the board: the work that is open, by what it waits on.
type BoardView struct {
	Active  []vault.Ref `json:"active"`
	Started []vault.Ref `json:"started"`
	Ready   []vault.Ref `json:"ready"`
	Blocked []vault.Ref `json:"blocked"`
	Waiting []vault.Ref `json:"waiting"`
	Stubs   []vault.Ref `json:"stubs"`
	Done    []vault.Ref `json:"done"`
}

// MaxDone is how many closed plans the board lists.
const MaxDone = 10

// Filter keeps the documents a board or a list shows.
type Filter struct {
	Tags       []string `json:"tags,omitempty" jsonschema:"only work that holds every one of these tags"`
	Repository string   `json:"repository,omitempty" jsonschema:"only plans that name this repository, by id or title"`
}

func (b *Board) keep(d *doc.Doc, f Filter) bool {
	if len(f.Tags) > 0 && !vault.Holds(d, f.Tags...) {
		return false
	}
	if f.Repository != "" {
		r, err := b.Idx.ResolveType(f.Repository, "repository")
		if err != nil {
			return false
		}
		if !slices.ContainsFunc(d.List("repositories"), func(l string) bool { return strings.EqualFold(doc.LinkTarget(l), r.Title()) }) {
			return false
		}
	}
	return true
}

// BoardView lists the open work: active plans, started plans, ready plans, blocked plans,
// the plans that wait on others, the open stubs; and the last ten closed plans.
func (b *Board) BoardView(f Filter) *BoardView {
	out := &BoardView{Active: []vault.Ref{}, Started: []vault.Ref{}, Ready: []vault.Ref{}, Blocked: []vault.Ref{}, Waiting: []vault.Ref{}, Stubs: []vault.Ref{}, Done: []vault.Ref{}}
	var plans, closed []*doc.Doc
	for _, s := range b.Specs {
		if s.Str("kind") != Plan || !b.keep(s, f) {
			continue
		}
		if Closed(b.Status(s)) {
			closed = append(closed, s)
		} else {
			plans = append(plans, s)
		}
	}
	sort.SliceStable(plans, func(i, j int) bool { return b.Less(plans[i], plans[j]) })
	for _, p := range plans {
		ref := b.Ref(p)
		switch {
		case b.Active(p):
			out.Active = append(out.Active, ref)
		case b.Blocked(p) != "":
			out.Blocked = append(out.Blocked, ref)
		case b.Status(p) == Started:
			out.Started = append(out.Started, ref)
		case b.Ready(p):
			out.Ready = append(out.Ready, ref)
		default:
			out.Waiting = append(out.Waiting, ref)
		}
	}
	var stubs []*doc.Doc
	for _, s := range b.Stubs {
		if b.Status(s) == Open && b.keep(s, f) && f.Repository == "" {
			stubs = append(stubs, s)
		}
	}
	sort.SliceStable(stubs, func(i, j int) bool { return b.Less(stubs[i], stubs[j]) })
	out.Stubs = b.refs(stubs)
	sort.SliceStable(closed, func(i, j int) bool { return b.Touched(closed[i]) > b.Touched(closed[j]) })
	if len(closed) > MaxDone {
		closed = closed[:MaxDone]
	}
	out.Done = b.refs(closed)
	return out
}

func (b *Board) refs(ds []*doc.Doc) []vault.Ref {
	out := make([]vault.Ref, 0, len(ds))
	for _, d := range ds {
		out = append(out, b.Ref(d))
	}
	return out
}
