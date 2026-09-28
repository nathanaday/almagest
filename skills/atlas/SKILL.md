---
name: atlas
description: "Orient in the Atlas vault, show the board, make the quick moves on a thread, and route any request to the skill that owns it. Use for /atlas, what is going on, status, where do I work on X, threads, the board, what is open, what should I work on, block, unblock, reprioritize, rename a thread, reopen, review the board, @atlas mentions, and any request when the right skill is not clear."
---

# atlas

Every request passes this skill first. It reads where the session stands, names the kind
of request, finds the scope when the request names work in a repository, and hands off.
It does the small thread moves itself.

Tools: `vault`, `search`, `context`, `thread` (list, show, set, reopen). References:
[threads.md](references/threads.md).

## Procedure

1. Read the opening context. Call `vault` when it is missing or the user asks for the
   state.
2. Name the kind of request with the table below.
3. When the request names work in a repository, find the scope: `search` with
   `types: [repository, area]` and the request's words, then `context` for the best
   match. When two repositories match, ask which, and name both. Never guess between
   two.
4. Once the work is known, write one line under `## Description` in this session's
   document (the opening context links it). Use Edit.
5. Hand off.

| The user wants | Skill | Thread? |
|---|---|---|
| an answer, an explanation, to explore | [wiki-query](../wiki-query/SKILL.md) | no |
| a change to one or more repositories | [thread-work](../thread-work/SKILL.md) | yes |
| to note an idea or a bug for later | [thread-stub](../thread-stub/SKILL.md) | yes, a stub |
| to ingest files, or process the inbox | [wiki-ingest](../wiki-ingest/SKILL.md) | no |
| to keep something from this conversation | [wiki-save](../wiki-save/SKILL.md) | no |
| to bring the wiki up to date with new documents | [wiki-sync](../wiki-sync/SKILL.md) | no |
| to link, unlink, or describe a repository | [repo-link](../repo-link/SKILL.md), [repo-unlink](../repo-unlink/SKILL.md), [repo-ingest](../repo-ingest/SKILL.md) | no |
| to check or fix the wiki | [wiki-review](../wiki-review/SKILL.md), [wiki-edit](../wiki-edit/SKILL.md) | no |
| to organize knowledge across repositories | [wiki-rollup](../wiki-rollup/SKILL.md) | no |
| a new vault | [atlas-onboard](../atlas-onboard/SKILL.md) | no |

A question can turn into work. When the user then asks for a change, route to
thread-work; the guard refuses the edit without a thread anyway.

## The board and the quick moves

These are small, so this skill does them itself.

- **The board**: `thread` list. Show the threads by stage, the active ones first with
  the session working on each. Link `threads/Threads.base` for the live view.
- **One thread**: `thread` show. Say its stage, its tasks and their status, and `next`.
- **A quick move**: `thread` set (priority, blocked, title, scope) or reopen. Say the
  result in one line.
- **What should I work on**: rank the open threads by priority, then by readiness (a
  ready task beats a stub), then by how long they waited. Recommend one, with the
  reason.
- **Review the board**, on request: stale threads, blocked threads with no plan to
  unblock, likely duplicates (`search` with each stub's words), and work the user
  mentioned that has no thread.

## Mentions

The opening context counts `@atlas` mentions: task lines in the user's notes addressed
to the agent. When the user asks about them, or the request is unclear, `vault` status
lists them. Offer each one. Route it like any request. When a document answers it (a
new stub, an applied change, this session's document), close it with `vault`
`action: mention`, the note's path, the line, and a link to the answer.

## Gate

None for reads. A quick move is one line back, not a question.

## Hand off

The routed skill.
