// Package thread keeps the threads and the chords. A thread is one piece of work, from
// idea to closed: a stub (its front page), a spec (what must be true), one or more task
// lists (the steps, as check boxes), and one or more verifications (the work checked
// against the spec). A chord is a goal that needs several threads, in an order. Nothing
// sets a status: code derives it from the check boxes, the verification results, the
// events, and the applied changes, and Sync writes it into the documents.
package thread

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/nathanaday/atlas-obsidian/internal/doc"
	"github.com/nathanaday/atlas-obsidian/internal/vault"
)

// Statuses of a thread, on its stub.
const (
	StatusStub = "stub"
	Specified  = "specified"
	Planned    = "planned"
	Started    = "started"
	Unverified = "unverified"
	Verified   = "verified"
	Closed     = "closed"
	Dropped    = "dropped"
	Resolved   = "resolved"
)

// Statuses of a spec.
const (
	NotImplemented     = "not implemented"
	CompleteUnverified = "complete (unverified)"
	CompleteVerified   = "complete (verified)"
)

// Verdicts of a verification.
const (
	Pass         = "pass"
	Fail         = "fail"
	OpenFindings = "findings"
	Stale        = "stale"
)

// Statuses of a chord.
const (
	ChordOpen    = "open"
	ChordStarted = "started"
	ChordDone    = "done"
	ChordClosed  = "closed"
	ChordDropped = "dropped"
)

// Trailer names the document a thread commit wrote.
const Trailer = "Atlas-Thread"

// Priorities, highest first.
var Priorities = []string{"high", "normal", "low", "someday"}

// liveStatuses are the session statuses that hold the threads a session started.
var liveStatuses = []string{"running", "waiting", "idle"}

// Thread is a stub and the documents that name it.
type Thread struct {
	Stub *doc.Doc
	Spec *doc.Doc
	// Lists are the task lists, by title.
	Lists []*doc.Doc
	// Rounds are the verifications, the first round first.
	Rounds []*doc.Doc
}

// Board is every thread, chord, and event of a vault, read from one index.
type Board struct {
	Idx *vault.Index
	// Expect, when set, hears the bytes Sync read from a file the index does not hold,
	// before it writes that file (SyncWith sets it to a guard's).
	Expect  func(rel string, raw []byte)
	Stubs   []*doc.Doc
	Chords  []*doc.Doc
	threads map[string]*Thread
	// Extra are the specs past the first that name one thread; lint reports them.
	Extra   []*doc.Doc
	events  map[string][]*doc.Doc // subject id → its events, oldest first
	live    map[string][]*doc.Doc // stub title, lower case → the live sessions that started it
	members map[string][]*doc.Doc // chord id → its stubs
	status  map[string]string
	rank    map[string]int
	// canvas is the hash of the graph Sync wrote to each chord's canvas, by chord id.
	canvas map[string]string
	// ForceCanvas names the chords whose canvas Sync writes from the stubs even when the
	// user changed it; TidyCanvas names those whose cards it places again.
	ForceCanvas map[string]bool
	TidyCanvas  map[string]bool
	// Joined names the stubs that joined a chord in this write. Each gets its card on the
	// canvas even while the user's drawing there is not saved.
	Joined map[string]bool
}

