# Threads

A thread is one line of work on one or more repositories: a feature, a fix, a chore.
Every edit an agent makes inside a linked repository belongs to a thread: the guard
refuses the edit while this session has no open thread that covers the repository.
Questions and exploration need no thread.

## The documents

| Document | Answers | How many | Filed by |
|---|---|---|---|
| stub | What is this about? The user's words, loose. | one; the thread's root | `thread` open |
| spec | What is true when it is done, and why? | zero or one | `thread` file, part spec |
| task | One piece of the work: what, where, how to verify, and the result | zero or more | `thread` tasks |
| receipt | How did it end: completed or killed? | zero or one | `thread` file, part receipt |

The stage is the furthest document that exists: stub, spec, tasks, closed. Nothing sets
it. A stage may be skipped: a small fix goes from stub to one task; a killed idea goes
from stub to receipt.

## Sections

- Stub: `## Stub` (the user's words, as given), `## Notes` (the user's; add to it only
  when asked).
- Spec: `## Goal`, `## Done when` (a list a reviewer can check), `## Decisions` (each
  with its reason), `## Out of scope`, `## Conventions` (each policy linked, with one
  line on why), `## Open questions`.
- Task: `## What`, `## Where` (files, modules, interfaces), `## Conventions` (policy
  links), `## Verify` (the commands or checks that prove it works), `## Progress`
  (dated lines, added as the work goes: `- 2026-09-27: scored boxes; tests pass`),
  `## Result` (written by `thread` task done).
- Receipt: `## Delivered`, `## Verified`, `## Follow-ups` (links to new stubs),
  `## Learned` (what the wiki should absorb). A killed receipt holds `## Why killed` in
  place of the first two.

## The thread tool

| Action | Takes | Does |
|---|---|---|
| list | — | the board, by stage |
| show | thread | the Thread View: the documents, the sessions, and `next` |
| open | text, title, scope, priority, inbox | the folder and the stub |
| attach | thread | nothing on disk; the hook binds this session to the thread |
| file | thread, part (spec or receipt), text, outcome | the document |
| tasks | thread, tasks [{title, text, repository, depends, order}] | one document per task |
| task | task (id, title, or T2 with thread), do: start, done, drop, reopen, set | start binds the task to this session; done writes the result |
| set | thread, title, priority, blocked, scope | the stub; a new title renames every document and link |
| reopen | thread | the receipt stays, marked superseded |

- `text` and `result` are your prose; the tool writes no prose of its own.
- `next` in the Thread View is the next step: spec, tasks, task T2, receipt, or none.
- The tool refuses: a second spec or receipt; a completed receipt while a task is open;
  a new document on a closed thread; a dependency loop; starting a task whose
  dependencies are open; starting a task another live session holds, unless `take`.
  Each refusal names the call that would succeed.

## Edit the repository with the edit tools

Change a repository's files with Edit and Write, not with the shell (`cat >`, `sed -i`,
`perl -i`). The thread rule and the session's record see the edit tools; a shell write
goes around both. Use the shell to run commands: tests, builds, git.

## Edit, never Write

After a document exists, revise its prose with Edit. Never write a thread document with
Write, and never edit its frontmatter or its first callout: the guard refuses both, and
the thread tool changes them.

## The session document

The opening context names this session's document, `sessions/<month>/<date time id>.md`.
Hooks keep its frontmatter, its first callout, and `## Subagents`. You write three
sections with Edit: `## Description` (one line on what the session works on),
`## Progress` (dated lines, for work that is not a task), and `## Summary` (at the end).
Work on a task writes its progress in the task document only.
