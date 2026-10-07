---
name: repo-unlink
description: "Unlink a repository from the vault: mark its document unlinked, and decide what happens to the topics under its tag. The document stays, so every link to it holds. The repository on disk is never touched. Use for unlink, remove this repository, forget this repo, the repository moved away. Linking is repo-link."
---

# repo-unlink

A repository can leave the vault while its code stays on disk. The repository document
stays too, marked `unlinked`, so the topics and sessions that cite it keep a live link.
Agents no longer work in the old folder. The topics under the repository's tag must go
somewhere, so this skill asks once, with a recommendation, and then makes the change.

Tools: `context`, `search`, `change`. References:
[changes.md](../almagest/references/changes.md).

## Procedure

1. Call `context` with `repository`: its own tag (`defines`).
2. Count the topics under its own tag: `search` with `tags: [<its tag>]` and
   `types: [topic]`.
3. Ask one question with the count and the recommended answer: keep the topics
   (recommended; they still describe the system, and the tag and the repository document
   remain), or remove the ones the user names.
4. Build the change: a modify of the repository document with
   `fields: {unlinked: true}`. Code empties `path`, drops the git facts, and turns the
   lead callout into `[!repository-missing]`. Add a remove for each topic the user chose
   to remove, with `redirect` to a document that covers its subject when one exists. A
   remove moves the topic to `trash/`. Give every write a `why`. Never remove the
   repository document.

## Gate

Propose the change. Show the preview. Wait for the yes, then apply the change. Apply runs
sync, which takes the path out of `.claude/settings.local.json`. Nothing touches the
folder on disk.

## Hand off

None.
