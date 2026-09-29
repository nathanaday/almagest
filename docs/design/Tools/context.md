# context

> Everything an agent needs to work in a repository, or under a set of tags. Takes a repository, tags, or a path, and returns a [[Context Brief]].

This is how an agent that starts in the vault reaches a repository: it finds the repository with [[search]] (`types: [repository]`), then calls `context` with it.

| Input | Returns |
|---|---|
| nothing | the vault: `Atlas.md`, the top-level tags with their pages and counts, every repository, the policies with no tags, the open work |
| `tags: [a, b]` | the tags, each with its page and the pages of every tag above it; the repositories that hold every one; the policies whose tags they all hold; the open work that holds them |
| `repository: <id>` | all of the above for the tags the repository holds and defines, and `instructions` and `repository` (git facts, now) |
| `path: <a path>` | the context of the repository whose `path` holds the path; refuses when none does |

- `pages` are the tag pages from the top down: for `work/p3/p3-edge`, the pages of `work`, `work/p3`, and `work/p3/p3-edge`, when they exist. An agent reads their `## Context` first.
- `policies` are every policy that applies ([[Topic#Policy]]), the most specific first: the most tags, then the deepest tags. A policy with no tags comes last.
- `work` is the open and started plans and the open stubs that hold the tags, or that name the repository.
- `instructions` are the repository's `AGENTS.md` and `CLAUDE.md`, and any at the root of `.claude/`, each bounded to 400 lines with its path given. Claude Code does not load them when the session started in the vault, so this is how the agent reads them.
- `repository` gives the git facts now, read from git at the call: the branch, the head, the files changed and not committed, the commits ahead of and behind the remote from the last fetch, the last three commits, and `behind` since `described`. The Obsidian plugin's repository panel reads the same facts with `--json`.

Refusals: a repository id that is not a repository document; a tag that breaks [[Documents#Form]].
