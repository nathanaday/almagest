# Work

Work is what the user means to do. Two document types hold it, and a third records
what happened to them:

| Document | Answers | Written by |
|---|---|---|
| stub | What is the idea? The user's words, loose. | `work` stub |
| spec, kind `plan` | What is true when the work is done, and why? A plan ends. | `work` spec or promote |
| spec, kind `design` | How must a part of a repository or a system work? A design does not end. | `work` spec or promote |
| event | What happened to a stub or a plan, and when? | code, in the commit of the call that caused it |

Every edit an agent makes inside a linked repository belongs to a started plan: the
guard refuses the edit otherwise ([the edit rule](#the-edit-rule)). Questions,
exploration, and stubs need no plan.

## Stub

- Fields: the common ones, and `priority` (high, normal, low, someday). Code owns
  `status` (open, resolved, dropped) and `became`.
- Sections: `## Idea` (the user's words, as given; never rewrite them), `## Notes` (the
  user's; add to it only when asked).
- A stub names no repository and never unlocks an edit.
- A stub becomes something in one of two ways:
  - **in place**, when it becomes one document: `work` promote makes it a spec;
    `change` with `{op: promote}` makes it a topic. It keeps its id, its file, and every
    link. Code keeps `## Idea` as `## Origin`, and writes a `promoted` event;
  - **by spawning**, when it becomes several documents or a source: each new document
    names the stub in `from` (`work` spec with `from`, a `change` create with `from` in
    `fields`, `source` capture with `resolves`), then `work` resolve with `stub` and
    `became`, or `resolve: true` on the `work` spec call that creates the last one.
- `work` drop with a reason closes a stub; `work` reopen brings it back.

## Spec

A plan may have child plans (its parts), to any depth. A **leaf plan** has no parts: one
piece of work in one repository that a reviewer can check alone. Only a leaf plan can be
started.

Fields you give: `kind`, `description`, `tags`, `aliases`, `parent` (plan: the plan it is
a part of), `repositories` (the repositories the work touches, or the design describes),
`depends` (plan: sibling plans that must be done first), `order` (plan: its place among
its siblings), `priority` (plan), `implements` (plan: the designs it builds),
`supersedes` (design: the design it replaces), `from` (the stub it spawned from). Code
owns `status`, `blocked`, `active`, `parts`, and `root`.

Sections of a plan, in order:

- `## Goal`: what the work is for, in a paragraph.
- `## Done when`: a list that a reviewer can check. Required before `work` done.
- `## Decisions`: each decision, with its reason.
- `## Out of scope`.
- `## Conventions`: the policies that bind the work, each linked, with one line on why
  ([conventions.md](conventions.md)).
- `## Where`: the files, modules, and interfaces. For a leaf plan.
- `## Verify`: the commands or checks that prove it works. For a leaf plan.
- `## Parts`: code's; a table of the child plans.
- `## Progress`: dated lines that you add with Edit as the work goes:
  `- 2026-09-30 10:52: scored boxes; tests pass`. The session document quotes the last
  line.
- `## History`: code's; the events of this spec.
- `## Open questions`.
- `## Origin`: code's; the stub's words, when the spec was a stub.
- `## Notes`: the user's.

A small leaf plan may hold only `## Goal`, `## Done when`, `## Where`, and `## Verify`.

Sections of a design: `## Purpose`, `## Behavior` (stated so that a test could check
it), `## Interfaces`, `## Constraints`, `## Decisions`, `## Open questions`,
`## Implemented by` (code's), `## Origin` (code's), `## Notes`.

## Event

Code writes every event, in the commit of the `work` call or the `change` apply that
caused it. You never create one. The lead callout is `[!event-<kind>]`.

| Kind | Subject | Written by | Holds prose | Sets status |
|---|---|---|---|---|
| `started` | plan | `work` start on an open plan; the first start of a part, on each open plan above it | no | `started` |
| `continued` | plan | `work` start on a plan started in an earlier session | no | `started` |
| `completed` | plan | `work` done | `## Delivered`, `## Verified`, `## Follow-ups`, `## Learned` | `done` |
| `dropped` | stub, plan | `work` drop | `## Why` | `dropped` |
| `reopened` | stub, plan | `work` reopen | `## Why`, when given | `open` |
| `blocked` | plan | `work` block | the one line | no; sets `blocked` |
| `unblocked` | plan | `work` unblock | no | no; clears `blocked` |
| `promoted` | the document a stub became in place | `work` promote; `change` apply of a promote | no | the new type's |
| `resolved` | stub | `work` resolve; `resolve: true`; capture with `resolves` | no | `resolved` |
| `note` | any document | `work` note | `## Note` | no |

You may Edit the prose of an event to correct a result. Never edit its frontmatter or
its lead callout.

## Status comes from events

Code derives the status of a stub or a plan from its lifecycle events: the last one by
time decides; with none, the status is `open`. No `work` call sets a status directly,
and `work` set takes no `status` or `blocked`.

| From | Call | Event | To |
|---|---|---|---|
| `open` | `work` start | `started` | `started` |
| `started` | `work` start, in a later session | `continued` | `started` |
| `started` | `work` done | `completed` | `done` |
| `open`, `started` | `work` drop | `dropped` | `dropped` |
| `done`, `dropped` | `work` reopen | `reopened` | `open` |

A design is `current` until a later design names it in `supersedes`; then it is
`superseded`.

## The work tool

| Action | Takes | Does |
|---|---|---|
| list | optional `tags`, `repository` | the board: `active`, `started`, `ready`, `blocked`, `waiting`, `stubs`, and the last ten `done` |
| show | `doc` | the Work View: the document, its root, parts, events, result, sessions, changes, and `next` |
| stub | `text`, `title`, `description`, `tags`, `priority`, `inbox`, `new_tags` | the stub; an `inbox` note it replaces leaves in the same commit |
| spec | `specs: [{title, kind, text, description, tags, parent, repositories, depends, order, priority, implements, supersedes, from}]`, `resolve`, `new_tags` | one document per spec, in one commit; `parent` and `depends` may name a spec of the same call by title |
| promote | `stub`, `kind` (plan or design), `text`, `title`, `description`, `tags`, `repositories`, `parent`, `priority`, `new_tags` | the stub becomes a spec in place; a `promoted` event |
| start | `spec`, optional `take` | binds this session to a leaf plan; `started` or `continued`; `started` on each open plan above it |
| done | `spec`, `result: {delivered, verified, follow_ups, learned}` | a `completed` event that holds the result |
| drop | `doc`, `reason` | a `dropped` event, and one for each open or started plan below |
| reopen | `doc`, optional `reason` | a `reopened` event |
| block | `spec`, `reason` (one line) | a `blocked` event |
| unblock | `spec` | an `unblocked` event |
| resolve | `stub`, `became` | `became` on the stub; a `resolved` event |
| note | `doc`, `text` | a `note` event on any typed document |
| set | `set: {doc, title, description, tags, aliases, priority, parent, repositories, depends, order, implements, supersedes, new_tags}` | the fields given; a new title renames the file and rewrites every link |

- `text`, `result`, and `reason` are your prose. The tool writes them into the
  document's sections and writes no prose of its own. A spec's `text` is its body below
  the lead callout; code puts back its own sections.
- Give a `title` for a stub. Without one, code cuts the first line of `text`.
- `next` in the Work View is the next step:
  - `write`: an open stub, or a plan with an empty `## Done when`;
  - `start <plan>`: an open leaf plan, or the first ready part of a plan. Decide
    whether a large leaf plan needs splitting (spec-split) before you start it;
  - `done`: a started leaf plan, or a plan whose parts are all done or dropped;
  - `none`: a closed plan or stub, or a design.
- `work` done needs `delivered` and `verified`; `follow_ups` and `learned` are
  optional. `learned` is what the wiki should absorb, and wiki-sync reads it first.
- In `tagging: known`, a tag that no document holds needs the user's yes, then
  `new_tags: true` (inside `set` for `work` set).
- list and show bind nothing and write nothing.
- Each write is one git commit, with its events.

The tool refuses, and each refusal names the call that would succeed:

- `work` start on a plan with parts, on a plan whose dependencies are not done, and on a
  plan that another live session started, unless `take: true`;
- `work` done on a plan that is not started, while `## Done when` is empty, or while a
  part is open or started;
- a new part under a done or dropped plan, until `work` reopen;
- `work` set of `parent`, `depends`, or `repositories` on a done or dropped plan;
- a `parent` or `depends` that makes a loop, a `depends` that names no sibling, and a
  `repositories` entry that names no repository;
- `start`, `done`, `parent`, or `depends` on a design;
- a title that another document holds, or that begins with `Tag · ` or `View · `;
- a promote of a document that is not an open stub.

## The edit rule

The guard refuses an edit inside a linked repository R unless this session has started a
plan that names R in `repositories`.

- Only `work` start binds a session to a plan. Editing a spec binds nothing.
- The plan must be `started`: not `open`, `done`, or `dropped`.
- A design never unlocks an edit. Work that builds a design is a plan that
  `implements` it.
- The guard refuses a second `work` start of a plan this session already started. Go on
  with the work, and add a line to its `## Progress`.
- A subagent that may write inherits its parent's started plans.

## Edit the repository with the edit tools

Change a repository's files with Edit and Write, not with the shell (`cat >`, `sed -i`,
`perl -i`). The edit rule and the session's record see the edit tools; a shell write
goes around both. Use the shell to run commands: tests, builds, git.

## Edit, never Write

A stub, a spec, or an event comes only from the `work` tool; the guard refuses a Write
that creates a file in `wiki/documents/`. After a document exists, revise its prose with
Edit. Never edit its frontmatter, its lead callout, or a section that is code's
(`## Parts`, `## History`, `## Implemented by`, `## Origin`): the guard refuses them.
Change fields with `work` set, and states with the action that fits.

## The session document

The opening context names this session's document, `sessions/<month>/<date time id>.md`.
Hooks keep its frontmatter, its first callout, and `## Subagents`. You write three
sections with Edit: `## Description` (one line on what the session works on),
`## Progress` (dated lines, for work that is not a plan), and `## Summary` (at the end).
Work on a plan writes its progress in the spec only.
