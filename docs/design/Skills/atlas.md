# atlas

> Orient in the vault, show the board, make the quick moves on work, and route any request to the skill that owns it. The skill every request passes first.

**Use for**: /atlas, what is going on, status, where do I work on X, the board, what is open, what should I work on, block, unblock, reprioritize, rename, drop, reopen, review the board, @atlas mentions, and any request when the right skill is not clear.

**Tools**: `vault`, `search`, `context`, `work` (list, show, set, block, unblock, drop, reopen). **References**: `references/work.md`.

## Procedure

1. Read the session-start context. Call `vault` when it is missing or the user asks for the state.
2. Name the kind of request, with the table below.
3. When the request names work in a repository, find the repository: `search` with `types: [repository]`, then `context`. When two repositories match, ask which, naming both. Never guess between two. When the request names categories ("my cs513 self-driving project"), use them as `tags`.
4. Once the work is known, write one line under `## Description` in this session's document (the opening context links it).
5. Hand off.

| The user wants | Skill | Needs a started plan? |
|---|---|---|
| an answer, an explanation, to explore | [[wiki-query]] | no |
| a change to one or more repositories | [[spec-work]] | yes |
| to note an idea, a bug, or a paper for later | [[wiki-stub]] | no |
| to ingest files, or process the inbox | [[wiki-ingest]] | no |
| to keep something from this conversation | [[wiki-save]] | no |
| to bring the wiki up to date with new documents | [[wiki-sync]] | no |
| to link, unlink, or describe a repository | [[repo-link]], [[repo-unlink]], [[repo-ingest]] | no |
| to check or fix the wiki, or rename a tag | [[wiki-review]], [[wiki-edit]] | no |
| to organize the knowledge under a tag | [[wiki-map]] | no |
| a new vault | [[atlas-onboard]] | no |

A question can turn into work. When the user then asks for a change, route to [[spec-work]]; the guard refuses the edit without a started plan anyway.

## The board and the quick moves

These are small, so this skill does them itself:

- **The board**: `work` list. Show the active plans with the session on each, then started, ready, blocked, and the open stubs. Link `View · Work` for the live view.
- **One plan or stub**: `work` show. Say its status, its parts and their status, its last events, and its `next`.
- **A quick move**: `work` set (priority, title, tags), block, unblock, drop, or reopen. One line back.
- **What should I work on**: rank the open plans by priority, then readiness (a ready part beats a stub), then how long they waited. Recommend one, with the reason.
- **Review the board**, on request: stale plans, blocked plans with no way to unblock, likely duplicates (`search` on each stub), and work the user mentioned that has no stub or plan.
- **Mentions**: offer the open mentions from `vault`. Route each to its skill, then close it with `vault` mention and a link to the answer.

## Hand off

The routed skill.
