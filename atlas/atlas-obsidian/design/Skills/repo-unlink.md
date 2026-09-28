
# repo-unlink

> Unlink a repository from the vault: remove its page, and decide what happens to the pages and threads that depend on it. The repository on disk is never touched.

**Use for**: unlink, remove this repository, forget this repo, the repository moved away.

**Tools**: `context`, `search`, `thread`, `change`. **References**: `references/changes.md`.

## Procedure

1. Call `context` for the repository. Count the pages scoped to it (`search` with its scope) and the open threads whose scope names it.
2. Ask one question with the counts and a recommended answer for each group:
   - pages: move them up to the parent scope (recommended), or remove them;
   - open threads: keep them and drop the repository from their scope (recommended), or kill them.
3. Build the change: remove the repository page with `redirect` to its parent. Code rewrites every link to it, the `scope` fields included, so the pages move up in the same commit. Add a remove for each page the user chose to remove.
4. For the threads: `thread` set scope, or a killed receipt, per the answer.

## Gate

Propose the change. Show the preview and the thread moves. Wait for the yes, then apply the change and make the thread writes.

## Hand off

None.
