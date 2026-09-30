---
name: atlas
description: "Orient in the Atlas vault, show the board, make the quick moves on a stub or a plan, and route any request to the skill that owns it. Use for /atlas, what is going on, status, where do I work on X, the board, what is open, what should I work on, block, unblock, reprioritize, rename, drop, reopen, review the board, @atlas mentions, and any request when the right skill is not clear. Noting an idea is wiki-stub; starting work is spec-work."
---

# atlas

Every request passes this skill first. It reads where the session stands, names the kind
of request, finds the repository or the tags that the request names, and hands off. It
makes the small moves on work itself: the board, one plan or stub, and the `work` calls
that change one field or one state.

Tools: `vault`, `search`, `context`, `work` (list, show, set, block, unblock, drop,
reopen). References: [work.md](references/work.md).

## Procedure

1. Read the opening context: the vault, the tags with their counts, the open work, the
   live sessions, and this session's document. Call `vault` when the context is missing
   or the user asks for the state.
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

| The user wants | Skill | Needs a started plan? |
|---|---|---|
| an answer, an explanation, to explore; what the vault holds under a tag | [wiki-query](../wiki-query/SKILL.md) | no |
| a change to one or more repositories | [spec-work](../spec-work/SKILL.md) | yes |
| to note an idea, a bug, or a paper for later ("note this", "remember to") | [wiki-stub](../wiki-stub/SKILL.md) | no |
| to ingest files, or process the inbox | [wiki-ingest](../wiki-ingest/SKILL.md) | no |
| to keep something from this conversation | [wiki-save](../wiki-save/SKILL.md) | no |
| to bring the wiki up to date with new documents | [wiki-sync](../wiki-sync/SKILL.md) | no |
| to link, unlink, or describe a repository | [repo-link](../repo-link/SKILL.md), [repo-unlink](../repo-unlink/SKILL.md), [repo-ingest](../repo-ingest/SKILL.md) | no |
| to check the wiki | [wiki-review](../wiki-review/SKILL.md) | no |
| to fix, rewrite, merge, or retag knowledge, or rename a tag | [wiki-edit](../wiki-edit/SKILL.md) | no |
| to organize the knowledge under a tag | [wiki-map](../wiki-map/SKILL.md) | no |
| a new vault | [atlas-onboard](../atlas-onboard/SKILL.md) | no |

A question can turn into work. When the user then asks for a change to a repository,
route to spec-work. The guard refuses an edit in a linked repository without a started
plan that names it.

## The board and the quick moves

These are small, so this skill does them itself.

- **The board**: `work` list, with `tags` or `repository` when the request names them.
  Show the groups in this order: `active` (with the session on each), `started`,
  `ready`, `blocked` (with the reason), `waiting` (open plans whose dependencies are not
  done), and the open `stubs`. Mention the last few of `done` only when the user asks
  what finished. Link `View · Work` for the live view.
- **One plan or stub**: `work` show with `doc`. Say its status, its parts and the status
  of each, its last events, and its `next`: `write` (it needs a spec or its
  `## Done when`), `start <plan>`, `done`, or `none`.
- **A quick move**: one `work` call, then one line back with the result.
  - priority: `work` set with `set: {doc, priority}` (high, normal, low, someday);
  - rename: `work` set with `set: {doc, title}`. Code renames the file and rewrites
    every link in the same commit;
  - tags: `work` set with `set: {doc, tags}`. In `tagging: known`, a tag that no
    document holds needs the user's yes, then `new_tags: true` in `set`;
  - block: `work` block with `spec` and `reason` (one line on what the plan waits on);
  - unblock: `work` unblock with `spec`;
  - drop: `work` drop with `doc` and `reason`. Dropping a plan drops every open or
    started plan below it;
  - reopen: `work` reopen with `doc`, and `reason` when the user gives one.
- **What should I work on**: rank the open plans and stubs by priority, then by
  readiness (a ready plan beats a stub), then by how long they waited. Recommend one,
  with the reason.
- **Review the board**, on request: plans started long ago with no recent event,
  blocked plans with no way to unblock, likely duplicates (`search` with each stub's
  words and `types: [stub, spec]`), and work the user mentioned that has no stub or
  plan.

`work` list and show bind nothing and write nothing. Only `work` start binds a session to
a plan; that belongs to spec-run.

## Mentions

The opening context counts `@atlas` mentions: open task lines in the user's notes
addressed to the agent. When the user asks about them, or the request is unclear,
`vault` status lists them. Offer each one. Route it like any request. When a document
answers it (a new stub, an applied change, a spec, this session's document), close it
with `vault` `action: mention`, `note` (the note's path), `line`, and `link` (the
document that answers it).

## Gate

None for reads. A quick move is one line back, not a question. Ask first only when the
move is a drop of a plan with parts, since the drop reaches every part.

## Hand off

The routed skill.
