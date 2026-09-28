---
name: thread-spec
description: "Turn a thread's stub into a spec: what will be true when the work is done, and why. Research first; ask only what research cannot answer. Use for spec this, define this thread, what should this be, design this, flesh out the stub, requirements."
---

# thread-spec

A spec says what is true when the work is done, in a list a reviewer can check, with the
decisions and their reasons. Most of it comes from reading the code and the wiki. The
user answers only what reading cannot decide and a wrong guess would change.

Tools: `thread` (show, file), `context`, `search`. References:
[threads.md](../atlas/references/threads.md),
[conventions.md](../atlas/references/conventions.md).

## Procedure

1. `thread` show: read the stub and its notes.
2. `context` for each scope of the thread: the instruction files, the repository facts,
   and the candidate policies.
3. Research: read the code the work touches, and the wiki (`search`, then Read) for what
   the vault already knows.
4. Decide what the research decides. Ask only what it cannot answer and a wrong guess
   would change, in one message, each question with a recommended answer. Ask no fixed
   number of questions.
5. **The conventions step** ([conventions.md](../atlas/references/conventions.md)): for
   each candidate policy, decide whether it applies to this thread. Keep the ones that
   do, each with one line on why.
6. `thread` file with `part: spec` and the text: `## Goal`, `## Done when` (a list a
   reviewer can check), `## Decisions` (each with its reason), `## Out of scope`,
   `## Conventions`, `## Open questions`.

## Gate

Link the spec and say its goal and its done-when list in a few lines. Wait for the user
to agree or to edit it. An edit in Obsidian counts: read the spec again before you go
on.

## Hand off

[thread-plan](../thread-plan/SKILL.md).
