
# atlas

> Orient in the vault, show the board, make the quick moves on a thread, and route any request to the skill that owns it. The one front door.

**Use for**: /atlas, what is going on, status, where do I work on X, threads, the board, what is open, what should I work on, block, unblock, reprioritize, rename a thread, reopen, review the board, and any request when the right skill is not clear.

**Tools**: `vault`, `search`, `context`, `thread` (list, show, set, reopen), `session`. **References**: `references/threads.md`.

## Procedure

1. Read the session-start context. Call `vault` when it is missing or the user asks for the state.
2. Name the kind of request, with the table below.
3. When the request names work in a repository, find the scope: `search` with `types: [repository, area]`, then `context`. When two repositories match, ask which, naming both. Never guess between two.
4. Once the work is known, call `session` describe with one line.
5. Hand off.

| The user wants | Skill | Thread? |
|---|---|---|
| an answer, an explanation, to explore | [[wiki-query]] | no |
| a change to one or more repositories | [[thread-work]] | yes |
| to note an idea or a bug for later | [[thread-stub]] | yes, a stub |
| to ingest files, or process the inbox | [[wiki-ingest]] | no |
| to keep something from this conversation | [[wiki-save]] | no |
| to bring the wiki up to date with new documents | [[wiki-sync]] | no |
| to link, unlink, or describe a repository | [[repo-link]], [[repo-unlink]], [[repo-ingest]] | no |
| to check or fix the wiki | [[wiki-review]], [[wiki-edit]] | no |
| to organize knowledge across repositories | [[wiki-rollup]] | no |
| a new vault | [[atlas-onboard]] | no |

A question can turn into work. When the user then asks for a change, route to [[thread-work]]; the guard would refuse the edit without a thread anyway.

## The board and the quick moves

These are small, so the front door does them itself:

- **The board**: `thread` list. Show the threads by stage, the active ones first with the session working on each. Link `Threads.base` for the live view.
- **One thread**: `thread` show. Say its stage, its tasks and their status, and its `next`.
- **A quick move**: `thread` set (priority, blocked, title, scope) or reopen. One line back.
- **What should I work on**: rank the open threads by priority, then readiness (a ready task beats a stub), then how long they waited. Recommend one, with the reason.
- **Review the board**, on request: stale threads, blocked threads with no plan to unblock, likely duplicates (`search` on each stub), and work the user mentioned that has no thread.

## Hand off

The routed skill.
