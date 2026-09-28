---
name: repo-unlink
description: "Unlink a repository from the vault: remove its page, and decide what happens to the pages and threads that depend on it. The repository on disk is never touched. Use for unlink, remove this repository, forget this repo, the repository moved away."
---

# repo-unlink

A repository page can go while its code stays on disk. The pages scoped to it and the
threads about it must go somewhere, so this skill asks once, with a recommendation for
each group, and then makes the change and the thread writes.

Tools: `context`, `search`, `thread`, `change`. References:
[changes.md](../atlas/references/changes.md).

## Procedure

1. Call `context` for the repository. Count the pages scoped to it (`search` with
   `scope` set to it and `types: [concept, entity, policy, source]`) and the open
   threads whose scope names it (`context` lists them).
2. Ask one question with the counts and a recommended answer for each group:
   - pages: move them up to the parent scope (recommended), or remove them;
   - open threads: keep them and drop the repository from their scope (recommended), or
     kill them.
3. Build the change: a remove of the repository page with `redirect` set to its parent
   area. Code rewrites every link to the page, the `scope` fields included, so the
   pages move up in the same commit. A task's `repository` never becomes an area; the
   preview warns of each one it leaves. Add a remove for each page the user chose to
   remove.
4. For the threads: `thread` set with the new `scope`, or `thread` file with
   `part: receipt`, `outcome: killed`, and `## Why killed`, per the answer.

## Gate

Propose the change. Show the preview and the thread moves together. Wait for the yes,
then apply the change and make the thread writes.

## Hand off

None.
