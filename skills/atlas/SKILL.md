---
name: atlas
description: "Orient in the Atlas vault, show the board of threads and chords, make the quick moves on one, and route any request to the skill that owns it. Use for /atlas, what is going on, status, where do I work on X, the board, what is open, what should I work on, why is this still open, block, unblock, reprioritize, rename, drop, reopen, reorder, review the board, @atlas mentions, and any request when the right skill is not clear. Noting an idea is thread-stub; starting or resuming work is thread-work or chord-work."
---

# atlas

Every request passes this skill first. It reads where the session stands, names the kind
of request, finds the repository or the tags that the request names, and hands off. It
makes the small moves itself: the board, one thread or chord, and the calls that change
one field or one state.

Tools: `vault`, `search`, `context`, `thread` (list, load, set, block, unblock, drop,
reopen, note), `chord` (list, load, add, remove, order, set). References:
[threads.md](references/threads.md).

## Procedure

1. Read the opening context: the vault, the tags with their counts, the open threads,
   the live sessions, and this session's document. Call `vault` when the context is
   missing or the user asks for the state.
2. Name the kind of request with the table below.
3. When the request names work in a repository, find the repository: `search` with
   `types: [repository]` and the request's words, then `context` with `repository` set
   to the best hit. When two repositories match, ask which, and name both. Never guess
   between two.
4. When the request names categories ("my cs513 self-driving project"), turn the words
   that name tags of the vocabulary into `tags`, and pass them to the next call. The
   opening context and `vault` list the tags that exist.
5. Once the work is known, write one line under `## Description` in this session's
   document (the opening context links it). Use Edit.
6. Hand off.

| The user wants | Skill | Needs a started thread? |
|---|---|---|
| an answer, an explanation, to explore; what the vault holds under a tag | [wiki-query](../wiki-query/SKILL.md) | no |
| `Resume Atlas thread <id>`; a change to a repository; to continue a piece of work | [thread-work](../thread-work/SKILL.md) | yes, with an open task |
| `Resume Atlas chord <id>`; to continue a goal of several threads | [chord-work](../chord-work/SKILL.md) | per thread |
| a goal too large for one thread; to order threads | [chord-create](../chord-create/SKILL.md) | no |
| to note an idea, a bug, or a paper for later ("note this", "remember to") | [thread-stub](../thread-stub/SKILL.md) | no |
| one step of a thread by name: the spec, the tasks, the verification, the close | [thread-spec](../thread-spec/SKILL.md), [thread-tasks](../thread-tasks/SKILL.md), [thread-run](../thread-run/SKILL.md), [thread-verify](../thread-verify/SKILL.md), [thread-close](../thread-close/SKILL.md) | as the step needs |
| to close a chord | [chord-close](../chord-close/SKILL.md) | no |
| to ingest files, or process the inbox | [wiki-ingest](../wiki-ingest/SKILL.md) | no |
| to keep something from this conversation | [wiki-save](../wiki-save/SKILL.md) | no |
| to bring the wiki up to date with new documents | [wiki-sync](../wiki-sync/SKILL.md) | no |
| to link, unlink, or describe a repository | [repo-link](../repo-link/SKILL.md), [repo-unlink](../repo-unlink/SKILL.md), [repo-ingest](../repo-ingest/SKILL.md) | no |
| to check the wiki | [wiki-review](../wiki-review/SKILL.md) | no |
| to fix, rewrite, merge, or retag knowledge, or rename a tag | [wiki-edit](../wiki-edit/SKILL.md) | no |
| to organize the knowledge under a tag | [wiki-map](../wiki-map/SKILL.md) | no |
| a new vault | [atlas-onboard](../atlas-onboard/SKILL.md) | no |

A question can turn into work. When the user then asks for a change to a repository,
route to thread-work. The guard refuses an edit in a linked repository unless this
session started a thread with an open task for it.

## The board and the quick moves

These are small, so this skill does them itself.

- **The board**: `thread` list, with `tags`, `repository`, or `chord` when the request
  names them. Show the chords first, each with its threads in order and their statuses.
  Then the threads in no chord, in this order: `active` (with the session on each),
  `started`, `verified` (each waits for its wiki change), `ready`, `blocked` (with the
  reason), `waiting` (it comes after a thread not yet verified), and `stubs`. Mention
  the last few of `ended` only when the user asks what finished. Link `View · Threads`
  for the live view.
- **One thread**: `thread` load with `thread`. Say its status, its task count, what
  `missing` lists, and `next`. This answers "why is this still open": the status is
  derived, and `missing` is what stands between the thread and closed. No call and no
  button closes a thread; never tell the user to close one by hand.
- **One chord**: `chord` load with `chord`. Say its threads in order, the ready ones,
  and `next`.
- **A quick move**: one call, then one line back with the result.
  - priority: `thread` set with `set: {doc, priority}` (high, normal, low, someday);
  - rename: `thread` set with `set: {doc, title}`. Code renames every document of the
    thread and rewrites every link in the same commit;
  - tags: `thread` set with `set: {doc, tags}`; the thread's documents follow. In
    `tagging: known`, a tag that no document holds needs the user's yes, then
    `new_tags: true` in `set`;
  - order: `thread` set with `set: {doc, after: [...]}`, or `chord` order for several
    threads at once; `chord` add and `chord` remove move a thread in or out;
  - block: `thread` block with `thread` and `reason` (one line on what it waits on);
  - unblock: `thread` unblock with `thread`;
  - drop: `thread` drop with `thread` and `reason`; for a chord, `chord` drop, which
    drops each of its threads that is not ended;
  - reopen: `thread` reopen, or `chord` reopen, for one that was dropped. A closed
    thread needs no reopen: a new task takes it up again;
  - a remark to keep: `thread` note with `thread` and `text`.
- **What should I work on**: rank by priority; then by readiness (a verified thread
  that waits for its wiki change, then a started one, then a ready one, then a stub);
  then by how long it waited. A thread that comes after one not yet verified is not a
  candidate. Recommend one, with the reason and its hand-off line.
- **Review the board**, on request: threads started long ago with no recent event,
  verified threads nobody closed, blocked threads with no way to unblock, chords where
  nothing is ready, likely duplicates (`search` with each stub's words and
  `types: [stub]`), and work the user mentioned that has no thread.

`thread` list, `thread` load, and `chord` load bind nothing and write nothing. Only
`thread` start binds a session to a thread; that belongs to thread-run.

## Mentions

The opening context counts `@atlas` mentions: open task lines in the user's notes
addressed to the agent. When the user asks about them, or the request is unclear,
`vault` status lists them. Offer each one. Route it like any request. When a document
answers it (a new stub, an applied change, a spec, this session's document), close it
with `vault` `action: mention`, `note` (the note's path), `line`, and `link` (the
document that answers it).

## Gate

None for reads. A quick move is one line back, not a question. Ask first only when the
move is a drop of a chord, since the drop reaches every thread in it.

## Hand off

The routed skill.
