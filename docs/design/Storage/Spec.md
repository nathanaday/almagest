# Spec

A spec says what will be true, and why. It has two kinds:

| Kind | Says | Ends |
|---|---|---|
| `plan` | a piece of work in one or more repositories: what done means, the decisions, how to verify | yes: `done` or `dropped` |
| `design` | how an important part of a repository or a system must work | no: `current` until a later design supersedes it |

A plan is the unit of work. A plan may have child plans, to any depth. A leaf plan (a plan with no children) is one piece of work in one repository that a reviewer can check alone, as a task was in 6.x. A plan may implement a design.

The `work` tool writes specs. The model and you edit their prose with Edit.

## Fields

```yaml
---
id: doc-9d4mzt
type: spec
kind: plan                          # plan | design
description: "Score detection boxes by motion to cut vehicle false alarms."
tags: [work/p3/p3-edge, ml]
aliases: []
created: 2026-09-29T15:02:10
updated: 2026-09-30T11:02:45
refreshed: 2026-09-30T11:02:45
parent: "[[Filter vehicle false alarms]]"   # plan only: the plan this is a part of
repositories: ["[[p3-edge]]"]       # the repositories the work touches, or the design describes
depends: ["[[Collect false alarm clips]]"]  # plan only: sibling plans that must be done first
order: 1                            # plan only: the place among its siblings
priority: normal                    # plan only: high | normal | low | someday
implements: []                      # plan only: the design specs this work builds
supersedes: ""                      # design only: the design spec this one replaces
from: ""                            # the stub it spawned from, when it did
# owned by code:
status: started                     # plan: open | started | done | dropped; design: current | superseded
blocked: ""                         # the text of the last blocked event, until unblocked
active: true                        # a live session has started this spec
parts: "1/2"                        # plan with children: done over total, dropped left out
root: "[[Filter vehicle false alarms]]"     # plan: the top of its tree (itself at the top)
---
```

## Lead callout

A plan:

```markdown
> [!spec] Started · plan · normal · active in [[2026-09-30 1040 f3e9a1]]
> [[Filter vehicle false alarms]] › **Score boxes by motion** · [[p3-edge]]
> Depends on [[Collect false alarm clips]] (done) · last event: continued 2026-09-30 10:40
```

A done plan links its result: `> [!spec-done] Done 2026-09-30 → [[Score boxes by motion · completed 2026-09-30 1102]]`. A dropped plan links the reason. A blocked plan says `Blocked: <the text>` on its first line.

A design:

```markdown
> [!design] Current · design · [[p3-edge]] · implemented by 2 plans
```

## Body

### Plan

1. The lead callout.
2. `## Goal`: what the work is for, in a paragraph.
3. `## Done when`: a list that a reviewer can check. Required before work starts.
4. `## Decisions`: each decision, with its reason.
5. `## Out of scope`.
6. `## Conventions`: the policies that apply, each linked, with one line on why.
7. `## Where`: the files, modules, and interfaces. For a leaf plan.
8. `## Verify`: the commands or checks that prove it works. For a leaf plan.
9. `## Parts`, code's: a table of the child plans in order, with the status, repository, and dependencies of each. Absent in a leaf plan.
10. `## Progress`: dated lines that the agent adds as the work goes: `- 2026-09-30 10:52: scored boxes; tests pass`. The session document quotes the last line.
11. `## History`, code's: an inline Base of the events whose subject is this spec, newest first. Obsidian renders it live.
12. `## Open questions`.
13. `## Origin`: the stub's words, when this spec was a stub.
14. `## Notes`: yours.

A small leaf plan may hold only `## Goal`, `## Done when`, `## Where`, and `## Verify`.

### Design

1. The lead callout.
2. `## Purpose`: what the part is for.
3. `## Behavior`: how it must work, stated so that a test could check it.
4. `## Interfaces`: what other parts call, and what it calls.
5. `## Constraints`: limits on speed, size, compatibility, security.
6. `## Decisions`: each with its reason.
7. `## Open questions`.
8. `## Implemented by`, code's: an inline Base of the plans whose `implements` names this design.
9. `## Origin`, when it was a stub; `## Notes`: yours.

## The lifecycle of a plan

| From | Call | Event | To |
|---|---|---|---|
| `open` | `work start` | `started` | `started` |
| `started` | `work start` | `continued` | `started` |
| `started` | `work done` | `completed` | `done` |
| `open`, `started` | `work drop` | `dropped` | `dropped` |
| `done`, `dropped` | `work reopen` | `reopened` | `open` |

Every row is a `work` call that writes an event ([[Event]]). The status is the last lifecycle event, so no field can disagree with the history.

- `work start` writes `started` on an open plan, and `continued` on a started plan (at most once per session). It binds the session to the plan ([[Sessions#Links]]). Only a plan with no parts can be started; work on a larger plan starts one of its parts.
- The first start of a child also writes `started` on each open ancestor, so a parent is started while any part of it is.
- `work done` takes the result prose (what was delivered, how it was verified, the follow-ups, what the wiki should learn) and writes the `completed` event that holds it. The event is the record of the result; the plan links it.
- `work drop` takes a reason and writes `dropped`. Dropping a plan drops every open or started plan below it, with one event each.
- `work reopen` writes `reopened`, and the plan is `open` again.
- `work block` and `work unblock` write `blocked` and `unblocked`. They do not change the status. `blocked` holds the text until the unblock.

## The edit rule

A hook refuses an edit inside a linked repository R unless the session has started a plan that covers R ([[Hooks#guard]]).

- A plan **covers** R when it names R in `repositories`. Only a plan with no parts can be started, so the session is bound to one piece of work.
- The plan must be `started`: not `open`, `done`, or `dropped`.
- Only `work start` binds a session to a plan. An Edit of a spec binds nothing, so editing a spec does not unlock a repository.
- A subagent that may write inherits its parent's started plans.
- A design spec never unlocks an edit. Work that builds a design is a plan that `implements` it.

## Rules the work tool enforces

- `parent` names a plan, and never makes a loop.
- `depends` names siblings (plans with the same parent), and never makes a loop.
- `repositories` names repository documents.
- `work start` is refused on a plan with parts, while a dependency is not `done`, and on a plan that another live session has started, unless the call sets `take: true`. Two sessions then never work one plan by accident.
- `work done` is refused while a child plan is `open` or `started`, and while `## Done when` is empty.
- A `done` or `dropped` plan takes no new child until `work reopen`.
- A design spec takes no `start`, `done`, `parent`, or `depends`. `supersedes` names a design spec, and makes that one `superseded`.
- A new spec is created only by `work` (`work spec` or `work promote`). The guard refuses a Write that creates a file in `wiki/documents/`.

## Pending

A spec is pending when its body, below the lead callout and without the code's sections, has changed since an applied change last absorbed it ([[Changes#Pending documents]]). So [[wiki-sync]] learns a design when it is written and again when it changes, and learns a plan's decisions.
