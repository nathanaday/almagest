# Threads and chords

A **thread** is one piece of work, from idea to closed. It is a set of linked documents,
and each document answers one question:

| Document | Answers | Written by |
|---|---|---|
| stub | What is the idea? The front page of the thread: the user's words, and what code derives. | `thread` stub |
| spec | What must be true when the work is done, and why? | `thread` spec |
| task list | What are the steps that take one repository to the spec? | `thread` tasks, `thread` check |
| verification | Does the done work meet each requirement? What did the check find? | `thread` verify, `thread` finding |
| event | What did someone decide about the thread, and when? | code, in the commit of the call that caused it |

A **chord** is a goal that needs several threads, with the order between them
([chords](#chords)).

The thread's id is its stub's id. Every `thread` action takes the id or the title of the
stub, or of any document of the thread.

Code names the documents from the stub's title, and a rename of the stub renames them
all:

```
Score boxes by motion.md                       the stub
Score boxes by motion · Spec.md
Score boxes by motion · Tasks (p3-edge).md     one list per repository
Score boxes by motion · Tasks.md               the list for work in no repository
Score boxes by motion · Verification 1.md      one per round
```

## One document, one job

A thread document holds only the sections of its type. The tool and the guard refuse
any other level-two heading, so do not add one. This table says where each kind of
content goes:

| Content | Goes to |
|---|---|
| what must be true, a rule, a decision with its reason | the spec |
| a step to do, how to do it, where | a task and its details |
| what was done | `thread` check on the task, with the commits |
| what a check found | a finding of a verification |
| the story of a session: what you tried, where you stopped | `## Progress` in your session document |
| a fact about a tool, a dataset, a repository, a method | the wiki, through a change ([wiki-save](../../wiki-save/SKILL.md)) |
| a short remark the thread should keep | `thread` note |
| an idea for other work | a new stub (`thread` stub) |

## Stub

The stub is the front page, and it stays short.

- You give `text` (the user's words, as given), `title`, `description`, `tags`,
  `priority` (high, normal, low, someday), `chord`, and `after` (the threads that must be
  verified first).
- Sections: `## Idea` (the user's words; never rewrite them), `## Thread` (code's: the
  documents of the thread with their counts, the wiki pages the spec cites, the
  sessions, and the hand-off line), `## Notes` (the user's; add to it only when asked).
- Code owns the lead callout. It says the status, the task count, what is missing, and
  the next step.
- The guard refuses an edit that adds a section, or that takes `## Idea` and `## Notes`
  past 40 lines together.
- A stub with no spec may become other documents: a topic through `change` with
  `{op: promote}`; a source through `source` capture with `resolves`; several documents
  through `thread` resolve with `became`.

## Spec

The spec says what, and why. It holds no task, no method, and no progress.

Sections, in order:

- `## Goal`: one paragraph on what the work is for.
- `## Requirements`: one line per requirement, each written `- R1: …`. A reviewer must
  be able to check each one: it names what is true, and how one can see it. An id never
  changes, and a removed requirement's id is not used again.
- `## Rules`: the business rules and principles that bind the work, and each policy
  that applies, linked, with one line on why ([conventions.md](conventions.md)).
- `## Decisions`: each decision, with its reason.
- `## Out of scope`.
- `## Knowledge`: the wiki pages the spec relied on, each linked, with one line on what
  it gave. The stub lists every topic, source, and repository the spec links.
- `## Open questions`.

Code owns `status`: `not implemented`, `complete (unverified)`, `complete (verified)`.
Level-three headings inside a section are yours to use. A check box is refused.

## Task list

One list per repository the thread touches. A thread that changes no repository
(research, writing, a decision) has one list with no repository.

```markdown
## Tasks

- [x] T1: Export each cell to TFLite INT8 (R1) · a3f9c21 · exported with the INT8 recipe
- [ ] T2: Score each artifact on the test split (R1, R2)
- [-] T3: Retrain with the old recipe (R1) · dropped: T6 replaces it

## Details

### T2

Where, how, and what to watch for.
```

- `thread` tasks writes the list, or appends to the list of that repository. Code gives
  each task its id; the ids run on across the lists of a thread. Each task names the
  requirements it serves.
- `thread` check marks one task: `state: done` (the default) with `commits`, or with
  `note` when the work made no commit; `state: dropped` with `reason`; `state: open` to
  take it up again. The guard refuses an Edit of `## Tasks`.
- `## Details` is yours to edit with Edit.
- The user checks or opens a box in Obsidian by hand. The next sync reads it.

## Verification

A verification is the record of one round. A read-only agent,
[thread-audit](../../../agents/thread-audit.md), checks the work; the skill files its
report with `thread` verify.

- `## Scope`: what was checked, each repository with its commits.
- `## Requirements`: one row per requirement of the spec: `pass` or `fail`, with the
  evidence (the command and its output, the file and line, the commit). The tool
  refuses a report that skips a requirement, or a result with no evidence.
- `## Findings`: what the check found that the spec or the tasks did not foresee, one
  line each. A failed requirement needs a finding that says what is wrong.
- `## Notes`: anything else the round should record. Yours to edit.

Each finding gets one outcome, with `thread` finding:

| Outcome | Give | What happens |
|---|---|---|
| `task` | `new_task`, `repository` | A task joins the list. The thread returns to `started`. |
| `spec` | nothing; revise the spec first, after the user's yes | The verification goes stale, and the thread needs a new round. |
| `stub` | `link` (a stub that exists) or `text` (a new one) | The finding is work for another thread. |
| `knowledge` | `link`: the change or the topic that holds it | The wiki takes it. Propose the change first. |
| `accepted` | `reason`, in the user's words | The user accepts it as it is. Ask first. |

A verification records a hash of the spec's requirements and rules, and of the task ids.
When either changes, it is `stale`, and only a new round verifies the thread.

## Status is derived

No call sets a status. Code derives the thread's status at every sync; the first row
that fits decides.

| Status | When |
|---|---|
| `dropped` | the last drop, reopen, or resolve event is a drop |
| `resolved` | the stub became other documents |
| `closed` | `verified`, and an applied change absorbed the spec and the last verification |
| `verified` | every task is done or dropped, and the last verification is current and passes: every requirement passes, and no finding is open |
| `unverified` | every task is done or dropped, and no current verification passes |
| `started` | work began (`thread` start, or a done task), and a task is open |
| `planned` | a task list exists |
| `specified` | a spec with one requirement or more exists |
| `stub` | none of the above |

- A thread **closes with no call**: when it is verified, propose the change that absorbs
  its spec and its verification ([thread-close](../../thread-close/SKILL.md)); the user's
  apply closes it. The change tool refuses that apply until the user has answered.
- New work on a closed thread needs no reopen: a new task or a revised spec takes it
  back to `started` or `unverified`.
- `blocked` and `active` (a live session started it) show beside the status.
- The stub's lead says what is missing. Read it before you say a thread is done.

## The thread tool

| Action | Takes | Does |
|---|---|---|
| list | optional `tags`, `repository`, `chord` | the board: `active`, `started`, `verified`, `ready`, `blocked`, `waiting`, `stubs`, the last ten `ended`, and the open `chords` |
| load | `thread` | everything to take the thread up, in one read (below) |
| stub | `text`, `title`, `description`, `tags`, `priority`, `chord`, `after`, `inbox`, `new_tags` | the stub |
| spec | `thread`, `text`, `description` | the spec, or the new text of the one it has |
| tasks | `thread`, `repository`, `tasks: [{text, requirements, details}]` | a task list for the repository, or more tasks in it |
| start | `thread`, optional `take` | binds this session to the thread; a `started` or `continued` event |
| check | `thread`, `task`, `state`, `commits`, `note`, `reason` | marks one task done, dropped, or open |
| verify | `thread`, `scope`, `results: [{requirement, result, evidence}]`, `findings`, `notes` | one verification, as the next round |
| finding | `thread`, `finding`, `outcome`, and what the outcome needs | closes one finding of the last round |
| drop | `thread`, `reason` | a `dropped` event |
| reopen | `thread`, optional `reason` | takes up a dropped or resolved thread |
| block | `thread`, `reason` (one line) | a `blocked` event |
| unblock | `thread` | an `unblocked` event |
| resolve | `thread`, `became` | closes a stub with no spec as the documents it became |
| note | `thread` or `doc`, `text` | a `note` event |
| set | `set: {doc, title, description, tags, aliases, priority, chord, after, new_tags}` | the fields given; a new title renames every document of the thread |

`thread` load returns: the stub and its idea; `missing` and `next` (the step, the skill
that does it, the reason); the spec's text and its requirements; each task list with
its repository and path, its open tasks with their details, and its closed tasks as
lines; the last verification with the requirements that fail and the open findings; the
thread's place (its chord, the threads before and after it, whether it is ready); the
wiki pages the spec cites; the last notes; the last sessions with their progress lines;
the last events; and the changes that served it. Read a cited page with Read when the
next step needs it.

Every write returns `state`: the thread, `missing`, and `next`. Follow `next`.

The tool refuses, and each refusal names the call that would succeed:

- `start` on a thread with no spec, with no task list, with every task done, that comes
  after a thread not yet verified, or that another live session holds (unless
  `take: true`);
- `check` on a thread no one started; a done task with no commit and no note; a dropped
  task with no reason;
- `tasks` before the spec; a task that names no requirement, or one the spec lacks;
- `verify` while a task is open; a report that skips a requirement; a failed
  requirement with no finding;
- `spec` text with a section a spec does not hold, a check box, no goal, or a
  requirement line with no id;
- an `after` that makes the threads wait on each other.

## The edit rule

The guard refuses an edit inside a linked repository R unless this session started a
thread that has a task list for R with an open task.

- Only `thread` start binds a session. One start per session per thread; after it, go on
  with the tasks.
- When the last task of the list is checked, the edits end. A fix needs a new task
  first: `thread` tasks, or `thread` finding with the outcome `task`.
- A subagent that may write inherits its parent's started threads.
- Change a repository's files with Edit and Write, not with the shell (`cat >`,
  `sed -i`). The edit rule and the session's record see the edit tools. Use the shell to
  run commands: tests, builds, git.

## Edit, never Write

A thread document comes only from the `thread` tool or the `chord` tool; the guard
refuses a Write in `wiki/documents/`. After a document exists, revise its prose with
Edit. Never edit its frontmatter, its lead callout, or a section that is code's
(`## Thread` of a stub, `## Tasks` of a task list, `## Scope`, `## Requirements`, and
`## Findings` of a verification, `## Threads` of a chord). Change fields with `thread`
set.

## Chords

A chord is one document: its `## Goal` (what is true when every thread is closed), and
`## Threads` (code's: the threads in order, with each one's status).

- A thread joins with `chord` on its stub, and takes its place with `after`. The order
  is a graph: a thread may come after several, and several may come after one. A thread
  belongs to one chord at most.
- A thread is **ready** when every thread it comes after is verified, closed, dropped,
  or resolved. `thread` start refuses a thread that is not ready.
- Code derives the chord's status: `open`, `started`, `done` (every thread closed or
  dropped), `closed` (done, and an applied change absorbed the chord), `dropped`.
- Code writes a canvas per chord, `chords/<title>.canvas`: a card per thread, an arrow
  per `after`. The user redraws it in Obsidian and saves the order. Never edit it.

| `chord` action | Takes | Does |
|---|---|---|
| list | optional `tags` | the open chords, each with its threads in order |
| load | `chord` | the chord, its goal, its threads in order, the ready ones, `next` |
| create | `title`, `text` (the goal), `description`, `tags`, `priority`, `threads: [{title, text, after} or {thread, after}]` | the chord and its stubs, in one commit; `after` may name a thread of the same call by title |
| add | `chord`, `thread`, optional `after` | a stub joins |
| remove | `chord`, `thread` | a thread leaves; it stays a thread |
| order | `chord`, `order: [{thread, after}]` | sets what each named thread comes after |
| drop | `chord`, `reason` | drops the chord and each of its threads that is not ended |
| reopen | `chord` | takes it up again, with the threads dropped with it |
| set | `set: {doc, title, description, tags, aliases, priority}` | the fields given |

## Events

Code writes every event. You never create one.

| Kind | About | Written by | Holds |
|---|---|---|---|
| `started`, `continued` | thread | `thread` start | nothing |
| `dropped` | thread, chord | drop | `## Why` |
| `reopened` | thread, chord | reopen | `## Why`, when given |
| `blocked`, `unblocked` | thread | block, unblock | the one line |
| `resolved` | stub | `thread` resolve; `source` capture with `resolves` | what it became |
| `promoted` | the topic a stub became | `change` apply of a promote | nothing |
| `note` | any document | `thread` note | `## Note` |

## The session document

The opening context names this session's document, `sessions/<month>/<date time id>.md`.
Hooks keep its frontmatter, its first callout, and `## Subagents`. You write three
sections with Edit: `## Description` (one line on what the session works on),
`## Progress` (dated lines: what you did, what you tried, where you stopped), and
`## Summary` (at the end). The story of the work goes here, not into a thread document.
