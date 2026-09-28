
# context

> Walk the context graph. Takes a scope (an area, a repository, or empty for the vault) or a path, and returns a [[Context Chain]].

This is how an agent that starts in the vault reaches a repository: it finds the repository with [[search]], then calls `context` with it.

| Input | Returns |
|---|---|
| `scope: ""` | the vault: `Atlas.md`, the top areas and repositories, every repository, the vault's policies, the open threads |
| `scope: <area>` | the chain to it, its children, every repository below it, the policies on the chain, the open threads in it |
| `scope: <repository>` | all of the above, and `instructions` and `repository` (git facts, now) |
| `path: <a path>` | the chain of the repository whose `path` holds it; refuses when none does |

- `policies` are every policy scoped to a node of the chain, nearest first: the repository's own, then its area's, up to the vault's. A policy lower in the chain comes first, because the nearer rule usually says more.
- `instructions` are the repository's `AGENTS.md` and `CLAUDE.md`, and any at the root of `.claude/`, each bounded to 400 lines with its path given. Claude Code does not load them when the session started in the vault, so this is how the agent reads them.
- `repository.behind` counts the commits from `described` to `head`. A page more than zero commits behind may be stale; [[repo-ingest]] brings it up to date.

Refusals: a scope that is not a scope page.
