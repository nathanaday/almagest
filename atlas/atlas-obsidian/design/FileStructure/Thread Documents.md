
# Thread Documents

A thread is one line of work on one or more repositories: a feature, a fix, a chore. Every change an agent makes to a repository belongs to a thread. A hook refuses an edit inside a linked repository while the session has no thread ([[Hooks#guard]]). Questions and exploration need no thread.

A thread is a set of documents, one per stage:

| Document | Answers | How many |
|---|---|---|
| **stub** | What is this about? The first words, loose, in the user's words. | one; it is the thread's root |
| **spec** | What is true when it is done, and why? | zero or one |
| **task** | One piece of the work: what, where, how to verify, and then the result. | zero or more |
| **receipt** | How did it end: completed, or killed? | zero or one |

## The stage is the furthest document

Nothing sets a thread's stage. Code derives it from the documents that exist:

| Documents | `stage` |
|---|---|
| a stub | `stub` |
| a spec | `spec` |
| one task or more | `tasks` |
| a receipt | `closed` |

Kept from V1, because it holds three things. The documents are always written, since there is nothing else to move. No status field can disagree with the files. A hand edit works: delete the spec in Obsidian, and the next sync moves the thread back.

A stage may be skipped. A small fix goes from stub to one task. A killed idea goes from stub to receipt.

## Layout

```text
threads/
├── Threads.base
└── Filter vehicle false alarms/
    ├── Filter vehicle false alarms.md                    stub
    ├── Filter vehicle false alarms — Spec.md
    ├── Filter vehicle false alarms — T1 Score boxes by motion.md
    ├── Filter vehicle false alarms — T2 Tune the threshold.md
    └── Filter vehicle false alarms — Receipt.md
```

- One folder per thread, named by the thread's title. Closed threads stay in place. `Threads.base` filters them out of the open views, and no link breaks.
- A document names its thread by a link to the stub (`thread: "[[Filter vehicle false alarms]]"`). Code finds a thread's documents by that link and by the `thread_id` field, never by the file name, so a file renamed by hand is still found.
- Renaming a thread (`thread` set title) renames the folder and every file, and rewrites every link to them, in one commit.

## Stub

The root of the thread. It carries the thread's state, so there is no separate card.

```yaml
---
id: thr-c7v2kq
type: stub
created: 2026-09-27
updated: 2026-09-27
scope: ["[[p3-edge]]"]              # the repositories and areas the work touches
priority: normal                    # high | normal | low | someday
blocked: ""                         # what the thread waits on, in one line
# owned by code:
stage: tasks                        # stub | spec | tasks | closed
outcome: ""                         # completed | killed, from the receipt
active: true                        # a running session works on this thread
tasks: "1/2"                        # done over total, dropped tasks left out
---
```

Body:

1. The lead callout, owned by code (see [[#Lead callouts]]).
2. `## Stub`: the user's words, kept as the user gave them.
3. `## Notes`: the trailer. Notes, references, concerns, links. It is yours, and the model adds to it only when you ask.

## Spec

```yaml
---
id: spc-3n8wpa
type: spec
thread: "[[Filter vehicle false alarms]]"
thread_id: thr-c7v2kq
created: 2026-09-27
updated: 2026-09-27
---
```

Body sections: `## Goal`, `## Done when` (a list that a reviewer can check), `## Decisions` (each with its reason), `## Out of scope`, `## Conventions` (the policies that apply, each linked with one line on why), `## Open questions`.

## Task

```yaml
---
id: tsk-9d4mzt
type: task
thread: "[[Filter vehicle false alarms]]"
thread_id: thr-c7v2kq
created: 2026-09-27
updated: 2026-09-27
order: 1
repository: "[[p3-edge]]"           # one repository; work in two repositories is two tasks
depends: []                         # other tasks of this thread that must be done first
status: open                        # open | done | dropped, set by the thread tool
# owned by code:
active: false                       # a running session works on this task
---
```

Body sections: `## What`, `## Where` (files, modules, interfaces), `## Conventions` (policy links), `## Verify` (the commands or checks that prove it works), `## Progress` (dated lines, added as the work goes), `## Result` (written by `thread` when the task is done: what changed, the commits, how it was verified).

A task is the unit a session works on. Two tasks with no dependency between them can run in two sessions at once.

## Receipt

```yaml
---
id: rcp-5k2hwe
type: receipt
thread: "[[Filter vehicle false alarms]]"
thread_id: thr-c7v2kq
created: 2026-09-28
updated: 2026-09-28
outcome: completed                  # completed | killed
---
```

Body sections: `## Delivered`, `## Verified`, `## Follow-ups` (links to new stubs), `## Learned` (what the wiki should absorb; [[wiki-sync]] reads it first). A killed receipt holds `## Why killed` in place of the first two.

## Rules the thread tool enforces

- A thread has at most one spec and one receipt.
- A `completed` receipt is refused while a task is `open`. A `killed` receipt sets every open task to `dropped`.
- A closed thread takes no new document until `thread` reopen, which removes the receipt. Git keeps the old one.
- `depends` names tasks of the same thread, and never makes a loop.
- `scope` and `repository` name scope pages.
- A new document is created only by `thread`. The guard refuses a Write that creates a file under `threads/`. Edit on the prose of a document that exists is allowed.

## Lead callouts

The first callout of every thread document is code's. It gives the page its stage's color and icon, and it leads to every other document of the thread.

```markdown
> [!spec] Filter vehicle false alarms
> [[Filter vehicle false alarms|Stub]] → **Spec** → Tasks 1/2 → Receipt
> `thr-c7v2kq` · [[p3-edge]] · high · active in [[2026-09-27 1432 a1b2c3]]
```

A task's callout shows its order, status, repository, and dependencies. Sync replaces only a leading callout of the types `stub`, `spec`, `task`, `receipt`, or `killed`. Every other callout is the user's.

## Sync

`threads.Sync` makes every derived part agree with the documents: the stub's `stage`, `outcome`, `active`, and `tasks`; each task's `active`; each lead callout. It writes a file only when its content differs and never changes `updated`. Every `thread` call ends in it, the session-start hook runs it, and the Obsidian plugin runs it when a thread document changes.

`active` comes from the session documents. A task is active while it is `open` and a session with status `running` or `waiting` lists it in `tasks`. A thread is active while such a session lists it in `threads`.

## Threads.base

The board, shipped by `init`. Views: open threads grouped by stage, the most advanced first; active now (with the session); blocked; tasks by status; closed, with the outcome. Obsidian renders it live, so the board needs no generated page.
