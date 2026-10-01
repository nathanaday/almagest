---
name: thread-spec
description: "Write the spec of a thread: research the wiki and the code, then say what must be true when the work is done, as numbered requirements, with the rules, the decisions, and the wiki pages it rests on. Bring the wiki up to date with what the research found. Use for spec this, define this, what should this be, requirements, flesh out the stub, what does done mean, revise the spec. The steps are thread-tasks; several threads are chord-create."
---

# thread-spec

A spec says what must be true when the work is done, and why. It holds requirements a
reviewer can check, the rules that bind the work, the decisions with their reasons, and
the wiki pages it rests on. It holds no task, no method, and no progress: those have
their own documents. Most of a spec comes from reading. The user answers only what
reading cannot decide and a wrong guess would change.

Tools: `thread` (load, spec), `context`, `search`, `match`, `change`. References:
[threads.md](../atlas/references/threads.md),
[conventions.md](../atlas/references/conventions.md),
[changes.md](../atlas/references/changes.md).

## Procedure

1. `thread` load with `thread`: read the idea, the notes, and the thread's place in its
   chord. A thread that comes after others builds on what they deliver; read their specs.
2. **Research the wiki.** List the subjects the idea names: repositories, entities
   (tools, datasets, models, services), concepts, methods. `search` each, then Read the
   pages that matter. For each repository, `context` with `repository`: its instruction
   files, its git facts, its tag pages, and the candidate policies.
3. **Research the code** the work touches: where it lands, what it changes, what tests
   exist.
4. Decide what the research decides. Ask only what it cannot answer and a wrong guess
   would change, in one message, each question with a recommended answer.
5. **The conventions step** ([conventions.md](../atlas/references/conventions.md)): keep
   the policies that bind this work, each with one line on why.
6. Write the spec's text:
   - `## Goal`: one paragraph;
   - `## Requirements`: each line `- R1: …`. Each names what is true and how one can see
     it ("`go test ./...` passes", "the INT8 mAP50 of each cell is in the results
     table"). Keep them few and checkable. Never renumber one that exists;
   - `## Rules`: business rules, principles, and the policies of step 5;
   - `## Decisions`: each with its reason;
   - `## Out of scope`;
   - `## Knowledge`: each wiki page the spec relied on, linked, with one line on what it
     gave;
   - `## Open questions`.
   Leave out how to do the work, and any list of steps: thread-tasks writes those.
7. `thread` spec with `thread` and `text`. The tool refuses a section a spec does not
   hold, a check box, and a requirement line with no id; fix the text and call again.
8. **Bring the wiki up to date.** For each subject of step 2:
   - the wiki has a page, and the research found it wrong or thin → one modify;
   - the wiki has no page → one create, from what the research found, cited (a source,
     a repository, or this session);
   - then add the page under the spec's `## Knowledge` with Edit.
   Build one Wiki Change Plan ([changes.md](../atlas/references/changes.md)) with `work`
   set to the thread, and `change` propose. A subject you know only by name is a stub,
   not a page (`thread` stub). No gap, no change.

Write the spec before you propose the change: a topic the spec cites gains a
`## Threads` section at the next sync, which is an edit of that topic.

## Gate

Link the spec. Say its goal and its requirements in a few lines, the questions that stay
open, and the wiki change when there is one. Wait for the user to agree or to edit. An
edit in Obsidian counts: `thread` load again before you go on. Apply the change after
the user's yes.

A run with no user present takes the recommended answers and says so.

## Hand off

[thread-tasks](../thread-tasks/SKILL.md).