// Load reads the threads of an index.
func Load(idx *vault.Index) *Board {
	b := &Board{Idx: idx, threads: map[string]*Thread{}, events: map[string][]*doc.Doc{}, live: map[string][]*doc.Doc{}, members: map[string][]*doc.Doc{}, status: map[string]string{}, rank: map[string]int{}, canvas: map[string]string{}}
	b.Stubs = idx.Of("stub")
	b.Chords = idx.Of("chord")
	for _, s := range b.Stubs {
		b.threads[s.ID()] = &Thread{Stub: s}
		if c := idx.Linked(s.Str("chord")); c != nil && c.Type() == "chord" {
			b.members[c.ID()] = append(b.members[c.ID()], s)
		}
	}
	for _, d := range idx.Of("spec", "tasks", "verification") {
		s := idx.Linked(d.Str("thread"))
		if s == nil || s.Type() != "stub" {
			continue
		}
		t := b.threads[s.ID()]
		switch d.Type() {
		case "spec":
			switch {
			case t.Spec == nil:
				t.Spec = d
			case d.Title() == SpecTitle(s.Title()):
				// The spec the tool named is the thread's spec; any other is extra.
				b.Extra = append(b.Extra, t.Spec)
				t.Spec = d
			default:
				b.Extra = append(b.Extra, d)
			}
		case "tasks":
			t.Lists = append(t.Lists, d)
		case "verification":
			t.Rounds = append(t.Rounds, d)
		}
	}
	for _, t := range b.threads {
		sort.SliceStable(t.Rounds, func(i, j int) bool {
			if a, c := t.Rounds[i].Front.Int("round"), t.Rounds[j].Front.Int("round"); a != c {
				return a < c
			}
			return t.Rounds[i].Path < t.Rounds[j].Path
		})
	}
	for _, e := range idx.Of("event") {
		if s := b.Subject(e); s != nil {
			b.events[s.ID()] = append(b.events[s.ID()], e)
		}
	}
	for id := range b.events {
		sortEvents(b.events[id])
	}
	for _, s := range idx.Of("session") {
		if !slices.Contains(liveStatuses, s.Str("status")) {
			continue
		}
		// A session of 7.x holds the plans it started in specs; a plan became a stub.
		for _, field := range []string{"threads", "specs"} {
			for _, l := range s.List(field) {
				k := strings.ToLower(doc.LinkTarget(l))
				if !slices.Contains(b.live[k], s) {
					b.live[k] = append(b.live[k], s)
				}
			}
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

// Subject is the document an event is about: by its subject_id, else by its link.
func (b *Board) Subject(e *doc.Doc) *doc.Doc {
	if d := b.Idx.ByID(e.Str("subject_id")); d != nil {
		return d
	}
	return b.Idx.Linked(e.Str("subject"))
}

// Events are a document's events, oldest first.
func (b *Board) Events(d *doc.Doc) []*doc.Doc { return b.events[d.ID()] }

// Thread is the thread of a stub, or of a spec, a task list, or a verification; nil for
// any other document.
func (b *Board) Thread(d *doc.Doc) *Thread {
	if d == nil {
		return nil
	}
	if d.Type() == "stub" {
		return b.threads[d.ID()]
	}
	if s := b.Idx.Linked(d.Str("thread")); s != nil {
		return b.threads[s.ID()]
	}
	return nil
}

// Docs are a thread's documents: the stub, the spec, the lists, and the rounds.
func (t *Thread) Docs() []*doc.Doc {
	out := []*doc.Doc{t.Stub}
	if t.Spec != nil {
		out = append(out, t.Spec)
	}
	out = append(out, t.Lists...)
	return append(out, t.Rounds...)
}

// Last is the thread's last verification, or nil.
func (t *Thread) Last() *doc.Doc {
	if len(t.Rounds) == 0 {
		return nil
	}
	return t.Rounds[len(t.Rounds)-1]
}

// Tasks are the tasks of every list of the thread.
func (t *Thread) Tasks() []Task {
	var out []Task
	for _, l := range t.Lists {
		out = append(out, Tasks(l)...)
	}
	return out
}

// Counts are the thread's tasks done over its tasks, dropped ones left out.
func (t *Thread) Counts() (done, total int) {
	for _, l := range t.Lists {
		d, n := ListCounts(l)
		done, total = done+d, total+n
	}
	return
}

// CountsText is what the tasks field holds: "7/12", or "" for a thread with no task.
func (t *Thread) CountsText() string {
	if len(t.Tasks()) == 0 {
		return ""
	}
	done, total := t.Counts()
	return fmt.Sprintf("%d/%d", done, total)
}

// ListCounts are one list's tasks done over its tasks, dropped ones left out.
func ListCounts(list *doc.Doc) (done, total int) {
	for _, task := range Tasks(list) {
		switch task.State {
		case TaskDone:
			done++
			total++
		case TaskOpen:
			total++
		}
	}
	return
}

// Repositories are the repositories the thread's task lists name, as links, in order.
func (t *Thread) Repositories() []string {
	out := []string{}
	for _, l := range t.Lists {
		if r := l.Str("repository"); r != "" && !slices.Contains(out, r) {
			out = append(out, r)
		}
	}
	return out
}

// Verdict is a verification's verdict: stale when the spec or the tasks changed after
// it; fail when a requirement fails or has no row; findings when one is open; else pass.
func (b *Board) Verdict(t *Thread, v *doc.Doc) string {
	if v.Str("spec_hash") != SpecHash(t.Spec) || v.Str("tasks_hash") != TasksHash(t.Lists) {
		return Stale
	}
	if len(b.failed(t, v)) > 0 {
		return Fail
	}
	if openFindings(v) > 0 {
		return OpenFindings
	}
	return Pass
}

// failed are the requirements of the spec that a verification does not pass.
func (b *Board) failed(t *Thread, v *doc.Doc) []string {
	if t.Spec == nil {
		return nil
	}
	rows := map[string]string{}
	for _, r := range Rows(v) {
		rows[r.Requirement] = r.Result
	}
	var out []string
	for _, r := range Requirements(t.Spec.Body) {
		if rows[r.ID] != Pass {
			out = append(out, r.ID)
		}
	}
	return out
}

func openFindings(v *doc.Doc) int {
	n := 0
	for _, f := range Findings(v) {
		if f.Open {
			n++
		}
	}
	return n
}

// lifecycle are the event kinds that end a thread or take it up again.
var lifecycle = map[string]string{"dropped": Dropped, "resolved": Resolved, "reopened": ""}

// started reports whether work on the thread began: a started event, or a done task.
func (b *Board) started(t *Thread) bool {
	for _, e := range b.events[t.Stub.ID()] {
		if k := e.Str("kind"); k == "started" || k == "continued" {
			return true
		}
	}
	done, _ := t.Counts()
	return done > 0
}

// absorbed reports whether an applied change absorbed the thread's spec and its last
// verification, as they are now. A type the vault does not wikify needs no change.
func (b *Board) absorbed(t *Thread) bool {
	wikify := b.Idx.V.Wikify()
	for _, d := range []*doc.Doc{t.Spec, t.Last()} {
		if d == nil || !slices.Contains(wikify, d.Type()) {
			continue
		}
		if b.Idx.AbsorbedBy(d) == nil {
			return false
		}
	}
	return true
}

// Status is a document's status, derived. A stub's is its thread's; a spec's follows its
// thread; a chord's follows its threads.
func (b *Board) Status(d *doc.Doc) string {
	if s, ok := b.status[d.ID()]; ok {
		return s
	}
	s := b.derive(d)
	b.status[d.ID()] = s
	return s
}

func (b *Board) derive(d *doc.Doc) string {
	switch d.Type() {
	case "chord":
		return b.chordStatus(d)
	case "spec":
		t := b.Thread(d)
		if t == nil {
			return NotImplemented
		}
		switch b.Status(t.Stub) {
		case Verified, Closed:
			return CompleteVerified
		case Unverified:
			return CompleteUnverified
		}
		return NotImplemented
	case "stub":
	default:
		return d.Str("status")
	}
	t := b.threads[d.ID()]
	ended := ""
	for _, e := range b.events[d.ID()] {
		if s, ok := lifecycle[e.Str("kind")]; ok {
			ended = s
		}
	}
	if ended != "" {
		return ended
	}
	if t.Spec == nil || len(Requirements(t.Spec.Body)) == 0 {
		return StatusStub
	}
	done, total := t.Counts()
	switch {
	case len(t.Tasks()) == 0:
		return Specified
	case done < total:
		if b.started(t) {
			return Started
		}
		return Planned
	}
	if last := t.Last(); last == nil || b.Verdict(t, last) != Pass {
		return Unverified
	}
	if b.absorbed(t) {
		return Closed
	}
	return Verified
}

// Ended reports whether a status ends the work: closed, dropped, or resolved.
func Ended(status string) bool { return status == Closed || status == Dropped || status == Resolved }

// Satisfied reports whether a thread with this status lets the threads after it go on.
func Satisfied(status string) bool { return status == Verified || Ended(status) }

// Blocked is the line of a document's last blocked event, until an unblocked event
// follows.
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

// Result is the event that ended a dropped or resolved document, or nil.
func (b *Board) Result(d *doc.Doc) *doc.Doc {
	want := b.Status(d)
	if want != Dropped && want != Resolved {
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

// Holders are the live sessions that started a thread.
func (b *Board) Holders(d *doc.Doc) []*doc.Doc { return b.live[strings.ToLower(d.Title())] }

// Active reports whether a live session started a thread.
func (b *Board) Active(d *doc.Doc) bool { return len(b.Holders(d)) > 0 }

// After are the threads a stub comes after: the stubs its after field names.
func (b *Board) After(d *doc.Doc) []*doc.Doc {
	var out []*doc.Doc
	for _, s := range b.Idx.LinkedAll(d.List("after")) {
		if s.Type() == "stub" && s.ID() != d.ID() {
			out = append(out, s)
		}
	}
	return out
}

// Before are the stubs that come after a stub.
func (b *Board) Before(d *doc.Doc) []*doc.Doc {
	var out []*doc.Doc
	for _, s := range b.Stubs {
		if slices.ContainsFunc(b.After(s), func(x *doc.Doc) bool { return x.ID() == d.ID() }) {
			out = append(out, s)
		}
	}
	return out
}

// Waits are the threads a stub comes after that are not yet verified, closed, dropped, or
// resolved.
func (b *Board) Waits(d *doc.Doc) []*doc.Doc {
	var out []*doc.Doc
	for _, s := range b.After(d) {
		if !Satisfied(b.Status(s)) {
			out = append(out, s)
		}
	}
	return out
}

// Ready reports whether work on a thread can go on: it is not ended, and it waits on no
// thread.
func (b *Board) Ready(d *doc.Doc) bool {
	return d.Type() == "stub" && !Ended(b.Status(d)) && len(b.Waits(d)) == 0
}

// ReadyToStart reports whether a thread that waits on no thread has not started yet: a
// chord marks it ready.
func (b *Board) ReadyToStart(d *doc.Doc) bool {
	if !b.Ready(d) || b.Blocked(d) != "" {
		return false
	}
	switch b.Status(d) {
	case StatusStub, Specified, Planned:
		return true
	}
	return false
}

// Chord is the chord a stub belongs to, or nil.
func (b *Board) Chord(d *doc.Doc) *doc.Doc {
	if c := b.Idx.Linked(d.Str("chord")); c != nil && c.Type() == "chord" {
		return c
	}
	return nil
}

// Rank is a stub's layer in the order of its chord: 0 for a thread that comes after no
// thread of the chord, else one more than the highest rank it comes after.
func (b *Board) Rank(d *doc.Doc) int {
	return b.rankOf(d, map[string]bool{})
}

func (b *Board) rankOf(d *doc.Doc, seen map[string]bool) int {
	if r, ok := b.rank[d.ID()]; ok {
		return r
	}
	if seen[d.ID()] {
		return 0
	}
	seen[d.ID()] = true
	chord := b.Chord(d)
	r := 0
	for _, a := range b.After(d) {
		if chord == nil || b.Chord(a) == nil || b.Chord(a).ID() != chord.ID() {
			continue
		}
		r = max(r, b.rankOf(a, seen)+1)
	}
	b.rank[d.ID()] = r
	return r
}

// Members are a chord's threads in order: by rank, then priority, then title.
func (b *Board) Members(chord *doc.Doc) []*doc.Doc {
	out := slices.Clone(b.members[chord.ID()])
	sort.SliceStable(out, func(i, j int) bool {
		if a, c := b.Rank(out[i]), b.Rank(out[j]); a != c {
			return a < c
		}
		if a, c := PriorityRank(out[i].Str("priority")), PriorityRank(out[j].Str("priority")); a != c {
			return a < c
		}
		return strings.ToLower(out[i].Title()) < strings.ToLower(out[j].Title())
	})
	return out
}

// ChordCounts are a chord's threads closed over its threads, the dropped and the
// resolved left out.
func (b *Board) ChordCounts(chord *doc.Doc) (closed, total int) {
	for _, s := range b.members[chord.ID()] {
		switch b.Status(s) {
		case Closed:
			closed++
			total++
		case Dropped, Resolved:
		default:
			total++
		}
	}
	return
}

// chordEnded reports whether a chord has threads and each of them has ended: closed,
// dropped, or resolved.
func (b *Board) chordEnded(chord *doc.Doc) bool {
	members := b.members[chord.ID()]
	for _, s := range members {
		switch b.Status(s) {
		case Closed, Dropped, Resolved:
		default:
			return false
		}
	}
	return len(members) > 0
}

func (b *Board) chordStatus(chord *doc.Doc) string {
	dropped := false
	for _, e := range b.events[chord.ID()] {
		switch e.Str("kind") {
		case "dropped":
			dropped = true
		case "reopened":
			dropped = false
		}
	}
	if dropped {
		return ChordDropped
	}
	if b.chordEnded(chord) {
		if slices.Contains(b.Idx.V.Wikify(), "chord") && b.Idx.AbsorbedBy(chord) == nil {
			return ChordDone
		}
		return ChordClosed
	}
	for _, s := range b.members[chord.ID()] {
		switch b.Status(s) {
		case StatusStub, Specified, Planned, Dropped, Resolved:
		default:
			return ChordStarted
		}
	}
	return ChordOpen
}

// ChordReady are a chord's threads that work can go on with now: ready, not blocked, and
// not yet verified.
func (b *Board) ChordReady(chord *doc.Doc) []*doc.Doc {
	var out []*doc.Doc
	for _, s := range b.Members(chord) {
		if b.Ready(s) && b.Status(s) != Verified && b.Blocked(s) == "" {
			out = append(out, s)
		}
	}
	return out
}

// Step is the next step of a thread or a chord, and the skill that does it.
type Step struct {
	Step   string `json:"step" jsonschema:"spec, tasks, start, run, verify, findings, close, create, work, wait, or none"`
	Skill  string `json:"skill,omitempty" jsonschema:"the skill that does the step"`
	Reason string `json:"reason"`
}

// Next is the next step of a thread.
func (b *Board) Next(d *doc.Doc) Step {
	t := b.threads[d.ID()]
	status := b.Status(d)
	if Ended(status) {
		return Step{Step: "none", Reason: "the thread is " + status}
	}
	if line := b.Blocked(d); line != "" {
		return Step{Step: "wait", Reason: "blocked: " + line}
	}
	switch status {
	case StatusStub:
		return Step{Step: "spec", Skill: "thread-spec", Reason: "the thread has no spec with requirements"}
	case Specified:
		return Step{Step: "tasks", Skill: "thread-tasks", Reason: "the spec has no task list"}
	case Verified:
		return Step{Step: "close", Skill: "thread-close", Reason: "the wiki has not absorbed the spec and the verification"}
	}
	if waits := b.Waits(d); len(waits) > 0 {
		return Step{Step: "wait", Reason: "it comes after " + titles(waits) + ", not verified yet"}
	}
	switch status {
	case Planned:
		return Step{Step: "start", Skill: "thread-run", Reason: "the task lists are written and no work has started"}
	case Started:
		for _, task := range t.Tasks() {
			if task.State == TaskOpen {
				return Step{Step: "run", Skill: "thread-run", Reason: "the next open task is " + orDefault(task.ID, task.Text) + " in " + doc.Link(task.List.Title())}
			}
		}
	}
	last := t.Last()
	if last == nil {
		return Step{Step: "verify", Skill: "thread-verify", Reason: "every task is done and no verification exists"}
	}
	switch {
	case b.Verdict(t, last) == Stale:
		return Step{Step: "verify", Skill: "thread-verify", Reason: fmt.Sprintf("round %d is stale: the spec or the tasks changed after it", last.Front.Int("round"))}
	case openFindings(last) > 0:
		return Step{Step: "findings", Skill: "thread-verify", Reason: "give each open finding of " + doc.Link(last.Title()) + " an outcome"}
	}
	return Step{Step: "findings", Skill: "thread-verify", Reason: strings.Join(b.failed(t, last), ", ") + " failed in " + doc.Link(last.Title()) + ": add a task that fixes it, or change the spec"}
}

// ChordNext is the next step of a chord.
func (b *Board) ChordNext(chord *doc.Doc) Step {
	switch b.Status(chord) {
	case ChordDropped, ChordClosed:
		return Step{Step: "none", Reason: "the chord is " + b.Status(chord)}
	case ChordDone:
		return Step{Step: "close", Skill: "chord-close", Reason: "every thread is closed or dropped, and the wiki has no overview of the chord"}
	}
	if len(b.members[chord.ID()]) == 0 {
		return Step{Step: "create", Skill: "chord-create", Reason: "the chord has no thread"}
	}
	for _, s := range b.Members(chord) {
		if b.Status(s) == Verified {
			return Step{Step: "work", Skill: "chord-work", Reason: doc.Link(s.Title()) + " is verified and waits for its wiki change"}
		}
	}
	if ready := b.ChordReady(chord); len(ready) > 0 {
		return Step{Step: "work", Skill: "chord-work", Reason: fmt.Sprintf("%d %s can go on now", len(ready), plural(len(ready), "thread", "threads"))}
	}
	return Step{Step: "wait", Reason: "every open thread is blocked, or comes after one that is"}
}

// Missing lists what stands between a thread and closed, in the order the work needs it.
func (b *Board) Missing(d *doc.Doc) []string {
	t := b.threads[d.ID()]
	status := b.Status(d)
	out := []string{}
	if Ended(status) {
		return out
	}
	for _, w := range b.Waits(d) {
		out = append(out, doc.Link(w.Title())+" first ("+b.Status(w)+")")
	}
	switch status {
	case StatusStub:
		return append(out, "a spec", "a task list", "a verification", "the wiki change")
	case Specified:
		return append(out, "a task list", "a verification", "the wiki change")
	case Verified:
		return append(out, "the wiki change that absorbs it")
	}
	for _, l := range t.Lists {
		if done, total := ListCounts(l); done < total {
			out = append(out, fmt.Sprintf("%d open %s in %s", total-done, plural(total-done, "task", "tasks"), doc.Link(l.Title())))
		}
	}
	last := t.Last()
	if last == nil {
		return append(out, "a verification", "the wiki change")
	}
	round := fmt.Sprintf("round %d", last.Front.Int("round"))
	switch b.Verdict(t, last) {
	case Stale:
		out = append(out, "a new verification ("+round+" is stale)")
	case Fail:
		out = append(out, round+" failed "+strings.Join(b.failed(t, last), ", "))
	case OpenFindings:
		n := openFindings(last)
		out = append(out, fmt.Sprintf("%d open %s in %s", n, plural(n, "finding", "findings"), doc.Link(last.Title())))
	}
	return append(out, "the wiki change")
}

// VerificationText is what a stub's verification field holds: none, or the last round and
// its verdict.
func (b *Board) VerificationText(t *Thread) string {
	last := t.Last()
	if last == nil {
		return "none"
	}
	return fmt.Sprintf("round %d: %s", last.Front.Int("round"), b.Verdict(t, last))
}

// Less orders open threads: an active one first, then by priority, then the newest
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

// Ref is the index's reference to a document, with the state this board derives, so a
// view is current before sync has written the fields.
func (b *Board) Ref(d *doc.Doc) vault.Ref {
	r := b.Idx.Ref(d)
	if r.State == nil {
		r.State = map[string]any{}
	}
	switch d.Type() {
	case "stub":
		t := b.threads[d.ID()]
		r.Status = b.Status(d)
		r.State["priority"] = orDefault(d.Str("priority"), "normal")
		r.State["tasks"] = t.CountsText()
		r.State["verification"] = b.VerificationText(t)
		r.State["blocked"] = b.Blocked(d)
		r.State["active"] = b.Active(d)
		r.State["ready"] = b.Ready(d)
		r.State["repositories"] = targets(t.Repositories())
		r.State["next"] = b.Next(d).Step
		delete(r.State, "chord")
		if c := b.Chord(d); c != nil {
			r.State["chord"] = c.Title()
		}
		delete(r.State, "after")
		if after := b.After(d); len(after) > 0 {
			r.State["after"] = titleList(after)
		}
	case "spec":
		r.Status = b.Status(d)
	case "tasks":
		done, total := ListCounts(d)
		r.Status = fmt.Sprintf("%d/%d", done, total)
	case "verification":
		if t := b.Thread(d); t != nil {
			r.Status = b.Verdict(t, d)
		}
	case "chord":
		r.Status = b.Status(d)
		closed, total := b.ChordCounts(d)
		r.State["priority"] = orDefault(d.Str("priority"), "normal")
		r.State["threads"] = fmt.Sprintf("%d/%d", closed, total)
		r.State["ready"] = titleList(b.ChordReady(d))
		r.State["next"] = b.ChordNext(d).Step
	}
	if len(r.State) == 0 {
		r.State = nil
	}
	return r
}

func (b *Board) refs(ds []*doc.Doc) []vault.Ref {
	out := make([]vault.Ref, 0, len(ds))
	for _, d := range ds {
		out = append(out, b.Ref(d))
	}
	return out
}

// ChordView is a chord with its threads in order.
type ChordView struct {
	Chord   vault.Ref   `json:"chord"`
	Threads []vault.Ref `json:"threads"`
	Next    Step        `json:"next"`
}

// ChordView builds the view of a chord.
func (b *Board) ChordView(c *doc.Doc) ChordView {
	return ChordView{Chord: b.Ref(c), Threads: b.refs(b.Members(c)), Next: b.ChordNext(c)}
}

// BoardView is the board: the threads that are not ended, by what each waits on, and the
// chords with their threads in order.
type BoardView struct {
	Active   []vault.Ref `json:"active"`
	Started  []vault.Ref `json:"started"`
	Verified []vault.Ref `json:"verified"`
	Ready    []vault.Ref `json:"ready"`
	Blocked  []vault.Ref `json:"blocked"`
	Waiting  []vault.Ref `json:"waiting"`
	Stubs    []vault.Ref `json:"stubs"`
	Ended    []vault.Ref `json:"ended"`
	Chords   []ChordView `json:"chords"`
}

// Open lists the threads of a board that are not ended, in board order.
func (v *BoardView) Open() []vault.Ref {
	var out []vault.Ref
	for _, g := range [][]vault.Ref{v.Active, v.Started, v.Verified, v.Blocked, v.Ready, v.Waiting, v.Stubs} {
		out = append(out, g...)
	}
	return out
}

// MaxEnded is how many ended threads the board lists.
const MaxEnded = 10

// Filter keeps the documents a board or a list shows.
type Filter struct {
	Tags       []string `json:"tags,omitempty" jsonschema:"only threads that hold every one of these tags"`
	Repository string   `json:"repository,omitempty" jsonschema:"only threads with a task list for this repository, by id or title"`
	Chord      string   `json:"chord,omitempty" jsonschema:"only the threads of this chord, by id or title"`
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
		if !slices.ContainsFunc(b.threads[d.ID()].Repositories(), func(l string) bool { return strings.EqualFold(doc.LinkTarget(l), r.Title()) }) {
			return false
		}
	}
	if f.Chord != "" {
		c, err := b.Idx.ResolveType(f.Chord, "chord")
		if err != nil {
			return false
		}
		if own := b.Chord(d); own == nil || own.ID() != c.ID() {
			return false
		}
	}
	return true
}

// BoardView lists the threads: active (a live session holds it), started (started or
// unverified), verified (it waits for its wiki change), ready (specified or planned, and
// it waits on no thread), blocked, waiting (it comes after a thread that is not verified),
// stubs (no spec), and the last ten ended; then the chords that are not ended.
func (b *Board) BoardView(f Filter) *BoardView {
	out := &BoardView{Active: []vault.Ref{}, Started: []vault.Ref{}, Verified: []vault.Ref{}, Ready: []vault.Ref{}, Blocked: []vault.Ref{}, Waiting: []vault.Ref{}, Stubs: []vault.Ref{}, Ended: []vault.Ref{}, Chords: []ChordView{}}
	var open, ended []*doc.Doc
	for _, s := range b.Stubs {
		if !b.keep(s, f) {
			continue
		}
		if Ended(b.Status(s)) {
			ended = append(ended, s)
		} else {
			open = append(open, s)
		}
	}
	sort.SliceStable(open, func(i, j int) bool { return b.Less(open[i], open[j]) })
	for _, s := range open {
		ref := b.Ref(s)
		status := b.Status(s)
		switch {
		case b.Active(s):
			out.Active = append(out.Active, ref)
		case b.Blocked(s) != "":
			out.Blocked = append(out.Blocked, ref)
		case status == Verified:
			out.Verified = append(out.Verified, ref)
		case len(b.Waits(s)) > 0:
			out.Waiting = append(out.Waiting, ref)
		case status == Started || status == Unverified:
			out.Started = append(out.Started, ref)
		case status == StatusStub:
			out.Stubs = append(out.Stubs, ref)
		default:
			out.Ready = append(out.Ready, ref)
		}
	}
	sort.SliceStable(ended, func(i, j int) bool { return b.EndedAt(ended[i]) > b.EndedAt(ended[j]) })
	if len(ended) > MaxEnded {
		ended = ended[:MaxEnded]
	}
	out.Ended = b.refs(ended)
	if f.Repository != "" {
		return out
	}
	for _, c := range b.Chords {
		if s := b.Status(c); s == ChordDropped || s == ChordClosed {
			continue
		}
		if len(f.Tags) > 0 && !vault.Holds(c, f.Tags...) {
			continue
		}
		if f.Chord != "" {
			if want, err := b.Idx.ResolveType(f.Chord, "chord"); err != nil || want.ID() != c.ID() {
				continue
			}
		}
		out.Chords = append(out.Chords, b.ChordView(c))
	}
	return out
}

// Closer is the applied change that closed a thread: the last one that absorbed its spec
// or its last verification. It is nil for a thread that is not closed, or that closed
// with no change because the vault wikifies neither type.
func (b *Board) Closer(d *doc.Doc) *doc.Doc {
	t := b.threads[d.ID()]
	if t == nil || b.Status(d) != Closed {
		return nil
	}
	var best *doc.Doc
	for _, x := range []*doc.Doc{t.Spec, t.Last()} {
		if x == nil {
			continue
		}
		if c := b.Idx.AbsorbedBy(x); c != nil && (best == nil || c.Str("applied") > best.Str("applied")) {
			best = c
		}
	}
	return best
}

// EndedAt is when a thread ended: its last event, or the change that closed it.
func (b *Board) EndedAt(d *doc.Doc) string {
	at := b.Touched(d)
	if c := b.Closer(d); c != nil && c.Str("applied") > at {
		at = c.Str("applied")
	}
	return at
}

func titles(ds []*doc.Doc) string {
	out := make([]string, len(ds))
	for i, d := range ds {
		out[i] = doc.Link(d.Title())
	}
	return strings.Join(out, ", ")
}

func titleList(ds []*doc.Doc) []string {
	out := make([]string, len(ds))
	for i, d := range ds {
		out[i] = d.Title()
	}
	return out
}

func targets(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		out = append(out, doc.LinkTarget(v))
	}
	return out
}
