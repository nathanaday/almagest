---
name: thread-stub
description: "Open a thread from a sentence, in the user's words, with no questions. Use for note this, stub this, remember to, idea for later, open a thread, report a bug, add a todo."
---

# thread-stub

A stub is the first words of a line of work, loose by design. It keeps the user's words
as they gave them. The questions come later, in thread-spec.

Tools: `search`, `thread` open. References: [threads.md](../atlas/references/threads.md).

## Procedure

1. `search` the open stubs for the same work: `types: [stub]`,
   `state: {stage: [stub, spec, tasks]}`, and the request's words. When one matches,
   offer to add the new words to its `## Notes` (with Edit), and stop if the user
   agrees.
2. Call `thread` with `action: open`: `text` is the user's words as given; `title` is a
   short name for the work; `scope` is the repositories or areas the request names or
   the conversation is about, or empty. The first scope is the thread's home: its folder
   goes under that scope in `threads/`. With no scope the thread waits at the top of
   `threads/`, and `thread` set scope files it later.
3. Say the stub's link in one line.

Ask nothing else. A stub is loose by design.

## Gate

None.

## Hand off

[thread-work](../thread-work/SKILL.md), when the user wants to start now.
